# Root Terragrunt Configuration
# Shared settings for all modules

locals {
  # AWS region for resources
  aws_region = "us-east-1"

  # Common tags
  tags = {
    ManagedBy   = "Terraform"
    Environment = "Development"
    Purpose     = "SMOP-Integration-Example"
  }

  # BeyondTrust SMOP bridge account ID
  smop_bridge_account_id = "109876543210"  # Sandbox/dev bridge account
}

# This is left intentionally minimal - provider configs are generated per-module
# to allow different provider sources (registry vs local)
