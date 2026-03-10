---
page_title: "Provider: BeyondTrust"
description: |-
  The BeyondTrust provider allows Terraform to manage BeyondTrust resources, including the Secrets Manager Operations Platform (SMOP).
---

# BeyondTrust Provider

The BeyondTrust provider allows you to manage BeyondTrust resources using infrastructure as code. It provides resources and data sources for managing the BeyondTrust Secrets Manager Operations Platform (SMOP), enabling secure secret storage, folder management, AWS integrations, and dynamic credential generation.

## Example Usage

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

resource "beyondtrust_secrets_folder" "production" {
  name = "production"
  tags = {
    environment = "production"
  }
}

resource "beyondtrust_secrets_aws_integration" "main" {
  name        = "prod-aws-account"
  role_arn    = "arn:aws:iam::123456789012:role/SMOPIntegrationRole"
  external_id = var.aws_external_id
}
```

## Schema

### Required

- `api_url` (String) The base URL for the BeyondTrust API
- `access_token` (String, Sensitive) The API access token for authentication
- `site_id` (String) The site/tenant ID in UUID format

### Optional

- `api_version` (String) The API header version (date-based, e.g., '2026-02-16'). Defaults to "2026-02-16"
- `api_path_version` (String) Optional API path version (e.g., 'v1'). Defaults to empty string (no path version)
- `role` (String) Role for X-BT-Role header. When set, X-BT-Auth-Type is automatically set to 'CUSTOM-IDP'
- `insecure` (Boolean) Skip TLS certificate verification. Defaults to false
- `timeout` (String) HTTP client timeout duration. Defaults to "30s"
