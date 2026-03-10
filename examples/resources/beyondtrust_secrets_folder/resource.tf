# Basic folder at root level
resource "beyondtrust_secrets_folder" "production" {
  name = "production"

  tags = {
    environment = "production"
    managed_by  = "terraform"
  }
}

# Nested folder
resource "beyondtrust_secrets_folder" "aws" {
  name   = "aws"
  folder = beyondtrust_secrets_folder.production.path

  tags = {
    cloud       = "aws"
    environment = "production"
  }
}
