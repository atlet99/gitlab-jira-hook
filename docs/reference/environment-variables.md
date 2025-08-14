# Environment Variables Reference

This document details all environment variables used by the GitLab ↔ Jira Hook service. Proper configuration of these variables is essential for correct operation.

## Core Configuration

### `JIRA_WEBHOOK_SECRET`
- **Description**: Secret key used for HMAC-SHA256 signature validation of incoming webhooks
- **Required**: Yes (in production)
- **Default**: None
- **Example**: `JIRA_WEBHOOK_SECRET=your-secure-random-32-byte-secret`
- **Security Note**: Must match the secret configured in Jira webhook settings. Never commit to version control.

### `DEBUG_MODE`
- **Description**: Enables development mode with relaxed security and detailed logging
- **Required**: No
- **Default**: `false`
- **Example**: `DEBUG_MODE=true`
- **Note**: Should be disabled in production environments.

### `PORT`
- **Description**: Port number for the HTTP server to listen on
- **Required**: No
- **Default**: `8080`
- **Example**: `PORT=9000`

### `LOG_LEVEL`
- **Description**: Logging verbosity level
- **Required**: No
- **Default**: `info`
- **Valid Values**: `debug`, `info`, `warn`, `error`
- **Example**: `LOG_LEVEL=debug`

## Jira Configuration

### `JIRA_BASE_URL`
- **Description**: Base URL of your Jira Cloud instance
- **Required**: Yes
- **Default**: None
- **Example**: `JIRA_BASE_URL=https://your-domain.atlassian.net`

### `JIRA_CONNECT_APP_KEY`
- **Description**: Key for Jira Connect app authentication
- **Required**: Yes (for Connect app integration)
- **Default**: None
- **Example**: `JIRA_CONNECT_APP_KEY=your-connect-app-key`

### `JIRA_WEBHOOK_SECRET`
- **Description**: Shared secret for Jira Connect app JWT validation
- **Required**: Yes (for Connect app integration)
- **Default**: None
- **Example**: `JIRA_WEBHOOK_SECRET=your-connect-app-secret`

## GitLab Configuration

### `GITLAB_BASE_URL`
- **Description**: Base URL of your GitLab instance
- **Required**: Yes
- **Default**: `https://gitlab.com`
- **Example**: `GITLAB_BASE_URL=https://gitlab.example.com`

### `GITLAB_TOKEN`
- **Description**: Personal access token for GitLab API access
- **Required**: Yes (for certain features)
- **Default**: None
- **Example**: `GITLAB_TOKEN=glpat-your-personal-access-token`
- **Permissions Required**: `api`, `read_api`, `read_repository`

## Cache Configuration

### `CACHE_STRATEGY`
- **Description**: Cache eviction strategy to use
- **Required**: No
- **Default**: `adaptive`
- **Valid Values**: `lru`, `lfu`, `fifo`, `ttl`, `adaptive`
- **Example**: `CACHE_STRATEGY=lru`

### `CACHE_TTL`
- **Description**: Time-to-live for cache entries (in seconds)
- **Required**: No
- **Default**: `3600` (1 hour)
- **Example**: `CACHE_TTL=7200`

### `CACHE_MAX_ITEMS`
- **Description**: Maximum number of items in cache
- **Required**: No
- **Default**: `10000`
- **Example**: `CACHE_MAX_ITEMS=5000`

## Rate Limiting

### `RATE_LIMIT`
- **Description**: Maximum requests per time window
- **Required**: No
- **Default**: `100`
- **Example**: `RATE_LIMIT=200`

### `RATE_LIMIT_WINDOW`
- **Description**: Time window for rate limiting (in seconds)
- **Required**: No
- **Default**: `60`
- **Example**: `RATE_LIMIT_WINDOW=30`

## Async Processing

### `WORKER_POOL_SIZE`
- **Description**: Number of worker goroutines for async processing
- **Required**: No
- **Default**: `10`
- **Example**: `WORKER_POOL_SIZE=20`

### `MAX_QUEUE_SIZE`
- **Description**: Maximum size of the job queue
- **Required**: No
- **Default**: `1000`
- **Example**: `MAX_QUEUE_SIZE=5000`

### `JOB_TIMEOUT`
- **Description**: Maximum time to process a job (in seconds)
- **Required**: No
- **Default**: `30`
- **Example**: `JOB_TIMEOUT=60`

## Error Recovery

### `MAX_RETRY_ATTEMPTS`
- **Description**: Maximum number of retry attempts for failed jobs
- **Required**: No
- **Default**: `3`
- **Example**: `MAX_RETRY_ATTEMPTS=5`

