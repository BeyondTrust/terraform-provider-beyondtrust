---
page_title: "Terraform Version Requirements"
description: |-
  Minimum Terraform version requirements for the BeyondTrust provider.
---

# Terraform Version Requirements

## Requirements

This provider **requires Terraform 1.11 or later** for full functionality.

### Why Terraform 1.11+?

The provider uses two features that require recent Terraform versions:

- **Ephemeral resources** (Terraform 1.10+) — read secret values without storing them in state or plan files
- **Write-only attributes** (Terraform 1.11+) — the `secret_wo` attribute uses `WriteOnly: true` to prevent secret values from ever being persisted

## Recommended Configuration

```hcl
terraform {
  required_version = ">= 1.11.0"

  required_providers {
    beyondtrust = {
      source  = "beyondtrust/beyondtrust"
      version = "~> 1.0"
    }
  }
}
```

## Version Compatibility Matrix

| Feature                                                | Minimum Terraform Version |
|--------------------------------------------------------|---------------------------|
| All resources and data sources                         | 1.11+                     |
| Ephemeral resources (read secrets without state)       | 1.11+                     |
| Write-only secret attributes                           | 1.11+                     |

## References

- [Terraform 1.10 Release Notes — Ephemeral Resources](https://www.hashicorp.com/en/blog/terraform-1-10-improves-handling-secrets-in-state-with-ephemeral-values)
- [HashiCorp Write-Only Attributes](https://developer.hashicorp.com/terraform/language/manage-sensitive-data/write-only)
