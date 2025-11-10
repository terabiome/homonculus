# Mac Mini + NUMA Server K3s Architecture

## Overview

This document describes the **hybrid K3s cluster** architecture with a Mac Mini M2 as the pure control plane and a dual-socket NUMA server running 3 worker VMs: 2 for SLM compute and 1 for storage/data processing.

---

## System Topology

```
┌─────────────────────────────────────────────────────┐
│  Mac Mini M2 (Pure Control Plane)                  │
│  ├─ K3s Master (control plane only)                 │
│  ├─ VictoriaLogs (log aggregation + UI)             │
│  ├─ VictoriaMetrics (metrics + UI)                  │
│  ├─ Temporal Server (workflow orchestration)        │
│  └─ Temporal UI                                      │
│                                                      │
│  Hardware:                                           │
│  ├─ CPU: M2 (8 cores)                               │
│  ├─ RAM: 8GB                                         │
│  ├─ Storage: VM disk only (~20GB)                   │
│  └─ Network: 1GbE (192.168.1.10)                    │
└──────────────────┬──────────────────────────────────┘
                   │ K3s Cluster Network
┌──────────────────┴──────────────────────────────────┐
│  NUMA Server (Compute + Storage)                    │
│  ├─ VM1: SLM Worker 1 (NUMA Node 0)                 │
│  │   ├─ 17 cores (34 threads)                       │
│  │   ├─ Emulator: 1 core (2 threads) reserved       │
│  │   ├─ SLM inference pods (3 models)               │
│  │   └─ Temporal workers (co-located)               │
│  │                                                   │
│  ├─ VM2: SLM Worker 2 (NUMA Node 1)                 │
│  │   ├─ 17 cores (34 threads)                       │
│  │   ├─ Emulator: 1 core (2 threads) reserved       │
│  │   ├─ SLM inference pods (3 models)               │
│  │   └─ Temporal workers (co-located)               │
│  │                                                   │
│  └─ VM3: MinIO Storage + Data Processing            │
│      ├─ 6 cores (flexible placement)                │
│      ├─ MinIO (8TB "Locness Lake" data gateway)     │
│      ├─ Data processing pods (DuckDB/Polars)        │
│      └─ Temporal workers (prep/aggregation)         │
│                                                      │
│  Hardware:                                           │
│  ├─ CPU: 2× E5-2697v4 (36 cores, 72 threads)        │
│  ├─ RAM: 252GB                                       │
│  ├─ SSD: ~350GB (VM disks + staging)                │
│  ├─ HDD: 8TB (MinIO data lake on VM3)               │
│  └─ Network: 1GbE (192.168.1.20)                    │
└─────────────────────────────────────────────────────┘
```

---

## Design Rationale

### **Why Mac Mini as Master?**

```yaml
Strengths:
✅ M2 is fast (overkill for control plane)
✅ Low power (~15-20W vs 200W for dual-socket)
✅ Already purchased (make it useful!)
✅ Perfect for lightweight services (monitoring, orchestration)
✅ Can run headless (SSH only, saves RAM)

Constraints:
⚠️ Only 8GB RAM (tight but manageable)
⚠️ macOS quirks (sleep, updates, overhead)
⚠️ Single point of failure (acceptable for home lab)
⚠️ K3s requires Linux VM (Multipass/Lima)

Solution:
├─ Run only control plane + lightweight monitoring
├─ NO compute workloads on Mac
├─ NO heavy storage (MinIO on NUMA server instead)
├─ Aggressive resource limits on pods
└─ Let NUMA server handle all compute + storage
```

### **Why 3 VMs (Not 2)?**

