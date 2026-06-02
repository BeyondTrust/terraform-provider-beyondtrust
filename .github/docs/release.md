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

```
push to main
  └─ release-please ── maintains the release PR; on merge cuts the tag
  │                    + a DRAFT GitHub release (outputs release_created, tag_name)
  ├─ goreleaser ────── (if release_created) build + GPG-sign → upload to draft
  ├─ sbom ──────────── generate SBOM → attach to draft
  └─ publish ───────── flip release out of draft → fires "release: published"
                       → Terraform Registry webhook ingests the complete release
```

The `goreleaser`, `sbom`, and `publish` jobs are gated on
`needs.release-please.outputs.release_created == 'true'`, so they run only when a
release PR merge actually cuts a release.

### release-please configuration

- [`release-please-config.json`](../../release-please-config.json) — `release-type:
  simple`, draft releases, root package.
- [`.release-please-manifest.json`](../../.release-please-manifest.json) — current
  version tracking.

See [README.md](README.md) for the GPG secrets and the "Allow GitHub Actions to
create and approve pull requests" repo setting that release-please requires.
