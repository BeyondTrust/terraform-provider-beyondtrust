# The dynamic secret defines what to mint and for how long. It is a long-lived
# definition, not a credential.
resource "beyondtrust_workload_credentials_azure_dynamic_secret" "deploy" {
  name                  = "ci-deployer"
  folder                = "production/azure"
  integration_name      = "corp-entra-tenant"
  credential_type       = "service_principal_password"
  application_object_id = "22222222-2222-2222-2222-222222222222"
  ttl                   = 3600
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
ephemeral "beyondtrust_workload_credentials_azure_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.folder
}

# Ephemeral values may flow into provider configuration, write-only attributes, other
# ephemeral resources, and locals or outputs marked ephemeral. Terraform rejects them
# anywhere else, including ordinary resource attributes.
provider "azurerm" {
  features {}

  client_id     = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.client_id
  client_secret = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.client_secret
  tenant_id     = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.tenant_id
}

# By default the generated password is deleted from the target application as soon as
# Terraform finishes with it. Disable that only when the credential is handed to a
# system that must keep using it after the apply completes — it then stays valid until
# the dynamic secret's TTL elapses.
ephemeral "beyondtrust_workload_credentials_azure_dynamic_secret" "handoff" {
  name            = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.name
  folder          = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.folder
  revoke_on_close = false
}
