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
var _ datasource.DataSource = &AwsIntegrationDataSource{}

func NewAwsIntegrationDataSource() datasource.DataSource {
	return &AwsIntegrationDataSource{}
}

// AwsIntegrationDataSource defines the data source implementation.
type AwsIntegrationDataSource struct {
	client *client.Client
}

// AwsIntegrationDataSourceModel describes the data source data model.
type AwsIntegrationDataSourceModel struct {
	Name       types.String `tfsdk:"name"`
	RoleArn    types.String `tfsdk:"role_arn"`
	ExternalId types.String `tfsdk:"external_id"`
	ID         types.String `tfsdk:"id"`
	CreatedAt  types.String `tfsdk:"created_at"`
	Version    types.Int64  `tfsdk:"version"`
	CreatedBy  types.String `tfsdk:"created_by"`
}

// AwsIntegrationDataSourceResponse represents the API response
type AwsIntegrationDataSourceResponse struct {
	Name     string              `json:"name"`
	Type     string              `json:"type"`
	Config   AwsIntegrationConfig `json:"config"`
	Metadata struct {
		ID        string `json:"id"`
		Version   int    `json:"version"`
		CreatedAt string `json:"createdAt"`
		CreatedBy string `json:"createdBy,omitempty"`
	} `json:"metadata"`
}

type AwsIntegrationConfig struct {
	RoleArn    *string `json:"roleArn,omitempty"`
	ExternalId *string `json:"externalId,omitempty"`
}

func (d *AwsIntegrationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_aws_integration"
}

func (d *AwsIntegrationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads an existing AWS integration from BeyondTrust Workload Credentials.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the integration to look up.",
				Required:    true,
				Validators: []validator.String{
					validators.ResourceNameValidator(),
				},
			},
			"role_arn": schema.StringAttribute{
				Description: "The ARN of the IAM role in the customer AWS account.",
				Computed:    true,
			},
			"external_id": schema.StringAttribute{
				Description: "The server-generated external ID for the role trust relationship. Use in IAM role trust policy conditions.",
				Computed:    true,
				Sensitive:   true,
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

func (d *AwsIntegrationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AwsIntegrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AwsIntegrationDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := d.client.BuildPath("/integrations/aws/" + name)

	// Get integration
	var integrationResp AwsIntegrationDataSourceResponse
	err := d.client.Get(ctx, apiPath, nil, &integrationResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading AWS Integration",
			fmt.Sprintf("Could not read AWS integration '%s': %s", name, err.Error()),
		)
		return
	}

	// Validate it's an AWS integration
	if integrationResp.Type != "aws" {
		resp.Diagnostics.AddError(
			"Invalid Integration Type",
			fmt.Sprintf("Integration '%s' is of type '%s', not 'aws'", name, integrationResp.Type),
		)
		return
	}

	// Update data model with response
	data.ID = types.StringValue(integrationResp.Metadata.ID)
	data.CreatedAt = types.StringValue(integrationResp.Metadata.CreatedAt)
	data.Version = types.Int64Value(int64(integrationResp.Metadata.Version))
	if integrationResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(integrationResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}

	if integrationResp.Config.RoleArn != nil {
		data.RoleArn = types.StringValue(*integrationResp.Config.RoleArn)
	}

	if integrationResp.Config.ExternalId != nil {
		data.ExternalId = types.StringValue(*integrationResp.Config.ExternalId)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
