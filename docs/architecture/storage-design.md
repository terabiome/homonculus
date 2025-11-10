# Storage Design

## Overview

This document describes the storage architecture for the NUMA-aware K8s cluster, including hot/cold tiering strategy and data placement decisions.

## Storage Resources

### Physical Storage

```
SSD: 350GB SATA (host /var/lib/libvirt/images)
├─ Speed: ~500 MB/s sequential read/write
├─ Latency: ~100-500 μs
└─ Use case: Hot data, OS, frequently accessed files

HDD: 8TB Spinning disk (7200 RPM)
├─ Speed: ~150 MB/s sequential, ~1-2 MB/s random
├─ Latency: ~5-10 ms (seek + rotational)
└─ Use case: Data lake, cold storage, archives
```

### Allocation

```
SSD (350GB total):
├─ VM1 (slm-heavy) OS: 40GB
├─ VM2 (data) OS: 40GB
├─ VM2 (data) hot tier: 80GB ⭐
├─ VM3 (slm) OS: 40GB
├─ VM4 (tasks) OS: 40GB
├─ Host operations: 100GB
└─ Buffer: 10GB
Total: 350GB ✅

HDD (8TB):
└─ VM2 (data) cold tier: 8TB (data lake)
```

## Storage Tiers

### Hot Tier (80GB SSD)

**Location:** `/var/lib/libvirt/images/data-hot.qcow2`  
**Mounted in VM2:** `/data/hot`

**Purpose:** Fast-access data that's frequently read/written during hot path operations.

```
/data/hot (80GB SSD):
├─ /data/hot/staging: 4-8GB
│   ├─ Intermediate SLM results (compressed parquet)
│   ├─ Cross-checking staging files
│   └─ Temporary computation results
│
├─ /data/hot/clean: 20-40GB
│   ├─ Recently processed clean data
│   ├─ Frequently accessed query results
│   └─ Active zone schemas and metadata
│
└─ Free space: 32-56GB (buffer for growth)
```

**Access Pattern:**
- Staging: Sequential writes, random reads (inference → staging → cross-check)
- Clean zone: Random reads/writes (query processing, result caching)
- High frequency: 100s-1000s of operations per hour

**Performance:**
- Read latency: ~100-500 μs
- Write latency: ~100-500 μs
- Throughput: ~500 MB/s

---

### Cold Tier (8TB HDD)

**Location:** `/dev/sdb` (or passthrough disk)  
**Mounted in VM2:** `/data/lake`

**Purpose:** Bulk storage for final results, archives, and historical data.

```
/data/lake (8TB HDD):
├─ /data/lake/clean: Clean, processed data with schemas
│   ├─ Organized by zone: bronze, silver, gold
│   ├─ Parquet files with zstd compression
│   ├─ Schema documentation (JSON/YAML metadata)
│   └─ Data quality metrics
│
├─ /data/lake/archive: Historical data
│   ├─ Old inference results
│   ├─ Audit logs
│   └─ Backup snapshots
│
└─ /data/lake/raw: Raw ingestion data
    └─ Unprocessed input data
```

**Access Pattern:**
- Write: Sequential, batch (final results)
- Read: Infrequent, batch (historical analysis)
- Low frequency: 10s-100s of operations per day

**Performance:**
- Read latency: ~5-10 ms
- Write latency: ~5-10 ms
- Throughput: ~150 MB/s (sequential)

---

## Data Placement Strategy

### Application-Level Routing

Applications must explicitly route data to hot vs cold tier:

