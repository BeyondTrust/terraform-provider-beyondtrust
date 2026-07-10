# CLAUDE.md

This file provides development guidance for working with the BeyondTrust Terraform Provider codebase.

## Project Overview

A Terraform provider for BeyondTrust Workload Credentials. Built using the Terraform Plugin Framework (not SDK v2), this provider enables infrastructure-as-code management of secrets, folders, AWS integrations, Azure integrations, and dynamic credential templates.

**Current Implementation:**
- 6 managed resources (folder, static secret, AWS integration, AWS dynamic secret, Azure integration, Azure dynamic secret)
- 2 data sources (AWS integration, Azure integration)
- 1 ephemeral resource (static secret - Terraform 1.11+)
- Full import support for all resources
- Auto-generated documentation
- Unit test coverage: 68.1% overall (client: 86.4%, provider: 89.5%)

**Key Technical Decisions:**
- Terraform Plugin Framework (not SDK v2)
- Ephemeral resources and write-only attributes for secrets (requires Terraform 1.11+)
- Path-based resource identification
- Merge-patch semantics for updates
- Separate metadata endpoints for tags

## Architecture

### Project Structure

```text
terraform-provider-beyondtrust/
├── main.go                          # Provider entry point
├── internal/
│   ├── provider/
│   │   ├── provider.go              # Provider config and registration
│   │   ├── provider_test.go         # Provider unit tests
│   │   └── testing.go               # Test provider configuration
│   ├── client/
│   │   ├── client.go                # HTTP client with Workload Credentials conventions
│   │   ├── client_test.go           # Client unit tests
│   │   ├── mock_client.go           # Mock client for testing
│   │   └── mock_client_test.go      # Mock client tests
│   └── acctest/
│       ├── helpers.go               # Test helpers (random names, pre-checks for AWS/Azure)
│       ├── helpers_test.go          # Helper tests
│       └── aws_helpers.go           # AWS SDK-specific test utilities
├── workload_credentials/
│   ├── resources/                   # Managed resource implementations
│   │   ├── folder_resource.go
│   │   ├── folder_resource_test.go
│   │   ├── static_secret_resource.go
│   │   ├── aws_integration_resource.go
│   │   ├── aws_dynamic_secret_resource.go
│   │   ├── azure_integration_resource.go
│   │   └── azure_dynamic_secret_resource.go
│   ├── datasources/                 # Data source implementations
│   │   ├── aws_integration_data_source.go
│   │   └── azure_integration_data_source.go
│   └── ephemeral/                   # Ephemeral resources (Terraform 1.11+)
│       └── static_secret_ephemeral.go
├── examples/                        # Example Terraform configurations
│   ├── provider/
│   ├── resources/
│   └── data-sources/
├── templates/                       # Documentation templates
│   ├── index.md.tmpl
│   ├── resources/
│   └── data-sources/
├── tools/
│   └── codegen/                     # OpenAPI to resource code generation
└── docs/                            # Auto-generated provider documentation
```

### Layered Architecture

```text
main.go
  ↓
internal/provider/provider.go
  ├── Configure() → creates client.Client
  ├── Resources() → registers managed resources
  ├── DataSources() → registers data sources
  └── EphemeralResources() → registers ephemeral resources
  ↓
internal/client/client.go
  ├── HTTP client with Workload Credentials API conventions
  ├── Header management (auth, API version, site ID)
  └── Merge-patch request building
  ↓
workload_credentials/resources/*.go
  ├── Resource interface implementations
  ├── Schema definitions with plan modifiers
  ├── CRUD operations via client
  └── Import state handling
```

### Provider Configuration Flow

1. User defines provider config in HCL
2. `BeyondTrustProvider.Configure()` called
3. Reads attributes or falls back to environment variables
4. Creates `client.Client` with configuration
5. Client passed to all resources via `ConfigureRequest.ProviderData`
6. Resources type-assert to `*client.Client` in their Configure methods

## Development Workflow

### Pre-Commit Checks (Recommended)

