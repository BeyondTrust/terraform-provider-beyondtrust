package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &AwsDynamicSecretResource{}
	_ resource.ResourceWithImportState = &AwsDynamicSecretResource{}
)

func NewAwsDynamicSecretResource() resource.Resource {
	return &AwsDynamicSecretResource{}
}

// AwsDynamicSecretResource defines the resource implementation.
type AwsDynamicSecretResource struct {
	client *client.Client
}

// AwsDynamicSecretResourceModel describes the resource data model.
type AwsDynamicSecretResourceModel struct {
	Name            types.String `tfsdk:"name"`
	Folder          types.String `tfsdk:"folder"`
	IntegrationName types.String `tfsdk:"integration_name"`
	CredentialType  types.String `tfsdk:"credential_type"`
	RoleArn         types.String `tfsdk:"role_arn"`
	ExternalId      types.String `tfsdk:"external_id"`
	TTL             types.Int64  `tfsdk:"ttl"`
	PolicyArns      types.List   `tfsdk:"policy_arns"`
	Policy          types.String `tfsdk:"policy"`
	Groups          types.List   `tfsdk:"groups"`
	AwsTags         types.Map    `tfsdk:"aws_tags"`
	Path            types.String `tfsdk:"path"`
	ID              types.String `tfsdk:"id"`
	IntegrationId   types.String `tfsdk:"integration_id"`
	CreatedAt       types.String `tfsdk:"created_at"`
	DeletedAt       types.String `tfsdk:"deleted_at"`
}

// AwsDynamicSecretCreateRequest represents the API request for creating a dynamic secret
type AwsDynamicSecretCreateRequest struct {
	Type            string              `json:"type"`
	IntegrationName string              `json:"integrationName"`
	CredentialType  string              `json:"credentialType"`
	RoleArn         string              `json:"roleArn"`
	ExternalId      *string             `json:"externalId,omitempty"`
	TTL             int64               `json:"ttl"`
	PolicyArns      *[]string           `json:"policyArns,omitempty"`
	Policy          *string             `json:"policy,omitempty"`
	Groups          *[]string           `json:"groups,omitempty"`
	AwsTags         *map[string]*string `json:"awsTags,omitempty"`
}

// AwsDynamicSecretResponse represents the API response for a dynamic secret
type AwsDynamicSecretResponse struct {
	Path     string              `json:"path"`
	Config   DynamicSecretConfig `json:"config"`
	Metadata struct {
		ID        string  `json:"id"`
		Version   int     `json:"version"`
		CreatedAt string  `json:"createdAt"`
		DeletedAt *string `json:"deletedAt,omitempty"`
	} `json:"metadata"`
}

type DynamicSecretConfig struct {
	Type            string              `json:"type"`
	CredentialType  string              `json:"credentialType"`
	TTL             int64               `json:"ttl"`
	IntegrationId   string              `json:"integrationId"`
	IntegrationName string              `json:"integrationName"`
	RoleArn         string              `json:"roleArn"`
	ExternalId      *string             `json:"externalId,omitempty"`
	PolicyArns      *[]string           `json:"policyArns,omitempty"`
	Policy          *string             `json:"policy,omitempty"`
	Groups          *[]string           `json:"groups,omitempty"`
	AwsTags         *map[string]*string `json:"awsTags,omitempty"`
}

// AwsDynamicSecretUpdateRequest represents the API request for updating a dynamic secret
type AwsDynamicSecretUpdateRequest struct {
	Type       string              `json:"type"`
	RoleArn    *string             `json:"roleArn,omitempty"`
	ExternalId *string             `json:"externalId,omitempty"`
	TTL        *int64              `json:"ttl,omitempty"`
	PolicyArns *[]string           `json:"policyArns,omitempty"`
	Policy     *string             `json:"policy,omitempty"`
	Groups     *[]string           `json:"groups,omitempty"`
	AwsTags    *map[string]*string `json:"awsTags,omitempty"`
}

func (r *AwsDynamicSecretResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secrets_aws_dynamic_secret"
}

