#!/bin/bash
# Import an AWS integration by name
terraform import beyondtrust_workload_credentials_aws_integration.production production-aws-account

# Note: After import, you must provide the external_id in your configuration.
# The external ID is not returned by the API for security reasons.
