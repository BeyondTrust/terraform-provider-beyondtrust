# Quick Start Guide

Get started with the BeyondTrust Terraform Provider in 5 minutes.

## Prerequisites

### 1. BeyondTrust Workload Credentials Access

You need access to a BeyondTrust BeyondTrust Workload Credentials instance.

**If you don't have access yet:**
- **Existing BeyondTrust Customer**: Contact your BeyondTrust account manager or support team
- **New to BeyondTrust**: Visit [beyondtrust.com](https://www.beyondtrust.com) for product information
- **Developer Sandbox**: Check with BeyondTrust for sandbox environment availability

### 2. Terraform Installed

- **Terraform 1.10+** (for ephemeral resources - recommended)
- **Terraform 1.0+** (for basic functionality without ephemeral resources)

Check your version:
```bash
terraform version
```

Download from: <https://www.terraform.io/downloads>

## Step 1: Obtain API Credentials

### Using the BeyondTrust Pathfinder Platform

1. Log in to app.beyondtrust.io
2. Navigate to **User Settings** → **Manage Profile** -> **Personal Access Tokens**
3. Click **Create Token**
4. Copy the access token (you won't be able to see it again!)

### Obtaining Your Site ID

Your site ID is a UUID (format: `550e8400-e29b-41d4-a716-...`) that identifies your tenant in the BeyondTrust Workload Credentials platform.

> **Note**: The Site ID is required for multi-tenant isolation. Each organization in BeyondTrust Pathfinder has a unique Site ID (UUID format). This value is sent in the `X-BT-Site-ID` header with every API request.

Contact your BeyondTrust Workload Credentials platform administrator to obtain your site ID. Administrators provision sites and provide the site ID to authorized users.

## Step 2: Configure Provider

Create a new directory for your Terraform configuration:

```bash
mkdir terraform-beyondtrust-test
cd terraform-beyondtrust-test
```

Create `main.tf`:

```hcl
terraform {
  required_version = ">= 1.10"

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

**Alternative: Explicit Configuration** (not recommended for sensitive values):

```hcl
provider "beyondtrust" {
  api_url      = "https://api.workload-credentials.example.com"
  access_token = var.beyondtrust_access_token  # Use variables for sensitive data
  site_id      = var.beyondtrust_site_id
}
```

## Step 3: Create Your First Resource

Add to `main.tf`:

```hcl
# Create a folder
resource "beyondtrust_workload_credentials_folder" "my_first_folder" {
  name = "my-first-folder"

  tags = {
    created_by = "terraform"
    purpose    = "quickstart"
  }
}

# Output the folder details
output "folder_id" {
  value = beyondtrust_workload_credentials_folder.my_first_folder.id
}

output "folder_path" {
  value = beyondtrust_workload_credentials_folder.my_first_folder.path
}
```

## Step 4: Initialize and Apply

```bash
# Initialize Terraform
terraform init

# See what will be created
terraform plan

# Create the resources
terraform apply
```

Type `yes` when prompted.

## Step 5: Verify in Workload Credentials

1. Log in to your Workload Credentials web console
2. Navigate to **Secrets** → **Folders**
3. You should see your new folder: `my-first-folder`

## Step 6: Create a Secret (Optional)

Add to your `main.tf`:

```hcl
# Create a static secret (write-only - never in state!)
resource "beyondtrust_workload_credentials_static_secret" "my_api_key" {
  name   = "my-api-key"
  folder = beyondtrust_workload_credentials_folder.my_first_folder.path

  # These values are write-only and never stored in Terraform state
  secret_wo = {
    api_key = "sk-1234567890abcdef"
    api_url = "https://api.example.com"
  }

  tags = {
    application = "quickstart-demo"
  }
}

# Read the secret using ephemeral resource (requires Terraform 1.10+)
ephemeral "beyondtrust_workload_credentials_static_secret" "read_api_key" {
  name   = beyondtrust_workload_credentials_static_secret.my_api_key.name
  folder = beyondtrust_workload_credentials_static_secret.my_api_key.folder
}

# Output metadata (safe - not the secret value)
output "secret_id" {
  value = beyondtrust_workload_credentials_static_secret.my_api_key.id
}

output "secret_path" {
  value = beyondtrust_workload_credentials_static_secret.my_api_key.path
}

# Note: The actual secret values are in the ephemeral resource
# They're available during plan/apply but never stored in state
```

Apply the changes:

```bash
terraform apply
```

## Step 7: Clean Up

When done testing:

```bash
terraform destroy
```

Type `yes` to confirm deletion.

## Next Steps

### Basic Usage

- [Folder Management](resources/workload_credentials_folder.md) - Organize secrets hierarchically
- [Static Secrets](resources/workload_credentials_static_secret.md) - Store write-only secrets
- [Ephemeral Resources](ephemeral-resources/workload_credentials_static_secret.md) - Read secrets without state storage

### AWS Integration

- [AWS Integration Setup](../examples/aws-integration/README.md) - Complete guide with IAM setup
- [AWS Integration Resource](resources/workload_credentials_aws_integration.md) - Reference documentation
- [AWS Dynamic Secrets](resources/workload_credentials_aws_dynamic_secret.md) - Generate temporary AWS credentials

### Advanced Topics

- **Import Existing Resources**: See import examples in each resource documentation
- **Multi-Account AWS Setup**: Check `examples/aws-integration/complete-setup.tf`
- **GitHub Actions Integration**: See `examples/github-actions/`

## Common Issues

### Can't Find Site ID?

**Best Approach**: Contact your BeyondTrust Workload Credentials platform administrator to obtain your site ID. This is the most reliable method.

**If you already have access:**
- **JWT Token Method**: Decode your access token to extract the `tenant_id` claim (see "Obtaining Your Site ID" section above)
- **Browser DevTools**: The UUID is sent with every API request as `X-BT-Site-ID` header
- **Multiple Sites**: If your organization has multiple sites, ensure you're using the correct site ID for your environment

### Authentication Fails

**Error**: `Invalid access token` or `401 Unauthorized`

**Solutions**:
- Verify your access token hasn't expired
- Check that API URL is correct (should start with `https://`)
- Ensure site ID is a valid UUID

```bash
# Test your credentials
curl -H "Authorization: Bearer ${BEYONDTRUST_ACCESS_TOKEN}" \
     -H "bt-secrets-api-version: 2026-02-16" \
     "${BEYONDTRUST_API_URL}/secrets/session"
```

### Terraform Version Error

**Error**: `Ephemeral resources require Terraform 1.10 or later`

**Solutions**:
- Upgrade to Terraform 1.10+: <https://www.terraform.io/downloads>
- Or remove ephemeral resource blocks (you can still use regular resources)

### Resource Not Found on Import

**Error**: `Resource not found` during import

**Solutions**:
- Verify the resource exists in Workload Credentials
- Check the import path format (use forward slashes: `folder/subfolder/name`)
- Ensure you have permissions to access the resource

### TLS Certificate Errors (Development)

**Error**: `x509: certificate signed by unknown authority`

**Solutions**:
- For development/testing only, you can disable TLS verification:
  ```hcl
  provider "beyondtrust" {
    insecure = true  # DO NOT USE IN PRODUCTION
  }
  ```
- For production, ensure your CA certificates are properly configured

## Getting Help

- **Documentation**: Full docs in the `docs/` directory
- **Examples**: Working examples in the `examples/` directory
- **Issues**: Report bugs at [GitHub Issues](https://github.com/beyondtrust/terraform-provider-beyondtrust/issues)
- **Support**: Contact BeyondTrust support for Workload Credentials-related questions

## Terraform Version Requirements

| Feature                                                | Minimum Terraform Version |
|--------------------------------------------------------|---------------------------|
| Basic Resources (folder, integration, dynamic secrets) | 1.0+                      |
| Ephemeral Resources (read secrets without state)       | 1.10+                     |
| All Features                                           | 1.10+ (recommended)       |

See [TERRAFORM_VERSION_REQUIREMENTS.md](TERRAFORM_VERSION_REQUIREMENTS.md) for detailed version compatibility information.
