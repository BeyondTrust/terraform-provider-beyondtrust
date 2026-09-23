#!/bin/bash
# Import an existing policy by name, to manage a policy that was created in the console or by
# some other tool.
terraform import beyondtrust_iam_policy.app_team_db_password app-team-read-db-password

# Use this if an apply fails with "IAM Policy Already Exists". Policy names are unique across your
# whole organization, so the name may belong to a policy this configuration did not create.

# After importing, make the cedar argument match the policy text the service returns. Anything
# that differs is applied the next time you run Terraform.
