package acctest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TestConfig holds configuration for acceptance tests
type TestConfig struct {
	APIURL      string `json:"api_url"`
	SiteID      string `json:"site_id"`
	AccessToken string `json:"access_token"`
	APIVersion  string `json:"api_version,omitempty"` // Optional, defaults to 2026-02-16
}

// LoadTestConfig loads test configuration from file or environment variables
// Priority: test.config.json > environment variables
func LoadTestConfig() (*TestConfig, error) {
	// 1. Try loading from config file first (for local development)
	// Search in multiple locations: current dir, project root
	configPaths := []string{
		"test.config.json",             // Current directory
		"../test.config.json",          // Parent directory
		"../../test.config.json",       // Two levels up
		"../../../test.config.json",    // Three levels up
		"../../../../test.config.json", // Four levels up
	}

	// Also try to find project root by looking for go.mod
	if root, err := findProjectRoot(); err == nil {
		configPaths = append([]string{filepath.Join(root, "test.config.json")}, configPaths...)
	}

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // File doesn't exist at this path, try next
		}

		var cfg TestConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}

		// Validate required fields from file
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid config file %s: %w", path, err)
		}

		return &cfg, nil
	}

	// 2. Fall back to environment variables (for CI/CD)
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
		return nil, fmt.Errorf("missing required environment variables: %w\nSet either test.config.json or environment variables (BEYONDTRUST_API_URL, BEYONDTRUST_SITE_ID, BEYONDTRUST_ACCESS_TOKEN)", err)
	}

	return cfg, nil
}

// Validate checks that all required fields are present
func (c *TestConfig) Validate() error {
	if c.APIURL == "" {
		return fmt.Errorf("api_url is required")
	}
	if c.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if c.AccessToken == "" {
		return fmt.Errorf("access_token is required")
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

// findProjectRoot searches for the project root by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
