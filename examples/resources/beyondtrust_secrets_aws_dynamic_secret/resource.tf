# AWS Dynamic Secret with assumed role credentials
resource "beyondtrust_secrets_aws_dynamic_secret" "developer_readonly" {
  name             = "developer-readonly-creds"
  folder           = "production/aws"
  integration_name = beyondtrust_secrets_aws_integration.production.name

  credential_type = "assumed_role"
  role_arn        = "arn:aws:iam::123456789012:role/DeveloperReadOnlyRole"
  ttl             = 3600 # 1 hour

  # Attach AWS managed policies
  policy_arns = [
    "arn:aws:iam::aws:policy/ReadOnlyAccess"
  ]

  # Add session tags for CloudTrail attribution
  aws_tags = {
    Environment = "production"
    Team        = "engineering"
    ManagedBy   = "terraform"
  }
}

# Dynamic secret with inline IAM policy
resource "beyondtrust_secrets_aws_dynamic_secret" "s3_specific" {
  name             = "s3-data-bucket-access"
  folder           = "production/aws"
  integration_name = beyondtrust_secrets_aws_integration.production.name

  credential_type = "assumed_role"
  role_arn        = "arn:aws:iam::123456789012:role/S3DataBucketRole"
  ttl             = 7200 # 2 hours

  # Inline policy for specific S3 bucket access
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::my-data-bucket",
          "arn:aws:s3:::my-data-bucket/*"
        ]
      }
    ]
  })

  aws_tags = {
    Application = "data-pipeline"
    Environment = "production"
  }
}

# Admin access with short TTL
resource "beyondtrust_secrets_aws_dynamic_secret" "admin" {
  name             = "admin-creds"
  folder           = "production/aws"
  integration_name = beyondtrust_secrets_aws_integration.production.name

  credential_type = "assumed_role"
  role_arn        = "arn:aws:iam::123456789012:role/AdminRole"
  ttl             = 900 # 15 minutes (short TTL for admin access)

  policy_arns = [
    "arn:aws:iam::aws:policy/AdministratorAccess"
  ]

  aws_tags = {
    AccessLevel = "admin"
    Environment = "production"
  }
}