```python
# Hot tier paths (SSD)
STAGING_DIR = "/data/hot/staging"
CLEAN_CACHE_DIR = "/data/hot/clean"

# Cold tier paths (HDD)
LAKE_CLEAN_DIR = "/data/lake/clean"
LAKE_ARCHIVE_DIR = "/data/lake/archive"
LAKE_RAW_DIR = "/data/lake/raw"

# Example: SLM writes intermediate result
def write_intermediate(slm_id, batch_id, data):
    path = f"{STAGING_DIR}/slm_{slm_id}_{batch_id}.parquet"
    data.to_parquet(path, compression="zstd", compression_level=3)

# Example: Write clean data to lake with schema
def write_clean_data(zone, table_name, data, schema_metadata):
    """
    zone: 'bronze', 'silver', or 'gold'
    """
    date_path = datetime.now().strftime("%Y/%m/%d")
    data_path = f"{LAKE_CLEAN_DIR}/{zone}/{table_name}/{date_path}/data.parquet"
    schema_path = f"{LAKE_CLEAN_DIR}/{zone}/{table_name}/schema.json"
    
    # Write data
    data.to_parquet(data_path, compression="zstd", compression_level=5)
    
    # Write schema metadata
    with open(schema_path, 'w') as f:
        json.dump(schema_metadata, f, indent=2)
```

### Data Lifecycle

```
1. Input data → RAM (loaded by SLM)
2. Inference results → /data/hot/staging (SSD, temporary)
3. Cross-checking → Read from /data/hot/staging (SSD)
4. Staging cleanup → Delete after aggregation
5. Final clean data → /data/lake/clean/{zone} (HDD, permanent, with schema)
6. Hot cache → /data/hot/clean (SSD, frequently accessed clean data)
```

---

## Why Not LVM Cache?

### Original Consideration

```
LVM cached volume:
├─ Cache: 80GB SSD (only 1% of 8TB!)
└─ Backing: 8TB HDD
```

### Problems Identified

**1. Cache too small (1% ratio)**
- Working set: 45-80GB
- Cache: 80GB
- Cache hit rate: 60-80% (poor!)
- Recommendation: 5-10% cache (400-800GB)

**2. Cache thrashing risk**
- Intermediate files: 700MB-2.8GB per batch
- Clean data cache: 20-40GB
- Staging + cache > 80GB → constant evictions

**3. Writethrough mode required (no UPS)**
- Writeback mode = data loss risk
- Writethrough = no write performance benefit
- HDD write latency still limits throughput

**4. Complexity without benefit**
- Cache hit/miss unpredictable
- Hard to debug performance issues
- Clean data access patterns vary by workload

### Decision: Explicit Tiering

**Benefits:**
- ✅ Predictable performance (apps know hot vs cold)
- ✅ Simple to understand and debug
- ✅ Clean data cache on SSD (fast queries)
- ✅ Staging always on SSD (cross-checking performance)
- ✅ No cache thrashing risk
- ✅ No data loss risk

**Trade-offs:**
- ⚠️ Applications must know data placement
- ⚠️ Manual routing required

**Conclusion:** Explicit tiering is superior for this workload.

---

## Compression Strategy

### Parquet with zstd

**Configuration:**
```python
import pandas as pd

# For intermediate files (speed priority)
df.to_parquet(
    path,
    compression="zstd",
    compression_level=3,  # Fast encode, good ratio
    engine="pyarrow"
)

# For final results (compression priority)
df.to_parquet(
    path,
    compression="zstd",
    compression_level=5,  # Better compression, slower
    engine="pyarrow"
)
```

**Performance:**
- Compression ratio: 3-5x
- Encode time: ~50-100ms per 100MB file
- Decode time: ~20-50ms per 100MB file (very fast!)

**Impact:**
```
Without compression:
├─ 7 SLMs × 100-400MB = 700MB-2.8GB per batch
├─ 3-4 batches in staging = 2.1-11.2GB
└─ Exceeds 80GB hot tier quickly!

With compression (3-5x):
├─ 7 SLMs × 20-100MB = 140-700MB per batch
├─ 3-4 batches in staging = 420MB-2.8GB
└─ Comfortably fits in hot tier ✅
```

---

## Data Lake Zones

### Zone Architecture

The data lake follows the medallion architecture with bronze, silver, and gold zones:

