# Finance Data Analysis with Heterogeneous SLM Ensemble

## Overview

This document describes the deployment and operation of a **heterogeneous SLM ensemble** for financial data analysis using the distributed two-machine architecture. The system analyzes market data within trading windows (30-60 minutes) and provides weighted consensus predictions.

---

## Use Case Description

### **Application Domain**

```
Type: Financial Market Analysis
Data: Generic finance time-series (cryptocurrency, stocks, commodities, etc.)
Pattern: Intraday analysis with rolling windows
Frequency: Continuous during market hours
Decision Window: 30-60 minutes from data collection to action
```

### **Typical Workflow**

```
1. Data Collection Window (60 minutes)
   ├─ OHLCV data (price/volume)
   ├─ Order book snapshots
   ├─ Market news/events
   └─ Technical indicators

2. Analysis Trigger (window closes)
   ├─ Prepare context (~20-35k tokens)
   ├─ Dispatch to ensemble (7 models)
   └─ Processing time: 3-4 minutes

3. Weighted Consensus (10-15 seconds)
   ├─ Cross-check model outputs
   ├─ Rank by confidence + model weight
   └─ Final LLM API synthesis

4. Decision Output (within 30-60 min window)
   ├─ Trading signal
   ├─ Confidence level
   └─ Supporting analysis

Total cycle: ~7-10 minutes (leaves 50+ min buffer)
```

---

## Architecture Configuration

### **Heterogeneous Ensemble Composition**

```yaml
Large Models (3× 7-9B parameters):
  - Gemma 7B (9B): High-quality analysis
  - Mistral 7B (7.3B): Alternative perspective
  - Qwen 7B (7B): Diverse training corpus

Small Models (4× <5B parameters):
  - Gemma 4B (4B): Fast validation
  - Phi-3 Mini (3.8B): Quick baseline
  - Gemma 2B (2.5B): Rapid check
  - Qwen 2.5B (2.5B): Secondary validator

Total: 7 heterogeneous models
Quantization: INT8 across all models
```

### **VM Distribution**

```
Worker Cluster (Machine 1):
├─ VM1 (NUMA Node 0): 16 cores, 118GB RAM
│   ├─ Gemma 7B (8 threads)
│   ├─ Mistral 7B (8 threads)
│   ├─ Gemma 4B (8 threads)
│   └─ Phi-3 Mini (8 threads)
│
└─ VM2 (NUMA Node 1): 16 cores, 118GB RAM
    ├─ Qwen 7B (10 threads)
    ├─ Gemma 2B (11 threads)
    └─ Qwen 2.5B (11 threads)

Data Server (Machine 2):
├─ K3s control plane
├─ Temporal orchestration
├─ Data lake (8TB HDD)
└─ Market data ingestion
```

---

## Context Size Strategy

### **Recommended: 25,000 Tokens**

```yaml
Default Context: 25k tokens

Rationale:
├─ Fits 90% of finance jobs (20-35k typical)
├─ Processing time: ~3.5 minutes
├─ Memory usage: 86GB / 236GB (36%)
├─ Parallel jobs: 2-3 symbols simultaneously
└─ Excellent balance of speed and capacity

Performance:
├─ Total cycle time: ~7-10 minutes
├─ Buffer in 60-min window: 50+ minutes
├─ Can analyze multiple symbols in parallel
└─ Fast enough for iterative backtesting
```

### **Fallback: 50,000 Tokens**

```yaml
Extended Context: 50k tokens (special cases)

Use When:
├─ High volatility events (unusual patterns)
├─ Earnings announcements (dense information)
├─ Market regime changes (deep historical context)
├─ Multi-day weekend analysis
└─ Frequency: 5-10% of jobs

Performance:
├─ Processing time: ~7 minutes
├─ Memory usage: 132GB / 236GB (56%)
├─ Parallel jobs: 1-2 max (tighter memory)
└─ Still fits in 30-60 minute window
```

---

## Token Budget Allocation

### **Finance Context Structure (25k tokens)**

