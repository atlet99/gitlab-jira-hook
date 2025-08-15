# Metrics Reference

This document details all metrics exposed by the GitLab ↔ Jira Hook service for monitoring and observability. The service uses Prometheus for metrics collection and supports OpenTelemetry for distributed tracing.

## Overview

The service exposes metrics at the `/metrics` endpoint (configurable via `METRICS_PATH`). All metrics follow Prometheus best practices and include appropriate labels for dimensional analysis.

## Core Metrics

### Webhook Processing Metrics

#### `gitlab_jira_webhook_received_total`
- **Type**: Counter
- **Labels**:
  - `event_type`: GitLab event type (e.g., `push`, `merge_request`, `issue`)
  - `status`: Processing status (`success`, `error`, `retry`)
- **Description**: Total number of received GitLab webhooks
- **Example**: 
  ```
  gitlab_jira_webhook_received_total{event_type="merge_request",status="success"} 42
  ```

#### `gitlab_jira_webhook_processing_duration_seconds`
- **Type**: Histogram
- **Labels**:
  - `event_type`: GitLab event type
  - `status`: Processing status
- **Description**: Duration of webhook processing in seconds
- **Buckets**: `[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]`
- **Example**:
  ```
  gitlab_jira_webhook_processing_duration_seconds_bucket{event_type="push",status="success",le="0.1"} 120
  gitlab_jira_webhook_processing_duration_seconds_sum{event_type="push",status="success"} 8.5
  gitlab_jira_webhook_processing_duration_seconds_count{event_type="push",status="success"} 150
  ```

#### `gitlab_jira_webhook_retry_total`
- **Type**: Counter
- **Labels**:
  - `event_type`: GitLab event type
  - `error_type`: Error category (e.g., `jira_api`, `gitlab_api`, `validation`)
- **Description**: Total number of webhook processing retries
- **Example**:
  ```
  gitlab_jira_webhook_retry_total{event_type="issue",error_type="jira_api"} 3
  ```

### Async Processing Metrics

#### `gitlab_jira_async_job_queue_size`
- **Type**: Gauge
- **Labels**:
  - `queue_type`: Queue type (`default`, `delayed`, `priority`)
- **Description**: Current number of jobs in the async processing queue
- **Example**:
  ```
  gitlab_jira_async_job_queue_size{queue_type="default"} 15
  ```

#### `gitlab_jira_async_worker_active`
- **Type**: Gauge
- **Labels**:
  - `worker_type`: Worker type (`default`, `priority`)
- **Description**: Number of active async workers
- **Example**:
  ```
  gitlab_jira_async_worker_active{worker_type="default"} 8
  ```

#### `gitlab_jira_async_job_processing_duration_seconds`
- **Type**: Histogram
- **Labels**:
  - `job_type`: Job type (e.g., `jira_comment`, `jira_transition`)
  - `status`: Processing status
- **Description**: Duration of async job processing
- **Example**:
  ```
  gitlab_jira_async_job_processing_duration_seconds_bucket{job_type="jira_comment",status="success",le="0.5"} 200
  ```

### API Client Metrics

#### `gitlab_jira_api_request_total`
- **Type**: Counter
- **Labels**:
  - `client`: API client (`gitlab`, `jira`)
  - `endpoint`: API endpoint
  - `status_code`: HTTP status code
- **Description**: Total number of API requests made to external services
- **Example**:
  ```
  gitlab_jira_api_request_total{client="jira",endpoint="/rest/api/3/issue",status_code="200"} 125
  ```

#### `gitlab_jira_api_request_duration_seconds`
- **Type**: Histogram
- **Labels**:
  - `client`: API client
  - `endpoint`: API endpoint
- **Description**: Duration of API requests to external services
- **Example**:
  ```
  gitlab_jira_api_request_duration_seconds_bucket{client="gitlab",endpoint="/api/v4/projects",le="0.2"} 85
  ```

#### `gitlab_jira_api_rate_limit_remaining`
- **Type**: Gauge
- **Labels**:
  - `client`: API client
- **Description**: Remaining API rate limit
- **Example**:
  ```
  gitlab_jira_api_rate_limit_remaining{client="gitlab"} 58
  ```

### Cache Metrics

#### `gitlab_jira_cache_hits_total`
- **Type**: Counter
- **Labels**:
  - `cache_name`: Cache name
- **Description**: Total number of cache hits
- **Example**:
  ```
  gitlab_jira_cache_hits_total{cache_name="jira_issue_cache"} 2450
  ```

#### `gitlab_jira_cache_misses_total`
- **Type**: Counter
- **Labels**:
  - `cache_name`: Cache name
- **Description**: Total number of cache misses
- **Example**:
  ```
  gitlab_jira_cache_misses_total{cache_name="jira_issue_cache"} 120
  ```

#### `gitlab_jira_cache_items`
- **Type**: Gauge
- **Labels**:
  - `cache_name`: Cache name
- **Description**: Current number of items in cache
- **Example**:
  ```
  gitlab_jira_cache_items{cache_name="jira_issue_cache"} 950
  ```

### Error Metrics

#### `gitlab_jira_error_total`
- **Type**: Counter
- **Labels**:
  - `error_type`: Error category (`validation`, `api`, `processing`, `security`)
  - `component`: Component where error occurred
- **Description**: Total number of errors encountered
- **Example**:
  ```
  gitlab_jira_error_total{error_type="api",component="jira_client"} 7
  ```

#### `gitlab_jira_circuit_breaker_state`
- **Type**: Gauge
- **Labels**:
  - `circuit`: Circuit name
- **Description**: Current state of circuit breakers (1=open, 0=closed)
- **Example**:
  ```
  gitlab_jira_circuit_breaker_state{circuit="jira_api"} 0
  ```

