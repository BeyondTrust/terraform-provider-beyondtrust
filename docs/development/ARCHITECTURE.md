# Architecture Documentation

This document provides a comprehensive overview of the BeyondTrust Terraform Provider's architecture, design decisions, and component interactions.

## Target Audience

This guide is intended for:
- Developers contributing to the provider codebase
- Maintainers reviewing pull requests and architectural changes
- Engineers extending the provider with new resources or functionality

For user-facing documentation, see [README.md](../../README.md) and [QUICKSTART.md](../QUICKSTART.md).

## Overview

The BeyondTrust Terraform Provider enables infrastructure-as-code management of BeyondTrust Workload Credentials resources including secrets, folders, AWS integrations, and dynamic credential templates.

**Key Characteristics:**
- Built on Terraform Plugin Framework (v1.19.0+)
- Multi-tenant architecture via Site ID isolation
- Ephemeral resource support for secrets (Terraform 1.10+)
- HTTP-based REST API integration
- Merge-patch semantics for updates (RFC 7396)

**High-Level System Diagram:**

```text
┌─────────────────────────────────────────────────────────────┐
│                    Terraform CLI                             │
│  (terraform plan/apply/destroy)                              │
└────────────────────────┬─────────────────────────────────────┘
                         │ RPC (gRPC)
                         ↓
┌─────────────────────────────────────────────────────────────┐
│           BeyondTrust Terraform Provider                     │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  Provider Layer (internal/provider/)               │    │
│  │  - Configuration & validation                      │    │
│  │  - Client initialization                           │    │
│  │  - Resource registration                           │    │
│  └───────────────────────┬────────────────────────────┘    │
│                          │                                   │
│  ┌───────────────────────▼────────────────────────────┐    │
│  │  Client Layer (internal/client/)                   │    │
│  │  - HTTP client                                     │    │
│  │  - Header management (auth, API version, site ID) │    │
│  │  - Error handling & typed errors                  │    │
│  └───────────────────────┬────────────────────────────┘    │
│                          │                                   │
│  ┌───────────────────────▼────────────────────────────┐    │
│  │  Resource Layer (workload_credentials/)            │    │
│  │  - Managed resources (CRUD + state)                │    │
│  │  - Data sources (read-only)                        │    │
│  │  - Ephemeral resources (no state persistence)     │    │
│  └────────────────────────────────────────────────────┘    │
└────────────────────────┬─────────────────────────────────────┘
                         │ HTTPS
                         ↓
┌─────────────────────────────────────────────────────────────┐
│         BeyondTrust Workload Credentials API                 │
│  /site/{site-id}/secrets[/version]/{endpoint}                │
└─────────────────────────────────────────────────────────────┘
```

## Package Organization

