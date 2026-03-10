package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/secrets/datasources"
	ephemeralresources "github.com/beyondtrust/terraform-provider-beyondtrust/secrets/ephemeral"
	"github.com/beyondtrust/terraform-provider-beyondtrust/secrets/resources"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ provider.Provider = &BeyondTrustProvider{}
var _ provider.ProviderWithEphemeralResources = &BeyondTrustProvider{}

// BeyondTrustProvider defines the provider implementation.
type BeyondTrustProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and run locally, and "test" when running acceptance
	// testing.
	version string
}

// BeyondTrustProviderModel describes the provider data model.
type BeyondTrustProviderModel struct {
	ApiUrl          types.String `tfsdk:"api_url"`
	AccessToken     types.String `tfsdk:"access_token"`
	SiteId          types.String `tfsdk:"site_id"`
	ApiVersion      types.String `tfsdk:"api_version"`       // Header version (date-based)
	ApiPathVersion  types.String `tfsdk:"api_path_version"`  // Optional path version (e.g., "v1")
	Role            types.String `tfsdk:"role"`              // X-BT-Role header value (auth type is always CUSTOM-IDP when role is set)
	Insecure        types.Bool   `tfsdk:"insecure"`
	Timeout         types.String `tfsdk:"timeout"`
}

func (p *BeyondTrustProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "beyondtrust"
	resp.Version = p.version
}

func (p *BeyondTrustProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The BeyondTrust provider allows you to manage BeyondTrust resources (SMOP, PRA, etc.) using infrastructure as code.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Description: "The base URL for the BeyondTrust API (e.g., https://api.smop.example.com). Can also be set via BEYONDTRUST_API_URL environment variable.",
				Optional:    true,
			},
			"access_token": schema.StringAttribute{
				Description: "The API access token for authentication. Can also be set via BEYONDTRUST_ACCESS_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"site_id": schema.StringAttribute{
				Description: "The site/tenant ID (UUID format). Required for multi-tenant deployments. Can also be set via BEYONDTRUST_SITE_ID environment variable.",
				Optional:    true,
			},
			"api_version": schema.StringAttribute{
				Description: "The API header version (date-based, e.g., '2026-02-16'). Defaults to '2026-02-16'. Can also be set via BEYONDTRUST_API_VERSION environment variable.",
				Optional:    true,
			},
			"api_path_version": schema.StringAttribute{
				Description: "Optional API path version (e.g., 'v1'). Defaults to empty string (no path version). Can also be set via BEYONDTRUST_API_PATH_VERSION environment variable.",
				Optional:    true,
			},
			"role": schema.StringAttribute{
				Description: "Role for X-BT-Role header (when set, X-BT-Auth-Type is automatically set to 'CUSTOM-IDP'). Can also be set via BEYONDTRUST_ROLE environment variable.",
				Optional:    true,
			},
			"insecure": schema.BoolAttribute{
				Description: "Skip TLS certificate verification. Only use for development. Defaults to false. Can also be set via BEYONDTRUST_INSECURE environment variable.",
				Optional:    true,
			},
			"timeout": schema.StringAttribute{
				Description: "HTTP client timeout duration (e.g., '30s', '1m'). Defaults to '30s'. Can also be set via BEYONDTRUST_TIMEOUT environment variable.",
				Optional:    true,
			},
		},
	}
}

func (p *BeyondTrustProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config BeyondTrustProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values and environment variable fallbacks
	apiUrl := os.Getenv("BEYONDTRUST_API_URL")
	accessToken := os.Getenv("BEYONDTRUST_ACCESS_TOKEN")
	siteId := os.Getenv("BEYONDTRUST_SITE_ID")
	apiVersion := os.Getenv("BEYONDTRUST_API_VERSION")
	apiPathVersion := os.Getenv("BEYONDTRUST_API_PATH_VERSION")
	role := os.Getenv("BEYONDTRUST_ROLE")
	insecure := os.Getenv("BEYONDTRUST_INSECURE") == "true"
	timeout := os.Getenv("BEYONDTRUST_TIMEOUT")

	// Configuration values override environment variables
	if !config.ApiUrl.IsNull() {
		apiUrl = config.ApiUrl.ValueString()
	}

	if !config.AccessToken.IsNull() {
		accessToken = config.AccessToken.ValueString()
	}

	if !config.SiteId.IsNull() {
		siteId = config.SiteId.ValueString()
	}

	if !config.ApiVersion.IsNull() {
		apiVersion = config.ApiVersion.ValueString()
	}

	if !config.ApiPathVersion.IsNull() {
		apiPathVersion = config.ApiPathVersion.ValueString()
	}

	if !config.Role.IsNull() {
		role = config.Role.ValueString()
	}

	if !config.Insecure.IsNull() {
		insecure = config.Insecure.ValueBool()
	}

	if !config.Timeout.IsNull() {
		timeout = config.Timeout.ValueString()
	}

	// Apply defaults
	if apiVersion == "" {
		apiVersion = "2026-02-16"
	}

	if timeout == "" {
		timeout = "30s"
	}

	// Validate required configuration
	if apiUrl == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Missing API URL",
			"The provider requires an API URL. Set the api_url attribute or BEYONDTRUST_API_URL environment variable.",
		)
	}

	if accessToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_token"),
			"Missing Access Token",
			"The provider requires an access token for authentication. Set the access_token attribute or BEYONDTRUST_ACCESS_TOKEN environment variable.",
		)
	}

	if siteId == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("site_id"),
			"Missing Site ID",
			"The provider requires a site ID for multi-tenant isolation. Set the site_id attribute or BEYONDTRUST_SITE_ID environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create API client
	clientConfig := &client.Config{
		BaseURL:        apiUrl,
		AccessToken:    accessToken,
		SiteID:         siteId,
		APIVersion:     apiVersion,
		APIPathVersion: apiPathVersion,
		Role:           role,
		Insecure:       insecure,
		Timeout:        timeout,
	}

	apiClient, err := client.NewClient(clientConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create BeyondTrust API Client",
			"An unexpected error occurred when creating the BeyondTrust API client. "+
				"Error: "+err.Error(),
		)
		return
	}

	// Validate the client by checking the session
	// TODO: Re-enable once /session endpoint permissions are fixed
	// if err := apiClient.ValidateSession(ctx); err != nil {
	// 	resp.Diagnostics.AddError(
	// 		"Unable to Authenticate with BeyondTrust API",
	// 		"The provider could not authenticate with the BeyondTrust API. "+
	// 			"Please check your access token and API URL. "+
	// 			"Error: "+err.Error(),
	// 	)
	// 	return
	// }

	// Make the client available to resources, data sources, and ephemeral resources
	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
	resp.EphemeralResourceData = apiClient
}

func (p *BeyondTrustProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewFolderResource,
		resources.NewStaticSecretResource,
		resources.NewAwsIntegrationResource,
		resources.NewAwsDynamicSecretResource,
	}
}

func (p *BeyondTrustProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewAwsIntegrationDataSource,
		// NewAwsDynamicSecretDataSource,
		// NewLeaseDataSource,
	}
}

func (p *BeyondTrustProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		ephemeralresources.NewStaticSecretEphemeral,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &BeyondTrustProvider{
			version: version,
		}
	}
}
