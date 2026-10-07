data "beyondtrust_workload_credentials_azure_integration" "production" {
  name = "production-azure"
}

output "azure_integration_id" {
  value = data.beyondtrust_workload_credentials_azure_integration.production.id
}

output "azure_tenant_id" {
  value = data.beyondtrust_workload_credentials_azure_integration.production.tenant_id
}
