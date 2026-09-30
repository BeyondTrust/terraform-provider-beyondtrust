---
page_title: "Ephemeral Resource beyondtrust_workload_credentials_azure_dynamic_secret - beyondtrust"
subcategory: ""
description: |-
  Generates a temporary Azure service principal password from a BeyondTrust Workload Credentials dynamic secret.
---

# beyondtrust_workload_credentials_azure_dynamic_secret (Ephemeral Resource)

Generates a temporary Azure service principal password from a BeyondTrust Workload Credentials dynamic secret. Credentials are minted on demand, returned once, and never stored in Terraform state or plan files. By default the credential is revoked via Microsoft Graph as soon as Terraform finishes with it.

~> **Important:** Ephemeral resources require Terraform 1.11+. Credential values are never stored in Terraform state or plan files.

## Example Usage

```terraform
# The dynamic secret defines what to mint and for how long. It is a long-lived
# definition, not a credential.
resource "beyondtrust_workload_credentials_azure_dynamic_secret" "deploy" {
  name                  = "ci-deployer"
  folder                = "production/azure"
  integration_name      = "corp-entra-tenant"
  credential_type       = "service_principal_password"
  application_object_id = "22222222-2222-2222-2222-222222222222"
  ttl                   = 3600
}

# Mint a service principal password from that definition.
#
# depends_on is required rather than decorative. Without it the ephemeral resource is
# opened during the plan walk, before the dynamic secret exists, and the plan fails.
# Referencing .name is not sufficient on its own: that value comes from configuration
# and is therefore already known at plan time, so Terraform has no reason to wait.
ephemeral "beyondtrust_workload_credentials_azure_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.folder

  depends_on = [beyondtrust_workload_credentials_azure_dynamic_secret.deploy]
}

# Ephemeral values may flow into provider configuration, write-only attributes, other
# ephemeral resources, and locals or outputs marked ephemeral. Terraform rejects them
# anywhere else, including ordinary resource attributes.
provider "azurerm" {
  features {}

  client_id     = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.client_id
  client_secret = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.client_secret
  tenant_id     = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.tenant_id
}

# By default the generated password is deleted from the target application as soon as
# Terraform finishes with it. Disable that only when the credential is handed to a
# system that must keep using it after the apply completes — it then stays valid until
# the dynamic secret's TTL elapses.
ephemeral "beyondtrust_workload_credentials_azure_dynamic_secret" "handoff" {
  name            = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.name
  folder          = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.folder
  revoke_on_close = false

  depends_on = [beyondtrust_workload_credentials_azure_dynamic_secret.deploy]
}
```

## Ordering against the dynamic secret

If the dynamic secret is managed in the same configuration, the ephemeral resource needs an explicit `depends_on` referencing it. Terraform otherwise opens the ephemeral resource during the plan walk, before the dynamic secret exists, and the plan fails.

Referencing the dynamic secret's `name` is not sufficient on its own — that value comes from configuration and is already known at plan time, so Terraform has no reason to defer.

## Credentials are minted per graph walk

Each `terraform apply` generates a password **twice** for every instance of this resource: once while building the plan, once while applying it. A bare `terraform plan` generates one. `count` and `for_each` multiply this.

Each generated password is a real credential added to the target app registration, and app registrations cap how many password credentials they can hold. With the default `revoke_on_close = true` these are cleaned up as Terraform finishes with them and the count stays flat. Setting `revoke_on_close = false` leaves every one of them in place until its TTL elapses, so use it deliberately.

## Revocation

By default the generated password is deleted from the target application via Microsoft Graph as soon as Terraform finishes using it. The delete is idempotent — a password that is already gone is treated as success.

Revocation is best effort. If it fails, Terraform emits a warning rather than failing the operation, and the credential remains valid until the dynamic secret's `ttl` elapses. A revocation that fails consistently usually means the principal lacks the `can_revoke_lease` permission on the dynamic secret. Terraform also cannot revoke anything if the process is killed outright.

There is no `expiration` attribute on this resource: the generate response does not include one. Read `ttl` from the `beyondtrust_workload_credentials_azure_dynamic_secret` resource instead.

## Schema

### Required

- `name` (String) The name of the dynamic secret to generate credentials from.

### Optional

- `folder` (String) The parent folder path (e.g., `production` or `production/azure`). Leave empty for root level.
- `revoke_on_close` (Boolean) Whether to delete the generated password from the target application when Terraform finishes using it. Defaults to `true`. Set to `false` when the credential is handed to a system that must keep using it after the apply completes; it then remains valid until the dynamic secret's TTL elapses.

### Read-Only

- `client_id` (String, Sensitive) The client ID of the target application the password was created on.
- `client_secret` (String, Sensitive) The generated service principal password.
- `tenant_id` (String, Sensitive) The Azure tenant ID the application belongs to.
- `key_id` (String) The identifier of the password credential on the target application.
- `lease_id` (String) The identifier of the lease tracking this credential.
