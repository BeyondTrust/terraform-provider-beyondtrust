#!/bin/bash
# Import a root-level dynamic secret
terraform import beyondtrust_workload_credentials_azure_dynamic_secret.app_creds production-azure:my-app-credentials

# Import a dynamic secret in a folder (use full path after the colon)
terraform import beyondtrust_workload_credentials_azure_dynamic_secret.app_creds production-azure:production/azure/my-app-credentials
