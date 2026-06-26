package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &WorkloadIdentityResource{}
	_ resource.ResourceWithImportState    = &WorkloadIdentityResource{}
	_ resource.ResourceWithValidateConfig = &WorkloadIdentityResource{}
)

// IDP categories supported by the auth service.
const (
	idpGitHubActions = "GitHubActions"
	idpAzureEntra    = "AzureEntra"
	idpCustom        = "Custom"

	maxConditionValueLength = 256
)

var validIdpCategories = []string{idpGitHubActions, idpAzureEntra, idpCustom}

// Claim keys the auth service accepts in conditions when idpCategory == GitHubActions.
var gitHubActionsAllowedKeys = map[string]struct{}{
	"sub": {}, "repository": {}, "repository_owner": {}, "ref": {},
	"job_workflow_ref": {}, "workflow_ref": {}, "environment": {}, "event_name": {}, "actor": {},
}

func NewWorkloadIdentityResource() resource.Resource {
	return &WorkloadIdentityResource{}
}

// WorkloadIdentityResource manages an OIDC issuer trust config (workload identity) in the BeyondTrust auth service.
type WorkloadIdentityResource struct {
	client *client.Client
}

// WorkloadIdentityResourceModel describes the resource data model.
type WorkloadIdentityResourceModel struct {
	ServiceName      types.String `tfsdk:"service_name"`
	IssuerURL        types.String `tfsdk:"issuer_url"`
	IdpCategory      types.String `tfsdk:"idp_category"`
	SiteID           types.String `tfsdk:"site_id"`
	Conditions       types.Map    `tfsdk:"conditions"`
	Description      types.String `tfsdk:"description"`
	ScopeLevel       types.String `tfsdk:"scope_level"`
	RegisteredScopes types.List   `tfsdk:"registered_scopes"`
	ID               types.String `tfsdk:"id"`
	OrganizationID   types.String `tfsdk:"organization_id"`
}

// issuerRequest is the create/update request body. Field casing matches the auth API.
type issuerRequest struct {
	SiteID           string              `json:"siteId"`
	ServiceName      string              `json:"serviceName"`
	IssuerURL        string              `json:"issuerUrl"`
	IdpCategory      string              `json:"idpCategory"`
	ScopeLevel       string              `json:"scopeLevel"`
	RegisteredScopes []string            `json:"registeredScopes"`
	Conditions       map[string][]string `json:"conditions"`
	Description      string              `json:"description"`
}

// issuer mirrors the auth service response payload.
type issuer struct {
	IdentityID       string              `json:"identityId"`
	ServiceName      string              `json:"serviceName"`
	IssuerURL        string              `json:"issuerUrl"`
	IdpCategory      string              `json:"idpCategory"`
	SiteID           string              `json:"siteId"`
	OrganizationID   string              `json:"organizationId"`
	ScopeLevel       string              `json:"scopeLevel"`
	RegisteredScopes []string            `json:"registeredScopes"`
	Conditions       map[string][]string `json:"conditions"`
	Description      string              `json:"description"`
}

// issuerEnvelope is the { "issuer": {...} } wrapper returned by create/get/update.
type issuerEnvelope struct {
	Issuer issuer `json:"issuer"`
}

func (r *WorkloadIdentityResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auth_workload_identity"
}

