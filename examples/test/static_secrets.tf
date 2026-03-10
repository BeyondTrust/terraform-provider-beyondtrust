# Test static secrets with write-only pattern

# Generate random external ID
resource "random_password" "external_id" {
  length  = 32
  special = true
}

# Store in SMOP with write-only pattern
resource "beyondtrust_secrets_static_secret" "external_id" {
  name   = "aws-integration-external-id"
  folder = ""

  secret_wo = {
    external_id = random_password.external_id.result
  }

  tags = {
    purpose     = "AWS Integration External ID"
    managed_by  = "terraform"
    environment = "test"
  }
}

# Read the secret using ephemeral resource (not stored in state)
ephemeral "beyondtrust_secrets_static_secret" "external_id_reader" {
  name   = beyondtrust_secrets_static_secret.external_id.name
  folder = beyondtrust_secrets_static_secret.external_id.folder
}

# Output secret_wo_version (stored in state, safe to output)
output "external_id_version" {
  description = "Version of the external ID secret"
  value       = beyondtrust_secrets_static_secret.external_id.secret_wo_version
}

# Output the ephemeral secret value (only during apply, never in state)
output "external_id_value" {
  description = "Ephemeral external ID value (not stored in state)"
  value       = ephemeral.beyondtrust_secrets_static_secret.external_id_reader.secret["external_id"]
  sensitive   = true
}

# Output secret path
output "external_id_path" {
  description = "Path to the external ID secret in SMOP"
  value       = beyondtrust_secrets_static_secret.external_id.path
}
