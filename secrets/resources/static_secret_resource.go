package resources

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &StaticSecretResource{}
var _ resource.ResourceWithImportState = &StaticSecretResource{}

func NewStaticSecretResource() resource.Resource {
	return &StaticSecretResource{}
}

// StaticSecretResource defines the resource implementation.
type StaticSecretResource struct {
	client *client.Client
}

// StaticSecretResourceModel describes the resource data model.
// NOTE: The 'secret_wo' field is write-only and never stored in state.
// Use the ephemeral resource to read secret values.
type StaticSecretResourceModel struct {
	Name            types.String `tfsdk:"name"`
	Folder          types.String `tfsdk:"folder"`
	SecretWo        types.Map    `tfsdk:"secret_wo"`         // Write-only: map[string]string
	SecretWoVersion types.Int64  `tfsdk:"secret_wo_version"` // Tracks secret changes
	Path            types.String `tfsdk:"path"`
	ID              types.String `tfsdk:"id"`
	CreatedAt       types.String `tfsdk:"created_at"`
	Tags            types.Map    `tfsdk:"tags"`
}

// StaticSecretCreateRequest represents the API request for creating a static secret
type StaticSecretCreateRequest struct {
	Secret map[string]string `json:"secret"`
}

// StaticSecretUpdateRequest represents the API request for updating a static secret
type StaticSecretUpdateRequest struct {
	Secret map[string]interface{} `json:"secret"` // Use interface{} to support null values for deletion
}

// StaticSecretMetadataResponse represents the API response for secret metadata
type StaticSecretMetadataResponse struct {
	ID        string            `json:"id"`
	Tags      map[string]string `json:"tags,omitempty"`
	Version   int64             `json:"version"`
	CreatedAt string            `json:"createdAt"`
}

// StaticSecretResponse represents the full API response (with secret value)
type StaticSecretResponse struct {
	Path     string                       `json:"path"`
	Secret   map[string]string            `json:"secret"`
	Metadata StaticSecretMetadataResponse `json:"metadata"`
}

func (r *StaticSecretResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secrets_static_secret"
}

func (r *StaticSecretResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a static secret in BeyondTrust Secrets Manager. The secret value is write-only and not stored in Terraform state. Use the ephemeral data source to read secret values.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the secret. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^%]+$ (max 100 chars)",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"folder": schema.StringAttribute{
				Description: "The parent folder path (e.g., 'production' or 'production/aws'). Leave empty for root level.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"secret_wo": schema.MapAttribute{
				Description: "Key-value pairs for the secret (e.g., {password = 'secret123'}). Write-only - not stored in state. Use the ephemeral resource to read values.",
				ElementType: types.StringType,
				Required:    true,
				Sensitive:   true,
			},
			"secret_wo_version": schema.Int64Attribute{
				Description: "Version tracker for the write-only secret. Increments when secret_wo changes. Stored in state to detect changes.",
				Computed:    true,
			},
			"path": schema.StringAttribute{
				Description: "The full path to the secret (computed).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier (UUID) of the secret.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the secret was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.MapAttribute{
				Description: "Key-value tags for the secret (max 50 tags, max 256 chars per value).",
				ElementType: types.StringType,
				Optional:    true,
			},
		},
	}
}