```text
terraform-provider-beyondtrust/
├── main.go                          # Provider entry point
│
├── internal/                        # Internal packages (not importable externally)
│   ├── provider/
│   │   ├── provider.go              # Provider configuration, validation, client initialization
│   │   ├── provider_test.go         # Provider unit tests (89.5% coverage)
│   │   └── testing.go               # Test provider factory
│   │
│   ├── client/
│   │   ├── client.go                # HTTP client with Workload Credentials API conventions
│   │   ├── client_test.go           # Client unit tests (86.4% coverage)
│   │   ├── mock_client.go           # Mock client for testing
│   │   └── mock_client_test.go      # Mock client tests
│   │
│   ├── constants/
│   │   └── constants.go             # Shared constants (env var names, defaults)
│   │
│   └── acctest/
│       ├── helpers.go               # Test utilities (random names, assertions)
│       ├── helpers_test.go          # Helper tests
│       └── aws_helpers.go           # AWS-specific test utilities
│
├── workload_credentials/            # Workload Credentials service resources
│   ├── resources/                   # Managed resources (CRUD + state)
│   │   ├── folder_resource.go
│   │   ├── static_secret_resource.go
│   │   ├── aws_integration_resource.go
│   │   └── aws_dynamic_secret_resource.go
│   │
│   ├── datasources/                 # Data sources (read-only)
│   │   └── aws_integration_data_source.go
│   │
│   └── ephemeral/                   # Ephemeral resources (Terraform 1.10+)
│       └── static_secret_ephemeral.go
│
├── tools/
│   └── codegen/                     # OpenAPI to resource code generation
│       ├── main.go
│       └── generator/
│
├── examples/                        # Example Terraform configurations
│   ├── provider/
│   ├── resources/
│   ├── data-sources/
│   └── ephemeral-resources/
│
├── templates/                       # Documentation templates for tfplugindocs
│   ├── index.md.tmpl
│   ├── resources/
│   ├── data-sources/
│   └── ephemeral-resources/
│
└── docs/                            # Auto-generated provider documentation
    ├── QUICKSTART.md
    ├── TERRAFORM_VERSION_REQUIREMENTS.md
    ├── development/
    │   ├── ARCHITECTURE.md (this file)
    │   ├── DEVELOPMENT.md
    │   ├── TESTING.md
    │   ├── CODEGEN.md
    │   └── CLAUDE.md
    ├── resources/
    ├── data-sources/
    └── ephemeral-resources/
```

### Separation of Concerns

