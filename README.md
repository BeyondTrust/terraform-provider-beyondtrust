<img width="941" height="722" alt="image" src="https://github.com/user-attachments/assets/81af5837-eb2f-4f84-81a8-9d0f8a423ad5" /><!-- markdownlint-disable MD041 -->
<a href="https://www.beyondtrust.com">
    <img src=".github/beyondtrust_logo.svg" alt="BeyondTrust" title="BeyondTrust" align="right" height="50">
</a>

# BeyondTrust Terraform Provider
<!-- markdownlint-enable MD041 -->

[![Tests](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/tests.yml/badge.svg)](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/tests.yml)
[![Release](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/promote.yml/badge.svg)](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/promote.yml)

The [BeyondTrust Terraform Provider](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs) allows [Terraform](https://terraform.io) to manage resources across multiple BeyondTrust products using infrastructure-as-code.

This unified provider consolidates management of BeyondTrust services, enabling seamless integration of security and access management into your Terraform workflows.

## Supported Products

### Workload Credentials

Manage secrets, dynamic credentials, and AWS integrations for [BeyondTrust Workload Credentials](https://docs.beyondtrust.com/bt-docs/docs/secrets-on-pathfinder).

#### Resources

- `beyondtrust_workload_credentials_folder` - Organize secrets in hierarchical folders.
- `beyondtrust_workload_credentials_static_secret` - Manage static secrets (write-only).
- `beyondtrust_workload_credentials_aws_integration` - Configure AWS integrations.
- `beyondtrust_workload_credentials_aws_dynamic_secret` - Provision dynamic AWS credentials.

#### Ephemeral resources

- `beyondtrust_workload_credentials_static_secret` - Read secrets without persisting values to Terraform state.

#### Data sources

- `beyondtrust_workload_credentials_aws_integration` - Query AWS integration details.

For detailed Workload Credentials documentation, examples, and setup instructions, see [workload_credentials/README.md](./workload_credentials/README.md).

### Additional Products

Support for additional BeyondTrust products will be added in future releases.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) 1.0 or later (basic functionality) or 1.10 or later (for ephemeral resources)
- [Go](https://golang.org/doc/install) 1.25.8 or later (development only)
- Access to one or more BeyondTrust products with API access

> **Note**: Some resources use **ephemeral resources** for secure secret handling, which require Terraform 1.10 or later.
> Ephemeral resources ensure sensitive values are never persisted in state or plan files.
> See [Terraform Version Requirements](docs/TERRAFORM_VERSION_REQUIREMENTS.md) for details.

## Quick Start

### Install the provider

```terraform
terraform {
  required_providers {
    beyondtrust = {
      source  = "beyondtrust/beyondtrust"
      version = "~> 1.0"
    }
  }
}
```

### Configure authentication

Configure authentication using environment variables:

```bash
export BEYONDTRUST_API_URL="https://api.beyondtrust.io"
export BEYONDTRUST_ACCESS_TOKEN="your-access-token"
export BEYONDTRUST_SITE_ID="your-site-id"
```

### Create resources

Use the provider in your Terraform configuration:

```terraform
provider "beyondtrust" {
  # Configuration will be read from environment variables:
  # - BEYONDTRUST_API_URL
  # - BEYONDTRUST_ACCESS_TOKEN
  # - BEYONDTRUST_SITE_ID
}
```

### Example Usage

The following example creates a folder, stores a secret, retrieves it through an ephemeral resource, and uses the value in another Terraform resource.

```terraform
# Create a folder for organizing secrets
resource "beyondtrust_workload_credentials_folder" "production" {
  name = "production"
}

# Store a static secret (write-only)
resource "beyondtrust_workload_credentials_static_secret" "api_key" {
  name   = "api-key"
  folder = beyondtrust_workload_credentials_folder.production.path
  secret_wo = {
    key = "secret-api-key-value"
  }
  tags = {
    environment = "production"
    managed_by  = "terraform"
  }
}

# Read secret value securely (ephemeral - never stored in state)
ephemeral "beyondtrust_workload_credentials_static_secret" "api_key" {
  name   = "api-key"
  folder = "production"
}

# Use the secret in another resource
resource "kubernetes_secret" "api_credentials" {
  metadata {
    name = "api-credentials"
  }
  data = {
    api_key = ephemeral.beyondtrust_workload_credentials_static_secret.api_key.secret["key"]
  }
}
```

For more examples, see the [examples](./examples) directory.

## Documentation

### Product Documentation

- **[Workload Credentials](./workload_credentials/README.md)** - Secrets management and dynamic credentials
- More products coming soon...

### Provider Documentation

- [Terraform Registry Docs](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs) - Complete resource documentation
- [Quick Start Guide](./docs/QUICKSTART.md) - Get started quickly
- [Terraform Version Requirements](./docs/TERRAFORM_VERSION_REQUIREMENTS.md) - Version compatibility information

### Development Documentation

- [Development Guide](./docs/development/DEVELOPMENT.md) - Local development setup and workflow
- [Architecture Overview](./docs/development/ARCHITECTURE.md) - Provider architecture and design
- [Testing Guide](./docs/development/TESTING.md) - Running tests
- [Contributing Guidelines](CONTRIBUTING.md) - How to contribute

## Development

### Developer Quick Start

Before committing or pushing code, run local checks:

```bash
make pre-commit        # Full checks
make pre-commit-quick  # Fast checks
make ci-local          # CI simulation
```

Install git hook for automatic checks (optional):
```bash
make install-git-hooks
```

### Building the Provider

```bash
make build
```

### Testing

Acceptance tests require access to supported BeyondTrust products and appropriate API credentials.

```bash
# Run unit tests
make test-unit

# Run acceptance tests (requires access to BeyondTrust products)
make test-acc
```

For more details, see [DEVELOPMENT.md](./docs/development/DEVELOPMENT.md).

## Getting Help

For assistance or to report issues:
- Review the [Documentation](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs)
- Check [existing issues](https://github.com/beyondtrust/terraform-provider-beyondtrust/issues)
- Contact [BeyondTrust Support](https://www.beyondtrust.com/support)

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

For security vulnerabilities, please see our [Security Policy](SECURITY.md).

## Support

See [SUPPORT.md](SUPPORT.md) for support information.

## License

Copyright (c) BeyondTrust. All rights reserved.

See [LICENSE](LICENSE) for license information.
