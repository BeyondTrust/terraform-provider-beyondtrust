# Auth service resources

Terraform resources for the BeyondTrust **auth** service, managed against the
operator's admin site at `/site/{site-id}/platform/auth`.

## `beyondtrust_auth_workload_identity`

Manages an OIDC issuer trust config ("workload identity") used for workload identity
federation — e.g. letting a GitHub Actions pipeline or an AWS workload exchange its OIDC
token for a BeyondTrust token.

### Behavior

- **CRUD targets the admin site** (the provider's configured `site_id`), via
  `POST/GET/PUT/DELETE /site/{admin-site}/platform/auth/workload-identities[/{id}]`.
- The resource's **`site_id`** is the site the identity *grants access to* — any site in
  the organization. It defaults to the provider's site when omitted.
- **Immutable** fields (`service_name`, `issuer_url`, `idp_category`, `site_id`) carry
  `RequiresReplace`: changing one destroys and recreates the resource. The API also rejects
  such a change with a 400 as a backstop.
- **Mutable** fields (`conditions`, `description`, `scope_level`, `registered_scopes`) are
  updated in place.
- **Import** by identity id: `terraform import beyondtrust_auth_workload_identity.example <identityId>`.

### Validation (plan-time)

- `idp_category` must be one of `GitHubActions`, `AzureEntra`, `Custom`.
- `registered_scopes` must be non-empty.
- `conditions` values: non-empty, ≤256 chars, at most one `*` and only as the final character.
- For `GitHubActions`: condition keys are restricted to the GitHub allowlist, and the `sub`
  value must start with `repo:<owner>/<repo>` (any `*` must come after the slash).

### Notes

- Org-admin role is **not** a resource attribute. It is derived server-side by the auth service from
  the registered scopes and the access site, so there is nothing to manage here.

See `examples/resources/beyondtrust_auth_workload_identity/` for usage.

### Acceptance tests

Workload-identity endpoints require an org-admin caller operating against the org's **admin
site**, which has its own dedicated credentials (separate from the normal-site credentials the
secrets tests use). Set `BEYONDTRUST_ADMIN_SITE_ID` and `BEYONDTRUST_ADMIN_ACCESS_TOKEN` (in
addition to the base `BEYONDTRUST_*` vars) to run them; they are skipped when those are unset.
