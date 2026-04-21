# AWS Integration for dynamic credential generation
resource "beyondtrust_secrets_aws_integration" "production" {
  name        = "production-aws-account"
  role_arn    = "arn:aws:iam::123456789012:role/beyondtrust/btp-account-role-for-workload-credentials"
  external_id = var.external_id
}

# Best practice: Generate external ID securely and store as write-only secret
resource "random_password" "external_id" {
  length  = 32
  special = true
  # AWS external ID allows: [a-zA-Z0-9+=,.@:/_-]
  override_special = "_+=,.@:/-"
}

resource "beyondtrust_secrets_static_secret" "external_id" {
  name   = "aws-integration-external-id"
  folder = "production"

  secret_wo = {
    external_id = random_password.external_id.result
  }
}

resource "beyondtrust_secrets_aws_integration" "production_secure" {
  name        = "production-aws-secure"
  role_arn    = "arn:aws:iam::123456789012:role/beyondtrust/btp-account-role-for-workload-credentials"
  external_id = random_password.external_id.result
}
