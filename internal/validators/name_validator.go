package validators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var resourceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9\-_@~\*\^]+$`)

type resourceNameValidator struct{}

func (v resourceNameValidator) Description(_ context.Context) string {
	return "value must match ^[a-zA-Z0-9\\-_@~\\*\\^]+$"
}

func (v resourceNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v resourceNameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if !resourceNamePattern.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Resource Name",
			fmt.Sprintf("Name %q contains invalid characters. Must match pattern: ^[a-zA-Z0-9\\-_@~\\*\\^]+$", value),
		)
	}
}

func ResourceNameValidator() validator.String {
	return resourceNameValidator{}
}

func IsValidResourceName(name string) bool {
	return resourceNamePattern.MatchString(name)
}
