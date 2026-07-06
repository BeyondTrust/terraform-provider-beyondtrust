# Azure Integration Testing Guide

This guide explains how to set up Azure resources and run Azure integration acceptance tests.

Unlike the AWS tests (which auto-create and clean up IAM roles), Azure tests require **manually pre-created** resources in your Azure AD tenant. This is a one-time setup.

## Overview: Two Distinct Azure AD Objects

The Azure integration requires two separate Azure AD objects with different roles:

| Object | Purpose | Env var used |
| --- | --- | --- |
| **Integration service principal** | What BeyondTrust authenticates as when connecting to Azure | `TENANT_ID`, `CLIENT_ID`, `CLIENT_SECRET` |
| **Target app registration** | The app whose passwords BeyondTrust generates | `APPLICATION_OBJECT_ID` |

The integration service principal must have permission to generate passwords on the target app.

## Prerequisites

- [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) installed and authenticated
- An Azure AD tenant where you have permissions to create app registrations and grant API permissions
- BeyondTrust Workload Credentials instance with API access

## Step 1: Create the Integration Service Principal

This is the identity BeyondTrust uses to authenticate to your Azure tenant.

```bash
# Log in to Azure
az login

# Create the app registration for the integration identity
az ad app create --display-name "beyondtrust-tf-test-integration"

# Note the appId (= client_id) from the output
export INTEGRATION_APP_ID=$(az ad app list --display-name "beyondtrust-tf-test-integration" --query '[0].appId' -o tsv)

# Create a service principal for the app
az ad sp create --id "$INTEGRATION_APP_ID"

# Create a client secret (note the password from the output — shown only once)
az ad app credential reset --id "$INTEGRATION_APP_ID" --display-name "tf-test-secret"
```

Set the env vars from the output:

```bash
export BEYONDTRUST_TEST_AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)
export BEYONDTRUST_TEST_AZURE_CLIENT_ID="$INTEGRATION_APP_ID"
export BEYONDTRUST_TEST_AZURE_CLIENT_SECRET="<password from credential reset output>"
```

## Step 2: Create the Target App Registration

This is the app whose passwords BeyondTrust will generate for dynamic secrets.

```bash
# Create the target app registration
az ad app create --display-name "beyondtrust-tf-test-target"

# Get the Object ID — this is APPLICATION_OBJECT_ID (NOT the appId / client ID)
export TARGET_OBJECT_ID=$(az ad app list --display-name "beyondtrust-tf-test-target" --query '[0].id' -o tsv)
```

> **Important**: `APPLICATION_OBJECT_ID` is the **Object ID** (`id` field), not the Application (client) ID (`appId` field). These look similar (both are UUIDs) but are different values. In the Azure Portal, find it under **App registrations** → select the app → **Overview** → **Object ID**.

```bash
export BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID="$TARGET_OBJECT_ID"
```

## Step 3: Grant Permissions

The integration service principal needs permission to manage passwords on the target app.

### Option A: Ownership (Recommended — Least Privilege)

Make the integration service principal an owner of the target app. Owners can add/remove credentials without needing tenant-wide API permissions.

```bash
# Get the Object ID of the integration service principal
INTEGRATION_SP_OBJECT_ID=$(az ad sp show --id "$INTEGRATION_APP_ID" --query id -o tsv)

# Add it as an owner of the target app
az ad app owner add --id "$TARGET_OBJECT_ID" --owner-object-id "$INTEGRATION_SP_OBJECT_ID"
```

### Option B: Application.ReadWrite.All (Broader Permission)

If ownership isn't sufficient for your scenario, grant the `Application.ReadWrite.All` Microsoft Graph permission:

