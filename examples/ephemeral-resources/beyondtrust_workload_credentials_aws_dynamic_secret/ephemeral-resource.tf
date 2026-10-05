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

# Mint from that definition.
#
# The dynamic secret above must ALREADY EXIST before this runs. An ephemeral resource is
# opened while the plan is built, so a single apply that both creates the definition and
# generates from it fails: the generate call happens first and the secret is not there yet.
# depends_on does not change that — the open precedes it.
#
# So in a configuration that manages the definition, apply it before adding this block.
# Once the definition exists, every later apply is a single step.
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_aws_dynamic_secret.deploy.folder
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

# No output of any attribute here: everything an ephemeral resource exposes is itself
# ephemeral, expiration and lease_id included, and the root module cannot publish an
# ephemeral value even as sensitive. Read lease metadata from the API instead.
