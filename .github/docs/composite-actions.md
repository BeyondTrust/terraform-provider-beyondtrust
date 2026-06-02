# Composite Actions

Shared step sequences live in [`.github/actions/`](../actions/). Local composite
actions require `actions/checkout` to run first, so the calling job always checks out
the repository before using them.

| Action | Purpose | Inputs |
| --- | --- | --- |
| [`setup-go`](../actions/setup-go/action.yml) | Runs `actions/setup-go` from `go.mod` with module caching enabled. | `check-latest` (default `true`) |
| [`setup-terraform`](../actions/setup-terraform/action.yml) | Reads `.terraform-version` and installs that exact Terraform CLI. | — |
| [`megalinter`](../actions/megalinter/action.yml) | Runs MegaLinter (documentation flavor), archives reports, and uploads SARIF to the Security tab. | `validate-all-codebase`, `github-token` |
| [`security-gate`](../actions/security-gate/action.yml) | Fails the build when an open Code Scanning alert is past its remediation SLA (Policy as Code, code scanning only, token-free). | — |

## Notes

- `setup-go` is used by every Go job (tests, lint, security, build-candidate,
  release) so caching and the Go version stay consistent in one place.
- `setup-terraform` centralizes the `.terraform-version` read used by the docs and
  acceptance-test jobs. The `terraform-fmt` job pins its own version independently
  and does not use this action.
- `megalinter` is consumed by the reusable [`megalinter.yml`](../workflows/megalinter.yml)
  workflow — see [linting.md](linting.md).
- `security-gate` is used by `release.yml` (before `goreleaser`); it reads the
  policy at [`.github/security-policy.yml`](../security-policy.yml) — see
  [security.md](security.md).