```bash
# Get the Microsoft Graph service principal ID
GRAPH_SP_ID=$(az ad sp show --id 00000003-0000-0000-c000-000000000000 --query id -o tsv)

# Application.ReadWrite.All app role ID (well-known ID)
APP_RW_ALL="1bfefb4e-e0b5-418b-a88f-73c46d2cc8e9"

# Assign the app role
az rest --method POST \
  --uri "https://graph.microsoft.com/v1.0/servicePrincipals/$INTEGRATION_SP_OBJECT_ID/appRoleAssignments" \
  --body "{
    \"principalId\": \"$INTEGRATION_SP_OBJECT_ID\",
    \"resourceId\": \"$GRAPH_SP_ID\",
    \"appRoleId\": \"$APP_RW_ALL\"
  }"
```

> **Note**: App role assignments (Option B) require admin consent and may take a few minutes to propagate.

## Step 4: Set Environment Variables

Add all four Azure env vars alongside the base BeyondTrust credentials:

```bash
export BEYONDTRUST_ACCESS_TOKEN="your-access-token"
export BEYONDTRUST_SITE_ID="your-site-uuid"

export BEYONDTRUST_TEST_AZURE_TENANT_ID="your-azure-tenant-uuid"
export BEYONDTRUST_TEST_AZURE_CLIENT_ID="integration-service-principal-client-id"
export BEYONDTRUST_TEST_AZURE_CLIENT_SECRET="integration-service-principal-secret"
export BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID="target-app-object-id"
```

