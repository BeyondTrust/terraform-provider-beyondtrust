# BeyondTrust Auth — Workload Identity Example

A minimal, runnable example that registers a workload identity (OIDC issuer trust config)
so an external workload — here, a GitHub Actions workflow — can federate into BeyondTrust.

## Prerequisites

- A BeyondTrust access token and the **admin site** id. Workload identities are managed
  against the organization's admin site, so the provider must be configured with the
  admin-site credentials.

Credentials are read from environment variables:

```shell
export BEYONDTRUST_API_URL="https://api.beyondtrust.io"
export BEYONDTRUST_SITE_ID="<your-admin-site-uuid>"
export BEYONDTRUST_ACCESS_TOKEN="<your-admin-site-access-token>"
```

## Run

```shell
terraform init
terraform plan
terraform apply
terraform destroy
```

> Developing the provider locally? Build it and use the repo's dev override instead of
> `terraform init`:
>
> ```shell
> make build
> make .terraformrc && eval "$(make tf-local)"
> cd examples/auth-workload-identity
> terraform plan    # dev_overrides skip init
> terraform apply
> ```

## What to try

- Change `description` or `conditions` → an **in-place update**.
- Change `service_name`, `issuer_url`, or `idp_category` → a **replacement** (these are immutable).
- `terraform import beyondtrust_auth_workload_identity.github_ci <identityId>` → import an existing one.
