---
page_title: "beyondtrust_secrets_folder Resource"
subcategory: "Secrets Manager"
description: |-
  Manages a folder in BeyondTrust Secrets Manager for organizing secrets.
---

# beyondtrust_secrets_folder (Resource)

Manages a folder in BeyondTrust Secrets Manager. Folders provide hierarchical organization for secrets and dynamic secrets.

## Example Usage

```hcl
resource "beyondtrust_secrets_folder" "production" {
  name = "production"
  tags = {
    environment = "production"
    managed_by  = "terraform"
  }
}

resource "beyondtrust_secrets_folder" "aws" {
  name   = "aws"
  folder = beyondtrust_secrets_folder.production.path
}
```

## Argument Reference

- `name` (String, Required) The name of the folder. **Changing this forces a new resource.**
- `folder` (String, Optional) The parent folder path. **Changing this forces a new resource.**
- `tags` (Map of String, Optional) Key-value tags for the folder

## Attribute Reference

- `id` (String) The unique identifier (UUID) of the folder
- `path` (String) The full path to the folder
- `created_at` (String) The RFC3339 timestamp when created
- `deleted_at` (String) The RFC3339 timestamp when soft-deleted (if applicable)

## Import

```bash
terraform import beyondtrust_secrets_folder.example production/aws
```
