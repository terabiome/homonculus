# NUMA-Aware K8s Cluster Documentation

## Overview

This documentation describes high-performance Kubernetes cluster architectures designed for running Small Language Model (SLM) inference workloads with data processing.

**Three deployment options:**
- **Mac + NUMA Cluster:** Hybrid setup with Mac Mini as control plane + NUMA workers (cost-effective)
- **Single Machine:** All workloads on one NUMA-aware machine (original design)
- **Distributed:** Compute workers (NUMA) + data server (UMA) connected via network (if you have multiple servers)

## Documentation Structure

### Architecture Documentation

1. **[Mac + NUMA K3s Architecture](architecture/mac-numa-k3s-architecture.md)** (`docs/architecture/mac-numa-k3s-architecture.md`) ⭐ **RECOMMENDED**
   - Hybrid Mac Mini (control plane) + NUMA server (workers)
   - 2-VM compute cluster (pure SLM workers)
   - Co-located Temporal workers
   - VictoriaLogs/VictoriaMetrics monitoring
   - Cost-effective for home labs

2. **[NUMA Design](architecture/numa-design.md)** (`docs/architecture/numa-design.md`)
   - Single-machine architecture overview
   - NUMA-aware data flow
   - Design principles and trade-offs
   - Expected performance metrics

3. **[Distributed Architecture](architecture/distributed-architecture.md)** (`docs/architecture/distributed-architecture.md`)
   - Two-machine distributed design
   - Compute/storage separation
   - Network data flow
   - Worker cluster + data server architecture

4. **[VM Specifications](architecture/vm-specifications.md)** (`docs/architecture/vm-specifications.md`)
   - Detailed specifications for each VM
   - CPU pinning configurations
   - Memory allocation
   - Performance tuning parameters
   - Monitoring and troubleshooting

5. **[Storage Design](architecture/storage-design.md)** (`docs/architecture/storage-design.md`)
   - Hot/cold storage tiering strategy
   - Data placement decisions
   - Compression settings
   - Data lake zones and schemas
   - Storage performance monitoring

### Implementation Guides

6. **[Mac + NUMA Deployment Guide](guides/mac-numa-deployment.md)** (`docs/guides/mac-numa-deployment.md`) ⭐ **START HERE**
   - Complete step-by-step deployment for Mac + NUMA cluster
   - Mac Mini setup and configuration
   - NUMA server VM creation
   - K3s cluster setup
   - Service deployment (MinIO, Victoria*, Temporal)
   - Verification and troubleshooting

7. **[Implementation Guide](guides/implementation.md)** (`docs/guides/implementation.md`)
   - Step-by-step deployment instructions (single machine)
   - System verification
   - VM creation and configuration
   - Storage setup
   - K8s cluster setup
   - Performance verification

8. **[Distributed Workflow Guide](guides/distributed-workflow.md)** (`docs/guides/distributed-workflow.md`)
   - End-to-end workflow for distributed architecture
   - Daily data preparation
   - Job execution with Temporal
   - Result aggregation
   - Monitoring and troubleshooting

### Example Configurations

9. **[Mac + NUMA VM Definitions](../definitions/mac-numa-cluster/)** ⭐ **NEW**
   - VM1: SLM Worker 1 (18 cores, 120GB, NUMA node 0)
   - VM2: SLM Worker 2 (18 cores, 120GB, NUMA node 1)

10. **[VM Cluster Configuration](../examples/definitions/virtualmachine/vm.cluster-numa.json.example)**
   - Complete example configuration for all 4 VMs (single machine)
   - CPU pinning settings
   - NUMA memory bindings
   - Emulator CPU allocation

11. **[Cluster with Tuning](../examples/definitions/virtualmachine/vm-cluster.tuning.json.example)**
   - General-purpose example showing tuning options
   - Multiple NUMA memory modes (strict/preferred)
   - Emulator pinning examples

### Use Case Documentation

