# Testing Guide

This document describes how to run tests for the BeyondTrust Terraform Provider.

## Test Structure

The provider includes three types of tests:

1. **Unit Tests** - Fast tests of business logic helpers and validators (no external dependencies)
2. **Acceptance Tests** - End-to-end tests that run against a real BeyondTrust SMOP instance
3. **Integration Tests** - Tests that verify the provider works with Terraform CLI

## Test Philosophy

> **"Coverage is not a goal - it's a byproduct of testing valuable logic."**

We focus on **testing business logic that matters** rather than chasing coverage percentages:

### ✅ What We Test (High Value)
- **Business logic helpers** - Path construction, import parsing, tag merge-patch semantics
- **Validation logic** - ARN formats, TTL ranges, external ID constraints, JSON policies
- **Type conversions** - Terraform types ↔ Go maps
- **Security-critical logic** - TTL validation (900-43200s), ARN validation, external ID validation
- **Full CRUD lifecycle** - Via acceptance tests against real API

### ❌ What We Don't Test (Low Value)
- **Framework plumbing** - Schema definitions, resource registration (framework's responsibility)
- **Trivial code** - Simple getters/setters, metadata methods
- **Simple assignments** - `data.ID = types.StringValue(resp.ID)`

**Result:** Focused test coverage that catches real bugs and prevents regressions.

---

## Prerequisites

### For Unit Tests
- Go 1.25 or later
- No external dependencies required
- Execution time: **< 1 second**

### For Acceptance Tests
- Go 1.25 or later
- Access to a BeyondTrust SMOP instance (local or remote)
- Valid API credentials
- Execution time: **30+ minutes** (creates/modifies real resources)

---

## Running Tests

### Unit Tests Only

Unit tests are fast and don't require external services:

```bash
# Run all unit tests
make test-unit

# Run unit tests with coverage
make test-coverage

# Run only resource unit tests (excludes acceptance tests)
go test -v -run "Test[^A]" ./secrets/resources/...

# Run specific unit test
go test -v -run TestBuildFolderPath ./secrets/resources/
```

**Expected output:**

```text
✅ 119 test cases pass
✅ Coverage: 13.1% (secrets/resources)
✅ Execution time: ~0.4 seconds
```

### Acceptance Tests

Acceptance tests require a running SMOP instance and proper environment variables.

#### Required Environment Variables

```bash
export BEYONDTRUST_API_URL="https://api.smop.local"
export BEYONDTRUST_ACCESS_TOKEN="your-access-token"
export BEYONDTRUST_SITE_ID="your-site-uuid"
```

#### Optional Environment Variables

```bash
export BEYONDTRUST_API_VERSION="2026-02-16"
export BEYONDTRUST_TEST_AWS_ROLE_ARN="arn:aws:iam::123456789012:role/test-role"
export BEYONDTRUST_TEST_AWS_TARGET_ROLE_ARN="arn:aws:iam::123456789012:role/target-role"
```

#### Run All Acceptance Tests

```bash
# Using make
make test-acc

# Or directly with go test
TF_ACC=1 go test -v -timeout=120m ./secrets/...

# Run only acceptance tests (excludes unit tests)
TF_ACC=1 go test -v -run "TestAcc" ./secrets/resources/...
```

#### Run Specific Acceptance Test

```bash
TF_ACC=1 go test -v -timeout=30m -run TestAccFolderResource_basic ./secrets/resources/
```

### Run All Tests

To run both unit and acceptance tests:

```bash
make test
```

---

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

### Current Coverage

**Overall: 68.1%** (focus on high-value business logic)

| Package | Coverage | Status | Focus |
| --------- | ---------- | -------- | ------- |
| **internal/client** | 84.3% | ✅ Excellent | HTTP client, error handling, CSRF |
| **internal/provider** | 89.5% | ✅ Excellent | Provider configuration, env vars |
| **secrets/resources** | 13.1% | ✅ Targeted | Business logic helpers only |
| **internal/acctest** | 13.2% | ⚠️ Low | Test helpers (integration with AWS SDK) |

**Note:** The `secrets/resources` package has intentionally low coverage because:
- We test **business logic helpers** (path construction, validators, type conversion)
- We **don't test framework code** (schema, CRUD boilerplate, simple assignments)
- Full resource testing is covered by **acceptance tests** (separate from coverage metrics)

---

## Test Organization

### File Naming Convention

We use different suffixes to distinguish unit tests from acceptance tests:

```text
secrets/resources/
├── *_test.go          → Unit tests (package resources)
│   └── Test internal business logic helpers
└── *_acc_test.go      → Acceptance tests (package resources_test)
    └── TestAcc* full CRUD lifecycle against real API
```

**Why this matters:**
- **Clear separation** - Unit vs acceptance tests immediately obvious
- **Selective execution** - Run fast unit tests separately from slow acceptance tests
- **Faster CI** - Unit tests can run in parallel, acceptance tests run sequentially

### Unit Test Files

**Located in:** `secrets/resources/*_test.go` (package: `resources`)

Tests for business logic helpers and validators:

| File | Lines | Tests | Coverage | Focus |
| ------ | ------- | ------- | ---------- | ------- |
| `resource_helpers_test.go` | 412 | 5 functions, 30 cases | Shared helpers | Path, import, tags, queries, errors |
| `folder_resource_test.go` | 20 | Helper only | N/A | stringPtr helper |
| `static_secret_resource_test.go` | 204 | 2 functions, 14 cases | Secret-specific | Type conversion, change detection |
| `aws_integration_resource_test.go` | 175 | 2 functions, 21 cases | AWS validators | ARN, External ID |
| `aws_dynamic_secret_resource_test.go` | 300 | 4 functions, 54 cases | AWS validators | TTL ⭐⭐⭐, credential type, JSON policy |

**Total: 13 test functions, 119 test cases**

#### Test Coverage by Category

**Shared Helpers (30 cases)** - Used by all resources
- `TestBuildFolderPath` (5 cases) - Path construction logic
- `TestParseImportPath` (5 cases) - Import parsing logic
- `TestBuildTagPatch` (8 cases) - RFC 7396 merge-patch semantics
- `TestBuildQueryParameters` (5 cases) - Query parameter construction
- `TestIsNotFoundError` (7 cases) - 404 error detection

**Secret-Specific (14 cases)** - Static secret business logic
- `TestConvertSecretMap` (6 cases) - Terraform types → Go map conversion
- `TestSecretMapsEqual` (8 cases) - Secret change detection

**AWS Integration (21 cases)** - AWS integration validators
- `TestValidateAwsRoleArn` (13 cases) - ARN format validation
- `TestValidateAwsExternalId` (8 cases) - External ID validation

**AWS Dynamic Secret (54 cases)** - AWS dynamic secret validators
- `TestValidateAwsAssumedRoleTTL` (9 cases) - **⭐⭐⭐ CRITICAL** security validation
- `TestValidateAwsCredentialType` (7 cases) - Credential type validation
- `TestValidateJSONPolicy` (8 cases) - JSON policy validation
- `TestConvertAwsTagsMap` (3 cases) - AWS tags conversion

### Acceptance Test Files

**Located in:** `secrets/resources/*_acc_test.go` (package: `resources_test`)

Full CRUD lifecycle tests against real SMOP API:

| File | Lines | Tests | Focus |
| ------ | ------- | ------- | ------- |
| `folder_resource_acc_test.go` | 207 | 5+ test scenarios | Folders CRUD + import |
| `static_secret_resource_acc_test.go` | 233 | 5+ test scenarios | Secrets CRUD + import |
| `aws_integration_resource_acc_test.go` | 172 | 4+ test scenarios | AWS integration CRUD |
| `aws_dynamic_secret_acc_test.go` | 287 | 6+ test scenarios | AWS dynamic secrets CRUD |

**Total: 899 lines covering full resource lifecycle**

### Other Test Files

**Client & Provider Tests:**
- `internal/client/client_test.go` - HTTP client tests (84.3% coverage)
- `internal/client/mock_client_test.go` - Mock client tests
- `internal/provider/provider_test.go` - Provider configuration tests (89.5% coverage)

**Test Helpers:**
- `internal/acctest/helpers.go` - Shared test utilities
- `internal/acctest/helpers_test.go` - Tests for test helpers
- `internal/acctest/config.go` - Test configuration loader
- `internal/acctest/aws_helpers.go` - AWS-specific test helpers

---

## Writing New Tests

### Unit Test Patterns

#### Testing Business Logic Helpers

Focus on extracting and testing **pure business logic functions**:

```go
// ✅ GOOD - Testable business logic helper
func validateAwsRoleArn(arn string) bool {
    if arn == "" {
        return false
    }

    parts := strings.Split(arn, ":")
    if len(parts) < 6 {
        return false
    }

    // Validation logic...
    return true
}

// Unit test
func TestValidateAwsRoleArn(t *testing.T) {
    tests := []struct {
        name    string
        arn     string
        isValid bool
    }{
        {"valid ARN", "arn:aws:iam::123456789012:role/MyRole", true},
        {"invalid format", "not-an-arn", false},
        {"empty string", "", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := validateAwsRoleArn(tt.arn)
            assert.Equal(t, tt.isValid, result)
        })
    }
}
```

#### Table-Driven Tests

Use table-driven tests for testing multiple scenarios with similar logic:

```go
func TestBuildFolderPath(t *testing.T) {
    tests := []struct {
        name         string
        resourceName string
        parentFolder string
        expectedPath string
        description  string
    }{
        {
            name:         "root level resource",
            resourceName: "my-folder",
            parentFolder: "",
            expectedPath: "my-folder",
            description:  "Resource at root should have no parent prefix",
        },
        {
            name:         "nested resource",
            resourceName: "my-folder",
            parentFolder: "production",
            expectedPath: "production/my-folder",
            description:  "Resource in folder should include parent path",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := buildFolderPath(tt.resourceName, tt.parentFolder)
            assert.Equal(t, tt.expectedPath, result, tt.description)
        })
    }
}
```

#### Testing RFC Compliance

Test that your code follows RFC specifications (e.g., RFC 7396 merge-patch):

```go
func TestBuildTagPatch(t *testing.T) {
    tests := []struct {
        name          string
        oldTags       map[string]string
        newTags       map[string]string
        expectedPatch map[string]*string
        description   string
    }{
        {
            name: "delete tag (RFC 7396 null semantics)",
            oldTags: map[string]string{
                "env": "prod",
            },
            newTags: map[string]string{},
            expectedPatch: map[string]*string{
                "env": nil, // nil = delete (per RFC 7396)
            },
            description: "Removed tags should have nil value per RFC 7396",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := buildTagPatch(tt.oldTags, tt.newTags)
            assert.Equal(t, tt.expectedPatch, result, tt.description)
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
        SiteID:      "test-site",
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
    // Save original env vars
    originalURL := os.Getenv("BEYONDTRUST_API_URL")
    defer os.Setenv("BEYONDTRUST_API_URL", originalURL)

    // Set test env vars
    os.Setenv("BEYONDTRUST_API_URL", "https://env.example.com")

    // Test that provider reads from environment
    // ... (see internal/provider/provider_test.go for full example)
}
```

### Acceptance Test Patterns

#### Basic CRUD Test

```go
func TestAccFolderResource_basic(t *testing.T) {
    folderName := acctest.RandomFolderName()

    resource.ParallelTest(t, resource.TestCase{
        PreCheck:                 func() { acctest.PreCheck(t) },
        ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
        CheckDestroy:             testAccCheckFolderDestroy,
        Steps: []resource.TestStep{
            // Create and Read
            {
                Config: testAccFolderResourceConfig_basic(folderName),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName),
                    resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.test", "id"),
                    resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.test", "created_at"),
                ),
            },
            // Import
            {
                ResourceName:      "beyondtrust_secrets_folder.test",
                ImportState:       true,
                ImportStateVerify: true,
            },
        },
    })
}
```

#### Update Test

```go
func TestAccFolderResource_update(t *testing.T) {
    folderName := acctest.RandomFolderName()

    resource.ParallelTest(t, resource.TestCase{
        PreCheck:                 func() { acctest.PreCheck(t) },
        ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
        CheckDestroy:             testAccCheckFolderDestroy,
        Steps: []resource.TestStep{
            // Create with initial tags
            {
                Config: testAccFolderResourceConfig_tags(folderName, map[string]string{
                    "env": "dev",
                }),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.env", "dev"),
                ),
            },
            // Update tags
            {
                Config: testAccFolderResourceConfig_tags(folderName, map[string]string{
                    "env":  "prod",
                    "team": "platform",
                }),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.env", "prod"),
                    resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.team", "platform"),
                ),
            },
        },
    })
}
```

#### CheckDestroy Function

```go
func testAccCheckFolderDestroy(s *terraform.State) error {
    // Get provider client from test state
    // ... (implementation varies by resource)

    for _, rs := range s.RootModule().Resources {
        if rs.Type != "beyondtrust_secrets_folder" {
            continue
        }

        // Verify resource is actually deleted
        name := rs.Primary.Attributes["name"]
        folder := rs.Primary.Attributes["folder"]

        // Try to read the resource
        // If it still exists, return error
        // If 404, continue (expected)
    }

    return nil
}
```

---

## Continuous Integration

Tests run automatically via GitHub Actions on:
- Pull requests to main branch
- Pushes to main branch

See `.github/workflows/` for CI configuration.

### CI Test Matrix

The CI pipeline runs:

1. **Unit Tests** - Always runs (< 1 second)
   - No credentials required
   - Tests business logic helpers
   - Fast feedback on code quality

2. **Acceptance Tests** - Only when credentials available
   - Requires SMOP instance + credentials
   - Tests full CRUD lifecycle
   - Slower (30+ minutes)

3. **Linting** - golangci-lint + gofumpt checks

4. **Documentation** - Validation of Terraform docs

5. **Build** - Cross-platform build verification

---

## Test Value Assessment

### Critical Security Tests (⭐⭐⭐)
- **TTL validation** - Prevents compliance violations (900-43200s range)
- **ARN validation** - Prevents AWS access failures
- **External ID validation** - Prevents security vulnerabilities
- **Path construction** - Prevents data loss and import failures

### High-Value Tests (⭐⭐)
- **Tag merge-patch** - RFC 7396 compliance, prevents accidental tag deletion
- **JSON policy validation** - Catches malformed policies before AWS API errors
- **Secret type conversion** - Prevents runtime errors
- **Import parsing** - Ensures terraform import works correctly

### Medium-Value Tests (⭐)
- **Query parameters** - Simple logic but reused everywhere
- **404 detection** - Basic error handling
- **Change detection** - Optimization logic

**Overall:** 85% critical/high-value, 15% medium-value, 0% low-value tests

---

## Debugging Tests

### Verbose Output

```bash
go test -v -run TestBuildFolderPath ./secrets/resources/
```

### With Debug Logs (Acceptance Tests)

```bash
TF_LOG=DEBUG TF_ACC=1 go test -v -run TestAccFolderResource_basic ./secrets/resources/
```

### Run Single Test Function

```bash
# Unit test
go test -v -run '^TestBuildFolderPath$' ./secrets/resources/

# Acceptance test
TF_ACC=1 go test -v -timeout=30m -run '^TestAccFolderResource_basic$' ./secrets/resources/
```

### Run Tests Matching Pattern

```bash
# All validation tests
go test -v -run 'Validate' ./secrets/resources/

# All AWS tests
go test -v -run 'Aws' ./secrets/resources/
```

---

## Troubleshooting

### "no tests to run"
- Ensure test files end with `_test.go`
- Ensure test functions start with `Test`
- Check you're in the right directory
- For acceptance tests, ensure `TF_ACC=1` is set

### Acceptance tests skipped
- Set `TF_ACC=1` environment variable
- Ensure required environment variables are set:
  - `BEYONDTRUST_API_URL`
  - `BEYONDTRUST_ACCESS_TOKEN`
  - `BEYONDTRUST_SITE_ID`
- Check `acctest.PreCheck(t)` for required variables

### Tests timeout
- Increase timeout: `-timeout=120m`
- Check if SMOP instance is accessible
- Verify network connectivity
- For unit tests: Should never timeout (< 1s execution)

### Import errors
- Run `go mod tidy`
- Verify Go version: `go version` (requires 1.25+)
- Check `go.mod` has correct dependencies

### Coverage seems low
- **This is intentional** - we test business logic, not framework code
- Unit tests: Focus on helpers and validators (13.1% is expected)
- Acceptance tests: Cover full CRUD (not included in coverage metrics)
- See "Test Philosophy" section above

---

## Best Practices

### Unit Test Best Practices

1. **Test business logic helpers** - Extract and test pure functions
2. **Use table-driven tests** - Test multiple scenarios with one test function
3. **Focus on edge cases** - Empty strings, nil values, boundary conditions
4. **Test error handling** - Verify errors are caught and handled correctly
5. **Keep tests fast** - Unit tests should run in < 1 second total
6. **Avoid mocking when possible** - Test pure functions without dependencies

### Acceptance Test Best Practices

1. **Always use test helpers** from `internal/acctest/` for generating random names
2. **Run tests in parallel** when possible using `resource.ParallelTest`
3. **Clean up resources** - Implement proper `CheckDestroy` functions
4. **Use descriptive test names** - Follow pattern `TestAcc<Resource>_<scenario>`
5. **Test error cases** - Don't just test happy paths
6. **Keep tests isolated** - Each test should be independent
7. **Test import functionality** - Verify `terraform import` works

### General Best Practices

1. **Don't chase coverage** - Test valuable logic, not trivial code
2. **Write tests for bugs** - When you find a bug, write a test first
3. **Keep tests readable** - Future you will thank present you
4. **Document test intent** - Use descriptive names and comments

---

## Performance Benchmarks

### Unit Test Performance

- **Total execution:** ~0.4 seconds
- **119 test cases** across 13 test functions
- **100% pass rate**

### Acceptance Test Performance

- **Total execution:** ~30-40 minutes
- **20+ test scenarios** covering full CRUD lifecycle
- **Requires:** Real SMOP instance + AWS account for integration tests

---

## References

- [Terraform Plugin Testing Framework](https://developer.hashicorp.com/terraform/plugin/framework/acctests)
- [HashiCorp Testing Guide](https://developer.hashicorp.com/terraform/plugin/testing)
- [Go Testing Package](https://pkg.go.dev/testing)
- [Table-Driven Tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [RFC 7396 - JSON Merge Patch](https://datatracker.ietf.org/doc/html/rfc7396)
