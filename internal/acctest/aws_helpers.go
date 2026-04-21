package acctest

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
)

// AWSTestRole represents a test IAM role for BeyondTrust acceptance tests
type AWSTestRole struct {
	RoleARN    string
	ExternalID string
	RoleName   string
	iamClient  *iam.IAM
}

// GetAWSSession creates an AWS session using default credential chain
// This respects AWS_PROFILE, ~/.aws/credentials, instance profiles, etc.
func GetAWSSession(t *testing.T) *session.Session {
	t.Helper()

	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		t.Fatalf("Failed to create AWS session: %v", err)
	}

	return sess
}

// GetAWSAccountID gets the current AWS account ID using STS GetCallerIdentity
func GetAWSAccountID(t *testing.T, sess *session.Session) string {
	t.Helper()

	stsClient := sts.New(sess)
	result, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatalf("Failed to get AWS account ID: %v", err)
	}

	return aws.StringValue(result.Account)
}

// GetOrGenerateExternalID gets external ID from env or generates a new one
func GetOrGenerateExternalID(t *testing.T) string {
	t.Helper()

	if externalID := os.Getenv(EnvTestAWSExternalID); externalID != "" {
		return externalID
	}

	// Generate a random external ID for this test run
	return RandomString(32)
}

// GetBeyondTrustAWSAccountID returns the BeyondTrust AWS account ID
// This is the account that BeyondTrust Workload Credentials uses to assume customer roles
func GetBeyondTrustAWSAccountID(t *testing.T) string {
	t.Helper()

	// Check if set in environment
	if accountID := os.Getenv(EnvAWSAccountID); accountID != "" {
		return accountID
	}

	// If not set, skip the test and provide instructions
	t.Skip(EnvAWSAccountID + " must be set to run AWS integration tests.\n" +
		"This is the AWS account ID that your BeyondTrust Workload Credentials instance uses to assume roles.\n" +
		"You can find this in your Workload Credentials console under AWS integration settings.\n" +
		"Example: export " + EnvAWSAccountID + "=123456789012")

	return ""
}

// CreateTestIAMRole creates a test IAM role that BeyondTrust can assume
func CreateTestIAMRole(t *testing.T, roleName string) *AWSTestRole {
	t.Helper()

	sess := GetAWSSession(t)
	iamClient := iam.New(sess)

	// Get BeyondTrust's AWS account ID for trust policy
	beyondTrustAccountID := GetBeyondTrustAWSAccountID(t)

	// Get or generate external ID
	externalID := GetOrGenerateExternalID(t)

	// Build trust policy
	trustPolicy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect": "Allow",
				"Principal": map[string]interface{}{
					"AWS": fmt.Sprintf("arn:aws:iam::%s:root", beyondTrustAccountID),
				},
				"Action": "sts:AssumeRole",
				"Condition": map[string]interface{}{
					"StringEquals": map[string]interface{}{
						"sts:ExternalId": externalID,
					},
				},
			},
		},
	}

	trustPolicyJSON, err := json.Marshal(trustPolicy)
	if err != nil {
		t.Fatalf("Failed to marshal trust policy: %v", err)
	}

	// Create the IAM role
	t.Logf("Creating IAM role: %s", roleName)
	createRoleOutput, err := iamClient.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(string(trustPolicyJSON)),
		Description:              aws.String("Test role for BeyondTrust Terraform provider acceptance tests"),
		Tags: []*iam.Tag{
			{Key: aws.String("Purpose"), Value: aws.String("TerraformProviderTesting")},
			{Key: aws.String("ManagedBy"), Value: aws.String("GoTest")},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create IAM role: %v", err)
	}

	roleARN := aws.StringValue(createRoleOutput.Role.Arn)
	t.Logf("Created IAM role: %s", roleARN)

	// Attach minimal test policy
	testPolicy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect": "Allow",
				"Action": []string{
					"iam:GetRole",
					"iam:GetUser",
					"iam:ListRoles",
					"sts:GetCallerIdentity",
				},
				"Resource": "*",
			},
		},
	}

	testPolicyJSON, err := json.Marshal(testPolicy)
	if err != nil {
		t.Fatalf("Failed to marshal test policy: %v", err)
	}

	_, err = iamClient.PutRolePolicy(&iam.PutRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyName:     aws.String("tf-acc-test-policy"),
		PolicyDocument: aws.String(string(testPolicyJSON)),
	})
	if err != nil {
		// Clean up role if policy attachment fails
		_, _ = iamClient.DeleteRole(&iam.DeleteRoleInput{
			RoleName: aws.String(roleName),
		})
		t.Fatalf("Failed to attach policy to role: %v", err)
	}

	return &AWSTestRole{
		RoleARN:    roleARN,
		ExternalID: externalID,
		RoleName:   roleName,
		iamClient:  iamClient,
	}
}

// Cleanup deletes the test IAM role and its policies
func (r *AWSTestRole) Cleanup(t *testing.T) {
	t.Helper()

	t.Logf("Cleaning up IAM role: %s", r.RoleName)

	// Delete inline policies first
	_, err := r.iamClient.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
		RoleName:   aws.String(r.RoleName),
		PolicyName: aws.String("tf-acc-test-policy"),
	})
	if err != nil {
		t.Logf("Warning: Failed to delete role policy: %v", err)
	}

	// Delete the role
	_, err = r.iamClient.DeleteRole(&iam.DeleteRoleInput{
		RoleName: aws.String(r.RoleName),
	})
	if err != nil {
		t.Logf("Warning: Failed to delete IAM role: %v", err)
	} else {
		t.Logf("Deleted IAM role: %s", r.RoleName)
	}
}

// SetupAWSTestRoles creates test IAM roles for acceptance tests
// Returns primary and secondary role ARNs and external ID
// Use defer testRole.Cleanup(t) to ensure cleanup
func SetupAWSTestRoles(t *testing.T) (roleARN1, roleARN2, externalID string, cleanup func()) {
	t.Helper()

	// Check if user provided pre-created roles
	if roleARN := os.Getenv(EnvTestAWSRoleARN); roleARN != "" {
		roleARN2 := GetAWSRoleARN2(t)
		externalID := GetOrGenerateExternalID(t)
		t.Logf("Using pre-created AWS role: %s", roleARN)
		return roleARN, roleARN2, externalID, func() {}
	}

	// Create roles dynamically
	role1 := CreateTestIAMRole(t, fmt.Sprintf("tf-acc-test-bt-%s", RandomString(8)))
	role2 := CreateTestIAMRole(t, fmt.Sprintf("tf-acc-test-bt-%s-2", RandomString(8)))

	cleanup = func() {
		role2.Cleanup(t)
		role1.Cleanup(t)
	}

	return role1.RoleARN, role2.RoleARN, role1.ExternalID, cleanup
}
