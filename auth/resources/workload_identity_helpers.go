package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// applyComputed sets the server-assigned and optional+computed fields from a create/update
// response. The required, config-driven fields (service_name, issuer_url, idp_category,
// conditions, registered_scopes) are intentionally left as their planned values so the
// post-apply state matches the plan.
func applyComputed(data *WorkloadIdentityResourceModel, iss issuer) diag.Diagnostics {
	data.ID = types.StringValue(iss.IdentityID)
	data.OrganizationID = types.StringValue(iss.OrganizationID)
	data.ExpectedAud = types.StringValue(iss.ExpectedAud)
	data.SiteID = types.StringValue(iss.SiteID)
	data.ScopeLevel = types.StringValue(iss.ScopeLevel)
	data.Description = types.StringValue(iss.Description)
	return nil
}

// applyRead refreshes state from a GET response for drift detection. The immutable identity
// fields (service_name, issuer_url, idp_category) are left as their state values: they can't
// change through this API, and the auth service lower-cases the issuer URL / service name in storage,
// so echoing the response back would cause spurious diffs.
func applyRead(ctx context.Context, data *WorkloadIdentityResourceModel, iss issuer) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(iss.IdentityID)
	data.OrganizationID = types.StringValue(iss.OrganizationID)
	data.ExpectedAud = types.StringValue(iss.ExpectedAud)
	data.SiteID = types.StringValue(iss.SiteID)
	data.ScopeLevel = types.StringValue(iss.ScopeLevel)
	data.Description = types.StringValue(iss.Description)

	conditions, d := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, iss.Conditions)
	diags.Append(d...)
	data.Conditions = conditions

	scopes, d := types.ListValueFrom(ctx, types.StringType, iss.RegisteredScopes)
	diags.Append(d...)
	data.RegisteredScopes = scopes

	return diags
}

// isNotFound reports whether err is (or wraps) a 404 from the API client.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsNotFound()
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found")
}

// validateConditions mirrors the auth service condition rules so violations surface at plan time.
func validateConditions(idpCategory string, conditions map[string][]string, resp *resource.ValidateConfigResponse) {
	condPath := path.Root("conditions")

	for key, values := range conditions {
		for _, value := range values {
			if value == "" {
				resp.Diagnostics.AddAttributeError(condPath,
					"Invalid condition value",
					fmt.Sprintf("Condition %q has an empty value, which cannot match anything in an incoming token.", key))
				continue
			}
			if len(value) > maxConditionValueLength {
				resp.Diagnostics.AddAttributeError(condPath,
					"Invalid condition value",
					fmt.Sprintf("Condition %q has a value exceeding the %d-character limit.", key, maxConditionValueLength))
			}
			if !hasAtMostOneTrailingWildcard(value) {
				resp.Diagnostics.AddAttributeError(condPath,
					"Invalid condition value",
					fmt.Sprintf("Condition %q values may include at most one '*', and only at the end.", key))
			}
		}
	}

	if idpCategory == idpGitHubActions {
		for key, values := range conditions {
			if _, ok := gitHubActionsAllowedKeys[key]; !ok {
				resp.Diagnostics.AddAttributeError(condPath,
					"Invalid condition key for GitHub Actions",
					fmt.Sprintf("Condition key %q is not allowed for GitHub Actions.", key))
				continue
			}
			if key == "sub" {
				for _, value := range values {
					if !hasValidGitHubSub(value) {
						resp.Diagnostics.AddAttributeError(condPath,
							"Invalid GitHub Actions 'sub' constraint",
							"'sub' for GitHub Actions must start with 'repo:<owner>/<repo>' (any '*' must come after the slash).")
					}
				}
			}
		}
	}
}

func hasAtMostOneTrailingWildcard(value string) bool {
	first := strings.Index(value, "*")
	if first < 0 {
		return true
	}
	return first == len(value)-1
}

func hasValidGitHubSub(value string) bool {
	const prefix = "repo:"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	prefixEnd := len(value)
	if star := strings.Index(value, "*"); star >= 0 {
		prefixEnd = star
	}
	return strings.Contains(value[len(prefix):prefixEnd], "/")
}

// idpCategoryValidator validates idp_category against the allowed set.
type idpCategoryValidator struct{}

func (v idpCategoryValidator) Description(_ context.Context) string {
	return "idp_category must be one of: " + strings.Join(validIdpCategories, ", ")
}

func (v idpCategoryValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v idpCategoryValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	for _, allowed := range validIdpCategories {
		if value == allowed {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path,
		"Invalid idp_category",
		fmt.Sprintf("idp_category must be one of: %s. Got %q.", strings.Join(validIdpCategories, ", "), value))
}
