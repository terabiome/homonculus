# Implementation Guide

## Overview

Step-by-step guide to implement the NUMA-aware K8s cluster with SLM workloads.

## Prerequisites

- Dual-socket NUMA system (2×18 cores, 2×126GB RAM)
- 350GB SSD + 8TB HDD storage
- Base OS image for VMs (Ubuntu/Debian recommended)
- `homonculus` tool installed and configured
- `kubectl` and `k3s` packages ready

---

## Phase 1: Verify System Configuration

### Step 1.1: Check NUMA Topology

```bash
# View NUMA nodes
lscpu | grep NUMA

# Expected output:
# NUMA node(s):          2
# NUMA node0 CPU(s):     0-17,36-53
# NUMA node1 CPU(s):     18-35,54-71

# Detailed topology
numactl --hardware
```

### Step 1.2: Check Storage

```bash
# Verify SSD space
df -h /var/lib/libvirt/images
# Should show ~350GB available

# Verify HDD
lsblk | grep sd
# Should show 8TB disk (e.g., /dev/sdb)
```

### Step 1.3: Verify Hyperthreading

```bash
# Check threads per core
lscpu | grep "Thread(s) per core"
# Expected: 2

# If disabled, enable in BIOS
```

---

## Phase 2: Configure CPU Isolation

### Step 2.1: Calculate CPU Isolation

**Critical for performance!** We need to isolate VM CPUs from the host scheduler using the `isolcpus` kernel parameter.

**Our CPU Allocation:**

```
Node 0 (Socket 0):
├─ VM1 (slm-heavy): threads 0-15, 36-51 (cores 0-15)
├─ Emulator:         threads 16-17, 52-53 (cores 16-17)
└─ Host:            (minimal, uses leftover capacity)

Node 1 (Socket 1):
├─ VM2 (data):      threads 18-23, 54-59 (cores 0-5)
├─ VM3 (slm):       threads 24-27, 60-63 (cores 6-9)
├─ VM4 (tasks):     threads 28-33, 64-69 (cores 10-15)
├─ Emulator:        threads 34-35, 70-71 (cores 16-17, shared)
└─ Host:            (minimal, uses leftover capacity)
```

**Threads to Isolate:** All VM threads + emulator threads = `0-17,36-53,18-35,54-71`

