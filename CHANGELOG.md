# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

### 1.0.0 / 2026-05-28

#### Features

* add test cleanup helpers and improve acceptance tests  ([#36](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/36))
* configure oidc auth for acceptance tests CI ([#45](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/45))
* implement typed error handling ([#33](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/33))

#### Bug Fixes

* Add name and folder path validators with regex patterns ([#53](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/53))
* AWS Dynamic Secret merge-patch semantics for optional fields ([#49](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/49))
* prevent CI/CD script injection via release tag name in promote workflow
* Secret key deletion in PATCH requests via RFC 7396 merge-patch ([#51](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/51))
* Update Terraform version requirement to 1.11 for write-only attributes ([#48](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/48))
* Validate base URL to prevent SSRF via fragment/query injection ([#50](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/50))

## [Unreleased]

## [1.0.0] - TBD

### Added

- Initial release of the BeyondTrust Workload Credentials Terraform Provider
- **Resources:**
  - `beyondtrust_workload_credentials_folder` - Manage folder hierarchy
  - `beyondtrust_workload_credentials_static_secret` - Manage static secrets (managed resource)
  - `beyondtrust_workload_credentials_aws_integration` - Manage AWS integrations for dynamic credential provisioning
  - `beyondtrust_workload_credentials_aws_dynamic_secret` - Manage AWS dynamic secrets with role assumption and credential templates
- **Data Sources:**
  - `beyondtrust_workload_credentials_aws_integration` - Read AWS integration configuration
- **Ephemeral Resources:**
  - `beyondtrust_workload_credentials_static_secret` - Retrieve secrets without persisting to state (requires Terraform 1.10+)
- **Features:**
  - Full import support for all managed resources
  - Path-based resource identification for folders and secrets
  - Merge-patch semantics for resource updates (RFC 7396)
  - Tag management via separate metadata endpoints
  - Soft and hard delete support
  - Multi-tenancy support via site ID configuration
  - Auto-generated documentation from schema
  - Comprehensive unit test coverage (68.1%)
  - Acceptance test infrastructure

### Documentation

- Provider configuration and authentication guide
- Resource usage examples with import commands
- Data source usage examples
- Ephemeral resource examples for Terraform 1.10+
- Version compatibility matrix
- Development and testing guides

### Testing

- Unit test coverage: 68.1% overall
  - Client package: 86.4%
  - Provider package: 89.5%
- Acceptance test framework with staging environment support
- Mock client for resource testing

[Unreleased]: https://github.com/beyondtrust/terraform-provider-beyondtrust/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/beyondtrust/terraform-provider-beyondtrust/releases/tag/v1.0.0
