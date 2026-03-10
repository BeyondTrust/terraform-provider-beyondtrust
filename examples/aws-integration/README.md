# BeyondTrust SMOP + AWS Integration Example

This directory contains a complete example of setting up BeyondTrust Secrets Manager Operations Platform (SMOP) with AWS dynamic secrets.

## Overview

This example demonstrates:

1. **AWS IAM Setup**: Creating the necessary IAM roles in your AWS account
2. **BeyondTrust Configuration**: Setting up folder structure and AWS integration in SMOP
3. **Dynamic Secrets**: Configuring multiple dynamic secrets with different access patterns
4. **Security Best Practices**: External ID for confused deputy prevention, least privilege, short TTLs

## Prerequisites

### 1. BeyondTrust SMOP

- SMOP instance running and accessible
- Valid access token (obtain via `secrets login` or API key generation)
- Site/tenant ID (UUID)

### 2. AWS Account

- AWS account with permissions to create IAM roles and policies
- AWS CLI configured or credentials available

### 3. Tools

- Terraform >= 1.0
- AWS CLI (optional, for validation)
- jq (optional, for parsing output)

## Setup Instructions

### Step 1: Generate External ID

Generate a secure external ID for confused deputy prevention:

```bash
# Generate a random external ID
openssl rand -base64 32

# Or use UUID
uuidgen
```

Save this value securely - you'll need it for both AWS and SMOP configuration.

### Step 2: Configure Variables

Copy the example variables file:

```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your values:

```hcl
smop_api_url       = "https://api.smop.example.com"
smop_access_token  = "your-access-token"
smop_site_id       = "your-site-uuid"
external_id        = "your-generated-external-id"
admin_external_id  = "your-admin-external-id"  # Optional
```

**IMPORTANT**: `terraform.tfvars` is in `.gitignore`. Never commit it to version control!

### Step 3: Initialize Terraform

```bash
terraform init
```

### Step 4: Plan

Review the planned changes:

```bash
terraform plan -var-file=terraform.tfvars
```

Expected resources to be created:
- **AWS**: 3 IAM roles (integration, developer, admin) + policies
- **SMOP**: 2 folders + 1 integration + 3 dynamic secrets

### Step 5: Apply

Create the resources:

```bash
terraform apply -var-file=terraform.tfvars
```

Review the plan and type `yes` to confirm.

## Using Dynamic Secrets

### Generate Credentials via CLI

```bash
# Generate developer read-only credentials
secrets dynamic generate production/aws/developer-readonly

# Generate admin credentials (shorter TTL)
secrets dynamic generate production/aws/admin

# Generate S3-specific credentials
secrets dynamic generate production/aws/s3-data-bucket
```

The output will include:
- AWS Access Key ID
- AWS Secret Access Key
- AWS Session Token
- Expiration timestamp

### Use Generated Credentials

Export as environment variables:

```bash
# Generate and parse credentials
CREDS=$(secrets dynamic generate production/aws/developer-readonly --format json)

export AWS_ACCESS_KEY_ID=$(echo $CREDS | jq -r '.accessKeyId')
export AWS_SECRET_ACCESS_KEY=$(echo $CREDS | jq -r '.secretAccessKey')
export AWS_SESSION_TOKEN=$(echo $CREDS | jq -r '.sessionToken')

