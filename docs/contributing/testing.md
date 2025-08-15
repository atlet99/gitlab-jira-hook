# Testing Guidelines

This document outlines the testing standards and practices for the GitLab ↔ Jira Hook project. Following these guidelines ensures comprehensive test coverage and reliable code quality.

## Testing Philosophy

### Core Principles
- **Test-Driven Development**: Write tests before implementation when possible
- **Behavior Verification**: Test what the code does, not how it does it
- **Isolation**: Each test should be independent and self-contained
- **Determinism**: Tests should produce the same results every time
- **Speed**: Tests should run quickly to enable frequent execution

## Test Structure

### Directory Organization
```
internal/
├── async/
│   ├── adapter_test.go
│   ├── delayed_queue_test.go
│   └── ...
├── cache/
│   ├── advanced_cache_test.go
│   └── ...
├── config/
│   ├── config_test.go
│   └── ...
└── ...
```

- **Test Files**: Named as `package_name_test.go`
- **Test Packages**: Use `_test` suffix (e.g., `package async_test`)
- **Test Data**: Store fixtures in `testdata/` directories

### Test Types
| Type | Location | Purpose | Coverage Target |
|------|----------|---------|----------------|
| Unit Tests | Same directory as code | Test individual functions | 100% of exported functions |
| Integration Tests | `internal/.../integration_test.go` | Test component interactions | Critical paths only |
| End-to-End Tests | `test/e2e/` | Test complete workflows | Key user journeys |
| Performance Tests | `internal/benchmarks/` | Measure performance characteristics | Performance-critical code |
| Fuzz Tests | `internal/fuzz/` | Find edge cases and crashes | Security-critical code |

## Writing Effective Tests

### Table-Driven Tests
Use table-driven tests for multiple input scenarios:

```go
func TestParseJiraIssueID(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        found    bool
    }{
        {"Standard format", "ABC-123", "ABC-123", true},
        {"Lowercase format", "abc-123", "ABC-123", true},
        {"No issue ID", "This is a regular comment", "", false},
        {"Multiple issue IDs", "Fixes ABC-123 and XYZ-456", "ABC-123", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            id, found := parser.ParseJiraIssueID(tt.input)
            assert.Equal(t, tt.expected, id)
            assert.Equal(t, tt.found, found)
        })
    }
}
```

### Test Setup and Teardown
Use setup/teardown functions for shared test resources:

```go
func setupTestEnvironment(t *testing.T) (*gitlab.MockClient, *jira.MockClient, context.Context) {
    t.Helper()
    ctx := context.Background()
    gitlabClient := gitlab.NewMockClient()
    jiraClient := jira.NewMockClient()
    return gitlabClient, jiraClient, ctx
}

func TestWebhookHandler(t *testing.T) {
    gitlabClient, jiraClient, ctx := setupTestEnvironment(t)
    
    // Test setup
    handler := NewWebhookHandler(gitlabClient, jiraClient)
    
    // Test execution
    result := handler.Process(ctx, testWebhookEvent)
    
    // Assertions
    assert.NotNil(t, result)
}
```

### Mocking Strategies
- **Interfaces**: Define interfaces for external dependencies
- **Mock Implementations**: Create mock implementations in test files
- **Testify Mocks**: Use testify/mock for complex mocking scenarios

```go
type MockGitLabClient struct {
    mock.Mock
}

func (m *MockGitLabClient) GetProject(ctx context.Context, id int) (*gitlab.Project, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*gitlab.Project), args.Error(1)
}

func TestEventProcessor(t *testing.T) {
    mockClient := new(MockGitLabClient)
    mockClient.On("GetProject", mock.Anything, 123).Return(&gitlab.Project{Name: "test-project"}, nil)
    
    processor := NewEventProcessor(mockClient)
    project, err := processor.GetProject(context.Background(), 123)
    
    assert.NoError(t, err)
    assert.Equal(t, "test-project", project.Name)
    mockClient.AssertExpectations(t)
}
```

## Testing Best Practices

### Assertions
- Use `testify/assert` for readable assertions
- Prefer specific assertions over generic ones
- Include meaningful error messages

```go
// Good
assert.Equal(t, expected, actual, "processing time should match expected value")

// Avoid
if expected != actual {
    t.Fail()
}
```

### Error Handling Tests
Test all error scenarios with specific error checks:

```go
func TestProcessWebhook_InvalidEvent(t *testing.T) {
    handler := NewWebhookHandler(nil, nil)
    ctx := context.Background()
    
    _, err := handler.Process(ctx, invalidWebhookEvent)
    
    // Check error type
    assert.Error(t, err)
    assert.True(t, errors.Is(err, ErrInvalidWebhook))
    
    // Check error message
    assert.Contains(t, err.Error(), "invalid event type")
}
```

### Context Testing
Test context cancellation and timeouts:

```go
func TestProcess_WithContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Immediately cancel
    
    processor := NewEventProcessor(nil)
    _, err := processor.Process(ctx, testEvent)
    
    assert.Error(t, err)
    assert.True(t, errors.Is(err, context.Canceled))
}
```

## Performance Testing

### Benchmarking
Create benchmarks for performance-critical code:

```go
func BenchmarkParseJiraIssueID(b *testing.B) {
    input := "This commit fixes ABC-123 and XYZ-456 issues"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        parser.ParseJiraIssueID(input)
    }
}
```

### Profiling
Use pprof to identify performance bottlenecks:

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.

# Memory profiling
go test -memprofile=mem.prof -bench=.

# Analyze profiles
go tool pprof cpu.prof
go tool pprof mem.prof
```

### Performance Test Structure
```go
func BenchmarkWebhookProcessing(b *testing.B) {
    // Setup
    handler := setupBenchmarkHandler()
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        handler.Process(ctx, generateTestWebhook())
    }
}

func generateTestWebhook() *gitlab.WebhookEvent {
    // Generate realistic test data
    return &gitlab.WebhookEvent{
        EventType: "push",
        Project:   &gitlab.Project{ID: 123, Name: "test-project"},
        // ... other fields
    }
}
```

## Integration Testing

### Test Containers
Use testcontainers for integration tests with real services:

```go
func TestJiraClient_Integration(t *testing.T) {
    ctx := context.Background()
    
    // Start Jira container
    jiraContainer, err := jiracontainer.StartContainer(ctx)
    require.NoError(t, err)
    defer jiraContainer.Terminate(ctx)
    
    // Create client
    client, err := jira.NewClient(jiraContainer.Endpoint, "admin", "admin")
    require.NoError(t, err)
    
    // Run tests
    project, err := client.GetProject(ctx, "TEST")
    require.NoError(t, err)
    assert.Equal(t, "TEST", project.Key)
}
```

### Test Data Management
- Use factory patterns to generate test data
- Clean up resources after tests
- Isolate test data between tests

```go
func createTestProject(t *testing.T, client *gitlab.Client) *gitlab.Project {
    project, _, err := client.Projects.CreateProject(&gitlab.CreateProjectOptions{
        Name:        gitlab.Ptr(fmt.Sprintf("test-project-%d", time.Now().UnixNano())),
        Description: gitlab.Ptr("Test project for integration tests"),
    })
    require.NoError(t, err)
    t.Cleanup(func() {
        _, _ = client.Projects.DeleteProject(project.ID)
    })
    return project
}
```

## Testing in CI

### CI Pipeline Configuration
```yaml
test:
  stage: test
  script:
    - go test -v -race ./...
    - go test -v -coverprofile=coverage.out ./...
    - go tool cover -func=coverage.out
    - go test -v -bench=. -run=^$ ./...
  coverage: '/coverage: ([0-9.]+)%/'
```

### Required CI Checks
- **Unit Tests**: Must pass with 100% success rate
- **Race Detection**: All tests must pass with `-race` flag
- **Test Coverage**: Minimum 80% overall coverage
- **Performance Tests**: No significant regressions
- **Linting**: Code must pass all linters

## Test Maintenance

### Flaky Test Handling
- Identify flaky tests with `go test -count=100`
- Fix root causes rather than increasing timeouts
- Document known flaky tests with issue references
- Quarantine flaky tests if necessary

### Test Refactoring
- Refactor tests when code changes
- Remove obsolete tests
- Update test data as needed
- Consolidate duplicate test setup code

## Advanced Testing Techniques

### Property-Based Testing
Use gopter for property-based testing:

```go
func TestParseJiraIssueID_Properties(t *testing.T) {
    parameters := gopter.DefaultTestParameters()
    parameters.MinSuccessfulTests = 100
    
    properties := gopter.NewProperties(parameters)
    
    properties.Property("Valid issue ID format is recognized", prop.ForAll(
        func(issueID string) bool {
            // Generate valid issue ID format
            formatted := fmt.Sprintf("%s-%d", strings.ToUpper(issueID[:3]), 123)
            _, found := parser.ParseJiraIssueID(formatted)
            return found
        },
        gen.AlphaString(),
    ))
    
    properties.TestingRun(t)
}
```

### Golden File Testing
Use golden files for complex output validation:

```go
func TestGenerateADFContent(t *testing.T) {
    event := createTestEvent()
    content := adf.GenerateContent(event)
    
    // Update golden file with -update flag
    if *update {
        os.WriteFile("testdata/generate_adf_content.golden", []byte(content), 0644)
        return
    }
    
    // Compare with golden file
    expected, _ := os.ReadFile("testdata/generate_adf_content.golden")
    assert.Equal(t, string(expected), content)
}
```

## Testing Metrics

### Key Metrics to Track
| Metric | Target | Monitoring |
|--------|--------|------------|
| Test Coverage | >80% | CI reports |
| Test Execution Time | <5 minutes | CI timing |
| Flaky Test Rate | 0% | CI tracking |
| Test-to-Code Ratio | 1:1.5 | Code analysis |
| Mutation Score | >80% | Mutation testing |

### Coverage Analysis
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage by package
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
```

## Resources

- [Go Testing Documentation](https://go.dev/doc/tutorial/add-a-test)
- [Testify Assertions](https://github.com/stretchr/testify)
- [Testcontainers for Go](https://golang.testcontainers.org/)
- [Go Fuzzing](https://go.dev/doc/fuzz/)
- [Property-Based Testing with Gopter](https://github.com/leanovate/gopter)
- [Golden File Testing](https://medium.com/@peterjgrainger/golden-file-testing-in-go-9f3bbf193a0a)