func (r *AwsDynamicSecretResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AWS dynamic secret configuration in BeyondTrust Secrets Manager. Dynamic secrets generate temporary AWS credentials on-demand with configurable TTL and permissions.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the dynamic secret. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^%]+$ (max 100 chars).",
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
			"integration_name": schema.StringAttribute{
				Description: "The name of the AWS integration to use for generating credentials.",
				Required:    true,
			},
			"credential_type": schema.StringAttribute{
				Description: "The type of AWS credentials to generate. Currently supported: 'assumed_role'. Other types (iam_user, federation_token, session_token) may be added in the future.",
				Required:    true,
			},
			"role_arn": schema.StringAttribute{
				Description: "The ARN of the AWS IAM role to assume when generating credentials. Must match pattern: arn:aws:iam::[0-9]+:role/.+",
				Required:    true,
			},
			"external_id": schema.StringAttribute{
				Description: "Optional external ID for the role assumption. Used for additional security in role trust relationships.",
				Optional:    true,
				Sensitive:   true,
			},
			"ttl": schema.Int64Attribute{
				Description: "Time-to-live in seconds for generated credentials. For assumed_role: 900-43200 (15 min - 12 hours). For other types: 900-129600 (15 min - 36 hours).",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"policy_arns": schema.ListAttribute{
				Description: "List of AWS managed policy ARNs to attach to the assumed role session. Each must match pattern: arn:aws:iam::.*:policy/.*",
				ElementType: types.StringType,
				Optional:    true,
			},
			"policy": schema.StringAttribute{
				Description: "Inline IAM policy document (JSON) to apply to the assumed role session. Allows fine-grained permissions control.",
				Optional:    true,
			},
			"groups": schema.ListAttribute{
				Description: "List of IAM group names whose policies will be merged and applied to the session. Groups must be in the target AWS account. Each group name: 1-128 chars, alphanumeric + _+=,.@-",
				ElementType: types.StringType,
				Optional:    true,
			},
			"aws_tags": schema.MapAttribute{
				Description: "Key-value tags to apply to the AWS session. Keys: 1-128 chars, Values: 0-256 chars.",
				ElementType: types.StringType,
				Optional:    true,
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
			"deleted_at": schema.StringAttribute{
				Description: "The timestamp when the dynamic secret was soft-deleted (if applicable).",
				Computed:    true,
			},
		},
	}
}

