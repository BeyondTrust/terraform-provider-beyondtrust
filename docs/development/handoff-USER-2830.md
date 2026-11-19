# Handoff: multi-site workload identity in the provider (USER-2830)

**Ticket:** [USER-2830](https://beyondtrust.atlassian.net/browse/USER-2830) · **Epic:** [USER-2878](https://beyondtrust.atlassian.net/browse/USER-2878) · **Design:** `pathfinder-platform-guide/docs/nomine/design/DD-20260811-multi-site-workload-identity.md`
**Release:** `feat:` commit → minor bump (1.4.1 → 1.5.0) via release-please. **Review:** `@beyondtrust/volt` (`.github/CODEOWNERS`).

The design is settled. This doc states each decision and why, so you can implement it without re-deriving it. Where this doc and the ticket or design doc disagree, **this doc wins** (see [Corrections](#corrections-to-the-ticket-and-design-doc)).

## Glossary

| Term | Meaning |
| --- | --- |
| **Site binding** | One `(site_id, registered_scopes, scope_level)` triple. An identity has one or more. |
| **`site` block** | The new set-nested block. One block = one site binding. |
| **Top-level access fields** | The existing `site_id`, `registered_scopes`, `scope_level`. Deprecated by this change. |
| **Default site** | The provider's configured site (`client.SiteID`). Used when neither form is configured. |
| **Admin site** | The site whose id equals `organization_id`. Binding to it grants org admin. |
| **Converge** | The API's update semantics: after a `PUT` with `sites[]`, the identity is bound to exactly that set. |

## What ships

Resource `beyondtrust_auth_workload_identity` (`auth/resources/workload_identity_resource.go:99`) gains a `site` set-nested block:

```hcl
resource "beyondtrust_auth_workload_identity" "secrets" {
  service_name = "secrets-service"
  # ...

  site {
    site_id           = "aaaa..."
    registered_scopes = ["secrets"]
  }
  site {
    site_id           = "bbbb..."
    registered_scopes = ["api"]
    scope_level       = "org"
  }
}
```

## Decisions

### 1. Two forms, mutually exclusive

- **Blocks set:** the top-level access fields are null in plan and state. This holds even with exactly one block, so there's one rule and no special case for cardinality 1.
- **Blocks plus any top-level access field in config is a plan-time error** (`ConflictsWith`).
  - *Why:* for an `Optional + Computed` attribute, the provider can't null a value the practitioner wrote. HashiCorp: "if an attribute value is configured, it is never valid to change that value in the plan" ([plan modification](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification)). Refusing at plan time is the only honest way to express "ignored".
- **Neither set:** today's behaviour. The identity binds to the default site (`workload_identity_resource.go:316-319`), which is returned in `site_id`, with `scope_level = "site"`.

### 2. Top-level fields: new schema behaviour

| Field | Today | After |
| --- | --- | --- |
| `site_id` | Optional + Computed, `RequiresReplace` + `UseStateForUnknown` (`:128-136`) | Optional + Computed, **no `RequiresReplace`** (decision 3). Planned null when blocks are set. Otherwise: the config value, or the state value, or the default site. |
| `registered_scopes` | Required (`:156-160`) | Optional + Computed. A config validator requires it only when no block is set. Planned null when blocks are set. |
| `scope_level` | `stringdefault.StaticString("site")` (`:150-155`) | Default removed. A plan modifier plans `"site"` only when no block is set and it's unconfigured, and plans null when blocks are set. |
| all three | | `DeprecationMessage`, removed in the next major. |

*Why the `scope_level` default must go:* a schema default plans `"site"` unconditionally. With blocks set, the provider returns null, and Terraform fails with *provider produced inconsistent result*.

*Pitfall, the same failure on `site_id`:* `UseStateForUnknown` copies the prior state value into the plan. After migrating to blocks, that would plan the old `"a"` where the result is null. Replace it with logic that plans null whenever blocks are set.

*Keep today's default-site stability:* when there are no blocks and `site_id` is unconfigured, prefer the state value over `client.SiteID`. Today `UseStateForUnknown` does this, so changing the provider's configured site doesn't move existing identities.

### 3. No field forces replacement for a site change *(change from the ticket)*

The ticket replaces the bare `RequiresReplace()` on `site_id` with a guarded `RequiresReplaceIf`. **Instead, remove `RequiresReplace` from `site_id` entirely.** Nothing in a `site` block is ForceNew either.

*Why:*

- **Replacing is destructive.** A replace means a new `identity_id`, a new token `sub`, and every RBAC grant on the old principal orphaned.
- **The API no longer needs it.** `PUT /api/workload-identities/{id}` now converges any site set in place under the same id (nomine `f09430fea`, #2569). A replace no longer buys anything.
- **The guard was inconsistent.** It replaced on `site_id = "a"` → `"b"` but updated in place on `site { "a" }` → `site { "b" }`, the same change written two ways. It also replaced when going from blocks back to top-level `site_id`: `RequiresReplace` fires whenever plan ≠ state on update, null → value included ([godoc](https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier#RequiresReplace)).

`service_name`, `issuer_url` and `idp_category` keep `RequiresReplace`. The API treats them as immutable.

### 4. The wire format is always `sites[]`

On both Create and Update, always send `sites: [{siteId, registeredScopes, scopeLevel}]` and never the flat `siteId`/`registeredScopes`/`scopeLevel`. `buildRequest` (`:311-337`) builds the list:

1. from the blocks, when set;
2. otherwise, as one element from the top-level fields;
3. otherwise, as one element for the default site.

*Why:*

- **A flat `PUT` can fail.** The API returns 400 "send sites[] to update it" for a flat `PUT` on an identity bound to more than one site (`nomine/src/backend/Core/Nomine.Application/Issuer/Commands/UpdateIssuerCommand.cs`, the `FlatSingleSiteRequest` check; `nomine/docs/adr/ADR-20261005-flat-update-refuses-multi-site-identity.md`). A config still on top-level `site_id` would then be stuck whenever someone added a binding outside Terraform. With `sites[]`, the same apply converges, which is the drift correction Terraform should do.
- **Nothing is lost.** For one site, flat and `sites[]` map to the same binding (`nomine/.../Models/Issuer/IssuerSitesInputMapper.cs`).

The API rejects sending both shapes, neither shape, or `sites: []` with a 400 (same file).

### 5. Read: `sites[]` is the source of truth

Create, `GET` and `PUT` responses all carry `sites[]` (`IssuerResponse.FromIdentity`, `nomine/.../Models/Issuer/IssuerResponse.cs:55-73`). The flat `siteId`/`scopeLevel`/`registeredScopes` are set only when there's exactly one binding, and are null otherwise. **Ignore the flat fields** and map from `sites[]`:

| Prior state uses | Bindings returned | Write to state |
| --- | --- | --- |
| blocks | any | blocks; top-level null |
| top-level, or nothing (import) | exactly 1 | top-level fields; no blocks |
| top-level, or nothing (import) | more than 1 | blocks; top-level null |

- **Import of a single-site identity fills the top-level fields** (row 2). This keeps existing `import.sh` and top-level configs diff-free.
- **Row 3 covers drift and multi-site import.** The resulting plan is an in-place update (decision 3).
- **Existing bug to avoid:** `applyRead` (`auth/resources/workload_identity_helpers.go:31-53`) writes `""` and an empty list when the flat fields are null. Make sure the new mapping never reaches that path.
- **Update can take state from the `PUT` response.** It now includes `sites[]`, so no follow-up `GET` is needed.

### 6. Plan-time validation in each block

Check only what the config alone can show, following the existing `ValidateConfig` pattern (`:196-223`):

- unique `site_id` across blocks;
- `registered_scopes` non-empty;
- `scope_level` is `org` or `site`. Inside a block it defaults to `site`, as in the API (`IssuerSiteBinding.cs:18`).

Leave these to the API so the provider can't drift from it: the 99-site cap, GUID format, and site ownership by the org (`CreateIssuerCommand.cs:78-95, 311-325`; ownership 404 lists every unowned site).

### 7. Docs warn that the admin site grants org admin

A binding to the admin site (`site_id == organization_id`) mints tokens with `Organization_Org_Admin`, whatever `registered_scopes` and `scope_level` say (`node-lambda-authentication/functions/check-auth-api/src/s2s/claims-mapper.ts:127-131`). This includes one `site` block among several. State it in the descriptions of both the `site` block and top-level `site_id`, then regenerate docs (`make generate`) so it lands in `docs/resources/auth_workload_identity.md`.

## Acceptance criteria

The test approach is up to the team.

- Migrating `site_id = "a"` to one `site { site_id = "a" }` plans as an in-place update.
- Adding, removing or re-scoping a `site` block is in place.
- Changing top-level `site_id` is in place *(change from the ticket, decision 3)*.
- Blocks plus any top-level access field fails at plan time.
- Setting neither binds to the default site as before, with `scope_level = "site"`.
- A block-configured identity applies with no inconsistent-result error on `site_id`, `scope_level` or `registered_scopes`, at one block and at several.
- Create and Update always send `sites[]` (decision 4). A top-level config applies cleanly against an identity that has extra bindings added outside Terraform.
- Importing a single-site identity populates the top-level fields; importing a multi-site identity populates blocks.
- Duplicate `site_id`, empty `registered_scopes`, or an invalid `scope_level` in a block fails at plan time.
- Plans touching the deprecated top-level fields emit the deprecation warning.
- The resource docs for the `site` block and top-level `site_id` carry the admin-site warning.

## Out of scope

- Removing the top-level access fields (next major).
- Data sources.
- Exposing `expectedAud` per site.

## Open item to verify early

Defaults and computed values inside a **set**-nested block: a block's `scope_level` defaults to `site`. Confirm early that a set element whose `scope_level` is omitted plans and refreshes without a perpetual diff (unverified). If it doesn't, raise it before switching to a list, because the ticket specifies a set.

## Corrections to the ticket and design doc

- **Resource name:** the ticket and its example say `beyondtrust_workload_identity`. The resource is `beyondtrust_auth_workload_identity`.
- **Design doc is stale:**
  - DD-20260811 still specifies a `site_ids` string set, the flat API field `siteIds`, and flat response fields that are null "at any cardinality, one included" (DD:469-534).
  - The shipped API takes `sites[]` with per-site access, and fills the flat response fields whenever there's exactly one binding.
  - Its claims-mapper citation (`:153-158`) is now `:127-131`.
- **`RequiresReplaceIf` guard:** superseded by decision 3.
- **Update constraint:** the API originally refused multi-site updates (USER-3002). That's resolved on nomine master (`f09430fea`, #2569).
