# Distributed Architecture

## Overview

This document describes the **distributed two-machine architecture** - an evolution of the original single-machine NUMA design. This architecture separates compute-intensive workloads (SLM inference) from data-intensive workloads (data lake and processing).

---

## Architecture Evolution

### **Original Design: Single NUMA Machine**
- All workloads on one dual-socket machine
- 4 VMs: SLM workers + data node + task node
- Storage co-located with compute

### **Distributed Design: Two-Machine Cluster**
- **Machine 1 (NUMA)**: Pure compute workers for SLM inference
- **Machine 2 (UMA)**: K3s control plane + data lake + data processing
- Clear separation of concerns
- Connected via 1GbE network

---

## Machine Overview

### **Machine 1: NUMA Worker Cluster**

**Hardware:**
- Dual Intel Xeon E5-2696v4 (2×18 cores, 36 cores total)
- 252GB RAM (2×126GB)
- 350GB SSD
- 1GbE network

**Role:** Pure compute - SLM inference workloads

**VMs:**
- 2 large worker nodes (1 per NUMA socket)
- Each VM: 16 cores (32 threads), 118GB RAM

---

### **Machine 2: UMA Data Server**

**Hardware:**
- Intel Xeon E3-1270v6 (4 cores, single socket)
- 64GB RAM
- SSD (OS) + 8TB HDD (data lake)
- 1GbE network

**Role:** 
- K3s control plane (master)
- Data lake storage (8TB HDD)
- Data processing and orchestration
- Temporal workflow engine

---

## Network Architecture

```
┌─────────────────────────────────────────────────────┐
│   Machine 1: NUMA Worker Cluster                    │
│   (Dual E5-2696v4, 252GB RAM, 350GB SSD)           │
│                                                      │
│   ┌──────────────────┐  ┌──────────────────┐       │
│   │  VM1 (Worker 1)  │  │  VM2 (Worker 2)  │       │
│   │  Node 0          │  │  Node 1          │       │
│   │  16c/32t, 118GB  │  │  16c/32t, 118GB  │       │
│   │  35GB + 100GB    │  │  35GB + 100GB    │       │
│   └────────┬─────────┘  └────────┬─────────┘       │
│            │                     │                  │
│            └──────────┬──────────┘                  │
│                       │ Bridge (br0)                │
└───────────────────────┼─────────────────────────────┘
                        │
                        │ 1GbE Network
                        │ (~125 MB/s)
                        │
┌───────────────────────┼─────────────────────────────┐
│                       │                              │
│   Machine 2: UMA Data Server                        │
│   (E3-1270v6, 64GB RAM, 8TB HDD)                    │
│                                                      │
│   ┌──────────────────────────────────────┐          │
│   │  K3s Control Plane                   │          │
│   │  - API Server                        │          │
│   │  - Scheduler                         │          │
│   │  - Controller Manager                │          │
│   └──────────────────────────────────────┘          │
│                                                      │
│   ┌──────────────────────────────────────┐          │
│   │  Temporal Orchestrator               │          │
│   │  - Workflow engine                   │          │
│   │  - Activity workers                  │          │
│   └──────────────────────────────────────┘          │
│                                                      │
│   ┌──────────────────────────────────────┐          │
│   │  Data Services                       │          │
│   │  - Daily data preparation            │          │
│   │  - Pattern memory server             │          │
│   │  - Result aggregation                │          │
│   └──────────────────────────────────────┘          │
│                                                      │
│   💾 Data Lake (8TB HDD)                            │
│   └─ /data/lake                                     │
│       ├─ clean/bronze/                              │
│       ├─ clean/silver/                              │
│       ├─ clean/gold/                                │
│       ├─ patterns/                                  │
│       ├─ archive/                                   │
│       └─ raw/                                       │
└─────────────────────────────────────────────────────┘
```

---

## Data Flow Architecture

### **Phase 1: Start of Day - Data Preparation**

