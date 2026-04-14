package acctest

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// ProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
//
// Note: This MUST NOT import internal/provider to avoid import cycles.
// Instead, it's set by RegisterProviderFactory() called from provider_test.go init().
var ProtoV6ProviderFactories = make(map[string]func() (tfprotov6.ProviderServer, error))

// RegisterProviderFactory registers the provider factory for testing.
// This is called by internal/provider/provider_test.go init() to avoid import cycles.
func RegisterProviderFactory(name string, factory func() (tfprotov6.ProviderServer, error)) {
	ProtoV6ProviderFactories[name] = factory
}

// PreCheck validates that required test configuration is available
// before running acceptance tests via environment variables.
//
// For local development, use direnv:
//  1. cp .envrc.example .envrc
//  2. Edit .envrc with your credentials
//  3. direnv allow
func PreCheck(t *testing.T) {
	t.Helper()

	// Try to load test configuration from environment variables
	if _, err := LoadTestConfig(); err != nil {
		t.Fatalf("Failed to load test configuration: %v\n\nSet environment variables:\n  %s\n  %s\n  %s\n\nFor local dev: cp .envrc.example .envrc (see TESTING.md)", err, EnvAPIURL, EnvSiteID, EnvAccessToken)
	}
}

// PreCheckAWS checks that AWS-specific environment variables are set
func PreCheckAWS(t *testing.T) {
	t.Helper()

	if v := os.Getenv(EnvTestAWSRoleARN); v == "" {
		t.Skipf("%s must be set for AWS integration acceptance tests", EnvTestAWSRoleARN)
	}
}

// GetAWSRoleARN returns the AWS role ARN for testing
func GetAWSRoleARN(t *testing.T) string {
	t.Helper()

	if v := os.Getenv(EnvTestAWSRoleARN); v != "" {
		return v
	}

	// Fallback to a dummy ARN for validation testing
	return "arn:aws:iam::123456789012:role/tf-acc-test-role"
}

// GetAWSRoleARN2 returns a second AWS role ARN for update testing
func GetAWSRoleARN2(t *testing.T) string {
	t.Helper()

	if v := os.Getenv(EnvTestAWSRoleARN2); v != "" {
		return v
	}

	// Fallback to a dummy ARN for validation testing
	return "arn:aws:iam::123456789012:role/tf-acc-test-role-2"
}

// RandomString generates a random string of the specified length using lowercase letters and numbers.
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// RandomInt generates a random integer between min and max (inclusive).
func RandomInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}

// RandomResourceName generates a random resource name with a prefix.
// Format: "tf-acc-test-{prefix}-{random}"
func RandomResourceName(prefix string) string {
	return fmt.Sprintf("tf-acc-test-%s-%s", prefix, RandomString(8))
}

// RandomFolderPath generates a random folder path for testing.
// Format: "tf-acc-test/{random}"
func RandomFolderPath() string {
	return fmt.Sprintf("tf-acc-test/%s", RandomString(8))
}

// RandomFolderName generates a random folder name for testing.
func RandomFolderName() string {
	return fmt.Sprintf("tf-acc-test-%s", RandomString(8))
}

// RandomSecretName generates a random secret name for testing.
func RandomSecretName() string {
	return RandomResourceName("secret")
}

// RandomIntegrationName generates a random integration name for testing.
func RandomIntegrationName() string {
	return RandomResourceName("integration")
}

// RandomDynamicSecretName generates a random dynamic secret name for testing.
func RandomDynamicSecretName() string {
	return RandomResourceName("dynamic")
}

// RandomARN generates a random AWS ARN for testing.
func RandomARN(service, resourceType string) string {
	return fmt.Sprintf("arn:aws:%s:us-east-1:123456789012:%s/tf-acc-test-%s",
		service, resourceType, RandomString(8))
}

// RandomRoleARN generates a random AWS IAM role ARN for testing.
func RandomRoleARN() string {
	return RandomARN("iam", "role")
}

// RandomTags generates a random map of tags for testing.
func RandomTags() map[string]string {
	return map[string]string{
		"Environment": "test",
		"ManagedBy":   "terraform",
		"TestRun":     RandomString(8),
	}
}
