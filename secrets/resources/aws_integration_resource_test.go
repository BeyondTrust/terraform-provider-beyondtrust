package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This file contains AWS Integration-specific unit tests.
// Shared helper tests are in resource_helpers_test.go

// TestValidateAwsRoleArn tests ARN format validation
// This is AWS-SPECIFIC and critical for security (wrong ARN = access failure)
func TestValidateAwsRoleArn(t *testing.T) {
	tests := []struct {
		name        string
		arn         string
		isValid     bool
		description string
	}{
		{
			name:        "valid standard role ARN",
			arn:         "arn:aws:iam::123456789012:role/MyRole",
			isValid:     true,
			description: "Standard AWS role ARN should be valid",
		},
		{
			name:        "valid role with path",
			arn:         "arn:aws:iam::123456789012:role/path/to/MyRole",
			isValid:     true,
			description: "Role ARN with path should be valid",
		},
		{
			name:        "valid role with hyphens",
			arn:         "arn:aws:iam::123456789012:role/my-role-name",
			isValid:     true,
			description: "Role name with hyphens should be valid",
		},
		{
			name:        "valid role with underscores",
			arn:         "arn:aws:iam::123456789012:role/my_role_name",
			isValid:     true,
			description: "Role name with underscores should be valid",
		},
		{
			name:        "aws-cn partition",
			arn:         "arn:aws-cn:iam::123456789012:role/MyRole",
			isValid:     true,
			description: "China partition ARN should be valid",
		},
		{
			name:        "aws-gov partition",
			arn:         "arn:aws-us-gov:iam::123456789012:role/MyRole",
			isValid:     true,
			description: "GovCloud partition ARN should be valid",
		},
		{
			name:        "invalid - not a role",
			arn:         "arn:aws:iam::123456789012:user/MyUser",
			isValid:     false,
			description: "User ARN should be invalid (must be role)",
		},
		{
			name:        "invalid - missing account ID",
			arn:         "arn:aws:iam:::role/MyRole",
			isValid:     false,
			description: "ARN without account ID should be invalid",
		},
		{
			name:        "invalid - wrong service",
			arn:         "arn:aws:s3:::bucket-name",
			isValid:     false,
			description: "S3 ARN should be invalid (must be IAM)",
		},
		{
			name:        "invalid - missing role name",
			arn:         "arn:aws:iam::123456789012:role/",
			isValid:     false,
			description: "ARN without role name should be invalid",
		},
		{
			name:        "invalid - malformed",
			arn:         "not-an-arn",
			isValid:     false,
			description: "Malformed string should be invalid",
		},
		{
			name:        "invalid - empty",
			arn:         "",
			isValid:     false,
			description: "Empty ARN should be invalid",
		},
		{
			name:        "invalid - account ID not numeric",
			arn:         "arn:aws:iam::abcdefghijkl:role/MyRole",
			isValid:     false,
			description: "Non-numeric account ID should be invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateAwsRoleArn(tt.arn)
			assert.Equal(t, tt.isValid, result, tt.description)
		})
	}
}

// TestValidateAwsExternalId tests external ID format validation
// This is AWS-SPECIFIC for security (confused deputy prevention)
func TestValidateAwsExternalId(t *testing.T) {
	tests := []struct {
		name        string
		externalId  string
		isValid     bool
		description string
	}{
		{
			name:        "valid alphanumeric",
			externalId:  "my-external-id-123",
			isValid:     true,
			description: "Alphanumeric with hyphens should be valid",
		},
		{
			name:        "valid with allowed special chars",
			externalId:  "id_+=,.@:\\/test-123",
			isValid:     true,
			description: "Allowed special characters should be valid",
		},
		{
			name:        "valid minimum length (2 chars)",
			externalId:  "ab",
			isValid:     true,
			description: "Minimum 2 characters should be valid",
		},
		{
			name:        "valid maximum length (1224 chars)",
			externalId:  strings.Repeat("a", 1224),
			isValid:     true,
			description: "Maximum 1224 characters should be valid",
		},
		{
			name:        "invalid - too short (1 char)",
			externalId:  "a",
			isValid:     false,
			description: "Single character should be invalid",
		},
		{
			name:        "invalid - too long (1225 chars)",
			externalId:  strings.Repeat("a", 1225),
			isValid:     false,
			description: "Over 1224 characters should be invalid",
		},
		{
			name:        "invalid - empty",
			externalId:  "",
			isValid:     false,
			description: "Empty external ID should be invalid",
		},
		{
			name:        "invalid - disallowed special chars",
			externalId:  "test!@#$%^&*()",
			isValid:     false,
			description: "Disallowed special characters should be invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateAwsExternalId(tt.externalId)
			assert.Equal(t, tt.isValid, result, tt.description)
		})
	}
}