- **main.go**: Provider binary entry point, minimal logic
- **internal/provider**: Provider-level concerns (config, registration, validation)
- **internal/client**: HTTP client abstraction, API conventions, error handling
- **workload_credentials/**: Service-specific resource implementations
- **internal/acctest**: Shared test utilities (not provider-specific)
- **tools/codegen**: Code generation from OpenAPI specs

## Core Components

### Provider Layer

**Location**: `internal/provider/provider.go`

**Responsibilities:**
- Define provider schema (api_url, access_token, site_id, etc.)
- Validate configuration and environment variables
- Initialize HTTP client with configuration
- Register resources, data sources, and ephemeral resources

**Key Types:**
```go
type BeyondTrustProvider struct {
    version string  // Provider version ("dev", "test", or semver)
}

type BeyondTrustProviderModel struct {
    ApiUrl         types.String  // Base API URL
    AccessToken    types.String  // Bearer token (sensitive)
    SiteId         types.String  // Multi-tenant site ID (UUID)
    ApiVersion     types.String  // Header version (date-based, e.g., "2026-02-16")
    ApiPathVersion types.String  // Optional path version (e.g., "v1")
    Role           types.String  // X-BT-Role header (sets X-BT-Auth-Type: CUSTOM-IDP)
    Insecure       types.Bool    // Skip TLS verification (dev only)
    Timeout        types.String  // HTTP timeout (e.g., "30s")
}
```

**Configuration Priority:**
1. Explicit HCL configuration attributes
2. Environment variables (fallback)
3. Default values

### Client Layer

**Location**: `internal/client/client.go`

**Responsibilities:**
- HTTP request/response handling
- Standard header injection (Authorization, API-Version, Site-ID, Role)
- Path construction with optional version segment
- CSRF token handling (currently disabled)
- Error response parsing into typed errors

**Key Types:**
```go
type Client struct {
    BaseURL        string        // e.g., https://api.workload-credentials.example.com
    AccessToken    string        // Bearer token
    SiteID         string        // Tenant/site UUID
    APIVersion     string        // Header version (date-based)
    APIPathVersion string        // Optional path version (empty or "v1")
    Role           string        // X-BT-Role header value
    HTTPClient     *http.Client  // Configured with TLS settings and timeout
    csrfToken      string        // CSRF token (cached, currently unused)
}

type APIError struct {
    Message    string                 // Human-readable error message
    Code       string                 // Machine-readable error code
    Details    map[string]interface{} // Additional context
    StatusCode int                    // HTTP status code
}
```

**API Conventions:**
- Path format: `/site/{site-id}/secrets[/version]/{endpoint}`
- Required headers on all requests:
  - `Authorization: Bearer <token>`
  - `bt-secrets-api-version: 2026-02-16` (configurable)
  - `X-BT-Site-ID: <uuid>`
- Optional headers:
  - `X-BT-Role: <role>` (when role is set, also sets `X-BT-Auth-Type: CUSTOM-IDP`)

**Error Helpers:**
```go
func (e *APIError) IsNotFound() bool        // 404 Not Found
func (e *APIError) IsConflict() bool        // 409 Conflict
func (e *APIError) IsBadRequest() bool      // 400 Bad Request
func (e *APIError) IsServerError() bool     // 5xx Server Error
func (e *APIError) IsAWSCredentialValidationError() bool  // AWS-specific validation failure
```

### Resource Layer

**Location**: `workload_credentials/resources/`, `workload_credentials/datasources/`, `workload_credentials/ephemeral/`

**Responsibilities:**
- Schema definition (attributes, plan modifiers, validators)
- CRUD operations (Create, Read, Update, Delete)
- State management
- Import support (parsing resource IDs)
- Business logic specific to each resource type

**Resource Interface Implementations:**
- **Managed Resources**: Implement `resource.Resource` + `resource.ResourceWithImportState`
- **Data Sources**: Implement `datasource.DataSource`
- **Ephemeral Resources**: Implement `ephemeral.EphemeralResource` (Terraform 1.10+)

**Example Resource Structure:**
```go
type FolderResource struct {
    client *client.Client  // Injected during Configure()
}

type FolderResourceModel struct {
    Name      types.String  // Required, immutable
    Folder    types.String  // Optional parent folder, immutable
    Path      types.String  // Computed full path
    ID        types.String  // Computed UUID
    CreatedAt types.String  // Computed timestamp
    DeletedAt types.String  // Computed soft-delete timestamp
    Tags      types.Map     // Optional tags (separate API endpoint)
}
```

### Test Layer

**Locations**: `internal/acctest/`, `*/testing.go`, `*/*_test.go`, `*/*_acc_test.go`

**Components:**

1. **Unit Tests** (`*_test.go`):
   - Test business logic without external dependencies
   - Use `httptest.NewServer` for HTTP client testing
   - Use mock client for resource testing
   - Current coverage: 68.1% overall (client: 86.4%, provider: 89.5%)

2. **Acceptance Tests** (`*_acc_test.go`):
   - End-to-end tests against real API
   - Require environment variables (API_URL, ACCESS_TOKEN, SITE_ID)
   - Use `resource.ParallelTest` for concurrency
   - Clean up resources via `CheckDestroy` functions

3. **Test Helpers** (`internal/acctest/`):
   - `RandomFolderName()`, `RandomSecretName()` - Generate unique test names
   - `RandomAWSRoleARN()`, `RandomAWSTags()` - Generate AWS test data
   - Pre-configured test provider factories

## Provider Lifecycle

### Initialization Flow

```text
1. Terraform CLI invokes provider binary
   ↓
2. main.go calls provider.New(version)()
   ↓
3. Terraform calls provider.Configure()
   ├─ Read HCL configuration
   ├─ Fall back to environment variables
   ├─ Apply defaults
   ├─ Validate required fields
   └─ Create client.Client
   ↓
4. Provider registers resources via Resources(), DataSources(), EphemeralResources()
   ↓
5. Resources receive client via Configure(req.ProviderData)
```

### Request Flow

```text
Terraform plan/apply
   ↓
Resource CRUD method (Create/Read/Update/Delete)
   ↓
client.Client method (Post/Get/Patch/Delete)
   ├─ Build path: client.BuildPath("/folders")
   ├─ Create request: client.newRequest(method, path, query, body)
   ├─ Inject headers: Authorization, API-Version, Site-ID, Role
   ├─ Execute request: client.do(req, requireCSRF)
   ├─ Handle errors: parse APIError from response
   └─ Unmarshal response into result struct
   ↓
