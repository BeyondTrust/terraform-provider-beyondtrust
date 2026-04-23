package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &FolderResource{}
	_ resource.ResourceWithImportState = &FolderResource{}
)

func NewFolderResource() resource.Resource {
	return &FolderResource{}
}

// FolderResource defines the resource implementation.
type FolderResource struct {
	client *client.Client
}

// FolderResourceModel describes the resource data model.
type FolderResourceModel struct {
	Name      types.String `tfsdk:"name"`
	Folder    types.String `tfsdk:"folder"`
	Path      types.String `tfsdk:"path"`
	ID        types.String `tfsdk:"id"`
	CreatedAt types.String `tfsdk:"created_at"`
	DeletedAt types.String `tfsdk:"deleted_at"`
	Tags      types.Map    `tfsdk:"tags"`
}

// FolderMetadataResponse represents the API response for folder metadata
type FolderMetadataResponse struct {
	ID        string            `json:"id"`
	Tags      map[string]string `json:"tags"`
	CreatedAt string            `json:"createdAt"`
	DeletedAt *string           `json:"deletedAt,omitempty"`
}

// FolderCreateResponse represents the API response when creating a folder
type FolderCreateResponse struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Metadata struct {
		ID        string            `json:"id"`
		Tags      map[string]string `json:"tags,omitempty"`
		CreatedAt string            `json:"createdAt"`
	} `json:"metadata"`
}

func (r *FolderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_folder"
}

func (r *FolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a folder in BeyondTrust Workload Credentials for organizing secrets and dynamic secrets.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the folder. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^%]+$ (max 100 chars)",
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
			"path": schema.StringAttribute{
				Description: "The full path to the folder (computed).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier (UUID) of the folder.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the folder was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deleted_at": schema.StringAttribute{
				Description: "The timestamp when the folder was soft-deleted (if applicable).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.MapAttribute{
				Description: "Key-value tags for the folder (max 50 tags, max 256 chars per value).",
				ElementType: types.StringType,
				Optional:    true,
			},
		},
	}
}

func (r *FolderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/folders/%s", name))

	// Add folder query parameter if parent folder is specified
	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildFolderQueryParam(parentFolder)

	// Create the folder
	var createResp FolderCreateResponse
	err := r.client.Post(ctx, apiPath, query, nil, &createResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Folder",
			fmt.Sprintf("Could not create folder '%s': %s", name, err.Error()),
		)
		return
	}

	// Update the model with response data
	data.ID = types.StringValue(createResp.ID)
	data.Path = types.StringValue(createResp.Path)

	// Handle tags if provided
	if !data.Tags.IsNull() && len(data.Tags.Elements()) > 0 {
		if err := r.updateTags(ctx, name, data.Folder.ValueString(), data.Tags); err != nil {
			resp.Diagnostics.AddError(
				"Error Setting Tags",
				fmt.Sprintf("Folder created but failed to set tags: %s", err.Error()),
			)
			// Don't return here - folder was created successfully
		}
	}

	// Read back the folder metadata to populate all computed fields (created_at, etc.)
	metadataPath := r.client.BuildPath(fmt.Sprintf("/folders/%s/metadata", name))
	var metadataResp FolderMetadataResponse
	err = r.client.Get(ctx, metadataPath, query, &metadataResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Created Folder",
			fmt.Sprintf("Folder created but could not read metadata: %s", err.Error()),
		)
		return
	}

	// Update state with metadata
	data.CreatedAt = types.StringValue(metadataResp.CreatedAt)
	if metadataResp.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*metadataResp.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}

	// Update tags from metadata response
	if len(metadataResp.Tags) > 0 {
		data.Tags = convertTagsToTerraformMap(metadataResp.Tags)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/folders/%s/metadata", name))

	// Add folder query parameter if parent folder is specified
	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildFolderQueryParam(parentFolder)

	// Get folder metadata
	var metadataResp FolderMetadataResponse
	err := r.client.Get(ctx, apiPath, query, &metadataResp)
	if err != nil {
		// Check if it's a 404 error using helper
		if isNotFoundError(err.Error()) {
			// Folder no longer exists, remove from state
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Folder",
			fmt.Sprintf("Could not read folder '%s': %s", name, err.Error()),
		)
		return
	}

	// Update state with response data
	data.ID = types.StringValue(metadataResp.ID)
	data.CreatedAt = types.StringValue(metadataResp.CreatedAt)

	// CRITICAL: Set deletedAt to null if not present (framework requirement)
	if metadataResp.DeletedAt != nil && *metadataResp.DeletedAt != "" {
		data.DeletedAt = types.StringValue(*metadataResp.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}

	// Update tags - set to null if empty (framework requirement)
	if len(metadataResp.Tags) > 0 {
		data.Tags = convertTagsToTerraformMap(metadataResp.Tags)
	} else {
		data.Tags = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Always update tags — passing null/empty clears them via PUT
	if err := r.updateTags(ctx, data.Name.ValueString(), data.Folder.ValueString(), data.Tags); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Tags",
			fmt.Sprintf("Could not update folder tags: %s", err.Error()),
		)
		return
	}

	// Read back the updated folder metadata
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/folders/%s/metadata", name))

	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildFolderQueryParam(parentFolder)

	var metadataResp FolderMetadataResponse
	err := r.client.Get(ctx, apiPath, query, &metadataResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Folder",
			fmt.Sprintf("Could not read folder after update: %s", err.Error()),
		)
		return
	}

	// Update tags in state - set to null if empty (framework requirement)
	if len(metadataResp.Tags) > 0 {
		data.Tags = convertTagsToTerraformMap(metadataResp.Tags)
	} else {
		data.Tags = types.MapNull(types.StringType)
	}

	// CRITICAL: Ensure deletedAt is set to null if not present (framework requirement)
	if metadataResp.DeletedAt != nil && *metadataResp.DeletedAt != "" {
		data.DeletedAt = types.StringValue(*metadataResp.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/folders/%s", name))

	// Build query parameters using helper (includes parent folder and permanent flag)
	parentFolder := ""
	if !data.Folder.IsNull() {
		parentFolder = data.Folder.ValueString()
	}
	query := buildQueryParameters(parentFolder, "delete", true)

	// Delete the folder
	err := r.client.Delete(ctx, apiPath, query)
	if err != nil {
		// Ignore 404 errors (already deleted) using helper
		if isNotFoundError(err.Error()) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Folder",
			fmt.Sprintf("Could not delete folder '%s': %s", name, err.Error()),
		)
		return
	}
}

func (r *FolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "path/to/folder" or "foldername"
	fullPath := req.ID

	// Parse the import path
	name, parentFolder := parseImportPath(fullPath)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), parentFolder)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), fullPath)...)
}

// Helper functions

func (r *FolderResource) updateTags(ctx context.Context, name string, parentFolder string, tags types.Map) error {
	resourcePath := fmt.Sprintf("/folders/%s/metadata/tags", name)
	return updateResourceTags(ctx, r.client, resourcePath, parentFolder, tags)
}
