package ephemeral

import (
	"context"
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
// Note the absence of ephemeral.EphemeralResourceWithClose. An AWS assumed-role lease
// is not revocable: the STS credential expires on its own and DELETE /leases/id/{id}
// answers 400 lease_not_revocable for it. Implementing Close would spend a guaranteed
// failing request — and a can_revoke_lease authorization check the caller may not even
// hold — on every apply, to swallow the error. The expiration attribute documents the
// lifetime instead.
var _ ephemeral.EphemeralResourceWithConfigure = &AwsDynamicSecretEphemeral{}

func NewAwsDynamicSecretEphemeral() ephemeral.EphemeralResource {
	return &AwsDynamicSecretEphemeral{}
}

// AwsDynamicSecretEphemeral generates temporary AWS credentials from a dynamic secret.
type AwsDynamicSecretEphemeral struct {
	client *client.Client
}

// AwsDynamicSecretEphemeralModel describes the ephemeral resource data model.
type AwsDynamicSecretEphemeralModel struct {
	Name            types.String `tfsdk:"name"`
	Folder          types.String `tfsdk:"folder"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	SessionToken    types.String `tfsdk:"session_token"`
	Expiration      types.String `tfsdk:"expiration"`
	LeaseID         types.String `tfsdk:"lease_id"`
}

// awsGeneratedSecret is the generate response payload for an AWS dynamic secret.
// Keys are camelCase: the API merges the lease fields into the provider's own output
// and normalizes the whole object before serializing it.
type awsGeneratedSecret struct {
	LeaseID         string `json:"leaseId"`
	Type            string `json:"type"`
	CredentialType  string `json:"credentialType"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Expiration      string `json:"expiration"`
}

func (e *AwsDynamicSecretEphemeral) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_credentials_aws_dynamic_secret"
}

func (e *AwsDynamicSecretEphemeral) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates temporary AWS credentials from a BeyondTrust Workload Credentials dynamic secret. " +
			"Credentials are minted on demand via sts:AssumeRole, returned once, and never stored in Terraform state or plan files. " +
			"They expire on their own at the dynamic secret's TTL and cannot be revoked early.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the dynamic secret to generate credentials from.",
				Required:    true,
				Validators: []validator.String{
					validators.ResourceNameValidator(),
				},
			},
			"folder": schema.StringAttribute{
				Description: "The parent folder path (e.g., 'production' or 'production/aws'). Leave empty for root level. Each segment must match: ^[a-zA-Z0-9\\-_@~\\*\\^]{1,130}$.",
				Optional:    true,
				Validators: []validator.String{
					validators.FolderPathValidator(),
				},
			},
			"access_key_id": schema.StringAttribute{
				Description: "The AWS access key ID for the generated session.",
				Computed:    true,
				Sensitive:   true,
			},
			"secret_access_key": schema.StringAttribute{
				Description: "The AWS secret access key for the generated session.",
				Computed:    true,
				Sensitive:   true,
			},
			"session_token": schema.StringAttribute{
				Description: "The AWS session token for the generated session.",
				Computed:    true,
				Sensitive:   true,
			},
			"expiration": schema.StringAttribute{
				Description: "The RFC3339 timestamp at which the generated credentials expire.",
				Computed:    true,
			},
			"lease_id": schema.StringAttribute{
				Description: "The identifier of the lease tracking these credentials. Recorded for auditing; AWS leases cannot be revoked before they expire.",
				Computed:    true,
			},
		},
	}
}

func (e *AwsDynamicSecretEphemeral) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
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

func (e *AwsDynamicSecretEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data AwsDynamicSecretEphemeralModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()

	secret, err := generateCredential[awsGeneratedSecret](ctx, e.client, name, data.Folder.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Generating AWS Credentials",
			fmt.Sprintf("Could not generate credentials from dynamic secret '%s': %s\n\n"+
				"A 403 here can mean either outcome: the API reports a dynamic secret you cannot see "+
				"as forbidden rather than missing. Check both.\n\n"+
				"  - The dynamic secret must already exist when this runs. It is opened during the "+
				"plan, so a configuration that creates it in the same apply fails here; apply the "+
				"dynamic secret first. depends_on does not help, because the open happens before it "+
				"takes effect.\n"+
				"  - The caller needs the GenerateDynamicCredential permission on it. Product admins "+
				"hold it already; anyone else needs a policy granting it.",
				name, err.Error()),
		)
		return
	}

	data.AccessKeyID = types.StringValue(secret.AccessKeyID)
	data.SecretAccessKey = types.StringValue(secret.SecretAccessKey)
	data.SessionToken = types.StringValue(secret.SessionToken)
	data.Expiration = types.StringValue(secret.Expiration)
	data.LeaseID = types.StringValue(secret.LeaseID)

	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