12. **[Finance Data Analysis with SLM Ensemble](use-cases/finance-ensemble.md)** (`docs/use-cases/finance-ensemble.md`)
   - Heterogeneous SLM ensemble (3 large + 4 small models)
   - Financial market analysis with 30-60 minute decision windows
   - Weighted consensus and confidence scoring
   - Risk management thresholds
   - Production deployment configuration

## Quick Start

### Choose Your Architecture

**Option 1: Mac + NUMA Cluster (Recommended for Home Lab)** ⭐
- ✅ Cost-effective (repurpose Mac Mini)
- ✅ Clean separation (control vs compute)
- ✅ Power-efficient (Mac for lightweight services)
- ✅ 2-VM setup (simple, symmetric, powerful)
- ✅ 68 vCPUs total (34 per VM, 4 threads reserved for emulators)
- ✅ Co-located Temporal workers (local filesystem access)
- ✅ Resource guarantees prevent CPU starvation
- ⚠️ Mac has only 8GB RAM (tight but works)
- 📚 Read: [Mac + NUMA Architecture](architecture/mac-numa-k3s-architecture.md) + [Deployment Guide](guides/mac-numa-deployment.md)

**Option 2: Single Machine (Original Design)**
- ✅ All-in-one deployment
- ✅ Simpler to set up
- ✅ No network dependencies
- ⚠️ Compute and storage compete for resources
- 📚 Read: [NUMA Design](architecture/numa-design.md) + [Implementation Guide](guides/implementation.md)

**Option 3: Distributed (If You Have Multiple Servers)**
- ✅ Clean separation: compute vs. storage
- ✅ Better resource utilization
- ✅ Data processing local to lake (zero remote I/O)
- ✅ Workers can scale horizontally
- ⚠️ Requires two machines + network
- 📚 Read: [Distributed Architecture](architecture/distributed-architecture.md) + [Distributed Workflow](guides/distributed-workflow.md)

---

### 1. Read Architecture Documentation

**For single machine:** Start with [NUMA Design](architecture/numa-design.md)  
**For distributed:** Start with [Distributed Architecture](architecture/distributed-architecture.md)

### 2. Review VM Specifications

Check [VM Specifications](architecture/vm-specifications.md) to understand resource allocation and tuning for each VM.

### 3. Understand Storage

Read [Storage Design](architecture/storage-design.md) to learn about hot/cold tiering and data placement strategy.

### 4. Follow Implementation Guide

Use the [Implementation Guide](guides/implementation.md) for step-by-step deployment instructions.

### 5. Use Example Configuration

Copy and customize [vm.cluster-numa.json.example](../examples/definitions/virtualmachine/vm.cluster-numa.json.example) for your environment.

## Key Features

- **NUMA-Aware Design**: Zero cross-NUMA memory access on hot path
- **CPU Pinning**: vCPUs pinned to specific physical cores with hyperthreading
- **Emulator Isolation**: Dedicated cores for QEMU emulator threads
- **Hot/Cold Storage Tiering**: SSD for hot data, HDD for cold storage
- **Compression**: Parquet with zstd compression (3-5x reduction)
- **Message-Passing Architecture**: Async coordination with explicit NUMA-aware staging

## Performance Expectations

| Metric | Without Tuning | With NUMA Tuning |
|--------|---------------|------------------|
| Hardware Utilization | 60-70% | 85-95% ⭐ |
| Cross-NUMA Penalty | 2.1x on all ops | 0% on hot path |
| Cache Efficiency | Contention | Dedicated L3 |
| SLM Inference Latency | Variable | Consistent |
| Storage Performance | Mixed | Predictable |

**Target Performance: 90% of hardware capacity** ✅

## System Requirements

- **CPU**: Dual-socket NUMA system with hyperthreading
  - 2× 18 cores = 36 physical cores
  - 2 threads per core = 72 logical threads total
- **Memory**: 2× 126GB RAM (252GB total)
- **Storage**:
  - 350GB SSD (host storage)
  - 8TB HDD (data lake)
- **Network**: Gigabit Ethernet minimum (10GbE recommended)
- **OS**: Linux with libvirt, KVM, qemu

