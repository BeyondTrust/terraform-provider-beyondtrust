# CI/CD — Internal Developer Notes

Internal documentation for this repository's GitHub Actions workflows and composite
actions. These notes live under `.github/docs/` (not the top-level `docs/`, which is
published to the Terraform Registry) so they stay internal to contributors.

## Index

| Doc | Covers |
| --- | --- |
| [composite-actions.md](composite-actions.md) | Shared composite actions in `.github/actions/`. |
| [testing.md](testing.md) | `tests.yml` — unit and acceptance tests. |
| [linting.md](linting.md) | `lint.yml`, MegaLinter, PR-title validation. |
| [security.md](security.md) | `security.yml`, `codeql.yml`, `scorecard.yml`. |
| [release.md](release.md) | `build-candidate.yml`, `release.yml`, release-please. |
| [secrets.md](secrets.md) | All CI secrets/variables and whether each is required. |

Related config files: [`.github/trivy.yaml`](../trivy.yaml) (Trivy scan settings) and
[`.github/security-policy.yml`](../security-policy.yml) (Code Scanning remediation SLA
for the security gate).

## Principles

- **Least privilege.** Every workflow sets `permissions: {}` at the top and grants
  each job only the scopes it needs.
- **Pinned actions.** All third-party actions are pinned to a full commit SHA with a
  trailing `# vX.Y.Z` comment. Dependabot bumps them (see
  [`dependabot.yaml`](../dependabot.yaml)).
- **No private org dependencies.** Workflows use only public actions — no
  `BeyondTrust/*` actions, reusable workflows, or GitHub App tokens — so the repo
  works as a public repository.
- **Concurrency.** Every workflow defines a `concurrency` group (above
  `permissions`) and cancels superseded runs. `release.yml` groups by workflow only.
- **Hardened checkout.** Every `actions/checkout` sets `persist-credentials: false`,
  except `megalinter.yml` which needs persisted credentials for its authenticated
  base-ref git diff on a private repo.
- **Conventions.** Jobs are named in Title Case; steps in sentence case. A blank line
  separates each job and each step.

## Bot and fork behavior

On **Dependabot PRs** (and **fork PRs** once public) GitHub provides a **read-only
`GITHUB_TOKEN`** and withholds Actions secrets. Jobs that need write scopes or
secrets are gated accordingly:

| Job | On Dependabot / fork PR | Reason |
| --- | --- | --- |
| `tests / unit-tests` | runs | No secrets needed. |
| `tests / acceptance-tests` | **skipped** | Needs OIDC + BeyondTrust secrets. |
| `lint / *` | runs | No secrets/write needed. |
| `security / govulncheck` | runs | No write needed. |
| `security / dependency-review` | runs | The key check for dependency PRs. |
| `security / trivy` | **skipped** | SARIF upload needs `security-events: write`. |
| `codeql / analyze` | **skipped** | SARIF upload needs `security-events: write`. |
| `megalinter` | **skipped** | Status reporter + SARIF upload need write. |
| `build-candidate / build` | runs | Snapshot build needs no secrets. |
| `validate-pr-title` | runs | Core check works; comment steps degrade. |

Skipped security scans are still covered on push-to-`main` and the weekly schedule.

## Secrets

See [secrets.md](secrets.md) for the full secret/variable reference (what each is
used by and whether it is required). In short: `GPG_PRIVATE_KEY`/`GPG_PASSPHRASE` for
release signing and the `BEYONDTRUST_*` secrets for acceptance tests; `GITHUB_TOKEN`
is automatic; no PAT or GitHub App token is used anywhere.

## Repository settings

- **Workflow permissions** (Settings → Actions → General): default `GITHUB_TOKEN` set
  to **read-only** ("Read repository contents and packages permissions"). Workflows
  request write scopes per job, so the read-only default is correct. Keep **"Allow
  GitHub Actions to create and approve pull requests"** enabled — release-please needs
  it to open and maintain the release PR.
- **Signed commits** are enforced by a **branch ruleset** ("Require signed commits"),
  not a workflow.
- **Suggested required status checks:** `Unit Tests`, the `lint` jobs, `govulncheck`,
  `Analyze Go`, `GoReleaser Snapshot Build`, `Validate PR Title`. (`Security Gate` runs
  on release, not PRs, so it isn't a PR status check.) Avoid requiring the full/shallow
  MegaLinter checks — they are mutually exclusive by path filter, so requiring one
  blocks PRs that take the other path.