```yaml
Original plan: 2 SLM VMs only
New plan: 2 SLM VMs + 1 Storage VM

Reason:
├─ Mac can't efficiently handle 8TB HDD (virtualization overhead)
├─ MinIO benefits from native Linux filesystem (ext4)
├─ Data processing should be near storage (minimize network I/O)
├─ Clean separation: Compute (VM1/VM2) vs Storage (VM3)
└─ ALL 34 threads per VM for SLM (maximum compute)

VM3 Role:
├─ MinIO server (8TB HDD, native Linux)
├─ Data processing pods (DuckDB/Polars)
├─ Temporal workers (daily prep, aggregation)
└─ Uses leftover CPU threads (emulator + extras)

Benefits:
✅ Native disk performance (125 MB/s HDD, no virtualization)
✅ Clean architecture (control, compute, storage separated)
✅ VM1/VM2 stay pure compute (34 vCPUs each for SLM)
✅ Network overhead acceptable (5% of job time)
✅ Simpler than distributed MinIO across VM1/VM2
```

### **Why Temporal Workers on SLM VMs?**

```yaml
Key insight: Temporal workers need local filesystem access!

SLM workflow:
1. Temporal worker fetches input from MinIO
2. Worker writes to local staging disk (/data/local)
3. SLM pod reads from local staging disk
4. SLM pod writes results to local staging disk
5. Temporal worker uploads results to MinIO

Benefits of co-location:
✅ No network overhead (local filesystem)
✅ Fast I/O (SSD on worker VM)
✅ Simpler data flow (no NFS/CSI)
✅ Temporal worker sees SLM output immediately
└─ Perfect for "write-local, sweep-remote" pattern! 🎯
```

---

## VM Specifications

### **VM1: k3s-slm-worker-1 (NUMA Node 0)**

```json
{
  "name": "k3s-slm-worker-1",
  "vcpu_count": 34,
  "memory_mb": 122880,
  "disk_path": "/var/lib/libvirt/images/slm1.qcow2",
  "disk_size_gb": 35,
  "base_image_path": "/var/lib/libvirt/images/ubuntu-22.04-base.qcow2",
  "bridge_network_interface": "br0",
  "tuning": {
    "vcpu_pins": [
      "0","36","1","37","2","38","3","39",
      "4","40","5","41","6","42","7","43",
      "8","44","9","45","10","46","11","47",
      "12","48","13","49","14","50","15","51",
      "16","52"
    ],
    "emulator_cpuset": "17,53",
    "numa_memory": {
      "nodeset": "0",
      "mode": "strict"
    }
  }
}
```

**Resources:**
- Cores: 17 physical (34 threads) - 1 core reserved for emulator
- Memory: 120GB
- NUMA: Node 0 (strict locality)
- Storage: 35GB OS + 100GB staging disk
- Emulator: Thread 17, 53 (dedicated)

**Workloads:**
```yaml
SLM Pods (3 models):
├─ Gemma-7B (9B): 6-10 cores, ~30GB RAM
├─ Mistral-7B (7.3B): 6-10 cores, ~30GB RAM
└─ Gemma-4B (4B): 6-10 cores, ~20GB RAM

Temporal Workers (DaemonSet):
├─ Workflow orchestration: 2 cores (guaranteed), ~4-8GB RAM
├─ Daily data prep (DuckDB/Polars processing)
├─ Pre-job data fetch + staging
├─ Post-job result sweep + upload
└─ Pattern aggregation + distribution

Total per VM:
├─ Guaranteed: 20 cores (2 + 18), 84GB RAM
├─ Burst capacity: 32 cores, 110GB RAM
├─ Headroom: 2 cores, 10GB RAM ✅
└─ Fits comfortably in 34 cores, 120GB! 🎯
```

---

### **VM2: k3s-slm-worker-2 (NUMA Node 1)**

```json
{
  "name": "k3s-slm-worker-2",
  "vcpu_count": 34,
  "memory_mb": 122880,
  "disk_path": "/var/lib/libvirt/images/slm2.qcow2",
  "disk_size_gb": 35,
  "base_image_path": "/var/lib/libvirt/images/ubuntu-22.04-base.qcow2",
  "bridge_network_interface": "br0",
  "tuning": {
    "vcpu_pins": [
      "18","54","19","55","20","56","21","57",
      "22","58","23","59","24","60","25","61",
      "26","62","27","63","28","64","29","65",
      "30","66","31","67","32","68","33","69",
      "34","70"
    ],
    "emulator_cpuset": "35,71",
    "numa_memory": {
      "nodeset": "1",
      "mode": "strict"
    }
  }
}
```

