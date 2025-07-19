# GitLab ↔ Jira Cloud Webhook

A lightweight Go webhook server that connects GitLab System Hooks and Project Webhooks with Jira Cloud. Automatically posts commit, merge request, and other GitLab activity to corresponding Jira issues with rich, informative comments.

## 🎯 Overview

This service listens to GitLab System Hook and Project Webhook events and automatically comments on Jira issues when commits, merge requests, or other activities reference them. It extracts Jira issue IDs from various sources and posts detailed activity updates to the corresponding Jira issues using Jira Cloud REST API v3.0.

## ✨ Features

- **GitLab System Hook Integration**: Listens to all System Hook events (push, merge_request, project_create, user_create, etc.)
- **GitLab Project Webhook Integration**: Listens to Project Webhook events (push, merge_request, issue, note, pipeline, etc.)
- **Jira Cloud API v3.0 Integration**: Posts rich comments via REST API with ADF (Atlassian Document Format)
- **Smart Issue Detection**: Extracts Jira issue IDs using regex patterns from various sources
- **Branch Filtering**: Configurable push event filtering by branch patterns with wildcard support
- **Project/Group Filtering**: Filter events by specific projects or groups
- **Secure Authentication**: Uses Jira API tokens with Basic Auth
- **Rate Limiting & Retry**: Built-in rate limiting and retry mechanisms for Jira API calls
- **Environment Configuration**: Flexible configuration via environment variables
- **Structured Logging**: Comprehensive logging for monitoring and debugging
- **Idempotent Operations**: Handles duplicate events gracefully

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

# Optional: Logging
LOG_LEVEL=info

