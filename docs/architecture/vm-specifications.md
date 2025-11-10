# VM Specifications

## Overview

This document provides detailed specifications for each VM in the NUMA-aware K8s cluster.

## VM Allocation Matrix

### Node 0 (Socket 0) - 18 Physical Cores Total

| Component | Cores | Threads | Purpose |
|-----------|-------|---------|---------|
| VM1 (slm-heavy) | 16 | 32 | Heavy SLM inference workload |
| Emulator + Host | 2 | 4 | QEMU emulator + host processes |

### Node 1 (Socket 1) - 18 Physical Cores Total

| Component | Cores | Threads | Purpose |
|-----------|-------|---------|---------|
| VM2 (data) | 6 | 12 | Database + data lake workload |
| VM3 (slm) | 4 | 8 | Light SLM inference workload |
| VM4 (server-tasks) | 6 | 12 | K3s control plane + heavy parallel processing |
| Emulator + Host | 2 | 4 | QEMU emulator (shared) + host |

## VM1: k3s-worker-slm-heavy

### Purpose
Heavy SLM ensemble inference - runs multiple SLM models continuously for parallel inference and cross-checking.

### Specifications

```json
{
  "name": "k3s-worker-slm-heavy",
  "vcpu_count": 32,
  "memory_mb": 120832,
  "disk_path": "/var/lib/libvirt/images/slm-heavy.qcow2",
  "disk_size_gb": 40,
  "base_image_path": "/var/lib/libvirt/images/base.qcow2",
  "bridge_network_interface": "br0",
  "cloud_init_iso_path": "/var/lib/libvirt/images/slm-heavy-cloud-init.iso",
  "tuning": {
    "vcpu_pins": [
      "0", "36", "1", "37", "2", "38", "3", "39",
      "4", "40", "5", "41", "6", "42", "7", "43",
      "8", "44", "9", "45", "10", "46", "11", "47",
      "12", "48", "13", "49", "14", "50", "15", "51"
    ],
    "emulator_cpuset": "16-17,52-53",
    "numa_memory": {
      "nodeset": "0",
      "mode": "strict"
    }
  },
  "user_configs": [
    {
      "username": "admin",
      "ssh_authorized_keys": ["ssh-rsa AAAAB3NzaC1yc2E... user@host"]
    }
  ]
}
```

### Resource Allocation

- **vCPUs**: 32 (16 physical cores with hyperthreading)
- **CPU Pinning**: Threads 0-15, 36-51 (cores 0-15 on socket 0)
- **Emulator**: Threads 16-17, 52-53 (cores 16-17 on socket 0)
- **Memory**: 118 GB (strict binding to NUMA node 0)
- **Storage**: 40 GB SSD (OS + applications)
- **Network**: Bridge interface br0

### Workload Characteristics

- **Type**: CPU-intensive, memory-bound
- **Pattern**: Continuous inference, low I/O
- **Latency**: Sensitive (ensemble cross-checking)
- **Cache**: Exclusive L3 cache (45 MB, no sharing)

### K8s Configuration

```yaml
# Node taint
taints:
  - key: "workload"
    value: "slm"
    effect: "NoSchedule"

# Pod toleration (for SLM workloads)
tolerations:
  - key: "workload"
    operator: "Equal"
    value: "slm"
    effect: "NoSchedule"
```

**Note:** Both `slm-heavy` and `slm` nodes use the same taint. Kubernetes will schedule pods based on resource requests to the appropriate node.

---

## VM2: k3s-worker-data

### Purpose
Data lake storage and processing - handles data ingestion, processing, and storage in a lakehouse architecture.

### Specifications

```json
{
  "name": "k3s-worker-data",
  "vcpu_count": 12,
  "memory_mb": 61440,
  "disk_path": "/var/lib/libvirt/images/data.qcow2",
  "disk_size_gb": 40,
  "base_image_path": "/var/lib/libvirt/images/base.qcow2",
  "bridge_network_interface": "br0",
  "cloud_init_iso_path": "/var/lib/libvirt/images/data-cloud-init.iso",
      "tuning": {
    "vcpu_pins": [
      "18", "54", "19", "55", "20", "56", "21", "57",
      "22", "58", "23", "59"
    ],
    "emulator_cpuset": "34-35,70-71",
    "numa_memory": {
      "nodeset": "1",
      "mode": "strict"
    }
  },
  "user_configs": [
    {
      "username": "admin",
      "ssh_authorized_keys": ["ssh-rsa AAAAB3NzaC1yc2E... user@host"]
    }
  ]
}
```

### Resource Allocation

