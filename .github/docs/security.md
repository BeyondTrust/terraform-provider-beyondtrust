# Security Scanning

Scanning reports to the **Security** tab where applicable. Code-scanning uploads need
`security-events: write` (+ `actions: read`), which fork/Dependabot PRs do not get, so
those jobs are gated to same-repo, non-Dependabot PRs and otherwise rely on
push-to-`main` and the weekly schedule.

## `security.yml`

Triggered on push to `main`, PRs, and a weekly cron (Mondays 09:00 UTC).

| Job | Scope | Notes |
| --- | --- | --- |
| `govulncheck` | Go module vulnerabilities | Runs everywhere; no write/secrets. |
| `dependency-review` | PR dependency diff | PR-only; the key gate for dependency bumps. |
| `trivy` | Single filesystem scan: `vuln`, `secret`, and `misconfig` (IaC) | One scan covers all three (no separate `config` scan needed); settings live in [`.github/trivy.yaml`](../trivy.yaml). Uploads one SARIF category (`trivy`). Gated (SARIF upload). |

## `codeql.yml`

CodeQL static analysis for Go (advanced setup). Triggered on push to `main`, PRs, and
the weekly cron. Init → autobuild → analyze, uploading results to the Security tab
(`security-events: write` + `actions: read`). Gated (SARIF upload).

> This advanced workflow requires GitHub's **default code-scanning setup to be
> disabled** in repo settings — the two cannot coexist ("CodeQL analyses from advanced
> configurations cannot be processed when the default setup is enabled").

## `scorecard.yml`

[OpenSSF Scorecard](https://securityscorecards.dev/) supply-chain posture analysis.
Triggered on push to `main`, branch-protection-rule changes, and the weekly cron (no
PR trigger). Uploads SARIF and publishes results to the public Scorecard API for a
badge.

## Security Gate (Policy as Code)

A `security-gate` job runs in [`release.yml`](../workflows/release.yml) **before
`goreleaser`**, so a release is blocked if code-scanning alerts are overdue. It uses
[`advanced-security/policy-as-code`](https://github.com/advanced-security/policy-as-code)
via the [`security-gate`](../actions/security-gate/action.yml) composite action and
fails when an open **Code Scanning** alert is past the remediation SLA defined in
[`.github/security-policy.yml`](../security-policy.yml).

> It runs only on the release (push) path, not on PRs: policy-as-code's PR "alert
> diff" call returns `403` with the Actions token, whereas the push path lists
> repo-level alerts, which the token can read.

- **Code scanning only** for now. Dependabot and secret-scanning alerts require a
  GitHub App/PAT to read, so the gate disables them (`--disable-dependabot`,
  `--disable-secret-scanning`, `--disable-dependencies`,
  `--disable-dependency-licensing`) and runs token-free on `github.token`
  (`security-events: read`).
- The policy is **SLA-driven**: `level: all` keeps every severity in scope, and the
  `remediate` windows decide what blocks — alerts within their window do not fail
  the build. Tune the day values to match BeyondTrust's SLA.

## Gating condition

The push/PR/schedule scanning workflows use:

```yaml
if: >-
  github.event_name != 'pull_request' ||
  (github.actor != 'dependabot[bot]' &&
   github.event.pull_request.head.repo.full_name == github.repository)
```