Simplified: `0-35,36-71` (all threads except... wait, that's all of them!)

Actually, we want to isolate **only VM threads**, leaving emulator threads for host scheduler flexibility:

**Final isolation:** `0-15,18-33,36-51,54-69` (VM threads only)

### Step 2.2: Configure GRUB

Edit the GRUB configuration file:

```bash
# Open GRUB config
sudo vi /etc/default/grub
```

Find the `GRUB_CMDLINE_LINUX` line and add `isolcpus`:

```bash
# Before:
GRUB_CMDLINE_LINUX="quiet splash"

# After:
GRUB_CMDLINE_LINUX="quiet splash isolcpus=0-15,18-33,36-51,54-69"
```

**Additional Recommended Parameters:**

```bash
# For even better performance, add these as well:
GRUB_CMDLINE_LINUX="quiet splash isolcpus=0-15,18-33,36-51,54-69 nohz_full=0-15,18-33,36-51,54-69 rcu_nocbs=0-15,18-33,36-51,54-69"
```

**Parameter Explanation:**
- `isolcpus`: Prevents host scheduler from using these CPUs for normal processes
- `nohz_full`: Reduces timer interrupts on isolated CPUs (adaptive tickless mode)
- `rcu_nocbs`: Offloads RCU callbacks to housekeeping CPUs (reduces noise)

### Step 2.3: Rebuild GRUB and Reboot

```bash
# Rebuild GRUB configuration
sudo grub2-mkconfig -o /boot/grub2/grub.cfg

# Or on some systems:
sudo update-grub

# Reboot to apply changes
sudo reboot
```

### Step 2.4: Verify CPU Isolation

After reboot, verify the configuration:

```bash
# Check kernel command line
cat /proc/cmdline | grep isolcpus
# Should show: isolcpus=0-15,18-33,36-51,54-69

# Check isolated CPUs
cat /sys/devices/system/cpu/isolated
# Should show: 0-15,18-33,36-51,54-69

# Verify host is NOT using isolated CPUs
# Run a stress test and check which CPUs are used
taskset -c 16-17,34-35,52-53,70-71 stress-ng --cpu 8 --timeout 10s &
top -H
# Only CPUs 16,17,34,35,52,53,70,71 should show activity

# Check which CPUs the init process can use
taskset -cp 1
# Should show: pid 1's current affinity list: 16,17,34,35,52,53,70,71
# (NOT the isolated CPUs)
```

**Expected Results:**
- ✅ Kernel command line contains `isolcpus`
- ✅ `/sys/devices/system/cpu/isolated` lists your VM CPUs
- ✅ Host processes (`systemd`, `sshd`, etc.) are bound to non-isolated CPUs
- ✅ Isolated CPUs show 0% usage when VMs are idle

**If verification fails:**
1. Double-check GRUB config syntax (no spaces in CPU ranges)
2. Ensure GRUB rebuild command completed successfully
3. Verify system uses GRUB (not another bootloader)
4. Check `dmesg | grep isolcpus` for kernel messages

---

## Phase 3: Prepare Base Image

### Step 3.1: Create Base VM Image

```bash
cd /var/lib/libvirt/images

# Download Ubuntu cloud image
wget https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img \
  -O base.qcow2

# Or use existing image
# cp /path/to/your/base.qcow2 ./base.qcow2
```

### Step 3.2: Customize Base Image (Optional)

```bash
# Install required packages
virt-customize -a base.qcow2 \
  --install qemu-guest-agent,python3,curl \
  --run-command 'systemctl enable qemu-guest-agent'
```

---

## Phase 4: Create VM Configuration

### Step 4.1: Create Configuration File

Create `definitions/homelab/virtualmachine/vm.cluster-numa.json`:

See `examples/definitions/virtualmachine/vm.cluster-numa.json.example` for full config.

**Key points:**
- Set `vcpu_count` to match threads (e.g., 32 = 16 cores × 2)
- Set `vcpu_pins` array with specific thread IDs
- Set `emulator_cpuset` for dedicated emulator cores
- Set `numa_memory.nodeset` to NUMA node (0 or 1)
- Set `numa_memory.mode` to `"strict"`

### Step 4.2: Create Cloud-Init Configs

Create cloud-init ISO for each VM with SSH keys and initial setup.

---

## Phase 5: Create and Start VMs

### Step 5.1: Create VMs

```bash
cd /home/nnurry/code/homonculus

# Build homonculus
go build -o ./homonculus cmd/*

# Create all VMs
./homonculus virtualmachine create \
  --config definitions/homelab/virtualmachine/vm.cluster-numa.json \
  --start
```

### Step 4.2: Verify VM Creation

```bash
# List running VMs
virsh list

# Should show:
# k3s-worker-slm-heavy
# k3s-worker-data
# k3s-worker-slm
# k3s-worker-tasks

# Check CPU pinning
virsh vcpupin k3s-worker-slm-heavy
virsh emulatorpin k3s-worker-slm-heavy

# Check NUMA binding
virsh numatune k3s-worker-slm-heavy
```

### Step 4.3: Get VM IP Addresses

```bash
# Get DHCP leases
virsh net-dhcp-leases default

# Or use qemu-guest-agent
virsh domifaddr k3s-worker-slm-heavy --source agent
```

---

## Phase 6: Configure VM2 (Data Node) Storage

### Step 5.1: Create Hot Tier Disk

```bash
# On host
cd /var/lib/libvirt/images
qemu-img create -f qcow2 data-hot.qcow2 80G

# Attach to VM2
virsh attach-disk k3s-worker-data \
  /var/lib/libvirt/images/data-hot.qcow2 \
  vdb --persistent --subdriver qcow2
```

### Step 5.2: Attach HDD to VM2

```bash
# Passthrough HDD to VM2
virsh attach-disk k3s-worker-data \
  /dev/sdb \
  vdc --persistent
```

### Step 5.3: Format and Mount in VM2

SSH into VM2:

```bash
ssh admin@<vm2-ip>

# Format hot tier
sudo mkfs.ext4 -L hot-storage /dev/vdb
sudo mkdir -p /data/hot
sudo mount /dev/vdb /data/hot

# Add to fstab
echo "LABEL=hot-storage /data/hot ext4 defaults,noatime,nodiratime 0 2" | \
  sudo tee -a /etc/fstab

# Format cold tier (HDD)
sudo mkfs.ext4 -L lake-storage -m 1 /dev/vdc
sudo mkdir -p /data/lake
sudo mount /dev/vdc /data/lake

# Add to fstab
echo "LABEL=lake-storage /data/lake ext4 defaults,noatime,commit=60 0 2" | \
  sudo tee -a /etc/fstab

# Create directories
sudo mkdir -p /data/hot/clean
sudo mkdir -p /data/hot/staging
sudo mkdir -p /data/lake/clean/bronze
sudo mkdir -p /data/lake/clean/silver
sudo mkdir -p /data/lake/clean/gold
sudo mkdir -p /data/lake/archive
sudo mkdir -p /data/lake/raw

# Set permissions
sudo chown -R $(whoami):$(whoami) /data/hot
sudo chown -R $(whoami):$(whoami) /data/lake
```

---

## Phase 7: Install K3s

### Step 7.1: Install K3s on Master Node (Pick One VM)

Choose VM4 (`k3s-server-tasks`) as the K3s server node:

```bash
ssh admin@<vm4-ip>

# Install K3s (server mode)
curl -sfL https://get.k3s.io | sh -s - server --disable traefik

# Get join token
sudo cat /var/lib/rancher/k3s/server/node-token
```

### Step 7.2: Join Worker Nodes

On each worker VM:

```bash
# VM1 (slm-heavy)
curl -sfL https://get.k3s.io | K3S_URL=https://<vm4-ip>:6443 \
  K3S_TOKEN=<token> sh -

# VM2 (data)
curl -sfL https://get.k3s.io | K3S_URL=https://<vm4-ip>:6443 \
  K3S_TOKEN=<token> sh -

# VM3 (slm)
curl -sfL https://get.k3s.io | K3S_URL=https://<vm4-ip>:6443 \
  K3S_TOKEN=<token> sh -
```

### Step 7.3: Verify Cluster

```bash
# On server (VM4)
kubectl get nodes

# Should show all 4 nodes as Ready
# Note: VM4 is both server and available for workloads
```

---

## Phase 8: Apply Kubernetes Taints

### Step 8.1: Taint Nodes

```bash
# VM1: SLM heavy
kubectl taint nodes k3s-worker-slm-heavy workload=slm:NoSchedule

# VM2: Data
kubectl taint nodes k3s-worker-data workload=data:NoSchedule

# VM3: SLM light
kubectl taint nodes k3s-worker-slm workload=slm:NoSchedule

# VM4: Tasks
kubectl taint nodes k3s-worker-tasks workload=tasks:NoSchedule
```

### Step 8.2: Verify Taints

```bash
kubectl describe node k3s-worker-slm-heavy | grep Taints
# Should show: workload=slm:NoSchedule
```

---

## Phase 9: Deploy Application

### Step 9.1: Create SLM Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: slm-inference
spec:
  replicas: 7  # 7 SLM instances
  selector:
    matchLabels:
      app: slm
  template:
    metadata:
      labels:
        app: slm
    spec:
      tolerations:
        - key: "workload"
          operator: "Equal"
          value: "slm"
          effect: "NoSchedule"
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app
                      operator: In
                      values:
                        - slm
                topologyKey: kubernetes.io/hostname
      containers:
        - name: slm
          image: your-slm-image:latest
          env:
            - name: STAGING_DIR
              value: "/data/hot/staging"
            - name: PARQUET_COMPRESSION
              value: "zstd"
            - name: PARQUET_COMPRESSION_LEVEL
              value: "3"
          volumeMounts:
            - name: staging
              mountPath: /data/hot/staging
      volumes:
        - name: staging
          hostPath:
            path: /data/hot/staging
            type: Directory
```

### Step 9.2: Deploy

```bash
kubectl apply -f slm-deployment.yaml

# Verify
kubectl get pods -o wide
# Check that SLM pods are distributed across slm nodes
```

---

## Phase 10: Configure System Tuning

### Step 10.1: Disable NUMA Balancing (Host)

```bash
# Disable automatic NUMA balancing
echo 0 | sudo tee /proc/sys/kernel/numa_balancing

# Make persistent
echo "kernel.numa_balancing = 0" | sudo tee -a /etc/sysctl.conf
```

### Step 10.2: Set CPU Governor (Host)

```bash
# Set to performance mode
for cpu in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
    echo performance | sudo tee $cpu
done

# Make persistent (install cpufrequtils)
sudo apt install cpufrequtils
echo 'GOVERNOR="performance"' | sudo tee /etc/default/cpufrequtils
sudo systemctl restart cpufrequtils
```

### Step 10.3: Enable Transparent Huge Pages

```bash
# Enable THP
echo always | sudo tee /sys/kernel/mm/transparent_hugepage/enabled
echo always | sudo tee /sys/kernel/mm/transparent_hugepage/defrag

# Make persistent
cat <<EOF | sudo tee /etc/rc.local
#!/bin/bash
echo always > /sys/kernel/mm/transparent_hugepage/enabled
echo always > /sys/kernel/mm/transparent_hugepage/defrag
exit 0
EOF
sudo chmod +x /etc/rc.local
```

---

## Phase 11: Set Up Monitoring

### Step 11.1: Install Monitoring Tools (Host)

```bash
sudo apt install -y sysstat numactl

# Enable sysstat
sudo systemctl enable sysstat
sudo systemctl start sysstat
```

### Step 11.2: Create Monitoring Scripts

Create `/usr/local/bin/numa-monitor.sh`:

```bash
#!/bin/bash
# Monitor NUMA statistics

echo "=== NUMA Statistics ==="
numastat -c qemu-kvm

echo ""
echo "=== VM vCPU Usage ==="
for vm in $(virsh list --name); do
    echo "VM: $vm"
    virsh domstats --vcpu $vm | grep "vcpu.*time"
done

echo ""
echo "=== Emulator CPU Usage ==="
for pid in $(pgrep qemu-system-x86_64); do
    echo "PID: $pid"
    ps -L -p $pid | grep emulator
done
```

Make executable and schedule:

```bash
sudo chmod +x /usr/local/bin/numa-monitor.sh

# Run every 5 minutes
echo "*/5 * * * * /usr/local/bin/numa-monitor.sh >> /var/log/numa-monitor.log 2>&1" | \
  sudo crontab -
```

### Step 11.3: Monitor Staging Cleanup (VM2)

Create `/usr/local/bin/staging-cleanup-monitor.sh` on VM2:

```bash
#!/bin/bash
# Monitor staging directory size

STAGING_DIR="/data/hot/staging"
SIZE=$(du -s $STAGING_DIR | awk '{print $1}')
SIZE_GB=$((SIZE / 1024 / 1024))

if [ $SIZE_GB -gt 10 ]; then
    logger -t staging-monitor "WARNING: Staging size is ${SIZE_GB}GB (>10GB threshold)"
fi
```

Make executable and schedule:

```bash
sudo chmod +x /usr/local/bin/staging-cleanup-monitor.sh
echo "*/5 * * * * /usr/local/bin/staging-cleanup-monitor.sh" | crontab -
```

---

## Phase 12: Verify Performance

### Step 12.1: Check NUMA Locality

```bash
# On host
numastat -c qemu-kvm

# Look for:
# - numa_hit high (good!)
# - numa_foreign low (<1% of numa_hit, good!)
# - numa_miss low
```

### Step 12.2: Check CPU Pinning

```bash
# Verify vCPU pinning
for vm in $(virsh list --name); do
    echo "=== $vm ==="
    virsh vcpupin $vm
    virsh emulatorpin $vm
done

# All vCPUs should show specific CPU pinning
# Emulators should show dedicated cores
```

### Step 12.3: Benchmark SLM Inference

Run inference workload and measure:
- Latency (should be consistent, low jitter)
- Throughput (operations per second)
- CPU utilization (should be high on pinned cores)

### Step 12.4: Check Storage Performance

```bash
# On VM2
# Check hot tier latency
fio --name=random-read --ioengine=libaio --rw=randread --bs=4k \
    --numjobs=4 --size=1G --runtime=60 --time_based \
    --directory=/data/hot

# Should see latency < 1ms, IOPS > 10k

# Check cold tier throughput
dd if=/dev/zero of=/data/lake/test.bin bs=1M count=1024 oflag=direct
# Should see ~150 MB/s
rm /data/lake/test.bin
```

---

## Troubleshooting

### VMs Not Starting

**Check:**
```bash
# View VM logs
virsh console <vm-name>

# Check libvirt logs
tail -f /var/log/libvirt/qemu/<vm-name>.log
```

### CPU Pinning Not Working

**Check:**
```bash
# Verify pinning applied
virsh vcpupin <vm-name>

# Check for conflicts (no other process using pinned CPUs)
taskset -pc $(pgrep qemu-system-x86_64)
```

### High Cross-NUMA Traffic

**Check:**
```bash
numastat -c qemu-kvm

# If numa_foreign > 5% of numa_hit:
# - Verify numa_memory.mode = "strict" in config
# - Check applications aren't requesting cross-NUMA memory
```

### Data Lake Query Performance

**Check:**
```bash
# On data node (VM2)
cd /data/lake/clean
find . -name "*.parquet" -exec ls -lh {} \; | head -20

# Look for:
# - Too many small files (< 10MB)
# - Very large files (> 500MB)
# - Missing partitioning structure
```

**Fix:**
- Compact small files
- Add partitioning by date/category
- Use Hive-style partitions for better query performance

---

## Rollback Procedure

If something goes wrong:

```bash
# Stop and delete VMs
for vm in $(virsh list --name); do
    virsh destroy $vm
    virsh undefine $vm --remove-all-storage
done

# Restore base image
cd /var/lib/libvirt/images
# (Keep backup of base.qcow2)

# Start over from Phase 4
```

---

## Next Steps

1. Monitor performance for 24-48 hours
2. Tune based on actual workload patterns
3. Implement backup strategy for data lake
4. Set up alerting for critical metrics
5. Document data schemas and quality rules
6. Implement data catalog for lake zones

---

## References

- Architecture overview: `docs/architecture/numa-design.md`
- VM specifications: `docs/architecture/vm-specifications.md`
- Storage design: `docs/architecture/storage-design.md`
- Tuning guide: `docs/guides/tuning-guide.md`

