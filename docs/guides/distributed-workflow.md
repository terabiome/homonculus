# Distributed Workflow Guide

## Overview

This guide describes the end-to-end workflow for the distributed two-machine architecture, covering daily data preparation, job execution, and result aggregation.

---

## System Components

### **Worker Cluster (Machine 1)**
- 2 VMs running SLM inference pods
- Local staging: 100GB SSD per VM
- Role: Pure compute

### **Data Server (Machine 2)**
- K3s control plane
- Temporal orchestrator
- Data lake (8TB HDD)
- Data preparation services
- Role: Orchestration + storage

---

## Daily Workflow

### **6:00 AM - Data Preparation**

**Location:** Data Server

```python
#!/usr/bin/env python3
# Daily data preparation job
# Runs on data server at start of day

import pandas as pd
import requests
from datetime import date

class DailyDataPreparation:
    def __init__(self):
        self.lake_dir = "/data/lake"
        self.workers = [
            "http://worker1-ip:8080",
            "http://worker2-ip:8080"
        ]
    
    def run(self, target_date=None):
        """
        Prepare and distribute daily data to all workers.
        """
        if target_date is None:
            target_date = date.today()
        
        print(f"=== Daily Prep: {target_date} ===")
        
        # 1. Extract data from lake
        input_data = self.extract_daily_inputs(target_date)
        print(f"Extracted {len(input_data)} records")
        
        # 2. Pre-process and optimize
        processed = self.preprocess(input_data)
        
        # 3. Load pattern memories (if shared)
        patterns = self.load_patterns(target_date)
        print(f"Loaded {len(patterns)} patterns")
        
        # 4. Distribute to all workers
        for worker_url in self.workers:
            self.transfer_to_worker(
                worker_url,
                processed,
                patterns,
                target_date
            )
        
        print("=== Daily Prep Complete ===")
    
    def extract_daily_inputs(self, target_date):
        """Extract today's input data from lake."""
        # Query data lake for relevant data
        path = f"{self.lake_dir}/clean/bronze/inputs/{target_date.year}/{target_date.month:02d}/*.parquet"
        df = pd.read_parquet(path)
        return df
    
    def preprocess(self, data):
        """Pre-process data for inference."""
        # Clean, transform, optimize
        processed = data.copy()
        
        # Apply transformations
        processed = self.normalize(processed)
        processed = self.feature_engineering(processed)
        
        return processed
    
    def load_patterns(self, target_date):
        """Load pattern memories for the day."""
        pattern_path = f"{self.lake_dir}/patterns/daily_{target_date}.pkl"
        
        import pickle
        with open(pattern_path, 'rb') as f:
            patterns = pickle.load(f)
        
        return patterns
    
    def transfer_to_worker(self, worker_url, data, patterns, target_date):
        """Transfer data to a single worker."""
        import tempfile
        import os
        
        # Save to temporary files
        with tempfile.TemporaryDirectory() as tmpdir:
            # Save input data
            data_path = os.path.join(tmpdir, f"inputs_{target_date}.parquet")
            data.to_parquet(
                data_path,
                compression="zstd",
                compression_level=3
            )
            
            # Save patterns
            pattern_path = os.path.join(tmpdir, f"patterns_{target_date}.pkl")
            import pickle
            with open(pattern_path, 'wb') as f:
                pickle.dump(patterns, f)
            
            # Upload to worker
            files = {
                'inputs': open(data_path, 'rb'),
                'patterns': open(pattern_path, 'rb')
            }
            
            response = requests.post(
                f"{worker_url}/staging/upload-daily",
                files=files,
                data={'date': str(target_date)},
                timeout=300  # 5 minute timeout
            )
            
            if response.status_code == 200:
                print(f"✓ Transferred to {worker_url}")
            else:
                print(f"✗ Failed to transfer to {worker_url}: {response.text}")
                raise Exception("Transfer failed")

if __name__ == "__main__":
    prep = DailyDataPreparation()
    prep.run()
```

**Cron job:**
```bash
# /etc/cron.d/daily-prep
0 6 * * * /opt/data-prep/daily_prep.py >> /var/log/daily-prep.log 2>&1
```

