package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
)

// testAccProtoV6ProviderFactories are used locally in provider package tests
var testAccProtoV6ProviderFactories = acctest.ProtoV6ProviderFactories

// testAccPreCheck validates that all required environment variables are set
// before running acceptance tests.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	// Required environment variables for acceptance tests
	requiredEnvVars := []string{
		"BEYONDTRUST_API_URL",
		"BEYONDTRUST_ACCESS_TOKEN",
	}

	// Optional but recommended environment variables
	optionalEnvVars := []string{
		"BEYONDTRUST_SITE_ID",
		"BEYONDTRUST_API_VERSION",
	}

	// Check required variables
	for _, envVar := range requiredEnvVars {
		if v := os.Getenv(envVar); v == "" {
			t.Fatalf("%s must be set for acceptance tests", envVar)
		}
	}

	// Log optional variables
	for _, envVar := range optionalEnvVars {
		if v := os.Getenv(envVar); v == "" {
			t.Logf("%s is not set (optional)", envVar)
		}
	}
}

// providerConfig returns a basic provider configuration for testing.
// It uses environment variables to configure the provider.
func providerConfig() string {
	return `
provider "beyondtrust" {
  # Configuration comes from environment variables:
  # BEYONDTRUST_API_URL
  # BEYONDTRUST_ACCESS_TOKEN
  # BEYONDTRUST_SITE_ID (optional)
  # BEYONDTRUST_API_VERSION (optional)
}
`
}

// TestProviderSchema validates that the provider schema is correct.
func TestProviderSchema(t *testing.T) {
	ctx := context.Background()
	prov := New("test")()

	schemaReq := provider.SchemaRequest{}
	schemaResp := &provider.SchemaResponse{}

	prov.Schema(ctx, schemaReq, schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema has errors: %v", schemaResp.Diagnostics)
	}

	// Validate required attributes exist
	requiredAttrs := []string{
		"api_url",
		"access_token",
		"site_id",
		"api_version",
		"api_path_version",
		"role",
		"insecure",
		"timeout",
	}

	for _, attr := range requiredAttrs {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q to be in provider schema", attr)
		}
	}
}

// TestProviderConfigure_EnvVarPrecedence validates that environment variables are used when config is not set.
func TestProviderConfigure_EnvVarPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "all env vars set",
			envVars: map[string]string{
				"BEYONDTRUST_API_URL":      "https://env.example.com",
				"BEYONDTRUST_ACCESS_TOKEN": "env-token",
				"BEYONDTRUST_SITE_ID":      "env-site-123",
			},
			wantErr: false,
		},
		{
			name: "missing api_url",
			envVars: map[string]string{
				"BEYONDTRUST_ACCESS_TOKEN": "env-token",
				"BEYONDTRUST_SITE_ID":      "env-site-123",
			},
			wantErr: true,
		},
		{
			name: "missing access_token",
			envVars: map[string]string{
				"BEYONDTRUST_API_URL": "https://env.example.com",
				"BEYONDTRUST_SITE_ID": "env-site-123",
			},
			wantErr: true,
		},
		{
			name: "missing site_id",
			envVars: map[string]string{
				"BEYONDTRUST_API_URL":      "https://env.example.com",
				"BEYONDTRUST_ACCESS_TOKEN": "env-token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			clearEnvVars()
			defer clearEnvVars()

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			ctx := context.Background()
			prov := New("test")()

			// Create empty config (all values from env vars)
			configValue := tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"api_url":          tftypes.String,
					"access_token":     tftypes.String,
					"site_id":          tftypes.String,
					"api_version":      tftypes.String,
					"api_path_version": tftypes.String,
					"role":             tftypes.String,
					"insecure":         tftypes.Bool,
					"timeout":          tftypes.String,
				},
			}, map[string]tftypes.Value{
				"api_url":          tftypes.NewValue(tftypes.String, nil),
				"access_token":     tftypes.NewValue(tftypes.String, nil),
				"site_id":          tftypes.NewValue(tftypes.String, nil),
				"api_version":      tftypes.NewValue(tftypes.String, nil),
				"api_path_version": tftypes.NewValue(tftypes.String, nil),
				"role":             tftypes.NewValue(tftypes.String, nil),
				"insecure":         tftypes.NewValue(tftypes.Bool, nil),
				"timeout":          tftypes.NewValue(tftypes.String, nil),
			})

			configReq := provider.ConfigureRequest{
				TerraformVersion: "1.10.0",
				Config: tfsdk.Config{
					Raw:    configValue,
					Schema: getProviderSchema(t, prov),
				},
			}
			configResp := &provider.ConfigureResponse{}

			prov.Configure(ctx, configReq, configResp)

			if tt.wantErr && !configResp.Diagnostics.HasError() {
				t.Error("expected error, got none")
			}
			if !tt.wantErr && configResp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", configResp.Diagnostics)
			}
		})
	}
}

