---
page_title: "beyondtrust_secrets_aws_dynamic_secret Resource"
subcategory: "Secrets Manager"
description: |-
  Manages an AWS dynamic secret configuration for generating temporary credentials.
---

# beyondtrust_secrets_aws_dynamic_secret (Resource)

Manages an AWS dynamic secret that generates temporary AWS credentials on-demand.

## Example Usage

```hcl
resource "beyondtrust_secrets_aws_dynamic_secret" "developer" {
  name             = "developer-access"
  integration_name = beyondtrust_secrets_aws_integration.main.name
  credential_type  = "assumed_role"
  role_arn         = "arn:aws:iam::123456789012:role/DeveloperRole"
  ttl              = 3600

  policy_arns = [
    "arn:aws:iam::aws:policy/ReadOnlyAccess"
  ]

  aws_tags = {
    Environment = "Production"
    Team        = "Engineering"
  }
}
```

## Argument Reference

### Required

- `name` (String) The name of the dynamic secret. **Changing this forces a new resource.**
- `integration_name` (String) The name of the AWS integration to use
- `credential_type` (String) The type of credentials (currently "assumed_role")
- `role_arn` (String) The ARN of the IAM role to assume
- `ttl` (Number) Time-to-live in seconds (900-43200 for assumed_role)

### Optional

- `folder` (String) The parent folder path. **Changing this forces a new resource.**
- `external_id` (String, Sensitive) Optional external ID for role assumption
- `policy_arns` (List of String) AWS managed policy ARNs to attach
- `policy` (String) Inline IAM policy document (JSON)
- `groups` (List of String) IAM group names whose policies to apply
- `aws_tags` (Map of String) AWS session tags

## Attribute Reference

- `id` (String) The unique identifier (UUID)
- `path` (String) The full path to the dynamic secret
- `integration_id` (String) The UUID of the associated integration
- `created_at` (String) The RFC3339 timestamp when created
- `deleted_at` (String) The RFC3339 timestamp when soft-deleted (if applicable)

## Import

```bash
terraform import beyondtrust_secrets_aws_dynamic_secret.example production/developer-access
```
