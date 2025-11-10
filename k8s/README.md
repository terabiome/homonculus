# Kubernetes Manifests for Mac + NUMA Cluster

This directory contains Kubernetes manifests for deploying the complete stack on a hybrid Mac Mini + NUMA server K3s cluster.

## Directory Structure

```
k8s/
├── storage/                    # Storage layer (MinIO)
│   ├── 00-namespace.yaml
│   └── 01-minio.yaml
├── monitoring/                 # Observability stack
│   ├── 00-namespace.yaml
│   ├── 01-victorialogs.yaml
│   ├── 02-victoriametrics.yaml
│   ├── 03-promtail.yaml
│   └── 04-vmagent.yaml
├── temporal/                   # Workflow orchestration
│   ├── 00-namespace.yaml
│   ├── 01-temporal-server.yaml
│   ├── 02-temporal-ui.yaml
│   └── README.md (this file)
└── workers/                    # Temporal workers (co-located with workloads)
    └── 01-temporal-worker-daemonset.yaml
```

## Prerequisites

### 1. Mac Mini Setup

**Requirements:**
- macOS with Docker/OrbStack installed
- K3s server installed
- 8TB HDD mounted at `/Volumes/DataLake`

**Create required directories:**
```bash
# On Mac Mini
sudo mkdir -p /Volumes/DataLake/{minio,victoria-logs,victoria-metrics,temporal}
sudo chown -R $(whoami) /Volumes/DataLake
```

**Install K3s (server mode):**
```bash
curl -sfL https://get.k3s.io | sh -s - server \
  --disable traefik \
  --disable servicelb \
  --write-kubeconfig-mode 644 \
  --node-name mac-master \
  --node-label role=master

# Get join token
sudo cat /var/lib/rancher/k3s/server/node-token
```

### 2. NUMA Server VM Setup

**On each Linux VM (VM1, VM2):**

```bash
# Install K3s (agent mode)
export K3S_URL="https://192.168.1.10:6443"
export K3S_TOKEN="<your-token-from-mac>"

curl -sfL https://get.k3s.io | sh -s - agent \
  --node-name $(hostname) \
  --node-label role=worker

# Create local staging directory
sudo mkdir -p /data/local
sudo chmod 777 /data/local
```

**Label nodes (whitelist strategy, no taints):**
```bash
# On Mac (kubectl configured)

# Label VM1 (SLM worker 1)
kubectl label node k3s-slm-worker-1 workload=slm numa-node=0

# Label VM2 (SLM worker 2)
kubectl label node k3s-slm-worker-2 workload=slm numa-node=1

# Verify labels
kubectl get nodes --show-labels
```

## Deployment Order

Deploy in the following order to ensure dependencies are met:

### 1. Storage Layer
```bash
# Deploy MinIO (data lake)
kubectl apply -f storage/00-namespace.yaml
kubectl apply -f storage/01-minio.yaml

# Wait for MinIO to be ready
kubectl wait --for=condition=ready pod -l app=minio -n storage --timeout=300s

# Verify MinIO is accessible
curl http://192.168.1.10:30901  # MinIO Console
```

### 2. Monitoring Stack
```bash
# Deploy VictoriaLogs and VictoriaMetrics
kubectl apply -f monitoring/00-namespace.yaml
kubectl apply -f monitoring/01-victorialogs.yaml
kubectl apply -f monitoring/02-victoriametrics.yaml

# Wait for Victoria services
kubectl wait --for=condition=ready pod -l app=victorialogs -n monitoring --timeout=300s
kubectl wait --for=condition=ready pod -l app=victoriametrics -n monitoring --timeout=300s

# Deploy collectors (Promtail, vmagent)
kubectl apply -f monitoring/03-promtail.yaml
kubectl apply -f monitoring/04-vmagent.yaml

# Verify monitoring
curl http://192.168.1.10:30428/select/vmui  # VictoriaLogs UI
curl http://192.168.1.10:30828/vmui         # VictoriaMetrics UI
```

### 3. Temporal Orchestration
```bash
# Deploy Temporal server and UI
kubectl apply -f temporal/00-namespace.yaml
kubectl apply -f temporal/01-temporal-server.yaml
kubectl apply -f temporal/02-temporal-ui.yaml

# Wait for Temporal
kubectl wait --for=condition=ready pod -l app=temporal -n temporal --timeout=300s

# Verify Temporal UI
curl http://192.168.1.10:30880  # Temporal UI
```

### 4. Temporal Workers
```bash
# IMPORTANT: Replace image in workers/01-temporal-worker-daemonset.yaml first!
# Then deploy:
kubectl apply -f workers/01-temporal-worker-daemonset.yaml

# Verify workers are running on all worker nodes
kubectl get pods -n temporal -l app=temporal-worker -o wide
```

## Accessing Services

| Service | URL | Purpose |
|---------|-----|---------|
| **MinIO Console** | http://192.168.1.10:30901 | Data lake management |
| **MinIO API** | http://192.168.1.10:30900 | S3-compatible API |
| **VictoriaLogs UI** | http://192.168.1.10:30428/select/vmui | Log search and analysis |
| **VictoriaMetrics UI** | http://192.168.1.10:30828/vmui | Metrics visualization |
| **Temporal UI** | http://192.168.1.10:30880 | Workflow monitoring |

## Configuration

### Update MinIO Password

Edit `storage/01-minio.yaml`:
```yaml
stringData:
  password: "your-secure-password-here"  # Change this!
```

