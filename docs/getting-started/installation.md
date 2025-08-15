# GitLab ↔ Jira Hook - Installation Guide

## Overview
This guide provides step-by-step instructions for installing and setting up the GitLab ↔ Jira Hook service. The service creates bidirectional integration between GitLab and Jira, enabling automatic updates between issues and merge requests.

## Prerequisites
Before installing the service, ensure you have:
- Go 1.24 installed (or Docker if using containerized deployment)
- GitLab instance with admin access
- Jira instance with admin access
- Redis server for async job processing
- Basic understanding of REST API concepts

## Installation Methods

### 1. From Source
```bash
# Clone the repository
git clone https://gitlab.com/gitlab-jira-hook.git
cd gitlab-jira-hook

# Build the service
go build -o gitlab-jira-hook

# Run the service
./gitlab-jira-hook
```

### 2. Using Docker
```bash
# Pull the latest image
docker pull gitlab-jira-hook:latest

# Run the container
docker run -d \
  -p 8080:8080 \
  -v ./config:/app/config \
  gitlab-jira-hook:latest
```

### 3. Kubernetes Deployment
```yaml
# Example Kubernetes deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gitlab-jira-hook
spec:
  replicas: 3
  selector:
    matchLabels:
      app: gitlab-jira-hook
  template:
    metadata:
      labels:
        app: gitlab-jira-hook
    spec:
      containers:
      - name: gitlab-jira-hook
        image: gitlab-jira-hook:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: gitlab-jira-config
        - secretRef:
            name: gitlab-jira-secrets
```

## Configuration Requirements

### Required Configuration Files
- `config/app.yaml` - Main application configuration
- `config/projects.yaml` - Project-specific integration settings
- `config/logging.yaml` - Logging configuration

### Example Configuration (config/app.yaml)
```yaml
server:
  port: 8080
  read_timeout: 10s
  write_timeout: 10s

gitlab:
  default_token: "your-gitlab-token"
  api_url: "https://gitlab.example.com/api/v4"

jira:
  base_url: "https://your-domain.atlassian.net"
  username: "your-jira-username"
  api_token: "your-jira-api-token"

redis:
  address: "localhost:6379"
  password: ""
  database: 0

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

## Service Initialization
After installation, initialize the service with:
```bash
# Create required directories
mkdir -p logs data

# Start the service
./gitlab-jira-hook --config config/app.yaml
```

## Post-Installation Verification
Verify the service is running with:
```bash
# Check service status
curl http://localhost:8080/health

# Expected response
{
  "status": "healthy",
  "gitlab": "connected",
  "jira": "connected",
  "redis": "connected"
}
```

## Troubleshooting Common Issues

### Connection Issues
- **GitLab**: Verify token has API access and correct scope
- **Jira**: Check API token and username are correctly configured
- **Redis**: Ensure Redis server is running and accessible

### Configuration Issues
- Use `./gitlab-jira-hook validate-config` to check configuration
- Check file permissions for config files
- Verify all required fields are present in configuration

## Next Steps
After successful installation, proceed with:
1. [Configuration Guide](configuration.md) - Set up project-specific integrations
2. [Quickstart Guide](quickstart.md) - Begin integrating GitLab and Jira
3. [Architecture Overview](../architecture/overview.md) - Understand system design
