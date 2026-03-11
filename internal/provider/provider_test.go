package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"

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

// TestProvider validates that the provider can be instantiated.
func TestProvider(t *testing.T) {
	_ = context.Background()
	prov := New("test")()

	if prov == nil {
		t.Fatal("expected provider to be non-nil")
	}

	// Verify provider implements expected interfaces
	if _, ok := prov.(interface{ Metadata(context.Context, provider.MetadataRequest, *provider.MetadataResponse) }); !ok {
		t.Fatal("provider does not implement Metadata method")
	}
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