# Use with AWS CLI
aws sts get-caller-identity
aws s3 ls
```

## Security Best Practices

### 1. External ID Management

- **Unique per integration**: Use different external IDs for each AWS account
- **Secure generation**: Use cryptographically secure random generator
- **Rotation**: Periodically rotate external IDs (requires updating both AWS and SMOP)
- **Storage**: Store securely (e.g., in a secrets manager, not version control)

### 2. TTL Configuration

Recommended TTL values based on access level:

| Access Level | Recommended TTL | Max TTL (assumed_role) |
|--------------|-----------------|------------------------|
| Admin/Write  | 15-30 minutes   | 12 hours               |
| Read-Write   | 1-2 hours       | 12 hours               |
| Read-Only    | 2-4 hours       | 12 hours               |

Shorter TTLs = better security, but more frequent credential regeneration.

### 3. Least Privilege

- **Use inline policies**: Restrict to specific resources (e.g., specific S3 bucket)
- **Avoid AdministratorAccess**: Only use for true admin needs
- **Separate roles by team/purpose**: Don't reuse roles across different use cases
- **Leverage session policies**: Further restrict assumed role permissions

### 4. AWS Tags for Session Tracking

If you use `aws_tags` in your dynamic secrets for CloudTrail tracking, the integration role **must** have `sts:TagSession` permission:

```hcl
resource "aws_iam_role_policy" "smop_assume_roles" {
  name = "smop-assume-target-roles"
  role = aws_iam_role.smop_integration.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"  # Required when using aws_tags
        ]
        Resource = ["arn:aws:iam::${account_id}:role/beyondtrust/*"]
      }
    ]
  })
}
```

Without this permission, credential generation will fail with `AccessDenied: not authorized to perform: sts:TagSession`.

### 5. Monitoring and Auditing

- **Enable CloudTrail**: Log all `AssumeRole` calls
- **Monitor SMOP audit logs**: Track credential generations
- **Set up alerts**: Notify on unusual access patterns
- **Review regularly**: Audit who is generating credentials and for what
- **Use AWS tags**: Leverage `aws_tags` in dynamic secrets for better CloudTrail attribution

## Troubleshooting

### Issue: "Unable to assume role"

**Symptoms**: SMOP fails to create integration or generate credentials

**Possible causes**:
1. External ID mismatch
2. Role doesn't exist
3. Trust relationship incorrect
4. SMOP bridge account ID wrong
5. IAM role not fully propagated (eventual consistency)

**Resolution**:
```bash
# Verify trust relationship
aws iam get-role --role-name btp-account-role-YOUR_ACCOUNT_ID \
  --query 'Role.AssumeRolePolicyDocument' | jq

# Check external ID matches
# Check SMOP bridge account ID
# Verify role ARN in SMOP integration

# For eventual consistency issues, wait 10-15 seconds and retry
```

### Issue: "AccessDenied: not authorized to perform: sts:TagSession"

**Symptoms**: Integration creation succeeds, but credential generation fails with TagSession error

**Cause**: Using `aws_tags` in dynamic secrets without `sts:TagSession` permission

**Resolution**:
```bash
# Add sts:TagSession to integration role's policy
# See "AWS Tags for Session Tracking" section above
terraform apply
```

### Issue: "Access Denied" when using generated credentials

**Symptoms**: Credentials work but specific AWS operations fail

**Possible causes**:
1. Session policy is too restrictive
2. Target role lacks required permissions
3. Resource-based policies block access

**Resolution**:
```bash
# Test role permissions directly
aws sts assume-role --role-arn arn:aws:iam::ACCOUNT:role/beyondtrust/DeveloperReadOnlyRole \
  --role-session-name test

# Check effective permissions
aws iam simulate-principal-policy \
  --policy-source-arn arn:aws:iam::ACCOUNT:role/beyondtrust/DeveloperReadOnlyRole \
  --action-names s3:GetObject \
  --resource-arns arn:aws:s3:::bucket/key
```

## Cleanup

To destroy all resources:

```bash
# Destroy SMOP and AWS resources
terraform destroy -var-file=terraform.tfvars
```

**Warning**: This will:
- Delete all dynamic secrets (existing leases remain valid until expiration)
- Delete the AWS integration
- Delete folders in SMOP (soft delete)
- Delete IAM roles in AWS

## Additional Resources

- [BeyondTrust Provider Documentation](../../docs/index.md)
- [AWS IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
- [Confused Deputy Problem](https://docs.aws.amazon.com/IAM/latest/UserGuide/confused-deputy.html)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
