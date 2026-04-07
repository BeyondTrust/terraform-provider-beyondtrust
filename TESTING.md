# Testing Guide

This document describes how to run tests for the BeyondTrust Terraform Provider.

## Test Structure

The provider includes three types of tests:

1. **Unit Tests** - Fast tests that don't require external dependencies
2. **Acceptance Tests** - End-to-end tests that run against a real BeyondTrust SMOP instance
3. **Integration Tests** - Tests that verify the provider works with Terraform CLI

## Prerequisites

### For Unit Tests
- Go 1.25 or later
- No external dependencies required

### For Acceptance Tests
- Go 1.25 or later
- Access to a BeyondTrust SMOP instance (local or remote)
- Valid API credentials

## Running Tests

### Unit Tests Only

Unit tests are fast and don't require external services:

```bash
make test-unit
```

Or directly with go test:

```bash
go test -v -cover ./internal/... ./secrets/...
```

### Acceptance Tests

Acceptance tests require a running SMOP instance and proper environment variables.

#### Required Environment Variables

```bash
export BEYONDTRUST_API_URL="https://api.smop.local"
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
TF_ACC=1 go test -v -timeout=30m -run TestAccFolderResource_basic ./secrets/resources/
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
- `internal/client/client_test.go` - Tests for API client
- `internal/provider/provider_test.go` - Tests for provider configuration

### Acceptance Test Files
Located in the same package as the resources/data sources:
- `secrets/resources/*_test.go` - Resource acceptance tests
- `secrets/datasources/*_test.go` - Data source acceptance tests
- `secrets/ephemeral/*_test.go` - Ephemeral resource acceptance tests

### Test Helpers
- `internal/acctest/` - Shared test utilities and helpers
- `*/testing.go` - Package-specific test configuration

## Writing New Tests

### Unit Test Example

```go
func TestNewClient(t *testing.T) {
    config := &Config{
        BaseURL:     "https://api.example.com",
        AccessToken: "test-token",
        Timeout:     "30s",
    }

    client, err := NewClient(config)
    assert.NoError(t, err)
    assert.NotNil(t, client)
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

See `.github/workflows/test.yml` for CI configuration.

### CI Test Matrix

The CI pipeline runs:
1. **Unit Tests** - Always runs, no credentials required
2. **Acceptance Tests** - Only runs when credentials are available (main repo, not forks)
3. **Linting** - golangci-lint checks
4. **Documentation** - Validation of Terraform docs
5. **Build** - Cross-platform build verification

## Test Coverage Goals

- **Unit Tests**: Aim for 80%+ coverage of core logic
- **Acceptance Tests**: Cover all resource CRUD operations
- **Integration Tests**: Verify end-to-end workflows

## Debugging Tests

### Verbose Output

```bash
go test -v -run TestAccFolderResource_basic ./secrets/resources/
```

### With Debug Logs

```bash
TF_LOG=DEBUG TF_ACC=1 go test -v -run TestAccFolderResource_basic ./secrets/resources/
```

### Run Single Test Method

```bash
TF_ACC=1 go test -v -timeout=30m -run '^TestAccFolderResource_basic$' ./secrets/resources/
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
- Check if SMOP instance is accessible
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
