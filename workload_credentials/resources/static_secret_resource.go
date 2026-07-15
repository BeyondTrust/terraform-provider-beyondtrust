package resources

import (
	"context"
	"errors"
	"fmt"
	"sort"

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
	DeletedAt       types.String `tfsdk:"deleted_at"`
	Version         types.Int64  `tfsdk:"version"`
	CreatedBy       types.String `tfsdk:"created_by"`
	Tags            types.Map    `tfsdk:"tags"`
}

// StaticSecretCreateRequest represents the API request for creating a static secret
type StaticSecretCreateRequest struct {
	Secret map[string]string `json:"secret"`
}

// StaticSecretUpdateRequest represents the API request for updating a static secret.
// The Secret map uses interface{} values so callers can emit explicit JSON null
// for keys removed from configuration: under RFC 7396 merge-patch semantics
// (which the PATCH endpoint uses) an omitted key means "leave unchanged" while
// null means "delete". The Update flow populates the map via buildSecretMergePatch.
type StaticSecretUpdateRequest struct {
	Secret map[string]interface{} `json:"secret"`
}

// StaticSecretMetadataResponse represents the API response for secret metadata
type StaticSecretMetadataResponse struct {
	ID        string            `json:"id"`
	Tags      map[string]string `json:"tags,omitempty"`
	Version   int64             `json:"version"`
	CreatedAt string            `json:"createdAt"`
	DeletedAt *string           `json:"deletedAt,omitempty"`
	CreatedBy string            `json:"createdBy,omitempty"`
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
		Description: "Manages a static secret in BeyondTrust Workload Credentials. The secret value is write-only and not stored in Terraform state. Use the ephemeral resource to read secret values.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the secret. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$ (single path segment, max 130 chars).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.ResourceNameValidator(),
				},
			},
			"folder": schema.StringAttribute{
				Description: "The parent folder path (e.g., 'production' or 'production/aws'). Leave empty for root level. Each segment must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.FolderPathValidator(),
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
			"deleted_at": schema.StringAttribute{
				Description: "The timestamp when the secret was soft-deleted (if applicable).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"version": schema.Int64Attribute{
				Description: "The current version number of the secret.",
				Computed:    true,
			},
			"created_by": schema.StringAttribute{
				Description: "The ID of the user who created the secret.",
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

	// Write-only attributes are nullified in req.Plan by the framework (they can't be
	// stored in state). Read secret_wo from req.Config where the actual value lives.
	var configData StaticSecretResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
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

	// Guard against null/unknown secret_wo - would create an empty secret
	if configData.SecretWo.IsNull() || configData.SecretWo.IsUnknown() {
		resp.Diagnostics.AddError(
			"Missing Secret Value",
			"secret_wo is required but was null or unknown. This should not happen - please report this as a provider bug.",
		)
		return
	}

	// Validate that all map values are known, non-null strings
	// Without this check, unknown/null values would be converted to empty strings,
	// potentially creating secrets with blank values
	if err := validateSecretMapValues(configData.SecretWo.Elements()); err != nil {
		resp.Diagnostics.AddError(
			"Invalid Secret Values",
			err.Error(),
		)
		return
	}

	// Convert Terraform secret map to API format using helper
	// Use configData.SecretWo instead of data.SecretWo because write-only values are only in Config
	secretMap := convertSecretMap(configData.SecretWo.Elements())

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
	if createResp.Metadata.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*createResp.Metadata.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}
	data.Version = types.Int64Value(createResp.Metadata.Version)
	if createResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(createResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}

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

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/static/%s/metadata", name))

	// Add folder query parameter if parent folder is specified
	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildFolderQueryParam(parentFolder)

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
	if metadataResp.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*metadataResp.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}
	data.Version = types.Int64Value(metadataResp.Version)
	if metadataResp.CreatedBy != "" {
		data.CreatedBy = types.StringValue(metadataResp.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}
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

	// Write-only attributes are nullified in req.Plan by the framework.
	// Read secret_wo from req.Config where the actual value lives.
	var configData StaticSecretResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
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
		// Fetch the current secret so we know which keys exist on the server.
		// secret_wo is write-only and not persisted in state, so the server is
		// the only source of truth for the prior key set. We need the prior keys
		// to emit explicit JSON null for any key removed from configuration —
		// under RFC 7396 merge-patch semantics, an omitted key means "leave
		// unchanged" while null means "delete".
		var currentSecret StaticSecretResponse
		if err := r.client.Get(ctx, apiPath, query, &currentSecret); err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Current Secret",
				"Could not read current secret for key comparison: "+err.Error(),
			)
			return
		}

		// Guard against null/unknown secret_wo when version changed
		// If the version changed, the user is signaling they want to update the secret,
		// so secret_wo must be present. If it's null/unknown, calling Elements() would
		// yield an empty map and the merge-patch would delete all existing keys.
		if configData.SecretWo.IsNull() || configData.SecretWo.IsUnknown() {
			resp.Diagnostics.AddError(
				"Missing Secret Value",
				"secret_wo_version was incremented but secret_wo is null or unknown. "+
					"When rotating secrets (incrementing secret_wo_version), you must provide the new secret_wo value.",
			)
			return
		}

		// Validate that all map values are known, non-null strings
		// Critical for merge-patch: unknown/null values would convert to empty strings
		// and unintentionally blank out keys or rotate to empty values
		if err := validateSecretMapValues(configData.SecretWo.Elements()); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Secret Values",
				err.Error(),
			)
			return
		}

		// Build the merge-patch body: new/updated keys with values, removed keys with null.
		// Use configData.SecretWo instead of data.SecretWo because write-only values are only in Config
		newSecret := convertSecretMap(configData.SecretWo.Elements())
		requestBody := StaticSecretUpdateRequest{
			Secret: buildSecretMergePatch(currentSecret.Secret, newSecret),
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

	// Read back the updated secret via metadata endpoint
	refreshPath := r.client.BuildPath(fmt.Sprintf("/static/%s/metadata", name))
	refreshQuery := buildFolderQueryParam(parentFolder)

	var metadataResp StaticSecretMetadataResponse
	err := r.client.Get(ctx, refreshPath, refreshQuery, &metadataResp)
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
	if metadataResp.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*metadataResp.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}
	data.Version = types.Int64Value(metadataResp.Version)
	if metadataResp.CreatedBy != "" {
		data.CreatedBy = types.StringValue(metadataResp.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}
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
			fmt.Sprintf("Name %q parsed from import ID is invalid. Must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$", name),
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

// validateSecretMapValues checks that all values in a secret_wo map are known, non-null strings.
// Returns an error listing any keys with null/unknown values.
func validateSecretMapValues(terraformMap map[string]attr.Value) error {
	var nullKeys, unknownKeys []string

	for key, value := range terraformMap {
		strVal, ok := value.(types.String)
		if !ok {
			// Not a string type at all - should not happen with schema validation
			continue
		}

		if strVal.IsNull() {
			nullKeys = append(nullKeys, key)
		} else if strVal.IsUnknown() {
			unknownKeys = append(unknownKeys, key)
		}
	}

	if len(nullKeys) > 0 || len(unknownKeys) > 0 {
		var errMsg string
		if len(nullKeys) > 0 {
			sort.Strings(nullKeys)
			errMsg += fmt.Sprintf("The following secret_wo keys have null values: %v. ", nullKeys)
		}
		if len(unknownKeys) > 0 {
			sort.Strings(unknownKeys)
			errMsg += fmt.Sprintf("The following secret_wo keys have unknown values: %v. ", unknownKeys)
		}
		errMsg += "All secret values must be known, non-null strings."
		return errors.New(errMsg)
	}

	return nil
}

// convertSecretMap converts a Terraform types.Map to a Go map[string]string.
// This is used when creating/updating secrets to convert from Terraform types.
// Assumes all values have been validated via validateSecretMapValues.
func convertSecretMap(terraformMap map[string]attr.Value) map[string]string {
	result := make(map[string]string)

	for key, value := range terraformMap {
		if strVal, ok := value.(types.String); ok {
			result[key] = strVal.ValueString()
		}
	}

	return result
}

// buildSecretMergePatch builds a merge-patch body for static secret updates.
// It returns a map where:
//   - Keys present in newSecret carry their string value
//   - Keys present in oldSecret but absent from newSecret carry nil
//     (per RFC 7396 merge-patch semantics, JSON null means "delete this key")
//
// The PATCH endpoint sends this map as application/merge-patch+json, so a key
// that is missing from the body is left unchanged on the server. Without the
// explicit nil entries, keys removed from configuration would silently persist.
func buildSecretMergePatch(oldSecret, newSecret map[string]string) map[string]interface{} {
	patch := make(map[string]interface{}, len(newSecret)+len(oldSecret))

	for k, v := range newSecret {
		patch[k] = v
	}

	for k := range oldSecret {
		if _, exists := newSecret[k]; !exists {
			patch[k] = nil
		}
	}

	return patch
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
