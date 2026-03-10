---
page_title: "beyondtrust_secrets_aws_integration Resource"
subcategory: "Secrets Manager"
description: |-
  Manages an AWS integration in BeyondTrust Secrets Manager.
---

# beyondtrust_secrets_aws_integration (Resource)

Manages an AWS integration for cross-account access and dynamic credential generation.

## Example Usage

```hcl
resource "beyondtrust_secrets_aws_integration" "production" {
  name        = "prod-aws-account"
  role_arn    = "arn:aws:iam::123456789012:role/SMOPIntegrationRole"
  external_id = var.external_id
}
```

## Argument Reference

- `name` (String, Required) The name of the integration. **Changing this forces a new resource.**
- `role_arn` (String, Required) The ARN of the IAM role to assume
- `external_id` (String, Required, Sensitive) External ID for confused deputy prevention

## Attribute Reference

- `id` (String) The unique identifier (UUID) of the integration
- `created_at` (String) The RFC3339 timestamp when created

## Import

```bash
terraform import beyondtrust_secrets_aws_integration.example prod-aws-account
```
