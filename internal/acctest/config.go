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
	EnvTestAzureTenantID    = "BEYONDTRUST_TEST_AZURE_TENANT_ID"
	EnvTestAzureClientID    = "BEYONDTRUST_TEST_AZURE_CLIENT_ID"
	EnvTestAzureClientSecret = "BEYONDTRUST_TEST_AZURE_CLIENT_SECRET"
	EnvTestAzureAppObjectID = "BEYONDTRUST_TEST_AZURE_APPLICATION_OBJECT_ID"
)

// Environment variable names for admin-site acceptance tests (workload identities).
// Those endpoints require an org-admin caller operating against the org's admin site,
// which is a different site than the normal (secrets) tests use.
const (
	EnvAdminSiteID      = "BEYONDTRUST_ADMIN_SITE_ID"
	EnvAdminAccessToken = "BEYONDTRUST_ADMIN_ACCESS_TOKEN"
)

// TestConfig holds configuration for acceptance tests
type TestConfig struct {
	APIURL      string `json:"api_url"`
	SiteID      string `json:"site_id"`
	AccessToken string `json:"access_token"`
	APIVersion  string `json:"api_version,omitempty"`
}

// LoadTestConfig loads test configuration from environment variables
func LoadTestConfig() (*TestConfig, error) {
	cfg := &TestConfig{
		APIURL:      os.Getenv(constants.EnvAPIURL),
		SiteID:      os.Getenv(constants.EnvSiteID),
		AccessToken: os.Getenv(constants.EnvAccessToken),
		APIVersion:  os.Getenv(constants.EnvAPIVersion),
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

// NewTestClient creates a new API client for acceptance testing.
// This is useful for destroy verification checks in acceptance tests.
func NewTestClient() (*client.Client, error) {
	cfg, err := LoadTestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load test config: %w", err)
	}

	clientCfg := &client.Config{
		BaseURL:     cfg.APIURL,
		AccessToken: cfg.AccessToken,
		SiteID:      cfg.SiteID,
		APIVersion:  cfg.APIVersion,
		Timeout:     "30s",
	}

	return client.NewClient(clientCfg)
}
