# Basic folder at root level
resource "beyondtrust_workload_credentials_folder" "production" {
  name = "production"

  tags = {
    environment = "production"
    managed_by  = "terraform"
  }
}

# Nested folder
resource "beyondtrust_workload_credentials_folder" "aws" {
  name   = "aws"
  folder = beyondtrust_workload_credentials_folder.production.path

  tags = {
    cloud       = "aws"
    environment = "production"
  }
}
