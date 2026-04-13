package acctest

import (
	"fmt"
	"os"
)

// TestConfig holds configuration for acceptance tests
type TestConfig struct {
	APIURL      string `json:"api_url"`
	SiteID      string `json:"site_id"`
	AccessToken string `json:"access_token"`
	APIVersion  string `json:"api_version,omitempty"` // Optional, defaults to 2026-02-16
}

// LoadTestConfig loads test configuration from environment variables
func LoadTestConfig() (*TestConfig, error) {
	cfg := &TestConfig{
		APIURL:      os.Getenv("BEYONDTRUST_API_URL"),
		SiteID:      os.Getenv("BEYONDTRUST_SITE_ID"),
		AccessToken: os.Getenv("BEYONDTRUST_ACCESS_TOKEN"),
		APIVersion:  os.Getenv("BEYONDTRUST_API_VERSION"),
	}

	// Set default API version if not specified
	if cfg.APIVersion == "" {
		cfg.APIVersion = "2026-02-16"
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
		return fmt.Errorf("BEYONDTRUST_API_URL is required")
	}
	if c.SiteID == "" {
		return fmt.Errorf("BEYONDTRUST_SITE_ID is required")
	}
	if c.AccessToken == "" {
		return fmt.Errorf("BEYONDTRUST_ACCESS_TOKEN is required")
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