```python
TOKEN_BUDGET = {
    # Current Market State (Priority 1)
    "current_ohlcv": 3000,           # Price/volume in analysis window
    "order_book": 4000,              # Market depth snapshots
    
    # Information Flow (Priority 2)
    "recent_news": 6000,             # Market-moving news/events
    
    # Technical Analysis (Priority 3)
    "indicators": 2000,              # RSI, MACD, Bollinger, etc.
    "volatility_metrics": 2000,      # ATR, realized vol, regime
    
    # Historical Context (Priority 4)
    "pattern_matching": 5000,        # Similar historical scenarios
    "correlation_data": 3000,        # Related symbols/markets
    
    # Total: 25,000 tokens
}

def prepare_finance_context(symbol, window_minutes=60):
    """
    Prepare market data context for ensemble analysis.
    """
    context = {
        "metadata": {
            "symbol": symbol,
            "window_minutes": window_minutes,
            "timestamp": datetime.now(),
            "market_phase": detect_market_phase()
        }
    }
    
    # 1. Current window OHLCV (highest priority)
    context["current_prices"] = format_ohlcv(
        data=fetch_ohlcv(symbol, window_minutes),
        max_tokens=3000,
        include_volume_profile=True
    )
    
    # 2. Order book (microstructure signals)
    context["market_depth"] = format_order_book(
        snapshots=fetch_order_book_snapshots(symbol, window_minutes),
        max_tokens=4000,
        aggregate_levels=10  # Top 10 levels
    )
    
    # 3. News and events (sentiment/catalysts)
    context["news_events"] = format_news(
        news=fetch_recent_news(symbol, hours=6),
        max_tokens=6000,
        time_decay=True,  # Recent news weighted higher
        sentiment_analysis=True
    )
    
    # 4. Technical indicators (derived metrics)
    context["technicals"] = calculate_indicators(
        data=fetch_ohlcv(symbol, days=30),
        indicators=["RSI", "MACD", "BB", "EMA", "Volume"],
        max_tokens=2000,
        summary_format=True
    )
    
    # 5. Volatility and risk metrics
    context["volatility"] = {
        "realized_vol": calculate_realized_volatility(symbol, days=30),
        "implied_vol": fetch_implied_volatility(symbol) if available else None,
        "vol_regime": classify_volatility_regime(symbol),
        "risk_metrics": calculate_risk_metrics(symbol)
    }  # ~2000 tokens
    
    # 6. Historical pattern matching
    context["patterns"] = find_similar_patterns(
        current=context["current_prices"],
        historical=fetch_historical_data(symbol, days=90),
        max_tokens=5000,
        top_n_matches=5
    )
    
    # 7. Correlation and cross-asset analysis
    context["correlations"] = analyze_correlations(
        symbol=symbol,
        related_symbols=get_related_symbols(symbol),
        max_tokens=3000,
        include_lead_lag=True
    )
    
    return context  # ~25k tokens total
```

---

## Weighted Ranking System

### **Model Weights**

```python
# Weight configuration
MODEL_WEIGHTS = {
    # Large models: Higher weight for critical decisions
    "gemma-7b": 3.0,
    "mistral-7b": 3.0,
    "qwen-7b": 3.0,
    
    # Small models: Validation and speed
    "gemma-4b": 1.5,
    "phi-3-mini": 1.5,
    "gemma-2b": 1.0,
    "qwen-2.5b": 1.0
}

# Influence distribution
# Large models: 9.0 / 14.0 = 64% total influence
# Small models: 5.0 / 14.0 = 36% total influence
```

### **Consensus Calculation**

