package acctest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRandomGeneration validates random string and integer generation.
func TestRandomGeneration(t *testing.T) {
	t.Run("string generation", func(t *testing.T) {
		result := RandomString(10)
		assert.Equal(t, 10, len(result), "string length should match")

		// Verify all characters are lowercase letters or numbers
		for _, char := range result {
			assert.True(t,
				(char >= 'a' && char <= 'z') || (char >= '0' && char <= '9'),
				"character should be lowercase letter or number")
		}
	})

	t.Run("integer generation", func(t *testing.T) {
		// Run multiple times to verify range boundaries
		for i := 0; i < 10; i++ {
			result := RandomInt(1, 10)
			assert.GreaterOrEqual(t, result, 1, "result should be >= min")
			assert.LessOrEqual(t, result, 10, "result should be <= max")
		}
	})
}

// TestRandomNaming validates resource name generation patterns.
func TestRandomNaming(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		wantPfx  string
		suffixLen int
	}{
		{
			name:      "resource name",
			fn:        func() string { return RandomResourceName("folder") },
			wantPfx:   "tf-acc-test-folder-",
			suffixLen: 8,
		},
		{
			name:      "folder path",
			fn:        RandomFolderPath,
			wantPfx:   "tf-acc-test/",
			suffixLen: 8,
		},
		{
			name:      "folder name",
			fn:        RandomFolderName,
			wantPfx:   "tf-acc-test-",
			suffixLen: 8,
		},
		{
			name:      "secret name",
			fn:        RandomSecretName,
			wantPfx:   "tf-acc-test-secret-",
			suffixLen: 8,
		},
		{
			name:      "integration name",
			fn:        RandomIntegrationName,
			wantPfx:   "tf-acc-test-integration-",
			suffixLen: 8,
		},
		{
			name:      "dynamic secret name",
			fn:        RandomDynamicSecretName,
			wantPfx:   "tf-acc-test-dynamic-",
			suffixLen: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn()
			assert.True(t, strings.HasPrefix(result, tt.wantPfx), "should start with %s", tt.wantPfx)
			suffix := result[len(tt.wantPfx):]
			assert.Equal(t, tt.suffixLen, len(suffix), "suffix should be %d characters", tt.suffixLen)
		})
	}
}

// TestRandomAWS validates AWS-specific random generation.
func TestRandomAWS(t *testing.T) {
	t.Run("ARN generation", func(t *testing.T) {
		result := RandomARN("iam", "role")

		// Verify ARN format components
		assert.True(t, strings.HasPrefix(result, "arn:aws:"), "should start with arn:aws:")
		assert.Contains(t, result, "iam", "should contain service name")
		assert.Contains(t, result, "role", "should contain resource type")
		assert.Contains(t, result, "us-east-1", "should contain region")
		assert.Contains(t, result, "123456789012", "should contain account ID")
		assert.Contains(t, result, "tf-acc-test-", "should contain test prefix")
	})

	t.Run("role ARN", func(t *testing.T) {
		result := RandomRoleARN()
		assert.True(t, strings.HasPrefix(result, "arn:aws:iam:"), "should be an IAM ARN")
		assert.Contains(t, result, "role", "should contain 'role'")
	})

	t.Run("tags generation", func(t *testing.T) {
		result := RandomTags()

		// Verify expected keys exist
		assert.Contains(t, result, "Environment")
		assert.Contains(t, result, "ManagedBy")
		assert.Contains(t, result, "TestRun")

		// Verify expected values
		assert.Equal(t, "test", result["Environment"])
		assert.Equal(t, "terraform", result["ManagedBy"])
		assert.Equal(t, 8, len(result["TestRun"]), "TestRun should be 8 characters")
	})
}
