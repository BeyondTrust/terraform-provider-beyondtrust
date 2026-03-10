# Integration Tests with Terratest

This directory contains integration tests using Terratest for end-to-end validation.

## Why Terratest?

Terratest provides:
- Real Terraform execution and validation
- Retry logic for eventual consistency
- Better error messages
- Easy validation of outputs
- Cross-resource dependency testing

## Structure

```
test/integration/
├── folder_test.go              # Folder resource tests
├── aws_integration_test.go     # AWS integration tests
├── dynamic_secret_test.go      # Dynamic secret tests
└── fixtures/                   # Terraform configurations for testing
    ├── folder/
    │   └── main.tf
    ├── aws_integration/
    │   └── main.tf
    └── complete/                # Full stack test
        └── main.tf
```

## Running Tests

```bash
# Install Terratest
go get github.com/gruntwork-io/terratest/modules/terraform

# Run all integration tests
cd test/integration
go test -v -timeout 30m

# Run specific test
go test -v -run TestFolderBasic -timeout 10m
```

## Environment Variables

```bash
export BEYONDTRUST_API_URL=https://api.smop.local
export BEYONDTRUST_ACCESS_TOKEN=xxx
export BEYONDTRUST_SITE_ID=xxx
export BEYONDTRUST_INSECURE=true  # For local dev
```

## Example Test

```go
package test

import (
    "testing"

    "github.com/gruntwork-io/terratest/modules/terraform"
    "github.com/stretchr/testify/assert"
)

func TestFolderBasic(t *testing.T) {
    terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
        TerraformDir: "./fixtures/folder",
        Vars: map[string]interface{}{
            "folder_name": "terratest-folder",
        },
        EnvVars: map[string]string{
            "BEYONDTRUST_API_URL":      os.Getenv("BEYONDTRUST_API_URL"),
            "BEYONDTRUST_ACCESS_TOKEN": os.Getenv("BEYONDTRUST_ACCESS_TOKEN"),
            "BEYONDTRUST_SITE_ID":      os.Getenv("BEYONDTRUST_SITE_ID"),
        },
    })

    defer terraform.Destroy(t, terraformOptions)

    terraform.InitAndApply(t, terraformOptions)

    // Validate outputs
    folderId := terraform.Output(t, terraformOptions, "folder_id")
    assert.NotEmpty(t, folderId)
}
```

## Test Matrix

| Test | Purpose | Duration |
|------|---------|----------|
| `TestFolderBasic` | Create/delete folder | ~30s |
| `TestFolderWithTags` | Folder with tag management | ~30s |
| `TestAwsIntegration` | AWS integration CRUD | ~1m |
| `TestDynamicSecret` | Dynamic secret CRUD | ~1m |
| `TestCompleteStack` | Full integration + dynamic secret + AWS resources | ~5m |

## Best Practices

1. **Use defer terraform.Destroy()** - Always clean up
2. **Use unique names** - Avoid test collisions (use random strings)
3. **Set timeouts** - Tests can be slow
4. **Test idempotency** - Apply twice, expect no changes
5. **Test imports** - Validate import functionality
