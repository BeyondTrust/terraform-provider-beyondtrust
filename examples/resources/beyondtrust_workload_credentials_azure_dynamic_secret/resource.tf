# client_secret is write-only — pass it via TF_VAR_azure_client_secret or a secrets manager
variable "azure_client_secret" {
  type      = string
  sensitive = true
}

resource "beyondtrust_workload_credentials_azure_integration" "production" {
  name                  = "production-azure"
  tenant_id             = "11111111-1111-1111-1111-111111111111"
  client_id             = "22222222-2222-2222-2222-222222222222"
  client_secret         = var.azure_client_secret
  client_secret_version = 1
}

resource "beyondtrust_workload_credentials_azure_dynamic_secret" "app_creds" {
  name             = "my-app-credentials"
  folder           = "production/azure"
  integration_name = beyondtrust_workload_credentials_azure_integration.production.name

  credential_type       = "service_principal_password"
  application_object_id = "33333333-3333-3333-3333-333333333333" # Object ID of the target app (not client ID)
  ttl                   = 3600                                   # seconds; valid range: 3600–86400
}