**Resources:**
- Cores: 17 physical (34 threads) - 1 core reserved for emulator
- Memory: 120GB
- NUMA: Node 1 (strict locality)
- Storage: 35GB OS + 100GB staging disk
- Emulator: Thread 35, 71 (dedicated)

**Workloads:**
```yaml
SLM Pods (3 models):
├─ Qwen-7B (7B): 6-10 cores, ~25GB RAM
├─ Gemma-2B (2.5B): 6-10 cores, ~12GB RAM
└─ Phi-3-Mini (3.8B): 6-10 cores, ~18GB RAM

Temporal Workers (DaemonSet):
├─ Workflow orchestration: 2 cores (guaranteed), ~4-8GB RAM
├─ Daily data prep (DuckDB/Polars processing)
├─ Pre-job data fetch + staging
├─ Post-job result sweep + upload
└─ Pattern aggregation + distribution

Total per VM:
├─ Guaranteed: 20 cores (2 + 18), 63GB RAM
├─ Burst capacity: 32 cores, 95GB RAM
├─ Headroom: 2 cores, 25GB RAM ✅
└─ Perfect symmetry with VM1! 🎯
```

---

### **VM3: k3s-minio-storage (Flexible Placement)**

```json
{
  "name": "k3s-minio-storage",
  "vcpu_count": 6,
  "memory_mb": 32768,
  "disk_path": "/var/lib/libvirt/images/minio.qcow2",
  "disk_size_gb": 35,
  "base_image_path": "/var/lib/libvirt/images/ubuntu-22.04-base.qcow2",
  "bridge_network_interface": "br0",
  "tuning": {
    "vcpu_pins": [
      "17","53","35","71","18","54"
    ],
    "emulator_cpuset": null,
    "numa_memory": null
  }
}
```

**Resources:**
- Cores: 6 vCPUs (flexible, uses emulator threads + extras)
- Memory: 32GB
- NUMA: No strict affinity (can span nodes)
- Storage: 35GB OS disk + 8TB HDD (passthrough)
- Emulator: Shared with host (no dedicated pinning)

**Workloads:**
```yaml
MinIO (Single-node):
├─ S3-compatible API: 2 cores, ~500MB-1GB RAM
├─ 8TB HDD (native ext4)
└─ Network I/O bound (not CPU bound)

Data Processing Pods:
├─ DuckDB/Polars: 2-4 cores, ~10-20GB RAM
├─ Daily prep, aggregation tasks
└─ Temporal workers (co-located with data)

Temporal Workers (DaemonSet):
├─ Daily data prep (reads/writes MinIO locally)
├─ Aggregation (reads all results from MinIO)
└─ Pattern merging (local operations)

Total per VM:
├─ MinIO: 2 cores, 1GB
├─ Data processing: 2-4 cores, 10-20GB
├─ Temporal: 1 core, 4GB
├─ Total: ~6 cores, 15-25GB
└─ Headroom: 7-17GB ✅
```

**HDD Passthrough:**
```bash
# 8TB HDD attached as /dev/sdb on host
# Passed through to VM3 as /dev/vdb
# Formatted as ext4 and mounted at /mnt/datalake
```

**Why flexible CPU placement?**
- MinIO is I/O bound (disk + network), not CPU intensive
- Data processing is batch (not latency sensitive)
- Can use leftover CPU cycles from either NUMA node
- No strict NUMA affinity needed

---

## Mac Mini Resource Allocation

### **Memory Budget (8GB Total)**

