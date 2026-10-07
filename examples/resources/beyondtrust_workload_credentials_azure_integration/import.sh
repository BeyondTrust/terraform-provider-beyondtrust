#!/bin/bash
# Import an Azure integration by name
terraform import beyondtrust_workload_credentials_azure_integration.production production-azure

# Note: After import, you must provide client_secret and client_secret_version in your configuration.
# The client secret is never returned by the API.