Resource updates state
   ↓
Terraform updates state file
```

## Resource Types & State Management

### Managed Resources

**Persistent state** stored in Terraform state file.

**Lifecycle**: Create → Read → Update → Delete

**Current Resources:**
- `beyondtrust_workload_credentials_folder` - Organize secrets in hierarchical folders
- `beyondtrust_workload_credentials_static_secret` - Store static secrets (passwords, API keys)
- `beyondtrust_workload_credentials_aws_integration` - Configure AWS IAM role assumption
- `beyondtrust_workload_credentials_aws_dynamic_secret` - Template for dynamic AWS credentials

**State Management:**
- Terraform tracks resource IDs and attributes
- Read operations refresh state from API
- Update operations compute diffs and apply changes
- Delete operations remove from API and state
- Import operations populate state from existing API resources

### Data Sources

**Ephemeral state** only exists during plan/apply, not persisted.

**Lifecycle**: Read only (no Create/Update/Delete)

**Current Data Sources:**
- `beyondtrust_workload_credentials_aws_integration` - Look up AWS integration by name

**Use Cases:**
- Reference existing resources created outside Terraform
- Query computed values for use in other resources
- Fetch dynamic data during plan/apply

### Ephemeral Resources

**No state persistence** - secrets never written to state files or plan files.

**Lifecycle**: Open → Read → Close (Terraform 1.10+)

**Current Ephemeral Resources:**
- `beyondtrust_workload_credentials_static_secret` - Retrieve secret value at apply time

**Security Benefits:**
- Secret values never persisted to disk
- No risk of secrets in state files or Git history
- Values only exist in memory during apply

**Example:**
```hcl
ephemeral "beyondtrust_workload_credentials_static_secret" "db_password" {
  path = "production/db-password"
}

resource "kubernetes_secret" "db" {
  data = {
    password = ephemeral.beyondtrust_workload_credentials_static_secret.db_password.value
  }
}
```

## API Integration Patterns

### Path Construction

```go
// Without path version
client.BuildPath("/folders")
// → /site/550e8400-e29b-41d4-a716-446655440000/secrets/folders

// With api_path_version="v1"
client.BuildPath("/folders")
// → /site/550e8400-e29b-41d4-a716-446655440000/secrets/v1/folders
```

**Note**: The `/api` prefix is added by CloudFront, not by provider code.

### Merge-Patch Semantics (RFC 7396)

Updates use `PATCH` with `Content-Type: application/merge-patch+json`:

```go
// Update a field: include in patch
patch := map[string]interface{}{
    "description": "new description",
}

// Delete a field: set to null
patch := map[string]interface{}{
    "description": nil,  // Removes description field
}

client.Patch(ctx, path, nil, patch)
```

**Behavior:**
- Only fields in the patch are modified
- Fields set to `null` are deleted
- Omitted fields are unchanged
- Arrays and objects are replaced, not merged

### Path-Based Resource Identification

Some resources use hierarchical paths instead of UUIDs:

```go
// Query by path
query := url.Values{}
query.Set("path", "production/aws/my-folder")
client.Get(ctx, "/folders", query, &result)

// Import by path
// terraform import beyondtrust_workload_credentials_folder.example production/aws/my-folder
```

### Separate Metadata Endpoints

Tags are managed via dedicated metadata endpoints:

```go
// Read tags
GET /folders/{folder-path}/metadata/tags

// Update tags (merge-patch)
PATCH /folders/{folder-path}/metadata/tags
{
  "environment": "production",  // Add/update tag
  "deprecated": null            // Delete tag
}
```

### Soft Deletes

Resources support soft deletion by default:

```go
// Soft delete (default) - resource marked as deleted but recoverable
client.Delete(ctx, path, nil)

