# Docker Compose Configuration

This project provides multiple Docker Compose configurations for different environments.

## Files Overview

- `docker-compose.yml` - Base configuration with production-ready resource limits
- `docker-compose.override.yml` - Development override (automatically loaded)
- `docker-compose.prod.yml` - Production-specific configuration
- `docker-compose.local.yml` - Local development configuration (gitignored)

## Resource Limits

### Production (docker-compose.yml + docker-compose.prod.yml)
- **Memory**: 2GB limit, 1GB reservation
- **CPU**: 2.0 cores limit, 1.0 core reservation
- **Security**: Read-only filesystem, no new privileges
- **Logging**: JSON driver with rotation (10MB max, 3 files)

### Development (docker-compose.yml + docker-compose.override.yml)
- **Memory**: 1GB limit, 256MB reservation
- **CPU**: 1.0 core limit, 0.25 core reservation
- **Debug**: Enabled with debug logging
- **Volumes**: Source code mounted for development

## Usage

### Development
```bash
# Uses docker-compose.yml + docker-compose.override.yml automatically
docker-compose up

# Or explicitly
docker-compose -f docker-compose.yml -f docker-compose.override.yml up
```

### Production
```bash
# Uses docker-compose.yml + docker-compose.prod.yml
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### Base Configuration Only
```bash
# Uses only docker-compose.yml (ignores override)
docker-compose -f docker-compose.yml up
```

### Custom Local Configuration
```bash
# Create your own local override
cp docker-compose.override.yml docker-compose.local.yml
# Edit docker-compose.local.yml as needed

# Use custom local configuration
docker-compose -f docker-compose.yml -f docker-compose.local.yml up
```

## Resource Monitoring

### Check Resource Usage
```bash
# View container stats
docker stats gitlab-jira-hook

# View compose service status
docker-compose ps
```

### Resource Limits Verification
```bash
# Check memory and CPU limits
docker inspect gitlab-jira-hook | grep -A 10 -B 5 "Memory\|CpuShares"
```

## Environment Variables

### Development
- `LOG_LEVEL=debug`
- `DEBUG=true`

### Production
- `LOG_LEVEL=info`
- `DEBUG=false`

## Security Features (Production)

- **Read-only filesystem**: Container filesystem is read-only
- **No new privileges**: Container cannot gain additional privileges
- **Temporary filesystems**: `/tmp` and `/var/tmp` are tmpfs mounts
- **Restart policy**: Automatic restart on failure with backoff

## Logging Configuration

### Development
- Console logging with debug level
- No log rotation

### Production
- JSON file logging
- Log rotation: 10MB max file size, 3 files max
- Structured logging for better monitoring

## Health Checks

All configurations include health checks:
- **Endpoint**: `http://localhost:8080/health`
- **Interval**: 30 seconds
- **Timeout**: 10 seconds
- **Retries**: 3
- **Start period**: 40 seconds

## Troubleshooting

### Container OOM (Out of Memory)
If the container runs out of memory:
1. Check current usage: `docker stats gitlab-jira-hook`
2. Increase memory limit in docker-compose.yml
3. Check application logs for memory leaks

### High CPU Usage
If CPU usage is consistently high:
1. Monitor with: `docker stats gitlab-jira-hook`
2. Check application performance
3. Consider increasing CPU limits if needed

### Container Won't Start
1. Check logs: `docker-compose logs gitlab-jira-hook`
2. Verify resource availability on host
3. Check health check endpoint is responding 