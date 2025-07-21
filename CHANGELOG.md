# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.5] - 2025-07-22

### Added
- **Phase 3: Error Handling & Monitoring Implementation**
  - Distributed tracing with OpenTelemetry integration
  - Advanced monitoring system with Prometheus metrics
  - Error recovery manager with multiple recovery strategies
  - Comprehensive health checks and alerting system
  - Structured logging enhancements with context support
- **Advanced Caching System**
  - Multi-level cache with L1/L2 architecture
  - Advanced cache with multiple eviction strategies (LRU, LFU, FIFO, TTL, Adaptive)
  - Distributed cache with consistent hashing
  - Cache compression and encryption support
  - Comprehensive cache monitoring and statistics
- **Configuration Hot Reload System**
  - Real-time configuration updates without service restart
  - File and environment variable change detection
  - Configurable reload intervals and retry mechanisms
  - Handler system for configuration change notifications
- **Rate Limiting and Security Enhancements**
  - Adaptive rate limiting based on system load
  - Per-IP and per-endpoint rate limiting
  - Security improvements with SHA-256 hashing
  - Protection against Slowloris attacks
  - Comprehensive rate limiting metrics
- **Comprehensive Test Coverage Improvements** for Phase 2 stability
  - Complete test suite for `internal/config` package with 81% coverage
  - Full test coverage for `internal/server` package with 97.2% coverage
  - Comprehensive test suite for `internal/async` package with stability fixes
  - Enhanced test coverage for configuration loading, validation, and edge cases
  - Server lifecycle testing including start, shutdown, and health checks
  - Worker pool configuration and monitoring tests
  - Health check endpoint testing with proper HTTP method handling
  - Performance testing for server creation and health checks
- **Test Infrastructure Enhancements**
  - Helper functions for creating test configurations with proper defaults
  - Fixed test stability issues in async components
  - Improved error handling in test scenarios
  - Enhanced test utilities for configuration management

### Fixed
- **Code Quality and Linter Compliance**
  - Fixed all critical linter errors (54 → 0 in main code)
  - Replaced deprecated MD5 with SHA-256 for security
  - Added proper error handling for all system calls
  - Fixed integer overflow issues in hash functions
  - Replaced magic numbers with named constants
  - Improved code readability and maintainability
- **Security Vulnerabilities**
  - Fixed potential integer overflow in hash calculations
  - Added ReadHeaderTimeout to prevent Slowloris attacks
  - Improved error handling for environment variable operations
  - Enhanced input validation and sanitization
- **Performance Optimizations**
  - Optimized cache eviction algorithms
  - Improved memory usage calculations
  - Enhanced worker pool resource management
  - Better error recovery mechanisms
- **Test Stability and Panic Resolution**
  - Fixed panic in `internal/async` tests due to zero interval in `time.NewTicker`
  - Added proper `ScaleInterval` and `HealthCheckInterval` values in all test configurations
  - Resolved test failures in `internal/config` due to missing required environment variables
  - Fixed test failures in `internal/server` due to nil configuration handling
  - Improved test reliability by adding proper timeouts and synchronization
- **Configuration Test Improvements**
  - Added missing `GITLAB_BASE_URL` environment variable in all config tests
  - Fixed boolean parsing tests to match Go's `strconv.ParseBool` behavior
  - Enhanced test coverage for environment variable parsing and validation
  - Improved test scenarios for configuration edge cases and performance
- **Server Test Enhancements**
  - Fixed server creation tests to handle nil configurations properly
  - Improved health check tests with correct HTTP method expectations
  - Enhanced worker pool configuration tests with proper initialization
  - Added comprehensive server lifecycle testing with graceful shutdown
- **Async Component Test Stability**
  - Fixed all test configurations to include required interval parameters
  - Improved test reliability for delayed queue and worker pool components
  - Enhanced error handling tests for async components
  - Added proper cleanup and shutdown procedures in tests

### Changed
- **API Compatibility**
  - Maintained backward compatibility while improving internal structure
  - Enhanced error messages and logging
  - Improved configuration validation
  - Better separation of concerns in monitoring components
- **Test Configuration Management**
  - Standardized test configuration creation with helper functions
  - Improved test setup and teardown procedures
  - Enhanced test documentation and inline comments
  - Better separation of test concerns and responsibilities

## [0.1.4] - 2025-07-20

### Added
- **Dynamic Job Prioritization System** with auto-detection of optimal resource usage
  - Priority-based job queue with configurable priority levels (High, Normal, Low)
  - Automatic priority assignment based on event types (merge requests get high priority)
  - Customizable priority decider interface for advanced use cases
  - Priority-aware job processing and ordering
- **Delayed Job Scheduling** with proper execution order
  - Delayed job queue for time-based task execution
  - Configurable delay intervals with millisecond precision
  - Automatic job movement from delayed queue to main priority queue
  - Background scheduler with 5ms processing interval for optimal responsiveness
  - Support for delayed merge request processing and rate limiting
- **Advanced Worker Pool with Middleware Support**
  - Middleware chain for job processing (logging, retry, timeout, circuit breaker)
  - Exponential backoff retry mechanism with configurable parameters
  - Circuit breaker pattern for fault tolerance
  - Request timeout handling with configurable timeouts
  - Comprehensive job lifecycle management
