# Build & Release

## `build-candidate.yml`

Triggered on every PR to `main`. Runs `goreleaser release --snapshot --clean` — a
dry-run build across the full OS/arch matrix with **no signing, no publish, and no
secrets**. It proves the code still produces a releasable artifact set, catching
`.goreleaser.yml` or compile breakage on the PR that introduces it (including
Dependabot dependency bumps). Uses `fetch-depth: 0` so GoReleaser can compute the
version from tag history.

## `release.yml`

Triggered on push to `main`. Runs as **dependent jobs in a single workflow run**:

```text
push to main
  └─ release-please ── maintains the release PR; on merge cuts the tag
  │                    + a DRAFT GitHub release (outputs release_created, tag_name)
  ├─ security-gate ─── (if release_created) fail if Code Scanning alerts are past SLA
  ├─ goreleaser ────── (needs security-gate) build + GPG-sign → upload to draft
  ├─ sbom ──────────── generate SBOM → attach to draft (best-effort)
  └─ publish ───────── flip release out of draft → fires "release: published"
                       → Terraform Registry webhook ingests the complete release
```

The `security-gate`, `goreleaser`, `sbom`, and `publish` jobs are gated on
`needs.release-please.outputs.release_created == 'true'`, so they run only when a
release PR merge actually cuts a release. `security-gate` (Policy as Code, code
scanning SLA) must pass before `goreleaser` builds — see [security.md](security.md).

`publish` runs after `sbom` for ordering, but the **SBOM is best-effort**: publish
proceeds even if `sbom` fails, as long as `goreleaser` succeeded
(`if: always() && … && needs.goreleaser.result == 'success'`). A failed SBOM does not
block the release.

### release-please configuration

- [`release-please-config.json`](../../release-please-config.json) — `release-type:
  simple`, draft releases, root package.
- [`.release-please-manifest.json`](../../.release-please-manifest.json) — current
  version tracking.

See [README.md](README.md) for the GPG secrets and the "Allow GitHub Actions to
create and approve pull requests" repo setting that release-please requires.
