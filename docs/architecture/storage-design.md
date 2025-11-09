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
├─ /data/hot/postgresql: 20-40GB
│   ├─ Database files (tables, indexes)
│   ├─ Write-ahead log (WAL)
│   └─ pg_stat, pg_xlog
│
├─ /data/hot/staging: 2-4GB
│   ├─ Intermediate SLM results (compressed parquet)
│   ├─ Cross-checking staging files
│   └─ Temporary computation results
│
└─ Free space: 36-58GB (buffer for growth)
```

**Access Pattern:**
- PostgreSQL: Random reads/writes (queries, transactions)
- Staging: Sequential writes, random reads (inference → staging → cross-check)
- High frequency: 1000s of operations per second

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
├─ /data/lake/results: Final aggregated results
│   ├─ Organized by date: YYYY/MM/DD/
│   └─ Parquet files with zstd compression
│
├─ /data/lake/archive: Historical data
│   ├─ Old inference results
│   └─ Audit logs
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
POSTGRESQL_DIR = "/data/hot/postgresql"
STAGING_DIR = "/data/hot/staging"

# Cold tier paths (HDD)
LAKE_RESULTS_DIR = "/data/lake/results"
LAKE_ARCHIVE_DIR = "/data/lake/archive"
LAKE_RAW_DIR = "/data/lake/raw"

# Example: SLM writes intermediate result
def write_intermediate(slm_id, batch_id, data):
    path = f"{STAGING_DIR}/slm_{slm_id}_{batch_id}.parquet"
    data.to_parquet(path, compression="zstd", compression_level=3)

# Example: Final result to lake
def write_final(batch_id, data):
    date_path = datetime.now().strftime("%Y/%m/%d")
    path = f"{LAKE_RESULTS_DIR}/{date_path}/batch_{batch_id}.parquet"
    data.to_parquet(path, compression="zstd", compression_level=5)
```

### Data Lifecycle

```
1. Input data → RAM (loaded by SLM)
2. Inference results → /data/hot/staging (SSD, temporary)
3. Cross-checking → Read from /data/hot/staging (SSD)
4. Staging cleanup → Delete after aggregation
5. Final results → /data/lake/results (HDD, permanent)
6. PostgreSQL data → /data/hot/postgresql (SSD, permanent)
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
- PostgreSQL hot data: 20-40GB
- Staging + DB > 80GB → constant evictions

**3. Writethrough mode required (no UPS)**
- Writeback mode = data loss risk
- Writethrough = no write performance benefit
- HDD write latency still limits throughput

**4. Complexity without benefit**
- Cache hit/miss unpredictable
- Hard to debug performance issues
- PostgreSQL performance varies based on cache state

### Decision: Explicit Tiering

**Benefits:**
- ✅ Predictable performance (apps know hot vs cold)
- ✅ Simple to understand and debug
- ✅ PostgreSQL always on SSD (critical!)
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

## PostgreSQL Configuration

### Data Directory

```bash
# Initialize on hot tier
sudo -u postgres initdb -D /data/hot/postgresql
```

### Configuration (/data/hot/postgresql/postgresql.conf)

```ini
# Memory settings (60GB VM)
shared_buffers = 15GB           # 25% of RAM
effective_cache_size = 45GB     # 75% of RAM (includes OS cache)
work_mem = 256MB                # Per operation
maintenance_work_mem = 2GB      # For VACUUM, CREATE INDEX

# Connection settings
max_connections = 200

# WAL settings (on SSD, can be aggressive)
wal_buffers = 16MB
checkpoint_timeout = 15min
checkpoint_completion_target = 0.9
max_wal_size = 4GB

# Query planner
random_page_cost = 1.1          # SSD is fast for random access
effective_io_concurrency = 200  # SSD can handle many concurrent I/O

# Logging (minimal for performance)
logging_collector = on
log_directory = '/data/hot/postgresql/log'
log_filename = 'postgresql-%Y-%m-%d.log'
log_rotation_age = 1d
log_min_duration_statement = 1000  # Log queries > 1s
```

### Backup Strategy

```bash
# Daily backup to lake (cold tier)
pg_dump dbname | gzip > /data/lake/archive/backup-$(date +%Y%m%d).sql.gz

# Keep last 30 days on HDD
find /data/lake/archive -name "backup-*.sql.gz" -mtime +30 -delete
```

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
# PostgreSQL needs free space for temp tables
```

### PostgreSQL Performance

```sql
-- Check cache hit ratio (should be > 95%)
SELECT 
  sum(heap_blks_read) as heap_read,
  sum(heap_blks_hit) as heap_hit,
  sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as cache_hit_ratio
FROM pg_statio_user_tables;

-- Check slow queries
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
WHERE mean_exec_time > 1000  -- > 1 second
ORDER BY mean_exec_time DESC
LIMIT 10;
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

### PostgreSQL Slow Queries

**Symptom:** Queries taking > 1 second

**Check:**
```sql
-- Check if indexes are being used
EXPLAIN ANALYZE SELECT ...;

-- Check for table bloat
SELECT schemaname, tablename, 
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

**Fix:**
```sql
-- Vacuum and analyze
VACUUM ANALYZE;

-- Reindex if needed
REINDEX TABLE tablename;
```

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
- Benefit: More PostgreSQL cache, larger staging buffer
- Cost: ~$20-50 for 256GB SATA SSD

**2. NVMe for hot tier**
- Benefit: 3-5x faster than SATA SSD (~3000 MB/s vs 500 MB/s)
- Cost: ~$50-80 for 500GB NVMe

**3. Separate PostgreSQL + staging SSDs**
- Benefit: No I/O contention between DB and staging
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
- ✅ PostgreSQL on SSD with tuned config
- ✅ Aggressive staging cleanup
- ✅ Simple, predictable, maintainable

**Performance expectations:**
- Hot tier: ~500 MB/s, <500μs latency
- Cold tier: ~150 MB/s, ~5-10ms latency
- PostgreSQL cache hit: >95%
- Staging overhead: <5% of hot tier

**This design achieves 90% performance target!** 🎯

---

## References

- Architecture overview: `docs/architecture/numa-design.md`
- VM specifications: `docs/architecture/vm-specifications.md`
- Implementation guide: `docs/guides/implementation.md`

