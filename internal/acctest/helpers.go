package acctest

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/constants"
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
		t.Fatalf("Failed to load test configuration: %v\n\nSet environment variables:\n  %s\n  %s\n  %s\n\nFor local dev: cp .envrc.example .envrc (see TESTING.md)", err, constants.EnvAPIURL, constants.EnvSiteID, constants.EnvAccessToken)
	}
}

// AWSSkipReason returns why the AWS acceptance tests cannot run, or "" when they can.
//
// Returned rather than skipped directly so callers can also count and report it — a suite
// that skips silently reports green while verifying nothing. PreCheckAWS is the shorthand
// for callers that only need to skip.
func AWSSkipReason() string {
	if os.Getenv(EnvTestAWSRoleARN) == "" {
		return EnvTestAWSRoleARN + " is not set"
	}
	return ""
}

// PreCheckAWS checks that AWS-specific environment variables are set
func PreCheckAWS(t *testing.T) {
	t.Helper()

	if reason := AWSSkipReason(); reason != "" {
		t.Skipf("AWS acceptance tests skipped: %s", reason)
	}
}

// AzureSkipReason returns why the Azure acceptance tests cannot run, or "" when they can.
// See AWSSkipReason for why this is returned rather than skipped.
func AzureSkipReason() string {
	var missing []string
	for _, env := range []string{EnvTestAzureTenantID, EnvTestAzureClientID, EnvTestAzureClientSecret, EnvTestAzureAppObjectID} {
		if os.Getenv(env) == "" {
			missing = append(missing, env)
		}
	}
	if len(missing) > 0 {
		return "missing environment variables: " + strings.Join(missing, ", ")
	}
	return ""
}

// PreCheckAzure checks that Azure-specific environment variables are set
func PreCheckAzure(t *testing.T) {
	t.Helper()

	if reason := AzureSkipReason(); reason != "" {
		t.Skipf("Azure acceptance tests skipped: %s", reason)
	}
}

// GetAzureTenantID returns the Azure tenant ID for testing
func GetAzureTenantID(t *testing.T) string {
	t.Helper()
	return os.Getenv(EnvTestAzureTenantID)
}

// GetAzureClientID returns the Azure client ID for testing
func GetAzureClientID(t *testing.T) string {
	t.Helper()
	return os.Getenv(EnvTestAzureClientID)
}

// GetAzureClientSecret returns the Azure client secret for testing
func GetAzureClientSecret(t *testing.T) string {
	t.Helper()
	return os.Getenv(EnvTestAzureClientSecret)
}

// GetAzureAppObjectID returns the Azure application object ID for testing
func GetAzureAppObjectID(t *testing.T) string {
	t.Helper()
	return os.Getenv(EnvTestAzureAppObjectID)
}

// PreCheckAdmin validates that admin-site credentials are available, skipping otherwise.
// Workload-identity endpoints require an org-admin caller operating against the org's admin
// site, which has its own dedicated credentials (a token is scoped to a single site), so both
// the admin site id and admin token are required in addition to the base env.
func PreCheckAdmin(t *testing.T) {
	t.Helper()

	if _, err := LoadAdminTestConfig(); err != nil {
		t.Skipf("workload-identity (admin-site) acceptance tests skipped: %v\n\nSet: %s, %s, %s", err, constants.EnvAPIURL, EnvAdminSiteID, EnvAdminAccessToken)
	}
}

// PolicyBindingSkipReason returns why the IAM policy binding tests cannot run, or "" when every
// credential they need is present: the admin site (authors the policy), the product site (seeds
// fixtures), and the low-privilege principal (the subject whose access must flip).
//
// Returned rather than skipped directly so callers can also count and report it — a suite that
// skips silently reports green while verifying nothing.
func PolicyBindingSkipReason() string {
	if _, err := LoadAdminTestConfig(); err != nil {
		return fmt.Sprintf("%v (set %s and %s)", err, EnvAdminSiteID, EnvAdminAccessToken)
	}
	if _, err := LoadPrincipalTestConfig(); err != nil {
		return fmt.Sprintf("%v (set %s, %s and %s)",
			err, constants.EnvSiteID, constants.EnvAccessToken, EnvTestPolicyPrincipalToken)
	}
	if reason := PolicyPrincipalSkipReason(); reason != "" {
		return reason
	}
	return ""
}

