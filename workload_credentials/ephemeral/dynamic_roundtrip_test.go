//go:build !acceptance

// These tests drive the real protocol server rather than calling Open and Close
// directly. That is the only way to cover the private state contract: Close receives
// nothing but private state, and the round trip through Terraform core serializes it
// to bytes and back. Calling the methods directly would skip exactly the step most
// likely to break silently.
package ephemeral_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

const azureEphemeralType = "beyondtrust_workload_credentials_azure_dynamic_secret"

// errorDiagnostics reduces protocol diagnostics to the error summaries, so a failing
// assertion names what actually went wrong instead of printing a struct.
func errorDiagnostics(diags []*tfprotov6.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			out = append(out, d.Summary+": "+d.Detail)
		}
	}
	return out
}

// objectValue builds a value for an object type with every attribute null, then applies
// the given overrides. Deriving the shape from the schema keeps these tests working when
// an unrelated attribute is added.
func objectValue(t *testing.T, ty tftypes.Type, overrides map[string]tftypes.Value) tfprotov6.DynamicValue {
	t.Helper()

	obj, ok := ty.(tftypes.Object)
	require.True(t, ok, "expected an object type")

	attrs := make(map[string]tftypes.Value, len(obj.AttributeTypes))
	for name, attrType := range obj.AttributeTypes {
		attrs[name] = tftypes.NewValue(attrType, nil)
	}
	for name, override := range overrides {
		require.Contains(t, attrs, name, "override names an attribute not in the schema")
		attrs[name] = override
	}

	dv, err := tfprotov6.NewDynamicValue(ty, tftypes.NewValue(ty, attrs))
	require.NoError(t, err)

	return dv
}

// configuredServer returns a provider server already pointed at apiURL.
func configuredServer(t *testing.T, apiURL string) (tfprotov6.ProviderServer, *tfprotov6.GetProviderSchemaResponse) {
	t.Helper()

	// Neutralise the settings a developer's .envrc could otherwise inject. The three
	// below are not overridden by the config set here, and api_path_version in
	// particular would insert an extra segment into every request path.
	t.Setenv("BEYONDTRUST_API_PATH_VERSION", "")
	t.Setenv("BEYONDTRUST_ROLE", "")
	t.Setenv("BEYONDTRUST_SERVICE_NAME", "")

	ctx := context.Background()
	srv := providerserver.NewProtocol6(provider.New("test")())()

	schemaResp, err := srv.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	require.Empty(t, errorDiagnostics(schemaResp.Diagnostics))

	cfg := objectValue(t, schemaResp.Provider.ValueType(), map[string]tftypes.Value{
		"api_url":      tftypes.NewValue(tftypes.String, apiURL),
		"access_token": tftypes.NewValue(tftypes.String, "test-token"),
		"site_id":      tftypes.NewValue(tftypes.String, "test-site"),
	})

	confResp, err := srv.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{Config: &cfg})
	require.NoError(t, err)
	require.Empty(t, errorDiagnostics(confResp.Diagnostics))

	return srv, schemaResp
}

