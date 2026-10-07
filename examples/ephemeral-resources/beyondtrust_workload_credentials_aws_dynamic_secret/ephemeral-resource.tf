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

# Generate credentials from that definition. The dynamic secret must already exist:
# apply it first, then add this block. depends_on does not help, because ephemeral
# resources are opened while the plan is built.
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.folder
}

# Ephemeral values can be used in provider configuration, write-only attributes and other
# ephemeral resources, but not in ordinary resource attributes.
provider "aws" {
  region     = "us-east-1"
  access_key = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.access_key_id
  secret_key = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.secret_access_key
  token      = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.deploy.session_token
}

# Ephemeral values cannot be root-module outputs, even as sensitive.
