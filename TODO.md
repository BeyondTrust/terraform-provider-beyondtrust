# BeyondTrust Terraform Provider - TODO List

> **Note:** This TODO list was generated from a comprehensive multi-persona audit covering Security, SRE/DevOps, Open Source Readiness, Developer Experience, Terraform Developer, Go Developer, Product Owner, and Customer perspectives. It represents a prioritized roadmap for moving from private to public release.

Based on comprehensive multi-persona audit (Security, SRE/DevOps, Open Source Readiness, DX, Terraform Developer, Go Developer, Product Owner, Customer perspectives).

**Last Updated:** 2026-03-10
**Overall Readiness:** 70% (Private release ready, needs work for public v1.0)

---

## 🚨 CRITICAL BLOCKERS (Must Fix Before ANY Public Release)

### P0 - Cannot Publish Without These

- [ ] **LICENSE File** (Open Source - BLOCKER)
  - Status: Missing
  - Action: Legal team approve MPL-2.0 (or Apache 2.0, MIT)
  - Create: `LICENSE` file with full license text
  - Update: All source files with SPDX license headers
  - Effort: Legal approval + 1 hour
  - Owner: Legal team + DevOps

- [ ] **Git Repository Setup** (Open Source - BLOCKER)
  - Status: No remote origin configured
  - Action: Create GitHub repository at `github.com/beyondtrust/terraform-provider-beyondtrust`
  - Configure: Remote origin, branch protection, PR templates
  - Effort: 30 minutes
  - Owner: DevOps

- [ ] **ZERO Test Coverage** (SRE, Go Dev - BLOCKER)
  - Status: 0 unit tests, 0 acceptance tests, 0 integration tests
  - Action: Implement comprehensive test suite
  - Priority: Cannot release untested code
  - Breakdown:
    - [ ] Unit tests for client package (15+ tests)
    - [ ] Acceptance tests for folder resource (5+ tests)
    - [ ] Acceptance tests for static secret resource (5+ tests)
    - [ ] Acceptance tests for AWS integration resource (5+ tests)
    - [ ] Acceptance tests for AWS dynamic secret resource (5+ tests)
    - [ ] Integration tests with Terratest (3+ scenarios)
  - Effort: 80-120 hours (2-3 weeks)
  - Owner: Development team

- [ ] **No CI/CD Workflows** (SRE, Open Source - BLOCKER)
  - Status: `.github/workflows/` directory is empty
  - Action: Create GitHub Actions workflows
  - Files needed:
    - [ ] `.github/workflows/test.yml` (run tests on PR)
    - [ ] `.github/workflows/lint.yml` (run linters)
    - [ ] `.github/workflows/release.yml` (automated releases)
  - Effort: 8 hours
  - Owner: DevOps

- [ ] **CSRF Protection Disabled** (Security - BLOCKER)
  - Status: TODO comment at `internal/client/client.go:233`
  - Reason: Session endpoint permissions issue (backend)
  - Action: Backend team must fix `/session` endpoint permissions
  - Then: Re-enable CSRF token support in provider
  - Effort: Backend work + 4 hours provider updates
  - Owner: Backend team + Development team
  - Note: Could release with documented workaround if backend delayed

- [ ] **Session Validation Disabled** (Security - BLOCKER)
  - Status: TODO comment at `internal/provider/provider.go:204`
  - Reason: Same backend dependency as CSRF
  - Action: Re-enable after backend fix
  - Effort: Same as CSRF
  - Owner: Backend team + Development team

---

## ⚠️ HIGH PRIORITY (Should Fix Before Public Release)

### P1 - Critical for Quality Release

- [ ] **Community Files Missing** (Open Source)
  - [ ] `CONTRIBUTING.md` - Contribution guidelines
  - [ ] `CODE_OF_CONDUCT.md` - Community standards (use Contributor Covenant 2.1)
  - [ ] `SECURITY.md` - Vulnerability reporting process
  - [ ] `CHANGELOG.md` - Release history and migration notes
  - Effort: 4 hours total
  - Owner: Product + DevOps

- [ ] **GitHub Templates Missing** (Open Source)
  - [ ] `.github/ISSUE_TEMPLATE/bug_report.md`
  - [ ] `.github/ISSUE_TEMPLATE/feature_request.md`
  - [ ] `.github/pull_request_template.md`
  - Effort: 2 hours
  - Owner: DevOps

- [ ] **Missing Resource Documentation** (DX, Terraform Dev)
  - Status: ✅ FIXED (completed in docs commit 2d7b116)
  - ~~Static secret resource docs~~
  - ~~Ephemeral resource docs~~

