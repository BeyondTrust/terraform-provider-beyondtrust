# client_secret is write-only — pass it via TF_VAR_azure_client_secret or a secrets manager
variable "azure_client_secret" {
  type      = string
  sensitive = true
}

resource "beyondtrust_workload_credentials_azure_integration" "production" {
  name      = "production-azure"
  tenant_id = "00000000-0000-0000-0000-000000000000"
  client_id = "11111111-1111-1111-1111-111111111111"

  client_secret         = var.azure_client_secret
  client_secret_version = 1 # increment to rotate the secret
}