### `RETRY_BACKOFF_BASE`
- **Description**: Base delay for exponential backoff (in milliseconds)
- **Required**: No
- **Default**: `1000`
- **Example**: `RETRY_BACKOFF_BASE=2000`

### `RETRY_BACKOFF_MAX`
- **Description**: Maximum delay for exponential backoff (in milliseconds)
- **Required**: No
- **Default**: `30000` (30 seconds)
- **Example**: `RETRY_BACKOFF_MAX=60000`

## Security

### `ALLOWED_ISSUERS`
- **Description**: Comma-separated list of allowed JWT issuers
- **Required**: No
- **Default**: None
- **Example**: `ALLOWED_ISSUERS=issuer1,issuer2,issuer3`
- **Note**: Required for JWT validation with Connect apps.

### `EXPECTED_AUDIENCE`
- **Description**: Expected audience for JWT validation
- **Required**: No
- **Default**: None
- **Example**: `EXPECTED_AUDIENCE=https://your-app.com`
- **Note**: Required for JWT validation with Connect apps.

## Monitoring

### `PROMETHEUS_ENABLED`
- **Description**: Enable Prometheus metrics endpoint
- **Required**: No
- **Default**: `true`
- **Example**: `PROMETHEUS_ENABLED=false`

### `TRACING_ENABLED`
- **Description**: Enable distributed tracing
- **Required**: No
- **Default**: `true`
- **Example**: `TRACING_ENABLED=false`

### `TRACING_SAMPLING_RATE`
- **Description**: Sampling rate for distributed traces (0.0-1.0)
- **Required**: No
- **Default**: `0.1`
- **Example**: `TRACING_SAMPLING_RATE=0.5`

## Configuration Management

### `CONFIG_HOT_RELOAD`
- **Description**: Enable configuration hot-reloading
- **Required**: No
- **Default**: `true`
- **Example**: `CONFIG_HOT_RELOAD=false`

### `CONFIG_WATCH_INTERVAL`
- **Description**: Interval for checking configuration changes (in seconds)
- **Required**: No
- **Default**: `5`
- **Example**: `CONFIG_WATCH_INTERVAL=10`

## Development

### `TEST_ENV`
- **Description**: Flag indicating test environment
- **Required**: No
- **Default**: `false`
- **Example**: `TEST_ENV=true`
- **Note**: Automatically set by test framework.

## Best Practices

### Security Recommendations
- Store secrets in environment variables, not in code
- Use a secrets management system in production
- Rotate secrets regularly
- Restrict environment variable access to necessary processes

### Configuration Management
- Use `.env` files for local development (add to `.gitignore`)
- Create `config.env.example` with placeholder values
- Use consistent naming conventions (UPPER_SNAKE_CASE)
- Document all configuration options

### Example `.env` File
```env
# Core Configuration
JIRA_WEBHOOK_SECRET=your-secure-secret-here
DEBUG_MODE=false
PORT=8080
LOG_LEVEL=info

# Jira Configuration
JIRA_BASE_URL=https://your-domain.atlassian.net
JIRA_CONNECT_APP_KEY=your-connect-app-key
JIRA_WEBHOOK_SECRET=your-connect-app-secret

# GitLab Configuration
GITLAB_BASE_URL=https://gitlab.com
GITLAB_TOKEN=glpat-your-token-here

# Cache Configuration
CACHE_STRATEGY=adaptive
CACHE_TTL=3600
CACHE_MAX_ITEMS=10000
```

## Troubleshooting

### Common Issues

#### "No signature found in webhook headers"
- **Cause**: Jira webhook not configured with a secret
- **Solution**: Add `JIRA_WEBHOOK_SECRET` and configure secret in Jira

#### "HMAC signature validation failed"
- **Cause**: Secret mismatch between Jira and service
- **Solution**: Verify both sides use the same secret

#### "JWT validation failed: invalid issuer"
- **Cause**: JWT issuer not in `ALLOWED_ISSUERS`
- **Solution**: Add issuer to `ALLOWED_ISSUERS` list

#### "Connection refused to Jira API"
- **Cause**: Incorrect `JIRA_BASE_URL`
- **Solution**: Verify URL format (must include `https://`)

### Debugging Tips
- Enable debug logging: `LOG_LEVEL=debug`
- Verify environment variables are set: `printenv | grep JIRA`
- Test with sample requests using curl
- Check server logs for detailed error messages
