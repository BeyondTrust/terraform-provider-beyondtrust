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
var _ resource.Resource = &FolderResource{}
var _ resource.ResourceWithImportState = &FolderResource{}

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
	resp.TypeName = req.ProviderTypeName + "_secrets_folder"
}

func (r *FolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a folder in BeyondTrust Secrets Manager for organizing secrets and dynamic secrets.",

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
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

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

	// Set created_at if provided by API
	if createResp.Metadata.CreatedAt != "" {
		data.CreatedAt = types.StringValue(createResp.Metadata.CreatedAt)
	}

	// DeletedAt should be null for newly created folders
	data.DeletedAt = types.StringNull()

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
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	// Get folder metadata
	var metadataResp FolderMetadataResponse
	err := r.client.Get(ctx, apiPath, query, &metadataResp)
	if err != nil {
		// Check if it's a 404 error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
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

	if metadataResp.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*metadataResp.DeletedAt)
	}

	// Update tags if present in response
	if len(metadataResp.Tags) > 0 {
		tagsMap := make(map[string]attr.Value)
		for k, v := range metadataResp.Tags {
			tagsMap[k] = types.StringValue(v)
		}
		data.Tags = types.MapValueMust(types.StringType, tagsMap)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Folders can only have tags updated (name and folder are ForceNew)
	if !data.Tags.IsNull() {
		if err := r.updateTags(ctx, data.Name.ValueString(), data.Folder.ValueString(), data.Tags); err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Tags",
				fmt.Sprintf("Could not update folder tags: %s", err.Error()),
			)
			return
		}
	}

	// Read back the updated folder metadata
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/folders/%s/metadata", name))

	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	var metadataResp FolderMetadataResponse
	err := r.client.Get(ctx, apiPath, query, &metadataResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Folder",
			fmt.Sprintf("Could not read folder after update: %s", err.Error()),
		)
		return
	}

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

func (r *FolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/folders/%s", name))

	// Add folder query parameter and permanent delete flag
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}
	query.Set("permanent", "true") // Permanent delete when using terraform destroy

	// Delete the folder
	err := r.client.Delete(ctx, apiPath, query)
	if err != nil {
		// Ignore 404 errors (already deleted)
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
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
}

// Helper functions

func (r *FolderResource) updateTags(ctx context.Context, name string, parentFolder string, tags types.Map) error {
	apiPath := r.client.BuildPath(fmt.Sprintf("/folders/%s/metadata/tags", name))

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
