# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.4] - 2025-07-18

### Added
- Support for GitLab `repository_update` system hook event
- Enhanced repository update event processing with detailed change information
- New test coverage for repository update event handler

### Fixed
- Fixed JSON parsing error for GitLab commit file arrays: changed Commit struct fields Added, Modified, Removed from string to []string to match GitLab webhook payload format
- Updated commit comment formatting to properly handle file arrays using strings.Join()
- Added strings package import in handler.go for array joining functionality
- Resolved webhook processing failures caused by unmarshal errors when GitLab sends file information as arrays
- Enhanced project filtering logic to support group-based filtering: now allows projects by group prefix (e.g., "devops" allows "devops/login/stg")
- Added strings.HasPrefix() support for both System Hook and Project Hook handlers
- Added comprehensive tests for group prefix filtering functionality

## [0.1.3] - 2025-07-17

### Fixed
- Fixed event filtering by project and group: now supports comparison by both name (Name) and full path (PathWithNamespace, FullPath), as well as additional fields (ProjectName, GroupName). This ensures correct filtering for all GitLab event types and passes all unit tests.
- Improved logging of event filtering reasons (displays project/group path and list of allowed values).

### Changed
- Removed nginx and all related files from docker-compose and repository to simplify local and production deployment.
- docker-compose now runs only the main service on the required port without unnecessary proxies.

## [0.1.2] - 2025-07-17

### Added
- Docker Compose support for easy local development and deployment
- Simple nginx reverse proxy configuration for localhost
- Environment setup script (`scripts/setup-env.sh`) for automated configuration
- Comprehensive Makefile with modern build flags and version management
- Version package with build-time variables and getters
- `--version` flag support in main application
- Enhanced security scanning with proper report handling
- GitHub Actions workflows for CI/CD pipeline
- Release workflow with Cosign key-based signing
- Cross-platform binary builds for multiple architectures
- SBOM (Software Bill of Materials) generation with Syft
- Comprehensive code quality checks and static analysis
- Vulnerability scanning with govulncheck
- Error checking with errcheck tool

### Changed
- Updated Makefile with modern versioning and build flags
- Enhanced security scan commands to handle missing reports gracefully
- Improved Cosign signing workflow with proper key management
- Updated release workflow with better changelog generation
- Enhanced Docker Compose configuration for simplicity
- Simplified nginx configuration for localhost development
- Updated version management to use build-time variables

### Fixed
- Fixed Makefile build target duplication warnings
- Corrected security scan report handling when no issues found
- Fixed Cosign verification instructions in release workflow
- Resolved staticcheck errors with proper string handling
- Fixed version package integration with main application
- Corrected Docker Compose environment variable handling

### Security
- Implemented Cosign key-based signing for release binaries
- Enhanced security scanning with multiple output formats
- Added vulnerability scanning to CI/CD pipeline
- Improved SBOM generation for supply chain security

### Documentation
- Added Docker Compose setup documentation
- Updated release workflow documentation
- Enhanced Makefile documentation with all available targets
- Added version management documentation

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

[Unreleased]: https://github.com/atlet99/gitlab-jira-hook/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/atlet99/gitlab-jira-hook/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/atlet99/gitlab-jira-hook/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/atlet99/gitlab-jira-hook/compare/v0.1.0...v0.1.2
[0.1.0]: https://github.com/atlet99/gitlab-jira-hook/releases/tag/v0.1.0 