# Testing

[`tests.yml`](../workflows/tests.yml) — triggered on PRs to `main`.

## Jobs

### `unit-tests`
Runs `make test-unit` on every PR (including Dependabot and forks — no secrets
needed) and uploads coverage to Codecov with the `unit` flag.

### `acceptance-tests`
Runs `make test-acc` against a live BeyondTrust backend. Gated to **same-repo,
non-Dependabot PRs** because it needs OIDC and org secrets that fork/Dependabot PRs
cannot access:

```yaml
if: >-
  github.event.pull_request.head.repo.full_name == github.repository &&
  github.actor != 'dependabot[bot]'
```

Flow: checkout → `setup-go` → `setup-terraform` → mint a GitHub OIDC token
(audience = `BEYONDTRUST_SITE_ID`) → `make test-acc` with the BeyondTrust env vars →
upload coverage with the `acceptance` flag.

Both coverage uploads use `if: always()` so coverage is reported even when tests
fail. See [README.md](README.md) for the secret list.
