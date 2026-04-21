# Development Guide

This guide explains how to develop and test the BeyondTrust Terraform provider locally.

## Prerequisites

- Go 1.21 or later
- Terraform 1.0 or later
- Access to a BeyondTrust Secrets Safe instance (for acceptance tests)

## Quick Start

```bash
# Build and install the provider locally
make install

# Run pre-commit checks (recommended before pushing)
make pre-commit        # Full checks (~8s)
make pre-commit-quick  # Fast checks for iteration (~4s)

# Run full CI simulation before opening PR
make ci-local          # (~12s)
```

### First Time Setup

Install required development tools:
```bash
make install-tools
```

Optional: Install git hooks for automatic pre-commit checks:
```bash
make install-git-hooks
```

## Local Development with Terraform

When developing the provider, you'll want to test your changes with actual Terraform configurations without publishing the provider to the registry.

### Setup for Local Testing

The repository includes a `.terraformrc.example` template. When you run `make tf-local-shell` or `make tf-local`, it automatically generates a `.terraformrc` file with your current repository path. This allows you to test your changes immediately without affecting other Terraform projects on your system.

**Note:** The `.terraformrc` file is git-ignored and generated locally, so it works regardless of where you clone the repository.

#### Option 1: New Shell Session (Recommended)

Start a new shell with the local provider configuration:

```bash
make tf-local-shell
```

This creates a new shell session where Terraform will use your local provider build. When you're done testing, simply type `exit` to return to your normal shell.

#### Option 2: Export in Current Shell

Alternatively, set the environment variable in your current shell:

```bash
eval $(make tf-local)
```

This remains active until you close the terminal or unset it:

```bash
unset TF_CLI_CONFIG_FILE
```

### Testing Your Changes

1. **Build the provider** (make install copies to ~/.terraform.d/plugins, but dev_overrides uses the repo root):
   ```bash
   make build
   ```

2. **Enable local provider mode** using one of the options above

3. **Create a test Terraform configuration** in the `examples/` directory or elsewhere:
   ```hcl
   terraform {
     required_providers {
       beyondtrust = {
         source = "beyondtrust/beyondtrust"
       }
     }
   }

   provider "beyondtrust" {
     api_url      = "https://your-instance.beyondtrust.com"
     client_id    = var.client_id
     client_secret = var.client_secret
   }

   data "beyondtrust_secret" "example" {
     path = "my-safe/my-secret"
   }
   ```

4. **Initialize and test**:
   ```bash
   cd examples/your-test-config
   terraform init
   terraform plan
   terraform apply
   ```

**Important:** When using dev overrides, Terraform will display this warning:

```text
Warning: Provider development overrides are in effect
```

This is expected and confirms your local build is being used.

### Iteration Workflow

When making changes:

1. Edit the provider code
2. During rapid iteration: `make pre-commit-quick` (~4s)
3. Rebuild: `make build`
4. Re-run Terraform (no need to run `terraform init` again unless you change provider schema)
   ```bash
   terraform plan
   ```

The dev override automatically picks up your newly built binary.

### Pre-Commit Workflow

Before committing or pushing code:

**During development (fast iteration):**
```bash
make pre-commit-quick  # Runs: fmt, lint, tests, tf-fmt-check (~4s)
```

**Before committing:**
```bash
make pre-commit  # Full checks including build, docs, and validation (~8s)
```

**Before opening a PR:**
```bash
make ci-local  # Full CI simulation with clean workspace (~12s)
```

These commands catch ~95% of CI failures locally, saving you from the push-wait-fix cycle.

#### What Each Target Does

- **`pre-commit-quick`** - Fast checks for rapid iteration:
  - Format code (gofmt + gofumpt)
  - Run linters (golangci-lint + gofumpt check)
  - Run unit tests
  - Check Terraform formatting