func (r *AwsDynamicSecretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AwsDynamicSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AwsDynamicSecretResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/dynamic/%s", name))

	// Add folder query parameter if specified
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	// Build request body
	createReq := AwsDynamicSecretCreateRequest{
		Type:            "aws",
		IntegrationName: data.IntegrationName.ValueString(),
		CredentialType:  data.CredentialType.ValueString(),
		RoleArn:         data.RoleArn.ValueString(),
		TTL:             data.TTL.ValueInt64(),
	}

	// Optional fields
	if !data.ExternalId.IsNull() && data.ExternalId.ValueString() != "" {
		externalId := data.ExternalId.ValueString()
		createReq.ExternalId = &externalId
	}

	if !data.PolicyArns.IsNull() && len(data.PolicyArns.Elements()) > 0 {
		policyArns := make([]string, 0, len(data.PolicyArns.Elements()))
		for _, elem := range data.PolicyArns.Elements() {
			if strVal, ok := elem.(types.String); ok {
				policyArns = append(policyArns, strVal.ValueString())
			}
		}
		createReq.PolicyArns = &policyArns
	}

	if !data.Policy.IsNull() && data.Policy.ValueString() != "" {
		policy := data.Policy.ValueString()
		createReq.Policy = &policy
	}

	if !data.Groups.IsNull() && len(data.Groups.Elements()) > 0 {
		groups := make([]string, 0, len(data.Groups.Elements()))
		for _, elem := range data.Groups.Elements() {
			if strVal, ok := elem.(types.String); ok {
				groups = append(groups, strVal.ValueString())
			}
		}
		createReq.Groups = &groups
	}

	if !data.AwsTags.IsNull() && len(data.AwsTags.Elements()) > 0 {
		awsTags := make(map[string]*string)
		for k, v := range data.AwsTags.Elements() {
			if strVal, ok := v.(types.String); ok {
				val := strVal.ValueString()
				awsTags[k] = &val
			}
		}
		createReq.AwsTags = &awsTags
	}

	// Create the dynamic secret
	var createResp AwsDynamicSecretResponse
	err := r.client.Post(ctx, apiPath, query, createReq, &createResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating AWS Dynamic Secret",
			fmt.Sprintf("Could not create AWS dynamic secret '%s': %s", name, err.Error()),
		)
		return
	}

	// Update the model with response data
	data.ID = types.StringValue(createResp.Metadata.ID)
	data.Path = types.StringValue(createResp.Path)
	data.CreatedAt = types.StringValue(createResp.Metadata.CreatedAt)
	data.IntegrationId = types.StringValue(createResp.Config.IntegrationId)

	if createResp.Metadata.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*createResp.Metadata.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}

	// Update with normalized values from API
	data.TTL = types.Int64Value(createResp.Config.TTL)
	data.RoleArn = types.StringValue(createResp.Config.RoleArn)
	data.CredentialType = types.StringValue(createResp.Config.CredentialType)
	data.IntegrationName = types.StringValue(createResp.Config.IntegrationName)

	if createResp.Config.ExternalId != nil {
		data.ExternalId = types.StringValue(*createResp.Config.ExternalId)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AwsDynamicSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AwsDynamicSecretResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/dynamic/%s", name))

	// Add folder query parameter if specified
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	// Get dynamic secret
	var secretResp AwsDynamicSecretResponse
	err := r.client.Get(ctx, apiPath, query, &secretResp)
	if err != nil {
		// Check if it's a 404 error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			// Dynamic secret no longer exists, remove from state
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading AWS Dynamic Secret",
			fmt.Sprintf("Could not read AWS dynamic secret '%s': %s", name, err.Error()),
		)
		return
	}

	// Update state with response data
	data.ID = types.StringValue(secretResp.Metadata.ID)
	data.Path = types.StringValue(secretResp.Path)
	data.CreatedAt = types.StringValue(secretResp.Metadata.CreatedAt)
	data.IntegrationId = types.StringValue(secretResp.Config.IntegrationId)

	if secretResp.Metadata.DeletedAt != nil {
		data.DeletedAt = types.StringValue(*secretResp.Metadata.DeletedAt)
	} else {
		data.DeletedAt = types.StringNull()
	}

	// Update config values
	data.TTL = types.Int64Value(secretResp.Config.TTL)
	data.RoleArn = types.StringValue(secretResp.Config.RoleArn)
	data.CredentialType = types.StringValue(secretResp.Config.CredentialType)
	data.IntegrationName = types.StringValue(secretResp.Config.IntegrationName)

	if secretResp.Config.ExternalId != nil {
		data.ExternalId = types.StringValue(*secretResp.Config.ExternalId)
	}

	// Update lists and maps
	if secretResp.Config.PolicyArns != nil && len(*secretResp.Config.PolicyArns) > 0 {
		policyArnsElements := make([]attr.Value, 0, len(*secretResp.Config.PolicyArns))
		for _, arn := range *secretResp.Config.PolicyArns {
			policyArnsElements = append(policyArnsElements, types.StringValue(arn))
		}
		data.PolicyArns = types.ListValueMust(types.StringType, policyArnsElements)
	}

	if secretResp.Config.Policy != nil {
		data.Policy = types.StringValue(*secretResp.Config.Policy)
	}

	if secretResp.Config.Groups != nil && len(*secretResp.Config.Groups) > 0 {
		groupsElements := make([]attr.Value, 0, len(*secretResp.Config.Groups))
		for _, group := range *secretResp.Config.Groups {
			groupsElements = append(groupsElements, types.StringValue(group))
		}
		data.Groups = types.ListValueMust(types.StringType, groupsElements)
	}

	if secretResp.Config.AwsTags != nil && len(*secretResp.Config.AwsTags) > 0 {
		tagsMap := make(map[string]attr.Value)
		for k, v := range *secretResp.Config.AwsTags {
			if v != nil {
				tagsMap[k] = types.StringValue(*v)
			}
		}
		data.AwsTags = types.MapValueMust(types.StringType, tagsMap)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AwsDynamicSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AwsDynamicSecretResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/dynamic/%s", name))

	// Add folder query parameter if specified
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}

	// Build update request with merge patch semantics
	updateReq := AwsDynamicSecretUpdateRequest{
		Type: "aws",
	}

	// Include changed fields
	if !data.RoleArn.IsNull() {
		roleArn := data.RoleArn.ValueString()
		updateReq.RoleArn = &roleArn
	}

	if !data.ExternalId.IsNull() {
		externalId := data.ExternalId.ValueString()
		updateReq.ExternalId = &externalId
	}

	if !data.TTL.IsNull() {
		ttl := data.TTL.ValueInt64()
		updateReq.TTL = &ttl
	}

	if !data.PolicyArns.IsNull() {
		policyArns := make([]string, 0, len(data.PolicyArns.Elements()))
		for _, elem := range data.PolicyArns.Elements() {
			if strVal, ok := elem.(types.String); ok {
				policyArns = append(policyArns, strVal.ValueString())
			}
		}
		updateReq.PolicyArns = &policyArns
	}

	if !data.Policy.IsNull() {
		policy := data.Policy.ValueString()
		updateReq.Policy = &policy
	}

	if !data.Groups.IsNull() {
		groups := make([]string, 0, len(data.Groups.Elements()))
		for _, elem := range data.Groups.Elements() {
			if strVal, ok := elem.(types.String); ok {
				groups = append(groups, strVal.ValueString())
			}
		}
		updateReq.Groups = &groups
	}

	if !data.AwsTags.IsNull() {
		awsTags := make(map[string]*string)
		for k, v := range data.AwsTags.Elements() {
			if strVal, ok := v.(types.String); ok {
				val := strVal.ValueString()
				awsTags[k] = &val
			}
		}
		updateReq.AwsTags = &awsTags
	}

	// Update the dynamic secret
	err := r.client.Patch(ctx, apiPath, query, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating AWS Dynamic Secret",
			fmt.Sprintf("Could not update AWS dynamic secret '%s': %s", name, err.Error()),
		)
		return
	}

	// Read back the updated dynamic secret
	var secretResp AwsDynamicSecretResponse
	err = r.client.Get(ctx, apiPath, query, &secretResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated AWS Dynamic Secret",
			fmt.Sprintf("Could not read AWS dynamic secret after update: %s", err.Error()),
		)
		return
	}

	// Update state with latest values
	data.ID = types.StringValue(secretResp.Metadata.ID)
	data.Path = types.StringValue(secretResp.Path)
	data.CreatedAt = types.StringValue(secretResp.Metadata.CreatedAt)
	data.IntegrationId = types.StringValue(secretResp.Config.IntegrationId)
	data.TTL = types.Int64Value(secretResp.Config.TTL)
	data.RoleArn = types.StringValue(secretResp.Config.RoleArn)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AwsDynamicSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AwsDynamicSecretResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath(fmt.Sprintf("/dynamic/%s", name))

	// Add folder query parameter and permanent delete flag
	query := url.Values{}
	if !data.Folder.IsNull() && data.Folder.ValueString() != "" {
		query.Set("folder", data.Folder.ValueString())
	}
	query.Set("permanent", "true") // Permanent delete for Terraform destroy

	// Delete the dynamic secret
	err := r.client.Delete(ctx, apiPath, query)
	if err != nil {
		// Ignore 404 errors (already deleted)
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting AWS Dynamic Secret",
			fmt.Sprintf("Could not delete AWS dynamic secret '%s': %s", name, err.Error()),
		)
		return
	}
}

