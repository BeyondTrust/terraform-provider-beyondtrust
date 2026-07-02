package validators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const uuidPatternStr = `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`

type uuidValidator struct{}

func (v uuidValidator) Description(_ context.Context) string {
	return "value must be a valid UUID (e.g. 550e8400-e29b-41d4-a716-446655440000)"
}

func (v uuidValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v uuidValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if !uuidPattern.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid UUID",
			fmt.Sprintf("Value %q is not a valid UUID. Expected format: %s", value, uuidPatternStr),
		)
	}
}

func UUIDValidator() validator.String {
	return uuidValidator{}
}

func IsValidUUID(value string) bool {
	return uuidPattern.MatchString(value)
}