- **`pre-commit`** - Comprehensive pre-commit checks:
  - All `pre-commit-quick` checks
  - Build provider binary
  - Generate documentation
  - Validate documentation
  - Verify go.mod/go.sum are clean

- **`ci-local`** - Full CI simulation:
  - Clean workspace
  - All `pre-commit` checks
  - Verify no uncommitted changes after generation

#### Tool Installation

Install all required development tools with correct versions:

```bash
make install-tools
```

This installs:
- golangci-lint v2.11.4
- gofumpt v0.8.0
- tfplugindocs (latest)

Verify tools are installed:
```bash
make check-tools
```

#### Git Hooks (Optional)

Auto-run checks on every commit:

```bash
make install-git-hooks
```

This installs a pre-commit hook that runs `make pre-commit-quick`. To skip the hook temporarily:

```bash
git commit --no-verify
```

### Disabling Local Provider

To return to using the published registry provider:

- **If using tf-local-shell:** Type `exit`
- **If using eval:** Run `unset TF_CLI_CONFIG_FILE`

Your other terminal sessions and Terraform projects are unaffected.

## Testing

### Unit Tests

Run unit tests that don't require external dependencies:

```bash
make test-unit
```

With coverage report:

```bash
make test-coverage
make test-coverage-html  # Opens HTML report
```

### Acceptance Tests

Acceptance tests run against a real BeyondTrust instance. Set the required environment variables:

```bash
export BEYONDTRUST_API_URL="https://your-instance.beyondtrust.com"
export BEYONDTRUST_CLIENT_ID="your-client-id"
export BEYONDTRUST_CLIENT_SECRET="your-client-secret"

make test-acc
```

**Note:** Acceptance tests may create and destroy real resources. Use a test instance.

## Code Generation

Generate provider documentation:

```bash
make generate
```

This updates the `docs/` directory based on the schema and example configurations.

Validate documentation:

```bash
make docs-validate
```

## Linting and Formatting

Format code:

```bash
make fmt
```

Run linters:

```bash
make lint
```

## Project Structure

```text
.
├── internal/
│   ├── client/          # BeyondTrust API client
│   └── provider/        # Terraform provider implementation
├── examples/            # Example Terraform configurations
├── docs/                # Generated provider documentation
├── tools/               # Code generation tools
├── .terraformrc         # Local development overrides
└── Makefile             # Build and test automation
```

## Common Issues

### "Provider not found" error

If Terraform can't find your provider:
1. Ensure you've run `make build`
2. Verify `TF_CLI_CONFIG_FILE` is set: `echo $TF_CLI_CONFIG_FILE`
3. Check that the binary exists: `ls -l terraform-provider-beyondtrust`

### Changes not reflected

If your code changes aren't showing up:
1. Rebuild the provider: `make build`
2. The dev override should pick it up immediately
3. If still not working, remove `.terraform.lock.hcl` and `.terraform/` from your test config and run `terraform init` again

### Wrong provider version in use

If Terraform is using the registry version instead of your local build:
1. Check that `TF_CLI_CONFIG_FILE` is set
2. Look for the "Provider development overrides are in effect" warning
3. Verify the path in `.terraformrc` is correct: `cat .terraformrc`

## Best Practices

1. **Run pre-commit checks** before pushing: `make pre-commit`
2. **Use quick checks** during iteration: `make pre-commit-quick`
3. **Run CI simulation** before opening PR: `make ci-local`
4. **Install git hooks** for automatic checking: `make install-git-hooks`
5. **Update documentation** if you change the schema: `make generate`
6. **Use separate shells** for development vs normal work
7. **Clean up** when done: `exit` or `unset TF_CLI_CONFIG_FILE`

## CI/CD

The project uses GitHub Actions for continuous integration. See `.github/workflows/` for details.

## Resources

- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform Provider Development](https://developer.hashicorp.com/terraform/plugin)
- [BeyondTrust Secrets Safe API Documentation](https://www.beyondtrust.com/docs/secrets-safe/api/index.htm)