func (r *AwsDynamicSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "path/to/dynamic-secret" or "secret-name"
	fullPath := req.ID

	// Split the path into name and parent folder
	parts := strings.Split(fullPath, "/")
	name := parts[len(parts)-1]
	var parentFolder string

	if len(parts) > 1 {
		parentFolder = strings.Join(parts[:len(parts)-1], "/")
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), fullPath)...)

	// Set folder to null when empty so it matches Optional-unset state from config
	if parentFolder != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), parentFolder)...)
	} else {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), types.StringNull())...)
	}
}

// Helper functions for AWS dynamic secret business logic

// validateJSONPolicy validates AWS IAM policy JSON format
func validateJSONPolicy(policy string) error {
	if policy == "" {
		return fmt.Errorf("policy cannot be empty")
	}

	var js map[string]interface{}
	err := json.Unmarshal([]byte(policy), &js)
	if err != nil {
		return err
	}

	// Must be an object, not array
	if js == nil {
		return fmt.Errorf("policy must be a JSON object, not array")
	}

	return nil
}

// validateAssumedRoleTTL validates TTL for assumed_role credential type
// Valid range: 900-43200 seconds (15 minutes - 12 hours)
func validateAssumedRoleTTL(ttl int64) bool {
	return ttl >= 900 && ttl <= 43200
}

// validateAwsCredentialType validates AWS credential type
// Currently only 'assumed_role' is supported
func validateAwsCredentialType(credentialType string) bool {
	// Currently only assumed_role is supported
	// Future: iam_user, federation_token, session_token
	return credentialType == "assumed_role"
}

// convertAwsTagsMap converts a Go map to AWS tags format (map with string pointers)
// AWS API requires tags as map[string]*string for proper null handling
func convertAwsTagsMap(tagsMap map[string]string) map[string]*string {
	result := make(map[string]*string)

	for key, value := range tagsMap {
		v := value
		result[key] = &v
	}

	return result
}
