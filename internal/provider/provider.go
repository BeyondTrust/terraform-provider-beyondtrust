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

	authresources "github.com/beyondtrust/terraform-provider-beyondtrust/auth/resources"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/constants"
	"github.com/beyondtrust/terraform-provider-beyondtrust/workload_credentials/datasources"
	ephemeralresources "github.com/beyondtrust/terraform-provider-beyondtrust/workload_credentials/ephemeral"
	"github.com/beyondtrust/terraform-provider-beyondtrust/workload_credentials/resources"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ provider.Provider                       = &BeyondTrustProvider{}
	_ provider.ProviderWithEphemeralResources = &BeyondTrustProvider{}
)

// Environment variable names for provider-specific configuration
const (
	EnvAPIPathVersion = "BEYONDTRUST_API_PATH_VERSION"
	EnvRole           = "BEYONDTRUST_ROLE"
	EnvServiceName    = "BEYONDTRUST_SERVICE_NAME"
	EnvInsecure       = "BEYONDTRUST_INSECURE"
	EnvTimeout        = "BEYONDTRUST_TIMEOUT"
)

// BeyondTrustProvider defines the provider implementation.
type BeyondTrustProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and run locally, and "test" when running acceptance
	// testing.
	version string
}

// BeyondTrustProviderModel describes the provider data model.
type BeyondTrustProviderModel struct {
	ApiUrl         types.String `tfsdk:"api_url"`
	AccessToken    types.String `tfsdk:"access_token"`
	SiteId         types.String `tfsdk:"site_id"`
	ApiVersion     types.String `tfsdk:"api_version"`      // Header version (date-based)
	ApiPathVersion types.String `tfsdk:"api_path_version"` // Optional path version (e.g., "v1")
	Role           types.String `tfsdk:"role"`             // X-BT-Role header value (auth type is always CUSTOM-IDP when role is set)
	ServiceName    types.String `tfsdk:"service_name"`     // X-BT-Service-Name header value (for GitHub OIDC authentication)
	Insecure       types.Bool   `tfsdk:"insecure"`
	Timeout        types.String `tfsdk:"timeout"`
}

func (p *BeyondTrustProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "beyondtrust"
	resp.Version = p.version
}

func (p *BeyondTrustProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The BeyondTrust provider allows you to manage BeyondTrust resources (Workload Credentials and other BeyondTrust services) using infrastructure as code.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Description: "The base URL for the BeyondTrust API. Defaults to " + client.DefaultAPIURL + "; override for other deployments such as GovCloud. Can also be set via " + constants.EnvAPIURL + " environment variable.",
				Optional:    true,
			},
			"access_token": schema.StringAttribute{
				Description: "The API access token for authentication. Can also be set via " + constants.EnvAccessToken + " environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"site_id": schema.StringAttribute{
				Description: "The site/tenant ID (UUID format). Required for multi-tenant deployments. Can also be set via " + constants.EnvSiteID + " environment variable.",
				Optional:    true,
			},
			"api_version": schema.StringAttribute{
				Description: "The API header version (date-based, e.g., '" + client.DefaultAPIVersion + "'). Defaults to '" + client.DefaultAPIVersion + "'. Can also be set via " + constants.EnvAPIVersion + " environment variable.",
				Optional:    true,
			},
			"api_path_version": schema.StringAttribute{
				Description: "Optional API path version (e.g., 'v1'). Defaults to empty string (no path version). Can also be set via " + EnvAPIPathVersion + " environment variable.",
				Optional:    true,
			},
			"role": schema.StringAttribute{
				Description: "Role for X-BT-Role header (when set, X-BT-Auth-Type is automatically set to 'CUSTOM-IDP'). Can also be set via " + EnvRole + " environment variable.",
				Optional:    true,
			},
			"service_name": schema.StringAttribute{
				Description: "Service name for GitHub OIDC authentication (X-BT-Service-Name header). Required when using GitHub OIDC tokens. Can also be set via " + EnvServiceName + " environment variable.",
				Optional:    true,
			},
			"insecure": schema.BoolAttribute{
				Description: "Skip TLS certificate verification. Only use for development. Defaults to false. Can also be set via " + EnvInsecure + " environment variable.",
				Optional:    true,
			},
			"timeout": schema.StringAttribute{
				Description: "HTTP client timeout duration (e.g., '30s', '1m'). Defaults to '30s'. Can also be set via " + EnvTimeout + " environment variable.",
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
	apiUrl := os.Getenv(constants.EnvAPIURL)
	accessToken := os.Getenv(constants.EnvAccessToken)
	siteId := os.Getenv(constants.EnvSiteID)
	apiVersion := os.Getenv(constants.EnvAPIVersion)
	apiPathVersion := os.Getenv(EnvAPIPathVersion)
	role := os.Getenv(EnvRole)
	serviceName := os.Getenv(EnvServiceName)
	insecure := os.Getenv(EnvInsecure) == "true"
	timeout := os.Getenv(EnvTimeout)

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

	if !config.ServiceName.IsNull() {
		serviceName = config.ServiceName.ValueString()
	}

	if !config.Insecure.IsNull() {
		insecure = config.Insecure.ValueBool()
	}

	if !config.Timeout.IsNull() {
		timeout = config.Timeout.ValueString()
	}

	// Apply defaults
	if apiVersion == "" {
		apiVersion = client.DefaultAPIVersion
	}

	// api_url defaults to the public commercial endpoint; GovCloud and other
	// deployments override it via the attribute or the environment variable.
	if apiUrl == "" {
		apiUrl = client.DefaultAPIURL
	}

	if timeout == "" {
		timeout = "30s"
	}

	if accessToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_token"),
			"Missing Access Token",
			"The provider requires an access token for authentication. Set the access_token attribute or "+constants.EnvAccessToken+" environment variable.",
		)
	}

	if siteId == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("site_id"),
			"Missing Site ID",
			"The provider requires a site ID for multi-tenant isolation. Set the site_id attribute or "+constants.EnvSiteID+" environment variable.",
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
		ServiceName:    serviceName,
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
		authresources.NewWorkloadIdentityResource,
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
