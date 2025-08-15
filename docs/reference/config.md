# Broker, Scheduler and Queue: Configuration and Tuning Formula

## Overview
This document describes the formulas and principles for configuring the async job broker, scheduler, and queue in the `gitlab-jira-hook` project. It provides practical recommendations for optimal performance, resource usage, and reliability.

---

## 1. Key Parameters
- **W** — Number of workers (goroutines processing jobs)
- **Q** — Queue size (max number of jobs in queue)
- **CPU** — Number of CPU cores available
- **MemoryLimitMB** — Memory limit for the process/container (in MB)
- **MemoryPerWorkerMB** — Estimated memory usage per worker (default: 128MB)
- **F** — Queue fill factor (recommended: 50–200)
- **AvgJobTime** — Average job processing time (seconds)
- **K** — Timeout safety factor (recommended: 2–3)

---

## 2. Worker Pool Sizing

```
W = min(CPU * M, MemoryLimitMB / MemoryPerWorkerMB)
```
- **M** — Worker multiplier (1–2, default: 2)
- Example: 4 CPU, 2048MB RAM → W = min(8, 16) = 8

---

## 3. Queue Sizing

```
Q = max(MinQueueSize, min(MaxQueueSize, W * F))
```
- **MinQueueSize** — Minimum queue size (default: 100)
- **MaxQueueSize** — Maximum queue size (default: 10,000)
- Example: W=8, F=100 → Q=800

---

## 4. Job Timeout Calculation

```
T_job = max(AvgJobTime * K, MinTimeout)
```
- **MinTimeout** — Minimum allowed timeout (default: 1s)
- Example: AvgJobTime=0.5s, K=3 → T_job=1.5s

---

## 5. Queue Timeout Calculation

```
T_queue = T_job * (Q / W)
```
- Example: T_job=1.5s, Q=800, W=8 → T_queue=150s

---

## 6. Scheduler Interval

- **SchedulerInterval** — How often the delayed job scheduler checks for ready jobs (default: 5ms–100ms)
- Should be much less than the minimum job delay to ensure timely execution.

---

## 7. Recommendations
- Always monitor CPU and memory usage under real load.
- Use auto-detection for W and Q if possible (see config auto-detect logic).
- For bursty workloads, increase Q and T_queue.
- For latency-sensitive workloads, decrease Q and T_queue, increase W if resources allow.
- Tune K based on observed job variance and SLA.
- Use structured logging and metrics for observability.

---

## 8. Architecture Principles
- **Dynamic Scaling:** Worker pool auto-scales between MinWorkers and MaxWorkers based on queue length and resource usage.
- **Priority Queue:** Jobs are prioritized using a decider interface; high-priority jobs are processed first.
- **Delayed Queue:** Supports scheduling jobs for future execution with millisecond precision.
- **Graceful Shutdown:** All queues and workers support graceful shutdown and draining.
- **Backpressure:** If the queue is full, new jobs are rejected with an error.
- **Retry/Backoff:** Failed jobs are retried with exponential backoff up to MaxRetries.
- **Observability:** All key metrics (queue length, worker count, job latency, error rate) are exported for monitoring.

---

## 9. Example Configuration

```
# .env or config
MIN_WORKERS=2
MAX_WORKERS=8
MAX_CONCURRENT_JOBS=100
JOB_TIMEOUT_SECONDS=10
QUEUE_TIMEOUT_MS=1000
MAX_RETRIES=3
RETRY_DELAY_MS=100
BACKOFF_MULTIPLIER=2.0
MAX_BACKOFF_MS=1000
HEALTH_CHECK_INTERVAL=5
```

---

## 10. References
- [Go Concurrency Patterns](https://go.dev/doc/effective_go#concurrency)
- [Go Context Package](https://pkg.go.dev/context)
- [Go Scheduler Design](https://github.com/golang/go/wiki/GoScheduler)
- [Async Job Processing Best Practices](https://12factor.net/backing-services)

---

## 11. See Also
- [docs/api/openapi.yaml](api/openapi.yaml) — API documentation
- [README.md](../README.md) — Project overview and quick start 