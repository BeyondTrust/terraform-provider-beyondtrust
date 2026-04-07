#!/bin/bash
# Import a root-level dynamic secret
terraform import beyondtrust_secrets_aws_dynamic_secret.developer developer-readonly-creds

# Import a dynamic secret in a folder (use full path)
terraform import beyondtrust_secrets_aws_dynamic_secret.developer production/aws/developer-readonly-creds