```yaml
macOS (base system): ~3GB
├─ Kernel: ~1GB
├─ WindowServer: ~300MB (or 0 if headless)
├─ Background services: ~500MB
└─ File cache: ~1.2GB

Lima/Multipass VM (4GB):
├─ Ubuntu guest OS: ~800MB
├─ K3s control plane: ~800MB
│   ├─ API server: ~300MB
│   ├─ Scheduler: ~100MB
│   ├─ Controller manager: ~150MB
│   ├─ Etcd: ~200MB
│   └─ kubelet: ~50MB
├─ Pods on Master:
│   ├─ VictoriaLogs: ~150-200MB
│   ├─ VictoriaMetrics: ~150-200MB
│   ├─ Temporal Server: ~1-1.5GB
│   ├─ Temporal UI: ~150-200MB
│   └─ Promtail: ~64-128MB
└─ Total VM: ~3.5-4GB

Total Mac RAM Usage:
├─ macOS: 3GB
├─ VM: 4GB
└─ Headroom: 1GB ✅

Comfortable! Mac can run headless (disable WindowServer) for more RAM.
```

### **Storage Allocation (VM Only)**

```yaml
Lima/Multipass VM disk (20GB):
├─ Ubuntu OS: ~5GB
├─ K3s + containerd: ~3GB
├─ Pod images: ~5GB
├─ Victoria* data (7-day retention): ~2GB
├─ Temporal DB (30-day history): ~2GB
└─ Logs + temp: ~3GB

Total: ~20GB (fits in VM disk)
No external HDD needed on Mac! ✅
```

---

## Data Flow

### **Phase 1: Daily Data Prep (Morning, Once Per Day)**

```
┌──────────────────────────────────────────────────────┐
│  1. Temporal Server (Mac)                            │
│     └─ Triggers daily prep workflow (06:00 AM)       │
└──────────────────┬───────────────────────────────────┘
                   │
┌──────────────────▼───────────────────────────────────┐
│  2. Temporal Worker (VM1 or VM2)                     │
│     ├─ Fetch from MinIO:                             │
│     │  ├─ s3://lake/market-data/last-7-days/         │
│     │  └─ s3://lake/patterns/last-30-days/           │
│     ├─ Process with DuckDB/Polars (in worker pod)    │
│     ├─ Generate: /data/local/daily_context_<date>.pq │
│     ├─ Size: 2-5GB                                   │
│     └─ Time: 5-10 minutes                            │
└──────────────────┬───────────────────────────────────┘
                   │
┌──────────────────▼───────────────────────────────────┐
│  3. Upload to MinIO (Mac)                            │
│     ├─ Upload: s3://lake/daily-prepared/<date>.pq   │
│     ├─ Cleanup: Delete /data/local/daily_prep_*     │
│     └─ Network: 16-40 seconds (1GbE)                 │
└──────────────────────────────────────────────────────┘

Result: Daily context ready for all jobs today
Cached locally: /data/local/daily_context_<date>.parquet
```

### **Phase 2: Per-Job Workflow (Per Symbol, e.g., BTC)**

