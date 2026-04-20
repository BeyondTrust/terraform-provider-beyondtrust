# AWS Integration Testing Guide

This guide explains how to run AWS integration acceptance tests.

## Quick Start

The tests will **automatically create and cleanup IAM roles** using your AWS credentials.

### Prerequisites

1. **AWS Credentials** configured on your machine (one of):
   - AWS CLI: `~/.aws/credentials` and `~/.aws/config`
   - Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
   - IAM instance profile (if running on EC2)
   - AWS SSO

2. **IAM Permissions** - Your AWS credentials need:
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "iam:CreateRole",
           "iam:DeleteRole",
           "iam:PutRolePolicy",
           "iam:DeleteRolePolicy",
           "iam:GetRole",
           "iam:TagRole",
           "sts:GetCallerIdentity"
         ],
         "Resource": "*"
       }
     ]
   }
   ```

3. **BeyondTrust AWS Account ID** - Set the AWS account ID that your BeyondTrust Workload Credentials uses:
   ```bash
   export BEYONDTRUST_AWS_ACCOUNT_ID="999999999999"  # Replace with actual ID
   ```

   > **Where to find this**: Check your BeyondTrust Workload Credentials console under AWS Integration settings, or ask your BeyondTrust administrator.

### Running Tests

```bash
# Set required environment variables
export BEYONDTRUST_API_URL="https://your-workload-credentials-instance.com"
export BEYONDTRUST_ACCESS_TOKEN="your-token"
export BEYONDTRUST_AWS_ACCOUNT_ID="999999999999"

# Optional: Use specific AWS profile
export AWS_PROFILE=my-profile

# Run AWS integration tests
TF_ACC=1 go test -v -timeout=30m -run TestAccAwsIntegration ./secrets/resources/
```

The tests will:
1. Detect your AWS account ID automatically using STS GetCallerIdentity
2. Create temporary IAM roles with names like `tf-acc-test-bt-a1b2c3d4`
3. Generate a random external ID for the test
4. Run the tests
5. Clean up the IAM roles automatically (even if tests fail)

## Advanced Configuration

### Use Pre-Created Roles (Optional)

If you prefer to use pre-created IAM roles instead of auto-creation:

```bash
export BEYONDTRUST_TEST_AWS_ROLE_ARN="arn:aws:iam::123456789012:role/my-test-role"
export BEYONDTRUST_TEST_AWS_ROLE_ARN_2="arn:aws:iam::123456789012:role/my-test-role-2"
export BEYONDTRUST_TEST_AWS_EXTERNAL_ID="my-external-id-uuid"
```

The roles must have this trust policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::<BEYONDTRUST_AWS_ACCOUNT_ID>:root"
      },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "sts:ExternalId": "<YOUR_EXTERNAL_ID>"
        }
      }
    }
  ]
}
```

### Specify AWS Region

```bash
export AWS_REGION=us-west-2
```

### Use AWS SSO

```bash
aws sso login --profile my-sso-profile
export AWS_PROFILE=my-sso-profile
```

## Example Test Usage

Here's how the new AWS helpers work in tests:

```go
func TestAccAwsIntegrationResource_basic(t *testing.T) {
    // This automatically creates roles and cleans them up
    roleARN1, roleARN2, externalID, cleanup := acctest.SetupAWSTestRoles(t)
    defer cleanup()

    integrationName := acctest.RandomIntegrationName()

    resource.ParallelTest(t, resource.TestCase{
        PreCheck:                 func() { acctest.PreCheck(t) },
        ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
        Steps: []resource.TestStep{
            {
                Config: testAccConfig(integrationName, roleARN1, externalID),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "role_arn", roleARN1),
                ),
            },
        },
    })
}
```

## Troubleshooting

### "BEYONDTRUST_AWS_ACCOUNT_ID must be set"

The tests need to know which AWS account BeyondTrust uses to assume roles. Check your Workload Credentials console or contact your BeyondTrust administrator.

### "Failed to create AWS session"

Ensure AWS credentials are configured:
```bash
aws sts get-caller-identity
```

### "AccessDenied" when creating IAM roles

Your AWS credentials need IAM permissions. Use an admin role or attach the policy shown in Prerequisites.

### Roles not cleaning up

The tests use `defer` to ensure cleanup. If tests panic or are force-killed, run:
```bash
# List test roles
aws iam list-roles --query 'Roles[?starts_with(RoleName, `tf-acc-test-bt-`)].RoleName' --output table

# Delete manually if needed
aws iam delete-role-policy --role-name tf-acc-test-bt-XXXXX --policy-name tf-acc-test-policy
aws iam delete-role --role-name tf-acc-test-bt-XXXXX
```

