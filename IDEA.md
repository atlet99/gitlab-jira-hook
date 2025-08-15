# GitLab ↔ Jira Hook - Project Development Plan

## 📋 Current State

### ✅ What's Implemented

#### 🏗️ **Architecture and Infrastructure**
- ✅ Clean Architecture with layer separation (cmd, internal, pkg)
- ✅ Configuration via environment variables
- ✅ Structured logging with log/slog
- ✅ Graceful shutdown with context timeout
- ✅ Docker multi-stage build
- ✅ Comprehensive Makefile with CI/CD targets
- ✅ Security scanning (gosec, govulncheck)
- ✅ SBOM generation (Syft)
- ✅ Code quality tools (golangci-lint, staticcheck, errcheck)

#### 🔧 **Core Functionality**
- ✅ HTTP server on port 8080
- ✅ GitLab System Hook endpoint (`/gitlab-hook`)
- ✅ Health check endpoint (`/health`)
- ✅ GitLab webhook event parsing (push, merge_request)
- ✅ Jira issue ID extraction from commit messages and MR titles
- ✅ Jira Cloud REST API client with Basic Auth
- ✅ Adding comments to Jira issues
- ✅ GitLab secret token validation

#### 🧪 **Testing and Quality**
- ✅ Unit tests for parser
- ✅ 100% passing all linters
- ✅ Security (HTTPS, input validation, secrets management)
- ✅ Error handling with wrapped errors
- ✅ Comprehensive logging

### 🔄 **In Progress**

#### 📊 **Monitoring and Metrics**
- 🔄 Prometheus metrics endpoint
- 🔄 Structured logging for production
- 🔄 Health check with dependencies

## 🚀 Development Roadmap

### 🎯 **Phase 1: Feature Expansion**

#### **Event Processing** ✅ **COMPLETED**
- ✅ Support for all GitLab System Hook events:
  - ✅ `project_create`, `project_destroy`
  - ✅ `user_create`, `user_destroy`
  - ✅ `user_add_to_team`, `user_remove_from_team`
  - ✅ `user_add_to_group`, `user_remove_from_group`
- ✅ Support for GitLab Project Hooks (not just System Hooks)
- ✅ Event filtering by projects/groups
- ✅ Rate limiting for Jira API calls
- ✅ Retry mechanism with exponential backoff

