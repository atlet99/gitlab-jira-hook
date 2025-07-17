# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-07-17

### Added
- Initial MVP release
- GitLab System Hook webhook handler
- GitLab Project Webhook handler with comprehensive event support
- Jira Cloud REST API v3.0 client with ADF (Atlassian Document Format) support
- Issue ID regex parser with enhanced pattern matching
- Environment-based configuration with comprehensive options
- Basic authentication with Jira API tokens
- Support for all GitLab System Hook events:
  - Push events
  - Merge request events
  - Project events (create, update, delete)
  - User events (create, delete)
  - Group events (create, delete)
  - Repository events (create, delete)
  - Tag push events
  - Release events
  - Deployment events
  - Feature flag events
  - Wiki page events
  - Pipeline events
  - Build events
  - Note events (comments)
  - Issue events
- Support for all GitLab Project Webhook events:
  - Push events
  - Merge request events
  - Issue events
  - Note events (comments)
  - Pipeline events
  - Build events
  - Tag push events
  - Release events
  - Deployment events
  - Feature flag events
  - Wiki page events
- Rich enrich-comments for all event types with detailed information
- Branch filtering for push events with wildcard support (* and ?)
- Project and group filtering capabilities
- Rate limiting and retry mechanisms for Jira API calls
- Structured logging with different levels
- Health check endpoint
- Docker support
- Makefile for build automation with comprehensive targets
- Comprehensive unit tests for all components
- Integration tests with Jira API mocks
- Test coverage for filtering functionality
- Configuration examples and documentation
- MIT License

### Changed
- Enhanced Jira comment format using ADF for better readability
- Improved error handling with wrapped errors (Go 1.13+)
- Updated GitLab event types to match official System Hook documentation
- Refactored project structure for better maintainability
- Enhanced configuration management with validation
- Improved logging with structured format
- Updated dependencies to latest compatible versions
- Enhanced Makefile with additional development targets

### Fixed
- Corrected GitLab event field mappings to match official documentation
- Fixed issue ID parsing regex patterns
- Resolved type compatibility issues in tests
- Fixed deprecated function usage
- Corrected package naming conventions
- Fixed linter warnings and errors
- Resolved test failures and improved test reliability

### Security
- Enhanced input validation for all webhook events
- Improved secret token validation
- Added rate limiting to prevent API abuse
- Implemented proper error handling to avoid information disclosure

### Documentation
- Comprehensive README.md with setup instructions
- API reference documentation
- Configuration examples and best practices
- Development guidelines and contributing instructions
- Architecture diagrams and project structure
- Troubleshooting guides

[Unreleased]: https://github.com/atlet99/gitlab-jira-hook/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/atlet99/gitlab-jira-hook/releases/tag/v0.1.0 