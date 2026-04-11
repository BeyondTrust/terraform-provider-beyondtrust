package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

// This file contains secret-specific unit tests.
// Shared helper tests are in resource_helpers_test.go

// TestConvertSecretMap tests Terraform types.Map → Go map conversion
// This is SECRET-SPECIFIC because it handles the secret_wo field
func TestConvertSecretMap(t *testing.T) {
	tests := []struct {
		name         string
		terraformMap map[string]attr.Value
		expectedMap  map[string]string
		description  string
	}{
		{
			name: "single key-value pair",
			terraformMap: map[string]attr.Value{
				"password": types.StringValue("secret123"),
			},
			expectedMap: map[string]string{
				"password": "secret123",
			},
			description: "Single secret key should convert correctly",
		},
		{
			name: "multiple key-value pairs",
			terraformMap: map[string]attr.Value{
				"username": types.StringValue("admin"),
				"password": types.StringValue("secret123"),
				"apikey":   types.StringValue("key-xyz"),
			},
			expectedMap: map[string]string{
				"username": "admin",
				"password": "secret123",
				"apikey":   "key-xyz",
			},
			description: "Multiple secret keys should convert correctly",
		},
		{
			name:         "empty map",
			terraformMap: map[string]attr.Value{},
			expectedMap:  map[string]string{},
			description:  "Empty secret map should convert to empty Go map",
		},
		{
			name: "special characters in values",
			terraformMap: map[string]attr.Value{
				"password": types.StringValue("p@ssw0rd!#$%^&*()"),
			},
			expectedMap: map[string]string{
				"password": "p@ssw0rd!#$%^&*()",
			},
			description: "Special characters in secret values should be preserved",
		},
		{
			name: "whitespace in values",
			terraformMap: map[string]attr.Value{
				"token": types.StringValue("  secret with spaces  "),
			},
			expectedMap: map[string]string{
				"token": "  secret with spaces  ",
			},
			description: "Whitespace in secret values should be preserved",
		},
		{
			name: "empty string value",
			terraformMap: map[string]attr.Value{
				"empty": types.StringValue(""),
			},
			expectedMap: map[string]string{
				"empty": "",
			},
			description: "Empty string values should be allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertSecretMap(tt.terraformMap)

			assert.Equal(t, len(tt.expectedMap), len(result), "Map size should match")

			for key, expectedValue := range tt.expectedMap {
				actualValue, exists := result[key]
				assert.True(t, exists, "Key %s should exist", key)
				assert.Equal(t, expectedValue, actualValue, "Value for key %s should match", key)
			}
		})
	}
}

// TestSecretMapsEqual tests secret value change detection
// This is SECRET-SPECIFIC for the Update method to detect if secret_wo changed
func TestSecretMapsEqual(t *testing.T) {
	tests := []struct {
		name        string
		map1        map[string]string
		map2        map[string]string
		shouldEqual bool
		description string
	}{
		{
			name: "identical maps",
			map1: map[string]string{
				"password": "secret123",
			},
			map2: map[string]string{
				"password": "secret123",
			},
			shouldEqual: true,
			description: "Identical maps should be equal",
		},
		{
			name: "value changed",
			map1: map[string]string{
				"password": "old-secret",
			},
			map2: map[string]string{
				"password": "new-secret",
			},
			shouldEqual: false,
			description: "Changed value should be detected",
		},
		{
			name: "key added",
			map1: map[string]string{
				"password": "secret123",
			},
			map2: map[string]string{
				"password": "secret123",
				"apikey":   "key-xyz",
			},
			shouldEqual: false,
			description: "Added key should be detected",
		},
		{
			name: "key removed",
			map1: map[string]string{
				"password": "secret123",
				"apikey":   "key-xyz",
			},
			map2: map[string]string{
				"password": "secret123",
			},
			shouldEqual: false,
			description: "Removed key should be detected",
		},
		{
			name:        "both empty",
			map1:        map[string]string{},
			map2:        map[string]string{},
			shouldEqual: true,
			description: "Empty maps should be equal",
		},
		{
			name:        "nil vs empty",
			map1:        nil,
			map2:        map[string]string{},
			shouldEqual: true,
			description: "Nil and empty are equivalent (both mean no secrets)",
		},
		{
			name: "order independent",
			map1: map[string]string{
				"a": "1",
				"b": "2",
				"c": "3",
			},
			map2: map[string]string{
				"c": "3",
				"a": "1",
				"b": "2",
			},
			shouldEqual: true,
			description: "Map comparison should be order-independent",
		},
		{
			name: "case sensitive values",
			map1: map[string]string{
				"password": "Secret",
			},
			map2: map[string]string{
				"password": "secret",
			},
			shouldEqual: false,
			description: "Values should be case-sensitive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := secretMapsEqual(tt.map1, tt.map2)
			assert.Equal(t, tt.shouldEqual, result, tt.description)
		})
	}
}