```
Data Server                    1GbE Network                Worker Cluster
┌──────────────┐                                          ┌──────────────┐
│ Prepare Data │                                          │              │
│ - Query lake │                                          │              │
│ - Transform  │                                          │              │
│ - Compress   │                                          │              │
└──────┬───────┘                                          │              │
       │                                                  │              │
       │ Push daily inputs (1-5GB per worker)           │              │
       │ + patterns (if shared, ~1GB)                    │              │
       ├──────────────────────────────────────────────>  │ Stage to     │
       │                                                  │ /data/local/ │
       │ Transfer time: ~1-2 minutes @ 125MB/s          │ (100GB SSD)  │
       │                                                  │              │
       └──────────────────────────────────────────────>  │              │
                                                          └──────────────┘
```

### **Phase 2: During the Day - Inference**

```
Worker Cluster (Local Operations Only)
┌─────────────────────────────────────────────┐
│                                             │
│  1. Read input from /data/local/inputs     │
│     (Local SSD, ~500 MB/s)                 │
│            ↓                                │
│  2. [Optional] Fetch pattern memory        │
│     (From data server, ~0.8s for 100MB)    │
│            ↓                                │
│  3. Run SLM inference                      │
│     (5-10 minutes, CPU-intensive)          │
│            ↓                                │
│  4. Write results to /data/local/outputs   │
│     (Local SSD, ~500 MB/s)                 │
│            ↓                                │
│  5. Signal Temporal: "Job Complete"        │
│                                             │
└─────────────────────────────────────────────┘

Network Usage During Inference: ~0 bytes
(Except optional pattern fetch: 10-100MB)
```

### **Phase 3: Job Completion - Result Transfer**

```
Worker Cluster                1GbE Network                Data Server
┌──────────────┐                                         ┌──────────────┐
│ Job Complete │                                         │ Temporal     │
│   Signal     ├────────────────────────────────────────>│ Receives     │
└──────────────┘                                         │ Signal       │
                                                         └──────┬───────┘
                                                                │
┌──────────────┐                                         ┌─────┴────────┐
│ Results in   │   Orchestrator pulls results            │ Trigger      │
│ /data/local/ │   (100MB-4GB total, all SLMs)          │ Transfer     │
│ /outputs/    │  <─────────────────────────────────────┤ Activity     │
└──────────────┘                                         └──────┬───────┘
                                                                │
       Transfer time: 8-32 seconds @ 125MB/s                   │
       (Typical: ~16 seconds for 2GB)                          │
                                                         ┌──────┴───────┐
                                                         │ Write to     │
                                                         │ Lake         │
                                                         │ /data/lake/  │
                                                         │ clean/silver/│
                                                         └──────┬───────┘
                                                                │
┌──────────────┐                                         ┌─────┴────────┐
│ Cleanup      │   Signal: "Transfer Complete"           │ Aggregate    │
│ Staging      │  <─────────────────────────────────────┤ Results      │
│ Delete files │                                         │ Write to gold│
└──────────────┘                                         └──────────────┘
```

---

## Worker VM Specifications

### **VM1: k3s-worker-1 (NUMA Node 0)**

```json
{
  "name": "k3s-worker-1",
  "vcpu_count": 32,
  "memory_mb": 120832,
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
  }
}
```

**Storage:**
- OS disk: 35GB (`/dev/vda`)
- Staging disk: 100GB (`/dev/vdb` → `/data/local`)

**Storage Layout:**
```
/dev/vda (35GB): OS
└─ /: System files, binaries, logs

/dev/vdb (100GB): Staging
├─ /data/local/inputs: Daily input data (1-5GB)
│   └─ daily_inputs_2025-11-10.parquet
│
├─ /data/local/outputs: Job results (cleaned after transfer)
│   ├─ slm_1_job_123.parquet (~100MB-2GB)
│   ├─ slm_2_job_123.parquet
│   └─ ... (deleted after successful transfer)
│
└─ /data/local/cache: Pattern memory (optional)
    └─ patterns_*.pkl (~10-100MB, LRU cache)

Typical usage: 5-15GB / 100GB = 5-15%
Peak usage: ~20GB / 100GB = 20%
Headroom: 80GB+ for spikes ✅
```

