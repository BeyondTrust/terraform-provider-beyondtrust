#!/bin/bash
# Import a root-level dynamic secret
terraform import beyondtrust_workload_credentials_aws_dynamic_secret.developer production-aws:developer-readonly-creds

# Import a dynamic secret in a folder (use full path after the colon)
terraform import beyondtrust_workload_credentials_aws_dynamic_secret.developer production-aws:production/aws/developer-readonly-creds