Then update:
```bash
kubectl delete secret minio-secret -n storage
kubectl apply -f storage/01-minio.yaml
kubectl rollout restart deployment/minio -n storage
```

### Adjust Resource Limits

If Mac Mini struggles with memory:

1. **Reduce VictoriaLogs memory:**
   ```yaml
   # In monitoring/01-victorialogs.yaml
   limits:
     memory: "128Mi"  # Reduce from 256Mi
   ```

2. **Reduce VictoriaMetrics memory:**
   ```yaml
   # In monitoring/02-victoriametrics.yaml
   limits:
     memory: "128Mi"  # Reduce from 256Mi
   ```

3. **Reduce Temporal memory:**
   ```yaml
   # In temporal/01-temporal-server.yaml
   limits:
     memory: "1Gi"  # Reduce from 1536Mi
   ```

### Configure Temporal Workers

Edit `workers/01-temporal-worker-daemonset.yaml`:

1. **Replace worker image:**
   ```yaml
   image: your-registry/temporal-worker:latest
   ```

2. **Update MinIO credentials (if changed):**
   ```yaml
   stringData:
     access-key: "admin"
     secret-key: "your-minio-password"
   ```

3. **Adjust resource limits per worker node:**
   ```yaml
   resources:
     limits:
       memory: "4Gi"   # Adjust based on VM RAM
       cpu: "2000m"    # Adjust based on VM cores
   ```

## Monitoring

### Check Pod Status
```bash
# All pods
kubectl get pods -A -o wide

# Pods on Mac master
kubectl get pods -A -o wide | grep mac-master

# Pods on worker nodes
kubectl get pods -A -o wide | grep worker
```

### Check Logs
```bash
# MinIO logs
kubectl logs -n storage deployment/minio

# VictoriaLogs logs
kubectl logs -n monitoring deployment/victorialogs

# Temporal logs
kubectl logs -n temporal deployment/temporal

# Temporal worker logs (specific node)
kubectl logs -n temporal daemonset/temporal-worker -c temporal-worker --tail=100
```

### Check Resource Usage
```bash
# Node resource usage
kubectl top nodes

# Pod resource usage
kubectl top pods -A

# Specific namespace
kubectl top pods -n monitoring
```

### Query Logs (VictoriaLogs)

Access http://192.168.1.10:30428/select/vmui

Example queries:
```
# All SLM pod logs
_stream:{namespace="slm"}

# Temporal worker logs
_stream:{app="temporal-worker"}

# Errors across cluster
error OR failed
```

### Query Metrics (VictoriaMetrics)

Access http://192.168.1.10:30828/vmui

Example queries:
```
# Node CPU usage
100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)

# Pod memory usage
container_memory_usage_bytes{namespace!=""}

# MinIO metrics
minio_cluster_disk_total_bytes
```

## Troubleshooting

### Pod Stuck in Pending

```bash
# Check events
kubectl describe pod <pod-name> -n <namespace>

# Common causes:
# 1. Node selector doesn't match any node
# 2. PersistentVolume not created
# 3. Resource limits too high
```

### PersistentVolume Not Binding

```bash
# Check PV and PVC status
kubectl get pv
kubectl get pvc -A

# Verify paths exist on Mac
ls -la /Volumes/DataLake/

# Check node affinity matches
kubectl get nodes --show-labels | grep mac-master
```

### Mac Mini Out of Memory

```bash
# Check Mac memory
vm_stat

# Reduce pod memory limits (see Configuration section)
# Or move services to Linux VMs
```

### Service Not Accessible

```bash
# Check service and endpoints
kubectl get svc -A
kubectl get endpoints -A

# Test from within cluster
kubectl run -it --rm debug --image=busybox --restart=Never -- sh
# Inside pod:
wget -O- http://minio.storage.svc.cluster.local:9000
```

## Backup and Restore

### Backup Critical Data

```bash
# On Mac Mini

# Backup MinIO data
tar -czf minio-backup-$(date +%Y%m%d).tar.gz /Volumes/DataLake/minio

# Backup Temporal data
tar -czf temporal-backup-$(date +%Y%m%d).tar.gz /Volumes/DataLake/temporal

# Backup Victoria data (optional, can regenerate)
tar -czf victoria-backup-$(date +%Y%m%d).tar.gz /Volumes/DataLake/victoria-*
```

### Restore from Backup

```bash
# Stop services
kubectl scale deployment/minio -n storage --replicas=0
kubectl scale deployment/temporal -n temporal --replicas=0

# Restore data
tar -xzf minio-backup-YYYYMMDD.tar.gz -C /
tar -xzf temporal-backup-YYYYMMDD.tar.gz -C /

# Restart services
kubectl scale deployment/minio -n storage --replicas=1
kubectl scale deployment/temporal -n temporal --replicas=1
```

## Uninstall

To remove all services:

```bash
# Delete in reverse order
kubectl delete -f workers/
kubectl delete -f temporal/
kubectl delete -f monitoring/
kubectl delete -f storage/

# Delete PVs (if needed)
kubectl delete pv minio-pv victorialogs-pv victoriametrics-pv temporal-pv

# On Mac, optionally remove data
rm -rf /Volumes/DataLake/*
```

## References

- Architecture: `docs/architecture/mac-numa-k3s-architecture.md`
- VM Definitions: `definitions/mac-numa-cluster/`
- Deployment Guide: `docs/guides/mac-numa-deployment.md`


