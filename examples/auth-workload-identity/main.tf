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
#
# Workload identities are managed against the organization's admin site, so
# BEYONDTRUST_SITE_ID / BEYONDTRUST_ACCESS_TOKEN must be the admin-site credentials.
provider "beyondtrust" {}

# A workload identity that lets a GitHub Actions workflow federate into BeyondTrust.
resource "beyondtrust_auth_workload_identity" "github_ci" {
  service_name = "example-ci"
  issuer_url   = "https://token.actions.githubusercontent.com"
  idp_category = "GitHubActions"
  description  = "CI pipeline for myorg/myrepo"

  registered_scopes = ["admin"]

  conditions = {
    sub              = ["repo:myorg/myrepo:ref:refs/heads/main"]
    repository       = ["myorg/myrepo"]
    repository_owner = ["myorg"]
  }
}

output "identity_id" {
  description = "The id assigned to the workload identity."
  value       = beyondtrust_auth_workload_identity.github_ci.id
}

output "expected_aud" {
  description = "The expected audience for token exchange."
  value       = beyondtrust_auth_workload_identity.github_ci.expected_aud
}
