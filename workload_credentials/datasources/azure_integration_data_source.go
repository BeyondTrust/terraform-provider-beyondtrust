package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AzureIntegrationDataSource{}

func NewAzureIntegrationDataSource() datasource.DataSource {
	return &AzureIntegrationDataSource{}
}

// AzureIntegrationDataSource defines the data source implementation.
type AzureIntegrationDataSource struct {
	client *client.Client
}

// AzureIntegrationDataSourceModel describes the data source data model.
type AzureIntegrationDataSourceModel struct {
	Name      types.String `tfsdk:"name"`
	TenantID  types.String `tfsdk:"tenant_id"`
	ClientID  types.String `tfsdk:"client_id"`
	ID        types.String `tfsdk:"id"`
	CreatedAt types.String `tfsdk:"created_at"`
	Version   types.Int64  `tfsdk:"version"`
	CreatedBy types.String `tfsdk:"created_by"`
}

// AzureIntegrationDataSourceResponse represents the API response.
// clientSecret is intentionally absent — the API never returns it.
type AzureIntegrationDataSourceResponse struct {
	Name     string                           `json:"name"`
	Type     string                           `json:"type"`
	Config   AzureIntegrationDataSourceConfig `json:"config"`
	Metadata struct {
		ID        string `json:"id"`
		Version   int    `json:"version"`
		CreatedAt string `json:"createdAt"`
		CreatedBy string `json:"createdBy,omitempty"`
	} `json:"metadata"`
}

// AzureIntegrationDataSourceConfig is the config block returned by the API.
type AzureIntegrationDataSourceConfig struct {
	TenantID *string `json:"tenantId,omitempty"`
	ClientID *string `json:"clientId,omitempty"`
}

func (d *AzureIntegrationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_azure_integration"
}

func (d *AzureIntegrationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads an existing Azure integration from BeyondTrust Workload Credentials. The client secret is not returned by the API.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the integration to look up.",
				Required:    true,
				Validators: []validator.String{
					validators.ResourceNameValidator(),
				},
			},
			"tenant_id": schema.StringAttribute{
				Description: "The Azure Active Directory tenant ID (directory ID).",
				Computed:    true,
			},
			"client_id": schema.StringAttribute{
				Description: "The Azure Active Directory application (client) ID.",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier (UUID) of the integration.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the integration was created.",
				Computed:    true,
			},
			"version": schema.Int64Attribute{
				Description: "The current version of the integration.",
				Computed:    true,
			},
			"created_by": schema.StringAttribute{
				Description: "The ID of the user who created the integration.",
				Computed:    true,
			},
		},
	}
}

func (d *AzureIntegrationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *AzureIntegrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AzureIntegrationDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := d.client.BuildPath("/integrations/azure/" + name)

	var integrationResp AzureIntegrationDataSourceResponse
	err := d.client.Get(ctx, apiPath, nil, &integrationResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Azure Integration",
			fmt.Sprintf("Could not read Azure integration '%s': %s", name, err.Error()),
		)
		return
	}

	if integrationResp.Type != "azure" {
		resp.Diagnostics.AddError(
			"Invalid Integration Type",
			fmt.Sprintf("Integration '%s' is of type '%s', not 'azure'", name, integrationResp.Type),
		)
		return
	}

	data.ID = types.StringValue(integrationResp.Metadata.ID)
	data.CreatedAt = types.StringValue(integrationResp.Metadata.CreatedAt)
	data.Version = types.Int64Value(int64(integrationResp.Metadata.Version))
	if integrationResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(integrationResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}

	if integrationResp.Config.TenantID != nil {
		data.TenantID = types.StringValue(*integrationResp.Config.TenantID)
	}
	if integrationResp.Config.ClientID != nil {
		data.ClientID = types.StringValue(*integrationResp.Config.ClientID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