```bash
# Fast checks during iteration
make pre-commit-quick    # ~4s: format, lint, test, tf-fmt

# Full checks before commit
make pre-commit          # ~8s: all checks + build + docs

# CI simulation before PR
make ci-local            # ~12s: full checks + uncommitted check
```

### Daily Development Cycle

```bash
# 1. Make code changes
# 2. Quick validation
make pre-commit-quick

# 3. Build provider
make build

# 4. Test locally with Terraform
make tf-local-shell      # Starts shell with dev overrides
cd examples/resources/beyondtrust_workload_credentials_folder/
terraform init
terraform plan
terraform apply

# 5. Run unit tests
make test-unit

# 6. Generate docs if schema changed
make generate

# 7. Full pre-commit check
make pre-commit
```

### First-Time Setup

```bash
# Install required development tools
make install-tools       # Installs golangci-lint, gofumpt, tfplugindocs

# Optional: Install git hooks for automatic checks
make install-git-hooks   # Runs pre-commit-quick on every commit
```

### Essential Commands

| Command | Purpose | Time |
| ------- | ------- | ---- |
| `make build` | Build provider binary | ~2s |
| `make install` | Install to ~/.terraform.d/plugins/ | ~3s |
| `make tf-local-shell` | Start shell with local provider override | instant |
| `make pre-commit-quick` | Fast checks (fmt, lint, test, tf-fmt) | ~4s |
| `make pre-commit` | Full checks (quick + build + docs + tidy) | ~8s |
| `make test-unit` | Run unit tests | ~2s |
| `make test-acc` | Run acceptance tests (requires Workload Credentials) | varies |
| `make test-coverage` | Generate coverage report | ~3s |
| `make fmt` | Format Go code | ~1s |
| `make lint` | Run golangci-lint + gofumpt | ~2s |
| `make generate` | Generate provider docs from schema | ~1s |
| `make docs-validate` | Validate generated docs | ~1s |
| `make tf-fmt-check` | Check Terraform formatting | <1s |
| `make tf-fmt-fix` | Fix Terraform formatting | <1s |
| `make clean` | Remove build artifacts | <1s |

## Workload Credentials API Conventions

The BeyondTrust Workload Credentials API has specific patterns that the provider implements.

### Path Construction

```go
// Base path: /secrets (or /secrets/v1 if api_path_version is set)
// /api prefix is added by CloudFront, not in provider code

client.BuildPath("/folders")              // → /secrets/folders
client.BuildPath("/folders/my-folder")    // → /secrets/folders/my-folder

// With api_path_version="v1":
client.BuildPath("/folders")              // → /secrets/v1/folders
```

### Required Headers

All requests include:
- `Authorization: Bearer <token>` - Authentication
- `bt-secrets-api-version: 2026-04-28` - API version (date-based)
- `X-BT-Role: <role>` - Optional role (sets `X-BT-Auth-Type: CUSTOM-IDP`)

### Merge-Patch Semantics (RFC 7396)

Updates use `PATCH` with `Content-Type: application/merge-patch+json`:

```go
// Update a field: include in patch
patch := map[string]interface{}{
    "description": "new description",
}

// Delete a field: set to null
patch := map[string]interface{}{
    "description": nil,  // Removes description
}

// Client automatically handles this
client.Patch(ctx, path, nil, patch)
```

### Path-Based Resource Identification

Many resources use paths instead of UUIDs:

```go
// Folders identified by path
query := url.Values{}
query.Set("path", "production/aws/my-folder")
client.Get(ctx, "/folders", query, &result)

// Import format
terraform import beyondtrust_workload_credentials_folder.example production/aws/my-folder
```

### Soft Deletes

Resources support soft deletion by default:

```go
// Soft delete (default)
client.Delete(ctx, path, nil)

// Hard delete
query := url.Values{}
query.Set("permanent", "true")
client.Delete(ctx, path, query)
```

## Adding a New Resource

### 1. Create Resource File

