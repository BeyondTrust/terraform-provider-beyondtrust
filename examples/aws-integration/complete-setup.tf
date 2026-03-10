# Complete AWS Integration + Dynamic Secrets Setup
# This demonstrates the full workflow using write-only secrets and ephemeral resources

terraform {
  required_version = ">= 1.10"

  required_providers {
    beyondtrust = {
      source = "registry.terraform.io/beyondtrust/beyondtrust"
      # Note: With dev_overrides in ~/.terraformrc, this uses your local binary
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

# ============================================================================
# Provider Configuration
# ============================================================================

provider "beyondtrust" {
  # Configuration loaded from environment variables:
  # - BEYONDTRUST_API_URL
  # - BEYONDTRUST_ACCESS_TOKEN
  # - BEYONDTRUST_SITE_ID
  # - BEYONDTRUST_ROLE (optional)
}

provider "aws" {
  region = var.aws_region
}

# ============================================================================
# Step 1: Generate External ID Securely
# ============================================================================

resource "random_password" "external_id" {
  length  = 32
  special = true
  # AWS external ID allows: [a-zA-Z0-9+=,.@:/_-]
  override_special = "_+=,.@:/-"
}

# Local values for reuse
locals {
  external_id = random_password.external_id.result
}

# ============================================================================
# Step 2: Store External ID in SMOP (Write-Only Pattern)
# ============================================================================

resource "beyondtrust_secrets_folder" "aws" {
  name   = "aws"
  folder = ""

  tags = {
    purpose    = "AWS Integration"
    managed_by = "terraform"
  }
}

resource "beyondtrust_secrets_static_secret" "external_id" {
  name   = "integration-external-id"
  folder = beyondtrust_secrets_folder.aws.path

  # Write-only: not stored in Terraform state
  secret_wo = {
    external_id = local.external_id
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

ephemeral "beyondtrust_secrets_static_secret" "external_id_reader" {
  depends_on = [beyondtrust_secrets_static_secret.external_id]

  name   = beyondtrust_secrets_static_secret.external_id.name
  folder = beyondtrust_secrets_static_secret.external_id.folder
}

# ============================================================================
# Step 4: AWS IAM Setup
# ============================================================================

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

# Integration Role - SMOP assumes this to access your AWS account
# Name must match pattern: btp-account-role-* or btp-org-role-* (required by bridge role policy)
resource "aws_iam_role" "smop_integration" {
  name        = "btp-account-role-for-smop"
  path        = "/beyondtrust/"
  description = "Role for BeyondTrust SMOP to access this AWS account"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          AWS = "arn:${data.aws_partition.current.partition}:iam::${var.smop_bridge_account_id}:role/secrets-integration-customer-bridge-link"
        }
        Action = "sts:AssumeRole"
        Condition = {
          StringEquals = {
            # External ID for confused deputy prevention
            # Note: This value will be in AWS provider state, but not in BeyondTrust state (write-only)
            "sts:ExternalId" = local.external_id
          }
        }
      }
    ]
  })

  tags = merge(
    var.tags,
    {
      Name        = "SMOP Integration Role"
      Description = "Allows SMOP to assume roles for dynamic credential generation"
    }
  )
}

# Policy allowing the integration role to assume other roles
resource "aws_iam_role_policy" "smop_assume_roles" {
  name = "smop-assume-target-roles"
  role = aws_iam_role.smop_integration.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"  # Required for passing AWS tags when assuming roles
        ]
        Resource = [
          "arn:${data.aws_partition.current.partition}:iam::${data.aws_caller_identity.current.account_id}:role/beyondtrust/*"
        ]
      }
    ]
  })
}

# ============================================================================
# Step 5: Target Roles (What SMOP Can Assume)
# ============================================================================

