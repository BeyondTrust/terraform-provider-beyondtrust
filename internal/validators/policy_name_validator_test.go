//go:build !acceptance
// +build !acceptance

package validators

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestIsValidPolicyName(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		want        bool
		description string
	}{
		{"simple", "devops", true, "plain lowercase name"},
		{"with hyphens", "devops-production-access", true, "hyphens are allowed in the interior"},
		{"with underscores", "devops_production_access", true, "underscores are allowed in the interior"},
		{"mixed case and digits", "DevOps2Prod", true, "letters and digits in any case"},
		{"single character", "a", true, "one character satisfies the optional tail group"},
		{"single digit", "7", true, "a digit is a valid boundary character"},
		{"max length", strings.Repeat("a", 100), true, "100 characters is the documented maximum"},

		{"empty", "", false, "empty is not a valid name"},
		{"too long", strings.Repeat("a", 101), false, "101 characters exceeds the maximum"},
		{"leading hyphen", "-devops", false, "must start with a letter or digit"},
		{"trailing hyphen", "devops-", false, "must end with a letter or digit"},
		{"leading underscore", "_devops", false, "must start with a letter or digit"},
		{"trailing underscore", "devops_", false, "must end with a letter or digit"},
		{"contains slash", "devops/prod", false, "slashes are not permitted"},
		{"contains space", "devops prod", false, "spaces are not permitted"},
		{"contains dot", "devops.prod", false, "dots are not permitted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidPolicyName(tt.value), tt.description)
		})
	}
}

func TestPolicyNameValidator(t *testing.T) {
	tests := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{"valid", types.StringValue("devops-production-access"), false},
		{"invalid", types.StringValue("-devops"), true},
		{"null is skipped", types.StringNull(), false},
		{"unknown is skipped", types.StringUnknown(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			PolicyNameValidator().ValidateString(
				context.Background(),
				validator.StringRequest{Path: path.Root("name"), ConfigValue: tt.value},
				resp,
			)
			assert.Equal(t, tt.wantError, resp.Diagnostics.HasError())
		})
	}
}
