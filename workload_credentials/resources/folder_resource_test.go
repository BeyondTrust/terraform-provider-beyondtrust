//go:build !acceptance
// +build !acceptance

package resources

// This file contains folder-specific unit tests.
// Shared helper tests are in resource_helpers_test.go

// NOTE: Currently, folder_resource.go uses only shared helpers.
// All business logic tests are in resource_helpers_test.go:
// - buildFolderPath() → TestBuildFolderPath
// - parseImportPath() → TestParseImportPath
// - buildTagPatch() → TestBuildTagPatch
// - buildQueryParameters() → TestBuildQueryParameters
// - isNotFoundError() → TestIsNotFoundError

// If folder-specific logic is added in the future (e.g., folder-only validation,
// special delete behavior, etc.), add tests here.

// Example of what a folder-specific test might look like:
// func TestFolderResource_SpecialFolderBehavior(t *testing.T) {
//     // Test folder-specific logic that doesn't apply to secrets
// }
