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

# Mint a service principal password from that definition.
#
# depends_on is required rather than decorative. Without it the ephemeral resource is
# opened during the plan walk, before the dynamic secret exists, and the plan fails.
# Referencing .name is not sufficient on its own: that value comes from configuration
# and is therefore already known at plan time, so Terraform has no reason to wait.
ephemeral "beyondtrust_workload_credentials_azure_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.folder

  depends_on = [beyondtrust_workload_credentials_azure_dynamic_secret.deploy]
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

  depends_on = [beyondtrust_workload_credentials_azure_dynamic_secret.deploy]
}
