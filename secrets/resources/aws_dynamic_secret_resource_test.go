package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// This file contains AWS Dynamic Secret-specific unit tests.
// Shared helper tests are in resource_helpers_test.go

// TestValidateAwsAssumedRoleTTL tests TTL validation for assumed_role credentials
// This is AWS-SPECIFIC and CRITICAL for security/compliance (wrong TTL = security violation)
func TestValidateAwsAssumedRoleTTL(t *testing.T) {
	tests := []struct {
		name        string
		ttl         int64
		isValid     bool
		description string
	}{
		{
			name:        "valid - minimum TTL (900 seconds = 15 min)",
			ttl:         900,
			isValid:     true,
			description: "Minimum TTL of 900 seconds should be valid",
		},
		{
			name:        "valid - maximum TTL (43200 seconds = 12 hours)",
			ttl:         43200,
			isValid:     true,
			description: "Maximum TTL of 43200 seconds should be valid",
		},
		{
			name:        "valid - middle range (3600 seconds = 1 hour)",
			ttl:         3600,
			isValid:     true,
			description: "TTL of 3600 seconds should be valid",
		},
		{
			name:        "valid - 2 hours",
			ttl:         7200,
			isValid:     true,
			description: "TTL of 2 hours should be valid",
		},
		{
			name:        "invalid - below minimum (899 seconds)",
			ttl:         899,
			isValid:     false,
			description: "TTL below 900 seconds should be invalid",
		},
		{
			name:        "invalid - above maximum (43201 seconds)",
			ttl:         43201,
			isValid:     false,
			description: "TTL above 43200 seconds should be invalid",
		},
		{
			name:        "invalid - zero",
			ttl:         0,
			isValid:     false,
			description: "Zero TTL should be invalid",
		},
		{
			name:        "invalid - negative",
			ttl:         -100,
			isValid:     false,
			description: "Negative TTL should be invalid",
		},
		{
			name:        "invalid - extremely high",
			ttl:         86400, // 24 hours
			isValid:     false,
			description: "24 hour TTL should be invalid for assumed_role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateAssumedRoleTTL(tt.ttl)
			assert.Equal(t, tt.isValid, result, tt.description)
		})
	}
}

// TestValidateAwsCredentialType tests credential type validation
// This is AWS-SPECIFIC for ensuring correct credential type usage
func TestValidateAwsCredentialType(t *testing.T) {
	tests := []struct {
		name           string
		credentialType string
		isValid        bool
		description    string
	}{
		{
			name:           "valid - assumed_role",
			credentialType: "assumed_role",
			isValid:        true,
			description:    "assumed_role is currently supported",
		},
		{
			name:           "invalid - iam_user (future)",
			credentialType: "iam_user",
			isValid:        false,
			description:    "iam_user is not yet supported",
		},
		{
			name:           "invalid - federation_token (future)",
			credentialType: "federation_token",
			isValid:        false,
			description:    "federation_token is not yet supported",
		},
		{
			name:           "invalid - session_token (future)",
			credentialType: "session_token",
			isValid:        false,
			description:    "session_token is not yet supported",
		},
		{
			name:           "invalid - empty",
			credentialType: "",
			isValid:        false,
			description:    "Empty credential type should be invalid",
		},
		{
			name:           "invalid - unknown type",
			credentialType: "unknown_type",
			isValid:        false,
			description:    "Unknown credential type should be invalid",
		},
		{
			name:           "invalid - case sensitive",
			credentialType: "ASSUMED_ROLE",
			isValid:        false,
			description:    "Credential type should be case-sensitive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateAwsCredentialType(tt.credentialType)
			assert.Equal(t, tt.isValid, result, tt.description)
		})
	}
}

// TestValidateJSONPolicy tests AWS IAM policy JSON validation
// This is AWS-SPECIFIC and critical (invalid JSON = policy creation failure)
func TestValidateJSONPolicy(t *testing.T) {
	tests := []struct {
		name        string
		policy      string
		isValid     bool
		description string
	}{
		{
			name: "valid - simple policy",
			policy: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Action": "s3:GetObject",
					"Resource": "*"
				}]
			}`,
			isValid:     true,
			description: "Valid IAM policy JSON should pass",
		},
		{
			name: "valid - complex policy",
			policy: `{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Action": ["s3:GetObject", "s3:PutObject"],
						"Resource": "arn:aws:s3:::bucket/*"
					},
					{
						"Effect": "Deny",
						"Action": "s3:DeleteObject",
						"Resource": "*"
					}
				]
			}`,
			isValid:     true,
			description: "Complex multi-statement policy should pass",
		},
		{
			name:        "valid - minimal JSON",
			policy:      `{"Version":"2012-10-17"}`,
			isValid:     true,
			description: "Minimal valid JSON should pass",
		},
		{
			name:        "invalid - malformed JSON (missing quote)",
			policy:      `{"Version: "2012-10-17"}`,
			isValid:     false,
			description: "Malformed JSON should fail",
		},
		{
			name:        "invalid - malformed JSON (trailing comma)",
			policy:      `{"Version": "2012-10-17",}`,
			isValid:     false,
			description: "JSON with trailing comma should fail",
		},
		{
			name:        "invalid - not JSON",
			policy:      "not json",
			isValid:     false,
			description: "Plain text should fail",
		},
		{
			name:        "invalid - empty string",
			policy:      "",
			isValid:     false,
			description: "Empty policy should fail",
		},
		{
			name:        "invalid - JSON array instead of object",
			policy:      `["item1", "item2"]`,
			isValid:     false,
			description: "JSON array should fail (must be object)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONPolicy(tt.policy)
			if tt.isValid {
				assert.NoError(t, err, tt.description)
			} else {
				assert.Error(t, err, tt.description)
			}
		})
	}
}

// TestConvertAwsTagsMap tests AWS tags map conversion
// This is AWS-SPECIFIC for converting Terraform map to AWS tags format
func TestConvertAwsTagsMap(t *testing.T) {
	tests := []struct {
		name        string
		tagsMap     map[string]string
		expected    map[string]*string
		description string
	}{
		{
			name: "simple tags",
			tagsMap: map[string]string{
				"Environment": "production",
				"Team":        "platform",
			},
			expected: map[string]*string{
				"Environment": stringPtr("production"),
				"Team":        stringPtr("platform"),
			},
			description: "Simple tags should convert to pointer map",
		},
		{
			name:        "empty tags",
			tagsMap:     map[string]string{},
			expected:    map[string]*string{},
			description: "Empty tags should produce empty pointer map",
		},
		{
			name: "tags with special values",
			tagsMap: map[string]string{
				"CostCenter": "12345",
				"Owner":      "user@example.com",
			},
			expected: map[string]*string{
				"CostCenter": stringPtr("12345"),
				"Owner":      stringPtr("user@example.com"),
			},
			description: "Tags with numbers and special chars should convert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAwsTagsMap(tt.tagsMap)

			assert.Equal(t, len(tt.expected), len(result), "Map size should match")

			for key, expectedVal := range tt.expected {
				actualVal, exists := result[key]
				assert.True(t, exists, "Key %s should exist", key)

				if expectedVal == nil {
					assert.Nil(t, actualVal, "Key %s should be nil", key)
				} else {
					assert.NotNil(t, actualVal, "Key %s should have value", key)
					if actualVal != nil {
						assert.Equal(t, *expectedVal, *actualVal, "Value for key %s should match", key)
					}
				}
			}
		})
	}
}
