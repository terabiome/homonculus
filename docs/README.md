# NUMA-Aware K8s Cluster Documentation

## Overview

This documentation describes a high-performance, NUMA-aware Kubernetes cluster designed for running Small Language Model (SLM) inference workloads with database and ETL processing.

## Documentation Structure

### Architecture Documentation

1. **[NUMA Design](architecture/numa-design.md)** (`docs/architecture/numa-design.md`)
   - High-level architecture overview
   - NUMA-aware data flow
   - Design principles and trade-offs
   - Expected performance metrics

2. **[VM Specifications](architecture/vm-specifications.md)** (`docs/architecture/vm-specifications.md`)
   - Detailed specifications for each VM
   - CPU pinning configurations
   - Memory allocation
   - Performance tuning parameters
   - Monitoring and troubleshooting

3. **[Storage Design](architecture/storage-design.md)** (`docs/architecture/storage-design.md`)
   - Hot/cold storage tiering strategy
  - Data placement decisions
  - Compression settings
  - Data lake zones and schemas
  - Storage performance monitoring

### Implementation Guides

4. **[Implementation Guide](guides/implementation.md)** (`docs/guides/implementation.md`)
   - Step-by-step deployment instructions
   - System verification
   - VM creation and configuration
   - Storage setup
   - K8s cluster setup
   - Performance verification

### Example Configurations

5. **[VM Cluster Configuration](../examples/definitions/virtualmachine/vm.cluster-numa.json.example)**
   - Complete example configuration for all 4 VMs
   - CPU pinning settings
   - NUMA memory bindings
   - Emulator CPU allocation

6. **[Cluster with Tuning](../examples/definitions/virtualmachine/vm-cluster.tuning.json.example)**
   - General-purpose example showing tuning options
   - Multiple NUMA memory modes (strict/preferred)
   - Emulator pinning examples

## Quick Start

### 1. Read Architecture Documentation

Start with [NUMA Design](architecture/numa-design.md) to understand the overall architecture and design decisions.

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

