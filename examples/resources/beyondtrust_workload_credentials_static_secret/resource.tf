# Static secret with write-only values
resource "beyondtrust_workload_credentials_static_secret" "api_key" {
  name   = "api-key"
  folder = "production"

  # Secret values are write-only and never stored in Terraform state
  secret_wo = {
    api_key = "sk-1234567890abcdef"
    api_url = "https://api.example.com"
  }

  # Increment secret_wo_version to rotate the secret value.
  # Required because write-only values cannot be diffed automatically.
  secret_wo_version = 1

  tags = {
    application = "backend-service"
    environment = "production"
  }
}

# Read the secret value using ephemeral resource
ephemeral "beyondtrust_workload_credentials_static_secret" "api_key_reader" {
  name   = beyondtrust_workload_credentials_static_secret.api_key.name
  folder = beyondtrust_workload_credentials_static_secret.api_key.folder
}

# Use the secret in another resource (ephemeral values available during plan/apply)
resource "kubernetes_secret" "api_credentials" {
  metadata {
    name = "api-credentials"
  }

  data = {
    api_key = ephemeral.beyondtrust_workload_credentials_static_secret.api_key_reader.secret["api_key"]
    api_url = ephemeral.beyondtrust_workload_credentials_static_secret.api_key_reader.secret["api_url"]
  }
}
