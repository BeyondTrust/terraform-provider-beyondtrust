package validators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var (
	resourceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9\-_@~\*\^]{1,130}$`)
	folderPathPattern   = regexp.MustCompile(`^[a-zA-Z0-9\-_@~\*\^]{1,130}(/[a-zA-Z0-9\-_@~\*\^]{1,130})*$`)
)

const (
	resourceNamePatternStr = `^[a-zA-Z0-9\-_@~\*\^]{1,130}$`
	folderPathPatternStr   = `^[a-zA-Z0-9\-_@~\*\^]{1,130}(/[a-zA-Z0-9\-_@~\*\^]{1,130})*$`
)

type resourceNameValidator struct{}

func (v resourceNameValidator) Description(_ context.Context) string {
	return "value must match " + resourceNamePatternStr
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
			fmt.Sprintf("Name %q must match pattern: %s (1-130 chars from [a-zA-Z0-9-_@~*^], no slashes)", value, resourceNamePatternStr),
		)
	}
}

func ResourceNameValidator() validator.String {
	return resourceNameValidator{}
}

func IsValidResourceName(name string) bool {
	return resourceNamePattern.MatchString(name)
}

type folderPathValidator struct{}

func (v folderPathValidator) Description(_ context.Context) string {
	return "value must match " + folderPathPatternStr
}

func (v folderPathValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v folderPathValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if value == "" {
		return
	}
	if !folderPathPattern.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Folder Path",
			fmt.Sprintf("Folder %q must match pattern: %s (slash-separated segments of 1-130 chars from [a-zA-Z0-9-_@~*^])", value, folderPathPatternStr),
		)
	}
}

func FolderPathValidator() validator.String {
	return folderPathValidator{}
}

func IsValidFolderPath(path string) bool {
	if path == "" {
		return true
	}
	return folderPathPattern.MatchString(path)
}