```bash
# Create file: workload_credentials/resources/<name>_resource.go
touch workload_credentials/resources/example_resource.go
```

### 2. Define Model Struct

```go
type ExampleResourceModel struct {
    ID          types.String `tfsdk:"id"`
    Name        types.String `tfsdk:"name"`
    Description types.String `tfsdk:"description"`
    Tags        types.Map    `tfsdk:"tags"`
    CreatedAt   types.String `tfsdk:"created_at"`
}
```

### 3. Implement Resource Interface

```go
type ExampleResource struct {
    client *client.Client
}

func (r *ExampleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_example"
}

func (r *ExampleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
    resp.Schema = schema.Schema{
        Description: "Manages an example resource",
        Attributes: map[string]schema.Attribute{
            "id": schema.StringAttribute{
                Computed: true,
                PlanModifiers: []planmodifier.String{
                    stringplanmodifier.UseStateForUnknown(),
                },
            },
            "name": schema.StringAttribute{
                Required: true,
                PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),  // Immutable
                },
            },
            // ... more attributes
        },
    }
}

func (r *ExampleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
    if req.ProviderData == nil {
        return
    }
    r.client = req.ProviderData.(*client.Client)
}

func (r *ExampleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    // 1. Read plan data
    // 2. Call API to create resource
    // 3. Store ID and computed values in state
}

func (r *ExampleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
    // 1. Get current state
    // 2. Call API to read resource
    // 3. Handle 404 (remove from state)
    // 4. Update state with latest values
}

func (r *ExampleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    // 1. Get plan and state
    // 2. Build merge-patch
    // 3. Call API to update resource
    // 4. Handle tags separately if needed
    // 5. Update state
}

func (r *ExampleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
    // 1. Get current state
    // 2. Call API to delete resource
    // 3. State automatically removed
}

func (r *ExampleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
    // Parse ID and populate state via Read
    resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

### 4. Register Resource

In `internal/provider/provider.go`:

```go
func (p *BeyondTrustProvider) Resources(ctx context.Context) []func() resource.Resource {
    return []func() resource.Resource{
        // ... existing resources
        NewExampleResource,  // Add your resource
    }
}
```

### 5. Write Tests

```go
// workload_credentials/resources/example_resource_test.go
func TestAccExampleResource_basic(t *testing.T) {
    resource.ParallelTest(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        CheckDestroy:             testAccCheckExampleDestroy,
        Steps: []resource.TestStep{
            {
                Config: testAccExampleResourceConfig_basic("test-example"),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("beyondtrust_example.test", "name", "test-example"),
                    resource.TestCheckResourceAttrSet("beyondtrust_example.test", "id"),
                ),
            },
        },
    })
}
```

### 6. Create Examples and Documentation

```bash
# Example Terraform config
mkdir -p examples/resources/beyondtrust_example
cat > examples/resources/beyondtrust_example/resource.tf <<EOF
resource "beyondtrust_example" "example" {
  name        = "my-example"
  description = "Example resource"
}
EOF

# Import example
cat > examples/resources/beyondtrust_example/import.sh <<EOF
terraform import beyondtrust_example.example example-id
EOF

# Documentation template
mkdir -p templates/resources
cat > templates/resources/example.md.tmpl <<EOF
---
page_title: "beyondtrust_example Resource - terraform-provider-beyondtrust"
subcategory: ""
description: |-
  {{ .Description }}
---

# beyondtrust_example (Resource)

{{ .Description }}

{{ .SchemaMarkdown }}
EOF
```

### 7. Generate Documentation

```bash
make generate
```

## Common Development Patterns

### Path-Based Resource Pattern

```go
// Construct path from name + folder
func (m *Model) GetPath() string {
    path := m.Name.ValueString()
    if !m.Folder.IsNull() && m.Folder.ValueString() != "" {
        path = m.Folder.ValueString() + "/" + path
    }
    return path
}

