# Testing Guide

This document describes how to run tests for the BeyondTrust Terraform Provider.

## Test Structure

The provider includes three types of tests:

1. **Unit Tests** - Fast tests that don't require external dependencies
2. **Acceptance Tests** - End-to-end tests that run against a real BeyondTrust Workload Credentials instance
3. **Integration Tests** - Tests that verify the provider works with Terraform CLI

## Prerequisites

### For Unit Tests
- Go 1.25.8 or later
- No external dependencies required

### For Acceptance Tests
- Go 1.25.8 or later
- Access to a BeyondTrust Workload Credentials instance (local or remote)
- Valid API credentials

## Running Tests

### Unit Tests Only

Unit tests are fast and don't require external services:

```bash
make test-unit
```

Or directly with go test:

```bash
go test -v -cover ./internal/... ./workload_credentials/...
```

### Acceptance Tests

Acceptance tests require a running Workload Credentials instance and proper environment variables.

#### Required Environment Variables

```bash
export BEYONDTRUST_API_URL="https://api.workload-credentials.local"
export BEYONDTRUST_ACCESS_TOKEN="your-access-token"
```

#### Optional Environment Variables

```bash
export BEYONDTRUST_SITE_ID="your-site-uuid"
export BEYONDTRUST_API_VERSION="2026-02-16"
export BEYONDTRUST_TEST_AWS_ROLE_ARN="arn:aws:iam::123456789012:role/test-role"
export BEYONDTRUST_TEST_AWS_ROLE_ARN_2="arn:aws:iam::123456789012:role/test-role-2"
```

#### Run All Acceptance Tests

```bash
make test-acc
```

Or directly with go test:

```bash
TF_ACC=1 go test -v -timeout=120m ./...
```

#### Run Specific Test

```bash
TF_ACC=1 go test -v -timeout=30m -run TestAccFolderResource_basic ./workload_credentials/resources/
```

### Run All Tests

To run both unit and acceptance tests:

```bash
make test
```

## Coverage Reports

### Generate Coverage Report

```bash
make test-coverage
```

### Generate HTML Coverage Report

```bash
make test-coverage-html
```

This creates a `coverage.html` file you can open in a browser.

## Test Organization

### Unit Tests
Located alongside the code they test:
- `internal/client/client_test.go` - Tests for API client (86.4% coverage)
- `internal/client/mock_client_test.go` - Tests for mock client
- `internal/provider/provider_test.go` - Tests for provider configuration (89.5% coverage)
- `internal/acctest/helpers_test.go` - Tests for test helper functions

**Current Coverage: 68.1% overall** (client: 86.4%, provider: 89.5%, acctest: 22.4%)

### Acceptance Test Files
Located in the same package as the resources/data sources:
- `workload_credentials/resources/*_test.go` - Resource acceptance tests
- `workload_credentials/datasources/*_test.go` - Data source acceptance tests
- `workload_credentials/ephemeral/*_test.go` - Ephemeral resource acceptance tests

### Test Helpers
- `internal/acctest/` - Shared test utilities and helpers
- `*/testing.go` - Package-specific test configuration

## Writing New Tests

### Unit Test Patterns

#### Table-Driven Tests

Use table-driven tests for testing multiple scenarios with similar logic:

```go
func TestRandomString(t *testing.T) {
    tests := []struct {
        name   string
        length int
    }{
        {"short string", 5},
        {"medium string", 10},
        {"long string", 20},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := RandomString(tt.length)
            assert.Equal(t, tt.length, len(result))
        })
    }
}
```

#### Testing with Mock HTTP Server

Use `httptest.NewServer` to test HTTP client logic without external dependencies:

```go
func TestDoRequest(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request headers
        assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

        // Return mock response
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    }))
    defer server.Close()

    client, err := NewClient(&Config{
        BaseURL:     server.URL,
        AccessToken: "test-token",
        Timeout:     "30s",
    })
    require.NoError(t, err)

    var result map[string]string
    err = client.Get(context.Background(), "/test", nil, &result)
    assert.NoError(t, err)
    assert.Equal(t, "ok", result["status"])
}
```

#### Testing Provider Configuration

Test provider configuration with environment variable manipulation:

```go
func TestProviderConfigure_EnvVarPrecedence(t *testing.T) {
    // Clear all env vars first
    os.Unsetenv("BEYONDTRUST_API_URL")
    defer os.Unsetenv("BEYONDTRUST_API_URL")

    // Set test env vars
    os.Setenv("BEYONDTRUST_API_URL", "https://env.example.com")

    // Test provider configuration
    ctx := context.Background()
    prov := New("test")()

    // Create config and verify it reads env vars
    // ... (see internal/provider/provider_test.go for full example)
}
```

