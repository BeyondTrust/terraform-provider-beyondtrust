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

# Generate credentials from that definition. The dynamic secret must already exist:
# apply it first, then add this block. depends_on does not help, because ephemeral
# resources are opened while the plan is built.
ephemeral "beyondtrust_workload_credentials_azure_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.folder
}

# Ephemeral values can be used in provider configuration, write-only attributes and other
# ephemeral resources, but not in ordinary resource attributes.
provider "azurerm" {
  features {}

  client_id     = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.client_id
  client_secret = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.client_secret
  tenant_id     = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.tenant_id
}

# revoke_on_close defaults to true, deleting the password once Terraform is done with it.
# Set it to false only when another system must keep using the password after the apply.
```

## The dynamic secret must already exist

An ephemeral resource is opened while Terraform builds the plan, not during apply. A single
configuration that both creates a dynamic secret and generates from it therefore fails: the
generate call runs first, before the secret exists.

`depends_on` does not fix this. The dependency is real, but the open happens before it can
take effect.

So apply the dynamic secret first, then add the generating block. Once the secret exists,
every later apply is a single step. The same applies to the permission below — grant it in
its own apply before the block that generates.

This is also why a failure here is ambiguous. The API reports a dynamic secret the caller
cannot see as `403 forbidden` rather than `404 not found`, so a secret that does not exist
yet and a missing permission produce the same error.

## Permissions

Generating requires the `GenerateDynamicCredential` permission on the dynamic secret. Product
admins already hold it and need no policy. Anyone else needs a grant:

```hcl
resource "beyondtrust_iam_policy" "generate" {
  # IAM policies are written against the admin site, so this needs its own provider.
  provider = beyondtrust.platform
  name     = "ci-generate-deployer"

  cedar = <<-EOT
    @siteId("${var.product_site_id}")
    permit(
      principal == Pathfinder::Workload::Id::"${var.ci_workload_id}",
      action == WorkloadCredentials::Action::"GenerateDynamicCredential",
      resource == WorkloadCredentials::DynamicSecret::"/production/azure/ci-deployer"
    );
  EOT
}
```

Two things to get right:

- The principal must be the identity whose token the provider authenticates with, not a
  separate subject. Granting to anyone else yields a policy that reports `ACTIVE` and
  changes nothing.
- `Owner` on the dynamic secret or its folder does **not** include generation. Grant
  `GenerateDynamicCredential` directly.

`revoke_on_close` also needs `RevokeLease` on the same dynamic secret, which `Owner` includes.
Without it the apply still succeeds, but Terraform only warns and the password stays valid until
its TTL.

Check the policy's `status` after applying. `ACTIVE` means the grant is in effect; anything
else, such as `WAITING_FOR_RESOURCE`, means it is not — and that is reported as a warning
rather than an error, so the apply succeeds while generation still fails.

## Credentials are minted per graph walk

Each `terraform apply` generates a password **twice** for every instance of this resource: once while building the plan, once while applying it. A bare `terraform plan` generates one. `count` and `for_each` multiply this.

Each generated password is a real credential added to the target app registration, and app registrations cap how many password credentials they can hold. With the default `revoke_on_close = true` these are cleaned up as Terraform finishes with them and the count stays flat. Setting `revoke_on_close = false` leaves every one of them in place until its TTL elapses, so use it deliberately.

## Revocation

By default the generated password is deleted from the target application via Microsoft Graph as soon as Terraform finishes using it. The delete is idempotent — a password that is already gone is treated as success.

Revocation is best effort. If it fails, Terraform emits a warning rather than failing the operation, and the credential remains valid until the dynamic secret's `ttl` elapses. A revocation that fails consistently usually means the principal lacks the `RevokeLease` permission on the dynamic secret. Terraform also cannot revoke anything if the process is killed outright.

There is no `expiration` attribute on this resource: the generate response does not include one. Read `ttl` from the `beyondtrust_workload_credentials_azure_dynamic_secret` resource instead.

## Using the generated credentials

Generation is the only operation that returns credential values. They are not stored anywhere,
so no later call can return them again.

Within Terraform those values may only flow to:

- provider configuration, as in the example above
- write-only attributes (`*_wo`) on another resource
- other ephemeral resources
- outputs and locals marked `ephemeral` in a **non-root** module

They cannot be assigned to an ordinary resource attribute, and they cannot be published as a
root-module output — not even with `sensitive = true`. Terraform rejects both. Every attribute
of this resource is ephemeral, including `lease_id`, so lease auditing has to go through the
API rather than a Terraform output.

The practical consequence: use this resource to let Terraform act with a short-lived
credential, not to obtain one for yourself. To hold the values directly, generate through the
API or the portal instead.

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
