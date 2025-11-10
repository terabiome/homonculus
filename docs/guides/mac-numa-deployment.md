# Mac Mini + NUMA Server K3s Deployment Guide

This guide walks you through deploying the complete hybrid K3s cluster with Mac Mini as control plane and NUMA server running 3 worker VMs: 2 for SLM compute and 1 for storage/data processing.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Phase 1: Mac Mini Setup](#phase-1-mac-mini-setup)
3. [Phase 2: NUMA Server VM Creation](#phase-2-numa-server-vm-creation)
4. [Phase 3: K3s Cluster Setup](#phase-3-k3s-cluster-setup)
5. [Phase 4: Deploy Core Services](#phase-4-deploy-core-services)
6. [Phase 5: Verification](#phase-5-verification)
7. [Phase 6: Deploy Workloads](#phase-6-deploy-workloads)

---

## Prerequisites

### Hardware Requirements

**Mac Mini M2:**
- 8GB RAM (minimum)
- No external HDD needed (lightweight control plane only)
- 1GbE network connection
- Static IP recommended: `192.168.1.10`

**NUMA Server:**
- Dual E5-2697v4 (36 cores, 72 threads)
- 252GB RAM
- ~350GB SSD for VM disks
- 8TB HDD for MinIO data lake
- 1GbE network connection
- Static IP recommended: `192.168.1.20`

### Software Requirements

**Mac Mini:**
- macOS (any recent version)
- Multipass or Lima (for Linux VM)
- kubectl
- Access to terminal

**NUMA Server:**
- Linux (Ubuntu 22.04 recommended)
- libvirt/KVM/QEMU
- Bridge networking configured
- 8TB HDD connected (will be passed to VM3)

---

## Phase 1: Mac Mini Setup

### Step 1.1: Disable macOS Sleep

```bash
# Prevent Mac from sleeping (CRITICAL!)
sudo pmset -a sleep 0
sudo pmset -a disksleep 0
sudo pmset -a displaysleep 30

# Disable auto-updates (do manually when YOU choose)
sudo softwareupdate --schedule off
```

### Step 1.2: Install Lima (Recommended) or Multipass

**Option A: Lima (Recommended - Faster, Better ARM64 Support)**

```bash
# Install Lima via Homebrew
brew install lima

# Verify installation
limactl --version
```

**Option B: Multipass (Alternative)**

```bash
# Install Multipass via Homebrew
brew install --cask multipass

# Verify installation
multipass version
```

### Step 1.3: Create K3s Master VM

**Using Lima:**

```bash
# Create Ubuntu VM for K3s master
# 4GB RAM, 20GB disk
limactl create --name=k3s-master \
  --cpus=4 \
  --memory=4 \
  --disk=20 \
  --vm-type=vz \
  --rosetta \
  template://ubuntu-lts

# Start VM
limactl start k3s-master

# Get VM IP (save this!)
limactl list
# Note the IP address (e.g., 192.168.5.15)
```

**Using Multipass:**

```bash
# Create Ubuntu VM for K3s master
multipass launch \
  --name k3s-master \
  --cpus 4 \
  --memory 4G \
  --disk 20G \
  22.04

# Get VM IP (save this!)
multipass info k3s-master | grep IPv4
```

### Step 1.4: Install K3s Inside VM

**Using Lima:**

```bash
# Enter VM shell
limactl shell k3s-master

# Inside the VM:

# Install K3s in server mode
curl -sfL https://get.k3s.io | sh -s - server \
  --disable traefik \
  --disable servicelb \
  --write-kubeconfig-mode 644 \
  --node-name mac-master \
  --node-label role=master \
  --bind-address 0.0.0.0 \
  --advertise-address $(hostname -I | awk '{print $1}')

# Wait for K3s to start
sleep 30

# Verify K3s is running
sudo systemctl status k3s

# Check node
sudo k3s kubectl get nodes
# Should show: mac-master   Ready    control-plane,master   1m

# Exit VM shell
exit
```

**Using Multipass:**

```bash
# Enter VM shell
multipass shell k3s-master

# (Same K3s installation commands as above)
```

### Step 1.5: Configure kubectl on Mac

**Using Lima:**

```bash
# Copy kubeconfig from VM to Mac
limactl shell k3s-master sudo cat /etc/rancher/k3s/k3s.yaml > ~/.kube/k3s-config

# Get VM IP
VM_IP=$(limactl list k3s-master --format='{{.IPAddress}}')

# Update kubeconfig with VM IP
sed -i.bak "s/127.0.0.1/$VM_IP/g" ~/.kube/k3s-config

# Set as default kubeconfig
export KUBECONFIG=~/.kube/k3s-config
echo 'export KUBECONFIG=~/.kube/k3s-config' >> ~/.zshrc  # or ~/.bash_profile

# Verify kubectl access from Mac
kubectl get nodes
# Should show: mac-master   Ready    control-plane,master   2m
```

**Using Multipass:**

```bash
# Copy kubeconfig from VM to Mac
multipass exec k3s-master -- sudo cat /etc/rancher/k3s/k3s.yaml > ~/.kube/k3s-config

# Get VM IP
VM_IP=$(multipass info k3s-master | grep IPv4 | awk '{print $2}')

# (Rest is the same as Lima)
```

### Step 1.6: Get Join Token

**Using Lima:**

```bash
# Get token for worker nodes (from inside VM)
limactl shell k3s-master sudo cat /var/lib/rancher/k3s/server/node-token

# Save this token - you'll need it for NUMA worker VMs!
# Example: K10abc123::server:xyz789...

# Also note the VM IP (you'll need this for K3S_URL)
# Example: https://192.168.5.15:6443
```

**Using Multipass:**

```bash
# Get token for worker nodes (from inside VM)
multipass exec k3s-master -- sudo cat /var/lib/rancher/k3s/server/node-token

# Save this token!
```

**Important Notes:**
- VM IP is on Lima/Multipass network (192.168.x.x)
- NUMA server worker nodes must reach this IP
- May need to configure Mac firewall to allow 6443
- Alternatively, use Mac's main network IP if workers can't reach VM IP

---

## Phase 2: NUMA Server VM Creation

### Step 2.1: Prepare Base Image

```bash
# On NUMA server

# Download Ubuntu 22.04 cloud image
cd /var/lib/libvirt/images
sudo wget https://cloud-images.ubuntu.com/releases/22.04/release/ubuntu-22.04-server-cloudimg-amd64.img

# Rename as base image
sudo mv ubuntu-22.04-server-cloudimg-amd64.img ubuntu-22.04-base.qcow2
```

### Step 2.2: Create VMs Using Homonculus

```bash
# Clone homonculus repo (if not already done)
cd ~/code
git clone <your-repo-url> homonculus
cd homonculus

# Build homonculus
go build -o homonculus ./cmd

# Update VM definitions with your SSH key
# Edit: definitions/mac-numa-cluster/vm*.json
# Replace: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC... your-key-here"
#   with your actual SSH public key

# Create VM1 (SLM Worker 1)
sudo ./homonculus vm create \
  --definition definitions/mac-numa-cluster/vm1-slm-worker-1.json

# Create VM2 (SLM Worker 2)
sudo ./homonculus vm create \
  --definition definitions/mac-numa-cluster/vm2-slm-worker-2.json

# Start both VMs
sudo virsh start k3s-slm-worker-1
sudo virsh start k3s-slm-worker-2

# Get VM IPs (wait ~30 seconds for cloud-init)
sudo virsh domifaddr k3s-slm-worker-1
sudo virsh domifaddr k3s-slm-worker-2

# Note the IPs - you'll need them!
```

### Step 2.3: Configure VMs

SSH into each VM and configure:

```bash
# SSH into VM1 (replace IP)
ssh admin@<vm1-ip>

# Create local staging directory
sudo mkdir -p /data/local
sudo chmod 777 /data/local

# Update system
sudo apt update && sudo apt upgrade -y

# Install required packages
sudo apt install -y curl

# Exit and repeat for VM2
```

---

## Phase 3: K3s Cluster Setup

### Step 3.1: Join Worker Nodes

On **each VM** (VM1, VM2):

```bash
# Set variables
export K3S_URL="https://192.168.1.10:6443"
export K3S_TOKEN="<your-token-from-mac>"  # From Phase 1, Step 1.5

# Install K3s agent
curl -sfL https://get.k3s.io | sh -s - agent \
  --node-name $(hostname) \
  --node-label role=worker

# Verify agent is running
sudo systemctl status k3s-agent
```

### Step 3.2: Verify Cluster

Back on **Mac Mini**:

```bash
# Check all nodes joined
kubectl get nodes

# Should show:
# NAME                 STATUS   ROLES                  AGE
# mac-master           Ready    control-plane,master   10m
# k3s-slm-worker-1     Ready    <none>                 2m
# k3s-slm-worker-2     Ready    <none>                 2m
```

### Step 3.3: Label Nodes (Whitelist Strategy)

On **Mac Mini**:

```bash
# Label VM1 (SLM worker 1, NUMA node 0)
kubectl label node k3s-slm-worker-1 workload=slm numa-node=0

# Label VM2 (SLM worker 2, NUMA node 1)
kubectl label node k3s-slm-worker-2 workload=slm numa-node=1

# Verify labels
kubectl get nodes --show-labels

# Note: No taints! We use whitelist via nodeSelector
# SLM pods explicitly request workload=slm
# Temporal workers request role=worker
```

---

## Phase 4: Deploy Core Services

### Step 4.1: Deploy Storage (MinIO)

```bash
# On Mac Mini (from homonculus directory)

# Create namespace
kubectl apply -f k8s/storage/00-namespace.yaml

# Deploy MinIO
kubectl apply -f k8s/storage/01-minio.yaml

# Wait for MinIO to be ready (may take 2-3 minutes)
kubectl wait --for=condition=ready pod -l app=minio -n storage --timeout=300s

# Verify MinIO is running
kubectl get pods -n storage
kubectl get svc -n storage

# Access MinIO Console
open http://192.168.1.10:30901
# Login: admin / locness-lake-password
```

### Step 4.2: Deploy Monitoring

```bash
# Create namespace
kubectl apply -f k8s/monitoring/00-namespace.yaml

# Deploy VictoriaLogs
kubectl apply -f k8s/monitoring/01-victorialogs.yaml

# Deploy VictoriaMetrics
kubectl apply -f k8s/monitoring/02-victoriametrics.yaml

# Wait for Victoria services
kubectl wait --for=condition=ready pod -l app=victorialogs -n monitoring --timeout=300s
kubectl wait --for=condition=ready pod -l app=victoriametrics -n monitoring --timeout=300s

# Deploy log/metric collectors
kubectl apply -f k8s/monitoring/03-promtail.yaml
kubectl apply -f k8s/monitoring/04-vmagent.yaml

# Verify monitoring stack
kubectl get pods -n monitoring

# Access UIs
open http://192.168.1.10:30428/select/vmui  # VictoriaLogs
open http://192.168.1.10:30828/vmui         # VictoriaMetrics
```

### Step 4.3: Deploy Temporal

```bash
# Create namespace
kubectl apply -f k8s/temporal/00-namespace.yaml

# Deploy Temporal server
kubectl apply -f k8s/temporal/01-temporal-server.yaml

# Deploy Temporal UI
kubectl apply -f k8s/temporal/02-temporal-ui.yaml

# Wait for Temporal (may take 2-3 minutes)
kubectl wait --for=condition=ready pod -l app=temporal -n temporal --timeout=300s

# Verify Temporal
kubectl get pods -n temporal

# Access Temporal UI
open http://192.168.1.10:30880
```

### Step 4.4: Deploy Temporal Workers

**IMPORTANT:** First, build your Temporal worker image!

```bash
# Example Dockerfile for Temporal worker
# See: docs/examples/temporal-worker-dockerfile

# Build worker image
docker build -t your-registry/temporal-worker:latest .

# Push to registry (or load to kind/k3s)
docker push your-registry/temporal-worker:latest

# Update image in k8s/workers/01-temporal-worker-daemonset.yaml
# Then deploy:
kubectl apply -f k8s/workers/01-temporal-worker-daemonset.yaml

# Verify workers are running on all worker nodes
kubectl get pods -n temporal -l app=temporal-worker -o wide
# Should show 2 pods (one per worker VM)
```

---

## Phase 5: Verification

### Step 5.1: Check All Pods

```bash
# All pods should be Running
kubectl get pods -A

# Check pod distribution
kubectl get pods -A -o wide | grep mac-master     # Master node pods
kubectl get pods -A -o wide | grep slm-worker     # SLM worker pods
kubectl get pods -A -o wide | grep data-worker    # Data worker pods
```

### Step 5.2: Check Services

```bash
# All services
kubectl get svc -A

# Test MinIO from within cluster
kubectl run -it --rm test --image=busybox --restart=Never -- sh
# Inside pod:
wget -O- http://minio.storage.svc.cluster.local:9000
```

### Step 5.3: Check Resource Usage

```bash
# Node resources
kubectl top nodes

# Should show:
# NAME                 CPU   MEMORY
# mac-master           25%   7GB/8GB          ← Mac should be ~85-95%
# k3s-slm-worker-1     5%    2GB/120GB        ← Workers idle for now
# k3s-slm-worker-2     5%    2GB/120GB
```

### Step 5.4: Test Logging

```bash
# Generate some logs
kubectl run test-logger --image=busybox --restart=Never -- sh -c "for i in {1..100}; do echo 'Test log message '$i; sleep 1; done"

# Wait 30 seconds, then check VictoriaLogs
open http://192.168.1.10:30428/select/vmui

# Query: _stream:{pod="test-logger"}
# You should see 100 log messages!

# Cleanup
kubectl delete pod test-logger
```

### Step 5.5: Test Metrics

```bash
# Check VictoriaMetrics has data
open http://192.168.1.10:30828/vmui

# Query: up
# You should see metrics for all services!

# Example queries:
# - container_memory_usage_bytes
# - node_cpu_seconds_total
# - minio_cluster_disk_total_bytes
```

---

## Phase 6: Deploy Workloads

### Important: Resource Guarantees

**CRITICAL:** The Temporal worker DaemonSet has resource guarantees to prevent CPU starvation:

```yaml
Temporal Worker (per VM):
  requests:
    cpu: "2000m"        # Guaranteed 2 cores
    memory: "4Gi"       # Guaranteed 4GB
  limits:
    cpu: "2000m"        # No burst (strict)
    memory: "8Gi"

SLM Pods (example):
  requests:
    cpu: "6000m"        # Guaranteed 6 cores
    memory: "20Gi"      # Guaranteed 20GB
  limits:
    cpu: "10000m"       # Can burst to 10 cores
    memory: "30Gi"

Why this matters:
├─ Temporal workers MUST get CPU to sweep results
├─ Without guarantees: SLM inference can starve Temporal
├─ With guarantees: K8s reserves 2 cores for Temporal
└─ Result: Workflow never hangs! ✅
```

See `k8s/examples/slm-pod-example.yaml` for proper SLM pod resource allocation.

---

### Step 6.1: Create MinIO Buckets

```bash
# Access MinIO Console: http://192.168.1.10:30901

# Create buckets:
# - finance-inputs
# - finance-staging
# - finance-gold
# - finance-archive
# - patterns
# - models

# Or via kubectl:
kubectl run -it --rm mc --image=minio/mc --restart=Never -- sh
# Inside pod:
mc alias set myminio http://minio.storage.svc.cluster.local:9000 admin locness-lake-password
mc mb myminio/finance-inputs
mc mb myminio/finance-staging
mc mb myminio/finance-gold
mc mb myminio/finance-archive
mc mb myminio/patterns
mc mb myminio/models
```

### Step 6.2: Deploy SLM Workloads

(This depends on your specific SLM implementation)

Example StatefulSet for SLM pods:
```yaml
# See: docs/examples/slm-statefulset.yaml
# Deploy your SLM pods with:
# - nodeSelector: workload=slm
# - toleration: workload=slm:NoSchedule
# - volumeMount: /data/local (hostPath)
```

### Step 6.3: Create Temporal Workflows

(This depends on your Temporal implementation)

Example workflow registration:
```bash
# Access Temporal UI: http://192.168.1.10:30880
# Register your workflows:
# - daily-data-prep
# - finance-analysis
# - result-aggregation
```

---

## Common Issues

### Mac Mini Out of Memory

**Symptoms:**
```bash
kubectl top nodes
# mac-master: 95%+ memory usage
```

**Solution:**
```bash
# Reduce pod memory limits
kubectl edit deployment minio -n storage
# Change: memory: "1Gi" → memory: "512Mi"

# Or move services to Linux VMs
# (See: docs/architecture/mac-numa-k3s-architecture.md)
```

### Pod Stuck in Pending

**Symptoms:**
```bash
kubectl get pods -A
# minio-xxx: Pending
```

**Diagnosis:**
```bash
kubectl describe pod minio-xxx -n storage
# Check: Events section for errors
```

**Common causes:**
1. PersistentVolume path doesn't exist
2. Node selector doesn't match
3. Resource limits too high

### Worker Node Not Joining

**Symptoms:**
```bash
kubectl get nodes
# Only shows mac-master
```

**Diagnosis on worker VM:**
```bash
sudo systemctl status k3s-agent
sudo journalctl -u k3s-agent -f
```

**Common causes:**
1. Wrong K3S_URL or K3S_TOKEN
2. Firewall blocking port 6443
3. Network issue between Mac and VM

### Temporal Workers Not Starting

**Symptoms:**
```bash
kubectl get pods -n temporal
# temporal-worker-xxx: CrashLoopBackOff
```

**Diagnosis:**
```bash
kubectl logs -n temporal temporal-worker-xxx
```

**Common causes:**
1. Worker image not built/pushed
2. MinIO credentials wrong
3. /data/local not accessible

---

## Next Steps

1. **Configure Workflows:** Set up your Temporal workflows for finance analysis
2. **Deploy SLM Models:** Deploy your SLM pods with proper node affinity
3. **Set Up Monitoring:** Create dashboards in VictoriaMetrics/Grafana
4. **Test End-to-End:** Run a complete finance analysis job

---

## References

- Architecture: `docs/architecture/mac-numa-k3s-architecture.md`
- K8s Manifests: `k8s/README.md`
- Finance Use Case: `docs/use-cases/finance-ensemble.md`
- VM Definitions: `definitions/mac-numa-cluster/`

---

## Congratulations! 🎉

You've successfully deployed a hybrid K3s cluster with Mac Mini + NUMA server!

Your cluster is now ready to run high-performance SLM workloads with centralized storage and monitoring.

**Total deployment time:** ~1-2 hours (first time)

**Making that Mac Mini purchase worth it!** 💪😎

