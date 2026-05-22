package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &StaticSecretResource{}
	_ resource.ResourceWithImportState = &StaticSecretResource{}
)

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
	SecretWoVersion types.Int64  `tfsdk:"secret_wo_version"` // User-controlled trigger for rotation
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
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_static_secret"
}

func (r *StaticSecretResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a static secret in BeyondTrust Workload Credentials. The secret value is write-only and not stored in Terraform state. Use the ephemeral data source to read secret values.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the secret. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^%]+$ (max 100 chars)",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.ResourceNameValidator(),
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
				WriteOnly:   true,
			},
			"secret_wo_version": schema.Int64Attribute{
				Description: "User-controlled version number for the write-only secret. Increment this value to signal that secret_wo has changed and should be re-applied. Write-only values cannot be diffed automatically against state, so this attribute serves as the rotation trigger.",
				Required:    true,
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
	apiPath := r.client.BuildPath("/static/" + name)

	// Add folder query parameter if parent folder is specified
	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildFolderQueryParam(parentFolder)

	// Convert Terraform secret map to API format using helper
	secretMap := convertSecretMap(data.SecretWo.Elements())

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
	// Compute path from name and folder
	pathStr := buildFolderPath(name, parentFolder)
	data.Path = types.StringValue(pathStr)
	// secret_wo_version is user-controlled; preserve the user's planned value
	data.CreatedAt = types.StringValue(createResp.Metadata.CreatedAt)

	// Handle tags if provided in the create response
	if len(createResp.Metadata.Tags) > 0 {
		data.Tags = convertTagsToTerraformMap(createResp.Metadata.Tags)
	} else if !data.Tags.IsNull() && len(data.Tags.Elements()) > 0 {
		// If tags were provided in config but not in response, update them
		if err := r.updateTags(ctx, name, data.Folder.ValueString(), data.Tags); err != nil {
			resp.Diagnostics.AddError(
				"Error Setting Tags",
				"Secret created but failed to set tags: "+err.Error(),
			)
			// Don't return here - secret was created successfully
		}
	}

	// WriteOnly prevents framework persistence, but null out as defense-in-depth
	data.SecretWo = types.MapNull(types.StringType)

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
	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildFolderQueryParam(parentFolder)

	// Get secret metadata
	var metadataResp StaticSecretMetadataResponse
	err := r.client.Get(ctx, apiPath, query, &metadataResp)
	if err != nil {
		// Check if it's a 404 error using helper
		if isNotFoundError(err) {
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
	// secret_wo_version is user-controlled; preserve the value already in state

	// Compute path from name and folder using helper
	pathStr := buildFolderPath(name, parentFolder)
	data.Path = types.StringValue(pathStr)

	// Update tags - set to null if empty (framework requirement)
	if len(metadataResp.Tags) > 0 {
		data.Tags = convertTagsToTerraformMap(metadataResp.Tags)
	} else {
		data.Tags = types.MapNull(types.StringType)
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
	apiPath := r.client.BuildPath("/static/" + name)

	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildFolderQueryParam(parentFolder)

	// secret_wo is write-only and cannot be diffed against state; the user-controlled
	// secret_wo_version is the trigger for rotations. Only push the secret when it changes.
	if !data.SecretWoVersion.Equal(state.SecretWoVersion) {
		// Convert Terraform secret map to API format using helper
		stringMap := convertSecretMap(data.SecretWo.Elements())

		// Convert to map[string]interface{} for PATCH request
		secretMap := make(map[string]interface{})
		for k, v := range stringMap {
			secretMap[k] = v
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
				"Could not update secret: "+err.Error(),
			)
			return
		}
	}

	// Check if tags changed
	if !data.Tags.Equal(state.Tags) {
		if err := r.updateTags(ctx, name, data.Folder.ValueString(), data.Tags); err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Tags",
				"Could not update secret tags: "+err.Error(),
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
			"Could not read secret after update: "+err.Error(),
		)
		return
	}

	// Update state with metadata
	data.ID = types.StringValue(metadataResp.ID)
	data.CreatedAt = types.StringValue(metadataResp.CreatedAt)
	// secret_wo_version is user-controlled; preserve the user's planned value

	// Update tags in state - set to null if empty (framework requirement)
	if len(metadataResp.Tags) > 0 {
		data.Tags = convertTagsToTerraformMap(metadataResp.Tags)
	} else {
		data.Tags = types.MapNull(types.StringType)
	}

	// WriteOnly prevents framework persistence, but null out as defense-in-depth
	data.SecretWo = types.MapNull(types.StringType)

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
	apiPath := r.client.BuildPath("/static/" + name)

	// Build query parameters using helper (includes parent folder and permanent flag)
	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildQueryParameters(parentFolder, "delete", true)

	// Delete the secret
	err := r.client.Delete(ctx, apiPath, query)
	if err != nil {
		// Ignore 404 errors (already deleted) using helper
		if isNotFoundError(err) {
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

	// Parse the import path using helper
	name, parentFolder := parseImportPath(fullPath)

	if !validators.IsValidResourceName(name) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Name %q parsed from import ID contains invalid characters. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^%%]+$", name),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), fullPath)...)

	// Set folder to null when empty so it matches Optional-unset state from config
	if parentFolder != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), parentFolder)...)
	} else {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), types.StringNull())...)
	}

	// Note: Secret value must be provided in config after import
	resp.Diagnostics.AddWarning(
		"Secret Value Required",
		"After importing, you must provide the 'secret_wo' and 'secret_wo_version' attributes in your configuration. The secret value is not retrieved during import for security reasons.",
	)
}

// Helper functions

func (r *StaticSecretResource) updateTags(ctx context.Context, name string, parentFolder string, tags types.Map) error {
	resourcePath := fmt.Sprintf("/static/%s/metadata/tags", name)
	return updateResourceTags(ctx, r.client, resourcePath, parentFolder, tags)
}

// Helper functions for static secret business logic

// convertSecretMap converts a Terraform types.Map to a Go map[string]string.
// This is used when creating/updating secrets to convert from Terraform types.
func convertSecretMap(terraformMap map[string]attr.Value) map[string]string {
	result := make(map[string]string)

	for key, value := range terraformMap {
		if strVal, ok := value.(types.String); ok {
			result[key] = strVal.ValueString()
		}
	}

	return result
}

// secretMapsEqual checks if two secret maps are equal.
// Returns true if equal, false if different.
// Used in Update to detect if secret values have changed.
func secretMapsEqual(map1, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}

	for key, val1 := range map1 {
		val2, exists := map2[key]
		if !exists || val1 != val2 {
			return false
		}
	}

	return true
}
