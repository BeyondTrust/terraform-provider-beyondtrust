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

# Generate credentials from that definition. The dynamic secret must already exist:
# apply it first, then add this block. depends_on does not help, because ephemeral
# resources are opened while the plan is built.
ephemeral "beyondtrust_workload_credentials_azure_dynamic_secret" "deploy" {
  name   = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.name
  folder = beyondtrust_workload_credentials_azure_dynamic_secret.deploy.folder
}

# Ephemeral values can be used in provider configuration, write-only attributes and other
# ephemeral resources, but not in ordinary resource attributes.
provider "azurerm" {
  features {}

  client_id     = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.client_id
  client_secret = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.client_secret
  tenant_id     = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.deploy.tenant_id
}

# revoke_on_close defaults to true, deleting the password once Terraform is done with it.
# Set it to false only when another system must keep using the password after the apply.