// Hard delete (permanent) - resource immediately destroyed
query := url.Values{}
query.Set("permanent", "true")
client.Delete(ctx, path, query)
```

## Error Handling

### Typed Error System

```go
type APIError struct {
    Message    string                 // Human-readable error message
    Code       string                 // Machine-readable error code (e.g., "NOT_FOUND")
    Details    map[string]interface{} // Additional error context
    StatusCode int                    // HTTP status code
}
```

**Helper Methods:**
```go
if err != nil {
    if apiErr, ok := err.(*client.APIError); ok {
        if apiErr.IsNotFound() {
            // Handle 404 - remove from state
            resp.State.RemoveResource(ctx)
            return
        }
        if apiErr.IsAWSCredentialValidationError() {
            // Handle AWS-specific validation failure
            resp.Diagnostics.AddError("AWS Credential Error", apiErr.Error())
            return
        }
    }
    resp.Diagnostics.AddError("API Error", err.Error())
}
```

### AWS Credential Validation

Special error detection for AWS integration validation failures:

```go
func (e *APIError) IsAWSCredentialValidationError() bool {
    return e.Code == "aws_integration_test_failed" ||
           e.Code == "aws_credential_validation_failed" ||
           strings.Contains(strings.ToLower(e.Message), "failed to validate aws integration credentials")
}
```

### Schema Validation

Schema-level validation via plan modifiers:

```go
"name": schema.StringAttribute{
    Required: true,
    PlanModifiers: []planmodifier.String{
        stringplanmodifier.RequiresReplace(),  // Immutable - force recreation
    },
    Validators: []validator.String{
        // TODO: Add runtime validators for pattern constraints
    },
}
```

## Testing Architecture

### Test Pyramid

```text
                     ▲
                    ╱│╲
                   ╱ │ ╲
                  ╱  │  ╲
                 ╱   │   ╲        Acceptance Tests
                ╱────┼────╲       (End-to-end, real API)
               ╱     │     ╲      ~10 tests
              ╱──────┼──────╲
             ╱       │       ╲    Integration Tests
            ╱────────┼────────╲   (Terraform CLI)
           ╱         │         ╲  ~5 tests
          ╱──────────┼──────────╲
         ╱███████████│███████████╲  Unit Tests
        ╱█████████████████████████╲ (Mock API, fast)
       ╱───────────────────────────╲ ~50 tests
      ╱                             ╲
     ╱                               ╲
```

### Unit Tests

**Purpose**: Test business logic without external dependencies

**Characteristics:**
- Fast (<1 second per test)
- No network calls
- Use `httptest.NewServer` for HTTP mocking
- Use `internal/client/mock_client.go` for resource testing

**Coverage**: 68.1% overall
- `internal/client`: 86.4% ✅
- `internal/provider`: 89.5% ✅
- `internal/acctest`: 22.4% (low due to AWS SDK integration)

**Example:**
```go
func TestClient_Get(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"id": "123"})
    }))
    defer server.Close()

    client, _ := client.NewClient(&client.Config{
        BaseURL:     server.URL,
        AccessToken: "test-token",
        SiteID:      "test-site",
        Timeout:     "30s",
    })

    var result map[string]string
    err := client.Get(context.Background(), "/test", nil, &result)
    assert.NoError(t, err)
    assert.Equal(t, "123", result["id"])
}
```

### Acceptance Tests

**Purpose**: End-to-end validation against real API

**Characteristics:**
- Slower (seconds to minutes per test)
- Require real Workload Credentials instance
- Require environment variables (API_URL, ACCESS_TOKEN, SITE_ID)
- Use `resource.ParallelTest` for concurrency
- Clean up resources via `CheckDestroy` functions

**Example:**
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
                    resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "name", folderName),
                    resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_folder.test", "id"),
                ),
            },
        },
    })
}
```

### Test Helpers