```python
def calculate_weighted_consensus(ensemble_results):
    """
    Compute weighted consensus with confidence scores.
    """
    weighted_predictions = []
    
    for model_name, result in ensemble_results.items():
        base_weight = MODEL_WEIGHTS[model_name]
        confidence = result.get("confidence", 0.5)  # Model's own confidence
        
        # Final weight = base × confidence
        final_weight = base_weight * confidence
        
        weighted_predictions.append({
            "model": model_name,
            "model_size": "large" if base_weight >= 3.0 else "small",
            "prediction": result["prediction"],
            "direction": result["direction"],  # "bullish", "bearish", "neutral"
            "base_weight": base_weight,
            "confidence": confidence,
            "final_weight": final_weight
        })
    
    # Normalize weights
    total_weight = sum(p["final_weight"] for p in weighted_predictions)
    for pred in weighted_predictions:
        pred["normalized_influence"] = pred["final_weight"] / total_weight
    
    # Calculate consensus metrics
    consensus = {
        "predictions": weighted_predictions,
        "large_model_agreement": check_large_model_consensus(weighted_predictions),
        "small_model_agreement": check_small_model_consensus(weighted_predictions),
        "overall_direction": compute_weighted_direction(weighted_predictions),
        "confidence_level": compute_overall_confidence(weighted_predictions),
        "agreement_strength": compute_agreement_strength(weighted_predictions)
    }
    
    return consensus

def check_large_model_consensus(predictions):
    """
    Check if all large models agree (critical for automated decisions).
    """
    large_preds = [p for p in predictions if p["model_size"] == "large"]
    
    if len(large_preds) < 2:
        return None
    
    # Check directional agreement
    directions = [p["direction"] for p in large_preds]
    most_common = max(set(directions), key=directions.count)
    agreement_pct = directions.count(most_common) / len(directions)
    
    return {
        "agree": agreement_pct >= 0.67,  # At least 2 of 3 agree
        "direction": most_common if agreement_pct >= 0.67 else "mixed",
        "agreement_pct": agreement_pct
    }
```

---

## Decision Thresholds

### **Risk Management Rules**

```python
# Confidence thresholds for different action types
DECISION_THRESHOLDS = {
    "automated_execution": {
        "min_confidence": 0.85,           # 85% ensemble agreement
        "require_large_consensus": True,  # All 3 large models must agree
        "min_influence": 0.70             # 70%+ weighted influence
    },
    
    "alert_human": {
        "min_confidence": 0.70,           # 70% ensemble agreement
        "require_large_consensus": False,
        "min_influence": 0.60
    },
    
    "informational": {
        "min_confidence": 0.50,           # 50% agreement
        "require_large_consensus": False,
        "min_influence": 0.40
    }
}

def evaluate_trading_decision(consensus):
    """
    Apply risk thresholds to ensemble consensus.
    
    High-stakes financial decisions require high confidence.
    """
    confidence = consensus["confidence_level"]
    large_consensus = consensus["large_model_agreement"]
    
    # Check for automated execution criteria
    if (confidence >= 0.85 and 
        large_consensus["agree"] and 
        large_consensus["agreement_pct"] >= 0.67):
        
        return {
            "action": "execute",
            "automated": True,
            "confidence": "high",
            "direction": consensus["overall_direction"],
            "reasoning": "Strong consensus across all models, large models unanimous"
        }
    
    # Medium confidence - alert human
    elif confidence >= 0.70:
        return {
            "action": "alert",
            "automated": False,
            "confidence": "medium",
            "direction": consensus["overall_direction"],
            "reasoning": "Moderate consensus, human review recommended"
        }
    
    # Low confidence - informational only
    else:
        return {
            "action": "inform",
            "automated": False,
            "confidence": "low",
            "direction": "uncertain",
            "reasoning": "Insufficient agreement across ensemble"
        }
```

---

## Performance Specifications

### **Processing Timeline (25k Context)**

```
┌────────────────────────────────────────────────────┐
│  Single Symbol Analysis (60-minute window)         │
└────────────────────────────────────────────────────┘

00:00 - Market window closes
00:00 - Data collection & preparation
        ├─ Fetch OHLCV, order book, news
        ├─ Calculate indicators
        ├─ Format into 25k token context
        └─ Duration: 2-3 minutes

02:30 - Dispatch to ensemble (7 models, parallel)
        ├─ VM1: 4 models processing
        └─ VM2: 3 models processing

04:30 - Small models complete (Gemma-2B, Qwen-2.5B, Gemma-4B, Phi-3)
06:00 - Large models complete (Gemma-7B, Mistral-7B, Qwen-7B)
06:05 - Cross-check predictions
06:15 - Weighted consensus calculation
06:30 - LLM API final synthesis
07:00 - Write results to data lake

07:00 - Decision ready

Total: ~7 minutes from window close
Buffer: 53 minutes remaining (in 60-min window) ✅
Suitable for: 30-60 minute decision windows ✅
```