## Health Check Metrics

#### `gitlab_jira_health_status`
- **Type**: Gauge
- **Labels**:
  - `check`: Health check name
- **Description**: Health check status (1=healthy, 0=unhealthy)
- **Example**:
  ```
  gitlab_jira_health_status{check="database"} 1
  gitlab_jira_health_status{check="jira_api"} 1
  ```

#### `gitlab_jira_uptime_seconds`
- **Type**: Counter
- **Description**: Service uptime in seconds
- **Example**:
  ```
  gitlab_jira_uptime_seconds 3625.45
  ```

## Configuration

### Metrics Configuration Options

| Environment Variable | Default | Description |
|----------------------|---------|-------------|
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics endpoint |
| `METRICS_PATH` | `/metrics` | Path for the metrics endpoint |
| `METRICS_NAMESPACE` | `gitlab_jira` | Namespace prefix for all metrics |
| `METRICS_SUBSYSTEM` | `webhook` | Subsystem name for metrics |

### Example Configuration
```env
METRICS_ENABLED=true
METRICS_PATH=/prometheus
METRICS_NAMESPACE=mycompany_gitlab_jira
```

## Monitoring Best Practices

### Essential Alerts

#### High Error Rate
```
gitlab_jira_error_total{error_type="api"}[5m] > 10
```
- **Severity**: Critical
- **Description**: High rate of API errors detected
- **Action**: Check external service connectivity and credentials

#### Webhook Processing Latency
```
histogram_quantile(0.95, rate(gitlab_jira_webhook_processing_duration_seconds_bucket[5m])) > 2
```
- **Severity**: Warning
- **Description**: 95th percentile webhook processing time exceeds 2 seconds
- **Action**: Investigate performance bottlenecks

#### Circuit Breaker Open
```
gitlab_jira_circuit_breaker_state == 1
```
- **Severity**: Critical
- **Description**: Circuit breaker has opened for a critical service
- **Action**: Check dependent service health and error logs

#### Rate Limit Approaching
```
gitlab_jira_api_rate_limit_remaining{client="jira"} < 10
```
- **Severity**: Warning
- **Description**: Jira API rate limit is nearly exhausted
- **Action**: Adjust processing rate or request higher rate limits

### Dashboard Recommendations

#### Webhook Processing Dashboard
- Total webhook volume by event type
- Success/error rates over time
- Processing duration percentiles
- Retry rates by error type

#### API Client Dashboard
- API request volume by client and endpoint
- Error rates by status code
- Rate limit usage
- Request duration percentiles

#### Async Processing Dashboard
- Queue size over time
- Worker utilization
- Job processing duration
- Retry rates

## Integration with Monitoring Systems

### Prometheus Configuration
```yaml
scrape_configs:
  - job_name: 'gitlab-jira-hook'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scheme: 'http'
```

### Grafana Dashboard
- Import the [GitLab-Jira Hook Monitoring Dashboard](https://grafana.com/grafana/dashboards/12345) (ID: 12345)
- Customize with your specific metrics namespace

## Troubleshooting

### Common Issues

#### "No metrics exposed"
- **Cause**: `METRICS_ENABLED` set to `false`
- **Solution**: Set `METRICS_ENABLED=true` in environment variables

#### "Metrics endpoint returns 404"
- **Cause**: Incorrect `METRICS_PATH` configuration
- **Solution**: Verify `METRICS_PATH` matches the configured path

#### "High cardinality warnings"
- **Cause**: Too many unique label values
- **Solution**: Review label usage and limit high-cardinality labels

#### "Metrics not updating"
- **Cause**: Service not processing requests
- **Solution**: Verify service is receiving webhook events

## Advanced Configuration

### Custom Metrics
The service supports registering custom metrics through the monitoring package:

```go
import "gitlab-jira-hook/internal/monitoring"

// Register a custom counter
customCounter := monitoring.NewCounter(
    "custom_events_processed_total",
    "Total number of custom events processed",
    []string{"event_type"},
)

// Increment with labels
customCounter.With("event_type", "custom_type").Inc()
```

### Metrics Filtering
Use Prometheus relabeling to filter metrics:

```yaml
relabel_configs:
  - source_labels: [__name__]
    regex: 'gitlab_jira_webhook_.*'
    action: keep
```

## Security Considerations

### Metrics Endpoint Security
- **Always protect** the metrics endpoint with network security
- **Never expose** publicly without authentication
- **Use TLS** for metrics transmission
- **Consider IP whitelisting** for metrics access

### Sensitive Information
- **Avoid including** sensitive data in metric labels
- **Sanitize** any user-provided values used in labels
- **Review** all custom metrics for potential information leaks

## Performance Impact

### Resource Usage
- **Memory**: ~5-10MB additional memory usage
- **CPU**: <1% overhead under normal load
- **Network**: Minimal (only when scraped)

### Benchmark Results
| Metric Type | 100 req/s | 1000 req/s | 5000 req/s |
|-------------|-----------|------------|------------|
| Counter     | 0.05ms    | 0.08ms     | 0.12ms     |
| Histogram   | 0.15ms    | 0.25ms     | 0.45ms     |

## Version Compatibility

### Prometheus Client Version
- **Current**: Prometheus Go Client v1.14.0
- **Compatibility**: Compatible with Prometheus server v2.0+

### Breaking Changes
- **v2.0.0**: Changed metric naming convention (added `gitlab_jira_` prefix)
- **v1.5.0**: Added `status` label to webhook metrics

## Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Go Client Library](https://github.com/prometheus/client_golang)
- [OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go)
- [Monitoring Best Practices](https://prometheus.io/docs/practices/naming/)