// Use in Read/Delete
query := url.Values{}
query.Set("path", data.GetPath())
err := r.client.Get(ctx, r.client.BuildPath("/resources"), query, &result)
```

### Tags Management Pattern

Tags are managed via separate metadata endpoint:

```go
// Read tags
func (r *Resource) readTags(ctx context.Context, resourcePath string) (map[string]string, error) {
    var tags map[string]string
    err := r.client.Get(ctx,
        r.client.BuildPath(fmt.Sprintf("%s/metadata/tags", resourcePath)),
        nil,
        &tags,
    )
    return tags, err
}

// Update tags (merge-patch)
func (r *Resource) updateTags(ctx context.Context, resourcePath string, oldTags, newTags map[string]string) error {
    patch := map[string]*string{}

    // Add or update tags
    for k, v := range newTags {
        val := v
        patch[k] = &val
    }

    // Delete removed tags
    for k := range oldTags {
        if _, exists := newTags[k]; !exists {
            patch[k] = nil
        }
    }

    return r.client.Patch(ctx,
        r.client.BuildPath(fmt.Sprintf("%s/metadata/tags", resourcePath)),
        nil,
        patch,
    )
}
```

### Error Handling Pattern

```go
if err != nil {
    if strings.Contains(err.Error(), "404") {
        resp.State.RemoveResource(ctx)
        return
    }
    resp.Diagnostics.AddError("Read Error", err.Error())
    return
}
```

### Plan Modifiers

```go
// Immutable attribute (force replacement on change)
"name": schema.StringAttribute{
    Required: true,
    PlanModifiers: []planmodifier.String{
        stringplanmodifier.RequiresReplace(),
    },
}

// Preserve computed value during update
"id": schema.StringAttribute{
    Computed: true,
    PlanModifiers: []planmodifier.String{
        stringplanmodifier.UseStateForUnknown(),
    },
}

// Sensitive value (never show in logs/plan)
"access_token": schema.StringAttribute{
    Optional:  true,
    Sensitive: true,
}
```

### Client API Usage

```go
// Create
requestBody := map[string]interface{}{
    "name": "example",
    "description": "Example resource",
}
var result CreateResponse
err := r.client.Post(ctx, r.client.BuildPath("/resources"), nil, requestBody, &result)

// Read
query := url.Values{}
query.Set("id", resourceID)
var result ReadResponse
err := r.client.Get(ctx, r.client.BuildPath("/resources"), query, &result)

// Update (merge-patch)
patch := map[string]interface{}{
    "description": "Updated description",
}
err := r.client.Patch(ctx, r.client.BuildPath(fmt.Sprintf("/resources/%s", id)), nil, patch)

// Delete
query := url.Values{}
query.Set("permanent", "true")
err := r.client.Delete(ctx, r.client.BuildPath(fmt.Sprintf("/resources/%s", id)), query)
```

## Testing

### Unit Tests

Run unit tests (no external dependencies):
```bash
make test-unit

# Specific package
go test -v ./internal/client/

# With coverage
make test-coverage
make test-coverage-html  # Opens in browser
```

### Acceptance Tests

Require a running Workload Credentials instance:

```bash
# Set environment variables
export BEYONDTRUST_ACCESS_TOKEN="your-token"
export BEYONDTRUST_SITE_ID="your-site-uuid"
export BEYONDTRUST_API_VERSION="2026-04-28"  # Optional

# For AWS integration tests
export BEYONDTRUST_TEST_AWS_ROLE_ARN="arn:aws:iam::123456789012:role/test"

# For Azure integration tests
export BEYONDTRUST_TEST_AZURE_TENANT_ID="your-azure-tenant-uuid"
export BEYONDTRUST_TEST_AZURE_CLIENT_ID="service-principal-client-id-uuid"
export BEYONDTRUST_TEST_AZURE_CLIENT_SECRET="service-principal-client-secret"
export BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID="target-app-object-id-uuid"

# Run all acceptance tests
make test-acc

# Run specific test
TF_ACC=1 go test -v -timeout=30m -run TestAccFolderResource_basic ./workload_credentials/resources/

