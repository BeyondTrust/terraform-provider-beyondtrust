# Read a secret value ephemerally (not stored in state)
ephemeral "beyondtrust_secrets_static_secret" "database_password" {
  name   = "db-master-password"
  folder = "production/database"
}

# Use the ephemeral secret in a resource
resource "kubernetes_secret" "db_credentials" {
  metadata {
    name      = "database-credentials"
    namespace = "production"
  }

  data = {
    username = "admin"
    password = ephemeral.beyondtrust_secrets_static_secret.database_password.secret["password"]
    host     = ephemeral.beyondtrust_secrets_static_secret.database_password.secret["host"]
  }
}

# Read a specific version of a secret
ephemeral "beyondtrust_secrets_static_secret" "api_key_v2" {
  name    = "api-key"
  folder  = "production"
  version = 2
}

# Output metadata (safe - not the secret value)
output "secret_path" {
  value = ephemeral.beyondtrust_secrets_static_secret.database_password.path
}

output "secret_version" {
  value = ephemeral.beyondtrust_secrets_static_secret.database_password.version
}