// TestProviderConfigure_ConfigOverridesEnv validates that config values override environment variables.
func TestProviderConfigure_ConfigOverridesEnv(t *testing.T) {
	tests := []struct {
		name         string
		envVars      map[string]string
		configValues map[string]interface{}
		wantErr      bool
	}{
		{
			name: "config overrides all env vars",
			envVars: map[string]string{
				"BEYONDTRUST_API_URL":      "https://env.example.com",
				"BEYONDTRUST_ACCESS_TOKEN": "env-token",
				"BEYONDTRUST_SITE_ID":      "env-site-123",
			},
			configValues: map[string]interface{}{
				"api_url":      "https://config.example.com",
				"access_token": "config-token",
				"site_id":      "config-site-456",
			},
			wantErr: false,
		},
		{
			name: "config overrides partial env vars",
			envVars: map[string]string{
				"BEYONDTRUST_API_URL":      "https://env.example.com",
				"BEYONDTRUST_ACCESS_TOKEN": "env-token",
				"BEYONDTRUST_SITE_ID":      "env-site-123",
			},
			configValues: map[string]interface{}{
				"api_url": "https://config.example.com",
			},
			wantErr: false,
		},
		{
			name:    "config only, no env vars",
			envVars: map[string]string{},
			configValues: map[string]interface{}{
				"api_url":      "https://config.example.com",
				"access_token": "config-token",
				"site_id":      "config-site-456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			clearEnvVars()
			defer clearEnvVars()

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			ctx := context.Background()
			prov := New("test")()

			// Create config with specified values
			configValue := buildConfigValue(tt.configValues)

			configReq := provider.ConfigureRequest{
				TerraformVersion: "1.10.0",
				Config: tfsdk.Config{
					Raw:    configValue,
					Schema: getProviderSchema(t, prov),
				},
			}
			configResp := &provider.ConfigureResponse{}

			prov.Configure(ctx, configReq, configResp)

			if tt.wantErr && !configResp.Diagnostics.HasError() {
				t.Error("expected error, got none")
			}
			if !tt.wantErr && configResp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", configResp.Diagnostics)
			}
		})
	}
}

// TestProviderConfigure_OptionalFields validates that optional fields and defaults work correctly.
func TestProviderConfigure_OptionalFields(t *testing.T) {
	tests := []struct {
		name         string
		configValues map[string]interface{}
		wantErr      bool
	}{
		{
			name: "with valid timeout",
			configValues: map[string]interface{}{
				"api_url":      "https://test.example.com",
				"access_token": "test-token",
				"site_id":      "test-site-123",
				"timeout":      "60s",
			},
			wantErr: false,
		},
		{
			name: "with all optional fields",
			configValues: map[string]interface{}{
				"api_url":          "https://test.example.com",
				"access_token":     "test-token",
				"site_id":          "test-site-123",
				"api_version":      "2025-01-01",
				"api_path_version": "v1",
				"role":             "admin",
				"insecure":         true,
				"timeout":          "60s",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			clearEnvVars()
			defer clearEnvVars()

			ctx := context.Background()
			prov := New("test")()

			// Create config with specified values
			configValue := buildConfigValue(tt.configValues)

			configReq := provider.ConfigureRequest{
				TerraformVersion: "1.10.0",
				Config: tfsdk.Config{
					Raw:    configValue,
					Schema: getProviderSchema(t, prov),
				},
			}
			configResp := &provider.ConfigureResponse{}

			prov.Configure(ctx, configReq, configResp)

			if tt.wantErr && !configResp.Diagnostics.HasError() {
				t.Error("expected error, got none")
			}
			if !tt.wantErr && configResp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", configResp.Diagnostics)
			}
		})
	}
}