Use [direnv](https://direnv.net/) to persist these: copy `.envrc.example` to `.envrc`, fill in the values, and run `direnv allow`.

## Running the Tests

```bash
# Run all Azure acceptance tests
TF_ACC=1 go test -tags=acceptance -v -timeout=30m -run TestAccAzure \
  ./workload_credentials/resources/ \
  ./workload_credentials/datasources/

# Run only Azure integration resource tests
TF_ACC=1 go test -tags=acceptance -v -timeout=30m -run TestAccAzureIntegrationResource \
  ./workload_credentials/resources/

# Run only Azure dynamic secret resource tests
TF_ACC=1 go test -tags=acceptance -v -timeout=30m -run TestAccAzureDynamicSecretResource \
  ./workload_credentials/resources/

# Run only Azure integration data source test
TF_ACC=1 go test -tags=acceptance -v -timeout=30m -run TestAccAzureIntegrationDataSource \
  ./workload_credentials/datasources/
```

When any of the four `BEYONDTRUST_TEST_AZURE_*` env vars are missing, the tests skip automatically via `acctest.PreCheckAzure(t)` — no test failure.

## Troubleshooting

### `azure_integration_test_failed`

BeyondTrust validates Azure credentials when creating an integration. This can fail transiently due to Azure AD propagation delays (newly created credentials can take 30–60 seconds to become usable globally).

The provider **automatically retries up to 3 times** (5s, then 10s backoff) before surfacing this as an error. If you still see it:

- Wait 60 seconds after creating the client secret and retry
- Verify `CLIENT_ID` and `TENANT_ID` are correct
- Check the client secret hasn't expired
- Ensure the service principal exists: `az ad sp show --id "$BEYONDTRUST_TEST_AZURE_CLIENT_ID"`

### Wrong Object ID for `APPLICATION_OBJECT_ID`

The most common mistake: using the Application (client) ID instead of the Object ID.

```bash
# Correct: Object ID (the 'id' field, NOT 'appId')
az ad app show --id "your-app-client-id" --query '{objectId:id, clientId:appId}' -o json
```

Both are UUIDs — double-check that `APPLICATION_OBJECT_ID` matches the `id` field, not `appId`.

### Permission Denied When Generating Passwords

If dynamic secret creation fails with a permissions error:

1. Confirm the integration SP is an owner of the target app:
   ```bash
   az ad app owner list --id "$TARGET_OBJECT_ID" --query '[].displayName'
   ```
2. If using Option B (App role assignment), confirm admin consent was granted:
   ```bash
   az ad sp show --id "$INTEGRATION_APP_ID" --query appRoles
   ```
3. Allow up to 5 minutes for permission changes to propagate.

### Tests Skip Unexpectedly

All four Azure env vars must be set. Check which is missing:

```bash
echo "TENANT_ID: ${BEYONDTRUST_TEST_AZURE_TENANT_ID:-(not set)}"
echo "CLIENT_ID: ${BEYONDTRUST_TEST_AZURE_CLIENT_ID:-(not set)}"
echo "CLIENT_SECRET: ${BEYONDTRUST_TEST_AZURE_CLIENT_SECRET:+(set)}"
echo "APP_OBJECT_ID: ${BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID:-(not set)}"
```

## CI/CD Integration

### GitHub Actions

Add the four Azure secrets to your GitHub repository (**Settings** → **Secrets and variables** → **Actions**), then reference them in the workflow:

```yaml
- name: Run Azure Acceptance Tests
  env:
    TF_ACC: "1"
    BEYONDTRUST_API_URL: ${{ secrets.BEYONDTRUST_API_URL }}
    BEYONDTRUST_ACCESS_TOKEN: ${{ secrets.BEYONDTRUST_ACCESS_TOKEN }}
    BEYONDTRUST_SITE_ID: ${{ secrets.BEYONDTRUST_SITE_ID }}
    BEYONDTRUST_TEST_AZURE_TENANT_ID: ${{ secrets.BEYONDTRUST_TEST_AZURE_TENANT_ID }}
    BEYONDTRUST_TEST_AZURE_CLIENT_ID: ${{ secrets.BEYONDTRUST_TEST_AZURE_CLIENT_ID }}
    BEYONDTRUST_TEST_AZURE_CLIENT_SECRET: ${{ secrets.BEYONDTRUST_TEST_AZURE_CLIENT_SECRET }}
    BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID: ${{ secrets.BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID }}
  run: |
    go test -tags=acceptance -v -timeout=30m -run TestAccAzure \
      ./workload_credentials/resources/ \
      ./workload_credentials/datasources/
```

The tests skip automatically when secrets are absent (e.g., on fork PRs), so no conditional logic is needed.

### Rotating the Test Client Secret

Client secrets expire. When rotating:

1. Create a new secret: `az ad app credential reset --id "$INTEGRATION_APP_ID" --display-name "tf-test-secret-v2"`
2. Update `BEYONDTRUST_TEST_AZURE_CLIENT_SECRET` in your secrets store / `.envrc`
3. Delete the old secret from the Azure Portal or via `az ad app credential delete`

## Cleanup

The test Azure AD objects persist between test runs (they are not auto-deleted). The tests create and delete BeyondTrust integration and dynamic secret resources, but the Azure AD service principal and app registration remain.

To remove them after you're done:

```bash
az ad app delete --id "$INTEGRATION_APP_ID"
az ad app delete --id "$TARGET_OBJECT_ID"
```

## Environment Variables Reference

| Variable | Required | Description |
| --- | --- | --- |
| `BEYONDTRUST_TEST_AZURE_TENANT_ID` | Yes | Azure AD directory (tenant) UUID |
| `BEYONDTRUST_TEST_AZURE_CLIENT_ID` | Yes | Application (client) ID of the integration service principal |
| `BEYONDTRUST_TEST_AZURE_CLIENT_SECRET` | Yes | Client secret for the integration service principal |
| `BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID` | Yes | **Object ID** (not client ID) of the target app registration |

## Security Best Practices

✅ **DO**:
- Use short-lived client secrets and rotate them regularly
- Use a dedicated test-only service principal (not shared with production)
- Use a dedicated test-only target app registration
- Store secrets in GitHub Actions secrets or a secrets manager — never in `.envrc` committed to git
- Grant only the minimum required permission (ownership over `Application.ReadWrite.All`)

❌ **DON'T**:
- Reuse production Azure credentials for testing
- Commit `.envrc` (only `.envrc.example`) to version control
- Use a target app registration that has production secrets attached
