package resources

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIdpCategoryValidator(t *testing.T) {
	cases := []struct {
		value   string
		wantErr bool
	}{
		{"GitHubActions", false},
		{"AzureEntra", false},
		{"Custom", false},
		{"githubactions", true},
		{"Other", true},
		{"", true},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			req := validator.StringRequest{ConfigValue: types.StringValue(tc.value), Path: path.Root("idp_category")}
			resp := &validator.StringResponse{}
			idpCategoryValidator{}.ValidateString(context.Background(), req, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantErr {
				t.Fatalf("value %q: hasError=%v, want %v", tc.value, got, tc.wantErr)
			}
		})
	}
}

func TestIdpCategoryValidator_SkipsUnknownAndNull(t *testing.T) {
	for _, v := range []types.String{types.StringNull(), types.StringUnknown()} {
		resp := &validator.StringResponse{}
		idpCategoryValidator{}.ValidateString(context.Background(), validator.StringRequest{ConfigValue: v, Path: path.Root("idp_category")}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected no error for null/unknown, got %v", resp.Diagnostics)
		}
	}
}

func TestHasAtMostOneTrailingWildcard(t *testing.T) {
	cases := map[string]bool{
		"plain":     true,
		"trailing*": true,
		"*":         true,
		"mid*dle":   false,
		"two**":     false,
		"*lead":     false,
	}
	for value, want := range cases {
		if got := hasAtMostOneTrailingWildcard(value); got != want {
			t.Errorf("hasAtMostOneTrailingWildcard(%q)=%v, want %v", value, got, want)
		}
	}
}

func TestHasValidGitHubSub(t *testing.T) {
	cases := map[string]bool{
		"repo:org/repo:ref:refs/heads/main": true,
		"repo:org/repo*":                    true,
		"repo:org/*":                        true,
		"repo:*":                            false, // wildcard before the slash
		"repo:org":                          false, // no slash
		"org/repo":                          false, // missing repo: prefix
	}
	for value, want := range cases {
		if got := hasValidGitHubSub(value); got != want {
			t.Errorf("hasValidGitHubSub(%q)=%v, want %v", value, got, want)
		}
	}
}

func TestValidateConditions(t *testing.T) {
	cases := []struct {
		name       string
		idp        string
		conditions map[string][]string
		wantErr    bool
	}{
		{"custom valid", idpCustom, map[string][]string{"sub": {"arn:aws:iam::123:role/x"}}, false},
		{"empty value", idpCustom, map[string][]string{"sub": {""}}, true},
		{"mid wildcard", idpCustom, map[string][]string{"sub": {"a*b"}}, true},
		{"github valid", idpGitHubActions, map[string][]string{"sub": {"repo:org/repo:ref:refs/heads/main"}, "ref": {"refs/heads/main"}}, false},
		{"github bad key", idpGitHubActions, map[string][]string{"sub": {"repo:org/repo"}, "not_allowed": {"x"}}, true},
		{"github bad sub", idpGitHubActions, map[string][]string{"sub": {"repo:*"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &resource.ValidateConfigResponse{}
			validateConditions(tc.idp, tc.conditions, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantErr {
				t.Fatalf("hasError=%v, want %v (diags: %v)", got, tc.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestIssuerRequestMarshalsExpectedKeys(t *testing.T) {
	b, err := json.Marshal(issuerRequest{
		SiteID:           "site-1",
		ServiceName:      "ci-pipeline",
		IssuerURL:        "https://token.actions.githubusercontent.com",
		IdpCategory:      idpGitHubActions,
		ScopeLevel:       "site",
		RegisteredScopes: []string{"admin"},
		Conditions:       map[string][]string{"sub": {"repo:org/repo"}},
		Description:      "desc",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"siteId", "serviceName", "issuerUrl", "idpCategory", "scopeLevel", "registeredScopes", "conditions", "description"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("marshaled body missing key %q: %s", key, b)
		}
	}
}

func TestIssuerEnvelopeUnmarshal(t *testing.T) {
	const payload = `{"issuer":{"identityId":"id-1","siteId":"site-1","organizationId":"org-1","expectedAud":"aud-1","serviceName":"ci-pipeline","issuerUrl":"https://x","idpCategory":"Custom","scopeLevel":"site","registeredScopes":["admin"],"conditions":{"sub":["x"]},"description":"d"}}`
	var env issuerEnvelope
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		t.Fatal(err)
	}
	if env.Issuer.IdentityID != "id-1" || env.Issuer.OrganizationID != "org-1" {
		t.Fatalf("unexpected envelope decode: %+v", env.Issuer)
	}
	if got := env.Issuer.Conditions["sub"]; len(got) != 1 || got[0] != "x" {
		t.Fatalf("unexpected conditions: %v", env.Issuer.Conditions)
	}
}

func TestApplyComputed_SetsComputedKeepsRequired(t *testing.T) {
	data := &WorkloadIdentityResourceModel{
		ServiceName: types.StringValue("ci-pipeline"),
		IssuerURL:   types.StringValue("https://x"),
		IdpCategory: types.StringValue(idpCustom),
	}
	applyComputed(data, issuer{
		IdentityID:     "id-1",
		OrganizationID: "org-1",
		SiteID:         "site-1",
		ScopeLevel:     "site",
		Description:    "from-api",
	})
	if data.ID.ValueString() != "id-1" || data.OrganizationID.ValueString() != "org-1" {
		t.Fatalf("computed fields not set: %+v", data)
	}
	if data.SiteID.ValueString() != "site-1" || data.ScopeLevel.ValueString() != "site" || data.Description.ValueString() != "from-api" {
		t.Fatalf("optional+computed fields not set: %+v", data)
	}
	// Required identity fields must be left as planned.
	if data.ServiceName.ValueString() != "ci-pipeline" || data.IssuerURL.ValueString() != "https://x" {
		t.Fatalf("required fields should be unchanged: %+v", data)
	}
}

func TestApplyRead_KeepsImmutableFieldsRefreshesMutable(t *testing.T) {
	data := &WorkloadIdentityResourceModel{
		ServiceName: types.StringValue("KEEP"),
		IssuerURL:   types.StringValue("https://KEEP"),
		IdpCategory: types.StringValue(idpCustom),
	}
	diags := applyRead(context.Background(), data, issuer{
		IdentityID:       "id-1",
		ServiceName:      "ci-pipeline",
		IssuerURL:        "https://x",
		SiteID:           "site-1",
		ScopeLevel:       "org",
		Description:      "from-api",
		Conditions:       map[string][]string{"sub": {"x"}},
		RegisteredScopes: []string{"admin"},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	// Immutable identity fields are NOT overwritten from the response.
	if data.ServiceName.ValueString() != "KEEP" || data.IssuerURL.ValueString() != "https://KEEP" {
		t.Fatalf("immutable fields should be kept from state: %+v", data)
	}
	// Mutable/computed fields are refreshed.
	if data.ScopeLevel.ValueString() != "org" || data.SiteID.ValueString() != "site-1" {
		t.Fatalf("mutable/computed fields not refreshed: %+v", data)
	}
	if data.RegisteredScopes.IsNull() || len(data.RegisteredScopes.Elements()) != 1 {
		t.Fatalf("registered_scopes not refreshed: %+v", data.RegisteredScopes)
	}
	if data.Conditions.IsNull() || len(data.Conditions.Elements()) != 1 {
		t.Fatalf("conditions not refreshed: %+v", data.Conditions)
	}
}