func (r *StaticSecretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *StaticSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data StaticSecretResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/static/%s", name))

	// Add folder query parameter if parent folder is specified
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	// Convert Terraform secret map to API format
	secretMap := make(map[string]string)
	for k, v := range data.SecretWo.Elements() {
		if strVal, ok := v.(types.String); ok {
			secretMap[k] = strVal.ValueString()
		}
	}

	// Create the secret
	requestBody := StaticSecretCreateRequest{
		Secret: secretMap,
	}

	var createResp StaticSecretResponse
	err := r.client.Post(ctx, apiPath, query, requestBody, &createResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Static Secret",
			fmt.Sprintf("Could not create secret '%s': %s", name, err.Error()),
		)
		return
	}

	// Update the model with response data (but NOT the secret value)
	data.ID = types.StringValue(createResp.Metadata.ID)
	data.Path = types.StringValue(createResp.Path)
	data.SecretWoVersion = types.Int64Value(createResp.Metadata.Version)
	data.CreatedAt = types.StringValue(createResp.Metadata.CreatedAt)

	// Handle tags if provided in the create response
	if len(createResp.Metadata.Tags) > 0 {
		tagsMap := make(map[string]attr.Value)
		for k, v := range createResp.Metadata.Tags {
			tagsMap[k] = types.StringValue(v)
		}
		data.Tags = types.MapValueMust(types.StringType, tagsMap)
	} else if !data.Tags.IsNull() && len(data.Tags.Elements()) > 0 {
		// If tags were provided in config but not in response, update them
		if err := r.updateTags(ctx, name, data.Folder.ValueString(), data.Tags); err != nil {
			resp.Diagnostics.AddError(
				"Error Setting Tags",
				fmt.Sprintf("Secret created but failed to set tags: %s", err.Error()),
			)
			// Don't return here - secret was created successfully
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StaticSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data StaticSecretResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path for metadata only (don't read the secret value)
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/static/%s/metadata", name))

	// Add folder query parameter if parent folder is specified
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	// Get secret metadata
	var metadataResp StaticSecretMetadataResponse
	err := r.client.Get(ctx, apiPath, query, &metadataResp)
	if err != nil {
		// Check if it's a 404 error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			// Secret no longer exists, remove from state
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Static Secret",
			fmt.Sprintf("Could not read secret '%s': %s", name, err.Error()),
		)
		return
	}

	// Update state with metadata (NOT the secret value)
	data.ID = types.StringValue(metadataResp.ID)
	data.CreatedAt = types.StringValue(metadataResp.CreatedAt)
	data.SecretWoVersion = types.Int64Value(metadataResp.Version)

	// Compute path from name and folder
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		data.Path = types.StringValue(fmt.Sprintf("%s/%s", data.Folder.ValueString(), name))
	} else {
		data.Path = types.StringValue(name)
	}

	// Update tags if present in response
	if len(metadataResp.Tags) > 0 {
		tagsMap := make(map[string]attr.Value)
		for k, v := range metadataResp.Tags {
			tagsMap[k] = types.StringValue(v)
		}
		data.Tags = types.MapValueMust(types.StringType, tagsMap)
	}

	// IMPORTANT: Secret value is not updated here - it remains from the plan
	// This ensures the secret can be updated if changed in config

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StaticSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data StaticSecretResourceModel
	var state StaticSecretResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/static/%s", name))

	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	// Check if secret value changed
	if !data.SecretWo.Equal(state.SecretWo) {
		// Convert Terraform secret map to API format
		secretMap := make(map[string]interface{})
		for k, v := range data.SecretWo.Elements() {
			if strVal, ok := v.(types.String); ok {
				secretMap[k] = strVal.ValueString()
			}
		}

		// Use PATCH to update the secret
		requestBody := StaticSecretUpdateRequest{
			Secret: secretMap,
		}

		var updateResp StaticSecretResponse
		err := r.client.DoRequest(ctx, "PATCH", apiPath, query, requestBody, &updateResp)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Static Secret",
				fmt.Sprintf("Could not update secret: %s", err.Error()),
			)
			return
		}

		// Update secret_wo_version from response
		data.SecretWoVersion = types.Int64Value(updateResp.Metadata.Version)
	}

	// Check if tags changed
	if !data.Tags.Equal(state.Tags) {
		if err := r.updateTags(ctx, name, data.Folder.ValueString(), data.Tags); err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Tags",
				fmt.Sprintf("Could not update secret tags: %s", err.Error()),
			)
			return
		}
	}

	// Read back the updated secret metadata
	metadataPath := r.client.BuildPath(fmt.Sprintf("/static/%s/metadata", name))

	var metadataResp StaticSecretMetadataResponse
	err := r.client.Get(ctx, metadataPath, query, &metadataResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Secret",
			fmt.Sprintf("Could not read secret after update: %s", err.Error()),
		)
		return
	}

	// Update state with metadata
	data.ID = types.StringValue(metadataResp.ID)
	data.CreatedAt = types.StringValue(metadataResp.CreatedAt)
	// secret_wo_version already updated above when secret changed

	// Update tags in state
	if len(metadataResp.Tags) > 0 {
		tagsMap := make(map[string]attr.Value)
		for k, v := range metadataResp.Tags {
			tagsMap[k] = types.StringValue(v)
		}
		data.Tags = types.MapValueMust(types.StringType, tagsMap)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StaticSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data StaticSecretResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/static/%s", name))

	// Add folder query parameter and permanent delete flag
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}
	query.Set("permanent", "true") // Permanent delete when using terraform destroy

	// Delete the secret
	err := r.client.Delete(ctx, apiPath, query)
	if err != nil {
		// Ignore 404 errors (already deleted)
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Static Secret",
			fmt.Sprintf("Could not delete secret '%s': %s", name, err.Error()),
		)
		return
	}
}

func (r *StaticSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "path/to/secret" or "secretname"
	fullPath := req.ID

	// Split the path into name and parent folder
	parts := strings.Split(fullPath, "/")
	name := parts[len(parts)-1]
	var parentFolder string

	if len(parts) > 1 {
		parentFolder = strings.Join(parts[:len(parts)-1], "/")
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), parentFolder)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), fullPath)...)

	// Note: Secret value must be provided in config after import
	resp.Diagnostics.AddWarning(
		"Secret Value Required",
		"After importing, you must provide the 'secret_wo' attribute in your configuration. The secret value is not retrieved during import for security reasons.",
	)
}

// Helper functions

func (r *StaticSecretResource) updateTags(ctx context.Context, name string, parentFolder string, tags types.Map) error {
	apiPath := r.client.BuildPath(fmt.Sprintf("/static/%s/metadata/tags", name))

	query := url.Values{}
	if parentFolder != "" {
		query.Set("folder", parentFolder)
	}

	// Convert Terraform tags to map
	tagsMap := make(map[string]string)
	for k, v := range tags.Elements() {
		if strVal, ok := v.(types.String); ok {
			tagsMap[k] = strVal.ValueString()
		}
	}

	// Use PATCH to update tags
	return r.client.Patch(ctx, apiPath, query, tagsMap)
}
