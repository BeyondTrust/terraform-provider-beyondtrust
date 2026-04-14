package acctest

import (
	"fmt"
	"os"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// Environment variable names for acceptance tests
const (
	EnvAPIURL      = "BEYONDTRUST_API_URL"
	EnvSiteID      = "BEYONDTRUST_SITE_ID"
	EnvAccessToken = "BEYONDTRUST_ACCESS_TOKEN"
	EnvAPIVersion  = "BEYONDTRUST_API_VERSION"

	// AWS-specific test environment variables
	EnvTestAWSRoleARN  = "BEYONDTRUST_TEST_AWS_ROLE_ARN"
	EnvTestAWSRoleARN2 = "BEYONDTRUST_TEST_AWS_ROLE_ARN_2"
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
		APIURL:      os.Getenv(EnvAPIURL),
		SiteID:      os.Getenv(EnvSiteID),
		AccessToken: os.Getenv(EnvAccessToken),
		APIVersion:  os.Getenv(EnvAPIVersion),
	}

	// Set default API version if not specified
	if cfg.APIVersion == "" {
		cfg.APIVersion = client.DefaultAPIVersion
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
		return fmt.Errorf("%s is required", EnvAPIURL)
	}
	if c.SiteID == "" {
		return fmt.Errorf("%s is required", EnvSiteID)
	}
	if c.AccessToken == "" {
		return fmt.Errorf("%s is required", EnvAccessToken)
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