---

### **Worker Staging API**

**Location:** Each worker VM

```python
#!/usr/bin/env python3
# Worker staging API
# Receives daily data and serves results

from flask import Flask, request, send_file, jsonify
import os
from werkzeug.utils import secure_filename
import logging

app = Flask(__name__)

STAGING_DIR = "/data/local"
INPUTS_DIR = os.path.join(STAGING_DIR, "inputs")
OUTPUTS_DIR = os.path.join(STAGING_DIR, "outputs")
CACHE_DIR = os.path.join(STAGING_DIR, "cache")

# Create directories
os.makedirs(INPUTS_DIR, exist_ok=True)
os.makedirs(OUTPUTS_DIR, exist_ok=True)
os.makedirs(CACHE_DIR, exist_ok=True)

# Simple API token auth
API_TOKEN = os.environ.get("STAGING_API_TOKEN", "changeme")

def require_auth(f):
    from functools import wraps
    @wraps(f)
    def decorated(*args, **kwargs):
        token = request.headers.get("X-API-Token")
        if token != API_TOKEN:
            return jsonify({"error": "Unauthorized"}), 403
        return f(*args, **kwargs)
    return decorated

@app.route("/staging/upload-daily", methods=["POST"])
@require_auth
def upload_daily():
    """Receive daily data from data server."""
    try:
        target_date = request.form.get("date")
        
        # Save inputs
        if "inputs" in request.files:
            inputs_file = request.files["inputs"]
            inputs_path = os.path.join(INPUTS_DIR, f"daily_inputs_{target_date}.parquet")
            inputs_file.save(inputs_path)
            logging.info(f"Saved inputs: {inputs_path}")
        
        # Save patterns
        if "patterns" in request.files:
            patterns_file = request.files["patterns"]
            patterns_path = os.path.join(INPUTS_DIR, f"patterns_{target_date}.pkl")
            patterns_file.save(patterns_path)
            logging.info(f"Saved patterns: {patterns_path}")
        
        return jsonify({"status": "ok", "date": target_date})
    
    except Exception as e:
        logging.error(f"Upload failed: {e}")
        return jsonify({"error": str(e)}), 500

@app.route("/staging/download", methods=["GET"])
@require_auth
def download_result():
    """Allow data server to download result files."""
    filename = request.args.get("filename")
    
    if not filename:
        return jsonify({"error": "filename required"}), 400
    
    # Security: prevent directory traversal
    if ".." in filename or filename.startswith("/"):
        return jsonify({"error": "Invalid filename"}), 400
    
    filepath = os.path.join(OUTPUTS_DIR, filename)
    
    if not os.path.exists(filepath):
        return jsonify({"error": "File not found"}), 404
    
    return send_file(filepath, as_attachment=True)

@app.route("/staging/cleanup", methods=["POST"])
@require_auth
def cleanup():
    """Delete a file after successful transfer."""
    data = request.json
    filename = data.get("filename")
    
    if not filename:
        return jsonify({"error": "filename required"}), 400
    
    filepath = os.path.join(OUTPUTS_DIR, filename)
    
    if os.path.exists(filepath):
        os.remove(filepath)
        logging.info(f"Deleted: {filepath}")
        return jsonify({"status": "deleted", "filename": filename})
    else:
        return jsonify({"error": "File not found"}), 404

@app.route("/staging/status", methods=["GET"])
@require_auth
def status():
    """Get staging directory status."""
    import shutil
    
    usage = shutil.disk_usage(STAGING_DIR)
    
    return jsonify({
        "disk": {
            "total_gb": usage.total / (1024**3),
            "used_gb": usage.used / (1024**3),
            "free_gb": usage.free / (1024**3),
            "percent": (usage.used / usage.total) * 100
        },
        "files": {
            "inputs": len(os.listdir(INPUTS_DIR)),
            "outputs": len(os.listdir(OUTPUTS_DIR)),
            "cache": len(os.listdir(CACHE_DIR))
        }
    })

if __name__ == "__main__":
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s %(levelname)s: %(message)s'
    )
    
    # Run on all interfaces, port 8080
    app.run(host="0.0.0.0", port=8080)
```

