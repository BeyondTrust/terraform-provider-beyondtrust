package ephemeral

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ ephemeral.EphemeralResourceWithConfigure = &StaticSecretEphemeral{}

func NewStaticSecretEphemeral() ephemeral.EphemeralResource {
	return &StaticSecretEphemeral{}
}

// StaticSecretEphemeral defines the ephemeral resource implementation.
type StaticSecretEphemeral struct {
	client *client.Client
}

// StaticSecretEphemeralModel describes the ephemeral resource data model.
type StaticSecretEphemeralModel struct {
	Name      types.String `tfsdk:"name"`
	Folder    types.String `tfsdk:"folder"`
	Version   types.Int64  `tfsdk:"version"`
	Secret    types.Map    `tfsdk:"secret"` // Ephemeral: map[string]string - NOT stored in state
	Path      types.String `tfsdk:"path"`
	ID        types.String `tfsdk:"id"`
	CreatedAt types.String `tfsdk:"created_at"`
	Tags      types.Map    `tfsdk:"tags"`
}

// StaticSecretResponse represents the API response for a static secret
type StaticSecretResponse struct {
	Path     string            `json:"path"`
	Secret   map[string]string `json:"secret"`
	Metadata struct {
		ID        string            `json:"id"`
		Tags      map[string]string `json:"tags,omitempty"`
		Version   int64             `json:"version"`
		CreatedAt string            `json:"createdAt"`
	} `json:"metadata"`
}

func (e *StaticSecretEphemeral) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_static_secret"
}

func (e *StaticSecretEphemeral) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Ephemeral resource for reading static secret values from BeyondTrust Workload Credentials. Secret values are never stored in Terraform state or plan files.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the secret to read.",
				Required:    true,
				Validators: []validator.String{
					validators.ResourceNameValidator(),
				},
			},
			"folder": schema.StringAttribute{
				Description: "The parent folder path (e.g., 'production' or 'production/aws'). Leave empty for root level. Each segment must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$.",
				Optional:    true,
				Validators: []validator.String{
					validators.FolderPathValidator(),
				},
			},
			"version": schema.Int64Attribute{
				Description: "The specific version of the secret to read. If not provided, reads the latest version. After reading, this will be set to the actual version that was read.",
				Optional:    true,
				Computed:    true,
			},
			"secret": schema.MapAttribute{
				Description: "Key-value pairs of the secret (ephemeral - never stored in state or plan).",
				ElementType: types.StringType,
				Computed:    true,
				Sensitive:   true,
			},
			"path": schema.StringAttribute{
				Description: "The full path to the secret.",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier (UUID) of the secret.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the secret was created.",
				Computed:    true,
			},
			"tags": schema.MapAttribute{
				Description: "Key-value tags for the secret.",
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

func (e *StaticSecretEphemeral) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Ephemeral Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	e.client = client
}

func (e *StaticSecretEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data StaticSecretEphemeralModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := e.client.BuildPath("/static/" + name)

	// Add query parameters
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}
	if !data.Version.IsNull() {
		query.Set("version", strconv.FormatInt(data.Version.ValueInt64(), 10))
	}

	// Get the secret (includes the secret value)
	var secretResp StaticSecretResponse
	err := e.client.Get(ctx, apiPath, query, &secretResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Static Secret",
			fmt.Sprintf("Could not read secret '%s': %s", name, err.Error()),
		)
		return
	}

	// Populate the model with response data
	data.Path = types.StringValue(secretResp.Path)
	data.ID = types.StringValue(secretResp.Metadata.ID)
	data.Version = types.Int64Value(secretResp.Metadata.Version)
	data.CreatedAt = types.StringValue(secretResp.Metadata.CreatedAt)

	// Convert secret map to Terraform map
	secretMap := make(map[string]attr.Value)
	for k, v := range secretResp.Secret {
		secretMap[k] = types.StringValue(v)
	}
	data.Secret = types.MapValueMust(types.StringType, secretMap)

	// Convert tags if present
	if len(secretResp.Metadata.Tags) > 0 {
		tagsMap := make(map[string]attr.Value)
		for k, v := range secretResp.Metadata.Tags {
			tagsMap[k] = types.StringValue(v)
		}
		data.Tags = types.MapValueMust(types.StringType, tagsMap)
	}

	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
