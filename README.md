# GitLab ↔ Jira Cloud Webhook

An advanced Go webhook server that provides bidirectional synchronization between GitLab and Jira Cloud. Automatically posts commit, merge request, and other GitLab activity to corresponding Jira issues with rich, informative comments, while also supporting Jira → GitLab synchronization for comprehensive issue lifecycle management.

## 🎯 Overview

This service provides comprehensive bidirectional synchronization between GitLab and Jira:

- **GitLab → Jira**: Automatically comments on Jira issues when commits, merge requests, or other activities reference them
- **Jira → GitLab**: Automatically creates GitLab issues from Jira issues with real-time status and comment synchronization
- **Advanced Features**: OAuth 2.0 authentication, JWT validation for Connect apps, dynamic worker pool scaling, advanced caching, comprehensive monitoring, and error recovery strategies

The service extracts Jira issue IDs from various sources and posts detailed activity updates to the corresponding Jira issues using Jira Cloud REST API v3.0 with Atlassian Document Format (ADF) for rich content.

## ✨ Features

### Core Functionality
- **Bidirectional Synchronization**: Complete two-way sync between GitLab and Jira
- **GitLab System Hook Integration**: Listens to all System Hook events (push, merge_request, project_create, user_create, etc.)
- **GitLab Project Webhook Integration**: Listens to Project Webhook events (push, merge_request, issue, note, pipeline, etc.)
- **Jira Cloud API v3.0 Integration**: Posts rich comments via REST API with ADF (Atlassian Document Format)
- **Smart Issue Detection**: Extracts Jira issue IDs using regex patterns from various sources
- **Branch Filtering**: Configurable push event filtering by branch patterns with wildcard support
- **Project/Group Filtering**: Filter events by specific projects or groups
- **Secure Authentication**: Multiple authentication methods (Basic Auth, OAuth 2.0, JWT for Connect apps)
- **Rate Limiting & Retry**: Built-in rate limiting and retry mechanisms for Jira API calls
- **Environment Configuration**: Flexible configuration via environment variables
- **JQL-based Event Filtering**: Filter events using JQL queries to process only relevant issues
- **Structured Logging**: Comprehensive logging for monitoring and debugging
- **Idempotent Operations**: Handles duplicate events gracefully

### Advanced Bidirectional Sync
- **Jira → GitLab Sync**: Automatic GitLab issue creation from Jira issues
- **Real-time User Assignment**: Email-based user matching and assignment
- **Status Synchronization**: Map Jira statuses to GitLab labels
- **Comment Synchronization**: Bidirectional comment and worklog sync
- **Conflict Resolution**: Multiple strategies (last_write_wins, merge, manual)
- **Project Mapping**: Flexible project routing with namespace and explicit mappings
- **Field Mapping**: Dynamic field synchronization between systems

### Authentication & Security
- **Multiple Auth Methods**: Basic Auth, OAuth 2.0, JWT validation for Connect apps
- **OAuth 2.0 Support**: Enhanced security with token management and auto-refresh
- **JWT Validation**: Secure authentication for Jira Connect apps
- **Input Validation**: Comprehensive input sanitization and validation
- **SHA-256 Hashing**: Secure hashing algorithms
- **Secret Management**: Secure handling of authentication tokens

### Performance & Scalability
- **Dynamic Worker Pool**: Auto-scaling based on queue length and system load
- **Priority Queue**: Job prioritization based on importance and urgency
- **Advanced Caching**: Multiple eviction strategies (LRU, LFU, FIFO, TTL, Adaptive)
- **Memory Optimization**: Compression and optional encryption for cached data
- **Load Balancing**: Consistent hashing for distributed environments
- **Throughput Optimization**: Configurable concurrency and timeout settings

### Monitoring & Observability
- **Distributed Tracing**: OpenTelemetry integration for request tracing and debugging
- **Advanced Monitoring**: Prometheus metrics for comprehensive system observability
- **Health Checks**: Comprehensive health monitoring with detailed status reporting
- **Performance Scoring**: Real-time performance metrics and trend analysis
- **Error Recovery Manager**: Multiple recovery strategies (retry, circuit breaker, fallback, graceful degradation, restart)
- **Alerting System**: Configurable alerts for system events and performance metrics
- **Structured Logging**: Enhanced logging with context support and correlation IDs

