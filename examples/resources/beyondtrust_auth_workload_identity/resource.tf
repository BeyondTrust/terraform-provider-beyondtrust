# Workload identity for a GitHub Actions pipeline.
resource "beyondtrust_auth_workload_identity" "github_ci" {
  service_name = "my-service"
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

# Workload identity granting access to a non-admin site within the org.
resource "beyondtrust_auth_workload_identity" "aws_broker" {
  service_name = "eks-workload"
  issuer_url   = "https://oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE"
  idp_category = "Custom"
  site_id      = "00000000-0000-0000-0000-000000000000" # any site in the organization

  registered_scopes = ["admin"]

  conditions = {
    sub = ["system:serviceaccount:default:my-app"]
  }
}
