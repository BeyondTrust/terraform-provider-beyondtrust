package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	_ resource.Resource                = &AwsIntegrationResource{}
	_ resource.ResourceWithImportState = &AwsIntegrationResource{}
)

func NewAwsIntegrationResource() resource.Resource {
	return &AwsIntegrationResource{}
}

// AwsIntegrationResource defines the resource implementation.
type AwsIntegrationResource struct {
	client *client.Client
}

// AwsIntegrationResourceModel describes the resource data model.
type AwsIntegrationResourceModel struct {
	Name       types.String `tfsdk:"name"`
	RoleArn    types.String `tfsdk:"role_arn"`
	ExternalId types.String `tfsdk:"external_id"`
	ID         types.String `tfsdk:"id"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

// AwsIntegrationCreateRequest represents the API request for creating an integration
type AwsIntegrationCreateRequest struct {
	Type       string  `json:"type"`
	RoleArn    string  `json:"roleArn"`
	ExternalId *string `json:"externalId,omitempty"`
}

// AwsIntegrationResponse represents the API response for an integration
type AwsIntegrationResponse struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Config   Config `json:"config"`
	Metadata struct {
		ID        string `json:"id"`
		Version   int    `json:"version"`
		CreatedAt string `json:"createdAt"`
	} `json:"metadata"`
}

type Config struct {
	RoleArn    *string `json:"roleArn,omitempty"`
	ExternalId *string `json:"externalId,omitempty"`
}

// AwsIntegrationUpdateRequest represents the API request for updating an integration
type AwsIntegrationUpdateRequest struct {
	Type       string  `json:"type"`
	RoleArn    *string `json:"roleArn,omitempty"`
	ExternalId *string `json:"externalId,omitempty"`
}

func (r *AwsIntegrationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_aws_integration"
}

func (r *AwsIntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AWS integration in BeyondTrust Workload Credentials. This integration provides credentials for accessing a customer AWS account to generate dynamic credentials.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the integration. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^%]+$ (max 100 chars). This is the resource identifier.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_arn": schema.StringAttribute{
				Description: "The ARN of the IAM role in the customer AWS account that Workload Credentials will assume. Must match pattern: arn:aws:iam::[0-9]+:role/.+",
				Required:    true,
			},
			"external_id": schema.StringAttribute{
				Description: "The external ID for the role trust relationship. Required for confused deputy prevention. Must be 2-1224 characters, alphanumeric plus _+=,.@:\\/- characters.",
				Required:    true,
				Sensitive:   true,
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier (UUID) of the integration.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the integration was created.",
				Computed:    true,
			},
		},
	}
}

func (r *AwsIntegrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AwsIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AwsIntegrationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/integrations/" + name)

	// Build request body
	createReq := AwsIntegrationCreateRequest{
		Type:    "aws",
		RoleArn: data.RoleArn.ValueString(),
	}

	if !data.ExternalId.IsNull() && data.ExternalId.ValueString() != "" {
		externalId := data.ExternalId.ValueString()
		createReq.ExternalId = &externalId
	}

	// Create the integration with retry for AWS IAM eventual consistency
	// AWS IAM roles can take time to propagate after creation
	var createResp AwsIntegrationResponse
	var err error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = r.client.Post(ctx, apiPath, nil, createReq, &createResp)
		if err == nil {
			break
		}

		// Only retry on credential test failures (likely IAM propagation delay)
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) || !apiErr.IsAWSCredentialValidationError() {
			break
		}

		if attempt < maxRetries {
			// Exponential backoff: 5s, 10s
			waitTime := 5 * attempt
			resp.Diagnostics.AddWarning(
				"Retrying AWS Integration Creation",
				fmt.Sprintf("AWS IAM role may not be fully propagated yet. Retrying in %d seconds... (attempt %d/%d)", waitTime, attempt, maxRetries),
			)
			time.Sleep(time.Duration(waitTime) * time.Second)
		}
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating AWS Integration",
			fmt.Sprintf("Could not create AWS integration '%s' after %d attempts: %s", name, maxRetries, err.Error()),
		)
		return
	}

	// Update the model with response data
	data.ID = types.StringValue(createResp.Metadata.ID)
	data.CreatedAt = types.StringValue(createResp.Metadata.CreatedAt)

	// Update with actual values from response (may have been normalized by API)
	if createResp.Config.RoleArn != nil {
		data.RoleArn = types.StringValue(*createResp.Config.RoleArn)
	}
	if createResp.Config.ExternalId != nil {
		data.ExternalId = types.StringValue(*createResp.Config.ExternalId)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AwsIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AwsIntegrationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/integrations/" + name)

	// Get integration
	var integrationResp AwsIntegrationResponse
	err := r.client.Get(ctx, apiPath, nil, &integrationResp)
	if err != nil {
		// Check if it's a 404 error using typed error handling
		if isNotFoundError(err) {
			// Integration no longer exists, remove from state
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading AWS Integration",
			fmt.Sprintf("Could not read AWS integration '%s': %s", name, err.Error()),
		)
		return
	}

	// Update state with response data
	data.ID = types.StringValue(integrationResp.Metadata.ID)
	data.CreatedAt = types.StringValue(integrationResp.Metadata.CreatedAt)

	// Update configuration (role_arn might change)
	if integrationResp.Config.RoleArn != nil {
		data.RoleArn = types.StringValue(*integrationResp.Config.RoleArn)
	}

	// Note: External ID is not returned in GET response for security reasons,
	// so we keep the value from state

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AwsIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AwsIntegrationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/integrations/" + name)

	// Build update request with merge patch semantics
	updateReq := AwsIntegrationUpdateRequest{
		Type: "aws",
	}

	// Only include fields that changed
	if !data.RoleArn.IsNull() {
		roleArn := data.RoleArn.ValueString()
		updateReq.RoleArn = &roleArn
	}

	if !data.ExternalId.IsNull() {
		externalId := data.ExternalId.ValueString()
		updateReq.ExternalId = &externalId
	}

	// Update the integration
	err := r.client.Patch(ctx, apiPath, nil, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating AWS Integration",
			fmt.Sprintf("Could not update AWS integration '%s': %s", name, err.Error()),
		)
		return
	}

	// Read back the updated integration
	var integrationResp AwsIntegrationResponse
	err = r.client.Get(ctx, apiPath, nil, &integrationResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated AWS Integration",
			"Could not read AWS integration after update: "+err.Error(),
		)
		return
	}

	// Update state with response data
	data.ID = types.StringValue(integrationResp.Metadata.ID)
	data.CreatedAt = types.StringValue(integrationResp.Metadata.CreatedAt)

	if integrationResp.Config.RoleArn != nil {
		data.RoleArn = types.StringValue(*integrationResp.Config.RoleArn)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AwsIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AwsIntegrationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the API path
	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/integrations/" + name)

	// Delete the integration
	err := r.client.Delete(ctx, apiPath, nil)
	if err != nil {
		// Ignore 404 errors (already deleted) using typed error handling
		if isNotFoundError(err) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting AWS Integration",
			fmt.Sprintf("Could not delete AWS integration '%s': %s", name, err.Error()),
		)
		return
	}
}

func (r *AwsIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by name (integration name)
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// Helper functions for AWS integration business logic

// validateAwsRoleArn validates AWS IAM role ARN format
// Pattern: arn:aws:iam::[0-9]+:role/.+
// Supports aws, aws-cn, aws-us-gov partitions
func validateAwsRoleArn(arn string) bool {
	if arn == "" {
		return false
	}

	// Basic structure check: arn:partition:service:region:account-id:resource
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return false
	}

	// Check prefix
	if parts[0] != "arn" {
		return false
	}

	// Check partition (aws, aws-cn, aws-us-gov, etc.)
	if !strings.HasPrefix(parts[1], "aws") {
		return false
	}

	// Check service
	if parts[2] != "iam" {
		return false
	}

	// Region should be empty for IAM
	// parts[3] is empty

	// Check account ID is numeric and 12 digits
	if len(parts[4]) != 12 {
		return false
	}
	for _, c := range parts[4] {
		if c < '0' || c > '9' {
			return false
		}
	}

	// Check resource type and name
	resourcePart := strings.Join(parts[5:], ":") // Handle case where resource contains ':'
	if !strings.HasPrefix(resourcePart, "role/") {
		return false
	}

	// Check role name exists (not just "role/")
	roleName := strings.TrimPrefix(resourcePart, "role/")
	if roleName == "" {
		return false
	}

	return true
}

// validateAwsExternalId validates AWS external ID format
// Must be 2-1224 characters, alphanumeric plus _+=,.@:\\/- characters
func validateAwsExternalId(externalId string) bool {
	if len(externalId) < 2 || len(externalId) > 1224 {
		return false
	}

	// Check allowed characters: alphanumeric plus _+=,.@:\/-
	for _, c := range externalId {
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '+' || c == '=' || c == ',' ||
			c == '.' || c == '@' || c == ':' || c == '\\' ||
			c == '/' || c == '-') {
			return false
		}
	}

	return true
}