## Workload Description

### VM1: Heavy SLM Worker (Node 0)
- **Resources**: 16 cores (32 threads), 118GB RAM
- **Workload**: Heavy SLM ensemble inference
- **Characteristics**: CPU-intensive, memory-bound, latency-sensitive

### VM2: Data Worker (Node 1)
- **Resources**: 6 cores (12 threads), 60GB RAM, 80GB SSD + 8TB HDD
- **Workload**: Data lake storage and processing
- **Characteristics**: Mixed CPU/I/O, lake queries, bulk writes

### VM3: Light SLM Worker (Node 1)
- **Resources**: 4 cores (8 threads), 32GB RAM
- **Workload**: Light SLM inference
- **Characteristics**: CPU-intensive, memory-bound

### VM4: K3s Server + Task Processing (Node 1)
- **Resources**: 6 cores (12 threads), 24GB RAM
- **Workload**: K3s control plane (API server, scheduler, controller) + heavy parallel processing
- **Characteristics**: K3s system services + CPU-intensive parallel compute

## Design Principles

1. **NUMA Locality First**: All hot-path operations stay within single NUMA node
2. **Explicit Control**: Applications control data placement (hot vs cold)
3. **Isolation**: Heavy workloads isolated on dedicated socket
4. **Simplicity**: Avoid complex caching layers, use explicit tiering
5. **Performance over Flexibility**: Optimize for known workload patterns

## Tools and Commands

### Verification Commands

```bash
# Check NUMA topology
lscpu | grep NUMA
numactl --hardware

# Verify CPU pinning
virsh vcpupin <vm-name>
virsh emulatorpin <vm-name>
virsh numatune <vm-name>

# Monitor NUMA statistics
numastat -c qemu-kvm

# Check storage performance
iostat -x 1
```

### Monitoring Commands

```bash
# VM CPU usage
virsh domstats --vcpu <vm-name>

# Memory usage per NUMA node
numastat -m

# Data lake structure
ls -lhR /data/lake/clean/

# Staging directory size
du -sh /data/hot/staging
```

## Troubleshooting

### Common Issues

1. **High cross-NUMA traffic**
   - Check `numa_memory.mode = strict` in VM config
   - Verify applications aren't requesting cross-NUMA memory

2. **Emulator CPU bottleneck**
   - Monitor emulator thread CPU usage
   - Add more cores to emulator pool if > 50% utilization

3. **Storage performance**
   - Verify hot data on SSD (`/data/hot`)
   - Check lake query performance with partitioning
   - Monitor staging directory cleanup

4. **Variable latency**
   - Verify CPU governor set to `performance`
   - Check for CPU frequency scaling
   - Verify no other processes on pinned CPUs

## Contributing

This documentation is maintained as part of the `homonculus` project. For updates or corrections:

1. Edit markdown files in `docs/`
2. Update examples in `examples/definitions/virtualmachine/`
3. Test configurations before committing
4. Document any changes to architecture or design decisions

## References

### External Resources

- [NUMA Architecture (Wikipedia)](https://en.wikipedia.org/wiki/Non-uniform_memory_access)
- [Libvirt Domain XML Format](https://libvirt.org/formatdomain.html)
- [KVM Performance Tuning](https://www.linux-kvm.org/page/Tuning)
- [Data Lakehouse Architecture](https://www.databricks.com/glossary/data-lakehouse)
- [Parquet Compression](https://parquet.apache.org/docs/file-format/data-pages/compression/)

### Project Files

- System capabilities: `capabilities.xml` (NUMA topology reference)
- VM tuning code: `internal/infrastructure/libvirt/manager.go`
- Template: `templates/libvirt/domain.xml.tpl`
- API definitions: `internal/api/vm.go`

## License

This documentation is part of the homonculus project. See project LICENSE for details.

---

**Last Updated**: 2025-11-09  
**Architecture Version**: 1.0  
**Target Performance**: 90% of hardware capacity ✅

