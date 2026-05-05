package resources

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// Shared helper functions for resource implementations

// buildFolderPath constructs the full path from name and parent folder.
// This is the core path construction logic used throughout folder and secret resources.
func buildFolderPath(name, parentFolder string) string {
	// Trim trailing slash from parent if present
	parentFolder = strings.TrimSuffix(parentFolder, "/")

	if parentFolder == "" {
		return name
	}
	return parentFolder + "/" + name
}

// parseImportPath parses an import ID into name and parent folder.
// This handles the path parsing logic for terraform import operations.
func parseImportPath(importID string) (name string, folder string) {
	if importID == "" {
		return "", ""
	}

	parts := strings.Split(importID, "/")
	if len(parts) == 1 {
		return parts[0], ""
	}

	name = parts[len(parts)-1]
	folder = strings.Join(parts[:len(parts)-1], "/")
	return name, folder
}

// buildTagPatch builds a merge-patch map for tag updates.
// It returns a map where:
// - New/updated tags have string pointer values
// - Deleted tags have nil values (per RFC 7396 merge-patch semantics)
func buildTagPatch(oldTags, newTags map[string]string) map[string]*string {
	patch := make(map[string]*string)

	// Add or update tags
	for key, val := range newTags {
		if oldVal, exists := oldTags[key]; !exists || oldVal != val {
			v := val
			patch[key] = &v
		}
	}

	// Delete tags (set to null per RFC 7396)
	for key := range oldTags {
		if _, exists := newTags[key]; !exists {
			patch[key] = nil
		}
	}

	return patch
}

// buildQueryParameters constructs URL query parameters for API operations.
// This centralizes the query parameter logic used across CRUD operations.
func buildQueryParameters(parentFolder, operation string, permanent bool) url.Values {
	query := url.Values{}

	// Add folder parameter if parent folder is specified
	if parentFolder != "" {
		query.Set("folder", parentFolder)
	}

	// Add permanent flag for delete operations
	if operation == "delete" && permanent {
		query.Set("permanent", "true")
	}

	return query
}

// buildFolderQueryParam constructs URL query parameters with just the folder parameter.
// This is used for most CRUD operations (create, read, update).
func buildFolderQueryParam(parentFolder string) url.Values {
	query := url.Values{}
	if parentFolder != "" {
		query.Set("folder", parentFolder)
	}
	return query
}

// isNotFoundError checks if an error indicates a resource was not found (404).
// This is used to determine when to remove a resource from Terraform state.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a typed APIError with 404 status (handles wrapped errors)
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsNotFound()
	}

	// Fallback to string checking for non-APIError errors
	errLower := strings.ToLower(err.Error())
	return strings.Contains(errLower, "404") || strings.Contains(errLower, "not found")
}

// convertTagsToTerraformMap converts API tag response to Terraform types.Map.
// Returns a Terraform Map type from a Go map[string]string.
func convertTagsToTerraformMap(apiTags map[string]string) types.Map {
	if len(apiTags) == 0 {
		return types.MapNull(types.StringType)
	}

	tagsMap := make(map[string]attr.Value)
	for k, v := range apiTags {
		tagsMap[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, tagsMap)
}

// convertTerraformTagsToMap converts Terraform types.Map to Go map[string]string.
// Returns a Go map from a Terraform Map type.
func convertTerraformTagsToMap(terraformTags types.Map) map[string]string {
	tagsMap := make(map[string]string)
	for k, v := range terraformTags.Elements() {
		if strVal, ok := v.(types.String); ok {
			tagsMap[k] = strVal.ValueString()
		}
	}
	return tagsMap
}

// updateResourceTags updates tags for a resource via the metadata/tags endpoint.
// This is shared logic used by both folder and secret resources.
func updateResourceTags(ctx context.Context, client *client.Client, resourcePath string, parentFolder string, tags types.Map) error {
	apiPath := client.BuildPath(resourcePath)

	query := buildFolderQueryParam(parentFolder)

	// Convert Terraform tags to map
	tagsMap := convertTerraformTagsToMap(tags)

	// Use PUT to update tags
	return client.Put(ctx, apiPath, query, tagsMap)
}

// convertTerraformListToStrings converts a Terraform types.List to a Go []string.
// Returns an empty slice if the list is null or empty.
func convertTerraformListToStrings(list types.List) []string {
	if list.IsNull() || len(list.Elements()) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(list.Elements()))
	for _, elem := range list.Elements() {
		if strVal, ok := elem.(types.String); ok {
			result = append(result, strVal.ValueString())
		}
	}
	return result
}

// convertTerraformMapToStringPointers converts a Terraform types.Map to a Go map[string]*string.
// This is useful for API fields that use pointers to distinguish between null and empty values.
// Returns an empty map if the input is null or empty.
func convertTerraformMapToStringPointers(m types.Map) map[string]*string {
	if m.IsNull() || len(m.Elements()) == 0 {
		return map[string]*string{}
	}

	result := make(map[string]*string)
	for k, v := range m.Elements() {
		if strVal, ok := v.(types.String); ok {
			val := strVal.ValueString()
			result[k] = &val
		}
	}
	return result
}