# Optional: Event Filtering
ALLOWED_PROJECTS=project1,project2
ALLOWED_GROUPS=group1,group2
# Optional: Push branch filter (comma-separated, supports * and ? wildcards)
PUSH_BRANCH_FILTER=main,release-*,hotfix/*
```

### Dynamic Worker Pool Scaling

| Parameter             | Description                                                      | Default |
|-----------------------|------------------------------------------------------------------|---------|
| `min_workers`         | Minimum number of workers in the pool                             | 2       |
| `max_workers`         | Maximum number of workers in the pool                             | 32      |
| `scale_up_threshold`  | Queue length above which the pool will scale up                   | 10      |
| `scale_down_threshold`| Queue length below which the pool will scale down                 | 2       |
| `scale_interval`      | Interval (in seconds) between scaling checks                      | 10      |

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

### Commit Messages

Include Jira issue IDs in your commit messages:

```bash
git commit -m "Fix login issue ABC-123"
git commit -m "Implements feature XYZ-456 and resolves ABC-789"
```

### Merge Request Titles

Include Jira issue IDs in MR titles:

```bash
# Good MR titles
"Implement user authentication ABC-123"
"Fix database connection XYZ-456"
```

### Issue Titles and Descriptions

Include Jira issue IDs in GitLab issue titles and descriptions:

```bash
# GitLab issue title
"Bug related to ABC-123 implementation"

# GitLab issue description
"This issue is related to ABC-123 and XYZ-456"
```

### Comments and Notes

Include Jira issue IDs in comments on merge requests, issues, or commits:

```bash
# Comment on MR
"This change addresses ABC-123 requirements"

# Comment on issue
"Related to ABC-123 and XYZ-456"
```

### Supported Jira ID Patterns

The service recognizes these patterns:
- `ABC-123` (standard format)
- `XYZ-456` (any 2+ letter prefix)
- `PROJ-789` (project-specific)

## 🏗️ Architecture

```
┌─────────────┐    ┌─────────────────┐    ┌─────────────┐
│   GitLab    │───▶│  Webhook Server │───▶│    Jira     │
│ System Hook │    │      (Go)       │    │   Cloud     │
│ Project Hook│    │                 │    │   API v3.0  │
└─────────────┘    └─────────────────┘    └─────────────┘
```

### Project Structure

```
gitlab-jira-hook/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── gitlab/
│   │   ├── handler.go           # GitLab System Hook handler
│   │   ├── project_hooks.go     # GitLab Project Hook handler
│   │   ├── types.go             # GitLab event types
│   │   ├── parser.go            # Event parsing logic
│   │   └── filter_test.go       # Filtering tests
│   ├── jira/
│   │   ├── client.go            # Jira API client
│   │   ├── types.go             # Jira types
│   │   └── comment.go           # Comment creation
│   └── server/
│       └── server.go            # HTTP server setup
├── pkg/
│   └── utils/
│       └── logger.go            # Logging utilities
├── config.env.example           # Example configuration
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

## 📊 API Reference

### System Hook Endpoint

**POST** `/gitlab-hook`

Handles GitLab System Hook events.

#### Headers
- `X-Gitlab-Event: System Hook`
- `X-Gitlab-Token: your-secret-token`
- `Content-Type: application/json`

### Project Hook Endpoint

**POST** `/project-hook`

Handles GitLab Project Webhook events.

#### Headers
- `X-Gitlab-Token: your-secret-token`
- `Content-Type: application/json`

#### Supported Events

##### Push Event
```json
{
  "object_kind": "push",
  "ref": "refs/heads/main",
  "commits": [
    {
      "id": "abc123",
      "message": "Fix login issue ABC-123",
      "author": {
        "name": "John Doe",
        "email": "john@example.com"
      },
      "url": "https://gitlab.com/project/commit/abc123",
      "timestamp": "2024-01-01T12:00:00Z"
    }
  ],
  "project": {
    "name": "My Project",
    "web_url": "https://gitlab.com/project"
  }
}
```

##### Merge Request Event
```json
{
  "object_kind": "merge_request",
  "object_attributes": {
    "id": 123,
    "title": "Implement user authentication ABC-123",
    "description": "This fixes ABC-123 requirements",
    "state": "opened",
    "action": "open",
    "source_branch": "feature/auth",
    "target_branch": "main",
    "url": "https://gitlab.com/project/merge_requests/123"
  },
  "project": {
    "name": "My Project",
    "web_url": "https://gitlab.com/project"
  }
}
```

##### Issue Event
```json
{
  "object_kind": "issue",
  "object_attributes": {
    "id": 456,
    "title": "Bug related to ABC-123",
    "description": "This issue is related to ABC-123",
    "state": "opened",
    "action": "open",
    "issue_type": "issue",
    "priority": "medium",
    "url": "https://gitlab.com/project/issues/456"
  },
  "project": {
    "name": "My Project",
    "web_url": "https://gitlab.com/project"
  }
}
```

##### Pipeline Event
```json
{
  "object_kind": "pipeline",
  "object_attributes": {
    "id": 101,
    "ref": "main",
    "status": "success",
    "sha": "abc123",
    "duration": 120,
    "url": "https://gitlab.com/project/pipelines/101"
  },
  "project": {
    "name": "My Project",
    "web_url": "https://gitlab.com/project"
  }
}
```

## 🔍 Monitoring

### Logs

The service provides structured logging with different levels:
- `DEBUG`: Detailed debugging information
- `INFO`: General operational messages
- `WARN`: Warning messages
- `ERROR`: Error messages

### Health Check

**GET** `/health`

Returns service health status.

## 🛡️ Security

- **Secret Token Validation**: Validates GitLab webhook secret tokens
- **HTTPS Required**: Use HTTPS in production
- **API Token Security**: Store Jira tokens securely
- **Input Validation**: Validates all incoming webhook data
- **Rate Limiting**: Built-in rate limiting for Jira API calls
- **Error Handling**: Graceful handling of API failures

## 🚀 Deployment

### Docker

```bash
make docker-build
make docker-run
```

### Environment Variables

Set these environment variables in production:

```bash
export PORT=8080
export GITLAB_SECRET=your-secret
export GITLAB_BASE_URL=https://gitlab.com
export JIRA_EMAIL=your-email@company.com
export JIRA_TOKEN=your-token
export JIRA_BASE_URL=https://yourcompany.atlassian.net
export JIRA_RATE_LIMIT=10
export JIRA_RETRY_MAX_ATTEMPTS=3
export JIRA_RETRY_BASE_DELAY_MS=200
export LOG_LEVEL=info
export ALLOWED_PROJECTS=project1,project2
export ALLOWED_GROUPS=group1,group2
export PUSH_BRANCH_FILTER=main,release-*,hotfix/*
```

## 🔧 Configuration Examples

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

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 References

- [GitLab System Hooks Documentation](https://docs.gitlab.com/ee/system_hooks/system_hooks.html)
- [GitLab Project Webhooks Documentation](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html)
- [Jira Cloud REST API v3.0](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
- [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/)
- [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens) 