```
┌─────────────────────────────────────────────────────────┐
│  Step 1: Pre-Job Data Prep (Temporal Worker)           │
│  ├─ Fetch pre-swept data from MinIO                    │
│  ├─ MOVE s3://lake/pre-swept/<symbol>/ →               │
│  │      s3://lake/daily-archived/<date>/               │
│  ├─ Write: /data/local/input_<job-id>_<symbol>.pq     │
│  └─ Time: 1-2 seconds (atomic operation)               │
└─────────────┬───────────────────────────────────────────┘
              │
┌─────────────▼───────────────────────────────────────────┐
│  Step 2: SLM Inference (6-7 Models in Parallel)        │
│  ├─ Read inputs (concurrent, safe):                    │
│  │  ├─ /data/local/input_<job-id>_<symbol>.pq         │
│  │  ├─ /data/local/daily_context_<date>.pq (cached!)  │
│  │  └─ /data/local/patterns_<date>.pq (cached!)       │
│  ├─ SLM inference (3-4 minutes)                        │
│  └─ Write outputs (unique files):                      │
│     ├─ /data/local/output_<model>_<job-id>.pq         │
│     └─ /data/local/patterns_new_<model>_<job-id>.pq   │
└─────────────┬───────────────────────────────────────────┘
              │
┌─────────────▼───────────────────────────────────────────┐
│  Step 3: Result Sweep (Temporal Workers on VM1 + VM2)  │
│  ├─ Collect: /data/local/output_*_<job-id>.pq          │
│  ├─ Upload: s3://lake/staging/results/<job-id>/        │
│  ├─ Collect: /data/local/patterns_new_*_<job-id>.pq    │
│  ├─ Upload: s3://lake/staging/patterns/<job-id>/       │
│  ├─ Keep local: /data/local/patterns_<date>.pq         │
│  ├─ Cleanup: Delete intermediate files                  │
│  └─ Time: 10-30 seconds per VM                          │
└─────────────┬───────────────────────────────────────────┘
              │
┌─────────────▼───────────────────────────────────────────┐
│  Step 4: Aggregation (Temporal Worker, Either VM)      │
│  ├─ Download all results from staging (6-7 files)      │
│  ├─ Weighted consensus calculation                      │
│  ├─ Upload: s3://lake/gold/predictions/<symbol>_<date>│
│  ├─ Merge patterns: /data/local/patterns_merged.pq     │
│  ├─ Upload: s3://lake/patterns/<date>/unified.pq       │
│  └─ Time: 30-60 seconds                                 │
└─────────────┬───────────────────────────────────────────┘
              │
┌─────────────▼───────────────────────────────────────────┐
│  Step 5: Pattern Distribution (Both VMs)               │
│  ├─ Download: s3://lake/patterns/<date>/unified.pq     │
│  ├─ Cache locally: /data/local/patterns_<date>.pq      │
│  └─ Ready for next job! (cache hit 100%)               │
└─────────────────────────────────────────────────────────┘

Total per job: ~5-6 minutes
Daily capacity: ~80-100 symbols (16 hours × 6 min/job)
```

### **Key Pattern: Co-located Workers + Local Caching**

```yaml
Why co-location matters:

Traditional (BAD):
├─ SLM pod writes to PV (network storage)
├─ Temporal worker on different VM reads from PV
└─ Need NFS/CSI + 2× network overhead!

Co-located (GOOD):
├─ SLM pod + Temporal worker on SAME VM
├─ Both mount /data/local (hostPath, same directory)
├─ SLM writes to /data/local (local SSD, 500 MB/s)
├─ Temporal reads from /data/local (instant!)
├─ Temporal uploads to MinIO (single network hop)
└─ Zero network overhead for hot path! ✅

Caching strategy:
├─ Daily context (2-5GB): Fetched 1× per day
├─ Patterns (50-500MB): Fetched 1× per day
├─ Both cached in /data/local
└─ Reused across ALL jobs same day (100% cache hit!)

Performance gain:
├─ Local read/write: ~500 MB/s (SSD)
├─ Network read/write: ~125 MB/s (1GbE)
├─ Cache hit: No network at all!
└─ 4× faster + zero network for cached data! 🚀
```

### **File Lifecycle on /data/local**

```yaml
Long-lived (Cached Daily):
├─ daily_context_<date>.parquet (2-5GB, 1 day)
└─ patterns_<date>.parquet (50-500MB, 1 day)

Short-lived (Per Job):
├─ input_<job-id>_<symbol>.parquet (50-100MB)
├─ output_<model>_<job-id>.parquet (50-100MB × 6-7)
└─ patterns_new_<model>_<job-id>.parquet (10-50MB)
   ↓ Deleted after upload to MinIO

Disk usage per VM:
├─ Cached: 3-5GB (daily + patterns)
├─ Active job: 1-2GB (inputs + outputs)
├─ Peak: 5-7GB
└─ 100GB staging disk: 93-95GB free ✅

Concurrency safety:
├─ Read-only files: Multiple SLM pods read simultaneously ✅
├─ Write files: Each pod writes unique filename ✅
└─ No file locking needed! ✅
```

