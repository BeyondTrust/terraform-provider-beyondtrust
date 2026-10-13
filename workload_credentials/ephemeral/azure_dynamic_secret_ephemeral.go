package ephemeral

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
//
// Unlike the AWS equivalent this does implement Close: an Azure service principal
// password is a real credential sitting on the target app registration until something
// removes it, so it is worth deleting as soon as Terraform is finished with it.
var (
	_ ephemeral.EphemeralResourceWithConfigure = &AzureDynamicSecretEphemeral{}
	_ ephemeral.EphemeralResourceWithClose     = &AzureDynamicSecretEphemeral{}
)

func NewAzureDynamicSecretEphemeral() ephemeral.EphemeralResource {
	return &AzureDynamicSecretEphemeral{}
}

// AzureDynamicSecretEphemeral generates a temporary Azure service principal password.
type AzureDynamicSecretEphemeral struct {
	client *client.Client
}

// AzureDynamicSecretEphemeralModel describes the ephemeral resource data model.
//
// There is no expiration attribute because the generate response does not include one.
// The dynamic secret's ttl is the source of truth.
type AzureDynamicSecretEphemeralModel struct {
	Name          types.String `tfsdk:"name"`
	Folder        types.String `tfsdk:"folder"`
	RevokeOnClose types.Bool   `tfsdk:"revoke_on_close"`
	ClientID      types.String `tfsdk:"client_id"`
	ClientSecret  types.String `tfsdk:"client_secret"`
	TenantID      types.String `tfsdk:"tenant_id"`
	KeyID         types.String `tfsdk:"key_id"`
	LeaseID       types.String `tfsdk:"lease_id"`
}

// azureGeneratedSecret is the generate response payload for an Azure dynamic secret.
type azureGeneratedSecret struct {
	LeaseID        string `json:"leaseId"`
	Type           string `json:"type"`
	CredentialType string `json:"credentialType"`
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	TenantID       string `json:"tenantId"`
	KeyID          string `json:"keyId"`
}

// leaseID satisfies the check in generateCredential that a response really carried a
// credential rather than decoding into an empty struct.
func (s azureGeneratedSecret) leaseID() string { return s.LeaseID }

func (e *AzureDynamicSecretEphemeral) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_azure_dynamic_secret"
}

func (e *AzureDynamicSecretEphemeral) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates a temporary Azure service principal password from a BeyondTrust Workload Credentials dynamic secret. " +
			"Credentials are minted on demand, returned once, and never stored in Terraform state or plan files. " +
			"By default the credential is revoked via Microsoft Graph as soon as Terraform finishes with it.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the dynamic secret to generate credentials from.",
				Required:    true,
				Validators: []validator.String{
					validators.ResourceNameValidator(),
				},
			},
			"folder": schema.StringAttribute{
				Description: "The parent folder path (e.g., 'production' or 'production/azure'). Leave empty for root level. Each segment must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$.",
				Optional:    true,
				Validators: []validator.String{
					validators.FolderPathValidator(),
				},
			},
			"revoke_on_close": schema.BoolAttribute{
				Description: "Whether to delete the generated password from the target application when Terraform finishes using it. " +
					"Defaults to `true`. Set to `false` when the credential is handed to a system that must keep using it after the " +
					"apply completes; it then remains valid until the dynamic secret's TTL elapses.",
				Optional: true,
				Computed: true,
			},
			"client_id": schema.StringAttribute{
				Description: "The client ID of the target application the password was created on.",
				Computed:    true,
				Sensitive:   true,
			},
			"client_secret": schema.StringAttribute{
				Description: "The generated service principal password.",
				Computed:    true,
				Sensitive:   true,
			},
			"tenant_id": schema.StringAttribute{
				Description: "The Azure tenant ID the application belongs to.",
				Computed:    true,
				Sensitive:   true,
			},
			"key_id": schema.StringAttribute{
				Description: "The identifier of the password credential on the target application.",
				Computed:    true,
			},
			"lease_id": schema.StringAttribute{
				Description: "The identifier of the lease tracking this credential.",
				Computed:    true,
			},
		},
	}
}