### Configuration Management
- **Hot Reload**: Real-time configuration updates without service restart
- **File Monitoring**: Automatic detection of configuration file changes
- **Environment Variables**: Dynamic environment variable change detection
- **Auto-Detection**: Automatic worker and queue sizing based on system resources
- **Validation**: Comprehensive configuration validation with helpful error messages

### Security & Performance
- **Adaptive Rate Limiting**: Dynamic rate limiting based on system load
- **Per-IP Limiting**: Granular rate limiting by IP address
- **Per-Endpoint Limiting**: Specific rate limits for different endpoints
- **Slowloris Protection**: Built-in protection against Slowloris attacks
- **Request Validation**: Comprehensive input sanitization and validation
- **Circuit Breaker**: Automatic failure detection and recovery

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- GitLab instance with System Hooks and/or Project Webhooks enabled
- Jira Cloud instance with API access

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/atlet99/gitlab-jira-hook.git
   cd gitlab-jira-hook
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Configure environment**
   ```bash
   cp config.env.example config.env
   # Edit config.env with your settings
   ```

4. **Build and run**
   ```bash
   make build
   make run
   ```

## ⚙️ Configuration

### Environment Variables

Create a `config.env` file with the following variables:

```env
# Server Configuration
PORT=8080
GITLAB_SECRET=your-gitlab-secret-token
GITLAB_BASE_URL=https://gitlab.com

# Jira Configuration
JIRA_EMAIL=your-email@company.com
JIRA_TOKEN=your-jira-api-token
JIRA_BASE_URL=https://yourcompany.atlassian.net
JIRA_RATE_LIMIT=10
JIRA_RETRY_MAX_ATTEMPTS=3
JIRA_RETRY_BASE_DELAY_MS=200

# Monitoring Configuration
ENABLE_TRACING=true
ENABLE_METRICS=true
METRICS_PORT=9090
TRACING_ENDPOINT=http://localhost:4318/v1/traces

# Cache Configuration
CACHE_ENABLED=true
CACHE_MAX_SIZE=1000
CACHE_TTL=3600
CACHE_STRATEGY=LRU

# Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_MINUTE=60
RATE_LIMIT_BURST=10

# Optional: Logging
LOG_LEVEL=info

# Optional: Event Filtering
ALLOWED_PROJECTS=project1,project2
ALLOWED_GROUPS=group1,group2
# Optional: Push branch filter (comma-separated, supports * and ? wildcards)
PUSH_BRANCH_FILTER=main,release-*,hotfix/*

# Optional: JQL filter to process only specific issues
# Example: JQL_FILTER=project = "TEST" AND issuetype = Bug
JQL_FILTER=
```

### Dynamic Worker Pool Scaling

| Parameter              | Description                                       | Default |
| ---------------------- | ------------------------------------------------- | ------- |
| `min_workers`          | Minimum number of workers in the pool             | 2       |
| `max_workers`          | Maximum number of workers in the pool             | 32      |
| `scale_up_threshold`   | Queue length above which the pool will scale up   | 10      |
| `scale_down_threshold` | Queue length below which the pool will scale down | 2       |
| `scale_interval`       | Interval (in seconds) between scaling checks      | 10      |

These parameters can be set via config file or environment variables. See `internal/config/config.go` for details.

### GitLab System Hook Setup

1. Go to **GitLab Admin Area** → **System Hooks**
2. Add new webhook:
   - **URL**: `https://yourdomain.com/gitlab-hook`
   - **Secret Token**: `your-gitlab-secret-token`
   - **Events**: 
     - ☑️ Push events
     - ☑️ Merge request events
     - ☑️ Project events
     - ☑️ User events
     - ☑️ Group events
     - ☑️ Repository events
     - ☑️ Tag push events
     - ☑️ Release events
     - ☑️ Deployment events
     - ☑️ Feature flag events
     - ☑️ Wiki page events
     - ☑️ Pipeline events
     - ☑️ Build events
     - ☑️ Note events
     - ☑️ Issue events
