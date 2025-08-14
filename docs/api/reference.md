# API Reference

## Overview
This document provides comprehensive reference documentation for the GitLab ↔ Jira Hook REST API. The API enables bidirectional communication between GitLab and Jira Cloud, allowing for automatic updates between issues and merge requests.

## Authentication
All endpoints require authentication via GitLab secret token:

- **Header**: `X-Gitlab-Token: <your-secret-token>`
- **Validation**: Token must match `GITLAB_SECRET` environment variable

## Endpoints

### System Hook Endpoint
`POST /gitlab-hook`

Handles GitLab System Hook events.

#### Request
- **Content-Type**: `application/json`
- **Body**: GitLab System Hook payload (see [GitLab System Hooks Documentation](https://docs.gitlab.com/ee/system_hooks/system_hooks.html))

#### Response Codes
- `202 Accepted`: Event queued for processing
- `400 Bad Request`: Invalid payload format
- `401 Unauthorized`: Invalid token
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Processing error

#### Example Response
```json
{
  "status": "accepted",
  "message": "webhook queued for processing"
}
```

### Project Hook Endpoint
`POST /project-hook`

Handles GitLab Project Webhook events.

#### Request
- **Content-Type**: `application/json`
- **Body**: GitLab Project Webhook payload (see [GitLab Project Webhooks Documentation](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html))

#### Response Codes
- `202 Accepted`: Event queued for processing
- `400 Bad Request`: Invalid payload format
- `401 Unauthorized`: Invalid token
- `403 Forbidden`: Event filtered by project/group
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Processing error

#### Example Response
```json
{
  "status": "accepted",
  "message": "webhook queued for processing"
}
```

### Health Check Endpoint
`GET /health`

Returns service health status with detailed component information.

#### Response Codes
- `200 OK`: Service is healthy
- `503 Service Unavailable`: Service is unhealthy

#### Example Response
```json
{
  "status": "healthy",
  "gitlab": "connected",
  "jira": "connected",
  "redis": "connected",
  "worker_pool": {
    "active_workers": 8,
    "queue_length": 2,
    "max_workers": 32
  }
}
```

### Readiness Check Endpoint
`GET /ready`

Returns service readiness status for Kubernetes health checks.

#### Response Codes
- `200 OK`: Service is ready to accept traffic
- `503 Service Unavailable`: Service is not ready

#### Example Response
```json
{
  "status": "ready",
  "message": "Service is ready to accept traffic"
}
```

### Metrics Endpoint
`GET /metrics`

Returns Prometheus metrics for monitoring and alerting.

#### Response Codes
- `200 OK`: Metrics returned successfully

#### Example Response
```
# HELP gitlab_jira_hook_http_request_duration_seconds HTTP request duration in seconds
# TYPE gitlab_jira_hook_http_request_duration_seconds histogram
gitlab_jira_hook_http_request_duration_seconds_bucket{method="POST",path="/gitlab-hook",status="202",le="0.005"} 1
gitlab_jira_hook_http_request_duration_seconds_bucket{method="POST",path="/gitlab-hook",status="202",le="0.01"} 1
gitlab_jira_hook_http_request_duration_seconds_bucket{method="POST",path="/gitlab-hook",status="202",le="0.025"} 1
# ... additional metrics
```

## Jira Cloud REST API v3 Integration

### Authentication
- **Basic Auth**: Email + API token (token created in Atlassian Account)
- **Base URL**: `https://<your-domain>.atlassian.net/rest/api/3`

### Key Endpoints Used

#### Get User by Query
`GET /user/search?query=<email_or_name>`

- **Response**: Array of user objects
- **Usage**: Get `accountId` for user assignment

#### Get Issue Transitions
`GET /issue/{key}/transitions?expand=transitions.fields`

- **Response**: Available transitions from current status
- **Usage**: Find transition ID for status changes

#### Transition Issue
`POST /issue/{key}/transitions`

- **Request Body**:
  ```json
  {
    "transition": {"id": "<transition_id>"},
    "fields": { ... }
  }
  ```
- **Usage**: Change issue status (only valid method in Jira Cloud)

#### Assign Issue
`PUT /issue/{key}/assignee`

- **Request Body**:
  ```json
  {"accountId": "<user_accountId>"}
  ```
- **Usage**: Assign issue to user

#### Edit Issue
`PUT /issue/{key}`

- **Request Body**:
  ```json
  { "fields": { "priority": {"name": "High"}, "labels": ["ops", "auto"] } }
  ```
- **Usage**: Update issue fields (except status)

#### Search Issues
`GET /search?jql=<JQL>&maxResults=100`

- **Response**: Paginated issue results
- **Usage**: Mass issue processing

#### Add Comment
`POST /issue/{key}/comment`

- **Request Body**:
  ```json
  {
    "body": {
      "version": 1,
      "type": "doc",
      "content": [ ... ] // ADF format
    }
  }
  ```
- **Usage**: Add rich comments to issues

## Rate Limiting
The service implements adaptive rate limiting:

- **Default**: 60 requests per minute
- **Burst**: 10 requests
- **Per-IP Limiting**: Configurable thresholds
- **Per-Endpoint Limiting**: Different limits for different endpoints

## Error Handling
All error responses follow the structured format:

```json
{
  "error": "GITLAB_API_ERROR",
  "message": "GitLab API create_issue failed",
  "details": "Connection timeout after 30s",
  "context": {
    "operation": "create_issue",
    "status_code": 502,
    "project_id": "123"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req-abc123",
  "user_message": "Unable to create issue. Please try again later.",
  "retry_after_seconds": 120
}
```

## Error Codes

### Client Errors (4xx)
- `INVALID_REQUEST`: Malformed or invalid request data
- `UNAUTHORIZED`: Authentication failed
- `FORBIDDEN`: Access denied
- `NOT_FOUND`: Resource not found
- `RATE_LIMITED`: Request rate limit exceeded
- `INVALID_SIGNATURE`: Webhook signature validation failed
- `INVALID_WEBHOOK`: Webhook format or content invalid
- `VALIDATION_FAILED`: Data validation errors

### Server Errors (5xx)
- `INTERNAL_ERROR`: General internal server error
- `SERVICE_UNAVAILABLE`: Service temporarily unavailable
- `TIMEOUT`: Request timeout
- `DATABASE_ERROR`: Database operation failed
- `EXTERNAL_API_ERROR`: External service error

### GitLab/Jira Specific Errors
- `GITLAB_API_ERROR` / `JIRA_API_ERROR`: API-specific failures
- `GITLAB_TIMEOUT` / `JIRA_TIMEOUT`: API timeout errors
- `GITLAB_RATE_LIMIT` / `JIRA_RATE_LIMIT`: API rate limiting
- `GITLAB_UNAUTHORIZED` / `JIRA_UNAUTHORIZED`: API authentication errors
- `GITLAB_NOT_FOUND` / `JIRA_NOT_FOUND`: API resource not found

## Performance Characteristics

### Latency
- **P50**: 150ms
- **P95**: 350ms
- **P99**: 600ms

### Throughput
- **Max**: 100 requests/second (depends on Jira API rate limits)
- **Burst**: 200 requests (with queueing)

### Queue Processing
- **Max Queue Size**: 1000 jobs
- **Processing Time**: 200ms/job (average)
- **Worker Scaling**: 2-32 workers (dynamic)

## Bidirectional Automation Guide

This section provides a comprehensive guide for automating Jira Cloud workflows using the REST API v3.

### 0) Authentication and Base Constants
- **Basic Auth**: Email + API token (passwords not used in Cloud). Token is created in Atlassian Account and used instead of password. Alternative — OAuth 2.0 / Personal Access Tokens (if enabled).
- **Base URL**: `https://<your-domain>.atlassian.net/rest/api/3`

### 1) Getting User ID by Email/Query
- `GET /user/search?query=<email_or_name>` → parse `.accountId` from the first result

### 2) Checking Available Transitions for an Issue
- `GET /issue/{key}/transitions?expand=transitions.fields` (only shows allowed transitions from current status)

### 3) Transitioning an Issue to a New Status
- `POST /issue/{key}/transitions` with body:
```json
{
  "transition": {"id": "<transition_id>"},
  "fields": { ... }  // if the transition has a screen/validators
}
```

### 4) Assigning an Issue
- `PUT /issue/{key}/assignee` with body:
```json
{"accountId": "<user_accountId>"}  // assign to user
{"accountId": null}              // Unassigned (if allowed)
{"accountId": "-1"}              // Default assignee for the project
```

### 5) Editing Issue Fields
- `PUT /issue/{key}` with body:
```json
{ "fields": { "priority": {"name": "High"}, "labels": ["ops", "auto"] } }
```

### 6) Bulk Issue Processing
- `GET /search?jql=<JQL>&maxResults=100` (pagination via `startAt`)
- Iterative processing through transitions and updates

### 7) Additional Actions
- **Comments**: `POST /issue/{key}/comment` (ADF format)
- **Links**: `POST /issueLink` (e.g., `type.name="Relates"`)
- **Watchers**: `POST /issue/{key}/watchers` with `{"accountId": "..."}`

### 8) Common Recipes
- **A**: "If issue is in To Do + High priority → In Progress + default assignee"
- **B**: "Assign by email and immediately transition to Done with resolution"
- **C**: "Scheduled/external triggers via Search → transition loop"

### 9) Permissions and Guarantees
- **Project Permissions Check**: Verify user has required permissions
- **Workflow Constraints**: Account for workflow restrictions
- **Error Handling**: Process 403/400 errors appropriately

### 10) Pagination and Reliability
- **Pagination**: Use `startAt`/`maxResults` for large result sets
- **Backoff**: Implement retry with backoff for 429 errors
- **Idempotency**: Ensure operations are idempotent where possible
