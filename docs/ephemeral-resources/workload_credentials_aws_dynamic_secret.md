---
page_title: "Ephemeral Resource beyondtrust_workload_credentials_aws_dynamic_secret - beyondtrust"
subcategory: ""
description: |-
  Generates temporary AWS credentials from a BeyondTrust Workload Credentials dynamic secret.
---

# beyondtrust_workload_credentials_aws_dynamic_secret (Ephemeral Resource)

Generates temporary AWS credentials from a BeyondTrust Workload Credentials dynamic secret. Credentials are minted on demand via `sts:AssumeRole`, returned once, and never stored in Terraform state or plan files. They expire on their own at the dynamic secret's TTL and cannot be revoked early.

~> **Important:** Ephemeral resources require Terraform 1.11+. Credential values are never stored in Terraform state or plan files.

## Example Usage

```terraform
# The dynamic secret defines what to mint and for how long. It is a long-lived
# definition, not a credential.
resource "beyondtrust_workload_credentials_aws_dynamic_secret" "deploy" {
  name             = "ci-deployer"
  folder           = "production/aws"
  integration_name = "production-aws-account"
  credential_type  = "assumed_role"
  role_arn         = "arn:aws:iam::123456789012:role/Deployer"
  ttl              = 3600
}

# Generate credentials from that definition. The dynamic secret must already exist:
# apply it first, then add this block. depends_on does not help, because ephemeral
# resources are opened while the plan is built.
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.folder
}

# Ephemeral values can be used in provider configuration, write-only attributes and other
# ephemeral resources, but not in ordinary resource attributes.
provider "aws" {
  region     = "us-east-1"
  access_key = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.access_key_id
  secret_key = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.secret_access_key
  token      = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.session_token
}

# Ephemeral values cannot be root-module outputs, even as sensitive.
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
      resource == WorkloadCredentials::DynamicSecret::"/production/aws/ci-deployer"
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

Check the policy's `status` after applying. `ACTIVE` means the grant is in effect; anything
else, such as `WAITING_FOR_RESOURCE`, means it is not — and that is reported as a warning
rather than an error, so the apply succeeds while generation still fails.

## Credentials are minted per graph walk

Each `terraform apply` generates credentials **twice** for every instance of this resource: once while building the plan, once while applying it. A bare `terraform plan` generates them once. `count` and `for_each` multiply this.

That is inherent to generating on demand, and it is harmless for AWS — each session expires on its own at the dynamic secret's TTL and costs nothing to leave behind. It does mean lease counts in audit logs grow faster than apply counts.

## Expiration and revocation

AWS assumed-role credentials cannot be revoked early. The session expires on its own at the time given by `expiration`. Choose the dynamic secret's `ttl` accordingly: it must comfortably outlast the longest apply that uses the credential, because there is no way to extend a session once issued.

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

- `folder` (String) The parent folder path (e.g., `production` or `production/aws`). Leave empty for root level.

### Read-Only

- `access_key_id` (String, Sensitive) The AWS access key ID for the generated session.
- `secret_access_key` (String, Sensitive) The AWS secret access key for the generated session.
- `session_token` (String, Sensitive) The AWS session token for the generated session.
- `expiration` (String) The RFC3339 timestamp at which the generated credentials expire.
- `lease_id` (String) The identifier of the lease tracking these credentials. Recorded for auditing; AWS leases cannot be revoked before they expire.
