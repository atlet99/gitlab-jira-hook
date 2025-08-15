# Contribution Guidelines

This document outlines the standards and processes for contributing to the GitLab ↔ Jira Hook project. Following these guidelines ensures your contributions align with project standards and increases the likelihood of your pull request being accepted.

## Code Structure & Organization

### Project Layout
The project follows standard Go project structure with clear separation of concerns:

```
internal/
├── async/          # Asynchronous job processing
├── cache/          # Caching implementations
├── config/         # Configuration management
├── gitlab/         # GitLab integration
├── jira/           # Jira integration
├── monitoring/     # Observability features
├── server/         # HTTP server implementation
└── ...             # Other domain-specific packages
```

### Package Organization Principles
- **Single Responsibility**: Each package should have a clear, focused purpose
- **Layered Architecture**: Follow clean architecture principles (domain → application → infrastructure)
- **Interface-Driven**: Define interfaces in the package that uses them
- **Testability**: Design packages to be easily testable in isolation

### Naming Conventions
- **Packages**: Use singular, lowercase names (e.g., `config`, `cache`)
- **Types**: Use PascalCase for exported types (e.g., `WebhookHandler`)
- **Functions**: Use camelCase for exported functions (e.g., `NewWebhookHandler`)
- **Variables**: Use descriptive names (avoid single-letter variables except in loops)

## Design Patterns

### Key Patterns Used
- **Dependency Injection**: All components receive dependencies through constructors
- **Adapter Pattern**: For integrating with external systems (GitLab, Jira)
- **Strategy Pattern**: For configurable behaviors (cache strategies, error recovery)
- **Middleware Pattern**: For HTTP request processing pipeline
- **Worker Pool Pattern**: For async job processing

### Error Handling
- Use Go 1.24 error wrapping: `fmt.Errorf("operation failed: %w", err)`
- Create custom error types for domain-specific errors
- Use `errors.Is` and `errors.As` for error inspection
- Include context in errors: `fmt.Errorf("processing webhook %s: %w", id, err)`

### Context Usage
- Always use `context.Context` for cancellation and timeouts
- Never store context in structs (pass as first parameter to functions)
- Use context values only for request-scoped data
- Create derived contexts with timeouts for external calls

## Pull Request Standards

### Requirements for Acceptance
- **Tests**: All new features must include unit and integration tests
- **Documentation**: Update relevant documentation for new features
- **Formatting**: Code must pass `gofmt` and linter checks
- **Commit Messages**: Follow conventional commits format
- **Size**: Keep PRs focused (ideally < 500 lines of changes)

### Good Pull Request Examples
- **Small Feature**: Adds a single, well-defined capability with tests
- **Bug Fix**: Includes reproduction steps and verification test
- **Refactor**: Improves code structure without changing behavior
- **Documentation**: Comprehensive updates with clear examples

### Code Review Expectations
- **Timeliness**: Reviews will be completed within 2 business days
- **Constructive Feedback**: Focus on improvement, not criticism
- **Technical Depth**: Expect detailed feedback on design decisions
- **Collaboration**: Be prepared to discuss alternatives and iterate

## Development Process

### Branching Strategy
- Create feature branches from `main`
- Name branches using: `feature/<description>`, `fix/<description>`, `refactor/<description>`
- Keep branches up-to-date with `main` through regular rebasing

### Testing Requirements
- **Unit Tests**: Cover all exported functions
- **Integration Tests**: Cover critical integration points
- **Table-Driven Tests**: For functions with multiple input cases
- **Benchmark Tests**: For performance-critical code paths
- **Test Coverage**: Maintain >80% coverage

### Documentation Standards
- **Godoc Comments**: All exported functions must have godoc
- **Examples**: Include usage examples in documentation
- **Error Documentation**: Document all possible error scenarios
- **API Documentation**: Keep OpenAPI spec up-to-date

## Code Quality Standards

### Linting Requirements
All code must pass these linters:
```bash
golangci-lint run --config .golangci.yml
```

### Performance Considerations
- Avoid unnecessary allocations in hot paths
- Use buffered channels where appropriate
- Minimize lock contention in concurrent code
- Profile before optimizing (use pprof)

### Security Requirements
- Validate all external inputs
- Use parameterized queries for database access
- Implement proper authentication/authorization
- Follow OWASP guidelines for web security

## Review Process

### Pull Request Checklist
- [ ] All tests pass
- [ ] Documentation updated
- [ ] Linter checks pass
- [ ] Performance impact analyzed
- [ ] Security implications considered
- [ ] Changelog entry added (if applicable)

### Merge Process
1. PR created with description of changes
2. Automated checks run (CI, linters, tests)
3. At least one maintainer approval required
4. PR squashed and merged to `main`
5. Release notes updated for significant changes

## Community Guidelines

### Communication
- Be respectful in all communications
- Assume positive intent
- Provide constructive feedback
- Document decisions in issues/PRs

### Issue Management
- Search for existing issues before creating new ones
- Provide clear reproduction steps
- Label issues appropriately
- Respond promptly to requests for clarification

## Resources

- [Go Best Practices](https://golang.org/doc/effective_go.html)
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Go Error Handling](https://go.dev/blog/go1.13-errors)
- [Go Generics Tutorial](https://go.dev/doc/tutorial/generics)
