# Secrets & Variables

Every secret and variable referenced by the CI workflows, what consumes it, and
whether it is required. No repository `vars` are used — only secrets and the
automatic `GITHUB_TOKEN`.

## Required — referenced in CI

| Secret | Used by | Required | Notes |
| --- | --- | --- | --- |
| `GITHUB_TOKEN` | build-candidate, megalinter, release (goreleaser), validate-pr-title | auto | Provided automatically by GitHub; nothing to provision. |
| `GPG_PRIVATE_KEY` | `release.yml` → goreleaser | ✅ | Signs the release checksums; the Terraform Registry requires signed releases. |
| `GPG_PASSPHRASE` | `release.yml` → goreleaser | ✅ | Passphrase for `GPG_PRIVATE_KEY`. |
| `BEYONDTRUST_API_URL` | `tests.yml` (acceptance) | ✅ | Acceptance config rejects an empty value (`internal/acctest/config.go`). |
| `BEYONDTRUST_SITE_ID` | `tests.yml` (acceptance) | ✅ | Required by acceptance config **and** used as the GitHub OIDC token audience. |
| `BEYONDTRUST_SERVICE_NAME` | `tests.yml` (acceptance) | ✅ | OIDC `X-BT-Service-Name` header (`internal/provider/provider.go`). |
| `BEYONDTRUST_TEST_AWS_ROLE_ARN` | `tests.yml` (acceptance) | ✅ | Gates the AWS integration tests — they `t.Skip` when it is unset. |
| `BEYONDTRUST_TEST_AWS_ROLE_ARN_2` | `tests.yml` (acceptance) | ✅ | Second role ARN used by the AWS integration tests. |

`BEYONDTRUST_ACCESS_TOKEN` is also required by the acceptance tests but is **minted
from GitHub OIDC at runtime** (not stored as a secret).

## Gaps — expected by tests but not configured in CI

| Secret | Effect if unset | Recommendation |
| --- | --- | --- |
| `BEYONDTRUST_AWS_ACCOUNT_ID` | The AWS *integration* tests `t.Skip` (`internal/acctest/aws_helpers.go`), so they are **silently skipped** in CI despite the two role-ARN secrets being set. | Add to the acceptance-tests job `env:` (as a secret) to actually run those tests. |
| `BEYONDTRUST_TEST_AWS_EXTERNAL_ID` | Optional — a random external ID is generated per run. | Leave unset unless deterministic external-ID testing is needed. |

## Not required in CI — defaulted provider/test knobs

These environment variables are read by the provider/tests but have defaults or are
optional, so they are intentionally **not** provided in CI:

| Variable | Purpose |
| --- | --- |
| `BEYONDTRUST_API_VERSION` | API version; defaults to `client.DefaultAPIVersion`. |
| `BEYONDTRUST_API_PATH_VERSION` | API path version override. |
| `BEYONDTRUST_INSECURE` | Disable TLS verification (local/dev only). |
| `BEYONDTRUST_TIMEOUT` | Client timeout override. |
| `BEYONDTRUST_ROLE` | Provider auth role option. |

## Conventions

- Store all secrets at the **repository** level (not inherited from an org secret that
  forks/public access cannot resolve).
