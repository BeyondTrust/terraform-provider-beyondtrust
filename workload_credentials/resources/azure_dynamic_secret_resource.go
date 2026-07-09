package resources

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &AzureDynamicSecretResource{}
	_ resource.ResourceWithImportState = &AzureDynamicSecretResource{}
)

func NewAzureDynamicSecretResource() resource.Resource {
	return &AzureDynamicSecretResource{}
}

// AzureDynamicSecretResource defines the resource implementation.
type AzureDynamicSecretResource struct {
	client *client.Client
}

// AzureDynamicSecretResourceModel describes the resource data model.
type AzureDynamicSecretResourceModel struct {
	Name                types.String `tfsdk:"name"`
	Folder              types.String `tfsdk:"folder"`
	IntegrationName     types.String `tfsdk:"integration_name"`
	CredentialType      types.String `tfsdk:"credential_type"`
	ApplicationObjectID types.String `tfsdk:"application_object_id"`
	TTL                 types.Int64  `tfsdk:"ttl"`
	Path                types.String `tfsdk:"path"`
	ID                  types.String `tfsdk:"id"`
	IntegrationID       types.String `tfsdk:"integration_id"`
	CreatedAt           types.String `tfsdk:"created_at"`
	Version             types.Int64  `tfsdk:"version"`
	CreatedBy           types.String `tfsdk:"created_by"`
	DeletedAt           types.String `tfsdk:"deleted_at"`
}

// AzureDynamicSecretCreateRequest represents the API request for creating an Azure dynamic secret.
type AzureDynamicSecretCreateRequest struct {
	Type                string `json:"type"`
	IntegrationName     string `json:"integrationName"`
	CredentialType      string `json:"credentialType"`
	ApplicationObjectID string `json:"applicationObjectId"`
	TTL                 int64  `json:"ttl"`
}

// AzureDynamicSecretResponse represents the API response for an Azure dynamic secret.
type AzureDynamicSecretResponse struct {
	Path     string                   `json:"path"`
	Config   AzureDynamicSecretConfig `json:"config"`
	Metadata struct {
		ID        string  `json:"id"`
		Version   int     `json:"version"`
		CreatedAt string  `json:"createdAt"`
		DeletedAt *string `json:"deletedAt,omitempty"`
		CreatedBy string  `json:"createdBy,omitempty"`
	} `json:"metadata"`
}

// AzureDynamicSecretConfig is the config block returned by the API.
type AzureDynamicSecretConfig struct {
	Type                string `json:"type"`
	CredentialType      string `json:"credentialType"`
	TTL                 int64  `json:"ttl"`
	IntegrationID       string `json:"integrationId"`
	IntegrationName     string `json:"integrationName,omitempty"` // present in POST response, absent in GET
	ApplicationObjectID string `json:"applicationObjectId"`
}

// AzureDynamicSecretUpdateRequest represents the API request for updating an Azure dynamic secret.
// Fields intentionally omit `omitempty` so nil pointers marshal to JSON null,
// which under RFC 7396 merge-patch semantics deletes the field on the server.
type AzureDynamicSecretUpdateRequest struct {
	Type                string  `json:"type"`
	TTL                 *int64  `json:"ttl"`
	ApplicationObjectID *string `json:"applicationObjectId"`
}

func (r *AzureDynamicSecretResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_azure_dynamic_secret"
}

