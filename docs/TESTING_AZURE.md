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

### Option A: Ownership + Application.ReadWrite.OwnedBy (Recommended — Least Privilege)

Two things are needed together, and ownership on its own is not enough.

BeyondTrust calls Microsoft Graph with an **app-only** token obtained via client credentials.
Directory ownership does not authorize an app-only token to call `addPassword`; the service
principal also needs a Graph application permission. `Application.ReadWrite.OwnedBy` is the
least-privileged one that fits, and it is scoped *by* ownership — it only reaches apps the
service principal owns. So you need both.

Granting only ownership produces an integration that validates successfully and then fails at generation with `azure_permission_denied`, because integration validation only acquires a Graph token and never exercises a write.

```bash
# 1. Make the integration service principal an owner of the target app.
INTEGRATION_SP_OBJECT_ID=$(az ad sp show --id "$INTEGRATION_APP_ID" --query id -o tsv)
az ad app owner add --id "$TARGET_OBJECT_ID" --owner-object-id "$INTEGRATION_SP_OBJECT_ID"

# 2. Grant it Application.ReadWrite.OwnedBy on Microsoft Graph.
GRAPH_SP_ID=$(az ad sp show --id 00000003-0000-0000-c000-000000000000 --query id -o tsv)
OWNED_BY_ROLE_ID=$(az ad sp show --id 00000003-0000-0000-c000-000000000000 \
  --query "appRoles[?value=='Application.ReadWrite.OwnedBy'].id | [0]" -o tsv)

az rest --method POST \
  --uri "https://graph.microsoft.com/v1.0/servicePrincipals/$INTEGRATION_SP_OBJECT_ID/appRoleAssignments" \
  --body "{
    \"principalId\": \"$INTEGRATION_SP_OBJECT_ID\",
    \"resourceId\": \"$GRAPH_SP_ID\",
    \"appRoleId\": \"$OWNED_BY_ROLE_ID\"
  }"
```

> **Note**: `--owner-object-id` takes the **service principal's** object id, not the app registration's. Passing the wrong one still succeeds and leaves generation failing.

Assigning an app role requires admin consent. If you do not hold it, see [Who can grant this](#who-can-grant-this) below.

Verify both halves:

```bash
# Expect the integration service principal in the owner list.
az ad app owner list --id "$TARGET_OBJECT_ID" --query "[].{id:id,displayName:displayName}" -o table

# Expect one assignment against Microsoft Graph. Empty means the role was never granted.
az rest --method GET \
  --uri "https://graph.microsoft.com/v1.0/servicePrincipals/$INTEGRATION_SP_OBJECT_ID/appRoleAssignments" \
  --query "value[].{appRoleId:appRoleId,resource:resourceDisplayName}" -o table
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

### Who can grant this

Both options assign a Microsoft Graph application permission, which always requires tenant-wide admin consent. Assigning it needs one of these Entra ID directory roles:

| Role | Can grant `Application.ReadWrite.OwnedBy`? |
| --- | --- |
| Global Administrator | Yes |
| Privileged Role Administrator | Yes |
| Cloud Application Administrator | Yes — the least-privileged role that suffices |
| Application Administrator | Yes |

Cloud Application Administrator and Application Administrator cannot consent to the most escalation-prone Graph permissions, notably `Application.ReadWrite.All` and `AppRoleAssignment.ReadWrite.All`. `Application.ReadWrite.OwnedBy` is not in that set, so either role is enough for Option A — which is another reason to prefer it over Option B.

If you only need this done once, an administrator can do it in the portal without granting you a standing role: **Entra ID → App registrations → the integration app → API permissions → Add a permission → Microsoft Graph → Application permissions → Application.ReadWrite.OwnedBy → Add**, then **Grant admin consent**.

Consent can take a few minutes to propagate; a generation attempt immediately afterwards may still fail.

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

Generation failing with `azure_permission_denied` means Microsoft Graph returned 401 or 403 on the `addPassword` call. It is a Graph refusal, not a Workload Credentials authorization problem, so no policy or grant on the BeyondTrust side affects it.

A healthy-looking integration proves nothing here: integration validation only acquires a Graph token, which succeeds with no application permissions at all. The first call that actually writes is generation.

Check both halves of the grant:

1. The integration service principal owns the target app. The id in the output must be the **service principal's** object id, not the app registration's:
   ```bash
   az ad app owner list --id "$TARGET_OBJECT_ID" --query "[].{id:id,displayName:displayName}" -o table
   ```
2. It holds a Graph application permission. Empty output means none was ever granted, which is the usual cause:
   ```bash
   INTEGRATION_SP_OBJECT_ID=$(az ad sp show --id "$INTEGRATION_APP_ID" --query id -o tsv)
   az rest --method GET \
     --uri "https://graph.microsoft.com/v1.0/servicePrincipals/$INTEGRATION_SP_OBJECT_ID/appRoleAssignments" \
     --query "value[].{appRoleId:appRoleId,resource:resourceDisplayName}" -o table
   ```
   Note this is different from `az ad sp show --query appRoles`, which lists the roles an app *defines* rather than the ones it has been *granted*.
3. Allow up to 5 minutes for admin consent to propagate.

If step 2 comes back empty, grant `Application.ReadWrite.OwnedBy` as shown in [Option A](#option-a-ownership--applicationreadwriteownedby-recommended--least-privilege).

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