---

### **VM2: k3s-worker-2 (NUMA Node 1)**

```json
{
  "name": "k3s-worker-2",
  "vcpu_count": 32,
  "memory_mb": 120832,
  "tuning": {
    "vcpu_pins": [
      "18", "54", "19", "55", "20", "56", "21", "57",
      "22", "58", "23", "59", "24", "60", "25", "61",
      "26", "62", "27", "63", "28", "64", "29", "65",
      "30", "66", "31", "67", "32", "68", "33", "69"
    ],
    "emulator_cpuset": "34-35,70-71",
    "numa_memory": {
      "nodeset": "1",
      "mode": "strict"
    }
  }
}
```

**Storage:** Same as VM1

---

## Data Server Configuration

### **K3s Control Plane**

```bash
# Install K3s in server mode
curl -sfL https://get.k3s.io | sh -s - server \
  --disable traefik \
  --node-name data-server

# Get join token for workers
sudo cat /var/lib/rancher/k3s/server/node-token
```

### **Storage Layout**

```
/dev/sda (SSD): OS + processing
├─ /: System files
├─ /var/lib/rancher/k3s: K3s data
├─ /opt/temporal: Temporal server
└─ /tmp: Temporary processing

/dev/sdb (8TB HDD): Data lake
└─ /data/lake:
    ├─ clean/
    │   ├─ bronze/: Raw, minimally processed
    │   ├─ silver/: Cleaned, validated
    │   └─ gold/: Business-ready, aggregated
    │
    ├─ patterns/: Pattern memory storage
    │   └─ *.pkl (served to workers on-demand)
    │
    ├─ archive/: Historical data, backups
    │
    └─ raw/: Unprocessed ingestion data
```

---

## Network Performance

### **Bandwidth Analysis (1GbE = ~125 MB/s)**

| Operation | Size | Time | Frequency |
|-----------|------|------|-----------|
| **Daily data prep** | 10GB (2 workers) | ~80s | Once/day |
| **Pattern fetch** | 100MB | ~0.8s | Per job (optional) |
| **Result transfer** | 4GB max (all SLMs) | ~32s | Per job |
| **Typical job result** | 2GB | ~16s | Per job |

**Daily network usage example (20 jobs/day):**
- Prep: 10GB
- Results: 20 × 2GB = 40GB
- Total: 50GB/day
- Active transfer time: ~10 minutes/day
- **Network utilization: <0.5%** ✅

---

## Why This Architecture?

### **Advantages Over Single-Machine Design**

#### **1. Data Locality for Processing**
```
Before: Data processing competed with SLM inference for CPU
After:  Data processing runs locally with data lake (zero remote I/O)
```

#### **2. Dedicated Compute for SLMs**
```
Before: 4 VMs sharing 36 cores (complex allocation)
After:  2 VMs, each owns entire NUMA socket (clean isolation)
```

#### **3. Network Efficiency**
```
Before: Cross-VM data transfers (internal bottleneck)
After:  Only start-of-day and end-of-job transfers (batched)
```

#### **4. Simplified Worker Topology**
```
Before: Mixed workloads (SLM + data + tasks)
After:  Workers = pure compute (homogeneous)
```

#### **5. Better Resource Utilization**
```
NUMA machine: Optimized for CPU-intensive workloads (SLM inference)
UMA machine:  Optimized for I/O-intensive workloads (data processing)
```

---

## Performance Characteristics

### **Inference Phase (Hot Path)**
- **Latency**: Sub-millisecond (local SSD reads/writes)
- **Network**: Zero (all data local to worker)
- **Bottleneck**: CPU (inference computation)

