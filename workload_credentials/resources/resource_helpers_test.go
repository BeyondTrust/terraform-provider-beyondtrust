package resources

import (
	"fmt"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/stretchr/testify/assert"
)

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}

// This file tests SHARED business logic helpers used across multiple resources.
// Resource-specific tests belong in their respective _unit_test.go files.

// TestBuildFolderPath tests path construction logic
// Used by: folders, secrets (any path-based resource)
func TestBuildFolderPath(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		parentFolder string
		expectedPath string
		description  string
	}{
		{
			name:         "root level resource",
			resourceName: "production",
			parentFolder: "",
			expectedPath: "production",
			description:  "Root level should have name only as path",
		},
		{
			name:         "nested resource",
			resourceName: "credentials",
			parentFolder: "production/aws",
			expectedPath: "production/aws/credentials",
			description:  "Nested resource should combine parent and name",
		},
		{
			name:         "deep nesting",
			resourceName: "keys",
			parentFolder: "prod/aws/us-east-1",
			expectedPath: "prod/aws/us-east-1/keys",
			description:  "Deep nesting should preserve full path hierarchy",
		},
		{
			name:         "trailing slash in parent",
			resourceName: "child",
			parentFolder: "parent/",
			expectedPath: "parent/child",
			description:  "Trailing slash should be normalized",
		},
		{
			name:         "special characters",
			resourceName: "my-resource_123",
			parentFolder: "",
			expectedPath: "my-resource_123",
			description:  "Valid special characters should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFolderPath(tt.resourceName, tt.parentFolder)
			assert.Equal(t, tt.expectedPath, result, tt.description)
		})
	}
}

// TestParseImportPath tests import ID parsing logic
// Used by: folders, secrets (any importable resource)
func TestParseImportPath(t *testing.T) {
	tests := []struct {
		name           string
		importID       string
		expectedName   string
		expectedFolder string
		description    string
	}{
		{
			name:           "simple root resource",
			importID:       "production",
			expectedName:   "production",
			expectedFolder: "",
			description:    "Root level resource with simple name",
		},
		{
			name:           "nested resource",
			importID:       "production/aws/credentials",
			expectedName:   "credentials",
			expectedFolder: "production/aws",
			description:    "Nested resource should parse correctly",
		},
		{
			name:           "deep nesting",
			importID:       "a/b/c/d/e",
			expectedName:   "e",
			expectedFolder: "a/b/c/d",
			description:    "Deep nesting should work",
		},
		{
			name:           "special characters",
			importID:       "my-resource_123",
			expectedName:   "my-resource_123",
			expectedFolder: "",
			description:    "Special characters should be preserved",
		},
		{
			name:           "empty import ID",
			importID:       "",
			expectedName:   "",
			expectedFolder: "",
			description:    "Empty import ID should return empty values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, folder := parseImportPath(tt.importID)

			assert.Equal(t, tt.expectedName, name, "Name should match")
			assert.Equal(t, tt.expectedFolder, folder, "Folder should match")

			// Verify round-trip: parse → build → should equal original
			if tt.importID != "" {
				reconstructed := buildFolderPath(name, folder)
				assert.Equal(t, tt.importID, reconstructed, "Round-trip should preserve path")
			}
		})
	}
}

// TestBuildTagPatch tests merge-patch semantics for tag operations
// Used by: folders, secrets (any taggable resource)
func TestBuildTagPatch(t *testing.T) {
	tests := []struct {
		name          string
		oldTags       map[string]string
		newTags       map[string]string
		expectedPatch map[string]*string
		description   string
	}{
		{
			name:    "add new tags",
			oldTags: map[string]string{},
			newTags: map[string]string{
				"env":  "prod",
				"team": "platform",
			},
			expectedPatch: map[string]*string{
				"env":  stringPtr("prod"),
				"team": stringPtr("platform"),
			},
			description: "Adding tags to resource without tags",
		},
		{
			name: "update existing tag",
			oldTags: map[string]string{
				"env": "dev",
			},
			newTags: map[string]string{
				"env": "prod",
			},
			expectedPatch: map[string]*string{
				"env": stringPtr("prod"),
			},
			description: "Updating an existing tag value",
		},
		{
			name: "delete tag (RFC 7396 null semantics)",
			oldTags: map[string]string{
				"env":   "dev",
				"owner": "alice",
			},
			newTags: map[string]string{
				"env": "dev",
			},
			expectedPatch: map[string]*string{
				"owner": nil, // nil = delete in merge-patch
			},
			description: "Deleting a tag should send null value",
		},
		{
			name: "mixed operations",
			oldTags: map[string]string{
				"env":   "dev",
				"owner": "alice",
			},
			newTags: map[string]string{
				"env":  "prod",     // Update
				"team": "platform", // Add
				// "owner" removed  // Delete
			},
			expectedPatch: map[string]*string{
				"env":   stringPtr("prod"),     // Update
				"team":  stringPtr("platform"), // Add
				"owner": nil,                   // Delete
			},
			description: "Mixed add/update/delete operations",
		},
		{
			name: "no changes",
			oldTags: map[string]string{
				"env": "prod",
			},
			newTags: map[string]string{
				"env": "prod",
			},
			expectedPatch: map[string]*string{},
			description:   "No changes should produce empty patch",
		},
		{
			name:          "both empty",
			oldTags:       map[string]string{},
			newTags:       map[string]string{},
			expectedPatch: map[string]*string{},
			description:   "Empty to empty should produce no patch",
		},
		{
			name:    "nil old tags",
			oldTags: nil,
			newTags: map[string]string{
				"env": "prod",
			},
			expectedPatch: map[string]*string{
				"env": stringPtr("prod"),
			},
			description: "Nil old tags treated as empty",
		},
		{
			name: "clear all tags",
			oldTags: map[string]string{
				"env":  "dev",
				"team": "platform",
			},
			newTags: map[string]string{},
			expectedPatch: map[string]*string{
				"env":  nil,
				"team": nil,
			},
			description: "Clearing all tags sends nulls for each",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch := buildTagPatch(tt.oldTags, tt.newTags)

			assert.Equal(t, len(tt.expectedPatch), len(patch), "Patch size should match")

			for key, expectedVal := range tt.expectedPatch {
				actualVal, exists := patch[key]
				assert.True(t, exists, "Patch should contain key: %s", key)

				if expectedVal == nil {
					assert.Nil(t, actualVal, "Key %s should be nil (delete)", key)
				} else {
					assert.NotNil(t, actualVal, "Key %s should have value", key)
					if actualVal != nil {
						assert.Equal(t, *expectedVal, *actualVal, "Key %s value should match", key)
					}
				}
			}
		})
	}
}

