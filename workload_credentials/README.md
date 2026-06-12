# BeyondTrust Workload Credentials Resources

This directory contains the Terraform resources, data sources, and ephemeral resources for managing [BeyondTrust Workload Credentials](https://docs.beyondtrust.com/bt-docs/docs/secrets-on-pathfinder).

For general provider configuration and usage, see the [main README](../README.md).

## Requirements

- BeyondTrust Workload Credentials instance with API access
- [Terraform](https://www.terraform.io/downloads.html) >= 1.11

> **Note**: Workload Credentials resources use **ephemeral resources** and **write-only attributes** for secure secret handling, which require Terraform 1.11 or later.
> These features ensure sensitive values are never persisted in state or plan files.
> See [Terraform Version Requirements](../docs/guides/terraform-version-requirements.md) for details.

## Use Cases

Workload Credentials enables:
* **Secret Management** - Create and manage static secrets with ephemeral access (never stored in state)
* **Folder Organization** - Organize secrets into a hierarchical folder structure
* **AWS Integration** - Configure AWS integrations for dynamic credential generation
* **Dynamic Secrets** - Provision short-lived AWS credentials on-demand for specific roles
* **Credential Rotation** - Leverage automatic credential rotation and lifecycle management
* **Access Control** - Manage tags and metadata for access policies

## Configuration

### Obtaining Credentials

BeyondTrust Workload Credentials is part of the BeyondTrust Pathfinder platform.

1. Log in to [app.beyondtrust.io](https://app.beyondtrust.io)
2. Navigate to **User Settings** → **Manage Profile** → **Personal Access Tokens**
3. Click **Create Token** and copy the access token

### Obtaining Your Site ID

Your site ID is a UUID that identifies your tenant in the BeyondTrust platform. Contact your BeyondTrust platform administrator to obtain your site ID.

### Environment Variables

Set these via environment variables:

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
  name = "production"
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

# Read secret value (ephemeral - requires Terraform 1.11+)
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
  name            = "readonly-access"
  folder          = beyondtrust_workload_credentials_folder.production.path
  integration_name = beyondtrust_workload_credentials_aws_integration.main.name

  credential_type = "assumed_role"
  role_arn        = "arn:aws:iam::123456789012:role/readonly-role"
  ttl             = 3600  # 1 hour
}
```

## Complete Examples

For complete examples with AWS IAM configuration and more advanced scenarios:
- [AWS Integration Setup](../examples/aws-integration/) - Complete example with AWS IAM configuration
- [Resource Examples](../examples/resources/) - Individual resource examples
- [Test Examples](../examples/test/) - Testing configurations

## Documentation

Full documentation is available on the [Terraform Registry](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs):
- [beyondtrust_workload_credentials_folder](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs/resources/workload_credentials_folder)
- [beyondtrust_workload_credentials_static_secret](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs/resources/workload_credentials_static_secret)
- [beyondtrust_workload_credentials_aws_integration](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs/resources/workload_credentials_aws_integration)
- [beyondtrust_workload_credentials_aws_dynamic_secret](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs/resources/workload_credentials_aws_dynamic_secret)

## Testing

For AWS integration testing, see [TESTING_AWS.md](../docs/TESTING_AWS.md) for detailed setup instructions.