---

## Kubernetes Configuration

### **Node Taints and Labels (Whitelist Strategy)**

```yaml
Mac Master:
├─ Role: control-plane
├─ Taint: node-role.kubernetes.io/control-plane:NoSchedule
└─ Labels: role=master, storage=true

VM1 (SLM Worker 1):
├─ Role: worker
├─ Taints: NONE (whitelist approach!)
└─ Labels: role=worker, workload=slm, numa-node=0

VM2 (SLM Worker 2):
├─ Role: worker
├─ Taints: NONE (whitelist approach!)
└─ Labels: role=worker, workload=slm, numa-node=1

Why no taints?
├─ Whitelist via nodeSelector (explicit opt-in)
├─ SLM pods must request workload=slm
├─ Temporal workers request role=worker
├─ Flexible for future workload types
└─ No need to manage tolerations! ✅
```

### **Pod Scheduling Strategy**

```yaml
Master Node Pods (Mac):
  nodeSelector:
    role: master
  tolerations:
  - key: node-role.kubernetes.io/control-plane
    effect: NoSchedule
  
  Pods:
  ├─ MinIO (pinned to master)
  ├─ VictoriaLogs (pinned to master)
  ├─ VictoriaMetrics (pinned to master)
  ├─ Temporal Server (pinned to master)
  └─ Temporal UI (pinned to master)

SLM Worker Pods (VM1, VM2):
  nodeSelector:
    workload: slm          # Must run on SLM nodes
  
  affinity:
    podAntiAffinity:       # Spread across NUMA nodes
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
            - key: app
              operator: In
              values: [slm-inference]
          topologyKey: numa-node
  
  volumes:
  - name: local-staging
    hostPath:
      path: /data/local    # Shared with Temporal worker!
      type: Directory

Temporal Workers (DaemonSet):
  nodeSelector:
    role: worker           # Runs on ALL worker nodes (VM1, VM2)
  
  volumes:
  - name: local-staging
    hostPath:
      path: /data/local    # Same as SLM pods!
      type: DirectoryOrCreate
  
  resources:              # CRITICAL: Prevent CPU starvation
    requests:
      cpu: "2000m"        # Guaranteed 2 cores
      memory: "4Gi"       # Guaranteed 4GB
    limits:
      cpu: "2000m"        # No burst (strict reservation)
      memory: "8Gi"       # Can use up to 8GB
```

### **Resource Guarantees (Prevent CPU Starvation)**

```yaml
Problem:
├─ SLM pods can use all 34 cores during inference
├─ Temporal worker needs CPU to sweep results
└─ Without guarantees: Temporal gets starved! ⚠️

Solution: K8s Resource Requests

Per VM (34 cores, 120GB RAM):

Temporal Worker (Guaranteed QoS):
  requests:
    cpu: "2000m"         # K8s reserves 2 cores
    memory: "4Gi"        # K8s reserves 4GB
  limits:
    cpu: "2000m"         # No burst (strict)
    memory: "8Gi"
  
  Priority: CRITICAL
  └─ Always gets 2 cores, even during SLM inference ✅

SLM Pods (Burstable QoS):
  requests:
    cpu: "6000m"         # Guaranteed 6 cores each
    memory: "20Gi"       # Guaranteed 20GB each
  limits:
    cpu: "10000m"        # Can burst to 10 cores
    memory: "30Gi"       # Can use up to 30GB
  
  Priority: HIGH
  └─ Can burst when Temporal idle, throttled when needed

Example: 3 SLM models per VM:
├─ Temporal: 2 cores (guaranteed)
├─ SLM 1: 6-10 cores (burstable)
├─ SLM 2: 6-10 cores (burstable)
├─ SLM 3: 6-10 cores (burstable)
├─ Total guaranteed: 2 + 18 = 20 cores
├─ Total burst: 2 + 30 = 32 cores
└─ Fits in 34 cores! ✅

How K8s enforces:
├─ Scheduling: Reserves guaranteed resources
├─ Runtime: CPU quota enforcement (cgroups)
├─ Pressure: Throttles burstable pods first
└─ Result: Temporal never starved! 🎯
```

