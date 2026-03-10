---
page_title: "beyondtrust_secrets_aws_integration Data Source"
subcategory: "Secrets Manager"
description: |-
  Reads an existing AWS integration from BeyondTrust Secrets Manager.
---

# beyondtrust_secrets_aws_integration (Data Source)

Reads an existing AWS integration for referencing in dynamic secrets.

## Example Usage

```hcl
data "beyondtrust_secrets_aws_integration" "existing" {
  name = "production-account"
}

resource "beyondtrust_secrets_aws_dynamic_secret" "app" {
  name             = "application-access"
  integration_name = data.beyondtrust_secrets_aws_integration.existing.name
  credential_type  = "assumed_role"
  role_arn         = var.app_role_arn
  ttl              = 3600
}
```

## Argument Reference

- `name` (String, Required) The name of the integration to look up

## Attribute Reference

- `id` (String) The unique identifier (UUID) of the integration
- `role_arn` (String) The ARN of the IAM role
- `created_at` (String) The RFC3339 timestamp when created
