package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	_ resource.Resource                = &AzureIntegrationResource{}
	_ resource.ResourceWithImportState = &AzureIntegrationResource{}
)

func NewAzureIntegrationResource() resource.Resource {
	return &AzureIntegrationResource{}
}

// AzureIntegrationResource defines the resource implementation.
type AzureIntegrationResource struct {
	client *client.Client
}

// AzureIntegrationResourceModel describes the resource data model.
type AzureIntegrationResourceModel struct {
	Name                types.String `tfsdk:"name"`
	TenantID            types.String `tfsdk:"tenant_id"`
	ClientID            types.String `tfsdk:"client_id"`
	ClientSecret        types.String `tfsdk:"client_secret"`         // Write-only
	ClientSecretVersion types.Int64  `tfsdk:"client_secret_version"` // User-controlled rotation trigger
	ID                  types.String `tfsdk:"id"`
	CreatedAt           types.String `tfsdk:"created_at"`
	Version             types.Int64  `tfsdk:"version"`
	CreatedBy           types.String `tfsdk:"created_by"`
}

// AzureIntegrationCreateRequest represents the API request for creating an integration
type AzureIntegrationCreateRequest struct {
	TenantID     string `json:"tenantId"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// AzureIntegrationResponse represents the API response for an integration
type AzureIntegrationResponse struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Config   AzureIntegrationConfig `json:"config"`
	Metadata struct {
		ID        string `json:"id"`
		Version   int    `json:"version"`
		CreatedAt string `json:"createdAt"`
		CreatedBy string `json:"createdBy,omitempty"`
	} `json:"metadata"`
}

// AzureIntegrationConfig is the config block returned by the API.
// clientSecret is intentionally absent — the API never returns it.
type AzureIntegrationConfig struct {
	TenantID *string `json:"tenantId,omitempty"`
	ClientID *string `json:"clientId,omitempty"`
}

// AzureIntegrationUpdateRequest represents the API request for updating an integration.
// All fields are optional (merge-patch semantics). clientSecret is only sent when
// client_secret_version changes, so it uses omitempty here.
type AzureIntegrationUpdateRequest struct {
	TenantID     *string `json:"tenantId,omitempty"`
	ClientID     *string `json:"clientId,omitempty"`
	ClientSecret *string `json:"clientSecret,omitempty"`
}

func (r *AzureIntegrationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_azure_integration"
}

func (r *AzureIntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Azure integration in BeyondTrust Workload Credentials. This integration uses an Azure Active Directory service principal to generate dynamic credentials for Azure applications.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the integration. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$ (single path segment, max 130 chars). This is the resource identifier.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.ResourceNameValidator(),
				},
			},
			"tenant_id": schema.StringAttribute{
				Description: "The Azure Active Directory tenant ID (directory ID). Must be a valid UUID.",
				Required:    true,
				Validators: []validator.String{
					validators.UUIDValidator(),
				},
			},
			"client_id": schema.StringAttribute{
				Description: "The Azure Active Directory application (client) ID. Must be a valid UUID.",
				Required:    true,
				Validators: []validator.String{
					validators.UUIDValidator(),
				},
			},
			"client_secret": schema.StringAttribute{
				Description: "The Azure Active Directory application client secret. Write-only — not stored in Terraform state and never returned by the API. Increment client_secret_version to trigger rotation.",
				Required:    true,
				Sensitive:   true,
				WriteOnly:   true,
			},
			"client_secret_version": schema.Int64Attribute{
				Description: "User-controlled version number for the client secret. Increment this value to signal that client_secret has changed and should be re-applied. Write-only values cannot be diffed automatically, so this attribute serves as the rotation trigger.",
				Required:    true,
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
			"version": schema.Int64Attribute{
				Description: "The current version of the integration.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"created_by": schema.StringAttribute{
				Description: "The ID of the user who created the integration.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AzureIntegrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AzureIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AzureIntegrationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Write-only attributes are nullified in req.Plan by the framework (they can't be
	// stored in state). Read client_secret from req.Config where the actual value lives.
	var configData AzureIntegrationResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/integrations/azure/" + name)

	// Guard against null/unknown client_secret
	if configData.ClientSecret.IsNull() || configData.ClientSecret.IsUnknown() {
		resp.Diagnostics.AddError(
			"Missing Client Secret",
			"client_secret is required but was null or unknown. This should not happen - please report this as a provider bug.",
		)
		return
	}

	createReq := AzureIntegrationCreateRequest{
		TenantID:     data.TenantID.ValueString(),
		ClientID:     data.ClientID.ValueString(),
		ClientSecret: configData.ClientSecret.ValueString(),
	}

	// Retry on Azure credential validation failures (similar to AWS IAM propagation delays)
	var createResp AzureIntegrationResponse
	var err error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = r.client.Post(ctx, apiPath, nil, createReq, &createResp)
		if err == nil {
			break
		}

		var apiErr *client.APIError
		if !errors.As(err, &apiErr) || !apiErr.IsAzureCredentialValidationError() {
			break
		}

		if attempt < maxRetries {
			waitTime := 5 * attempt
			resp.Diagnostics.AddWarning(
				"Retrying Azure Integration Creation",
				fmt.Sprintf("Azure credentials may not be fully propagated. Retrying in %d seconds... (attempt %d/%d)", waitTime, attempt, maxRetries),
			)
			time.Sleep(time.Duration(waitTime) * time.Second)
		}
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Azure Integration",
			fmt.Sprintf("Could not create Azure integration '%s' after %d attempts: %s", name, maxRetries, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(createResp.Metadata.ID)
	data.CreatedAt = types.StringValue(createResp.Metadata.CreatedAt)
	data.Version = types.Int64Value(int64(createResp.Metadata.Version))
	if createResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(createResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}

	if createResp.Config.TenantID != nil {
		data.TenantID = types.StringValue(*createResp.Config.TenantID)
	}
	if createResp.Config.ClientID != nil {
		data.ClientID = types.StringValue(*createResp.Config.ClientID)
	}

	// client_secret is write-only — null it out before persisting to state
	data.ClientSecret = types.StringNull()

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AzureIntegrationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/integrations/azure/" + name)

	var integrationResp AzureIntegrationResponse
	err := r.client.Get(ctx, apiPath, nil, &integrationResp)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Azure Integration",
			fmt.Sprintf("Could not read Azure integration '%s': %s", name, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(integrationResp.Metadata.ID)
	data.CreatedAt = types.StringValue(integrationResp.Metadata.CreatedAt)
	data.Version = types.Int64Value(int64(integrationResp.Metadata.Version))
	if integrationResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(integrationResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}

	if integrationResp.Config.TenantID != nil {
		data.TenantID = types.StringValue(*integrationResp.Config.TenantID)
	}
	if integrationResp.Config.ClientID != nil {
		data.ClientID = types.StringValue(*integrationResp.Config.ClientID)
	}

	// client_secret is never returned by the API — preserve state value (null from write-only)
	// client_secret_version is user-controlled — preserve state value

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AzureIntegrationResourceModel
	var state AzureIntegrationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Write-only attributes are nullified in req.Plan. Read client_secret from req.Config.
	var configData AzureIntegrationResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/integrations/azure/" + name)

	updateReq := AzureIntegrationUpdateRequest{}

	if !data.TenantID.IsNull() {
		v := data.TenantID.ValueString()
		updateReq.TenantID = &v
	}

	if !data.ClientID.IsNull() {
		v := data.ClientID.ValueString()
		updateReq.ClientID = &v
	}

	// Only send client_secret when the version has been bumped by the user
	if !data.ClientSecretVersion.Equal(state.ClientSecretVersion) {
		// Guard against null/unknown client_secret when version changed
		if configData.ClientSecret.IsNull() || configData.ClientSecret.IsUnknown() {
			resp.Diagnostics.AddError(
				"Missing Client Secret",
				"client_secret_version was incremented but client_secret is null or unknown. "+
					"When rotating secrets (incrementing client_secret_version), you must provide the new client_secret value.",
			)
			return
		}

		v := configData.ClientSecret.ValueString()
		updateReq.ClientSecret = &v
	}

	err := r.client.Patch(ctx, apiPath, nil, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Azure Integration",
			fmt.Sprintf("Could not update Azure integration '%s': %s", name, err.Error()),
		)
		return
	}

	var integrationResp AzureIntegrationResponse
	err = r.client.Get(ctx, apiPath, nil, &integrationResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Azure Integration",
			"Could not read Azure integration after update: "+err.Error(),
		)
		return
	}

	data.ID = types.StringValue(integrationResp.Metadata.ID)
	data.CreatedAt = types.StringValue(integrationResp.Metadata.CreatedAt)
	data.Version = types.Int64Value(int64(integrationResp.Metadata.Version))
	if integrationResp.Metadata.CreatedBy != "" {
		data.CreatedBy = types.StringValue(integrationResp.Metadata.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}

	if integrationResp.Config.TenantID != nil {
		data.TenantID = types.StringValue(*integrationResp.Config.TenantID)
	}
	if integrationResp.Config.ClientID != nil {
		data.ClientID = types.StringValue(*integrationResp.Config.ClientID)
	}

	// client_secret is write-only — null it out before persisting to state
	data.ClientSecret = types.StringNull()

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AzureIntegrationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	apiPath := r.client.BuildPath("/integrations/azure/" + name)

	err := r.client.Delete(ctx, apiPath, nil)
	if err != nil {
		if isNotFoundError(err) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Azure Integration",
			fmt.Sprintf("Could not delete Azure integration '%s': %s", name, err.Error()),
		)
		return
	}
}

func (r *AzureIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := req.ID

	if !validators.IsValidResourceName(name) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Name %q is invalid. Must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$", name),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}