func (r *AzureDynamicSecretResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Azure dynamic secret configuration in BeyondTrust Workload Credentials. Dynamic secrets generate temporary Azure service principal password credentials on-demand with configurable TTL.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the dynamic secret. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$ (single path segment, max 130 chars).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.ResourceNameValidator(),
				},
			},
			"folder": schema.StringAttribute{
				Description: "The parent folder path (e.g., 'production' or 'production/azure'). Leave empty for root level. Each segment must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.FolderPathValidator(),
				},
			},
			"integration_name": schema.StringAttribute{
				Description: "The name of the Azure integration to use for generating credentials. Changing this requires replacing the resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential_type": schema.StringAttribute{
				Description: "The type of Azure credentials to generate. Currently supported: 'service_principal_password'. Changing this requires replacing the resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"application_object_id": schema.StringAttribute{
				Description: "The Object ID of the Azure Active Directory application registration to generate credentials for. Must be a valid UUID. Note: this is the Object ID, not the Application (client) ID.",
				Required:    true,
				Validators: []validator.String{
					validators.UUIDValidator(),
				},
			},
			"ttl": schema.Int64Attribute{
				Description: "Time-to-live in seconds for generated credentials. Valid range: 3600-86400 (1 hour to 24 hours).",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Description: "The full path to the dynamic secret (computed).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier (UUID) of the dynamic secret.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"integration_id": schema.StringAttribute{
				Description: "The UUID of the associated integration (computed from integration_name).",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the dynamic secret was created.",
				Computed:    true,
			},
			"version": schema.Int64Attribute{
				Description: "The current version of the dynamic secret.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"created_by": schema.StringAttribute{
				Description: "The ID of the user who created the dynamic secret.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deleted_at": schema.StringAttribute{
				Description: "The timestamp when the dynamic secret was soft-deleted (if applicable).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AzureDynamicSecretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *AzureDynamicSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AzureDynamicSecretResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/dynamic/" + name)

	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	createReq := AzureDynamicSecretCreateRequest{
		Type:                "azure",
		IntegrationName:     data.IntegrationName.ValueString(),
		CredentialType:      data.CredentialType.ValueString(),
		ApplicationObjectID: data.ApplicationObjectID.ValueString(),
		TTL:                 data.TTL.ValueInt64(),
	}

	var createResp AzureDynamicSecretResponse
	err := r.client.Post(ctx, apiPath, query, createReq, &createResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Azure Dynamic Secret",
			fmt.Sprintf("Could not create Azure dynamic secret '%s': %s", name, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(createResp.Metadata.ID)
	data.Path = types.StringValue(createResp.Path)
	data.CreatedAt = types.StringValue(createResp.Metadata.CreatedAt)
	data.Version = types.Int64Value(int64(createResp.Metadata.Version))
	if createResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(createResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}
	data.IntegrationID = types.StringValue(createResp.Config.IntegrationID)

	if createResp.Metadata.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*createResp.Metadata.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}

	data.TTL = types.Int64Value(createResp.Config.TTL)
	data.ApplicationObjectID = types.StringValue(createResp.Config.ApplicationObjectID)
	data.CredentialType = types.StringValue(createResp.Config.CredentialType)
	if createResp.Config.IntegrationName != "" {
		data.IntegrationName = types.StringValue(createResp.Config.IntegrationName)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureDynamicSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AzureDynamicSecretResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/dynamic/" + name)

	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	var secretResp AzureDynamicSecretResponse
	err := r.client.Get(ctx, apiPath, query, &secretResp)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Azure Dynamic Secret",
			fmt.Sprintf("Could not read Azure dynamic secret '%s': %s", name, err.Error()),
		)
		return
	}

	if secretResp.Config.Type != "azure" {
		resp.Diagnostics.AddError(
			"Unexpected Dynamic Secret Type",
			fmt.Sprintf("Dynamic secret '%s' has type %q, expected \"azure\". This resource only manages Azure dynamic secrets.", name, secretResp.Config.Type),
		)
		return
	}

	data.ID = types.StringValue(secretResp.Metadata.ID)
	data.Path = types.StringValue(secretResp.Path)
	data.CreatedAt = types.StringValue(secretResp.Metadata.CreatedAt)
	data.Version = types.Int64Value(int64(secretResp.Metadata.Version))
	if secretResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(secretResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}
	data.IntegrationID = types.StringValue(secretResp.Config.IntegrationID)

	if secretResp.Metadata.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*secretResp.Metadata.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}

	data.TTL = types.Int64Value(secretResp.Config.TTL)
	data.ApplicationObjectID = types.StringValue(secretResp.Config.ApplicationObjectID)
	data.CredentialType = types.StringValue(secretResp.Config.CredentialType)
	// integration_name is not returned by GET — preserve the state value

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureDynamicSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AzureDynamicSecretResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/dynamic/" + name)

	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	// Build update request. Fields omit `omitempty` so nil pointers marshal to
	// JSON null, which under RFC 7396 merge-patch semantics deletes the field.
	updateReq := AzureDynamicSecretUpdateRequest{
		Type: "azure",
	}

	if !data.TTL.IsNull() {
		ttl := data.TTL.ValueInt64()
		updateReq.TTL = &ttl
	}

	if !data.ApplicationObjectID.IsNull() {
		appID := data.ApplicationObjectID.ValueString()
		updateReq.ApplicationObjectID = &appID
	}

	err := r.client.Patch(ctx, apiPath, query, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Azure Dynamic Secret",
			fmt.Sprintf("Could not update Azure dynamic secret '%s': %s", name, err.Error()),
		)
		return
	}

	var secretResp AzureDynamicSecretResponse
	err = r.client.Get(ctx, apiPath, query, &secretResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Azure Dynamic Secret",
			"Could not read Azure dynamic secret after update: "+err.Error(),
		)
		return
	}

	data.ID = types.StringValue(secretResp.Metadata.ID)
	data.Path = types.StringValue(secretResp.Path)
	data.CreatedAt = types.StringValue(secretResp.Metadata.CreatedAt)
	data.Version = types.Int64Value(int64(secretResp.Metadata.Version))
	if secretResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(secretResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}
	data.IntegrationID = types.StringValue(secretResp.Config.IntegrationID)
	data.TTL = types.Int64Value(secretResp.Config.TTL)
	data.ApplicationObjectID = types.StringValue(secretResp.Config.ApplicationObjectID)
	data.CredentialType = types.StringValue(secretResp.Config.CredentialType)
	// integration_name is not returned by GET — preserve the plan value already in data

	if secretResp.Metadata.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*secretResp.Metadata.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureDynamicSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AzureDynamicSecretResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/dynamic/" + name)

	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}
	query.Set("permanent", "true")

	err := r.client.Delete(ctx, apiPath, query)
	if err != nil {
		if isNotFoundError(err) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Azure Dynamic Secret",
			fmt.Sprintf("Could not delete Azure dynamic secret '%s': %s", name, err.Error()),
		)
		return
	}
}