### **Multi-Symbol Parallel Processing**

```
Memory at 25k context: 86GB per job

Parallel capacity:
├─ Symbol 1 (BTC): 86GB
├─ Symbol 2 (ETH): 86GB
└─ Total: 172GB / 236GB = 73% usage ✅

Can analyze 2-3 symbols simultaneously
└─ All complete in ~7 minutes (not 14!)

Example: Portfolio analysis
├─ BTC, ETH, SOL analyzed in parallel
├─ Total time: 7 minutes
└─ Results ready for all 3 symbols simultaneously
```

---

## Memory Requirements

### **Resource Allocation by Context Size**

| Context | Memory per Job | Parallel Jobs | Total Usage | Headroom |
|---------|----------------|---------------|-------------|----------|
| **6k** | 52GB | 4 | 208GB (88%) | 28GB |
| **12k** | 63GB | 3 | 189GB (80%) | 47GB |
| **25k** | 86GB | 2 | 172GB (73%) | 64GB ✅ |
| **50k** | 132GB | 1 | 132GB (56%) | 104GB |

**Recommended configuration:** 25k context with 2 parallel jobs

---

## Kubernetes Deployment

### **ConfigMap: Finance Ensemble**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: finance-slm-ensemble
  namespace: finance-analysis
data:
  # Context configuration
  default_context_size: "25000"
  max_context_size: "50000"
  auto_expand_threshold: "0.90"  # Switch to 50k if >90% of 25k used
  
  # Timing
  market_window_minutes: "60"
  max_processing_time_seconds: "300"    # 5 minutes hard limit
  decision_window_minutes: "30"         # Min time for decision
  
  # Model configuration
  large_models: |
    - name: gemma-7b
      size: 9B
      weight: 3.0
      quantization: int8
    - name: mistral-7b
      size: 7.3B
      weight: 3.0
      quantization: int8
    - name: qwen-7b
      size: 7B
      weight: 3.0
      quantization: int8
  
  small_models: |
    - name: gemma-4b
      size: 4B
      weight: 1.5
      quantization: int8
    - name: phi-3-mini
      size: 3.8B
      weight: 1.5
      quantization: int8
    - name: gemma-2b
      size: 2.5B
      weight: 1.0
      quantization: int8
    - name: qwen-2.5b
      size: 2.5B
      weight: 1.0
      quantization: int8
  
  # Decision thresholds
  confidence_threshold_execute: "0.85"
  confidence_threshold_alert: "0.70"
  require_large_model_consensus: "true"
  
  # Resource limits
  max_concurrent_symbols: "2"
  queue_strategy: "priority"      # High-priority symbols first
  enable_parallel_processing: "true"
```

### **StatefulSet: SLM Ensemble Pods**

```yaml
---
# VM1: 4 models (2 large, 2 small)
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: finance-slm-vm1
  namespace: finance-analysis
spec:
  serviceName: finance-slm-vm1
  replicas: 4
  selector:
    matchLabels:
      app: finance-slm
      vm: vm1
  template:
    metadata:
      labels:
        app: finance-slm
        vm: vm1
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kubernetes.io/hostname
                operator: In
                values: ["k3s-worker-1"]
      
      containers:
      - name: slm
        image: finance-slm:latest
        
        resources:
          requests:
            memory: "20Gi"
            cpu: "8"
          limits:
            memory: "22Gi"
            cpu: "8"
        
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        
        - name: MODEL_NAME
          value: "$(POD_NAME)"  # Will be slm-0, slm-1, slm-2, slm-3
        
        - name: CONTEXT_SIZE
          valueFrom:
            configMapKeyRef:
              name: finance-slm-ensemble
              key: default_context_size
        
        - name: QUANTIZATION
          value: "int8"
        
        - name: THREADS
          value: "8"
        
        - name: TEMPORAL_SERVER
          value: "temporal-server:7233"
        
        volumeMounts:
        - name: staging
          mountPath: /data/local
        
        - name: models
          mountPath: /models
      
      volumes:
      - name: staging
        hostPath:
          path: /data/local
          type: Directory
      
      - name: models
        persistentVolumeClaim:
          claimName: model-storage