// TestProviderConfigure_TimeoutValidation validates timeout parsing.
func TestProviderConfigure_TimeoutValidation(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		wantErr bool
	}{
		{
			name:    "invalid timeout format",
			timeout: "invalid",
			wantErr: true,
		},
		{
			name:    "valid timeout",
			timeout: "30s",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			clearEnvVars()
			defer clearEnvVars()

			ctx := context.Background()
			prov := New("test")()

			// Create config with timeout
			configValue := buildConfigValue(map[string]interface{}{
				"api_url":      "https://test.example.com",
				"access_token": "test-token",
				"site_id":      "test-site-123",
				"timeout":      tt.timeout,
			})

			configReq := provider.ConfigureRequest{
				TerraformVersion: "1.10.0",
				Config: tfsdk.Config{
					Raw:    configValue,
					Schema: getProviderSchema(t, prov),
				},
			}
			configResp := &provider.ConfigureResponse{}

			prov.Configure(ctx, configReq, configResp)

			if tt.wantErr && !configResp.Diagnostics.HasError() {
				t.Error("expected error, got none")
			}
			if !tt.wantErr && configResp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", configResp.Diagnostics)
			}
		})
	}
}

// Helper functions

// clearEnvVars clears all BeyondTrust-related environment variables.
func clearEnvVars() {
	os.Unsetenv("BEYONDTRUST_API_URL")
	os.Unsetenv("BEYONDTRUST_ACCESS_TOKEN")
	os.Unsetenv("BEYONDTRUST_SITE_ID")
	os.Unsetenv("BEYONDTRUST_API_VERSION")
	os.Unsetenv("BEYONDTRUST_API_PATH_VERSION")
	os.Unsetenv("BEYONDTRUST_ROLE")
	os.Unsetenv("BEYONDTRUST_INSECURE")
	os.Unsetenv("BEYONDTRUST_TIMEOUT")
}

// getProviderSchema returns the provider schema for testing.
func getProviderSchema(t *testing.T, prov provider.Provider) schema.Schema {
	t.Helper()
	ctx := context.Background()
	schemaReq := provider.SchemaRequest{}
	schemaResp := &provider.SchemaResponse{}
	prov.Schema(ctx, schemaReq, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get provider schema: %v", schemaResp.Diagnostics)
	}
	return schemaResp.Schema
}

// buildConfigValue builds a tftypes.Value for provider configuration.
func buildConfigValue(values map[string]interface{}) tftypes.Value {
	configMap := map[string]tftypes.Value{
		"api_url":          tftypes.NewValue(tftypes.String, nil),
		"access_token":     tftypes.NewValue(tftypes.String, nil),
		"site_id":          tftypes.NewValue(tftypes.String, nil),
		"api_version":      tftypes.NewValue(tftypes.String, nil),
		"api_path_version": tftypes.NewValue(tftypes.String, nil),
		"role":             tftypes.NewValue(tftypes.String, nil),
		"insecure":         tftypes.NewValue(tftypes.Bool, nil),
		"timeout":          tftypes.NewValue(tftypes.String, nil),
	}

	for k, v := range values {
		switch k {
		case "insecure":
			configMap[k] = tftypes.NewValue(tftypes.Bool, v)
		default:
			configMap[k] = tftypes.NewValue(tftypes.String, v)
		}
	}

	return tftypes.NewValue(tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"api_url":          tftypes.String,
			"access_token":     tftypes.String,
			"site_id":          tftypes.String,
			"api_version":      tftypes.String,
			"api_path_version": tftypes.String,
			"role":             tftypes.String,
			"insecure":         tftypes.Bool,
			"timeout":          tftypes.String,
		},
	}, configMap)
}