**Systemd service:**
```ini
# /etc/systemd/system/staging-api.service
[Unit]
Description=Worker Staging API
After=network.target

[Service]
Type=simple
User=nobody
WorkingDirectory=/opt/staging-api
Environment=STAGING_API_TOKEN=your-secret-token-here
ExecStart=/usr/bin/python3 /opt/staging-api/app.py
Restart=always

[Install]
WantedBy=multi-user.target
```

---

## Job Execution Workflow

### **Temporal Workflow Definition**

**Location:** Data Server

```python
from temporalio import workflow, activity
from datetime import timedelta
import asyncio

@workflow.defn
class SLMJobWorkflow:
    """
    Main workflow for SLM inference jobs.
    """
    
    def __init__(self):
        self.completed_slms = []
        self.total_slms = 7
    
    @workflow.run
    async def run(self, job_id: str, input_params: dict):
        """
        Execute SLM inference job across all workers.
        
        Steps:
        1. Dispatch job to all SLM pods
        2. Wait for all completions
        3. Transfer results from workers
        4. Aggregate and write to lake
        """
        workflow.logger.info(f"Starting job {job_id}")
        
        # 1. Dispatch to all SLMs in parallel
        slm_tasks = []
        for slm_id in range(self.total_slms):
            task = workflow.execute_activity(
                dispatch_slm_job,
                args=[slm_id, job_id, input_params],
                start_to_close_timeout=timedelta(minutes=15),
                retry_policy=workflow.RetryPolicy(
                    initial_interval=timedelta(seconds=1),
                    maximum_attempts=3
                )
            )
            slm_tasks.append(task)
        
        # Wait for all SLMs to signal completion
        await workflow.wait_condition(
            lambda: len(self.completed_slms) == self.total_slms,
            timeout=timedelta(minutes=30)
        )
        
        workflow.logger.info(f"All {self.total_slms} SLMs completed for job {job_id}")
        
        # 2. Transfer all results in parallel
        transfer_tasks = []
        for slm_result in self.completed_slms:
            task = workflow.execute_activity(
                transfer_result_from_worker,
                args=[slm_result],
                start_to_close_timeout=timedelta(minutes=5),
                retry_policy=workflow.RetryPolicy(
                    initial_interval=timedelta(seconds=5),
                    maximum_attempts=3
                )
            )
            transfer_tasks.append(task)
        
        transferred_results = await asyncio.gather(*transfer_tasks)
        
        workflow.logger.info(f"Transferred {len(transferred_results)} results")
        
        # 3. Aggregate results
        final_result = await workflow.execute_activity(
            aggregate_results,
            args=[job_id, transferred_results],
            start_to_close_timeout=timedelta(minutes=10)
        )
        
        workflow.logger.info(f"Job {job_id} complete: {final_result}")
        
        return {
            "job_id": job_id,
            "status": "complete",
            "slms_completed": self.total_slms,
            "total_size_mb": sum(r['size_mb'] for r in transferred_results),
            "lake_path": final_result['lake_path']
        }
    
    @workflow.signal
    def slm_completed(self, slm_result: dict):
        """
        Signal handler: SLM worker notifies job completion.
        """
        workflow.logger.info(
            f"SLM {slm_result['slm_id']} completed job {slm_result['job_id']}"
        )
        self.completed_slms.append(slm_result)


@activity.defn
async def dispatch_slm_job(slm_id: int, job_id: str, input_params: dict):
    """
    Dispatch job to specific SLM pod.
    """
    # Find pod for this SLM
    pod_name = f"slm-worker-{slm_id}"
    
    # Send job request to pod
    import requests
    response = requests.post(
        f"http://{pod_name}:8080/inference/run",
        json={
            "job_id": job_id,
            "params": input_params
        },
        timeout=60
    )
    
    if response.status_code != 200:
        raise Exception(f"Failed to dispatch to SLM {slm_id}: {response.text}")
    
    return {"slm_id": slm_id, "status": "dispatched"}


@activity.defn
async def transfer_result_from_worker(slm_result: dict):
    """
    Transfer result file from worker to data server.
    """
    import requests
    import os
    
    worker_ip = slm_result['worker_ip']
    filename = slm_result['filename']
    job_id = slm_result['job_id']
    slm_id = slm_result['slm_id']
    
    # Download from worker
    response = requests.get(
        f"http://{worker_ip}:8080/staging/download",
        params={"filename": filename},
        headers={"X-API-Token": os.environ.get("STAGING_API_TOKEN")},
        stream=True,
        timeout=300
    )
    
    if response.status_code != 200:
        raise Exception(f"Failed to download {filename}: {response.text}")
    
    # Save to temporary location
    temp_path = f"/tmp/slm_{slm_id}_job_{job_id}.parquet"
    with open(temp_path, 'wb') as f:
        for chunk in response.iter_content(chunk_size=8192):
            f.write(chunk)
    
    # Write to lake (silver zone)
    from datetime import date
    today = date.today()
    lake_path = f"/data/lake/clean/silver/slm_results/{today.year}/{today.month:02d}/{today.day:02d}/slm_{slm_id}_job_{job_id}.parquet"
    
    os.makedirs(os.path.dirname(lake_path), exist_ok=True)
    os.rename(temp_path, lake_path)
    
    # Cleanup worker staging
    requests.post(
        f"http://{worker_ip}:8080/staging/cleanup",
        headers={"X-API-Token": os.environ.get("STAGING_API_TOKEN")},
        json={"filename": filename}
    )
    
    size_mb = os.path.getsize(lake_path) / (1024 ** 2)
    
    return {
        "slm_id": slm_id,
        "job_id": job_id,
        "lake_path": lake_path,
        "size_mb": size_mb
    }


@activity.defn
async def aggregate_results(job_id: str, results: list):
    """
    Aggregate all SLM results and write to gold zone.
    """
    import pandas as pd
    from datetime import date
    
    # Load all results
    dfs = []
    for result in results:
        df = pd.read_parquet(result['lake_path'])
        dfs.append(df)
    
    # Concatenate
    combined = pd.concat(dfs, ignore_index=True)
    
    # Aggregate (application-specific logic)
    aggregated = combined.groupby('key').agg({
        'value': 'mean',
        'confidence': 'mean'
    }).reset_index()
    
    # Write to gold zone
    today = date.today()
    gold_path = f"/data/lake/clean/gold/aggregated/{today.year}/{today.month:02d}/{today.day:02d}/job_{job_id}.parquet"
    
    os.makedirs(os.path.dirname(gold_path), exist_ok=True)
    aggregated.to_parquet(
        gold_path,
        compression="zstd",
        compression_level=5
    )
    
    # Write schema metadata
    schema_path = gold_path.replace('.parquet', '_schema.json')
    import json
    schema_metadata = {
        "table_name": "aggregated_results",
        "job_id": job_id,
        "date": str(today),
        "num_records": len(aggregated),
        "schema": {
            "fields": [
                {"name": col, "type": str(dtype)}
                for col, dtype in aggregated.dtypes.items()
            ]
        }
    }
    
    with open(schema_path, 'w') as f:
        json.dump(schema_metadata, f, indent=2)
    
    return {
        "gold_path": gold_path,
        "num_records": len(aggregated),
        "size_mb": os.path.getsize(gold_path) / (1024 ** 2)
    }
```