---

## Network Services

### **Exposed Services (NodePort)**

```yaml
Mac Mini (192.168.1.10):
├─ MinIO API: :30900
├─ MinIO Console: :30901
├─ VictoriaLogs UI: :30428
├─ VictoriaMetrics UI: :30828
├─ Temporal UI: :30880
└─ K3s API: :6443

Access from anywhere:
├─ MinIO: http://192.168.1.10:30901
├─ Logs: http://192.168.1.10:30428/select/vmui
├─ Metrics: http://192.168.1.10:30828/vmui
└─ Temporal: http://192.168.1.10:30880
```

### **Internal Services (ClusterIP)**

```yaml
Within cluster:
├─ minio.storage.svc.cluster.local:9000
├─ victorialogs.monitoring.svc.cluster.local:9428
├─ victoriametrics.monitoring.svc.cluster.local:8428
├─ temporal.temporal.svc.cluster.local:7233
└─ All accessible via DNS
```

---

## Performance Characteristics

### **SLM Inference**

```yaml
6-Model Ensemble (25k context):
├─ Processing: 3-4 minutes (parallel)
├─ Memory: ~140GB / 220GB (64%)
├─ CPU: 56 threads utilized
└─ Quality: Excellent (3 large + 3 small)

Throughput:
├─ Sequential: ~15 jobs/hour
├─ Parallel (2 symbols): ~24 jobs/hour
└─ Daily capacity: ~360 jobs (16 hours)
```

### **Data Transfers**

```yaml
Daily prep (VM3 → MinIO):
├─ Size: 2-5GB
├─ Time: 16-40 seconds (1GbE)
└─ Frequency: Once per day

SLM input fetch (MinIO → VM1/VM2):
├─ Size: 50-100MB per worker
├─ Time: 1-2 seconds
└─ Frequency: Per job

SLM result upload (VM1/VM2 → MinIO):
├─ Size: 100-400MB per model
├─ Time: 5-20 seconds
└─ Frequency: Per job (6 models)

Total network per job: ~1-3GB
Time on network: ~10-30 seconds
Network utilization: ~1-2% of job time ✅
```

---

## Failure Modes

### **Mac Mini Failure**

```yaml
Impact:
├─ K3s control plane unavailable
├─ No new pod scheduling
├─ MinIO unavailable (data lake offline)
├─ Temporal server unavailable
└─ Workers keep running (existing pods OK)

Recovery:
├─ Reboot Mac Mini
├─ K3s auto-starts
├─ Pods restart automatically
└─ Time: ~2-5 minutes

Prevention:
├─ Disable macOS sleep
├─ Disable auto-updates
├─ Monitor with external ping
```

### **Worker VM Failure**

```yaml
VM1 or VM2 fails:
├─ Impact: 3 SLM models offline + Temporal worker offline
├─ Cluster still has 3 models running on other VM
├─ Jobs take 2× longer (50% capacity loss)
├─ Daily prep can still run on surviving VM
└─ Pattern distribution only to surviving VM

Recovery:
├─ Restart VM via libvirt (virsh start <vm>)
├─ K3s agent reconnects automatically
├─ Pods rescheduled (SLM inference resumes)
├─ Temporal worker re-downloads cached files
└─ Time: ~2-3 minutes

Mitigation:
├─ Monitor VM health (systemd watchdog)
├─ Automate VM restart on failure
├─ Temporal workflows auto-retry on worker failure
└─ MinIO staging acts as buffer (no data loss)

Both VMs fail:
├─ Impact: NO SLM inference capacity
├─ Mac continues to run (control plane OK)
├─ Jobs queued until VMs recover
└─ Manual intervention required
```