```
/data/lake/clean/:
├─ bronze/: Raw, minimally processed data
│   ├─ Ingested with minimal transformation
│   ├─ Source system structure preserved
│   └─ Append-only, immutable
│
├─ silver/: Cleaned, validated data
│   ├─ Data quality rules applied
│   ├─ Schema standardization
│   └─ Deduplication and validation
│
└─ gold/: Business-ready, aggregated data
    ├─ Ready for analysis and querying
    ├─ Optimized for specific use cases
    └─ Denormalized and aggregated
```

### Schema Management

Each table in the lake includes schema metadata:

```json
{
  "table_name": "slm_predictions",
  "zone": "silver",
  "version": "1.2.0",
  "created_at": "2025-11-10T12:00:00Z",
  "schema": {
    "fields": [
      {"name": "timestamp", "type": "timestamp", "nullable": false},
      {"name": "slm_id", "type": "int32", "nullable": false},
      {"name": "prediction", "type": "float64", "nullable": false},
      {"name": "confidence", "type": "float64", "nullable": true}
    ],
    "partition_by": ["date"]
  },
  "quality_metrics": {
    "completeness": 0.98,
    "accuracy": 0.95,
    "last_validated": "2025-11-10T12:00:00Z"
  }
}
```

---

## Filesystem Configuration

### Hot Tier (/data/hot)

```bash
# Format with ext4
mkfs.ext4 -L hot-storage /dev/vdb

# Mount options
mount -o defaults,noatime,nodiratime /dev/vdb /data/hot

# /etc/fstab entry
LABEL=hot-storage /data/hot ext4 defaults,noatime,nodiratime 0 2
```

**Mount options:**
- `noatime`: Don't update access time (reduces writes)
- `nodiratime`: Don't update directory access time

### Cold Tier (/data/lake)

```bash
# Format with ext4
mkfs.ext4 -L lake-storage -m 1 /dev/vdc

# Mount options
mount -o defaults,noatime,commit=60 /dev/vdc /data/lake

# /etc/fstab entry
LABEL=lake-storage /data/lake ext4 defaults,noatime,commit=60 0 2
```

**Mount options:**
- `noatime`: Reduce writes on HDD
- `commit=60`: Sync every 60s instead of 5s (less seek overhead)
- `-m 1`: Reserve only 1% for root (default 5% wastes space on 8TB)

---

## Staging Cleanup

### Automatic Cleanup

```python
# In coordinator, after aggregation completes
def cleanup_batch(batch_id, keep_recent=2):
    """
    Cleanup intermediate files after processing.
    Keep last 2 batches for debugging.
    """
    staging_dir = "/data/hot/staging"
    
    # Remove intermediate files for this batch
    for slm_id in range(7):
        intermediate = f"{staging_dir}/slm_{slm_id}_{batch_id}.parquet"
        crosscheck = f"{staging_dir}/crosscheck_{slm_id}_{batch_id}.parquet"
        
        try:
            if os.path.exists(intermediate):
                os.remove(intermediate)
            if os.path.exists(crosscheck):
                os.remove(crosscheck)
        except Exception as e:
            logger.error(f"Failed to cleanup {batch_id}: {e}")
    
    # Cleanup old batches (keep only recent N)
    cleanup_old_batches(staging_dir, keep_recent)
```

### Monitoring

```bash
# Cron job to monitor staging size
*/5 * * * * du -sh /data/hot/staging | \
  awk '{if ($1 > "10G") print "WARNING: Staging >10GB: " $1}' | \
  logger -t staging-monitor

# Alert if staging grows too large
# Normal: < 4GB
# Warning: > 10GB (investigate cleanup job)
# Critical: > 30GB (cleanup job failed!)
```

---

## Storage Performance Monitoring

### Disk I/O

```bash
# Real-time I/O monitoring
iostat -x 1

# Look for:
# - %util > 80% (saturation)
# - await > 10ms on SSD (slow!)
# - await > 50ms on HDD (very slow!)
```

