# BeyondTrust Integration Module
# This module handles all BeyondTrust-specific resources
# Note: Provider is configured via dev_overrides, no required_providers needed

# ============================================================================
# Step 2: Store External ID in Workload Credentials (Write-Only Pattern)
# ============================================================================

resource "beyondtrust_workload_credentials_folder" "aws" {
  name   = "aws"
  folder = ""

  tags = {
    purpose    = "AWS Integration"
    managed_by = "terraform"
  }
}

resource "beyondtrust_workload_credentials_static_secret" "external_id" {
  name   = "integration-external-id"
  folder = beyondtrust_workload_credentials_folder.aws.path

  # Write-only: not stored in Terraform state
  secret_wo = {
    external_id = var.external_id
  }

  tags = {
    purpose     = "AWS Integration External ID"
    managed_by  = "terraform"
    environment = var.environment
  }
}

# ============================================================================
# Step 3: Read External ID Ephemerally (When Needed)
# ============================================================================

ephemeral "beyondtrust_workload_credentials_static_secret" "external_id_reader" {
  depends_on = [beyondtrust_workload_credentials_static_secret.external_id]

  name   = beyondtrust_workload_credentials_static_secret.external_id.name
  folder = beyondtrust_workload_credentials_static_secret.external_id.folder
}

# ============================================================================
# Step 6: Create BeyondTrust AWS Integration
# ============================================================================

resource "beyondtrust_workload_credentials_aws_integration" "main" {
  name = var.integration_name

  role_arn = var.beyondtrust_workload_credentials_integration_role_arn

  # External ID is stored in state (required for AWS and BeyondTrust integration)
  # The write-only secret in BeyondTrust is for retrieval via CLI/API, not for Terraform state management
  external_id = var.external_id
}

# ============================================================================
# Step 7: Create Dynamic Secrets
# ============================================================================

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "developer_access" {
  name             = "developer-readonly-creds"
  folder           = beyondtrust_workload_credentials_folder.aws.path
  integration_name = beyondtrust_workload_credentials_aws_integration.main.name

  credential_type = "assumed_role"
  role_arn        = var.developer_role_arn
  ttl             = 3600 # 1 hour

  policy_arns = var.developer_policy_arns

  aws_tags = {
    Environment = var.environment
    Team        = "Engineering"
    ManagedBy   = "Terraform"
  }
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "admin_access" {
  name             = "admin-creds"
  folder           = beyondtrust_workload_credentials_folder.aws.path
  integration_name = beyondtrust_workload_credentials_aws_integration.main.name

  credential_type = "assumed_role"
  role_arn        = var.admin_role_arn
  ttl             = 900 # 15 minutes (short TTL for admin)

  aws_tags = {
    Environment = var.environment
    AccessLevel = "High"
    ManagedBy   = "Terraform"
  }
}
