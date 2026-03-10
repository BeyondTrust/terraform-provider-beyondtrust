# Testing Strategy

This document outlines the testing approach for the BeyondTrust Terraform provider.

## Testing Layers

### 1. Unit Tests (Go test)

**Purpose**: Test individual functions and business logic in isolation.

**Location**: `*_test.go` files alongside implementation

**Run**: `go test ./...`

**Example**:
```go
func TestFolderResourceSchema(t *testing.T) {
    // Test schema validation
}

func TestClientAuthentication(t *testing.T) {
    // Test auth logic with mocked HTTP responses
}
```

**Coverage**:
- Schema validation
- Helper functions
- Client methods with mocked responses
- Edge cases and error handling

### 2. Acceptance Tests (Terraform Native)

**Purpose**: Test resources with real API calls. This is the **primary testing method** for Terraform providers.

**Location**: `secrets/resources/*_test.go`

**Run**:
```bash
TF_ACC=1 \
  BEYONDTRUST_API_URL=https://api.smop.local \
  BEYONDTRUST_ACCESS_TOKEN=xxx \
  BEYONDTRUST_SITE_ID=xxx \
  go test ./secrets/resources -v -timeout 30m
```

**Example**:
```go
func TestAccFolder_basic(t *testing.T) {
    resource.Test(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        Steps: []resource.TestStep{
            {
                Config: testAccFolderConfig_basic("test-folder"),
                Check: resource.ComposeTestCheckFunc(
                    resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", "test-folder"),
                    resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.test", "id"),
                ),
            },
        },
    })
}
```

**Coverage**:
- Create, Read, Update, Delete operations
- Import functionality
- State management
- Attribute validation
- Error scenarios

**Required for**:
- Provider certification
- Terraform Registry publication
- Standard provider development

### 3. Integration Tests (Terratest)

**Purpose**: End-to-end testing with real Terraform execution. Tests complex workflows and cross-resource dependencies.

**Location**: `test/integration/*_test.go`

**Run**:
```bash
cd test/integration
go test -v -timeout 30m
```

**Example**:
```go
func TestCompleteAWSIntegration(t *testing.T) {
    terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
        TerraformDir: "./fixtures/complete",
    })

    defer terraform.Destroy(t, terraformOptions)
    terraform.InitAndApply(t, terraformOptions)

    // Test that credentials can be generated
    dynamicSecretPath := terraform.Output(t, terraformOptions, "dynamic_secret_path")
    assert.NotEmpty(t, dynamicSecretPath)
}
```

**Coverage**:
- Complete workflows (folder → integration → dynamic secret)
- Cross-resource dependencies
- Real AWS integration with actual credentials
- Idempotency testing
- Import/export workflows

**Benefits**:
- More realistic testing
- Better error messages
- Retry logic for eventual consistency
- Easy validation of complex scenarios

## Testing Matrix

| Test Type | Speed | Cost | Realism | Required | When to Use |
|-----------|-------|------|---------|----------|-------------|
| Unit | Fast | Low | Low | Yes | Always, for helper functions |
| Acceptance | Medium | Medium | High | Yes | Primary resource testing |
| Integration | Slow | High | Highest | No | Complex workflows, CI/CD |

## Test Naming Conventions

### Unit Tests
```
Test<Function>               // TestParseARN
Test<Function>_<Scenario>    // TestParseARN_Invalid
```

### Acceptance Tests
```
TestAcc<Resource>_<scenario>           // TestAccFolder_basic
TestAcc<Resource>_<scenario>_<detail>  // TestAccFolder_tags_update
```

### Integration Tests
```
Test<Scenario>                    // TestCompleteAWSIntegration
Test<Resource><Action>            // TestFolderHierarchy
```

## CI/CD Pipeline

### Pull Request Checks
```yaml
- Lint (golangci-lint)
- Unit tests (go test ./...)
- Acceptance tests (selected, fast tests only)
```

### Nightly Build
```yaml
- Full acceptance test suite
- Integration tests (Terratest)
- Performance benchmarks
```

### Release
```yaml
- All tests must pass
- Manual approval required
```

## Test Data Management

### Naming
- Use unique identifiers: `terratest-${random_id}`
- Include test name: `test-folder-basic-${timestamp}`
- Avoid collisions in shared environments

### Cleanup
- **Always** use `defer terraform.Destroy()`
- Tag resources with `managed_by: terratest`
- Run cleanup scripts periodically

### Credentials
- **Never** commit credentials
- Use environment variables
- Rotate test credentials regularly

## Pre-Commit Checklist

- [ ] Unit tests pass locally
- [ ] Acceptance tests pass for modified resources
- [ ] Code is linted (no errors)
- [ ] Documentation updated
- [ ] CHANGELOG entry added

## Running Specific Tests

```bash
# Run unit tests only
go test ./internal/client

# Run acceptance tests for folder resource
TF_ACC=1 go test ./secrets/resources -run TestAccFolder

# Run specific acceptance test
TF_ACC=1 go test ./secrets/resources -run TestAccFolder_basic -v

# Run integration tests with timeout
cd test/integration && go test -v -timeout 30m

# Run specific integration test
cd test/integration && go test -run TestFolderBasic -v
```

## Debugging Tests

### Enable verbose output
```bash
TF_ACC=1 TF_LOG=DEBUG go test ./secrets/resources -run TestAccFolder_basic -v
```

### Run with debugger
```bash
TF_ACC=1 dlv test ./secrets/resources -- -test.run TestAccFolder_basic
```

### Keep resources for inspection
Comment out `defer terraform.Destroy()` temporarily (don't commit!)

## Best Practices

1. **Test Isolation**: Each test should be independent
2. **Fast Feedback**: Unit tests should run in < 1s
3. **Deterministic**: Tests should not be flaky
4. **Clear Assertions**: Use descriptive error messages
5. **Clean Up**: Always destroy test resources
6. **Parallel Safe**: Use `t.Parallel()` when possible
7. **Meaningful Names**: Test names should describe the scenario

## Resources

- [Terraform Plugin Testing](https://developer.hashicorp.com/terraform/plugin/testing)
- [Terratest Documentation](https://terratest.gruntwork.io/)
- [AWS Provider Testing](https://hashicorp.github.io/terraform-provider-aws/running-and-writing-acceptance-tests/)
