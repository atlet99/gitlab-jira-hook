# Testing Guidelines

## Test Structure and Organization

The GitLab ↔ Jira Hook service follows Go testing best practices with a comprehensive test suite covering all critical functionality. Understanding the test structure is essential for maintaining high code quality.

### Test File Organization
- **Unit Tests**: Located in the same package directory with `_test.go` suffix
- **Integration Tests**: In `internal/*/integration_test.go` files
- **Benchmarks**: In `internal/benchmarks/` directory
- **Test Helpers**: In `internal/testutil/` (to be created)

### Test Types
| Test Type | Location | Coverage Target | Run Command |
|-----------|----------|-----------------|-------------|
| Unit Tests | `*_test.go` | Business logic, individual functions | `make test` |
| Integration Tests | `*_integration_test.go` | Component interactions, external dependencies | `make test-integration` |
| End-to-End Tests | `test/e2e/` | Full workflow validation | `make test-e2e` |
| Performance Benchmarks | `internal/benchmarks/` | Critical path optimization | `make bench` |
| Error Recovery Tests | `internal/errors/recovery_test.go` | Failure scenarios and recovery | `make test-recovery` |

## Writing Effective Tests

### Table-Driven Tests
All unit tests should use table-driven patterns for comprehensive coverage:

```go
func TestProcessWebhook(t *testing.T) {
    tests := []struct {
        name          string
        input         []byte
        expectedError error
        expectedCount int
    }{
        {
            name:          "Valid push event",
            input:         []byte(validPushEvent),
            expectedError: nil,
            expectedCount: 1,
        },
        {
            name:          "Invalid event type",
            input:         []byte(invalidEventType),
            expectedError: errors.New("unsupported event type"),
            expectedCount: 0,
        },
        // Add more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            count, err := processor.Process(tt.input)
            if !errors.Is(err, tt.expectedError) {
                t.Errorf("Process() error = %v, want %v", err, tt.expectedError)
            }
            if count != tt.expectedCount {
                t.Errorf("Process() count = %d, want %d", count, tt.expectedCount)
            }
        })
    }
}
```

### Key Testing Principles
- **Isolation**: Each test should be independent and not rely on global state
- **Determinism**: Tests should produce the same results every time
- **Readability**: Clear test names and structure
- **Completeness**: Cover success, failure, and edge cases
- **Performance**: Keep tests fast (aim for <100ms per test)

## Mocking Strategies

### Interface-Based Mocking
Use Go interfaces to enable clean mocking:

```go
// Define interface for dependency
type JiraClient interface {
    CreateComment(ctx context.Context, issueID string, comment string) error
}

// In tests, use mock implementation
type mockJiraClient struct {
    createCommentFunc func(ctx context.Context, issueID string, comment string) error
}

func (m *mockJiraClient) CreateComment(ctx context.Context, issueID string, comment string) error {
    return m.createCommentFunc(ctx, issueID, comment)
}

// Usage in test
client := &mockJiraClient{
    createCommentFunc: func(ctx context.Context, issueID string, comment string) error {
        if issueID == "ABC-123" {
            return nil
        }
        return errors.New("invalid issue")
    },
}
```

### HTTP Server Mocking
For HTTP dependencies, use `httptest`:

```go
func TestGitLabClient(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/api/v4/projects" {
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode([]Project{{ID: 1, Name: "Test Project"}})
            return
        }
        w.WriteHeader(http.StatusNotFound)
    }))
    defer server.Close()

    client := NewGitLabClient(server.URL, "token")
    projects, err := client.GetProjects(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if len(projects) != 1 {
        t.Errorf("Expected 1 project, got %d", len(projects))
    }
}
```

## Error Handling Tests

Comprehensive error testing is critical for reliability:

```go
func TestErrorRecovery(t *testing.T) {
    tests := []struct {
        name           string
        initialError   error
        expectedAction RecoveryAction
        expectedError  error
    }{
        {
            name:           "Context canceled",
            initialError:   context.Canceled,
            expectedAction: SkipRecovery,
            expectedError:  context.Canceled,
        },
        {
            name:           "Temporary network error",
            initialError:   &net.OpError{Temporary: true},
            expectedAction: Retry,
            expectedError:  nil,
        },
        {
            name:           "Permanent error",
            initialError:   errors.New("invalid configuration"),
            expectedAction: Fail,
            expectedError:  errors.New("invalid configuration"),
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            action, err := recoverySystem.HandleError(context.Background(), tt.initialError)
            if action != tt.expectedAction {
                t.Errorf("HandleError() action = %v, want %v", action, tt.expectedAction)
            }
            if !errors.Is(err, tt.expectedError) {
                t.Errorf("HandleError() error = %v, want %v", err, tt.expectedError)
            }
        })
    }
}
```

## Performance Benchmarking

### Writing Benchmarks
Benchmarks should focus on critical paths:

```go
func BenchmarkProcessWebhook(b *testing.B) {
    processor := NewWebhookProcessor()
    payload := loadTestPayload("large-push-event.json")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = processor.Process(payload)
    }
}

func BenchmarkCacheHitRate(b *testing.B) {
    cache := NewAdvancedCache()
    for i := 0; i < 1000; i++ {
        cache.Set(fmt.Sprintf("key-%d", i), "value")
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = cache.Get(fmt.Sprintf("key-%d", i%1000))
    }
}
```