- **vCPUs**: 12 (6 physical cores with hyperthreading)
- **CPU Pinning**: Threads 18-23, 54-59 (cores 0-5 on socket 1)
- **Emulator**: Threads 34-35, 70-71 (shared with VM3, VM4)
- **Memory**: 60 GB (strict binding to NUMA node 1)
- **Storage**:
  - OS: 40 GB SSD (`/dev/vda`)
  - Hot tier: 80 GB SSD (`/dev/vdb` → `/data/hot`)
  - Cold tier: 8 TB HDD (`/dev/vdc` → `/data/lake`)

### Storage Layout

```
/dev/vda (40GB SSD): Root filesystem
├─ /: OS, binaries, configs
├─ /var: Logs, temporary files
└─ /tmp: Temporary staging

/dev/vdb (80GB SSD): Hot storage
├─ /data/hot/clean: 20-40GB (frequently accessed clean data)
├─ /data/hot/staging: 4-8GB (compressed intermediate files)
└─ Free space: 32-56GB buffer

/dev/vdc (8TB HDD): Cold storage
└─ /data/lake: Parquet files with schemas, archives
    ├─ clean/bronze: Raw ingested data
    ├─ clean/silver: Cleaned, validated data
    ├─ clean/gold: Business-ready aggregated data
    ├─ archive: Historical data
    └─ raw: Unprocessed input data
```

### Workload Characteristics

- **Type**: Mixed (CPU, memory, I/O)
- **Pattern**: Data lake queries + bulk writes
- **Latency**: Moderate (lake queries important, not critical)
- **Cache**: Shared L3 cache with VM3, VM4

### K8s Configuration

```yaml
# Node taint
taints:
  - key: "workload"
    value: "data"
    effect: "NoSchedule"

# Pod toleration
tolerations:
  - key: "workload"
    operator: "Equal"
    value: "data"
    effect: "NoSchedule"
```

---

## VM3: k3s-worker-slm

### Purpose
Light SLM inference - handles lighter SLM workloads and assists with cross-checking.

### Specifications

```json
{
  "name": "k3s-worker-slm",
  "vcpu_count": 8,
  "memory_mb": 32768,
  "disk_path": "/var/lib/libvirt/images/slm.qcow2",
  "disk_size_gb": 40,
  "base_image_path": "/var/lib/libvirt/images/base.qcow2",
  "bridge_network_interface": "br0",
  "cloud_init_iso_path": "/var/lib/libvirt/images/slm-cloud-init.iso",
      "tuning": {
    "vcpu_pins": [
      "24", "60", "25", "61", "26", "62", "27", "63"
    ],
    "emulator_cpuset": "34-35,70-71",
    "numa_memory": {
      "nodeset": "1",
      "mode": "strict"
    }
  },
  "user_configs": [
    {
      "username": "admin",
      "ssh_authorized_keys": ["ssh-rsa AAAAB3NzaC1yc2E... user@host"]
    }
  ]
}
```

### Resource Allocation

- **vCPUs**: 8 (4 physical cores with hyperthreading)
- **CPU Pinning**: Threads 24-27, 60-63 (cores 6-9 on socket 1)
- **Emulator**: Threads 34-35, 70-71 (shared)
- **Memory**: 32 GB (strict binding to NUMA node 1)
- **Storage**: 40 GB SSD (OS + model weights)

### Workload Characteristics

- **Type**: CPU-intensive, memory-bound
- **Pattern**: Inference workload (lighter than VM1)
- **Latency**: Sensitive
- **Cache**: Shared L3 cache with VM2, VM4

### K8s Configuration

```yaml
# Node taint
taints:
  - key: "workload"
    value: "slm"
    effect: "NoSchedule"

# Pod toleration
tolerations:
  - key: "workload"
    operator: "Equal"
    value: "slm"
    effect: "NoSchedule"
```

---

## VM4: k3s-server-tasks

### Purpose
K3s control plane server + task processing - runs the K3s API server, scheduler, controller-manager, and also handles ETL jobs, batch processing, and coordination tasks.

### Specifications

```json
{
  "name": "k3s-server-tasks",
  "vcpu_count": 12,
  "memory_mb": 24576,
  "disk_path": "/var/lib/libvirt/images/tasks.qcow2",
  "disk_size_gb": 40,
  "base_image_path": "/var/lib/libvirt/images/base.qcow2",
  "bridge_network_interface": "br0",
  "cloud_init_iso_path": "/var/lib/libvirt/images/tasks-cloud-init.iso",
      "tuning": {
    "vcpu_pins": [
      "28", "64", "29", "65", "30", "66", "31", "67",
      "32", "68", "33", "69"
    ],
    "emulator_cpuset": "34-35,70-71",
    "numa_memory": {
      "nodeset": "1",
      "mode": "strict"
    }
  },
  "user_configs": [
    {
      "username": "admin",
      "ssh_authorized_keys": ["ssh-rsa AAAAB3NzaC1yc2E... user@host"]
    }
  ]
}
```

### Resource Allocation

