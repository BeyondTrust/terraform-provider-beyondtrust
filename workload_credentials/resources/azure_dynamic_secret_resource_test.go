//go:build !acceptance
// +build !acceptance

package resources

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/stretchr/testify/assert"
)

// TestAzureDynamicSecretUpdateRequest_MergePatchSemantics verifies that the
// update request marshals correctly under RFC 7396 merge-patch semantics.
// TTL and applicationObjectId are Required in the schema so they will always
// be set on update; nonetheless the JSON tags must be correct.
func TestAzureDynamicSecretUpdateRequest_MergePatchSemantics(t *testing.T) {
	t.Run("set fields marshal with correct JSON keys", func(t *testing.T) {
		ttl := int64(7200)
		appID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

		req := AzureDynamicSecretUpdateRequest{
			Type:                "azure",
			TTL:                 &ttl,
			ApplicationObjectID: &appID,
		}

		data, err := json.Marshal(req)
		assert.NoError(t, err)

		var got map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal(data, &got))

		assert.Equal(t, `"azure"`, string(got["type"]))
		assert.Equal(t, `7200`, string(got["ttl"]))
		assert.Equal(t, `"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"`, string(got["applicationObjectId"]))
	})

	t.Run("type field is always present", func(t *testing.T) {
		req := AzureDynamicSecretUpdateRequest{Type: "azure"}

		data, err := json.Marshal(req)
		assert.NoError(t, err)

		var got map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal(data, &got))

		assert.Equal(t, `"azure"`, string(got["type"]))
	})

	t.Run("nil required fields marshal to JSON null", func(t *testing.T) {
		// TTL and applicationObjectId have no omitempty, so nil marshals to JSON null
		// rather than being omitted. Under RFC 7396 merge-patch, null deletes the
		// server-side value. In practice these fields are always Required and are
		// always set by the Update function; this verifies the tag behavior.
		req := AzureDynamicSecretUpdateRequest{Type: "azure"}

		data, err := json.Marshal(req)
		assert.NoError(t, err)

		var got map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal(data, &got))

		raw, ok := got["ttl"]
		assert.True(t, ok, "ttl must be present in the PATCH body")
		assert.Equal(t, "null", string(raw), "nil ttl must marshal to JSON null")

		raw, ok = got["applicationObjectId"]
		assert.True(t, ok, "applicationObjectId must be present in the PATCH body")
		assert.Equal(t, "null", string(raw), "nil applicationObjectId must marshal to JSON null")
	})
}

// TestValidateAzureTTL tests TTL validation for Azure dynamic secrets.
// Valid range: 3600-86400 seconds (1 hour to 24 hours)
func TestValidateAzureTTL(t *testing.T) {
	tests := []struct {
		name    string
		ttl     int64
		isValid bool
	}{
		{"minimum (3600 = 1 hour)", 3600, true},
		{"maximum (86400 = 24 hours)", 86400, true},
		{"mid-range (14400 = 4 hours)", 14400, true},
		{"below minimum (3599)", 3599, false},
		{"above maximum (86401)", 86401, false},
		{"zero", 0, false},
		{"negative", -1, false},
		{"AWS minimum (900) — too short for Azure", 900, false},
		{"AWS maximum (43200) — valid for Azure too", 43200, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isValid, validateAzureTTL(tt.ttl))
		})
	}
}

// TestAzureDynamicSecretSchema_RequiredFieldsAreNotComputed guards the invariant
// that ttl and application_object_id are Required and not Computed. If they were
// Computed, an unchanged value could appear as Unknown in the plan and the Update
// method's IsNull() check would wrongly skip sending the value to the API.
func TestAzureDynamicSecretSchema_RequiredFieldsAreNotComputed(t *testing.T) {
	r := &AzureDynamicSecretResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	for _, name := range []string{"ttl", "application_object_id"} {
		attr, ok := resp.Schema.Attributes[name]
		assert.True(t, ok, "schema must declare %q", name)
		assert.True(t, attr.IsRequired(), "%q must be Required", name)
		assert.False(t, attr.IsComputed(), "%q must NOT be Computed", name)
	}
}
