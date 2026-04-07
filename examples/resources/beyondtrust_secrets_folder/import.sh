#!/bin/bash
# Import a root-level folder
terraform import beyondtrust_secrets_folder.production production

# Import a nested folder (use full path with forward slashes)
terraform import beyondtrust_secrets_folder.aws production/aws
