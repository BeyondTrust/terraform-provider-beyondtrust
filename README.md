# Terraform Provider for BeyondTrust

[![Tests](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/test.yml/badge.svg)](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/test.yml)
[![Release](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/release.yml/badge.svg)](https://github.com/beyondtrust/terraform-provider-beyondtrust/actions/workflows/release.yml)

The BeyondTrust Terraform provider allows you to manage BeyondTrust resources using infrastructure as code.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.10 (for ephemeral resources) or >= 1.0 (for basic functionality)
- [Go](https://golang.org/doc/install) >= 1.25 (for development)

> **Note**: This provider uses **ephemeral resources** for secure secret handling, which require Terraform 1.10 or later.
> Ephemeral resources ensure sensitive values are never persisted in state or plan files.
> See [TERRAFORM_VERSION_REQUIREMENTS.md](TERRAFORM_VERSION_REQUIREMENTS.md) for details on version compatibility.

## Using the Provider

```hcl
terraform {
  required_providers {
    beyondtrust = {
      source  = "beyondtrust/beyondtrust"
      version = "~> 1.0"
    }
  }
}

provider "beyondtrust" {
  api_url      = "https://api.smop.example.com"
  access_token = var.beyondtrust_access_token
  site_id      = var.beyondtrust_site_id
}
```

## Documentation

Full documentation is available on the [Terraform Registry](https://registry.terraform.io/providers/beyondtrust/beyondtrust/latest/docs).

## Supported Resources

### SMOP (Secrets Manager Operations Platform)

- `beyondtrust_secrets_folder` - Manage folder hierarchy
- `beyondtrust_secrets_aws_integration` - Manage AWS integrations
- `beyondtrust_secrets_aws_dynamic_secret` - Configure AWS dynamic secrets

### Data Sources

- `beyondtrust_secrets_aws_integration` - Read AWS integration details
- `beyondtrust_secrets_aws_dynamic_secret` - Read dynamic secret configuration
- `beyondtrust_secrets_lease` - Read lease information

## Example Usage

See the [examples](./examples/) directory for complete examples:

- [AWS Integration Setup](./examples/aws-integration/) - Complete example with AWS IAM configuration
- [Dynamic Secrets](./examples/dynamic-secrets/) - AWS dynamic secret examples

## Development

### Building the Provider

```bash
go build -o terraform-provider-beyondtrust
```

### Testing

```bash
# Run unit tests
go test ./...

# Run acceptance tests (requires SMOP instance)
TF_ACC=1 \
  BEYONDTRUST_API_URL=https://api.smop.local \
  BEYONDTRUST_ACCESS_TOKEN=xxx \
  BEYONDTRUST_SITE_ID=xxx \
  go test ./... -v
```

### Local Development

1. Build the provider:
```bash
go build -o terraform-provider-beyondtrust
```

2. Create `~/.terraformrc` with local provider override:
```hcl
provider_installation {
  dev_overrides {
    "beyondtrust/beyondtrust" = "/Users/yourusername/workspace/terraform-provider-beyondtrust"
  }
  direct {}
}
```

3. Run Terraform commands in your test configuration directory.

## Contributing

Contributions are welcome! Please open an issue or pull request.

## License

Copyright (c) BeyondTrust. All rights reserved.
