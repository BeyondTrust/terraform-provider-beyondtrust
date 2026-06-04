terraform {
  required_version = ">= 1.11.0"

  required_providers {
    beyondtrust = {
      source  = "beyondtrust/beyondtrust"
      version = "~> 1.0"
    }
  }
}

# Credentials are read from environment variables:
#   BEYONDTRUST_API_URL
#   BEYONDTRUST_ACCESS_TOKEN
#   BEYONDTRUST_SITE_ID
provider "beyondtrust" {}

# Create a folder to organize secrets
resource "beyondtrust_workload_credentials_folder" "example" {
  name = "production"

  tags = {
    env     = "production"
    team    = "platform"
    managed = "terraform"
  }
}