func (e *AzureDynamicSecretEphemeral) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	apiClient, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Ephemeral Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	e.client = apiClient
}

func (e *AzureDynamicSecretEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data AzureDynamicSecretEphemeralModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if e.client == nil {
		resp.Diagnostics.AddError(errUnconfiguredClient("beyondtrust_workload_credentials_azure_dynamic_secret"))
		return
	}

	// Ephemeral schemas have no Default, so the default is applied here. An unknown
	// value resolves to true as well: erring towards revoking is the safe direction.
	revokeOnClose := data.RevokeOnClose.IsNull() || data.RevokeOnClose.IsUnknown() || data.RevokeOnClose.ValueBool()
	data.RevokeOnClose = types.BoolValue(revokeOnClose)

	name := data.Name.ValueString()

	secret, err := generateCredential[azureGeneratedSecret](ctx, e.client, name, data.Folder.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Generating Azure Credentials",
			generateFailureDetail(name, err),
		)
		return
	}

	// From here on a real credential exists. If anything below fails, Terraform will not
	// call Close, so revoke it here. revoke_on_close = false is still honoured.
	defer func() {
		if !resp.Diagnostics.HasError() || !revokeOnClose {
			return
		}
		if err := revokeWithTimeout(ctx, e.client, secret.LeaseID); err != nil {
			resp.Diagnostics.AddWarning(
				"Azure Credential Not Revoked",
				fmt.Sprintf("Generating credentials from '%s' failed after the credential had already been created, "+
					"and revoking it also failed: %s\n\nThe credential remains valid until the dynamic secret's TTL "+
					"elapses. Revoke lease '%s' manually to remove it sooner.", name, err.Error(), secret.LeaseID),
			)
		}
	}()

	data.ClientID = types.StringValue(secret.ClientID)
	data.ClientSecret = types.StringValue(secret.ClientSecret)
	data.TenantID = types.StringValue(secret.TenantID)
	data.KeyID = types.StringValue(secret.KeyID)
	data.LeaseID = types.StringValue(secret.LeaseID)

	// Close receives private state and nothing else, so the lease and the caller's
	// revoke preference have to travel there through it.
	if resp.Private != nil {
		encoded, err := marshalLeasePrivateState(secret.LeaseID, revokeOnClose)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Recording Lease State",
				fmt.Sprintf("Could not encode lease private state for '%s': %s", name, err.Error()),
			)
			return
		}
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, privateStateLeaseKey, encoded)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}

func (e *AzureDynamicSecretEphemeral) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	if e.client == nil || req.Private == nil {
		return
	}

	// The key is a constant, so GetKey cannot legitimately fail. Either way, having
	// nothing recorded means there is nothing to revoke.
	encoded, diags := req.Private.GetKey(ctx, privateStateLeaseKey)
	if diags.HasError() || len(encoded) == 0 {
		return
	}

	var state leasePrivateState
	if err := json.Unmarshal(encoded, &state); err != nil {
		resp.Diagnostics.AddWarning(
			"Azure Credential Not Revoked",
			fmt.Sprintf("Could not decode the recorded lease state, so the generated credential was not revoked: %s\n\n"+
				"It remains valid until the dynamic secret's TTL elapses.", err.Error()),
		)
		return
	}

	if !state.RevokeOnClose || state.LeaseID == "" {
		return
	}

	// Diagnostics from Close are warnings without exception. An error here would fail
	// the whole Terraform operation over cleanup, long after the real work succeeded.
	if err := revokeWithTimeout(ctx, e.client, state.LeaseID); err != nil {
		resp.Diagnostics.AddWarning(
			"Azure Credential Not Revoked",
			fmt.Sprintf("Could not revoke lease '%s': %s\n\nThe credential remains valid until the dynamic secret's TTL "+
				"elapses. If this persists, confirm the principal holds the RevokeLease permission on the dynamic secret.",
				state.LeaseID, err.Error()),
		)
	}
}
