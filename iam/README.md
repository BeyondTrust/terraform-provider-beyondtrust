# IAM policy resources

Terraform resources for BeyondTrust IAM policies — [Cedar](https://www.cedarpolicy.com/)
authorization policies that grant people, workloads, groups and roles access to product resources.

## `beyondtrust_iam_policy`

Manages a single Cedar policy. Each policy holds one `permit` statement saying that a principal may
perform an action on a resource:

```hcl
resource "beyondtrust_iam_policy" "app_team_db_password" {
  provider = beyondtrust.platform

  name = "app-team-read-db-password"

  cedar = <<-EOT
    @siteId("${var.site_id}")
    permit(
      principal == Pathfinder::User::Email::"app-team@example.com",
      action == WorkloadCreds::Action::"can_read_secret",
      resource == WorkloadCreds::Secret::"/production/database/password"
    );
  EOT
}
```

See `examples/resources/beyondtrust_iam_policy/` for more: granting a group, granting several
actions at once, and generating the Cedar text instead of writing it by hand.

## Setting up the provider

Policies are managed against your organization's **admin site**, while each policy grants access on
a **target site** — usually a different one. An access token reaches a single site, so if you also
manage folders or secrets in the same configuration you need two provider blocks:

```hcl
provider "beyondtrust" {           # product site: folders, secrets
  site_id      = var.site_id
  access_token = var.access_token
}

provider "beyondtrust" {           # admin site: policies
  alias        = "platform"
  site_id      = var.admin_site_id
  access_token = var.admin_access_token
}
```

Add `provider = beyondtrust.platform` to each policy. One `terraform apply` still handles both — a
policy can reference a folder created through the other block, and Terraform orders them correctly.

## Writing the policy

Every policy needs an **`@siteId("<uuid>")` annotation** naming the site it applies to. The site is
never inferred from your provider configuration or your token, so a policy without one is rejected.

**Action and type names must match the service exactly, including case.** They come from the
service's Cedar schema rather than from this provider, so check there for the current list; the
names in the examples are the ones in use today. A name the service does not recognize fails when
you apply, reported with its error code and a trace id.

Not accepted yet: `forbid` statements, `when` and `unless` conditions, and more than one statement
per policy. This provider catches all three during `terraform plan`, so you find out before
anything is sent.

## Things worth knowing

- **Applying waits for the grant to take effect.** Policies activate asynchronously, so a
  successful apply means the access is really in force. Adjust with a `timeouts` block.
- **A policy may name a resource that does not exist yet.** It reports `WAITING_FOR_RESOURCE` and
  starts working on its own once the resource is created. Terraform reports that as a warning, not
  a failure.
- **Names are unique across your whole organization**, not per site, and the name identifies the
  policy. Changing it replaces the policy. If a name is already taken, Terraform refuses to
  overwrite it and points you at `terraform import`.
- **`created_at` changes whenever the policy changes**, because each update writes a new version.
- **There is no concurrency control.** Two configurations managing the same policy name will
  overwrite each other silently.

## Acceptance tests

Lifecycle tests need admin-site credentials, a target site, and a real principal:

```bash
export BEYONDTRUST_ADMIN_SITE_ID=... BEYONDTRUST_ADMIN_ACCESS_TOKEN=...
export BEYONDTRUST_SITE_ID=...
export BEYONDTRUST_TEST_POLICY_PRINCIPAL_EMAIL=...

make test-acc-policy
```

`make test-acc-binding` additionally proves a policy changes what the product API returns. That
needs two more identities on the product site: an owner to create the test fixtures
(`BEYONDTRUST_ACCESS_TOKEN`) and a low-privilege principal whose access must change
(`BEYONDTRUST_TEST_POLICY_PRINCIPAL_TOKEN`). Mint the principal token fresh each run — a token can
keep access to resources the test has already deleted, which would invalidate the result.

Tests skip when credentials are missing, and the suite prints what it skipped and why, because a
run that verifies nothing should not look like one that passed. `BEYONDTRUST_TEST_REQUIRE_ACC=1`
turns that into a failure, as CI does.

Set `BEYONDTRUST_TEST_POLICY_SCHEMA=current` once the newer Cedar schema is deployed. Until then
the tests use the vocabulary the service accepts today, and the list-scoping test skips because the
permission it needs does not exist yet.