---
# VM2: 3 models (1 large, 2 small)
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: finance-slm-vm2
  namespace: finance-analysis
spec:
  serviceName: finance-slm-vm2
  replicas: 3
  selector:
    matchLabels:
      app: finance-slm
      vm: vm2
  template:
    metadata:
      labels:
        app: finance-slm
        vm: vm2
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kubernetes.io/hostname
                operator: In
                values: ["k3s-worker-2"]
      
      containers:
      - name: slm
        resources:
          requests:
            memory: "20Gi"
            cpu: "10"
          limits:
            memory: "22Gi"
            cpu: "10"
        
        env:
        - name: THREADS
          value: "10"
        
        # (Other env vars same as VM1)
```

---

## Monitoring and Observability

### **Key Metrics**

```yaml
Performance Metrics:
├─ processing_time_seconds: Job duration (target: <300s)
├─ consensus_confidence: Ensemble agreement (0-1)
├─ large_model_agreement: Large model consensus (boolean)
├─ memory_usage_gb: Per-job memory consumption
└─ parallel_jobs_count: Concurrent symbol analysis

Business Metrics:
├─ decisions_per_hour: Throughput
├─ automated_execution_rate: Auto vs manual
├─ prediction_accuracy: Backtested performance
└─ latency_to_decision: Time from window close to action

System Health:
├─ staging_disk_usage_percent: Worker storage health
├─ model_load_time_seconds: Startup performance
├─ api_call_latency_ms: External LLM API response time
└─ error_rate: Failed jobs per hour
```

### **Prometheus Metrics Example**

```python
from prometheus_client import Counter, Histogram, Gauge

# Processing metrics
processing_time = Histogram(
    'finance_ensemble_processing_seconds',
    'Time to process ensemble inference',
    ['symbol', 'context_size']
)

consensus_confidence = Gauge(
    'finance_ensemble_consensus_confidence',
    'Weighted consensus confidence level',
    ['symbol']
)

automated_decisions = Counter(
    'finance_ensemble_automated_decisions_total',
    'Number of automated trading decisions',
    ['symbol', 'direction']
)

# Usage
with processing_time.labels(symbol="BTC", context_size="25k").time():
    result = process_ensemble(symbol="BTC")

consensus_confidence.labels(symbol="BTC").set(result["confidence"])

if result["automated"]:
    automated_decisions.labels(
        symbol="BTC",
        direction=result["direction"]
    ).inc()
```

---

## Backtesting and Validation

### **Historical Performance Testing**

```python
def backtest_ensemble(historical_data, start_date, end_date):
    """
    Test ensemble performance on historical data.
    """
    results = []
    
    for window in generate_windows(start_date, end_date, window_size=60):
        # Prepare historical context
        context = prepare_finance_context(
            symbol=window.symbol,
            window_minutes=60,
            timestamp=window.end_time
        )
        
        # Run ensemble (offline)
        ensemble_prediction = run_ensemble_inference(context)
        
        # Compare to actual outcome
        actual_outcome = get_actual_outcome(
            symbol=window.symbol,
            prediction_time=window.end_time,
            lookback_minutes=30
        )
        
        # Record accuracy
        results.append({
            "timestamp": window.end_time,
            "symbol": window.symbol,
            "predicted": ensemble_prediction["direction"],
            "actual": actual_outcome["direction"],
            "confidence": ensemble_prediction["confidence"],
            "correct": ensemble_prediction["direction"] == actual_outcome["direction"]
        })
    
    # Calculate metrics
    accuracy = sum(r["correct"] for r in results) / len(results)
    
    # Accuracy by confidence level
    high_conf = [r for r in results if r["confidence"] >= 0.85]
    high_conf_accuracy = sum(r["correct"] for r in high_conf) / len(high_conf)
    
    return {
        "overall_accuracy": accuracy,
        "high_confidence_accuracy": high_conf_accuracy,
        "total_predictions": len(results),
        "automated_count": len(high_conf)
    }
