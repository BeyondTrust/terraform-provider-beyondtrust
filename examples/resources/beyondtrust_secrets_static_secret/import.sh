#!/bin/bash
# Import a root-level secret
terraform import beyondtrust_secrets_static_secret.api_key api-key

# Import a secret in a folder (use full path)
terraform import beyondtrust_secrets_static_secret.api_key production/api-key

# Note: After import, you must provide the secret_wo attribute in your configuration.
# The secret value is not retrieved during import for security reasons.
