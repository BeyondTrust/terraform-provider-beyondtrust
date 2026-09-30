//go:build !acceptance

package ephemeral

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func awsEphemeralSchema(t *testing.T) ephemeral.SchemaResponse {
	t.Helper()

	var resp ephemeral.SchemaResponse
	(&AwsDynamicSecretEphemeral{}).Schema(context.Background(), ephemeral.SchemaRequest{}, &resp)
	require.Empty(t, resp.Diagnostics)

	return resp
}

func azureEphemeralSchema(t *testing.T) ephemeral.SchemaResponse {
	t.Helper()

	var resp ephemeral.SchemaResponse
	(&AzureDynamicSecretEphemeral{}).Schema(context.Background(), ephemeral.SchemaRequest{}, &resp)
	require.Empty(t, resp.Diagnostics)

	return resp
}

// Every attribute carrying credential material must be marked sensitive, or Terraform
// will print it in plan output and logs.
func TestEphemeralSchemas_CredentialAttributesAreSensitive(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		schema ephemeral.SchemaResponse
		fields []string
	}{
		"aws":   {awsEphemeralSchema(t), []string{"access_key_id", "secret_access_key", "session_token"}},
		"azure": {azureEphemeralSchema(t), []string{"client_id", "client_secret", "tenant_id"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, field := range tc.fields {
				attr, ok := tc.schema.Schema.Attributes[field]
				require.True(t, ok, "schema must declare %q", field)
				assert.True(t, attr.IsSensitive(), "%q must be Sensitive", field)
				assert.True(t, attr.IsComputed(), "%q must be Computed", field)
			}
		})
	}
}

func TestEphemeralSchemas_LookupAttributes(t *testing.T) {
	t.Parallel()

	for name, schemaResp := range map[string]ephemeral.SchemaResponse{
		"aws":   awsEphemeralSchema(t),
		"azure": azureEphemeralSchema(t),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			nameAttr, ok := schemaResp.Schema.Attributes["name"]
			require.True(t, ok)
			assert.True(t, nameAttr.IsRequired(), "name must be Required")

			folderAttr, ok := schemaResp.Schema.Attributes["folder"]
			require.True(t, ok)
			assert.True(t, folderAttr.IsOptional(), "folder must be Optional")
			assert.False(t, folderAttr.IsComputed(), "folder must not be Computed")

			leaseAttr, ok := schemaResp.Schema.Attributes["lease_id"]
			require.True(t, ok, "lease_id must be exposed for auditing")
			assert.True(t, leaseAttr.IsComputed())
		})
	}
}

// An AWS assumed-role lease is never revocable, so offering the knob would promise
// something the credential type cannot deliver. The expiration it does expose is the
// only lifetime control there is.
func TestAwsEphemeralSchema_HasExpirationAndNoRevokeKnob(t *testing.T) {
	t.Parallel()

	attrs := awsEphemeralSchema(t).Schema.Attributes

	expiration, ok := attrs["expiration"]
	require.True(t, ok, "AWS must expose expiration")
	assert.True(t, expiration.IsComputed())

	assert.NotContains(t, attrs, "revoke_on_close", "AWS leases cannot be revoked, so the knob must not exist")
}

// The Azure generate response carries no expiration, so the resource must not invent
// one. revoke_on_close is Optional+Computed because ephemeral schemas have no Default
// and the resolved value is echoed back from Open.
func TestAzureEphemeralSchema_HasRevokeKnobAndNoExpiration(t *testing.T) {
	t.Parallel()

	attrs := azureEphemeralSchema(t).Schema.Attributes

	revoke, ok := attrs["revoke_on_close"]
	require.True(t, ok, "Azure must expose revoke_on_close")
	assert.True(t, revoke.IsOptional(), "revoke_on_close must be Optional")
	assert.True(t, revoke.IsComputed(), "revoke_on_close must be Computed so Open can echo the default")

	assert.NotContains(t, attrs, "expiration", "the Azure generate response has no expiration to report")
	assert.Contains(t, attrs, "key_id", "Azure must expose key_id to identify the password credential")
}

// The Close asymmetry between the two types is deliberate and load-bearing. Asserting
// it here means flipping either one is a conscious change rather than a silent one.
func TestEphemeralCloseSupport_IsAsymmetric(t *testing.T) {
	t.Parallel()

	_, awsImplementsClose := NewAwsDynamicSecretEphemeral().(ephemeral.EphemeralResourceWithClose)
	assert.False(t, awsImplementsClose, "AWS leases are not revocable, so Close must not be implemented")

	_, azureImplementsClose := NewAzureDynamicSecretEphemeral().(ephemeral.EphemeralResourceWithClose)
	assert.True(t, azureImplementsClose, "Azure credentials are revocable and must be cleaned up on Close")
}

// Neither type may set RenewAt: there is no renew endpoint, and the framework requires
// a Renew implementation whenever RenewAt is set.
func TestEphemeralResources_DoNotSupportRenew(t *testing.T) {
	t.Parallel()

	_, awsRenew := NewAwsDynamicSecretEphemeral().(ephemeral.EphemeralResourceWithRenew)
	assert.False(t, awsRenew)

	_, azureRenew := NewAzureDynamicSecretEphemeral().(ephemeral.EphemeralResourceWithRenew)
	assert.False(t, azureRenew)
}