**Location**: `internal/acctest/helpers.go`

**Utilities:**
- `RandomFolderName()` - Generate unique folder name with prefix `tf-test-folder-`
- `RandomSecretName()` - Generate unique secret name with prefix `tf-test-secret-`
- `RandomAWSRoleARN()` - Generate test AWS role ARN
- `RandomAWSTags(count)` - Generate random AWS tags

### CI/CD Testing

**GitHub Actions** (`.github/workflows/tests.yml`):
1. **Linting** - golangci-lint + gofumpt
2. **Unit Tests** - Always run (no credentials required)
3. **Acceptance Tests** - Conditional (requires secrets)
4. **Documentation** - Validate generated docs
5. **Build** - Cross-platform compilation

## Key Design Decisions

### 1. Terraform Plugin Framework (Not SDK v2)

**Decision**: Use Plugin Framework v1.19.0+

**Rationale:**
- Modern type system with `types.String`, `types.Bool`, etc.
- Better null/unknown value handling
- Native ephemeral resource support
- Improved plan modifier system
- Better error diagnostics

**Trade-offs:**
- Less mature ecosystem compared to SDK v2
- Requires Terraform 1.0+

### 2. Ephemeral Resources for Secrets

**Decision**: Implement ephemeral resources for secret retrieval

**Rationale:**
- Prevents secrets from persisting to disk
- No risk of secrets in state files or Git history
- Values only exist in memory during apply

**Trade-offs:**
- Requires Terraform 1.10+ for ephemeral resource support
- More complex lifecycle (Open → Read → Close)

**See**: [TERRAFORM_VERSION_REQUIREMENTS.md](../TERRAFORM_VERSION_REQUIREMENTS.md)

### 3. Path-Based Resource Identification

**Decision**: Use hierarchical paths (not UUIDs) for folders and secrets

**Rationale:**
- Intuitive for users (matches directory structure)
- Human-readable import identifiers
- Natural folder hierarchy representation

**Trade-offs:**
- Path changes require resource replacement
- More complex validation (path uniqueness)

**Example:**
```bash
terraform import beyondtrust_workload_credentials_folder.prod production/aws
```

### 4. Separate Metadata Endpoints for Tags

**Decision**: Tags managed via `/metadata/tags` endpoints

**Rationale:**
- API design separates tags from resource attributes
- Allows independent tag updates without full resource PATCH

**Trade-offs:**
- Requires two API calls for resources with tags (one for resource, one for tags)
- Slightly more complex error handling

### 5. Multi-Tenancy via Site ID

**Decision**: Require Site ID in all requests via `X-BT-Site-ID` header

**Rationale:**
- API enforces tenant isolation at the infrastructure level
- Prevents accidental cross-tenant access
- Supports SaaS deployment model

**Trade-offs:**
- Users must know their Site ID (UUID)
- Cannot manage multiple sites from single provider block

### 6. Environment Variable First Configuration

**Decision**: Environment variables as fallback for all configuration

**Rationale:**
- CI/CD friendly (store secrets in environment)
- Works with secret management tools (AWS Secrets Manager, Vault)
- Explicit HCL config overrides when needed

**Trade-offs:**
- Two places to check configuration (HCL + env vars)
- Environment variable naming must be consistent

## Security Considerations

### Sensitive Fields

All secrets marked as sensitive in schema:

```go
"access_token": schema.StringAttribute{
    Optional:  true,
    Sensitive: true,  // Never logged or shown in plan output
}
```

### Secret Handling via Ephemeral Resources

Secrets retrieved via ephemeral resources:
- Never persisted to state files
- Never persisted to plan files
- Only exist in memory during apply

### TLS Configuration

```go
"insecure": schema.BoolAttribute{
    Description: "Skip TLS certificate verification. Only use for development.",
    Optional:    true,
}
```

**Warning**: `insecure: true` should only be used for local development with self-signed certificates.

