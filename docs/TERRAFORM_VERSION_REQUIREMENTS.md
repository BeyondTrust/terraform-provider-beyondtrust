# Terraform Version Requirements

## Current Implementation

This provider currently **requires Terraform 1.10 or later** for full functionality.

### Why Terraform 1.10+?

The provider uses **ephemeral resources** for secure secret handling, which were introduced in Terraform 1.10. Ephemeral resources ensure that sensitive values (like AWS external IDs, secret values) are never persisted in Terraform state or plan files.

### Affected Features

- **Static Secrets with Ephemeral Read**: Reading secret values without storing them in state
- **Write-Only Attributes**: The `secret_wo` pattern for static secrets

### Features That Work on Earlier Versions

The following features work on Terraform 1.0+:
- Folder management (`beyondtrust_secrets_folder`)
- AWS Integration resources (`beyondtrust_secrets_aws_integration`)
- AWS Dynamic Secret resources (`beyondtrust_secrets_aws_dynamic_secret`)
- Data sources

## Future Considerations for Production Release

### Version Compatibility Strategy

Before publishing to the Terraform Registry, consider:

1. **Dual Implementation Approach**
   - Maintain ephemeral resource implementation (Terraform 1.10+)
   - Provide fallback data source implementation (Terraform 1.0+)
   - Use provider version constraints to guide users

2. **OpenTofu Compatibility**
   - Test against OpenTofu releases
   - Document any differences in behavior
   - Consider OpenTofu-specific features if beneficial

3. **Version Matrix Testing**
   - Test against Terraform 1.0, 1.5, 1.10, latest
   - Test against OpenTofu stable releases
   - Document supported version ranges in registry metadata

### Recommended terraform Block for Users

**With Ephemeral Resources (Recommended):**
```hcl
terraform {
  required_version = ">= 1.10.0"

  required_providers {
    beyondtrust = {
      source  = "beyondtrust/beyondtrust"
      version = "~> 1.0"
    }
  }
}
```

**Without Ephemeral Resources (Legacy):**
```hcl
terraform {
  required_version = ">= 1.0.0"

  required_providers {
    beyondtrust = {
      source  = "beyondtrust/beyondtrust"
      version = "~> 1.0"
    }
  }
}

# Note: Secrets will be stored in state (marked sensitive)
# For production, upgrade to Terraform 1.10+ for ephemeral resources
```

## Implementation Notes

### Current Architecture

```text
terraform-plugin-framework v1.18.0
├── Ephemeral Resources (framework v1.14+, Terraform 1.10+)
│   └── beyondtrust_secrets_static_secret (ephemeral)
├── Resources (framework v1.0+, Terraform 1.0+)
│   ├── beyondtrust_secrets_folder
│   ├── beyondtrust_secrets_static_secret (write-only)
│   ├── beyondtrust_secrets_aws_integration
│   └── beyondtrust_secrets_aws_dynamic_secret
└── Data Sources (framework v1.0+, Terraform 1.0+)
    └── beyondtrust_secrets_aws_integration
```

### Backward Compatibility Strategy for v1.0 Release

**Option A: Ephemeral-Only (Current)**
- Pros: Maximum security, follows best practices
- Cons: Requires Terraform 1.10+, smaller user base initially
- Recommendation: Document clearly in README and registry

**Option B: Dual Implementation**
- Pros: Works with Terraform 1.0+, larger user base
- Cons: More code to maintain, users on older versions get degraded security
- Implementation:
  ```text
  - Static secret resource with secret_wo (Terraform 1.0+)
  - Static secret ephemeral resource (Terraform 1.10+)
  - Data source fallback for reading secrets (Terraform 1.0+)
  ```

**Option C: Versioned Approach**
- v0.x: Support Terraform 1.0+, no ephemeral resources
- v1.0+: Require Terraform 1.10+, full ephemeral support
- Allows gradual migration path for users

## OpenTofu Considerations

### Compatibility

OpenTofu is a fork of Terraform 1.5.x and has diverged since. Key considerations:

1. **Plugin Protocol**: OpenTofu uses the same plugin protocol initially, but may diverge
2. **Ephemeral Resources**: Check if OpenTofu has equivalent or better secret handling
3. **State Format**: Ensure compatibility with OpenTofu state files
4. **Testing**: Set up CI/CD to test against both Terraform and OpenTofu

### Testing OpenTofu

```bash
# Install OpenTofu
brew install opentofu

# Test provider
tofu init
tofu plan
tofu apply
```

### Feature Parity Matrix

| Feature               | Terraform 1.10+ | Terraform 1.0-1.9 | OpenTofu      |
|-----------------------|-----------------|-------------------|---------------|
| Ephemeral Resources   | ✅ Full         | ❌ Not Available  | ❓ TBD        |
| Write-Only Attributes | ✅ Full         | ⚠️ Degraded       | ❓ TBD        |
| Standard Resources    | ✅ Full         | ✅ Full           | ❓ TBD        |
| Data Sources          | ✅ Full         | ✅ Full           | ❓ TBD        |

## Recommendation for v1.0 Release

1. **Document current requirements prominently** in README
2. **Test with Terraform 1.10+** as primary target
3. **Consider Option C (versioned approach)** for broader adoption:
   - v0.9.x: Terraform 1.0+ compatible (data sources for secrets)
   - v1.0.0: Terraform 1.10+ required (ephemeral resources)
4. **Add OpenTofu testing** to CI/CD before registry publication
5. **Provide migration guide** for users upgrading from v0.9 to v1.0

## Current Development Status

✅ **Implemented**: Ephemeral resources (Terraform 1.10+)
✅ **Implemented**: Write-only secret attributes
⏳ **TODO**: Legacy data source fallback (Terraform 1.0+)
⏳ **TODO**: OpenTofu compatibility testing
⏳ **TODO**: Version compatibility matrix in CI/CD

## References

- [Terraform 1.10 Release Notes - Ephemeral Resources](https://www.hashicorp.com/en/blog/terraform-1-10-improves-handling-secrets-in-state-with-ephemeral-values)
- [HashiCorp Write-Only Attributes](https://developer.hashicorp.com/terraform/language/manage-sensitive-data/write-only)
- [OpenTofu Documentation](https://opentofu.org/)
- [terraform-plugin-framework Changelog](https://github.com/hashicorp/terraform-plugin-framework/blob/main/CHANGELOG.md)