```

---

## Cost Analysis

### **Computational Costs**

```
Per inference job (25k context):
├─ Processing: 3.5 minutes × 7 models
├─ CPU time: ~24.5 model-minutes
├─ Memory: 86GB peak
└─ LLM API call: 1× (external cost)

Daily throughput (16 market hours):
├─ Jobs per hour: 8-10 (with 2 parallel)
├─ Daily capacity: ~128-160 jobs
├─ Per-symbol coverage: 60-min rolling windows
└─ Can track: 2-3 symbols continuously

Monthly operational cost estimate:
├─ Infrastructure: $0 (owned hardware)
├─ Power: ~200W × 24h × 30d = 144 kWh = $15-20
├─ LLM API: $0.01-0.10 per call × 3,840 calls = $38-384
└─ Total: ~$53-404/month

Cost per decision: $0.01-0.10 (mostly LLM API)
```

---

## Best Practices

### **Operational Guidelines**

1. **Start with 25k context**
   - Covers 90% of typical finance workloads
   - Expand to 50k only for special events

2. **Monitor large model consensus**
   - Require all 3 large models to agree for automated actions
   - Flag disagreements for human review

3. **Use confidence thresholds**
   - ≥85%: Automated execution
   - 70-85%: Alert human
   - <70%: Informational only

4. **Track accuracy over time**
   - Regular backtesting on historical data
   - Adjust thresholds based on performance

5. **Parallel processing for efficiency**
   - Analyze 2-3 symbols simultaneously
   - Better throughput during market hours

6. **Prepare for volatility spikes**
   - Auto-expand to 50k context when needed
   - Maintain memory headroom (>30%)

---

## Troubleshooting

### **Common Issues**

**Issue: Processing time exceeds 5 minutes**
```
Likely cause: Context size too large or system overload

Diagnosis:
├─ Check context size (should be ≤25k for standard jobs)
├─ Verify memory availability (df -h /data/local)
├─ Check CPU usage (top -H)
└─ Review parallel job count

Fix:
├─ Reduce context size if >25k
├─ Serialize jobs if memory tight
└─ Consider moving to 50k for high-priority only
```

**Issue: Low consensus confidence**
```
Likely cause: Market uncertainty or data quality

Diagnosis:
├─ Review ensemble predictions (check for divergence)
├─ Verify data quality (missing OHLCV, news gaps)
├─ Check market volatility (high vol = lower confidence)
└─ Review historical similar scenarios

Action:
├─ Flag for human review (don't automate)
├─ Investigate data pipeline issues
└─ Consider adding more historical context
```

**Issue: Large models disagree**
```
Likely cause: Genuinely uncertain market conditions

Diagnosis:
├─ Check if small models also disagree
├─ Review market news for conflicting signals
├─ Verify technical indicators (mixed signals)
└─ Check for unusual market events

Action:
├─ Never automate when large models disagree
├─ Alert human trader
├─ Provide both perspectives in output
└─ Flag as "high uncertainty" scenario
```

---

## Summary

**This finance-specific deployment provides:**

✅ **Fast Processing:** 3-4 minutes for typical jobs (25k context)  
✅ **High Quality:** 64% influence from large models  
✅ **Parallel Efficiency:** 2-3 symbols analyzed simultaneously  
✅ **Risk Management:** Strict thresholds for automated decisions  
✅ **Scalability:** Can handle 128-160 jobs/day per cluster  
✅ **Cost Effective:** ~$0.01-0.10 per decision  
✅ **Reliable:** Weighted consensus with confidence scoring  

**Perfect for financial analysis workloads requiring fast, high-quality predictions within 30-60 minute decision windows!** 📈🎯

---

## References

- Base architecture: `docs/architecture/distributed-architecture.md`
- Workflow implementation: `docs/guides/distributed-workflow.md`
- VM specifications: `docs/architecture/vm-specifications.md`
- Storage design: `docs/architecture/storage-design.md`

