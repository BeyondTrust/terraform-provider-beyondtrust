# Terraform Provider Test Configuration

This directory contains a test configuration to validate the BeyondTrust Terraform provider.

## Prerequisites

1. Running SMOP instance (local dev at https://api.smop.local)
2. Valid API access token
3. Site ID (UUID)

## Setup for Local Development

1. Build the provider:
```bash
cd /Users/macole/workspace/terraform-provider-beyondtrust
go build -o terraform-provider-beyondtrust
```

2. Configure Terraform to use the local provider. Create or edit `~/.terraformrc`:
```hcl
provider_installation {
  dev_overrides {
    "beyondtrust/beyondtrust" = "/Users/macole/workspace/terraform-provider-beyondtrust"
  }

  # For all other providers, install them directly using their origin provider
  # registries as normal.
  direct {}
}
```

3. Copy the example variables file and fill in your values:
```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your actual access_token and site_id
```

## Testing

1. Initialize Terraform (with dev_overrides, this won't download anything):
```bash
terraform init
```

2. Validate the configuration:
```bash
terraform validate
```

3. See what Terraform plans to create:
```bash
terraform plan
```

4. Apply the configuration (creates folders in SMOP):
```bash
terraform apply
```

5. Verify the folders were created in SMOP UI or CLI

6. Clean up:
```bash
terraform destroy
```

## What This Tests

- Provider initialization and authentication
- Session validation
- Folder resource creation
- Folder hierarchy (parent/child relationships)
- Tag management
- Computed attributes (id, path, created_at)
- Output values

## Troubleshooting

### Provider not found
Make sure:
- You've built the provider (`go build`)
- The `~/.terraformrc` file points to the correct directory
- The binary name is `terraform-provider-beyondtrust`

### Authentication errors
Check:
- Your access token is valid
- Your site_id is correct (UUID format)
- SMOP API is accessible at the configured URL
- If using local dev, `insecure = true` is set for self-signed certs

### TLS certificate errors
For local development with self-signed certs, make sure `insecure = true` is set in the provider configuration.