#### **Jira Integration**
- **Jira Webhook Support** (based on [Jira Cloud REST API v3](https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/#version)):
  - [x] Support for Jira Platform Webhooks (Admin-defined)
  - [x] Support for Jira Automation Rule Webhooks
  - [x] Webhook signature verification using JWT tokens
  - [x] Dynamic URL variables (${issue.key}, ${project.id}, ${user.accountId})
  - [x] JQL filters for webhook events
  - [x] Support for all Jira events:
    - [x] `jira:issue_created`, `jira:issue_updated`, `jira:issue_deleted`
    - [x] `comment_created`, `comment_updated`, `comment_deleted`
    - [x] `worklog_created`, `worklog_updated`, `worklog_deleted`
    - [x] `sprint_created`, `sprint_started`, `sprint_closed`
    - [x] `version_released`, `version_unreleased`
    - [x] `project_created`, `project_updated`, `project_deleted`
    - [x] `user_created`, `user_updated`, `user_deleted`

- **Enhanced Jira API Integration**:
  - [x] Upgrade to Jira REST API v3 with ADF support
  - [x] Implement proper authentication (OAuth 2.0 for production)
  - [x] Add validation layer for ADF content with plain text fallback
  - [x] Implement resource expansion (expand parameter)
  - [x] Pagination support for bulk operations
  - [x] Support for JQL search and filtering
  - [x] Add issue linking capabilities
  - [x] Support for custom fields and their values
  - [x] Implement transition handling via `/issue/{key}/transitions` (not PUT `/issue`)
  - [x] Use accountId instead of username for user references
  - [x] Idempotent operations with status validation before transitions

#### **Bidirectional Synchronization**

**Implementation Strategy**:
- Implement transactional processing with retry mechanisms for all sync operations
- Use event sourcing pattern to track synchronization state
- Implement idempotency keys for all operations to prevent duplicate processing
- Add comprehensive audit logging for all synchronization activities

**Jira → GitLab Webhook Handler**:
  - [ ] Jira webhook endpoint (`/jira-webhook`)
  - [ ] JWT token validation for webhook security using Atlassian's public keys
  - [ ] Event filtering using JQL expressions for selective processing
  - [ ] Event transformation to GitLab issue format with field mapping
  - [ ] Automatic GitLab issue creation from Jira with proper labels and milestones
  - [ ] Status synchronization workflow:
    * GET `/issue/{key}/transitions` to verify available transitions
    * POST `/issue/{key}/transitions` with transition ID for status changes
    * Idempotency check: GET `/issue/{key}` to verify current status before transition
    * Fallback to manual status mapping when workflow differs between systems
  - [ ] Comment synchronization:
    * ADF validation with automatic fallback to plain text
    * Comment metadata preservation (author, timestamp)
    * Cross-reference linking between systems
  - [ ] Assignee synchronization:
    * PUT `/issue/{key}/assignee` with accountId (not username)
    * Special values handling: `null` for Unassigned, `-1` for Default assignee
    * User mapping between Jira and GitLab accounts
  - [x] Conflict resolution strategies:
    * Last-write-wins with timestamp comparison
    * Manual resolution queue for critical conflicts
    * Audit trail with before/after state snapshots
    * Notification system for conflict detection
  - [x] Idempotent operations:
    * Current status validation before transitions
    * Retry handling with exponential backoff for 429/5xx errors
    * Transactional processing with rollback capability

**GitLab → Jira Synchronization Enhancements**:
  - [x] Two-way status mapping configuration
  - [x] Custom field synchronization with type conversion
  - [x] Label ↔ Component mapping
  - [x] Milestone ↔ Version synchronization
  - [x] Merge request ↔ Pull request linking
  - [x] Branch URL tracking in Jira issues
  - [x] Commit message parsing for Jira issue updates

#### **Advanced Features**
- Service Management (JSM) Support**:
  - [ ] Service desk integration
  - [ ] Request type handling
  -  ] SLA management
  - [ ] Customer portal integration

- [ ] **Jira Software (Agile) Support**:
  - [ ] Sprint management
  - [ ] Board and backlog integration
  -  and story linking
  - [ ] Velocity tracking

- [ ] **Advanced Integration Features**:
  - [ ] Custom field mapping with ADF support
  - [ ] Workflow transition triggers
  - [ ] Bulk operations with pagination
  - [ ] Real-time synchronization
  - [ ] Conflict detection and resolution
  - [ ] Audit trail and logging

### 🎯 **Phase 2: Enterprise Capabilities**

#### **Production Readiness**
- [ ] High Availability (HA) deployment
- [ ] Load balancing and horizontal scaling
- [ ] Database persistence (PostgreSQL/MySQL)
- [ ] Message queue integration (Redis/RabbitMQ)
- [ ] Distributed tracing (OpenTelemetry)
- [ ] Advanced monitoring (Prometheus + Grafana)
- [ ] Alerting and incident management
- [ ] Backup and disaster recovery

#### **Multi-tenant Architecture**
- [ ] Multi-tenant support
- [ ] Tenant isolation
- [ ] Resource quotas
- [ ] Usage analytics
- [ ] Billing integration
- [ ] Self-service portal

#### **Enterprise Features**
- [ ] SSO integration (SAML, OAuth2)
- [ ] Role-based access control (RBAC)
- [ ] Audit logging
- [ ] Compliance reporting (SOX, GDPR)
- [ ] Advanced security features
- [ ] API rate limiting
- [ ] Webhook signature verification

### 🎯 **Phase 3: Automation**

#### **Advanced Automation**
- [ ] Visual workflow builder
- [ ] Custom automation rules
- [ ] Integration marketplace
- [ ] Plugin architecture
- [ ] Custom webhook transformations
- [ ] Advanced templating engine

## 🛠️ **Technical Improvements**

### **Architectural Enhancements**
- [ ] Event sourcing for audit trail
- [ ] CQRS pattern for read/write separation
- [ ] Microservices decomposition
- [ ] API Gateway integration
- [ ] Service mesh (Istio/Linkerd)
- [ ] Kubernetes native deployment

### **Performance Optimizations**
- [ ] Connection pooling for Jira API
- [ ] Caching layer (Redis)
- [ ] Database query optimization
- [ ] Async processing with workers
- [ ] Batch processing for bulk operations
- [ ] CDN integration for static assets

### **Security Enhancements**
- [ ] OAuth2/JWT authentication
- [ ] Webhook signature verification
- [ ] Rate limiting per tenant
- [ ] Input sanitization
- [ ] SQL injection prevention
- [ ] XSS protection
- [ ] CSRF protection



## 🔧 **Tools and Technologies**

### **Current Stack**
- **Language**: Go 10.21+
- **Framework**: Standard library (net/http)
- **Logging**: log/slog (structured)
- **Configuration**: Environment variables
- **Containerization**: Docker
- **CI/CD**: Makefile + GitHub Actions
- **Testing**: Standard library + testify
- **Security**: gosec, govulncheck
- **Documentation**: README.md + inline godoc
- **Jira Integration**: REST API v3 with Basic Auth (OAuth20planned)

### **Planned Stack**
- **Database**: PostgreSQL (primary), Redis (cache)
- **Message Queue**: RabbitMQ/Apache Kafka
- **Monitoring**: Prometheus + Grafana
- **Tracing**: OpenTelemetry + Jaeger
- **API Gateway**: Kong/Envoy
- **Service Mesh**: Istio/Linkerd
- **Orchestration**: Kubernetes
- **Security**: Vault (secrets), OAuth2/JWT
- **Jira Integration**: 
  - REST API v3 with OAuth 2.0authentication
  - Atlassian Document Format (ADF) support
  - Webhook signature verification (JWT)
  - Resource expansion and pagination
  - JQL search and filtering capabilities

## 🔗 **Links and Resources**

### **Documentation**
- [GitLab System Hooks](https://docs.gitlab.com/ee/system_hooks/system_hooks.html)
- [Jira Cloud REST API](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
- [Jira Webhooks Documentation](https://developer.atlassian.com/cloud/jira/platform/webhooks/)
- [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-token)

### **Standards and Best Practices**
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [REST API Design](https://restfulapi.net/)
-Webhook Security](https://webhooks.fyi/)

## 🔧 **Jira API v3 Integration Requirements**

### **Authentication & Security**
- **Basic Auth**: For development and testing (email + API token)
- **OAuth 2.0**: For production deployments (recommended by Atlassian)
- **Webhook Security**: JWT token verification for incoming webhooks
- **Rate Limiting**: Respect Jira's rate limits (1000 requests per hour per user)

### **API Endpoints & Features**
- **Base URL**: `https://<site-url>/rest/api/3/`
- **Atlassian Document Format (ADF)**: Strict validation implemented with fallback to plain text
- **Transition Handling**: Status changes must use POST `/issue/{key}/transitions` with transition ID (obtained via GET `/issue/{key}/transitions`)
- **Assignee Management**: Use PUT `/issue/{key}/assignee` with accountId (not username). Special values: `null` for Unassigned, `-1` for Default assignee
- **Resource Expansion**: Use `expand` parameter for additional data
- **Pagination**: Standard pagination for large collections with backoff for 429
- **JQL Support**: Advanced search and filtering capabilities
- **Idempotency**: Operations check current state before execution (e.g., verify status before transition)

### **Webhook Integration**
- **Incoming Webhooks**: Handle Jira events with JWT validation
- **Outgoing Webhooks**: Send GitLab events to Jira with proper authentication
- **Dynamic Variables**: Support for ${issue.key}, ${project.id}, ${user.accountId}
- **Event Filtering**: JQL-based filtering for webhook events
- **Error Handling**: Comprehensive error responses with retry strategies

### **Data Synchronization**
- **Bidirectional Sync**: GitLab ↔ Jira with conflict resolution and audit trail
- **Real-time Updates**: Webhook-based event processing with idempotency checks
- **Custom Fields**: Support for Jira custom fields with ADF validation
- **Issue Linking**: Link related issues across systems using POST `/issueLink`
- **Status Synchronization**: Verified through workflow transition validation
- **Comment Synchronization**: ADF validation with fallback to plain text

### **Advanced Features**
- **Service Management (JSM)**: Service desk integration
- **Agile Support**: Sprint management, board integration
- **Workflow Integration**: Trigger Jira workflow transitions
- **Bulk Operations**: Efficient handling of large datasets

---

*Last updated: 2025
*Document version: 1.0*
