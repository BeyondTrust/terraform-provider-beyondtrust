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

# Mint a session from that definition.
#
# depends_on is required rather than decorative. Without it the ephemeral resource is
# opened during the plan walk, before the dynamic secret exists, and the plan fails.
# Referencing .name is not sufficient on its own: that value comes from configuration
# and is therefore already known at plan time, so Terraform has no reason to wait.
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.folder

  depends_on = [beyondtrust_workload_credentials_aws_dynamic_secret.deploy]
}

# Ephemeral values may flow into provider configuration, write-only attributes, other
# ephemeral resources, and locals or outputs marked ephemeral. Terraform rejects them
# anywhere else, including ordinary resource attributes.
provider "aws" {
  region     = "us-east-1"
  access_key = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.access_key_id
  secret_key = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.secret_access_key
  token      = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.session_token
}

# Metadata is safe to output. The credentials themselves are not.
output "session_expires_at" {
  value = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.expiration
}
```

## Ordering against the dynamic secret

If the dynamic secret is managed in the same configuration, the ephemeral resource needs an explicit `depends_on` referencing it. Terraform otherwise opens the ephemeral resource during the plan walk, before the dynamic secret exists, and the plan fails.

Referencing the dynamic secret's `name` is not sufficient on its own — that value comes from configuration and is already known at plan time, so Terraform has no reason to defer.

## Credentials are minted per graph walk

Each `terraform apply` generates credentials **twice** for every instance of this resource: once while building the plan, once while applying it. A bare `terraform plan` generates them once. `count` and `for_each` multiply this.

That is inherent to generating on demand, and it is harmless for AWS — each session expires on its own at the dynamic secret's TTL and costs nothing to leave behind. It does mean lease counts in audit logs grow faster than apply counts.

## Expiration and revocation

AWS assumed-role credentials cannot be revoked early. The session expires on its own at the time given by `expiration`, and revoking the lease answers `400 lease_not_revocable` for this credential type. Choose the dynamic secret's `ttl` accordingly: it must comfortably outlast the longest apply that uses the credential, because there is no way to extend a session once issued.

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
