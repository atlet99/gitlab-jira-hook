# GitLab ↔ Jira Cloud Webhook

A lightweight Go webhook server that connects GitLab System Hooks with Jira Cloud. Automatically posts commit and merge request activity to corresponding Jira issues.

## 🎯 Overview

This service listens to GitLab System Hook events and automatically comments on Jira issues when commits or merge requests reference them. It extracts Jira issue IDs from commit messages or MR titles and posts activity updates to the corresponding Jira issues.

## ✨ Features

- **GitLab System Hook Integration**: Listens to push and merge request events
- **Jira Cloud API Integration**: Posts comments via REST API
- **Smart Issue Detection**: Extracts Jira issue IDs using regex patterns
- **Secure Authentication**: Uses Jira API tokens with Basic Auth
- **Environment Configuration**: Flexible configuration via environment variables
- **Structured Logging**: Comprehensive logging for monitoring and debugging

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- GitLab instance with System Hooks enabled
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

# Jira Configuration
JIRA_EMAIL=your-email@company.com
JIRA_TOKEN=your-jira-api-token
JIRA_BASE_URL=https://yourcompany.atlassian.net

# Optional: Logging
LOG_LEVEL=info
```

### GitLab System Hook Setup

1. Go to **GitLab Admin Area** → **System Hooks**
2. Add new webhook:
   - **URL**: `https://yourdomain.com/gitlab-hook`
   - **Secret Token**: `your-gitlab-secret-token`
   - **Events**: 
     - ☑️ Push events
     - ☑️ Merge request events
3. Click **Add system hook**

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
│   │   ├── handler.go           # GitLab webhook handler
│   │   └── parser.go            # Event parsing logic
│   ├── jira/
│   │   ├── client.go            # Jira API client
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

## 📊 API Reference

### Webhook Endpoint

**POST** `/gitlab-hook`

Handles GitLab System Hook events.

#### Headers
- `X-Gitlab-Event: System Hook`
- `X-Gitlab-Token: your-secret-token`
- `Content-Type: application/json`

#### Supported Events

##### Push Event
```json
{
  "event_name": "push",
  "commits": [
    {
      "id": "abc123",
      "message": "Fix login issue ABC-123",
      "author": {
        "name": "John Doe",
        "email": "john@example.com"
      }
    }
  ]
}
```

##### Merge Request Event
```json
{
  "object_kind": "merge_request",
  "object_attributes": {
    "id": 123,
    "title": "Implement user authentication ABC-123",
    "state": "opened",
    "action": "open"
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

- **Secret Token Validation**: Validates GitLab webhook secret
- **HTTPS Required**: Use HTTPS in production
- **API Token Security**: Store Jira tokens securely
- **Input Validation**: Validates all incoming webhook data

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
export JIRA_EMAIL=your-email@company.com
export JIRA_TOKEN=your-token
export JIRA_BASE_URL=https://yourcompany.atlassian.net
export LOG_LEVEL=info
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run linting and tests
6. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 References

- [GitLab System Hooks Documentation](https://docs.gitlab.com/ee/system_hooks/system_hooks.html)
- [Jira Cloud REST API](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
- [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens) 