3. Click **Add system hook**

### GitLab Project Webhook Setup

1. Go to your **Project** → **Settings** → **Webhooks**
2. Add new webhook:
   - **URL**: `https://yourdomain.com/project-hook`
   - **Secret Token**: `your-gitlab-secret-token`
   - **Events**: 
     - ☑️ Push events
     - ☑️ Merge request events
     - ☑️ Issues events
     - ☑️ Comments events
     - ☑️ Pipeline events
     - ☑️ Build events
     - ☑️ Tag push events
     - ☑️ Release events
     - ☑️ Deployment events
     - ☑️ Feature flag events
     - ☑️ Wiki page events
3. Click **Add webhook**

### Jira API Token Setup

1. Go to [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Create a new API token
3. Save the token securely
4. Use your email + token for Basic Auth

## 📋 Usage

### GitLab → Jira Synchronization

#### Commit Messages

Include Jira issue IDs in your commit messages:

```bash
git commit -m "Fix login issue ABC-123"
git commit -m "Implements feature XYZ-456 and resolves ABC-789"
```

#### Merge Request Titles

Include Jira issue IDs in MR titles:

```bash
# Good MR titles
"Implement user authentication ABC-123"
"Fix database connection XYZ-456"
```

#### Issue Titles and Descriptions

Include Jira issue IDs in GitLab issue titles and descriptions:

```bash
# GitLab issue title
"Bug related to ABC-123 implementation"

# GitLab issue description
"This issue is related to ABC-123 and XYZ-456"
```

#### Comments and Notes

Include Jira issue IDs in comments on merge requests, issues, or commits:

```bash
# Comment on MR
"This change addresses ABC-123 requirements"

# Comment on issue
"Related to ABC-123 and XYZ-456"
```

#### Supported Jira ID Patterns

The service recognizes these patterns:
- `ABC-123` (standard format)
- `XYZ-456` (any 2+ letter prefix)
- `PROJ-789` (project-specific)

### Jira → GitLab Synchronization

#### Automatic Issue Creation
When bidirectional sync is enabled, Jira issues are automatically created in GitLab with:
- Proper project mapping
- User assignment based on email matching
- Status labels applied
- Comments and worklogs synchronized

#### Configuration Requirements
To enable Jira → GitLab sync, ensure you have:
- `GITLAB_API_TOKEN` with appropriate permissions
- `JIRA_WEBHOOK_SECRET` for webhook validation
- Project mappings configured via `PROJECT_MAPPINGS` or `GITLAB_NAMESPACE`

#### Supported Jira Events
- `issue_created`, `issue_updated`, `issue_deleted`
- `comment_created`, `comment_updated`, `comment_deleted`
- `worklog_created`, `worklog_updated`, `worklog_deleted`

#### Conflict Resolution
The system provides multiple conflict resolution strategies:
- `last_write_wins`: Most recent update takes precedence
- `merge`: Attempts to merge changes from both systems
- `manual`: Requires manual intervention for conflicts

## 🏗️ Architecture

### System Architecture

```
┌─────────────┐    ┌─────────────────┐    ┌─────────────┐
│   GitLab    │───▶│  Webhook Server │───▶│    Jira     │
│ System Hook │    │      (Go)       │    │   Cloud     │
│ Project Hook│    │                 │    │   API v3.0  │
└─────────────┘    └─────────────────┘    └─────────────┘
       ▲                                        │
       │                                        ▼
       │                            ┌─────────────────┐
       └───────────────────────────▶│   Jira Webhook  │
                                    │   (for bidirectional) │
                                    └─────────────────┘
                        │
                        ▼
              ┌─────────────────---┐
              │   Monitoring       │
              │   & Tracing        │
              │   (Prometheus      │
              │   + OpenTelemetry) │
              └─────────────────---┘
```

### Key Components

1. **Webhook Server**: Main HTTP server handling GitLab and Jira webhooks
2. **Bidirectional Sync Manager**: Handles synchronization between GitLab and Jira
3. **Advanced Cache**: Multi-level caching with multiple eviction strategies
4. **Worker Pool**: Dynamic scaling worker pool for job processing
5. **Monitoring System**: Comprehensive metrics and health monitoring
6. **Error Recovery**: Multiple recovery strategies for system resilience

### Project Structure

```
gitlab-jira-hook/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── async/                   # Asynchronous job processing
│   │   ├── adapter.go           # Job adapter interface
│   │   ├── delayed_queue.go     # Delayed job queue
│   │   ├── errors.go            # Error definitions
│   │   ├── interface.go         # Core interfaces
│   │   ├── middleware.go        # Processing middleware
│   │   ├── priority_worker_pool.go # Priority-based worker pool
│   │   ├── queue.go             # Job queue implementation
│   │   └── worker_pool.go       # Worker pool management
│   ├── benchmarks/              # Performance benchmarks
│   │   └── benchmarks_test.go   # Benchmark tests
│   ├── cache/                   # Caching system
│   │   ├── advanced_cache.go    # Advanced caching with multiple strategies
│   │   └── cache.go             # Basic memory cache
│   ├── common/                  # Shared utilities
│   │   └── userlink.go          # User linking utilities
│   ├── config/                  # Configuration management
│   │   ├── config.go            # Configuration loading and validation
│   │   └── hot_reload.go        # Hot reload functionality
│   ├── gitlab/                  # GitLab integration
│   │   ├── branch_url_test.go   # Branch URL tests
│   │   ├── event_processor.go   # Event processing logic
│   │   ├── filter_test.go       # Filtering tests
│   │   ├── handler.go           # GitLab System Hook handler
│   │   ├── integration_test.go  # Integration tests
│   │   ├── mr_linking_test.go   # MR linking tests
│   │   ├── parser.go            # Event parsing logic
│   │   ├── project_hooks.go     # GitLab Project Hook handler
│   │   ├── types.go             # GitLab event types
│   │   └── url_builder.go       # URL building utilities
│   ├── jira/                    # Jira integration
│   │   ├── adf.go               # Atlassian Document Format
│   │   ├── client.go            # Jira API client
│   │   ├── interfaces.go        # Jira interfaces
│   │   └── types.go             # Jira types
│   ├── monitoring/              # Monitoring and observability
│   │   ├── advanced_monitoring.go # Advanced monitoring features
│   │   ├── common.go            # Shared monitoring utilities
│   │   ├── error_recovery.go    # Error recovery manager
│   │   ├── handlers.go          # Monitoring HTTP handlers
│   │   ├── prometheus.go        # Prometheus metrics
│   │   ├── tracing.go           # Distributed tracing
│   │   └── webhook_monitor.go   # Webhook monitoring
│   ├── server/                  # HTTP server
│   │   ├── rate_limiter.go      # Rate limiting middleware
│   │   └── server.go            # HTTP server setup
│   ├── timezone/                # Timezone utilities
│   │   └── timezone.go          # Timezone handling
│   ├── utils/                   # Utility functions
│   │   └── time.go              # Time utilities
│   ├── version/                 # Version management
│   │   └── version.go           # Version information
│   └── webhook/                 # Webhook interfaces
│       └── interfaces.go        # Webhook interface definitions
├── pkg/
│   └── logger/                  # Logging package
│       └── logger.go            # Structured logging
├── docs/                        # Documentation
│   ├── api/                     # API documentation
│   │   └── openapi.yaml         # OpenAPI specification
│   ├── architecture/            # Architecture documentation
│   │   ├── async-processing.md  # Async processing details
│   │   ├── error-handling.md    # Error handling implementation
│   │   └── overview.md          # Architecture overview
│   ├── advanced/                # Advanced features
│   │   └── security.md          # Security implementation
│   ├── contributing/            # Contribution guidelines
│   │   ├── guidelines.md        # Contribution guidelines
│   │   └── testing.md           # Testing documentation
│   ├── getting-started/         # Getting started guides
│   │   └── installation.md      # Installation guide
│   └── reference/               # Reference documentation
│       ├── config.md            # Configuration reference
│       └── environment-variables.md # Environment variables
├── scripts/                     # Build and deployment scripts
│   └── setup-env.sh             # Environment setup script
├── config.env.example           # Example configuration
├── docker-compose.yml           # Docker Compose configuration
├── Dockerfile                   # Docker build configuration
├── go.mod                       # Go modules
├── go.sum                       # Go modules checksum
├── Makefile                     # Build automation
├── .release-version             # Current version
├── .go-version                  # Go version
├── CHANGELOG.md                 # Release changelog
└── README.md                    # This file
```

## 🔧 Development

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Linting

```bash
make lint
```

### Running Locally

```bash
make run
```

### Development Mode

```bash
make dev
```

### Code Coverage

```bash
make coverage
```

### Performance Benchmarks

```bash
make benchmark
```

## 📊 Documentation

- [Getting Started](docs/getting-started/installation.md)
- [Architecture Overview](docs/architecture/overview.md)
- [Async Processing](docs/architecture/async-processing.md)
- [Error Handling](docs/architecture/error-handling.md)
- [Monitoring & Configuration Hot-Reloading](docs/architecture/monitoring.md)
- [Security](docs/advanced/security.md)
- [Contributing Guidelines](docs/contributing/guidelines.md)
- [Testing Documentation](docs/contributing/testing.md)
- [Configuration Reference](docs/reference/config.md)
- [Environment Variables](docs/reference/environment-variables.md)
- [Metrics Reference](docs/reference/metrics.md)
- [API Reference](docs/api/reference.md)

## 🔍 Monitoring & Observability

### Health Checks

The service provides comprehensive health monitoring:

- **Health Endpoint**: `/health` - Overall service health
- **Readiness Endpoint**: `/ready` - Service readiness for traffic
- **Metrics Endpoint**: `/metrics` - Prometheus metrics

### Distributed Tracing

OpenTelemetry integration provides:

- **Request Tracing**: Track requests across service boundaries
- **Span Correlation**: Correlate related operations
- **Performance Analysis**: Identify bottlenecks and slow operations
- **Error Tracking**: Trace error propagation through the system

### Prometheus Metrics

Comprehensive metrics collection:

- **HTTP Metrics**: Request counts, durations, status codes
- **Job Processing**: Queue lengths, processing times, success rates
- **Cache Metrics**: Hit rates, eviction counts, memory usage
- **Rate Limiting**: Request rates, throttling events
- **System Metrics**: Memory usage, goroutine counts, GC stats
- **Custom Metrics**: Business-specific metrics and alerts

### Error Recovery

Advanced error handling with multiple strategies:

- **Retry Strategy**: Exponential backoff with configurable parameters
- **Circuit Breaker**: Automatic failure detection and recovery
- **Fallback Strategy**: Graceful degradation with alternative paths
- **Graceful Degradation**: Maintain service availability during failures
- **Restart Strategy**: Automatic service restart for critical failures

### Logging

Structured logging with context support:

- **Log Levels**: DEBUG, INFO, WARN, ERROR
- **Context Correlation**: Request correlation IDs
- **Structured Fields**: JSON-formatted log entries
- **Performance Logging**: Request timing and performance data

## 🛡️ Security

- **Secret Token Validation**: Validates GitLab webhook secret tokens
- **HTTPS Required**: Use HTTPS in production
- **API Token Security**: Store Jira tokens securely
- **Input Validation**: Validates all incoming webhook data
- **Rate Limiting**: Built-in rate limiting for Jira API calls
- **Error Handling**: Graceful handling of API failures
- **SHA-256 Hashing**: Secure hashing algorithms
- **Slowloris Protection**: Protection against Slowloris attacks
- **Input Sanitization**: Comprehensive input validation and sanitization

## 🚀 Deployment

### Docker

```bash
make docker-build
make docker-run
```

### Docker Compose

The project provides multiple Docker Compose configurations for different environments:

#### Development
```bash
# Uses docker-compose.yml + docker-compose.override.yml automatically
docker-compose up

# Resource limits: 1GB memory, 1 CPU core
```

#### Production
```bash
# Uses docker-compose.yml + docker-compose.prod.yml
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Resource limits: 2GB memory, 2 CPU cores
# Security: Read-only filesystem, no new privileges
```

#### Base Configuration Only
```bash
# Uses only docker-compose.yml (ignores override)
docker-compose -f docker-compose.yml up -d
```

For detailed Docker Compose configuration options, see [docs/docker-compose.md](docs/docker-compose.md).

### Kubernetes

```yaml
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
        image: ghcr.io/atlet99/gitlab-jira-hook:v0.1.5
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: PORT
          value: "8080"
        - name: GITLAB_SECRET
          valueFrom:
            secretKeyRef:
              name: gitlab-jira-hook-secrets
              key: gitlab-secret
        - name: GITLAB_API_TOKEN
          valueFrom:
            secretKeyRef:
              name: gitlab-jira-hook-secrets
              key: gitlab-api-token
        - name: JIRA_EMAIL
          valueFrom:
            secretKeyRef:
              name: gitlab-jira-hook-secrets
              key: jira-email
        - name: JIRA_TOKEN
          valueFrom:
            secretKeyRef:
              name: gitlab-jira-hook-secrets
              key: jira-token
        - name: JIRA_BASE_URL
          value: "https://yourcompany.atlassian.net"
        - name: JIRA_WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: gitlab-jira-hook-secrets
              key: jira-webhook-secret
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Environment Variables

Set these environment variables in production:

```bash
export PORT=8080
export GITLAB_SECRET=your-secret
export GITLAB_BASE_URL=https://gitlab.com
export GITLAB_API_TOKEN=your-gitlab-api-token
export JIRA_EMAIL=your-email@company.com
export JIRA_TOKEN=your-token
export JIRA_BASE_URL=https://yourcompany.atlassian.net
export JIRA_WEBHOOK_SECRET=your-jira-webhook-secret
export JIRA_RATE_LIMIT=10
export JIRA_RETRY_MAX_ATTEMPTS=3
export JIRA_RETRY_BASE_DELAY_MS=200
export LOG_LEVEL=info
export ALLOWED_PROJECTS=project1,project2
export ALLOWED_GROUPS=group1,group2
export PUSH_BRANCH_FILTER=main,release-*,hotfix/*
export ENABLE_TRACING=true
export ENABLE_METRICS=true
export METRICS_PORT=9090
export CACHE_ENABLED=true
export RATE_LIMIT_ENABLED=true
export BIDIRECTIONAL_ENABLED=true
export MIN_WORKERS=5
export MAX_WORKERS=50
export MAX_CONCURRENT_JOBS=100
```

## 🔧 Configuration Examples

### Basic Setup

```env
# Server Configuration
PORT=8080
LOG_LEVEL=info
TIMEZONE=Etc/GMT-5

# GitLab Configuration
GITLAB_SECRET=your-secret-token
GITLAB_BASE_URL=https://gitlab.com
GITLAB_API_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
GITLAB_NAMESPACE=jira-sync

# Jira Configuration
JIRA_EMAIL=your-email@company.com
JIRA_TOKEN=your-jira-api-token
JIRA_BASE_URL=https://yourcompany.atlassian.net
JIRA_WEBHOOK_SECRET=your-jira-webhook-secret

# Basic Rate Limiting
JIRA_RATE_LIMIT=10
JIRA_RETRY_MAX_ATTEMPTS=3
JIRA_RETRY_BASE_DELAY_MS=200
```

### Bidirectional Sync Setup

```env
# Enable bidirectional synchronization
BIDIRECTIONAL_ENABLED=true
BIDIRECTIONAL_EVENTS=issue_created,issue_updated,comment_created
BIDIRECTIONAL_SYNC_INTERVAL=30
BIDIRECTIONAL_CONFLICT_STRATEGY=last_write_wins

# Status mapping
STATUS_MAPPING_ENABLED=true
STATUS_MAPPING="To Do=todo,In Progress=in-progress,Done=done,Blocked=blocked"
DEFAULT_GITLAB_LABEL=external-sync

# Comment sync
COMMENT_SYNC_ENABLED=true
COMMENT_SYNC_DIRECTION=bidirectional
COMMENT_TEMPLATE_JIRA_TO_GITLAB="**From Jira**: {content}"
COMMENT_TEMPLATE_GITLAB_TO_JIRA="**From GitLab**: {content}"

# User mapping
ASSIGNEE_SYNC_ENABLED=true
USER_MAPPING="john.doe@company.com=jdoe,jane.smith@company.com=jsmith"
DEFAULT_GITLAB_ASSIGNEE=jdoe

# Sync limits
SYNC_BATCH_SIZE=10
SYNC_RATE_LIMIT=60
SYNC_RETRY_ATTEMPTS=3
SKIP_OLD_EVENTS=true
MAX_EVENT_AGE=24
```

### OAuth 2.0 Authentication

```env
# Enable OAuth 2.0
JIRA_AUTH_METHOD=oauth2
JIRA_OAUTH2_CLIENT_ID=your-oauth2-client-id
JIRA_OAUTH2_CLIENT_SECRET=your-oauth2-client-secret
JIRA_OAUTH2_SCOPE="read:jira-work write:jira-work manage:jira-webhook"
JIRA_OAUTH2_TOKEN_URL=https://auth.atlassian.com/oauth/token
JIRA_OAUTH2_AUTH_URL=https://auth.atlassian.com/authorize
JIRA_OAUTH2_REDIRECT_URL=https://your-app.com/auth/jira/callback
```

### JWT Authentication for Connect Apps

```env
# JWT Configuration
JWT_ENABLED=true
JWT_EXPECTED_AUDIENCE=https://your-app.com/webhook
JWT_ALLOWED_ISSUERS=atlassian-connect-examples,my-connect-app
```

### High-Performance Setup

```env
# Worker pool configuration
MIN_WORKERS=10
MAX_WORKERS=100
SCALE_UP_THRESHOLD=5
SCALE_DOWN_THRESHOLD=2
SCALE_INTERVAL=10

# Rate limiting and throughput
MAX_CONCURRENT_JOBS=200
JOB_TIMEOUT_SECONDS=120
QUEUE_TIMEOUT_MS=5000

# Retry configuration
MAX_RETRIES=5
RETRY_DELAY_MS=500
BACKOFF_MULTIPLIER=1.5
MAX_BACKOFF_MS=30000

# Monitoring
METRICS_ENABLED=true
HEALTH_CHECK_INTERVAL=15
```

### Advanced Caching

```env
# Cache configuration (auto-detected based on system resources)
# Manual configuration if needed:
CACHE_MAX_SIZE=2000
CACHE_TTL=3600
CACHE_STRATEGY=adaptive
CACHE_COMPRESSION=true
CACHE_ENCRYPTION=false
```

### Branch Filtering

Filter push events by specific branches:

```env
# Only main and develop branches
PUSH_BRANCH_FILTER=main,develop

# Main branch and all release branches
PUSH_BRANCH_FILTER=main,release-*

# Main, develop, and all hotfix branches
PUSH_BRANCH_FILTER=main,develop,hotfix/*

# All branches (wildcard)
PUSH_BRANCH_FILTER=*

# Complex patterns
PUSH_BRANCH_FILTER=main,release-*,hotfix/*,feature/??-*
```

### Project Filtering

Filter events by specific projects or groups:

```env
# Specific projects
ALLOWED_PROJECTS=my-org/my-project,another-org/another-project

# Specific groups
ALLOWED_GROUPS=my-org,another-org

# No filtering (all projects allowed)
# Leave empty or don't set
```

### JQL Filter Configuration

Filter events using JQL queries to process only relevant issues:

```env
# Process only issues in the "TEST" project with "Bug" issue type
JQL_FILTER=project = "TEST" AND issuetype = Bug

# Process only high priority issues
JQL_FILTER=priority = "High" OR priority = "Highest"

# Process only issues assigned to a specific user
JQL_FILTER=assignee = "user@example.com"

# Process only issues in a specific status
JQL_FILTER=status = "In Progress" OR status = "To Do"

# Process only issues created in the last 7 days
JQL_FILTER=created >= -7d
```

### Monitoring Configuration

Configure monitoring and tracing:

```env
# Enable tracing
ENABLE_TRACING=true
TRACING_ENDPOINT=http://localhost:4318/v1/traces

# Enable metrics
ENABLE_METRICS=true
METRICS_PORT=9090

# Error recovery
ERROR_RECOVERY_ENABLED=true
ERROR_RECOVERY_STRATEGY=retry
ERROR_RECOVERY_MAX_ATTEMPTS=3
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run linting and tests
6. Submit a pull request

### Development Guidelines

- Follow Go best practices and style guide
- Write comprehensive tests for new features
- Update documentation for any changes
- Use conventional commit messages
- Ensure all tests pass before submitting PR
- Maintain code quality with linting
- Add benchmarks for performance-critical code

### Code Quality

- **Linting**: All code must pass `golangci-lint`
- **Testing**: Maintain >80% test coverage
- **Documentation**: Update README and inline comments
- **Security**: Follow security best practices
- **Performance**: Add benchmarks for critical paths

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 References

- [GitLab System Hooks Documentation](https://docs.gitlab.com/ee/system_hooks/system_hooks.html)
- [GitLab Project Webhooks Documentation](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html)
- [Jira Cloud REST API v3.0](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
- [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/)
- [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Go Best Practices](https://golang.org/doc/effective_go.html) 

## 🧪 Testing & CI

- All main and test linter errors are fixed, and the test suite is stable and fast.
- Async and performance tests have been optimized for CI: job counts and sleep intervals reduced, flaky assertions fixed.
- **Performance test `TestResourceEfficiency` is temporarily disabled** due to CI timeouts. To enable, remove the `t.Skip` in `internal/async/performance_integration_test.go`.
- All other tests pass reliably. See [CHANGELOG.md](CHANGELOG.md) for details.

### Recent Linter Fixes (v1.0.1 - 2025-08-22)

- Fixed duplicate code (dupl) in Jira client modules by refactoring similar functions
- Removed unused functions to eliminate warnings and improve code maintainability
- Fixed circuit breaker test assertions to match actual error message formats
- Resolved race conditions in monitoring tests for better reliability
- Enhanced test stability and performance across the entire codebase

### Running Tests

```bash
make test
```

---

## 🐞 Debug Mode for Webhook Development

- Enable detailed debug logging for all incoming GitLab webhook data.
- Logs request headers (with token masking), pretty-printed JSON body, and parsed event info.
- Supports all GitLab webhook event types.
- **Enable via environment variable:**

```env
DEBUG_MODE=true
```

- Use for development and troubleshooting. Do not enable in production.
- See `config.env.example` for usage.

---

## 📊 Performance Monitoring & Observability

- Real-time performance monitoring with Prometheus metrics and OpenTelemetry tracing.
- Performance score (0-100) based on response time, error rate, throughput, and memory usage.
- Target compliance tracking and automatic alerting.
- Performance history and trend analysis available via API endpoints:
  - `/performance`, `/performance/history`, `/performance/targets`, `/performance/reset`
- All monitoring and performance code is covered by tests (see [CHANGELOG.md](CHANGELOG.md)).

---

## 📋 Changelog

See [CHANGELOG.md](CHANGELOG.md) for a full list of changes, fixes, and improvements, including:
- Comprehensive bidirectional synchronization system
- Advanced OAuth 2.0 and JWT authentication support
- Dynamic worker pool scaling with auto-detection
- Advanced caching with multiple eviction strategies
- Enhanced monitoring and observability features
- Improved error recovery and resilience
- Performance optimizations and security enhancements
- Comprehensive test suite and documentation
