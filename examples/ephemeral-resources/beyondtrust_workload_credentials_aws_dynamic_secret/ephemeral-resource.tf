# The dynamic secret defines what to mint and for how long. It is a long-lived
# definition, not a credential.
resource "beyondtrust_workload_credentials_aws_dynamic_secret" "deploy" {
  name             = "ci-deployer"
  folder           = "production/aws"
  integration_name = "production-aws-account"
  credential_type  = "assumed_role"
  role_arn         = "arn:aws:iam::123456789012:role/Deployer"
  ttl              = 3600
}

# Mint a session from that definition.
#
# depends_on is required rather than decorative. Without it the ephemeral resource is
# opened during the plan walk, before the dynamic secret exists, and the plan fails.
# Referencing .name is not sufficient on its own: that value comes from configuration
# and is therefore already known at plan time, so Terraform has no reason to wait.
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.folder

  depends_on = [beyondtrust_workload_credentials_aws_dynamic_secret.deploy]
}

# Ephemeral values may flow into provider configuration, write-only attributes, other
# ephemeral resources, and locals or outputs marked ephemeral. Terraform rejects them
# anywhere else, including ordinary resource attributes.
provider "aws" {
  region     = "us-east-1"
  access_key = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.access_key_id
  secret_key = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.secret_access_key
  token      = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.session_token
}

# Metadata is safe to output. The credentials themselves are not.
output "session_expires_at" {
  value = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.expiration
}