### CSRF Token Handling

**Current Status**: Disabled pending backend fix

**Reason**: Backend `/session` endpoint requires admin permissions

**Implementation** (`internal/client/client.go:263-273`):
```go
// TODO: Re-enable CSRF token support once session endpoint permissions are fixed
// if requireCSRF {
//     if err := c.ensureCSRFToken(req.Context()); err != nil {
//         return nil, fmt.Errorf("failed to get CSRF token: %w", err)
//     }
//     if c.csrfToken != "" {
//         req.Header.Set("X-CSRF-Token", c.csrfToken)
//     }
// }
```

### Credential Validation

**Current Status**: Disabled pending backend fix

**Reason**: Backend `/session` endpoint requires admin permissions

**Implementation** (`internal/provider/provider.go:215-224`):
```go
// TODO: Re-enable once /session endpoint permissions are fixed
// if err := apiClient.ValidateSession(ctx); err != nil {
//     resp.Diagnostics.AddError(
//         "Unable to Authenticate with BeyondTrust API",
//         "The provider could not authenticate with the BeyondTrust API. "+
//             "Please check your access token and API URL. "+
//             "Error: "+err.Error(),
//     )
//     return
// }
```

## Extension Points

### Adding a New Resource

1. **Create resource file**: `workload_credentials/resources/example_resource.go`
2. **Define model struct**:
   ```go
   type ExampleResourceModel struct {
       ID          types.String `tfsdk:"id"`
       Name        types.String `tfsdk:"name"`
       Description types.String `tfsdk:"description"`
   }
   ```
3. **Implement `resource.Resource` interface**: Metadata, Schema, Configure, Create, Read, Update, Delete
4. **Implement `resource.ResourceWithImportState`**: ImportState method
5. **Register in provider**: Add to `Resources()` in `internal/provider/provider.go`
6. **Write tests**: Unit tests + acceptance tests
7. **Create examples**: `examples/resources/beyondtrust_example/resource.tf`
8. **Generate docs**: `make generate`

**See**: [CLAUDE.md](./CLAUDE.md#adding-a-new-resource) for detailed step-by-step guide.

### Adding a New Data Source

Similar to resources, but implement `datasource.DataSource` interface:
- Metadata, Schema, Configure, Read (no Create/Update/Delete)

### Adding an Ephemeral Resource

Implement `ephemeral.EphemeralResource` interface (Terraform 1.10+):
- Metadata, Schema, Configure, Open, Read, Close

## Dependencies

### Core Dependencies

```go
// Terraform Plugin Framework - core provider framework
require github.com/hashicorp/terraform-plugin-framework v1.19.0

// Terraform Plugin Go - protocol implementation
require github.com/hashicorp/terraform-plugin-go v0.25.0

// Terraform Plugin Testing - acceptance test framework
require github.com/hashicorp/terraform-plugin-testing v2.0.0
```

### Development Dependencies

```go
// AWS SDK - test utilities only (not core provider)
require github.com/aws/aws-sdk-go v1.55.8

// Testing utilities
require github.com/stretchr/testify v1.10.0
```

### Tool Dependencies

```bash
# Code generation and linting tools (installed via make install-tools)
golangci-lint  # Linting and static analysis
gofumpt        # Stricter Go formatting
tfplugindocs   # Generate Terraform documentation from schema
```

**Note**: AWS SDK is only used in test helpers (`internal/acctest/aws_helpers.go`), not in the core provider logic.

## References

- [DEVELOPMENT.md](./DEVELOPMENT.md) - Local development setup
- [TESTING.md](./TESTING.md) - Testing guide with environment variables
- [CODEGEN.md](./CODEGEN.md) - OpenAPI to resource code generation
- [CLAUDE.md](./CLAUDE.md) - Development patterns and best practices
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [RFC 7396 - JSON Merge Patch](https://datatracker.ietf.org/doc/html/rfc7396)