## CI/CD Integration

### GitHub Actions with OIDC (Recommended)

**Step 1: Create IAM Role for GitHub Actions** (one-time setup)

```bash
# Trust policy for GitHub OIDC
cat > github-trust-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::YOUR_ACCOUNT_ID:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:YOUR_ORG/terraform-provider-beyondtrust:*"
        }
      }
    }
  ]
}
EOF

# Create the role
aws iam create-role \
  --role-name GitHubActions-BeyondTrust-Tests \
  --assume-role-policy-document file://github-trust-policy.json

# Attach permissions
aws iam attach-role-policy \
  --role-name GitHubActions-BeyondTrust-Tests \
  --policy-arn arn:aws:iam::aws:policy/IAMFullAccess  # Or create custom policy
```

**Step 2: GitHub Actions Workflow**

```yaml
name: Acceptance Tests
on: [push, pull_request]

permissions:
  id-token: write  # Required for OIDC
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Configure AWS Credentials via OIDC
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::YOUR_ACCOUNT_ID:role/GitHubActions-BeyondTrust-Tests
          aws-region: us-east-1
          # No credentials needed! Uses OIDC web identity token

      - name: Verify AWS Identity
        run: aws sts get-caller-identity

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Run Acceptance Tests
        env:
          TF_ACC: "1"
          BEYONDTRUST_API_URL: ${{ secrets.BEYONDTRUST_API_URL }}
          BEYONDTRUST_ACCESS_TOKEN: ${{ secrets.BEYONDTRUST_ACCESS_TOKEN }}
          BEYONDTRUST_AWS_ACCOUNT_ID: ${{ secrets.BEYONDTRUST_AWS_ACCOUNT_ID }}
        run: go test -v -timeout=30m ./...
```

### Alternative: AWS Assume Role from EC2/ECS

If running on AWS infrastructure:

```yaml
jobs:
  test:
    runs-on: self-hosted  # EC2 with IAM instance profile
    steps:
      - uses: actions/checkout@v4

      - name: Run Tests
        env:
          TF_ACC: "1"
          # AWS credentials come from instance profile automatically
          BEYONDTRUST_API_URL: ${{ secrets.BEYONDTRUST_API_URL }}
          BEYONDTRUST_ACCESS_TOKEN: ${{ secrets.BEYONDTRUST_ACCESS_TOKEN }}
          BEYONDTRUST_AWS_ACCOUNT_ID: ${{ secrets.BEYONDTRUST_AWS_ACCOUNT_ID }}
        run: go test -v -timeout=30m ./...
```

## Environment Variables Reference

| Variable                           | Required | Default        | Description                                           |
|------------------------------------|----------|----------------|-------------------------------------------------------|
| `BEYONDTRUST_AWS_ACCOUNT_ID`       | Yes      | -              | BeyondTrust Workload Credentials's AWS account ID     |
| `BEYONDTRUST_TEST_AWS_ROLE_ARN`    | No       | Auto-created   | Pre-created test role ARN                             |
| `BEYONDTRUST_TEST_AWS_ROLE_ARN_2`  | No       | Auto-created   | Second test role ARN                                  |
| `BEYONDTRUST_TEST_AWS_EXTERNAL_ID` | No       | Auto-generated | External ID for role trust                            |
| `AWS_PROFILE`                      | No       | `default`      | AWS CLI profile to use (local dev)                    |
| `AWS_REGION`                       | No       | `us-east-1`    | AWS region                                            |

**Note**: AWS credentials are obtained via:
- **Preferred**: IAM role via OIDC web identity token (CI/CD)
- **Alternative**: IAM instance profile (EC2/ECS)
- **Local dev**: AWS CLI profiles (`~/.aws/credentials` with SSO or temporary credentials)
- **Never use**: Hardcoded access keys

## Security Best Practices

✅ **DO**:
- Use OIDC web identity tokens for CI/CD (GitHub Actions, GitLab CI, etc.)
- Use IAM instance profiles for EC2/ECS workloads
- Use AWS SSO for local development
- Use temporary credentials with short expiration
- Rotate credentials regularly (when using long-lived credentials)

❌ **DON'T**:
- Store AWS access keys in code or environment variables
- Commit credentials to version control
- Use root account credentials
- Share credentials between users or systems
- Use long-lived IAM user access keys in production
