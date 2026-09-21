# Policies are managed against your organization's admin site, while each policy grants access on
# a target site. An access token reaches one site, so policies need their own provider block.
provider "beyondtrust" {
  alias        = "platform"
  site_id      = var.admin_site_id
  access_token = var.admin_access_token
}

# Let one person read one secret.
resource "beyondtrust_iam_policy" "app_team_db_password" {
  provider = beyondtrust.platform

  name = "app-team-read-db-password"

  cedar = <<-EOT
    @siteId("${var.site_id}")
    permit(
      principal == Pathfinder::User::Email::"app-team@example.com",
      action == WorkloadCredentials::Action::"ReadSecret",
      resource == WorkloadCredentials::Secret::"/production/database/password"
    );
  EOT
}

# Give a group full control of a folder and everything inside it.
resource "beyondtrust_iam_policy" "platform_admins_production" {
  provider = beyondtrust.platform

  name = "platform-admins-production"

  cedar = <<-EOT
    @siteId("${var.site_id}")
    permit(
      principal == Pathfinder::Group::"platform-admins",
      action == WorkloadCredentials::Action::"Owner",
      resource == WorkloadCredentials::Folder::"/production"
    );
  EOT
}

# Grant several actions at once, here to a workload rather than a person.
resource "beyondtrust_iam_policy" "ci_rotates_api_key" {
  provider = beyondtrust.platform

  name = "ci-rotate-api-key"

  cedar = <<-EOT
    @siteId("${var.site_id}")
    permit(
      principal == Pathfinder::Workload::Id::"${var.ci_workload_id}",
      action in [
        WorkloadCredentials::Action::"ReadSecret",
        WorkloadCredentials::Action::"UpdateSecret"
      ],
      resource == WorkloadCredentials::Secret::"/production/api-key"
    );
  EOT
}

# A policy may name a secret or folder that does not exist yet. It stays valid and starts working
# on its own once the resource is created, so you do not have to order the two.
resource "beyondtrust_iam_policy" "auditors_next_quarter" {
  provider = beyondtrust.platform

  name = "auditors-next-quarter"

  cedar = <<-EOT
    @siteId("${var.site_id}")
    permit(
      principal == Pathfinder::Role::"auditor",
      action == WorkloadCredentials::Action::"ReadFolderMetadata",
      resource == WorkloadCredentials::Folder::"/2027-q1"
    );
  EOT

  # Applying waits for the grant to take effect. Raise this if your environment is slow.
  timeouts {
    create = "10m"
  }
}

# If you would rather not write Cedar by hand, the Common Fate Cedar provider renders it for you.
# Use one policy block per data source: each beyondtrust_iam_policy holds a single statement.
data "cedar_policyset" "reporting" {
  policy {
    effect = "permit"

    annotation {
      name  = "siteId"
      value = var.site_id
    }

    principal = {
      type = "Pathfinder::User::Email"
      id   = "reporting@example.com"
    }
    action = {
      type = "WorkloadCredentials::Action"
      id   = "ReadSecret"
    }
    resource = {
      type = "WorkloadCredentials::Secret"
      id   = "/production/reporting/token"
    }
  }
}

resource "beyondtrust_iam_policy" "reporting" {
  provider = beyondtrust.platform

  name  = "reporting-read-token"
  cedar = data.cedar_policyset.reporting.text
}
