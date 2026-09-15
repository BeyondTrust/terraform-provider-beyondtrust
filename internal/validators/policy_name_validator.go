package validators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// policyNamePatternStr mirrors the PolicyName schema the IAM policy service enforces on the
// {name} path parameter: 1-100 characters, alphanumeric at both ends, with hyphens and
// underscores allowed in between.
const policyNamePatternStr = `^[a-zA-Z0-9]([a-zA-Z0-9_-]{0,98}[a-zA-Z0-9])?$`

var policyNamePattern = regexp.MustCompile(policyNamePatternStr)

type policyNameValidator struct{}

func (v policyNameValidator) Description(_ context.Context) string {
	return "value must match " + policyNamePatternStr
}

func (v policyNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v policyNameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if !policyNamePattern.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Policy Name",
			fmt.Sprintf("Name %q must match pattern: %s (1-100 chars, starting and ending with a letter or digit, with hyphens and underscores allowed in between)", value, policyNamePatternStr),
		)
	}
}

// PolicyNameValidator returns a validator enforcing the IAM policy service's name pattern.
func PolicyNameValidator() validator.String {
	return policyNameValidator{}
}

// IsValidPolicyName reports whether name matches the IAM policy service's name pattern. It is
// the plain-Go twin of PolicyNameValidator, used where no validator.String is available (e.g.
// ImportState).
func IsValidPolicyName(name string) bool {
	return policyNamePattern.MatchString(name)
}