#### Using Mock Client for Resource Tests

Use the mock client to test resource logic without making HTTP requests:

```go
func TestResourceCreate(t *testing.T) {
    mockClient := client.NewMockClient()

    // Configure expected API response
    mockClient.ExpectResponse(map[string]string{
        "id":   "folder-123",
        "name": "test-folder",
    })

    // Test resource creation logic
    // ... (use mockClient in resource CRUD operations)

    // Verify the right API calls were made
    err := mockClient.AssertCalled(1)
    assert.NoError(t, err)
}
```

#### Testing Error Handling

Test that errors are properly handled and parsed:

```go
func TestHandleErrorResponse(t *testing.T) {
    tests := []struct {
        name         string
        statusCode   int
        responseBody map[string]interface{}
        wantErrMsg   string
    }{
        {
            name:       "not found",
            statusCode: http.StatusNotFound,
            responseBody: map[string]interface{}{
                "message": "Resource not found",
                "code":    "NOT_FOUND",
            },
            wantErrMsg: "Resource not found (code: NOT_FOUND)",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test error handling logic
            // ... (see internal/client/client_test.go for full example)
        })
    }
}
```

### Acceptance Test Example

```go
func TestAccFolderResource_basic(t *testing.T) {
    folderName := acctest.RandomFolderName()

    resource.ParallelTest(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        CheckDestroy:             testAccCheckFolderDestroy,
        Steps: []resource.TestStep{
            {
                Config: testAccFolderResourceConfig_basic(folderName),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName),
                    resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.test", "id"),
                ),
            },
        },
    })
}
```

## Continuous Integration

Tests run automatically on:
- Pull requests to main/develop branches
- Pushes to main/develop branches

See `.github/workflows/tests.yml` for CI configuration.

### CI Test Matrix

The CI pipeline runs:
1. **Unit Tests** - Always runs, no credentials required
2. **Acceptance Tests** - Only runs when credentials are available (main repo, not forks)
3. **Linting** - golangci-lint checks
4. **Documentation** - Validation of Terraform docs
5. **Build** - Cross-platform build verification

## Test Coverage Goals

- **Unit Tests**: ✅ Current: **68.1%** overall, Target: 80%+ coverage of core logic
  - internal/client: 86.4% ✅
  - internal/provider: 89.5% ✅
  - internal/acctest: 22.4% (low due to AWS SDK integration helpers)
- **Acceptance Tests**: Cover all resource CRUD operations (in progress)
- **Integration Tests**: Verify end-to-end workflows

See `UNIT_TEST_PROGRESS.md` for detailed unit test status and roadmap.

## Debugging Tests

### Verbose Output

```bash
go test -v -run TestAccFolderResource_basic ./workload_credentials/resources/
```

### With Debug Logs

```bash
TF_LOG=DEBUG TF_ACC=1 go test -v -run TestAccFolderResource_basic ./workload_credentials/resources/
```

### Run Single Test Method

```bash
TF_ACC=1 go test -v -timeout=30m -run '^TestAccFolderResource_basic$' ./workload_credentials/resources/
```

## Troubleshooting

### "no tests to run"
- Ensure test files end with `_test.go`
- Ensure test functions start with `Test`
- Check you're in the right directory

### Acceptance tests skipped
- Set `TF_ACC=1` environment variable
- Ensure required environment variables are set (check `testAccPreCheck`)

### Tests timeout
- Increase timeout: `-timeout=120m`
- Check if Workload Credentials instance is accessible
- Verify network connectivity

### Import errors
- Run `go mod tidy`
- Verify Go version: `go version`
- Check `go.mod` has correct dependencies

## Best Practices

1. **Always use test helpers** from `internal/acctest/` for generating random names
2. **Run tests in parallel** when possible using `resource.ParallelTest`
3. **Clean up resources** - Implement proper `CheckDestroy` functions
4. **Use descriptive test names** - Follow pattern `TestAcc<Resource>_<scenario>`
5. **Test error cases** - Don't just test happy paths
6. **Keep tests isolated** - Each test should be independent
7. **Use table-driven tests** for testing multiple scenarios with similar logic

## References

- [Terraform Plugin Testing Framework](https://developer.hashicorp.com/terraform/plugin/framework/acctests)
- [HashiCorp Testing Guide](https://developer.hashicorp.com/terraform/plugin/testing)
- [Go Testing Package](https://pkg.go.dev/testing)