- **Auto-Configuration System** for optimal resource usage
  - CPU-based worker count detection (2x CPU cores by default)
  - Memory-based resource limits with automatic adjustment
  - Configurable memory per worker (128MB default) and per job (2MB default)
  - Automatic queue size calculation based on available resources
  - Support for container environments (Docker/Kubernetes) with cgroup detection
- **Enhanced Configuration Management**
  - New configuration parameters for advanced queue management
  - Auto-detection of optimal worker and queue sizes
  - Memory limit detection from cgroups and /proc/meminfo
  - Configurable retry policies and backoff strategies
  - Monitoring and health check configuration options
- **Comprehensive Testing Suite**
  - Unit tests for all new async components
  - Integration tests for delayed job processing
  - Middleware chain testing with various scenarios
  - Performance and scaling tests
  - Error handling and edge case coverage
- **Comprehensive Benchmark Suite** for performance analysis
  - Priority queue operations benchmarking with different priority levels
  - Delayed job processing performance tests with various delay intervals
  - Middleware chain performance analysis for different middleware combinations
  - Resource scaling benchmarks for worker pool scale up/down scenarios
  - Error handling and circuit breaker performance tests
  - Configuration loading and validation performance benchmarks
  - Logging performance tests for text, JSON, and structured logging
  - Memory efficiency benchmarks for event creation and JSON operations
  - Concurrency pattern benchmarks for burst, steady stream, and mixed workloads
  - HTTP request and response performance testing
  - Memory usage and allocation tracking for all components
- **Performance Integration Tests** for comprehensive system validation
  - End-to-end performance testing with realistic workloads
  - Integration tests for async job processing pipeline
  - Performance regression detection and monitoring
- **Documentation Enhancements**
  - Async architecture documentation with detailed system design
  - Broker formula documentation explaining resource calculation algorithms
  - Performance testing guidelines and best practices
  - Comprehensive API documentation for all new components

### Fixed
- **Deadlock Resolution** in worker pool statistics update
  - Fixed race condition in `updateStats` method causing test hangs
  - Removed deferred call holding locks during job processing
  - Improved thread safety in worker pool operations
- **Test Stability Improvements**
  - Fixed timing issues in delayed queue tests with proper sleep intervals
  - Resolved race conditions in worker pool scaling tests
  - Added proper error handling for unchecked return values
  - Improved test synchronization and reliability
- **Benchmark Stability and Performance**
  - Fixed queue overflow issues in benchmark tests by implementing proper flow control
  - Resolved deadlock problems during worker pool shutdown with timeout-based stop mechanism
  - Fixed parallel benchmark execution issues with proper RunParallel implementation
  - Added retry logic for queue timeout errors in performance tests
  - Improved benchmark configuration with optimized parameters for reliable execution
  - Fixed memory allocation tracking and performance metrics collection
- **Configuration Validation**
  - Enhanced configuration loading with proper error handling
  - Added validation for new configuration parameters
  - Improved default value handling and fallback mechanisms
  - Fixed environment variable parsing for new options
- **Code Quality and Standards**
  - Fixed all Russian comments in benchmark code to comply with .cursorrules English-only requirement
  - Improved code documentation and inline comments for better maintainability
  - Enhanced error messages and logging for better debugging experience

### Changed
- **Worker Pool Architecture** completely redesigned for better performance
  - Replaced simple worker pool with priority-based system
  - Added middleware support for extensible job processing
  - Implemented delayed job scheduling for time-sensitive operations
  - Enhanced resource management with auto-detection capabilities
- **Configuration Structure** reorganized for better maintainability
  - Grouped related configuration parameters logically
  - Added comprehensive documentation for all new options
  - Improved default value handling and validation
  - Enhanced environment variable support for new features
- **Performance Optimizations**
  - Reduced scheduler interval to 5ms for faster job processing
  - Optimized memory usage with configurable limits
  - Improved thread safety and concurrency handling
  - Enhanced error recovery and retry mechanisms
- **Benchmark Infrastructure** enhanced for comprehensive performance analysis
  - Restructured benchmark tests with proper categorization and organization
  - Implemented reliable worker pool lifecycle management with timeout-based shutdown
  - Added comprehensive performance metrics collection and reporting
  - Enhanced benchmark configuration with realistic workload simulation
  - Improved parallel benchmark execution with proper synchronization

### Technical
- **New Async Package** with comprehensive job processing capabilities
  - `PriorityQueue` for priority-based job ordering
  - `DelayedQueue` for time-delayed job execution
  - `PriorityWorkerPool` with middleware support
  - `Middleware` interfaces for extensible processing
- **Resource Management** with intelligent auto-detection
  - CPU count detection for optimal worker allocation
  - Memory limit detection from container environments
  - Configurable resource limits and safeguards
  - Automatic fallback to reasonable defaults
- **Enhanced Logging and Monitoring**
  - Structured logging for all async operations
  - Detailed metrics for queue and worker performance
  - Health check integration for monitoring systems
  - Comprehensive error reporting and debugging information
- **Performance Testing Framework** with comprehensive coverage
  - Multi-dimensional performance analysis across different workload patterns
  - Memory efficiency tracking and optimization recommendations
  - Concurrency pattern analysis for optimal resource utilization
  - Integration testing for end-to-end performance validation

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