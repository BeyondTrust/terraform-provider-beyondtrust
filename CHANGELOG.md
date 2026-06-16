# Changelog

## [1.1.0](https://github.com/BeyondTrust/terraform-provider-beyondtrust/compare/v1.0.0...v1.1.0) (2026-06-16)


### Features

* add test cleanup helpers and improve acceptance tests  ([#36](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/36)) ([b706953](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/b70695393ad3b1109b1e511f68208bc99064813f))
* configure oidc auth for acceptance tests CI ([#45](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/45)) ([d272db9](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/d272db99afd28ac56b0263c626777019393f53f3))
* implement typed error handling ([#33](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/33)) ([ecc6b89](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/ecc6b897125ded713b33c69568c68608124d13af))


### Bug Fixes

* Add name and folder path validators with regex patterns ([#53](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/53)) ([7762253](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/77622537525a92b55a9c75e16488036e4177f1af))
* AWS Dynamic Secret merge-patch semantics for optional fields ([#49](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/49)) ([4f3a5cf](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/4f3a5cf9e90ed8eb9def0f69b220fae28876c0e3))
* **ci:** pass --repo to SBOM release upload ([#73](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/73)) ([77bda66](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/77bda66c25273e3e41bcf34df75c71ca8f8effce))
* enable credential validation and remove dead CSRF code ([#60](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/60)) ([e016563](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/e0165634dce6eef8e3c849a33d245e8ef7e112c7))
* prevent CI/CD script injection via release tag name in promote workflow ([3feaeab](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/3feaeab5e276ca37736dd169cdf4975da293a214))
* Secret key deletion in PATCH requests via RFC 7396 merge-patch ([#51](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/51)) ([9e48622](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/9e486228c80001ed8da68dd037bb817e3fc751be))
* Update Terraform version requirement to 1.11 for write-only attributes ([#48](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/48)) ([86c0d9d](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/86c0d9d5ea656ea7a8206ba5fa9ec64b4382d709))
* Validate base URL to prevent SSRF via fragment/query injection ([#50](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/50)) ([bd3e6e8](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/bd3e6e822c852563179c7b6bdb53382af54e3ca2))

## 1.0.0 (2026-06-15)


### Features

* add test cleanup helpers and improve acceptance tests  ([#36](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/36)) ([b706953](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/b70695393ad3b1109b1e511f68208bc99064813f))
* configure oidc auth for acceptance tests CI ([#45](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/45)) ([d272db9](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/d272db99afd28ac56b0263c626777019393f53f3))
* implement typed error handling ([#33](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/33)) ([ecc6b89](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/ecc6b897125ded713b33c69568c68608124d13af))


### Bug Fixes

* Add name and folder path validators with regex patterns ([#53](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/53)) ([7762253](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/77622537525a92b55a9c75e16488036e4177f1af))
* AWS Dynamic Secret merge-patch semantics for optional fields ([#49](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/49)) ([4f3a5cf](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/4f3a5cf9e90ed8eb9def0f69b220fae28876c0e3))
* enable credential validation and remove dead CSRF code ([#60](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/60)) ([e016563](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/e0165634dce6eef8e3c849a33d245e8ef7e112c7))
* prevent CI/CD script injection via release tag name in promote workflow ([3feaeab](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/3feaeab5e276ca37736dd169cdf4975da293a214))
* Secret key deletion in PATCH requests via RFC 7396 merge-patch ([#51](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/51)) ([9e48622](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/9e486228c80001ed8da68dd037bb817e3fc751be))
* Update Terraform version requirement to 1.11 for write-only attributes ([#48](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/48)) ([86c0d9d](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/86c0d9d5ea656ea7a8206ba5fa9ec64b4382d709))
* Validate base URL to prevent SSRF via fragment/query injection ([#50](https://github.com/BeyondTrust/terraform-provider-beyondtrust/issues/50)) ([bd3e6e8](https://github.com/BeyondTrust/terraform-provider-beyondtrust/commit/bd3e6e822c852563179c7b6bdb53382af54e3ca2))