---

### **SLM Worker Pod**

**Location:** Worker VMs (K8s pods)

```python
#!/usr/bin/env python3
# SLM worker pod - runs inference

from flask import Flask, request, jsonify
import pandas as pd
import os
from datetime import date
from temporalio.client import Client

app = Flask(__name__)

SLM_ID = int(os.environ.get("SLM_ID", "0"))
STAGING_DIR = "/data/local"
INPUTS_DIR = os.path.join(STAGING_DIR, "inputs")
OUTPUTS_DIR = os.path.join(STAGING_DIR, "outputs")

# Load model (pseudo-code)
model = load_slm_model(SLM_ID)

@app.route("/inference/run", methods=["POST"])
async def run_inference():
    """
    Run inference job.
    """
    data = request.json
    job_id = data['job_id']
    params = data['params']
    
    try:
        # 1. Load input data from local SSD
        today = date.today()
        input_path = os.path.join(INPUTS_DIR, f"daily_inputs_{today}.parquet")
        input_data = pd.read_parquet(input_path)
        
        # 2. Load patterns from local SSD
        pattern_path = os.path.join(INPUTS_DIR, f"patterns_{today}.pkl")
        import pickle
        with open(pattern_path, 'rb') as f:
            patterns = pickle.load(f)
        
        # 3. Run inference
        results = model.predict(input_data, patterns, **params)
        
        # 4. Write results to local SSD
        output_filename = f"slm_{SLM_ID}_job_{job_id}.parquet"
        output_path = os.path.join(OUTPUTS_DIR, output_filename)
        results.to_parquet(
            output_path,
            compression="zstd",
            compression_level=3
        )
        
        size_mb = os.path.getsize(output_path) / (1024 ** 2)
        
        # 5. Signal Temporal workflow
        temporal_client = await Client.connect("data-server:7233")
        
        await temporal_client.get_workflow_handle(
            workflow_id=f"slm-job-{job_id}"
        ).signal(
            "slm_completed",
            {
                "slm_id": SLM_ID,
                "job_id": job_id,
                "filename": output_filename,
                "worker_ip": os.environ.get("NODE_IP"),
                "size_mb": size_mb
            }
        )
        
        return jsonify({
            "status": "complete",
            "slm_id": SLM_ID,
            "job_id": job_id,
            "output_path": output_path,
            "size_mb": size_mb
        })
    
    except Exception as e:
        return jsonify({
            "status": "error",
            "error": str(e)
        }), 500

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
```

