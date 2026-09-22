package acctest

import (
	"fmt"
	"os"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/constants"
)

// Environment variable names for AWS-specific acceptance tests
const (
	EnvTestAWSRoleARN       = "BEYONDTRUST_TEST_AWS_ROLE_ARN"
	EnvTestAWSRoleARN2      = "BEYONDTRUST_TEST_AWS_ROLE_ARN_2"
	EnvTestAWSTargetRoleARN = "BEYONDTRUST_TEST_AWS_TARGET_ROLE_ARN"
	EnvTestAWSExternalID    = "BEYONDTRUST_TEST_AWS_EXTERNAL_ID"
	EnvAWSAccountID         = "BEYONDTRUST_AWS_ACCOUNT_ID"
)

// Environment variable names for Azure-specific acceptance tests
const (
	EnvTestAzureTenantID     = "BEYONDTRUST_TEST_AZURE_TENANT_ID"
	EnvTestAzureClientID     = "BEYONDTRUST_TEST_AZURE_CLIENT_ID"
	EnvTestAzureClientSecret = "BEYONDTRUST_TEST_AZURE_CLIENT_SECRET"
	EnvTestAzureAppObjectID  = "BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID"
)

// Environment variable names for admin-site acceptance tests (workload identities).
// Those endpoints require an org-admin caller operating against the org's admin site,
// which is a different site than the normal (secrets) tests use.
const (
	EnvAdminSiteID      = "BEYONDTRUST_ADMIN_SITE_ID"
	EnvAdminAccessToken = "BEYONDTRUST_ADMIN_ACCESS_TOKEN"
)

// Environment variable names for the IAM policy binding tests, which prove a Cedar policy
// actually changes what the product API returns.
//
// These need a third identity: a low-privilege user on the product site whose access must be
// denied before the policy is granted and allowed after. The admin-site credentials author the
// policy and the normal-site credentials create the fixtures, so neither can play that role.
const (
	// EnvTestPolicyPrincipal is the Cedar principal the test policies grant to, written as a
	// complete entity so any principal type works without the tests knowing about each one:
	//
	//	Pathfinder::Workload::Id::"<uuid>"     a workload identity, which has no email
	//	Pathfinder::User::Email::"a@b.com"     a human
	//	Pathfinder::Group::"platform-admins"   a group
	//
	// A workload id is taken at face value and never checked for existence, so a wrong one
	// still reports ACTIVE. The binding tests catch that by asserting access actually changes.
	EnvTestPolicyPrincipal = "BEYONDTRUST_TEST_POLICY_PRINCIPAL"

	// EnvTestPolicyPrincipalToken is an access token for the principal above. Set either this
	// or EnvTestPolicyPrincipalServiceName.
	EnvTestPolicyPrincipalToken = "BEYONDTRUST_TEST_POLICY_PRINCIPAL_TOKEN"

	// EnvTestPolicyPrincipalServiceName selects the principal's workload identity by service
	// name, reusing the product-site token. This suits CI, where identities are federated
	// rather than issued long-lived tokens: one token is minted for the site, and the service
	// name picks which registration it resolves to.
	EnvTestPolicyPrincipalServiceName = "BEYONDTRUST_TEST_POLICY_PRINCIPAL_SERVICE_NAME"

	// EnvTestPolicyOwnerServiceName selects the workload identity that seeds the binding tests'
	// fixtures. It exists because BEYONDTRUST_SERVICE_NAME cannot serve both sites at once: in
	// the policy job that variable names the admin-site identity, which the provider and the
	// admin client need, while the fixture owner is a separate identity on the product site.
	// Unset falls back to BEYONDTRUST_SERVICE_NAME, which is right when one identity covers both.
	EnvTestPolicyOwnerServiceName = "BEYONDTRUST_TEST_POLICY_OWNER_SERVICE_NAME"

	EnvTestPolicySiteID = "BEYONDTRUST_TEST_POLICY_SITE_ID"
)

