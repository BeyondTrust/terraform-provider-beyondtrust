terraform {
  required_providers {
    beyondtrust = {
      source = "beyondtrust/beyondtrust"
    }
  }
}

provider "beyondtrust" {
  # Configuration loaded from environment variables:
  # - BEYONDTRUST_API_URL
  # - BEYONDTRUST_ACCESS_TOKEN
  # - BEYONDTRUST_SITE_ID
}

# Test folder resource
resource "beyondtrust_workload_credentials_folder" "test" {
  name   = "terraform-test"
  folder = ""

  tags = {
    managed_by  = "terraform"
    environment = "test"
  }
}

# Test nested folder
resource "beyondtrust_workload_credentials_folder" "test_nested" {
  name   = "nested"
  folder = beyondtrust_workload_credentials_folder.test.path

  tags = {
    managed_by = "terraform"
    parent     = "test"
  }
}

output "test_folder_id" {
  value = beyondtrust_workload_credentials_folder.test.id
}

output "test_folder_path" {
  value = beyondtrust_workload_credentials_folder.test.path
}

output "nested_folder_path" {
  value = beyondtrust_workload_credentials_folder.test_nested.path
}