---

## Monitoring Strategy

### **VictoriaLogs Queries**

```
# All SLM pod logs
_stream:{namespace="slm"}

# Temporal worker logs
_stream:{pod=~"temporal-worker.*"}

# Errors across cluster
error OR failed OR exception

# Recent job completions
_stream:{namespace="slm"} | "job completed"
```

### **VictoriaMetrics Queries**

```
# Node CPU usage
100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)

# Pod memory usage
container_memory_usage_bytes{namespace="slm"}

# SLM inference time
histogram_quantile(0.95, rate(slm_inference_duration_seconds_bucket[5m]))

# Temporal job success rate
rate(temporal_workflow_completed_total{status="success"}[5m])
```

---

## Cost Analysis

### **Power Consumption**

```yaml
Mac Mini M2:
├─ Idle: 10-15W
├─ Load: 25-35W
└─ Average: ~20W

Dual E5-2697v4:
├─ Idle: 150-200W
├─ Load: 350-450W
└─ Average: ~250W

Total: ~270W average
Monthly: ~195 kWh
Cost: ~$20-30/month (electricity)

vs Single NUMA server: $40-60/month
Savings: $20-30/month ✅
```

### **Operational Costs**

```yaml
Monthly:
├─ Electricity: ~$25
├─ Internet: $0 (existing)
├─ LLM API calls: $10-50 (variable)
└─ Total: ~$35-75/month

Per job:
├─ Compute: ~$0.01 (electricity)
├─ LLM API: ~$0.05-0.10
└─ Total: ~$0.06-0.11 per analysis

Very cost-effective for home lab! 💰
```

---

## Upgrade Path

### **When Mac Becomes Bottleneck**

```yaml
Option 1: Add RAM to Mac
├─ Upgrade to 16GB or 24GB
├─ Cost: N/A (M2 RAM is soldered 💀)
└─ Verdict: Not possible! ❌

Option 2: Replace Mac with Mini PC
├─ Intel N100 / AMD 5700U
├─ 16-32GB RAM
├─ Cost: $200-400
└─ Verdict: Good future option ✅

Option 3: Move master to VM on NUMA server
├─ Create 4th VM for K3s master
├─ Use Mac as pure storage (just MinIO)
├─ Cost: $0 (use existing hardware)
└─ Verdict: Best option! ⭐
```

### **When Need More Compute**

```yaml
Option 1: Add more NUMA nodes
├─ More E5-2697v4 or newer CPUs
├─ More worker VMs
└─ Scales linearly

Option 2: Add second NUMA server
├─ Another dual-socket machine
├─ Join to same K3s cluster
└─ Double capacity

Option 3: Cloud burst
├─ Use cloud for peak loads
├─ Expensive but flexible
└─ Hybrid approach
```

---

## References

- Original NUMA design: `docs/architecture/numa-design.md`
- Distributed architecture: `docs/architecture/distributed-architecture.md`
- VM specifications: `docs/architecture/vm-specifications.md`
- Finance use case: `docs/use-cases/finance-ensemble.md`
- Deployment manifests: `k8s/`

---

## Summary

**Architecture Type:** Hybrid K3s cluster (Mac Mini + NUMA server)

**Key Characteristics:**
- ✅ Cost-effective (repurpose Mac Mini failure 💀)
- ✅ Power-efficient (Mac for control plane)
- ✅ NUMA-aware (strict locality for SLM VMs)
- ✅ Co-located workers (Temporal + SLM on same node)
- ✅ Centralized storage (8TB "Locness Lake" 🏴󠁧󠁢󠁳󠁣󠁴󠁿)
- ✅ Built-in monitoring (Victoria stack with UIs)
- ✅ Production-ready (for home lab scale)

**Perfect for:** Finance analysis, SLM ensembles, data processing workloads requiring fast local I/O and centralized storage!

**Life lesson:** Even purchasing mistakes can be turned into useful infrastructure! 😂💪