// azureCredentialServer answers a generate with a fixed credential and records revokes.
func azureCredentialServer(revokes *atomic.Int32, revokedIDs *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/generate"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"secret":{
				"leaseId":"lease-roundtrip","type":"azure","credentialType":"service_principal_password",
				"clientId":"33333333-3333-3333-3333-333333333333","clientSecret":"generated-password",
				"tenantId":"00000000-0000-0000-0000-000000000000","keyId":"44444444-4444-4444-4444-444444444444"}}`))
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/leases/id/"):
			revokes.Add(1)
			if revokedIDs != nil {
				parts := strings.Split(r.URL.Path, "/leases/id/")
				*revokedIDs = append(*revokedIDs, parts[len(parts)-1])
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// openAzure opens the ephemeral resource with the given config overrides.
func openAzure(t *testing.T, srv tfprotov6.ProviderServer, schemaResp *tfprotov6.GetProviderSchemaResponse, overrides map[string]tftypes.Value) *tfprotov6.OpenEphemeralResourceResponse {
	t.Helper()

	ephemeralSchema, ok := schemaResp.EphemeralResourceSchemas[azureEphemeralType]
	require.True(t, ok, "provider must register %s", azureEphemeralType)

	cfg := objectValue(t, ephemeralSchema.ValueType(), overrides)

	openResp, err := srv.OpenEphemeralResource(context.Background(), &tfprotov6.OpenEphemeralResourceRequest{
		TypeName: azureEphemeralType,
		Config:   &cfg,
	})
	require.NoError(t, err)
	require.Empty(t, errorDiagnostics(openResp.Diagnostics))

	return openResp
}

func TestAzureEphemeral_RevokesLeaseOnClose(t *testing.T) {
	var revokes atomic.Int32
	var revokedIDs []string
	api := azureCredentialServer(&revokes, &revokedIDs)
	defer api.Close()

	srv, schemaResp := configuredServer(t, api.URL)

	openResp := openAzure(t, srv, schemaResp, map[string]tftypes.Value{
		"name": tftypes.NewValue(tftypes.String, "my-secret"),
	})
	require.NotNil(t, openResp.Result)
	require.NotEmpty(t, openResp.Private, "Open must record the lease for Close to revoke")
	assert.Equal(t, int32(0), revokes.Load(), "Open must not revoke the credential it just minted")

	closeResp, err := srv.CloseEphemeralResource(context.Background(), &tfprotov6.CloseEphemeralResourceRequest{
		TypeName: azureEphemeralType,
		Private:  openResp.Private,
	})
	require.NoError(t, err)
	require.Empty(t, errorDiagnostics(closeResp.Diagnostics))

	assert.Equal(t, int32(1), revokes.Load(), "Close must revoke the lease recorded during Open")
	assert.Equal(t, []string{"lease-roundtrip"}, revokedIDs)
}

func TestAzureEphemeral_RevokeOnCloseFalseSuppressesRevoke(t *testing.T) {
	var revokes atomic.Int32
	api := azureCredentialServer(&revokes, nil)
	defer api.Close()

	srv, schemaResp := configuredServer(t, api.URL)

	openResp := openAzure(t, srv, schemaResp, map[string]tftypes.Value{
		"name":            tftypes.NewValue(tftypes.String, "my-secret"),
		"revoke_on_close": tftypes.NewValue(tftypes.Bool, false),
	})

	closeResp, err := srv.CloseEphemeralResource(context.Background(), &tfprotov6.CloseEphemeralResourceRequest{
		TypeName: azureEphemeralType,
		Private:  openResp.Private,
	})
	require.NoError(t, err)
	require.Empty(t, errorDiagnostics(closeResp.Diagnostics))

	assert.Equal(t, int32(0), revokes.Load(), "revoke_on_close = false must leave the credential in place")
}

// A revoke that fails must not fail the Terraform operation: the real work is already
// done by the time Close runs.
func TestAzureEphemeral_CloseWarnsRatherThanErroringOnRevokeFailure(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"secret":{"leaseId":"lease-roundtrip","clientSecret":"pw"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"forbidden","message":"denied"}`))
	}))
	defer api.Close()

	srv, schemaResp := configuredServer(t, api.URL)

	openResp := openAzure(t, srv, schemaResp, map[string]tftypes.Value{
		"name": tftypes.NewValue(tftypes.String, "my-secret"),
	})

	closeResp, err := srv.CloseEphemeralResource(context.Background(), &tfprotov6.CloseEphemeralResourceRequest{
		TypeName: azureEphemeralType,
		Private:  openResp.Private,
	})
	require.NoError(t, err)
	assert.Empty(t, errorDiagnostics(closeResp.Diagnostics), "a failed revoke must not fail the operation")
	assert.NotEmpty(t, closeResp.Diagnostics, "a failed revoke must still be surfaced as a warning")
}

// Close is reachable with no recorded lease (nothing was ever opened). It must be a
// silent no-op rather than an error.
func TestAzureEphemeral_CloseWithoutPrivateStateIsNoOp(t *testing.T) {
	var revokes atomic.Int32
	api := azureCredentialServer(&revokes, nil)
	defer api.Close()

	srv, _ := configuredServer(t, api.URL)

	closeResp, err := srv.CloseEphemeralResource(context.Background(), &tfprotov6.CloseEphemeralResourceRequest{
		TypeName: azureEphemeralType,
	})
	require.NoError(t, err)
	assert.Empty(t, errorDiagnostics(closeResp.Diagnostics))
	assert.Equal(t, int32(0), revokes.Load())
}