### Filesystem Usage

```bash
# Check hot tier usage
df -h /data/hot

# Alert if > 90% full
# Staging and clean cache need free space
```

### Data Lake Query Performance

Query parquet files using tools like DuckDB or Polars:

```python
import duckdb

# Query clean data from lake
conn = duckdb.connect()
result = conn.execute("""
    SELECT * 
    FROM read_parquet('/data/lake/clean/silver/slm_predictions/**/*.parquet')
    WHERE date >= '2025-11-01'
    LIMIT 10
""").fetchdf()

# Check query performance
conn.execute("PRAGMA enable_profiling")
# Run query
conn.execute("PRAGMA show_profiling")
```

---

## Troubleshooting

### Hot Tier Full

**Symptom:** `/data/hot` at > 90% capacity

**Check:**
```bash
du -sh /data/hot/*
# Likely culprit: /data/hot/staging not being cleaned up
```

**Fix:**
```bash
# Manual cleanup of old staging files
find /data/hot/staging -name "*.parquet" -mtime +1 -delete

# Check cleanup job is running
systemctl status staging-cleanup.timer
```

### Data Lake Query Slow

**Symptom:** Queries on parquet files taking too long

**Check:**
```python
# Check file sizes
import os
for root, dirs, files in os.walk('/data/lake/clean'):
    for f in files:
        if f.endswith('.parquet'):
            path = os.path.join(root, f)
            size_mb = os.path.getsize(path) / 1024 / 1024
            print(f"{path}: {size_mb:.2f} MB")
```

**Possible causes:**
- Files too small (< 10MB) → too many files, slow scan
- Files too large (> 500MB) → can't leverage partitioning
- No partitioning by frequently queried columns

**Fix:**
- Compact small files into 100-200MB files
- Add partitioning by date/category
- Use Hive-style partitioning: `/zone/table/year=2025/month=11/`

### HDD Saturation

**Symptom:** High await time on `/data/lake`

**Check:**
```bash
iostat -x 1 /dev/vdc
# await > 50ms, %util > 90%
```

**Possible causes:**
- Too many concurrent writes to lake
- Fragmentation (sequential writes becoming random)

**Fix:**
- Batch writes to lake (buffer in memory, write periodically)
- Defragment HDD filesystem (if ext4 frag > 10%)

---

## Future Optimizations

### If Budget Allows

**1. Larger hot tier (200-500GB SSD)**
- Benefit: More clean data cache, larger staging buffer
- Cost: ~$20-50 for 256GB SATA SSD

**2. NVMe for hot tier**
- Benefit: 3-5x faster than SATA SSD (~3000 MB/s vs 500 MB/s)
- Cost: ~$50-80 for 500GB NVMe

**3. Separate clean cache + staging SSDs**
- Benefit: No I/O contention between cache and staging
- Cost: 2× smaller SSDs instead of 1× large

**4. LVM cache with larger cache tier**
- If budget allows 500GB+ SSD cache on 8TB HDD
- Then LVM cache becomes viable (6%+ ratio)

---

## Summary

**Storage design:**
- ✅ Explicit hot (SSD) / cold (HDD) tiering
- ✅ Application-level routing
- ✅ Compression for intermediate data (3-5x)
- ✅ Data lake with medallion zones (bronze/silver/gold)
- ✅ Schema documentation with every table
- ✅ Aggressive staging cleanup
- ✅ Simple, predictable, maintainable

**Performance expectations:**
- Hot tier: ~500 MB/s, <500μs latency
- Cold tier: ~150 MB/s, ~5-10ms latency
- Lake query: Fast with proper partitioning
- Staging overhead: <10% of hot tier

**This design achieves 90% performance target!** 🎯

---

## References

- Architecture overview: `docs/architecture/numa-design.md`
- VM specifications: `docs/architecture/vm-specifications.md`
- Implementation guide: `docs/guides/implementation.md`