- **vCPUs**: 12 (6 physical cores with hyperthreading)
- **CPU Pinning**: Threads 28-33, 64-69 (cores 10-15 on socket 1)
- **Emulator**: Threads 34-35, 70-71 (shared)
- **Memory**: 24 GB (strict binding to NUMA node 1)
- **Storage**: 40 GB SSD (OS + applications)

### Workload Characteristics

- **Type**: Mixed (K3s control plane + batch processing)
- **Pattern**: K3s API requests + batch jobs, coordination
- **Latency**: Medium (API server should be responsive)
- **Cache**: Shared L3 cache with VM2, VM3
- **Additional**: Hosts K3s etcd, API server, scheduler, controller-manager

### K8s Configuration

```yaml
# Node taints (control plane + workload)
taints:
  - key: "node-role.kubernetes.io/control-plane"
    effect: "NoSchedule"
  - key: "workload"
    value: "tasks"
    effect: "NoSchedule"

# Task pod toleration
tolerations:
  - key: "workload"
    operator: "Equal"
    value: "tasks"
    effect: "NoSchedule"

# K3s system pods automatically tolerate control-plane taint
```

---

## Emulator CPU Allocation

### Node 0 Emulator (VM1 Only)

```
Cores: 16-17 on socket 0
Threads: 16-17, 52-53
Usage:
├─ VM1 (slm-heavy) QEMU emulator threads
└─ Host processes (libvirt, monitoring)
```

**Rationale:**
- VM1 has high I/O for model loading
- Dedicated emulator cores prevent interference with inference

### Node 1 Emulator (VM2, VM3, VM4 Shared)

```
Cores: 16-17 on socket 1
Threads: 34-35, 70-71
Usage:
├─ VM2 (data) QEMU emulator threads
├─ VM3 (slm) QEMU emulator threads
├─ VM4 (tasks) QEMU emulator threads
└─ Host processes
```

**Rationale:**
- 3 VMs share emulator cores (acceptable overhead)
- Host does minimal work (virtualization only)
- Monitor if emulator CPU usage > 50% (add cores if needed)

---

## Performance Tuning Parameters

### CPU Governor

```bash
# Set to performance mode for all VMs
for cpu in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
    echo performance > $cpu
done
```

### NUMA Balancing (Disable)

```bash
# Disable automatic NUMA balancing (we handle it explicitly)
echo 0 > /proc/sys/kernel/numa_balancing
```

### Transparent Huge Pages

```bash
# Enable THP for better memory performance
echo always > /sys/kernel/mm/transparent_hugepage/enabled
echo always > /sys/kernel/mm/transparent_hugepage/defrag
```

### VM CPU Affinity Verification

```bash
# Verify vCPU pinning
virsh vcpupin k3s-worker-slm-heavy

# Verify emulator pinning
virsh emulatorpin k3s-worker-slm-heavy

# Verify NUMA node binding
virsh numatune k3s-worker-slm-heavy
```

---

## Monitoring

### Key Metrics to Track

1. **CPU utilization per VM**
   ```bash
   virsh domstats --vcpu k3s-worker-slm-heavy
   ```

2. **NUMA statistics**
   ```bash
   numastat -c qemu-kvm
   ```

3. **Emulator CPU usage**
   ```bash
   top -H -p $(pgrep qemu-kvm) | grep emulator
   ```

4. **Memory usage per NUMA node**
   ```bash
   numastat -m
   ```

5. **Cache hit rates** (if available)
   ```bash
   perf stat -e LLC-loads,LLC-load-misses -p <qemu-pid>
   ```

---

## Troubleshooting

### High Cross-NUMA Traffic

**Symptom:** `numastat` shows high remote memory access

**Check:**
```bash
numastat -c qemu-kvm
# Look for numa_foreign > 1% of total
```

**Fix:**
- Verify `numa_memory.mode = strict` in VM config
- Check that applications aren't explicitly requesting memory from remote node

### Emulator CPU Bottleneck

**Symptom:** Emulator threads using > 50% of allocated cores

**Check:**
```bash
top -H -p $(pgrep qemu-system-x86_64)
# Look for emulator threads
```

**Fix:**
- Add 1 more core to emulator pool (e.g., cores 16-18 instead of 16-17)
- Or reduce I/O load (use caching, batch operations)

### vCPU Scheduling Delays

**Symptom:** High latency in SLM inference

**Check:**
```bash
virsh domstats --vcpu k3s-worker-slm-heavy
# Look for vcpu.wait time
```

**Fix:**
- Verify no other processes using pinned CPUs
- Check CPU governor is set to `performance`
- Verify emulator isn't stealing vCPU cores

---

## References

- Architecture overview: `docs/architecture/numa-design.md`
- Storage design: `docs/architecture/storage-design.md`
- Implementation guide: `docs/guides/implementation.md`

