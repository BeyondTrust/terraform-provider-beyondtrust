# Module Outputs

output "external_id_version" {
  description = "Version of the external ID secret (safe to output)"
  value       = beyondtrust_secrets_static_secret.external_id.secret_wo_version
}

output "external_id_path" {
  description = "Path to external ID secret in Workload Credentials"
  value       = beyondtrust_secrets_static_secret.external_id.path
}

output "integration_id" {
  description = "BeyondTrust integration ID"
  value       = beyondtrust_secrets_aws_integration.main.id
}

output "developer_dynamic_secret_path" {
  description = "Path to developer dynamic secret"
  value       = beyondtrust_secrets_aws_dynamic_secret.developer_access.path
}

output "admin_dynamic_secret_path" {
  description = "Path to admin dynamic secret"
  value       = beyondtrust_secrets_aws_dynamic_secret.admin_access.path
}
