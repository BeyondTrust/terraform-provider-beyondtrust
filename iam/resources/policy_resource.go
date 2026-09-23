package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &PolicyResource{}
	_ resource.ResourceWithImportState    = &PolicyResource{}
	_ resource.ResourceWithValidateConfig = &PolicyResource{}
)

// defaultWaitTimeout bounds how long Create and Update wait for a policy to become ACTIVE.
const defaultWaitTimeout = 10 * time.Minute

func NewPolicyResource() resource.Resource {
	return &PolicyResource{}
}

// PolicyResource manages a Cedar authorization policy in the BeyondTrust IAM policy service.
type PolicyResource struct {
	client *client.Client
}

// PolicyResourceModel describes the resource data model.
type PolicyResourceModel struct {
	Name      types.String   `tfsdk:"name"`
	Cedar     types.String   `tfsdk:"cedar"`
	Site      types.String   `tfsdk:"site"`
	Product   types.String   `tfsdk:"product"`
	Status    types.String   `tfsdk:"status"`
	CreatedAt types.String   `tfsdk:"created_at"`
	UpdatedAt types.String   `tfsdk:"updated_at"`
	Timeouts  timeouts.Value `tfsdk:"timeouts"`
}

func (r *PolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_policy"
}

func (r *PolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cedar authorization policy. A policy grants a principal — a user, " +
			"workload, group, or role — permission to act on a resource in a BeyondTrust product. " +
			"Policies take effect asynchronously, so applying one waits until the grant is live.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "Name for the policy. Must be unique across your organization, and is how " +
					"the policy is identified. Changing it replaces the policy.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{validators.PolicyNameValidator()},
			},
			"cedar": schema.StringAttribute{
				Description: "The Cedar policy text: a single permit statement carrying an " +
					"@siteId(\"<uuid>\") annotation that names the site it applies to. Stored exactly " +
					"as written.",
				Required: true,
			},
			"site": schema.StringAttribute{
				Description: "The site this policy applies to, taken from its @siteId annotation.",
				Computed:    true,
			},
			"product": schema.StringAttribute{
				Description: "The BeyondTrust product whose resources this policy grants access to.",
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "Whether the grant is in effect. ACTIVE means it is. WAITING_FOR_RESOURCE " +
					"means the policy is valid but the resource it names does not exist yet; it takes " +
					"effect on its own once that resource is created.",
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Description: "When this version of the policy was written. Each change writes a new " +
					"version, so this updates whenever the policy does.",
				Computed: true,
			},
			"updated_at": schema.StringAttribute{
				Description: "When the policy was last changed.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
			}),
		},
	}
}

func (r *PolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig parses the Cedar text at plan time so structural problems surface before apply.
func (r *PolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The Cedar text is commonly interpolated from another provider's data source, in which case
	// it is not known until apply and there is nothing to check yet.
	if data.Cedar.IsNull() || data.Cedar.IsUnknown() {
		return
	}

	if err := validateCedar(data.Cedar.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("cedar"), err.Summary, err.Detail)
	}
}

