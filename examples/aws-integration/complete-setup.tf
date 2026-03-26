# Complete AWS Integration + Dynamic Secrets Setup
# This demonstrates the full workflow using write-only secrets and ephemeral resources

terraform {
  required_version = ">= 1.10"

  required_providers {
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
# BeyondTrust Integration Module
# ============================================================================
# WORKFLOW:
# 1. Init AWS providers:     terraform init -var="skip_beyondtrust=true"
# 2. Plan/Apply with local:  terraform plan (skip_beyondtrust defaults to false)

module "beyondtrust" {
  count  = var.skip_beyondtrust ? 0 : 1
  source = "./modules/beyondtrust-integration"

  external_id                = local.external_id
  integration_name           = "production-aws-account"
  smop_integration_role_arn  = aws_iam_role.smop_integration.arn
  developer_role_arn         = aws_iam_role.developer_readonly.arn
  admin_role_arn             = aws_iam_role.admin.arn
  developer_policy_arns      = ["arn:${data.aws_partition.current.partition}:iam::aws:policy/ReadOnlyAccess"]
  environment                = var.environment
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
  value       = try(module.beyondtrust[0].external_id_version, null)
}

output "external_id_path" {
  description = "Path to external ID secret in SMOP"
  value       = try(module.beyondtrust[0].external_id_path, null)
}

output "integration_id" {
  description = "BeyondTrust integration ID"
  value       = try(module.beyondtrust[0].integration_id, null)
}

output "developer_dynamic_secret_path" {
  description = "Path to developer dynamic secret"
  value       = try(module.beyondtrust[0].developer_dynamic_secret_path, null)
}

output "admin_dynamic_secret_path" {
  description = "Path to admin dynamic secret"
  value       = try(module.beyondtrust[0].admin_dynamic_secret_path, null)
}

output "aws_account_id" {
  description = "AWS account ID"
  value       = data.aws_caller_identity.current.account_id
}

# Note: External ID value is ephemeral and NOT in state
# To retrieve it later, use the CLI: secrets kv get aws/integration-external-id
