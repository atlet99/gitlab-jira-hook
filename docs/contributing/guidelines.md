# Contribution Guidelines

## Code Structure and Organization

The GitLab ↔ Jira Hook service follows Go best practices and clean architecture principles. Understanding the code structure is essential for making effective contributions.

### Project Layout
```
.
├── cmd/                    # Application entry points
├── internal/               # Private application code
│   ├── async/              # Async processing components
│   ├── cache/              # Caching implementation
│   ├── config/             # Configuration management
│   ├── errors/             # Error handling utilities
│   ├── gitlab/             # GitLab integration
│   ├── jira/               # Jira integration
│   ├── monitoring/         # Monitoring and observability
│   ├── server/             # HTTP server implementation
│   ├── sync/               # Bidirectional sync logic
│   └── ...                 # Other internal packages
├── pkg/                    # Public libraries
│   └── logger/             # Structured logging
└── ...                     # Other top-level directories
```

### Key Design Patterns
- **Clean Architecture**: Clear separation between business logic and infrastructure
- **Dependency Injection**: All components are injected via interfaces
- **Composition over Inheritance**: Extensive use of Go interfaces
- **Error Wrapping**: Using Go 1.13+ error wrapping with `%w`
- **Context Propagation**: All operations use `context.Context`
- **Structured Logging**: Using `log/slog` with JSON output

## Making a Good Pull Request

### Before You Start
1. Check if an issue exists for your work (create one if needed)
2. Discuss major changes in the issue before implementation
3. Ensure your work aligns with the project's roadmap

### Pull Request Requirements
- **Clear Description**: Explain what the PR does and why
- **Issue Reference**: Link to the relevant issue (e.g., "Fixes #123")
- **Tests**: Include appropriate unit/integration tests
- **Documentation**: Update relevant documentation
- **Small Scope**: Keep PRs focused on a single change
- **Conventional Commits**: Follow [conventional commits](https://www.conventionalcommits.org/) format

### Example PR Description
```markdown
## Summary
This PR implements JWT validation for Jira Connect apps, addressing #45.

## Changes
- Added `jwt_validator.go` with RS256 and HS256 support
- Integrated with existing authentication system
- Added comprehensive test coverage

## Testing
- Added 15 new test cases covering all validation scenarios
- Verified with real Jira Connect app tokens
- Benchmarked performance impact (negligible)

## Documentation
- Updated security.md with JWT implementation details
- Added examples to API reference
```

## Code Review Expectations

### What We Look For
- **Correctness**: Does the code work as intended?
- **Readability**: Is the code clear and well-structured?
- **Test Coverage**: Are all edge cases covered?
- **Performance**: Any potential bottlenecks?
- **Error Handling**: Proper error wrapping and recovery?
- **Documentation**: Clear comments and documentation?

### Common Feedback Areas
- Missing or insufficient test coverage
- Inadequate error handling
- Lack of performance considerations
- Insufficient documentation
- Violations of Go idioms
- Overly complex implementations

## Development Workflow

### Setting Up Your Environment
1. Install Go 1.21+
2. Clone the repository: `git clone https://github.com/atlet99/gitlab-jira-hook.git`
3. Install dependencies: `make deps`
4. Run tests: `make test`
5. Start development server: `make run`

### Testing Guidelines
- **Unit Tests**: Test individual functions with table-driven tests
- **Integration Tests**: Test component interactions
- **Performance Tests**: Benchmark critical paths
- **Error Scenarios**: Test all error conditions

Example test structure:
```go
func TestValidateRequest(t *testing.T) {
    tests := []struct {
        name          string
        setup         func() *AuthValidator
        request       *http.Request
        body          []byte
        expectedValid bool
        expectedError string
    }{
        {
            name: "Valid HMAC signature",
            setup: func() *AuthValidator {
                return NewAuthValidator(logger, []byte("secret"), false)
            },
            request: func() *http.Request {
                req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(`{"test":"data"}`))
                req.Header.Set("X-Atlassian-Webhook-Signature", "valid-signature")
                return req
            }(),
            body:          []byte(`{"test":"data"}`),
            expectedValid: true,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            validator := tt.setup()
            result := validator.ValidateRequest(context.Background(), tt.request, tt.body)
            if result.Valid != tt.expectedValid {
                t.Errorf("ValidateRequest() valid = %v, want %v", result.Valid, tt.expectedValid)
            }
            // More assertions...
        })
    }
}
```

## Style Guidelines

### Go Formatting
- Follow `gofmt` standards
- Use 4-space indentation
- Maximum line length: 120 characters
- Use `golangci-lint` for static analysis

### Naming Conventions
- Use clear, descriptive names
- Avoid abbreviations unless they're well-known
- Use `CamelCase` for exported identifiers
- Use `snake_case` for configuration variables

### Comments and Documentation
- Write comprehensive godoc for all exported functions
- Include examples where helpful
- Document non-obvious implementation details
- Keep comments up-to-date with code changes

## Performance Considerations

### Critical Paths
- Webhook processing pipeline
- Async job queue management
- Cache operations
- Error recovery mechanisms

### Profiling
Use Go's built-in profiling tools:
```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.

# Memory profiling
go test -memprofile=mem.prof -bench=.

# Analyze profiles
go tool pprof -http=:8080 cpu.prof
```

## Security Guidelines

### Input Validation
- Validate all incoming webhook data
- Sanitize user-provided content
- Use parameterized queries for database access
- Implement proper rate limiting

### Secrets Management
- Never hardcode secrets
- Use environment variables for production secrets
- Support secret management systems
- Rotate secrets regularly

## Getting Help

If you need assistance:
1. Check existing issues for similar questions
2. Join the community Slack channel
3. Ask in the relevant issue thread
4. For urgent matters, tag maintainers in your PR

## Next Steps

1. Create an issue for your proposed change
2. Fork the repository
3. Create a feature branch
4. Implement your changes
5. Write tests and documentation
6. Submit a pull request