---

## Monitoring

### **Worker Health Check**

```bash
#!/bin/bash
# Check worker staging status

for worker in worker1-ip worker2-ip; do
    echo "=== $worker ==="
    curl -H "X-API-Token: $STAGING_API_TOKEN" \
         http://$worker:8080/staging/status | jq
done
```

### **Job Status**

```bash
# List running workflows
tctl workflow list

# Describe specific job
tctl workflow describe -w slm-job-12345

# Show workflow history
tctl workflow show -w slm-job-12345
```

### **Disk Usage**

```bash
# Workers
ssh worker1 "df -h /data/local"
ssh worker2 "df -h /data/local"

# Data server
ssh data-server "df -h /data/lake"
```

---

## Troubleshooting

### **Job Stuck in Transfer**

```bash
# Check network connectivity
ping worker1-ip
iperf3 -c worker1-ip

# Check staging API
curl http://worker1-ip:8080/staging/status

# Check if files exist on worker
ssh worker1 "ls -lh /data/local/outputs/"

# Manually trigger transfer
curl -H "X-API-Token: $STAGING_API_TOKEN" \
     http://worker1-ip:8080/staging/download?filename=slm_1_job_123.parquet \
     -o /tmp/manual-download.parquet
```

### **Staging Disk Full**

```bash
# Check usage
ssh worker1 "df -h /data/local"

# List large files
ssh worker1 "du -sh /data/local/* | sort -h"

# Manual cleanup (BE CAREFUL!)
ssh worker1 "find /data/local/outputs -name '*.parquet' -mtime +1 -delete"
```

### **Daily Prep Failed**

```bash
# Check prep logs
tail -f /var/log/daily-prep.log

# Manually run prep
ssh data-server
sudo su - prep-user
/opt/data-prep/daily_prep.py
```

---

## Summary

**Daily Cycle:**
1. 6:00 AM - Data prep distributes inputs to workers
2. Throughout day - Jobs run, write to local SSD
3. Per job - Results transfer to data server (16-32s)
4. End of day - All results in lake, workers cleaned up

**Key Points:**
- Workers are stateless (daily refresh)
- All inference happens on local SSD (fast)
- Network only used for start/end transfers (efficient)
- Temporal handles orchestration (reliable)
- Manual recovery acceptable (home lab)