// TestBuildQueryParameters tests query parameter construction
// Used by: folders, secrets, AWS resources (any resource with query params)
func TestBuildQueryParameters(t *testing.T) {
	tests := []struct {
		name           string
		parentFolder   string
		operation      string
		permanent      bool
		expectedParams map[string]string
		description    string
	}{
		{
			name:           "root resource - no params",
			parentFolder:   "",
			operation:      "read",
			permanent:      false,
			expectedParams: map[string]string{},
			description:    "Root level should have no query parameters",
		},
		{
			name:         "nested resource - folder param",
			parentFolder: "production/aws",
			operation:    "read",
			permanent:    false,
			expectedParams: map[string]string{
				"folder": "production/aws",
			},
			description: "Nested resource includes folder parameter",
		},
		{
			name:         "delete with permanent flag",
			parentFolder: "",
			operation:    "delete",
			permanent:    true,
			expectedParams: map[string]string{
				"permanent": "true",
			},
			description: "Permanent delete includes permanent=true",
		},
		{
			name:         "nested delete with permanent",
			parentFolder: "parent",
			operation:    "delete",
			permanent:    true,
			expectedParams: map[string]string{
				"folder":    "parent",
				"permanent": "true",
			},
			description: "Nested permanent delete has both params",
		},
		{
			name:           "soft delete - no permanent flag",
			parentFolder:   "",
			operation:      "delete",
			permanent:      false,
			expectedParams: map[string]string{},
			description:    "Soft delete has no permanent flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := buildQueryParameters(tt.parentFolder, tt.operation, tt.permanent)

			assert.Equal(t, len(tt.expectedParams), len(query), "Should have correct number of parameters")

			for key, expectedValue := range tt.expectedParams {
				actualValue := query.Get(key)
				assert.Equal(t, expectedValue, actualValue, "Parameter %s should match", key)
			}

			// Verify no unexpected parameters
			for key := range query {
				_, expected := tt.expectedParams[key]
				assert.True(t, expected, "Unexpected parameter: %s", key)
			}
		})
	}
}

// TestIsNotFoundError tests 404 error detection (both typed and fallback paths)
// Used by: all resources (for state cleanup)
func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		want        bool
		description string
	}{
		// Typed APIError path tests
		{
			name: "typed APIError with 404",
			err: &client.APIError{
				Message:    "resource not found",
				StatusCode: 404,
			},
			want:        true,
			description: "Typed APIError with 404 should be detected",
		},
		{
			name: "typed APIError with 401",
			err: &client.APIError{
				Message:    "unauthorized",
				StatusCode: 401,
			},
			want:        false,
			description: "Typed APIError with non-404 status should not be detected",
		},
		{
			name: "typed APIError with 500",
			err: &client.APIError{
				Message:    "internal server error",
				StatusCode: 500,
			},
			want:        false,
			description: "Typed APIError with server error should not be detected",
		},

		// Fallback string-based path tests
		{
			name:        "fmt.Errorf with 404 in message",
			err:         fmt.Errorf("HTTP 404: resource not found"),
			want:        true,
			description: "String error with '404' should be caught by fallback",
		},
		{
			name:        "fmt.Errorf with 'not found' phrase",
			err:         fmt.Errorf("folder not found"),
			want:        true,
			description: "String error with 'not found' should be caught by fallback",
		},
		{
			name:        "fmt.Errorf case insensitive",
			err:         fmt.Errorf("Resource NOT FOUND"),
			want:        true,
			description: "Fallback should be case-insensitive",
		},
		{
			name:        "fmt.Errorf different error",
			err:         fmt.Errorf("permission denied"),
			want:        false,
			description: "Non-404 string errors should not be detected",
		},
		{
			name:        "fmt.Errorf 500 error",
			err:         fmt.Errorf("HTTP 500: internal server error"),
			want:        false,
			description: "Other HTTP errors should not be detected",
		},
		{
			name:        "fmt.Errorf network error",
			err:         fmt.Errorf("connection refused"),
			want:        false,
			description: "Network errors should not be detected",
		},

		// Edge cases
		{
			name:        "nil error",
			err:         nil,
			want:        false,
			description: "Nil error should return false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFoundError(tt.err)
			assert.Equal(t, tt.want, result, tt.description)
		})
	}
}

// Note: stringPtr helper is defined in folder_resource_test.go (acceptance tests)
// and is available to all test files in this package.
