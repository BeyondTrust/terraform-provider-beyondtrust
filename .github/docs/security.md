# Security Scanning

Three workflows, all reporting to the **Security** tab where applicable. Code-scanning
uploads need `security-events: write`, which fork/Dependabot PRs do not get, so those
jobs are gated to same-repo, non-Dependabot PRs and otherwise rely on push-to-`main`
and the weekly schedule.

## `security.yml`

Triggered on push to `main`, PRs, and a weekly cron (Mondays 09:00 UTC).

| Job | Scope | Notes |
|-----|-------|-------|
| `govulncheck` | Go module vulnerabilities | Runs everywhere; no write/secrets. |
| `dependency-review` | PR dependency diff | PR-only; the key gate for dependency bumps. |
| `trivy` | Filesystem (`vuln`,`secret`) + config (IaC misconfig) scans | Uploads two SARIF categories (`trivy-fs`, `trivy-config`), severity `CRITICAL,HIGH`. Gated (SARIF upload). |

## `codeql.yml`

CodeQL static analysis for Go. Triggered on push to `main`, PRs, and the weekly cron.
Init → autobuild → analyze, uploading results to the Security tab. Gated (SARIF
upload).

## `scorecard.yml`

[OpenSSF Scorecard](https://securityscorecards.dev/) supply-chain posture analysis.
Triggered on push to `main`, branch-protection-rule changes, and the weekly cron (no
PR trigger). Uploads SARIF and publishes results to the public Scorecard API for a
badge.

## Gating condition

The push/PR/schedule workflows use:

```yaml
if: >-
  github.event_name != 'pull_request' ||
  (github.actor != 'dependabot[bot]' &&
   github.event.pull_request.head.repo.full_name == github.repository)
```