func (r *PolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := data.Timeouts.Create(ctx, defaultWaitTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.refuseIfExists(ctx, data.Name.ValueString(), &resp.Diagnostics) {
		return
	}

	r.write(ctx, &data, timeout, "create", &resp.Diagnostics, &resp.State)
}

// refuseIfExists reports whether Create should stop because the name is already taken.
//
// The service has no create-only verb — the same PUT creates or replaces — and policy names are
// unique per organization rather than per site. Without this check, applying a config whose name
// collides with an existing policy would silently adopt and overwrite it, and a later destroy
// would delete a policy this configuration never created.
func (r *PolicyResource) refuseIfExists(ctx context.Context, name string, diags *diag.Diagnostics) bool {
	existing, err := r.getPolicy(ctx, name)
	switch {
	case isNotFound(err):
		return false // free
	case err != nil:
		diags.AddError("Error Creating IAM Policy", policyErrorDetail("check for an existing", name, err))
		return true
	case existing.Status == statusDeleted || existing.Status == statusPendingUnbind:
		// A deleted policy still answers reads until its tombstone expires, but the name is
		// reusable, so this is not a collision.
		return false
	}

	diags.AddError(
		"IAM Policy Already Exists",
		fmt.Sprintf("A policy named %q already exists in this organization (status %s). Policy "+
			"names are unique per organization, not per site.\n\nEither choose a different name, "+
			"or bring the existing policy under Terraform's management:\n\n"+
			"    terraform import <resource address> %s", name, existing.Status, name),
	)
	return true
}

func (r *PolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := data.Timeouts.Update(ctx, defaultWaitTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.write(ctx, &data, timeout, "update", &resp.Diagnostics, &resp.State)
}

// write performs the create-or-replace PUT shared by Create and Update, then waits for the
// policy to settle.
//
// State is written as soon as the PUT succeeds and again after the wait. That ordering matters:
// the policy exists server-side the moment the PUT returns 202, so a failure while waiting must
// not leave Terraform without a record of it — otherwise the next apply would try to create a
// policy that is already there and the practitioner would have no way to destroy it.
func (r *PolicyResource) write(
	ctx context.Context,
	data *PolicyResourceModel,
	timeout time.Duration,
	verb string,
	diags *diag.Diagnostics,
	state *tfsdk.State,
) {
	name := data.Name.ValueString()

	pol, err := r.putPolicy(ctx, name, data.Cedar.ValueString())
	if err != nil {
		diags.AddError(errorTitle(verb), policyErrorDetail(verb, name, err))
		return
	}

	applyPolicy(data, pol)
	diags.Append(state.Set(ctx, data)...)
	if diags.HasError() {
		return
	}

	settled, waitErr := r.waitForPolicy(ctx, name, timeout)
	if settled != nil {
		applyPolicy(data, settled)
		diags.Append(state.Set(ctx, data)...)
	}

	var pending *pendingStateError
	switch {
	case waitErr == nil:
	case errors.As(waitErr, &pending):
		// The write succeeded; the grant just is not bound yet. Failing here would be
		// misleading, and tearing the resource down would be wrong.
		diags.AddWarning(
			fmt.Sprintf("IAM Policy %q Is Not Active Yet", name),
			pending.Error(),
		)
	default:
		diags.AddError(
			fmt.Sprintf("IAM Policy %q Did Not Become Active", name),
			fmt.Sprintf("The policy was saved, but %s\n\nThe policy still exists and is tracked by "+
				"Terraform; correct the problem and re-apply.", waitErr.Error()),
		)
	}
}

func errorTitle(verb string) string {
	if verb == "create" {
		return "Error Creating IAM Policy"
	}
	return "Error Updating IAM Policy"
}

func (r *PolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	pol, err := r.getPolicy(ctx, name)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading IAM Policy", policyErrorDetail("read", name, err))
		return
	}

	// A 404 is not the only way a policy can be gone. Deletion is asynchronous: the service keeps
	// serving the policy as PENDING_UNBIND while the unbind propagates, then as a DELETED
	// tombstone for a couple of days. Without this, deleting a policy out of band would leave
	// state intact and `terraform plan` would report "No changes" — for an authorization
	// resource, silently failing to notice a revoked grant is the worst kind of drift.
	if pol.Status == statusDeleted || pol.Status == statusPendingUnbind {
		resp.State.RemoveResource(ctx)
		return
	}

	applyPolicy(&data, pol)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Deletion is asynchronous and the service keeps returning the policy (as PENDING_UNBIND,
	// then a short-lived DELETED tombstone) until the unbind propagates, so there is nothing
	// useful to poll for here. DELETE is idempotent and never 404s, but tolerate it anyway.
	name := data.Name.ValueString()
	if err := r.client.Delete(ctx, r.client.BuildIAMPath("/policies/"+name), nil); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting IAM Policy", policyErrorDetail("delete", name, err))
	}
}

func (r *PolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := req.ID
	if !validators.IsValidPolicyName(name) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Policy name %q is invalid. Import using the policy name, which must be 1-100 "+
				"characters starting and ending with a letter or digit.", name),
		)
		return
	}
	// Read repopulates cedar and the computed attributes from the API.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

// applyPolicy copies the API response onto the model. The service stores Cedar text byte for
// byte, so writing it back verbatim is what keeps the resource free of perpetual diffs.
func applyPolicy(data *PolicyResourceModel, pol *policy) {
	data.Cedar = types.StringValue(pol.Cedar)
	data.Site = types.StringValue(pol.Site)
	data.Product = types.StringValue(pol.Product)
	data.Status = types.StringValue(pol.Status)
	data.CreatedAt = types.StringValue(pol.CreatedAt)
	data.UpdatedAt = types.StringValue(pol.UpdatedAt)
}