### **Transfer Phase (Cold Path)**
- **Throughput**: 125 MB/s (1GbE)
- **Typical time**: 16 seconds (2GB)
- **Worst case**: 32 seconds (4GB)
- **Overhead**: <1% of total job time ✅

### **Job Timeline (Example)**
```
00:00 - Job starts
00:00 - Fetch pattern (optional): 0.8s
00:00 - Load input from SSD: 0.1s
00:01 - Inference: 5 minutes
05:01 - Write output to SSD: 0.5s
05:02 - Signal Temporal
05:02 - Transfer results: 16s (2GB typical)
05:18 - Job complete

Total: 5m 18s
Inference: 5m 0s
Overhead: 18s (6% of total) ✅
```

---

## Failure Modes & Recovery

### **Worker Node Failure**
```
Impact: 
- Jobs on failed worker are lost
- Other worker continues operating
- Data server unaffected

Recovery:
1. SSH to hypervisor
2. Restart failed VM
3. Temporal retries failed jobs
4. Resume operations
```

### **Data Server Failure**
```
Impact:
- K3s control plane unavailable (no new scheduling)
- Temporal workflows paused
- Workers can't transfer results (staging fills up)
- Pattern fetches fail (if not cached)

Recovery:
1. SSH to data server
2. Restart services (K3s, Temporal)
3. Workers retry transfers automatically
4. Resume operations

Acceptable for home lab use (manual recovery OK)
```

### **Network Failure**
```
Impact:
- Workers can't transfer results
- Staging disk accumulates data
- Pattern fetches fail (if not cached)

Recovery:
- Workers continue inference (data local)
- Results queue in staging (100GB buffer)
- Transfers resume when network restored
```

---

## Monitoring & Observability

### **Key Metrics to Monitor**

**Workers:**
```bash
# Staging disk usage
df -h /data/local
# Alert if > 70%

# CPU usage
top -H
# Should show pinned cores at ~100% during inference

# Network throughput
iftop -i eth0
# Should spike during transfers, idle during inference
```

**Data Server:**
```bash
# Lake disk usage
df -h /data/lake
# Alert if > 90%

# Transfer queue
curl http://localhost:8080/transfer-status
# Monitor pending transfers

# Temporal workflows
tctl workflow list
# Check for stuck workflows
```

---

## Future Expansion

### **Adding More Worker Machines**

Easy to scale horizontally:

```
1. Provision new NUMA machine
2. Create 2 worker VMs (same config)
3. Join to K3s cluster
4. Daily prep distributes data to all workers
5. Temporal schedules jobs across all workers
```

**Benefits:**
- Linear scaling (2 machines = 2× capacity)
- No architectural changes needed
- Same data flow pattern

### **Upgrading Network to 10GbE**

If needed in the future:

```
Current (1GbE):
- 4GB transfer: 32 seconds
- Typical 2GB: 16 seconds

With 10GbE:
- 4GB transfer: 3.2 seconds
- Typical 2GB: 1.6 seconds

Benefit: Negligible for current workload (<1% improvement)
Recommendation: Not worth the cost
```

---

## References

- Original single-machine design: `docs/architecture/numa-design.md`
- VM specifications: `docs/architecture/vm-specifications.md`
- Storage design: `docs/architecture/storage-design.md`
- Implementation guide: `docs/guides/implementation.md`

---

## Summary

**Architecture Type:** Distributed two-machine cluster

**Key Characteristics:**
- ✅ Clean separation: compute vs. storage
- ✅ Data locality: processing local to lake
- ✅ Network efficiency: batched transfers
- ✅ Simple topology: 2 homogeneous workers
- ✅ Appropriate for 1GbE network
- ✅ Manual recovery acceptable (home lab)

**Perfect for:**
- CPU-intensive SLM workloads
- Moderate data transfer volumes (< 100GB/day)
- Home lab environments
- Development and experimentation

**Not suitable for:**
- Production systems requiring HA
- Ultra-low latency requirements (< 1s end-to-end)
- Massive data volumes (> 1TB/day transfers)

