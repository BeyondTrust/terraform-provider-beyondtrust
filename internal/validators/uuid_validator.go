package validators

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

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
	if err := uuid.Validate(value); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid UUID",
			fmt.Sprintf("Value %q is not a valid UUID: %s", value, err),
		)
	}
}

func UUIDValidator() validator.String {
	return uuidValidator{}
}

func IsValidUUID(value string) bool {
	return uuid.Validate(value) == nil
}
