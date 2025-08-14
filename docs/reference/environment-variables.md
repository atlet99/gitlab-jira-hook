# Environment Variables

This document lists all environment variables used by the GitLab ↔ Jira Hook service, including their purpose, valid values, defaults, and usage contexts.

## Core Configuration

### `LOG_LEVEL`
- **Description**: Sets the logging verbosity level
- **Values**: `debug`, `info`, `warn`, `error`
- **Default**: `info`
- **Usage Context**:
  - Development: `debug`
  - Production: `info`
- **Example**: `LOG_LEVEL=debug`

### `DEBUG`
- **Description**: Enables debug mode with additional logging and features
- **Values**: `true`, `false`
- **Default**: `false`
- **Usage Context**:
  - Development: `true`
  - Production: `false`
- **Security Note**: Never enable in production environments
- **Example**: `DEBUG=true`

### `JIRA_WEBHOOK_SECRET`
- **Description**: Secret key for webhook signature validation
- **Required**: Yes (in production)
- **Default**: None
- **Security**: Must be a cryptographically secure random string (minimum 32 characters)
- **Generation**: `openssl rand -hex 32`
- **Example**: `JIRA_WEBHOOK_SECRET=your-secure-secret-here`

## Docker-Specific Configuration

### `DOCKER_MEMORY_LIMIT`
- **Description**: Memory limit for the container
- **Default**: 2GB (production), 1GB (development)
- **Units**: GB
- **Example**: `DOCKER_MEMORY_LIMIT=2GB`

### `DOCKER_CPU_LIMIT`
- **Description**: CPU limit for the container
- **Default**: 2.0 cores (production), 1.0 core (development)
- **Units**: CPU cores
- **Example**: `DOCKER_CPU_LIMIT=2.0`

### `READ_ONLY_FILESYSTEM`
- **Description**: Enables read-only filesystem mode
- **Values**: `true`, `false`
- **Default**: `true` (production), `false` (development)
- **Security Critical**: Required for production security
- **Example**: `READ_ONLY_FILESYSTEM=true`

## Logging Configuration

### `LOG_ROTATION_SIZE`
- **Description**: Maximum size of log files before rotation
- **Default**: 10MB
- **Units**: MB
- **Example**: `LOG_ROTATION_SIZE=10MB`

### `LOG_ROTATION_FILES`
- **Description**: Number of rotated log files to keep
- **Default**: 3
- **Example**: `LOG_ROTATION_FILES=3`

## Health Check Configuration

### `HEALTH_CHECK_INTERVAL`
- **Description**: Interval between health checks
- **Default**: 30s
- **Units**: Seconds
- **Example**: `HEALTH_CHECK_INTERVAL=30s`

### `HEALTH_CHECK_TIMEOUT`
- **Description**: Timeout for health check requests
- **Default**: 10s
- **Units**: Seconds
- **Example**: `HEALTH_CHECK_TIMEOUT=10s`

### `HEALTH_CHECK_RETRIES`
- **Description**: Number of retries before marking container unhealthy
- **Default**: 3
- **Example**: `HEALTH_CHECK_RETRIES=3`

### `HEALTH_CHECK_START_PERIOD`
- **Description**: Initialization time before health checks start
- **Default**: 40s
- **Units**: Seconds
- **Example**: `HEALTH_CHECK_START_PERIOD=40s`

## JWT Configuration

### `JIRA_JWT_VALIDATION_ENABLED`
- **Description**: Enables JWT validation for Connect apps
- **Values**: `true`, `false`
- **Default**: `true`
- **Example**: `JIRA_JWT_VALIDATION_ENABLED=true`

### `JIRA_JWT_ALLOWED_ISSUERS`
- **Description**: Comma-separated list of allowed issuer client keys
- **Default**: None (must be configured)
- **Example**: `JIRA_JWT_ALLOWED_ISSUERS=client-key-1,client-key-2`

### `JIRA_JWT_AUDIENCE`
- **Description**: Expected audience for JWT validation
- **Default**: None (must be configured)
- **Example**: `JIRA_JWT_AUDIENCE=https://your-app.com`

## Usage Examples

### Development Environment
```env
LOG_LEVEL=debug
DEBUG=true
JIRA_WEBHOOK_SECRET=dev-secret-123
DOCKER_MEMORY_LIMIT=1GB
DOCKER_CPU_LIMIT=1.0
READ_ONLY_FILESYSTEM=false
```

### Production Environment
```env
LOG_LEVEL=info
DEBUG=false
JIRA_WEBHOOK_SECRET=prod-secret-456
DOCKER_MEMORY_LIMIT=2GB
DOCKER_CPU_LIMIT=2.0
READ_ONLY_FILESYSTEM=true
LOG_ROTATION_SIZE=10MB
LOG_ROTATION_FILES=3
JIRA_JWT_ALLOWED_ISSUERS=client-key-1,client-key-2
JIRA_JWT_AUDIENCE=https://your-app.com
```

## Best Practices

- **Secret Management**: Store secrets in secure vaults, never in version control
- **Resource Limits**: Adjust based on monitoring data and workload patterns
- **Logging**: Enable structured logging in production with appropriate rotation
- **Security**: Always set `READ_ONLY_FILESYSTEM=true` in production environments
- **JWT Configuration**: Configure `JIRA_JWT_ALLOWED_ISSUERS` and `JIRA_JWT_AUDIENCE` for production security

## Troubleshooting

### Missing Required Variables
- **Symptom**: Application fails to start or behaves unexpectedly
- **Solution**: Verify all required variables are set using `env | grep JIRA`

### Incorrect Values
- **Symptom**: Validation errors or unexpected behavior
- **Solution**: Check variable values against documentation and logs

### Security Issues
- **Symptom**: Unauthorized access or security vulnerabilities
- **Solution**: Ensure production variables follow security best practices, especially `JIRA_WEBHOOK_SECRET` and `READ_ONLY_FILESYSTEM`

### JWT Validation Failures
- **Symptom**: "JWT signature verification failed" errors
- **Solution**: Verify `JIRA_JWT_ALLOWED_ISSUERS` and `JIRA_JWT_AUDIENCE` match your Connect app configuration