### Interpreting Results
Run benchmarks with:
```bash
go test -bench=. -benchmem ./internal/...
```

Key metrics to monitor:
- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Number of allocations per operation (lower is better)

Compare changes with:
```bash
# Run before changes
go test -bench=. -benchmem ./internal/... > before.txt

# Make changes

# Run after changes
go test -bench=. -benchmem ./internal/... > after.txt

# Compare
benchcmp before.txt after.txt
```

## Test Coverage Requirements

### Minimum Coverage Standards
- **Core Business Logic**: 90%+ statement coverage
- **Error Handling**: 100% coverage of error paths
- **Critical Paths**: 100% coverage with performance benchmarks
- **Public APIs**: 100% coverage of exported functions

### Generating Coverage Reports
```bash
# Generate coverage profile
make coverage

# View HTML report
open coverage.html

# Check coverage percentage
go tool cover -func=coverage.out
```

## Integration Testing

### Setting Up Test Dependencies
Use Docker Compose for integration test dependencies:

```go
func setupIntegrationTest(t *testing.T) (func(), error) {
    ctx := context.Background()
    compose, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    if err != nil {
        return nil, err
    }

    cleanup := func() {
        compose.Terminate(ctx)
    }

    // Wait for services to be ready
    if err := waitForService("http://localhost:8080/health"); err != nil {
        cleanup()
        return nil, err
    }

    return cleanup, nil
}
```

### Example Integration Test
```go
func TestEndToEndWebhookFlow(t *testing.T) {
    cleanup, err := setupIntegrationTest(t)
    if err != nil {
        t.Fatal(err)
    }
    defer cleanup()

    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()

    // Configure webhook
    webhookURL := server.URL + "/webhook"
    configureWebhook(t, webhookURL)

    // Trigger GitLab event
    payload := loadTestPayload("push-event.json")
    resp, err := http.Post("http://localhost:8080/gitlab-hook", "application/json", bytes.NewBuffer(payload))
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()

    // Verify Jira comment was created
    assertWebhookProcessed(t, "ABC-123")
}
```

## Testing Best Practices

### 1. Test Setup and Teardown
- Use `t.Cleanup()` for proper resource cleanup
- Reset global state between tests
- Use unique test data to prevent collisions

### 2. Test Data Management
- Store test fixtures in `test/fixtures/`
- Use helper functions to load test data
- Avoid hardcoding test data in test files

### 3. Parallel Testing
- Use `t.Parallel()` for independent tests
- Ensure tests don't share mutable state
- Be cautious with database and network tests

### 4. Error Message Quality
- Include context in error messages
- Show expected vs actual values
- Include relevant test parameters

### 5. Test Maintenance
- Refactor tests when code changes
- Remove obsolete tests
- Update tests when requirements change

## CI/CD Testing Pipeline

Our CI pipeline runs the following test stages:

1. **Unit Tests**: All package unit tests
2. **Integration Tests**: Component integration tests
3. **End-to-End Tests**: Full workflow validation
4. **Performance Tests**: Critical path benchmarks
5. **Security Scans**: Dependency vulnerability checks
6. **Code Quality**: Linting and static analysis

### Local Testing Commands
| Command | Description |
|---------|-------------|
| `make test` | Run all unit tests |
| `make test-race` | Run tests with race detector |
| `make test-integration` | Run integration tests |
| `make test-e2e` | Run end-to-end tests |
| `make bench` | Run performance benchmarks |
| `make coverage` | Generate coverage report |
| `make lint` | Run linters |

## Troubleshooting Tests

### Common Issues and Solutions

#### Flaky Tests
- **Symptoms**: Tests pass locally but fail in CI
- **Causes**: Timing issues, shared state, resource contention
- **Solutions**:
  - Add proper synchronization
  - Use unique test data
  - Increase timeouts
  - Isolate test environment

#### Slow Tests
- **Symptoms**: Long test execution time
- **Causes**: Heavy setup, network calls, database operations
- **Solutions**:
  - Mock external dependencies
  - Use in-memory implementations
  - Parallelize tests
  - Optimize test setup

#### Memory Leaks
- **Symptoms**: Increasing memory usage over test runs
- **Causes**: Unclosed resources, global state
- **Solutions**:
  - Use `t.Cleanup()`
  - Verify resource cleanup
  - Run tests with memory profiling

## Next Steps for Contributors

1. **Before Implementing**:
   - Check if tests exist for the feature
   - Review existing test patterns
   - Consider edge cases to cover

2. **When Implementing**:
   - Write tests first (TDD approach preferred)
   - Ensure 100% coverage of new code
   - Include error scenario tests

3. **Before Submitting**:
   - Run all relevant tests locally
   - Verify coverage hasn't decreased
   - Check CI pipeline results

4. **After Submission**:
   - Address any test failures in CI
   - Update tests if implementation changes
   - Verify all test types pass