// validateAzureTTL validates TTL for Azure dynamic secrets.
// Valid range: 3600-86400 seconds (1 hour - 24 hours)
func validateAzureTTL(ttl int64) bool {
	return ttl >= 3600 && ttl <= 86400
}

func (r *AzureDynamicSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	colonIdx := strings.Index(req.ID, ":")
	if colonIdx < 0 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Import ID %q must be in the format \"integration-name:[folder/]secret-name\". The integration name is required because the API does not return it on read.", req.ID),
		)
		return
	}

	integrationName := req.ID[:colonIdx]
	fullPath := req.ID[colonIdx+1:]

	parts := strings.Split(fullPath, "/")
	name := parts[len(parts)-1]
	var parentFolder string

	if len(parts) > 1 {
		parentFolder = strings.Join(parts[:len(parts)-1], "/")
	}

	if !validators.IsValidResourceName(integrationName) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Integration name %q parsed from import ID is invalid. Must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$", integrationName),
		)
		return
	}
	if !validators.IsValidResourceName(name) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Secret name %q parsed from import ID is invalid. Must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$", name),
		)
		return
	}
	if !validators.IsValidFolderPath(parentFolder) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Folder %q parsed from import ID is invalid. Each segment must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$", parentFolder),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("integration_name"), integrationName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), fullPath)...)

	if parentFolder != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), parentFolder)...)
	} else {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), types.StringNull())...)
	}
}
