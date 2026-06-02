# Linting & Code Quality

## `lint.yml`

Triggered on PRs to `main`. Three independent jobs:

| Job | What it does |
|-----|--------------|
| `docs-validation` | `setup-go` + `setup-terraform`, installs `tfplugindocs`, runs `make docs-validate`. |
| `golangci-lint` | `setup-go`, installs `golangci-lint` + `gofumpt`, runs `make lint`. Runs on Dependabot PRs too (dependency bumps are exactly when lint matters). |
| `terraform-fmt` | `terraform fmt -check -recursive examples/` against a pinned Terraform version. |

## MegaLinter

MegaLinter (documentation flavor) lints non-Go files: Markdown, YAML, JSON, Bash,
Terraform, and spelling. Configuration lives in [`.mega-linter.yml`](../../.mega-linter.yml).

The run logic is centralized in the [`megalinter`](../actions/megalinter/action.yml)
composite action, wrapped by a reusable workflow and two thin callers:

| File | Role |
|------|------|
| [`megalinter.yml`](../workflows/megalinter.yml) | Reusable workflow (`workflow_call`), input `full`. Calls the composite action. |
| [`megalinter-full.yml`](../workflows/megalinter-full.yml) | PRs that **touch** linter config (`.github/workflows/megalinter*.yml`, `.mega-linter.yml`, `.github/linters/**`) → full-codebase scan. |
| [`megalinter-shallow.yml`](../workflows/megalinter-shallow.yml) | PRs that **don't** touch linter config → changed-files-only scan. |

Both callers are gated to same-repo, non-Dependabot PRs because the status reporter
and SARIF upload need a writable token.

> **Required-check caveat:** full and shallow are mutually exclusive by path filter
> and surface as different check names. Do not mark either as a required status check
> — PRs taking the other path would block on a check that never runs.

## PR title validation

[`validate-pr-title.yml`](../workflows/validate-pr-title.yml) enforces
[Conventional Commits](https://www.conventionalcommits.org/) on PR titles (feeds
release-please). The core validation works on all PRs; the pass/fail comment steps
need `pull-requests: write` and degrade on fork/Dependabot PRs.
