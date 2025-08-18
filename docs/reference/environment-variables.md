# Environment Variables Reference

This document details all environment variables used by the GitLab ↔ Jira Hook service. Proper configuration of these variables is essential for the service to function correctly in different environments.

## Core Configuration

### `JIRA_WEBHOOK_SECRET`
- **Description**: Secret key used for HMAC-SHA256 signature validation of Jira webhooks
- **Required**: Yes (for production)
- **Default**: None
- **Example**: `JIRA_WEBHOOK_SECRET=your-secure-webhook-secret-here`
- **Security Note**: Must be a cryptographically secure random string (minimum 32 characters)

### `GITLAB_TOKEN`
- **Description**: Personal access token for GitLab API access
- **Required**: Yes
- **Default**: None
- **Example**: `GITLAB_TOKEN=glpat-abc123xyz456`
- **Permissions Required**: `api` scope for repository access

### `JIRA_BASE_URL`
- **Description**: Base URL of your Jira instance
- **Required**: Yes
- **Default**: None
- **Example**: `JIRA_BASE_URL=https://your-company.atlassian.net`
- **Note**: Must include protocol (https://)

### `JIRA_USER_EMAIL`
- **Description**: Email address of the Jira user for API authentication
- **Required**: Yes (for basic auth)
- **Default**: None
- **Example**: `JIRA_USER_EMAIL=user@example.com`

### `JIRA_API_TOKEN`
- **Description**: API token for Jira Cloud authentication
- **Required**: Yes (for basic auth)
- **Default**: None
- **Example**: `JIRA_API_TOKEN=your-atlassian-api-token`
- **Note**: Create tokens at https://id.atlassian.com/manage-profile/security/api-tokens

### `JQL_FILTER`
- **Description**: JQL filter to determine which events to process. If set, only events that match the JQL filter will be processed. If not set, all events will be processed.
- **Required**: No
- **Default**: None
- **Example**: `JQL_FILTER=project = "TEST" AND issuetype = Bug`

## Security Configuration

### `DEBUG_MODE`
- **Description**: Enables development mode with relaxed security
- **Required**: No
- **Default**: `false`
- **Example**: `DEBUG_MODE=true`
- **Warning**: Never enable in production environments

### `ALLOWED_GITLAB_IP_RANGES`
- **Description**: Comma-separated list of allowed GitLab IP ranges
- **Required**: No
- **Default**: All IPs allowed
- **Example**: `ALLOWED_GITLAB_IP_RANGES=34.74.12.0/24,35.202.128.0/24`
- **Note**: See GitLab's [IP ranges documentation](https://docs.gitlab.com/ee/user/gitlab_com/index.html#ip-range)

### `JWT_ENABLED`
- **Description**: Enable JWT validation for Jira Connect apps
- **Required**: No
- **Default**: `false`
- **Example**: `JWT_ENABLED=true`
- **Note**: Set to `true` when using Jira Connect apps

### `JWT_EXPECTED_AUDIENCE`
- **Description**: Expected audience for JWT tokens from Jira Connect apps
- **Required**: Yes (when `JWT_ENABLED=true`)
- **Default**: None
- **Example**: `JWT_EXPECTED_AUDIENCE=https://your-app.com/webhook`
- **Note**: Should match your webhook endpoint URL

### `JWT_ALLOWED_ISSUERS`
- **Description**: Comma-separated list of allowed Jira Connect app client keys
- **Required**: Yes (when `JWT_ENABLED=true`)
- **Default**: None
- **Example**: `JWT_ALLOWED_ISSUERS=client-key-1,client-key-2`
- **Note**: Required for JWT validation of Connect apps

## Server Configuration

### `SERVER_PORT`
- **Description**: Port for the HTTP server to listen on
- **Required**: No
- **Default**: `8080`
- **Example**: `SERVER_PORT=9000`

### `READ_TIMEOUT`
- **Description**: Maximum duration for reading the entire request
- **Required**: No
- **Default**: `15s`
- **Example**: `READ_TIMEOUT=30s`
- **Format**: Go duration string (e.g., "30s", "1m")

### `WRITE_TIMEOUT`
- **Description**: Maximum duration before timing out writes
- **Required**: No
- **Default**: `15s`
- **Example**: `WRITE_TIMEOUT=30s`

### `IDLE_TIMEOUT`
- **Description**: Maximum amount of time to wait for the next request
- **Required**: No
- **Default**: `60s`
- **Example**: `IDLE_TIMEOUT=120s`

## Async Processing Configuration

### `WORKER_POOL_SIZE`
- **Description**: Number of concurrent workers for async processing
- **Required**: No
- **Default**: `10`
- **Example**: `WORKER_POOL_SIZE=20`
- **Note**: Adjust based on system resources

### `MAX_QUEUE_SIZE`
- **Description**: Maximum number of jobs in the processing queue
- **Required**: No
- **Default**: `1000`
- **Example**: `MAX_QUEUE_SIZE=5000`

### `QUEUE_TIMEOUT`
- **Description**: Maximum time a job can wait in the queue
- **Required**: No
- **Default**: `5m`
- **Example**: `QUEUE_TIMEOUT=10m`

### `DELAYED_QUEUE_TTL`
- **Description**: Time-to-live for delayed queue items
- **Required**: No
- **Default**: `24h`
- **Example**: `DELAYED_QUEUE_TTL=48h`

## Cache Configuration

### `CACHE_TTL`
- **Description**: Default time-to-live for cache entries
- **Required**: No
- **Default**: `5m`
- **Example**: `CACHE_TTL=10m`

### `CACHE_JITTER`
- **Description**: Random jitter percentage to prevent cache stampedes
- **Required**: No
- **Default**: `10`
- **Example**: `CACHE_JITTER=20`
- **Note**: Value between 0-100 (percentage)

### `CACHE_MAX_ITEMS`
- **Description**: Maximum number of items in the cache
- **Required**: No
- **Default**: `1000`
- **Example**: `CACHE_MAX_ITEMS=5000`

## Monitoring Configuration

### `METRICS_ENABLED`
- **Description**: Enable Prometheus metrics endpoint
- **Required**: No
- **Default**: `true`
- **Example**: `METRICS_ENABLED=false`

### `METRICS_PATH`
- **Description**: Path for the metrics endpoint
- **Required**: No
- **Default**: `/metrics`
- **Example**: `METRICS_PATH=/prometheus`

### `TRACING_ENABLED`
- **Description**: Enable distributed tracing
- **Required**: No
- **Default**: `false`
- **Example**: `TRACING_ENABLED=true`

### `TRACING_SAMPLING_RATE`
- **Description**: Sampling rate for traces (0.0 to 1.0)
- **Required**: No
- **Default**: `0.1`
- **Example**: `TRACING_SAMPLING_RATE=0.5`

## Logging Configuration

### `LOG_LEVEL`
- **Description**: Minimum log level to output
- **Required**: No
- **Default**: `info`
- **Options**: `debug`, `info`, `warn`, `error`, `fatal`
- **Example**: `LOG_LEVEL=debug`

### `LOG_FORMAT`
- **Description**: Log output format
- **Required**: No
- **Default**: `json`
- **Options**: `json`, `text`
- **Example**: `LOG_FORMAT=text`

### `LOG_INCLUDE_SOURCE`
- **Description**: Include source file and line number in logs
- **Required**: No
- **Default**: `false`
- **Example**: `LOG_INCLUDE_SOURCE=true`

## Advanced Configuration

### `CONFIG_HOT_RELOAD`
- **Description**: Enable configuration hot-reloading
- **Required**: No
- **Default**: `true`
- **Example**: `CONFIG_HOT_RELOAD=false`
- **Note**: Requires configuration file monitoring

### `CONFIG_FILE_PATH`
- **Description**: Path to configuration file
- **Required**: No
- **Default**: `config.yaml`
- **Example**: `CONFIG_FILE_PATH=/etc/gitlab-jira-hook/config.yaml`

### `MAX_REQUEST_SIZE`
- **Description**: Maximum size of incoming requests
- **Required**: No
- **Default**: `4MB`
- **Example**: `MAX_REQUEST_SIZE=8MB`

### `RATE_LIMIT_ENABLED`
- **Description**: Enable rate limiting for webhook endpoints
- **Required**: No
- **Default**: `true`
- **Example**: `RATE_LIMIT_ENABLED=false`

### `RATE_LIMIT_PER_SECOND`
- **Description**: Requests per second per IP
- **Required**: No
- **Default**: `10`
- **Example**: `RATE_LIMIT_PER_SECOND=20`

## Development Configuration

### `DEV_MODE`
- **Description**: Enable development-specific features
- **Required**: No
- **Default**: `false`
- **Example**: `DEV_MODE=true`
- **Note**: Not for production use

### `MOCK_GITLAB`
- **Description**: Use mock GitLab API responses
- **Required**: No
- **Default**: `false`
- **Example**: `MOCK_GITLAB=true`

### `MOCK_JIRA`
- **Description**: Use mock Jira API responses
- **Required**: No
- **Default**: `false`
- **Example**: `MOCK_JIRA=true`

## Example Configuration File

```env
# Production configuration example
JIRA_WEBHOOK_SECRET=your-secure-secret-1234567890abcdef
GITLAB_TOKEN=glpat-abc123xyz456
JIRA_BASE_URL=https://your-company.atlassian.net
JIRA_USER_EMAIL=service-account@your-company.com
JIRA_API_TOKEN=your-atlassian-api-token

# JWT configuration for Connect apps (optional)
JWT_ENABLED=false
JWT_EXPECTED_AUDIENCE=https://your-app.com/webhook
JWT_ALLOWED_ISSUERS=client-key-1,client-key-2

SERVER_PORT=8080
WORKER_POOL_SIZE=20
CACHE_TTL=10m
LOG_LEVEL=info
METRICS_ENABLED=true
```

## Security Best Practices

1. **Never commit secrets to version control**
   - Always use `.env` files excluded from git
   - Add `.env` to your `.gitignore`

2. **Use secret management systems in production**
   - AWS Secrets Manager
   - HashiCorp Vault
   - Kubernetes Secrets

3. **Rotate secrets regularly**
   - Set up a schedule for secret rotation
   - Monitor for secret leaks

4. **Restrict environment variable access**
   - Use file permissions to restrict access
   - Limit who can view environment variables

## Troubleshooting

### Common Issues

#### "HMAC signature validation failed"
- **Cause**: Mismatch between Jira webhook secret and `JIRA_WEBHOOK_SECRET`
- **Solution**: Verify both values match exactly

#### "JWT validation failed: invalid issuer"
- **Cause**: Jira Connect app client key not in `JWT_ALLOWED_ISSUERS`
- **Solution**: Add the client key to the allowed issuers list

#### "JWT_EXPECTED_AUDIENCE is required when JWT_ENABLED is true"
- **Cause**: JWT validation is enabled but no audience is configured
- **Solution**: Set `JWT_EXPECTED_AUDIENCE` to your webhook endpoint URL

#### "JWT_ALLOWED_ISSUERS is required when JWT_ENABLED is true"
- **Cause**: JWT validation is enabled but no allowed issuers are configured
- **Solution**: Set `JWT_ALLOWED_ISSUERS` to your Jira Connect app client keys

#### "Connection refused to Jira API"
- **Cause**: Incorrect `JIRA_BASE_URL` or network issues
- **Solution**: Verify URL format and network connectivity

#### "Rate limit exceeded"
- **Cause**: Too many requests from the same IP
- **Solution**: Adjust `RATE_LIMIT_PER_SECOND` or implement retry logic

## Validation

All environment variables are validated at startup. The service will fail to start if:
- Required variables are missing
- Variables have invalid formats
- Security-sensitive variables are insecure

Validation errors include detailed messages to help with configuration fixes.

## Dynamic Configuration

Some configuration values can be changed at runtime:
- `WORKER_POOL_SIZE` - Can be adjusted via hot-reload
- `CACHE_TTL` - Can be updated without restart
- `LOG_LEVEL` - Can be changed dynamically

See the [Configuration Hot-Reloading](../architecture/monitoring.md) documentation for details.
