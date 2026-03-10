# Read an existing AWS integration
data "beyondtrust_secrets_aws_integration" "existing" {
  name = "production-aws-account"
}

# Use the integration details in a dynamic secret
resource "beyondtrust_secrets_aws_dynamic_secret" "from_existing" {
  name             = "new-dynamic-secret"
  folder           = "production"
  integration_name = data.beyondtrust_secrets_aws_integration.existing.name

  credential_type = "assumed_role"
  role_arn        = "arn:aws:iam::123456789012:role/MyRole"
  ttl             = 3600
}

# Output integration details
output "integration_id" {
  value = data.beyondtrust_secrets_aws_integration.existing.id
}

output "integration_role_arn" {
  value = data.beyondtrust_secrets_aws_integration.existing.role_arn
}