// GenerateGrantSkipReason returns why the dynamic credential tests cannot run, or "" when
// they can: they need the admin site to author the grant and a way to name the identity the
// grant is for, either EnvTestGeneratePrincipal or a service name to resolve it from.
func GenerateGrantSkipReason() string {
	if _, err := LoadAdminTestConfig(); err != nil {
		return fmt.Sprintf("%v (set %s and %s)", err, EnvAdminSiteID, EnvAdminAccessToken)
	}

	principal := os.Getenv(EnvTestGeneratePrincipal)
	if principal == "" {
		if os.Getenv(constants.EnvServiceName) == "" {
			return fmt.Sprintf("neither %s (a Cedar entity, e.g. Pathfinder::Workload::Id::%q) nor %s "+
				"(a service name to resolve it from) is set",
				EnvTestGeneratePrincipal, "<uuid>", constants.EnvServiceName)
		}
		return ""
	}
	if !strings.Contains(principal, "::") || !strings.Contains(principal, `"`) {
		return fmt.Sprintf("%s must be a whole Cedar entity such as Pathfinder::Workload::Id::%q, got %q",
			EnvTestGeneratePrincipal, "<uuid>", principal)
	}

	return ""
}

// PolicyPrincipalSkipReason returns why the configured Cedar principal is unusable, or "".
//
// The value is a whole Cedar entity rather than a bare name, because a workload identity has no
// email and the tests should not have to know which principal type they were handed. The check
// is deliberately shallow — the service validates the rest — but it catches a bare email or id
// pasted in, which is the mistake this variable invites.
func PolicyPrincipalSkipReason() string {
	principal := os.Getenv(EnvTestPolicyPrincipal)
	if principal == "" {
		return fmt.Sprintf("%s is not set (a Cedar entity, e.g. Pathfinder::Workload::Id::%q)",
			EnvTestPolicyPrincipal, "<uuid>")
	}
	if !strings.Contains(principal, "::") || !strings.Contains(principal, `"`) {
		return fmt.Sprintf("%s must be a whole Cedar entity such as Pathfinder::Workload::Id::%q, got %q",
			EnvTestPolicyPrincipal, "<uuid>", principal)
	}
	return ""
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

// GetAWSTargetRoleARN returns the AWS target role ARN for dynamic secret testing.
// This is the role that the integration will assume when generating credentials,
// and may be in a different AWS account than the integration role.
func GetAWSTargetRoleARN(t *testing.T) string {
	t.Helper()

	if v := os.Getenv(EnvTestAWSTargetRoleARN); v != "" {
		return v
	}

	// Fall back to the primary integration role ARN
	return GetAWSRoleARN(t)
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
	return "tf-acc-test/" + RandomString(8)
}

// RandomFolderName generates a random folder name for testing.
func RandomFolderName() string {
	return "tf-acc-test-" + RandomString(8)
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

// workloadIdentityPage is the shape of GET /platform/auth/workload-identities, reduced to
// what resolving a service name needs.
type workloadIdentityPage struct {
	Issuers []struct {
		IdentityID  string `json:"identityId"`
		ServiceName string `json:"serviceName"`
	} `json:"issuers"`
	TotalCount int `json:"totalCount"`
}

// ResolveGeneratePrincipal returns the Cedar entity the dynamic credential tests grant to.
//
// EnvTestGeneratePrincipal wins when set. Otherwise the provider identity's service name
// (BEYONDTRUST_SERVICE_NAME) is looked up among the org's workload identities through the
// admin site, and the entity is Pathfinder::Workload::Id::"<identityId>", the form a workload
// identity takes in Cedar. Service names are matched case-insensitively.
func ResolveGeneratePrincipal(ctx context.Context, admin *client.Client) (string, error) {
	if principal := os.Getenv(EnvTestGeneratePrincipal); principal != "" {
		return principal, nil
	}

	serviceName := os.Getenv(constants.EnvServiceName)
	if serviceName == "" {
		return "", fmt.Errorf("set %s to a Cedar entity, or %s to a service name to resolve one from",
			EnvTestGeneratePrincipal, constants.EnvServiceName)
	}

	var page workloadIdentityPage
	if err := admin.Get(ctx, admin.BuildAuthPath("/workload-identities"), nil, &page); err != nil {
		return "", fmt.Errorf("listing workload identities to resolve %q: %w", serviceName, err)
	}
	for _, issuer := range page.Issuers {
		if strings.EqualFold(issuer.ServiceName, serviceName) {
			return fmt.Sprintf("Pathfinder::Workload::Id::%q", issuer.IdentityID), nil
		}
	}
	if page.TotalCount > len(page.Issuers) {
		return "", fmt.Errorf("no workload identity named %q among the first %d of %d listed; set %s instead",
			serviceName, len(page.Issuers), page.TotalCount, EnvTestGeneratePrincipal)
	}
	return "", fmt.Errorf("no workload identity named %q among the %d the admin site lists",
		serviceName, len(page.Issuers))
}