- [ ] **No Schema Validators** (Terraform Dev)
  - Status: Documented constraints not enforced at runtime
  - Action: Implement validators for all documented patterns
  - Examples:
    - [ ] Name pattern: `^[a-zA-Z0-9\-_@~\*\^%]+ (max 100 chars)
    - [ ] ARN pattern validation
    - [ ] TTL range validation (900-43200 for assumed_role)
    - [ ] External ID character restrictions
    - [ ] Tag limits (max 50 tags, max 256 chars per value)
  - Effort: 16 hours
  - Owner: Development team

- [ ] **Fragile Error Detection** (Go Dev, Terraform Dev)
  - Status: Using string-based 404 detection
  - Current: `strings.Contains(err.Error(), "404")`
  - Action: Implement typed errors with status codes
  - Files: All resources + `internal/client/client.go`
  - Effort: 8 hours
  - Owner: Development team

- [ ] **GoReleaser Configuration** (Open Source)
  - Status: Missing `.goreleaser.yml`
  - Action: Create config for multi-platform builds
  - Platforms: darwin/linux/windows × amd64/arm64
  - Effort: 4 hours
  - Owner: DevOps

- [ ] **golangci-lint Configuration** (Go Dev)
  - Status: No `.golangci.yml` file
  - Action: Create strict linting rules
  - Effort: 2 hours
  - Owner: Development team

---

## 📋 MEDIUM PRIORITY (Nice to Have for v1.0)

### P2 - Quality of Life Improvements

- [ ] **TROUBLESHOOTING.md** (DX, Customer)
  - Status: Partial troubleshooting in AWS integration README
  - Action: Centralized troubleshooting guide
  - Sections:
    - Authentication issues
    - Network/TLS certificate issues
    - State file problems
    - Resource creation failures
    - Import failures
    - Version compatibility
  - Effort: 3 hours
  - Owner: Documentation team

- [ ] **State Schema Versioning** (Terraform Dev)
  - Status: No schema version tracking
  - Action: Add `Version: 1` to all resource schemas
  - Action: Implement `UpgradeState` functions
  - Risk: Breaking changes will require manual state manipulation
  - Effort: 8 hours
  - Owner: Development team

- [ ] **Improved Error Messages** (DX)
  - Status: Generic error wrapping
  - Action: Add common error guidance in error messages
  - Example: "Folder already exists" → include import command
  - Effort: 4 hours
  - Owner: Development team

- [ ] **ForceNew for Immutable Attributes** (Terraform Dev)
  - Status: Missing on some attributes
  - Action:
    - [ ] `integration_name` should RequiresReplace
    - [ ] `credential_type` should RequiresReplace
  - Effort: 1 hour
  - Owner: Development team

- [ ] **UseStateForUnknown Plan Modifiers** (Terraform Dev)
  - Status: Missing on some computed attributes
  - Action: Add to `created_at`, `deleted_at`, `integration_id`
  - Effort: 2 hours
  - Owner: Development team

- [ ] **Code Duplication Cleanup** (Go Dev)
  - Status: Tag update logic duplicated across 3 files
  - Status: Import state logic duplicated across 4 files
  - Action: Extract to `internal/provider/helpers.go`
  - Effort: 6 hours
  - Owner: Development team

- [ ] **Provider Configuration Defaults** (Terraform Dev)
  - Status: Manual default application in Configure()
  - Action: Use framework defaults (stringdefault.StaticString)
  - Benefit: Defaults visible in schema
  - Effort: 3 hours
  - Owner: Development team

---

## 📊 BACKEND API DEPENDENCIES

### Changes Needed in Backend for Full Functionality

- [ ] **Session Endpoint Permissions** (CRITICAL)
  - Current: Requires admin permissions
  - Needed: Allow service accounts to validate own session
  - Impact: Enables CSRF + session validation in provider
  - Endpoint: `GET /api/v1/session`
  - Owner: Backend team
  - Priority: P0

- [ ] **Integration Tags Support** (HIGH)
  - Current: No tags endpoint for integrations
  - Needed: `PATCH /api/v1/integrations/{name}/metadata/tags`
  - Impact: Allows tagging integrations in Terraform
  - Owner: Backend team
  - Priority: P1

- [ ] **Force Delete for Dynamic Secrets** (HIGH)
  - Current: Must manually revoke all leases first
  - Needed: `DELETE /api/v1/dynamic/{name}?force=true`
  - Behavior: Auto-revoke leases before deletion
  - Impact: Better Terraform destroy experience
  - Owner: Backend team
  - Priority: P1
  - Ticket: Created in audit findings

- [ ] **List Operations for Data Sources** (MEDIUM)
  - Needed for discovery data sources:
    - `GET /api/v1/folders?path={path}&recursive=true`
    - `GET /api/v1/integrations?type=aws`
    - `GET /api/v1/dynamic?path={path}`
  - Impact: Enables resource discovery workflows
  - Owner: Backend team
  - Priority: P2

---

## 🚀 POST-v1.0 ENHANCEMENTS

### Features for Future Releases

- [ ] **Additional AWS Credential Types** (v1.1)
  - IAM User credentials
  - Federation token credentials
  - Session token credentials
  - Currently: Only `assumed_role` implemented

- [ ] **Additional Data Sources** (v1.2)
  - `beyondtrust_secrets_aws_dynamic_secret` (commented out)
  - `beyondtrust_secrets_lease` (commented out)
  - `beyondtrust_secrets_folder` (list/discovery)
  - `beyondtrust_secrets_static_secret` (metadata only)

- [ ] **Restore Operations** (v1.2)
  - Soft delete recovery for folders, secrets, integrations
  - Disaster recovery workflows

- [ ] **Authorization Resources** (v2.0)
  - OpenFGA tuple management
  - Permission resources
  - Breaking change - new resource names

- [ ] **Timeout Configuration** (v1.3)
  - Per-resource timeout blocks
  - For slow operations (AWS integration with IAM propagation)

- [ ] **Structured Logging** (v1.3)
  - Use `terraform-plugin-log/tflog`
  - Debug mode with request/response logging
  - Correlation IDs for tracing

- [ ] **Metrics and Tracing** (v1.4)
  - OpenTelemetry integration
  - Operation counters, latency histograms
  - Distributed tracing

---

## ✅ COMPLETED ITEMS

### Recently Completed (2026-03-10)

- [x] **Documentation Examples** (commit 2d7b116)
  - [x] Custom templates for all resources
  - [x] Working .tf examples (19 files)
  - [x] Import examples (.sh files)
  - [x] QUICKSTART.md guide
  - [x] Ephemeral resource documentation
  - [x] Static secret resource documentation

- [x] **Documentation Validation**
  - [x] All docs validated with `tfplugindocs validate`
  - [x] Terraform Registry compliant

- [x] **go.mod Cleanup**
  - [x] Removed terraform-plugin-docs from root dependencies
  - [x] Kept only in tools/go.mod (correct location)

---

## 📅 RECOMMENDED TIMELINE

### Week 1: Legal & Infrastructure
- [ ] Legal approval for MPL-2.0 license
- [ ] Create GitHub repository (private first)
- [ ] Add LICENSE and community files
- [ ] Set up basic CI/CD (test, lint)
- [ ] Backend team: Start session endpoint fix

### Week 2: Testing Foundation
- [ ] Implement acceptance tests (20+ tests minimum)
- [ ] Add unit tests for client package
- [ ] Integration tests with Terratest
- [ ] Fix 404 detection (typed errors)

### Week 3: Quality Improvements
- [ ] Implement schema validators
- [ ] Add ForceNew to immutable attributes
- [ ] Create TROUBLESHOOTING.md
- [ ] Re-enable CSRF + session validation (if backend ready)

### Week 4: Release Preparation
- [ ] Test against Terraform 1.0, 1.10, latest
- [ ] Security audit sign-off
- [ ] GoReleaser configuration
- [ ] Terraform Registry submission prep
- [ ] Publish v1.0.0

---

## 🎯 GO/NO-GO CHECKLIST

### Private Release (Ready Now)
- [x] Core functionality works
- [x] Documentation complete
- [x] Examples functional
- [ ] Git repository exists
- [ ] Basic CI/CD

### Public Beta (v0.9.0-beta) - 2 Weeks
- [ ] LICENSE file
- [ ] GitHub repository public
- [ ] Community files
- [ ] Basic test coverage (50%+)
- [ ] Beta disclaimer in README

### Production v1.0 - 4 Weeks
- [ ] All P0 blockers resolved
- [ ] 80%+ test coverage
- [ ] Full CI/CD pipeline
- [ ] Backend session permissions fixed
- [ ] Security audit passed
- [ ] Terraform Registry submitted

---

## 📊 CURRENT STATUS

| Area                   | Score  | Status                                            |
|------------------------|--------|---------------------------------------------------|
| Code Quality           | 8/10   | ✅ Good                                           |
| Documentation          | 8.5/10 | ✅ Excellent (after docs commit)                  |
| Testing                | 0/10   | ❌ Critical gap                                   |
| CI/CD                  | 0/10   | ❌ Missing                                        |
| Open Source Readiness  | 6/10   | ⚠️ Needs LICENSE + community files                |
| Security               | 7/10   | ⚠️ Two disabled features (backend dependency)     |
| Overall                | 7/10   | ⚠️ Private release ready, not public ready        |

---

## 📝 NOTES

- This list is based on 8-persona comprehensive audit conducted 2026-03-10
- Audit included: Security Engineer, SRE/DevOps, Open Source Readiness, Developer Experience, Terraform Developer (Pedantic), Go Developer (Pedantic), Product Owner, Customer perspectives
- Priorities may shift based on business needs and timeline constraints