func (r *WorkloadIdentityResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a workload identity (OIDC issuer trust config) in BeyondTrust, enabling workload identity federation against the BeyondTrust auth service.",
		Attributes: map[string]schema.Attribute{
			"service_name": schema.StringAttribute{
				Description: "Human label for this workload identity registration. Immutable — changing it forces replacement.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"issuer_url": schema.StringAttribute{
				Description: "OIDC issuer URL (the token 'iss' claim). Immutable — changing it forces replacement.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"idp_category": schema.StringAttribute{
				Description: "Identity provider category. One of: GitHubActions, AzureEntra, Custom. Immutable — changing it forces replacement.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{idpCategoryValidator{}},
			},
			"site_id": schema.StringAttribute{
				Description: "The site (UUID) this workload identity grants access to — any site within the organization. Defaults to the provider's configured site when omitted. Immutable — changing it forces replacement.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"conditions": schema.MapAttribute{
				Description: "Claim constraints evaluated against the incoming token. Map of claim key to a list of allowed values. Updatable in place.",
				Required:    true,
				ElementType: types.ListType{ElemType: types.StringType},
			},
			"description": schema.StringAttribute{
				Description: "Free-form description. Updatable in place.",
				Optional:    true,
				Computed:    true,
			},
			"scope_level": schema.StringAttribute{
				Description: "Scope level for the identity: 'site' or 'org'. Defaults to 'site'. Updatable in place.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("site"),
			},
			"registered_scopes": schema.ListAttribute{
				Description: "Product scopes granted to this workload identity. At least one is required. Updatable in place.",
				Required:    true,
				ElementType: types.StringType,
			},
			"id": schema.StringAttribute{
				Description: "The stable identity id (UUID) assigned by the auth service.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "The organization (UUID) that owns this workload identity.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *WorkloadIdentityResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

// ValidateConfig enforces the cross-field condition rules that the auth service applies, so the
// practitioner gets actionable plan-time errors instead of a 400 at apply.
func (r *WorkloadIdentityResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data WorkloadIdentityResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// registered_scopes must be non-empty (the auth service rejects an empty list).
	if !data.RegisteredScopes.IsNull() && !data.RegisteredScopes.IsUnknown() && len(data.RegisteredScopes.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("registered_scopes"),
			"At least one registered scope is required",
			"registered_scopes must contain at least one scope.",
		)
	}

	if data.Conditions.IsNull() || data.Conditions.IsUnknown() || data.IdpCategory.IsUnknown() {
		return
	}

	conditions := make(map[string][]string)
	resp.Diagnostics.Append(data.Conditions.ElementsAs(ctx, &conditions, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateConditions(data.IdpCategory.ValueString(), conditions, resp)
}

func (r *WorkloadIdentityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkloadIdentityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := r.buildRequest(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var env issuerEnvelope
	if err := r.client.Post(ctx, r.client.BuildAuthPath("/workload-identities"), nil, body, &env); err != nil {
		resp.Diagnostics.AddError("Error Creating Workload Identity", err.Error())
		return
	}

	resp.Diagnostics.Append(applyComputed(&data, env.Issuer)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkloadIdentityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkloadIdentityResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var env issuerEnvelope
	err := r.client.Get(ctx, r.client.BuildAuthPath("/workload-identities/"+data.ID.ValueString()), nil, &env)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Workload Identity", err.Error())
		return
	}

	resp.Diagnostics.Append(applyRead(ctx, &data, env.Issuer)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkloadIdentityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorkloadIdentityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := r.buildRequest(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var env issuerEnvelope
	if err := r.client.Put(ctx, r.client.BuildAuthPath("/workload-identities/"+data.ID.ValueString()), nil, body, &env); err != nil {
		resp.Diagnostics.AddError("Error Updating Workload Identity", err.Error())
		return
	}

	resp.Diagnostics.Append(applyComputed(&data, env.Issuer)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkloadIdentityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkloadIdentityResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, r.client.BuildAuthPath("/workload-identities/"+data.ID.ValueString()), nil)
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Workload Identity", err.Error())
	}
}

func (r *WorkloadIdentityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by identityId; Read repopulates the rest from the API.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildRequest assembles the API request body from the plan, defaulting the access site
// to the provider's configured (admin) site when the practitioner omits site_id.
func (r *WorkloadIdentityResource) buildRequest(ctx context.Context, data *WorkloadIdentityResourceModel) (issuerRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	siteID := data.SiteID.ValueString()
	if data.SiteID.IsNull() || data.SiteID.IsUnknown() {
		siteID = r.client.SiteID
	}

	conditions := make(map[string][]string)
	diags = append(diags, data.Conditions.ElementsAs(ctx, &conditions, false)...)

	var scopes []string
	diags = append(diags, data.RegisteredScopes.ElementsAs(ctx, &scopes, false)...)

	return issuerRequest{
		SiteID:           siteID,
		ServiceName:      data.ServiceName.ValueString(),
		IssuerURL:        data.IssuerURL.ValueString(),
		IdpCategory:      data.IdpCategory.ValueString(),
		ScopeLevel:       data.ScopeLevel.ValueString(),
		RegisteredScopes: scopes,
		Conditions:       conditions,
		Description:      data.Description.ValueString(),
	}, diags
}