# Run only Azure acceptance tests
TF_ACC=1 go test -tags=acceptance -v -timeout=30m -run TestAccAzure ./workload_credentials/resources/ ./workload_credentials/datasources/
```

### Test Helpers

```go
// Random resource names
folderName := acctest.RandomFolderName()      // "tf-test-folder-abc123"
secretName := acctest.RandomSecretName()      // "tf-test-secret-xyz789"

// Random AWS resources
roleArn := acctest.RandomAWSRoleARN()         // "arn:aws:iam::123456789012:role/..."
tags := acctest.RandomAWSTags(3)              // map[string]string with 3 random tags
```

### Test Configuration Helpers

```go
func testAccResourceConfig_basic(name string) string {
    return fmt.Sprintf(`
resource "beyondtrust_example" "test" {
  name = %[1]q
}
`, name)
}

func testAccResourceConfig_withDescription(name, desc string) string {
    return fmt.Sprintf(`
resource "beyondtrust_example" "test" {
  name        = %[1]q
  description = %[2]q
}
`, name, desc)
}
```

## Known Issues & TODOs

### High Priority

- **Typed errors**: Replace string-based 404 detection with typed errors containing HTTP status codes
- **Schema validators**: Add runtime validation for documented constraints (name patterns, ARN formats, TTL ranges)
- **Code duplication**: Extract repeated tag management and import logic to shared helpers
- **Plan modifiers**: Add `RequiresReplace` for all immutable attributes, `UseStateForUnknown` for computed attributes

### Test Coverage

- **Unit coverage**: Current 68.1% (target: 80%+)
  - Client: 86.4% ✅
  - Provider: 89.5% ✅
  - Need: More resource/datasource unit tests
- **Acceptance tests**: Need dedicated staging tenant for CI

### Quality Improvements

- Add `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`
- Add GitHub issue/PR templates
- Improve error messages with actionable suggestions
- Add TROUBLESHOOTING.md guide

## Multi-Tenancy

The site ID is embedded in the request path for tenant isolation:

```go
// Provider configuration
provider "beyondtrust" {
  api_url      = "https://api.beyondtrust.io"
  access_token = var.access_token
  site_id      = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"  // Required
}
```

## Ephemeral Resources

Secrets support ephemeral resources (Terraform 1.11+):

```hcl
# Never persisted to state or plan files
ephemeral "beyondtrust_workload_credentials_static_secret" "db_password" {
  path = "production/db-password"
}

resource "kubernetes_secret" "db" {
  data = {
    password = ephemeral.beyondtrust_workload_credentials_static_secret.db_password.value
  }
}
```

See [Terraform Version Requirements](../guides/terraform-version-requirements.md) for version compatibility details.

## Code Generation

The `tools/codegen/` directory contains OpenAPI-to-resource code generation:

```bash
cd tools/codegen
go run . --spec ../../api/openapi.yaml --resource folder

# Generates:
# - workload_credentials/resources/folder_resource.go
# - workload_credentials/resources/folder_resource_test.go (skeleton)
# - examples/resources/beyondtrust_folder/
```

See `CODEGEN.md` for detailed usage.

## References

- `README.md` - Provider usage and examples
- `DEVELOPMENT.md` - Detailed local development setup
- `TESTING.md` - Testing guide with environment variables
- [Terraform Version Requirements](../guides/terraform-version-requirements.md) - Version compatibility matrix

## Development Tips

1. **Use pre-commit checks** - Catch issues before CI: `make pre-commit`
2. **Test locally first** - Use `make tf-local-shell` to test changes before pushing
3. **Watch test coverage** - Run `make test-coverage-html` regularly
4. **Follow existing patterns** - Look at `folder_resource.go` as a reference implementation
5. **Generate docs early** - Run `make generate` to catch schema issues before tests
6. **Use mock client** - Unit test resources with `internal/client/mock_client.go`
7. **Parallel tests** - Use `resource.ParallelTest` for faster acceptance test runs
8. **Check examples** - Ensure example configs actually work with `make tf-local-shell`