// TestConfig holds configuration for acceptance tests
type TestConfig struct {
	APIURL      string `json:"api_url"`
	SiteID      string `json:"site_id"`
	AccessToken string `json:"access_token"`
	APIVersion  string `json:"api_version,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

// LoadTestConfig loads test configuration from environment variables
func LoadTestConfig() (*TestConfig, error) {
	cfg := &TestConfig{
		APIURL:      os.Getenv(constants.EnvAPIURL),
		SiteID:      os.Getenv(constants.EnvSiteID),
		AccessToken: os.Getenv(constants.EnvAccessToken),
		APIVersion:  os.Getenv(constants.EnvAPIVersion),
		ServiceName: os.Getenv(constants.EnvServiceName),
	}

	// Set default API version if not specified
	if cfg.APIVersion == "" {
		cfg.APIVersion = client.DefaultAPIVersion
	}

	// api_url defaults to the public endpoint; override via env for GovCloud/other.
	if cfg.APIURL == "" {
		cfg.APIURL = client.DefaultAPIURL
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("missing required environment variables: %w", err)
	}

	return cfg, nil
}

// Validate checks that all required fields are present
func (c *TestConfig) Validate() error {
	if c.APIURL == "" {
		return fmt.Errorf("%s is required", constants.EnvAPIURL)
	}
	if c.SiteID == "" {
		return fmt.Errorf("%s is required", constants.EnvSiteID)
	}
	if c.AccessToken == "" {
		return fmt.Errorf("%s is required", constants.EnvAccessToken)
	}
	return nil
}

// ProviderConfig returns a Terraform provider configuration block using this config
func (c *TestConfig) ProviderConfig() string {
	config := fmt.Sprintf(`
provider "beyondtrust" {
  api_url      = %q
  site_id      = %q
  access_token = %q
`, c.APIURL, c.SiteID, c.AccessToken)

	if c.APIVersion != "" {
		config += fmt.Sprintf("  api_version  = %q\n", c.APIVersion)
	}

	config += "}\n"
	return config
}

// LoadAdminTestConfig loads configuration for admin-site acceptance tests (workload
// identities). Those run against the org's admin site, which has its own dedicated
// credentials, so they require BEYONDTRUST_ADMIN_SITE_ID and BEYONDTRUST_ADMIN_ACCESS_TOKEN.
// The base/normal-site site id and token are not used (only the shared API URL/version are).
func LoadAdminTestConfig() (*TestConfig, error) {
	cfg := &TestConfig{
		APIURL:      os.Getenv(constants.EnvAPIURL),
		SiteID:      os.Getenv(EnvAdminSiteID),
		AccessToken: os.Getenv(EnvAdminAccessToken),
		APIVersion:  os.Getenv(constants.EnvAPIVersion),
		ServiceName: os.Getenv(constants.EnvServiceName),
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = client.DefaultAPIVersion
	}

	if cfg.APIURL == "" {
		cfg.APIURL = client.DefaultAPIURL
	}
	if cfg.SiteID == "" {
		return nil, fmt.Errorf("%s is required", EnvAdminSiteID)
	}
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("%s is required", EnvAdminAccessToken)
	}
	return cfg, nil
}

// LoadPrincipalTestConfig returns the product-site config for the low-privilege principal. The
// site is the same as LoadTestConfig's — only the identity differs, because the whole point is to
// observe two identities against one site.
//
// The identity comes from whichever is set: a dedicated access token, or a service name that
// selects a different workload identity while reusing the product-site token.
func LoadPrincipalTestConfig() (*TestConfig, error) {
	cfg, err := LoadTestConfig()
	if err != nil {
		return nil, err
	}

	if token := os.Getenv(EnvTestPolicyPrincipalToken); token != "" {
		cfg.AccessToken = token
		cfg.ServiceName = os.Getenv(EnvTestPolicyPrincipalServiceName) // usually empty
		return cfg, nil
	}
	if name := os.Getenv(EnvTestPolicyPrincipalServiceName); name != "" {
		cfg.ServiceName = name
		return cfg, nil
	}
	return nil, fmt.Errorf("set %s or %s", EnvTestPolicyPrincipalToken, EnvTestPolicyPrincipalServiceName)
}

// PolicyTargetSiteID returns the site a test policy's @siteId annotation should name: the product
// site the fixtures live on, unless explicitly overridden.
func PolicyTargetSiteID() string {
	if override := os.Getenv(EnvTestPolicySiteID); override != "" {
		return override
	}
	return os.Getenv(constants.EnvSiteID)
}

// NewClientForConfig builds an API client for an explicit site/token pair, so one test can hold
// several identities at once — the admin-site policy author, the product-site owner that seeds
// fixtures, and the low-privilege principal under test.
func NewClientForConfig(cfg *TestConfig) (*client.Client, error) {
	return client.NewClient(&client.Config{
		BaseURL:     cfg.APIURL,
		AccessToken: cfg.AccessToken,
		SiteID:      cfg.SiteID,
		APIVersion:  cfg.APIVersion,
		ServiceName: cfg.ServiceName,
		Timeout:     "30s",
	})
}

// NewTestClient creates a new API client for acceptance testing, configured the
// same way the provider configures its own.
//
// Use it for checks that read an object expected to exist. Destroy checks want
// NewDestroyCheckClient instead: they assert an object is gone, so the
// stale-read retry can only burn its whole budget failing.
func NewTestClient() (*client.Client, error) {
	cfg, err := LoadTestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load test config: %w", err)
	}
	return NewClientForConfig(cfg)
}

// NewPolicyOwnerTestClient creates the product-site client that seeds the binding tests'
// fixtures, selecting its workload identity by service name when one is configured.
func NewPolicyOwnerTestClient() (*client.Client, error) {
	cfg, err := LoadTestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load test config: %w", err)
	}
	if name := os.Getenv(EnvTestPolicyOwnerServiceName); name != "" {
		cfg.ServiceName = name
	}
	return NewClientForConfig(cfg)
}

// NewAdminTestClient creates a client against the org's admin site, where the IAM policy and
// auth services live.
func NewAdminTestClient() (*client.Client, error) {
	cfg, err := LoadAdminTestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load admin test config: %w", err)
	}
	return NewClientForConfig(cfg)
}

// NewPrincipalTestClient creates a client for the low-privilege principal on the product site,
// with the stale-read retry on 403 disabled.
//
// The binding tests treat 403 as a result rather than a symptom: they assert the principal is
// denied before a policy is granted, and poll for the denial to turn into access afterwards.
// Retrying inside the client would burn the whole budget proving a denial that is expected, and
// would sit underneath the tests' own waiting for the grant to take effect.
func NewPrincipalTestClient() (*client.Client, error) {
	cfg, err := LoadPrincipalTestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load principal test config: %w", err)
	}
	c, err := NewClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	c.StaleReadRetry = client.StaleReadRetryDisabled()
	return c, nil
}

// NewDestroyCheckClient creates an API client for destroy verification, with the
// stale-read retry on 403 disabled.
//
// The client retries a 403 on GET because it may mean a just-written object is
// not yet visible to reads. A destroy check asserts the opposite — that the
// object is gone — so the object really is absent and every retry is guaranteed
// to fail. Retrying there burns the full backoff budget per resource for a
// result that cannot change.
//
// Use NewTestClient for checks that read an object expected to exist, where the
// retry is doing useful work.
func NewDestroyCheckClient() (*client.Client, error) {
	c, err := NewTestClient()
	if err != nil {
		return nil, err
	}

	c.StaleReadRetry = client.StaleReadRetryDisabled()

	return c, nil
}