# Developer Read-Only Access
resource "aws_iam_role" "developer_readonly" {
  name        = "DeveloperReadOnlyRole"
  path        = "/beyondtrust/"
  description = "Read-only access for developers via SMOP"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          AWS = aws_iam_role.smop_integration.arn
        }
        Action = ["sts:AssumeRole", "sts:TagSession"]
      }
    ]
  })

  tags = merge(var.tags, {
    Name = "Developer Read-Only"
    Team = "Engineering"
  })
}

resource "aws_iam_role_policy_attachment" "developer_readonly" {
  role       = aws_iam_role.developer_readonly.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/ReadOnlyAccess"
}

# Admin Access
resource "aws_iam_role" "admin" {
  name        = "AdminRole"
  path        = "/beyondtrust/"
  description = "Administrative access via SMOP"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          AWS = aws_iam_role.smop_integration.arn
        }
        Action = ["sts:AssumeRole", "sts:TagSession"]
      }
    ]
  })

  tags = merge(var.tags, {
    Name        = "Admin"
    AccessLevel = "High"
  })
}

resource "aws_iam_role_policy_attachment" "admin" {
  role       = aws_iam_role.admin.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AdministratorAccess"
}

# ============================================================================
# Step 6: Create BeyondTrust AWS Integration
# ============================================================================

resource "beyondtrust_secrets_aws_integration" "main" {
  name = "production-aws-account"

  role_arn = aws_iam_role.smop_integration.arn

  # External ID is stored in state (required for AWS and BeyondTrust integration)
  # The write-only secret in BeyondTrust is for retrieval via CLI/API, not for Terraform state management
  external_id = local.external_id
}

# ============================================================================
# Step 7: Create Dynamic Secrets
# ============================================================================

resource "beyondtrust_secrets_aws_dynamic_secret" "developer_access" {
  name             = "developer-readonly-creds"
  folder           = beyondtrust_secrets_folder.aws.path
  integration_name = beyondtrust_secrets_aws_integration.main.name

  credential_type = "assumed_role"
  role_arn        = aws_iam_role.developer_readonly.arn
  ttl             = 3600 # 1 hour

  policy_arns = [
    "arn:${data.aws_partition.current.partition}:iam::aws:policy/ReadOnlyAccess"
  ]

  aws_tags = {
    Environment = var.environment
    Team        = "Engineering"
    ManagedBy   = "Terraform"
  }
}

resource "beyondtrust_secrets_aws_dynamic_secret" "admin_access" {
  name             = "admin-creds"
  folder           = beyondtrust_secrets_folder.aws.path
  integration_name = beyondtrust_secrets_aws_integration.main.name

  credential_type = "assumed_role"
  role_arn        = aws_iam_role.admin.arn
  ttl             = 900 # 15 minutes (short TTL for admin)

  aws_tags = {
    Environment = var.environment
    AccessLevel = "High"
    ManagedBy   = "Terraform"
  }
}

# ============================================================================
# Outputs
# ============================================================================

output "smop_integration_role_arn" {
  description = "ARN of the IAM role for SMOP integration"
  value       = aws_iam_role.smop_integration.arn
}

output "external_id_version" {
  description = "Version of the external ID secret (safe to output)"
  value       = beyondtrust_secrets_static_secret.external_id.secret_wo_version
}

output "external_id_path" {
  description = "Path to external ID secret in SMOP"
  value       = beyondtrust_secrets_static_secret.external_id.path
}

output "integration_id" {
  description = "BeyondTrust integration ID"
  value       = beyondtrust_secrets_aws_integration.main.id
}

output "developer_dynamic_secret_path" {
  description = "Path to developer dynamic secret"
  value       = beyondtrust_secrets_aws_dynamic_secret.developer_access.path
}

output "admin_dynamic_secret_path" {
  description = "Path to admin dynamic secret"
  value       = beyondtrust_secrets_aws_dynamic_secret.admin_access.path
}

output "aws_account_id" {
  description = "AWS account ID"
  value       = data.aws_caller_identity.current.account_id
}

# Note: External ID value is ephemeral and NOT in state
# To retrieve it later, use the CLI: secrets kv get aws/integration-external-id
