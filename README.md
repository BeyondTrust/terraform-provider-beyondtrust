<!-- markdownlint-disable MD041 -->
<a href="https://www.beyondtrust.com">
    <img src=".github/beyondtrust_logo.svg" alt="BeyondTrust" title="BeyondTrust" align="right" height="50">
</a>

# BeyondTrust Workload Credentials Terraform Provider
<!-- markdownlint-enable MD041 -->

[![Tests](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/tests.yml/badge.svg)](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/tests.yml)
[![Release](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/promote.yml/badge.svg)](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/promote.yml)

The [BeyondTrust Workload Credentials Provider](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs)
allows [Terraform](https://terraform.io) to manage secrets and credentials for
[BeyondTrust Workload Credentials](https://docs.beyondtrust.com/bt-docs/docs/secrets-on-pathfinder).
This provider enables infrastructure-as-code management of secrets, folders, AWS integrations, and dynamic credential templates.

See the Workload Credentials Provider documentation in the Terraform Registry as well as your BeyondTrust instance documentation for more information on supported endpoints and parameters.

This provider requires BeyondTrust Workload Credentials. Using this provider with other BeyondTrust products is not supported.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.10 (for ephemeral resources) or >= 1.0 (for basic functionality)
- [Go](https://golang.org/doc/install) >= 1.25.8 (for development)
- BeyondTrust Workload Credentials instance with API access

> **Note**: This provider uses **ephemeral resources** for secure secret handling, which require Terraform 1.10 or later.
> Ephemeral resources ensure sensitive values are never persisted in state or plan files.
> See [Terraform Version Requirements](docs/TERRAFORM_VERSION_REQUIREMENTS.md) for details on version compatibility.

## Use Cases

The primary use case for this provider is to manage secrets and credentials for all workloads managed within your Terraform infrastructure, integrated with BeyondTrust Workload Credentials.

This provider enables:
* **Secret Management** - Create and manage static secrets with ephemeral access (never stored in state)
* **Folder Organization** - Organize secrets into a hierarchical folder structure
* **AWS Integration** - Configure AWS integrations for dynamic credential generation
* **Dynamic Secrets** - Provision short-lived AWS credentials on-demand for specific roles
* **Credential Rotation** - Leverage automatic credential rotation and lifecycle management
* **Access Control** - Manage tags and metadata for access policies

Examples for all of these use cases can be found in the [examples](https://github.com/beyondtrust/terraform-provider-beyondtrust/tree/main/examples) directory.

## Configuration

To use the provider, you need:
1. Your BeyondTrust Workload Credentials instance API URL
2. An API access token
3. Your site/tenant ID

### Obtaining Credentials

BeyondTrust Workload Credentials is part of the BeyondTrust Pathfinder platform.

1. Log in to [app.beyondtrust.io](https://app.beyondtrust.io)
2. Navigate to **User Settings** → **Manage Profile** → **Personal Access Tokens**
3. Click **Create Token** and copy the access token

### Obtaining Your Site ID

Your site ID is a UUID that identifies your tenant in the BeyondTrust platform. Contact your BeyondTrust platform administrator to obtain your site ID. 

### Environment Variables

The recommended approach is to set these via environment variables:

```bash
export BEYONDTRUST_API_URL="https://api.workload-credentials.example.com"
export BEYONDTRUST_ACCESS_TOKEN="your-access-token"
export BEYONDTRUST_SITE_ID="your-site-id"
```

Then configure the provider in your Terraform configuration:

```terraform
terraform {
  required_providers {
    beyondtrust = {
      source  = "beyondtrust/beyondtrust"
      version = "~> 1.0"
    }
  }
}

provider "beyondtrust" {
  # Configuration will be read from environment variables:
  # - BEYONDTRUST_API_URL
  # - BEYONDTRUST_ACCESS_TOKEN
  # - BEYONDTRUST_SITE_ID
}
```

While not recommended, you can also set values directly in the configuration:

```terraform
provider "beyondtrust" {
  api_url      = "https://api.workload-credentials.example.com"
  access_token = var.beyondtrust_access_token  # Use variables for sensitive values
  site_id      = var.beyondtrust_site_id
}
```

## Supported Resources

### Managed Resources

- `beyondtrust_workload_credentials_folder` - Manage folder hierarchy for organizing secrets
- `beyondtrust_workload_credentials_static_secret` - Manage static secrets (write-only, use ephemeral to read)
- `beyondtrust_workload_credentials_aws_integration` - Manage AWS integrations for dynamic credentials
- `beyondtrust_workload_credentials_aws_dynamic_secret` - Configure AWS dynamic secret templates

### Ephemeral Resources

- `beyondtrust_workload_credentials_static_secret` - Read secret values (never persisted to state)

### Data Sources

- `beyondtrust_workload_credentials_aws_integration` - Read AWS integration details

## Example Usage

### Creating a Static Secret

```terraform
# Create a folder
resource "beyondtrust_workload_credentials_folder" "production" {
  name        = "production"
  description = "Production environment secrets"
}

# Store a secret (write-only)
resource "beyondtrust_workload_credentials_static_secret" "db_password" {
  name   = "database-password"
  folder = beyondtrust_workload_credentials_folder.production.path
  secret_wo = {
    password = "super-secret-password"
  }
  tags = {
    environment = "production"
    service     = "database"
  }
}

# Read secret value (ephemeral - requires Terraform 1.10+)
ephemeral "beyondtrust_workload_credentials_static_secret" "db_password" {
  name   = "database-password"
  folder = "production"
}

# Use the secret in another resource
resource "kubernetes_secret" "db" {
  metadata {
    name = "database-credentials"
  }
  data = {
    password = ephemeral.beyondtrust_workload_credentials_static_secret.db_password.secret["password"]
  }
}
```

### AWS Dynamic Credentials

```terraform
# Create AWS integration
resource "beyondtrust_workload_credentials_aws_integration" "main" {
  name        = "production-aws"
  role_arn    = "arn:aws:iam::123456789012:role/beyondtrust-role"
  external_id = "unique-external-id"
}

# Configure dynamic secret for a specific role
resource "beyondtrust_workload_credentials_aws_dynamic_secret" "readonly" {
  name           = "readonly-access"
  folder         = beyondtrust_workload_credentials_folder.production.path
  integration_id = beyondtrust_workload_credentials_aws_integration.main.id

  role_arn = "arn:aws:iam::123456789012:role/readonly-role"
  ttl      = 3600  # 1 hour
}
```

For complete examples, see:
- [AWS Integration Setup](./examples/aws-integration/) - Complete example with AWS IAM configuration
- [Basic Examples](./examples/) - Additional resource examples

## Documentation

Full documentation is available on the [Terraform Registry](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs).

For development documentation, see:
- [DEVELOPMENT.md](./docs/development/DEVELOPMENT.md) - Local development setup and workflow
- [TESTING.md](./docs/development/TESTING.md) - Running tests
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines

## Development

### Quick Start

Before committing or pushing code, run local checks:

```bash
make pre-commit        # Full checks (~8s)
make pre-commit-quick  # Fast checks (~4s)
make ci-local         # CI simulation (~12s)
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

```bash
# Run unit tests
make test-unit

# Run acceptance tests (requires Workload Credentials instance)
make test-acc
```

For more details, see [DEVELOPMENT.md](./docs/development/DEVELOPMENT.md).

## Getting Help

For assistance or to report any issues, please:
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
