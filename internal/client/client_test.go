package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				BaseURL:        "https://api.example.com",
				AccessToken:    "test-token",
				SiteID:         "test-site",
				APIVersion:     "2026-02-16",
				APIPathVersion: "v1",
				Role:           "test-role",
				Insecure:       false,
				Timeout:        "30s",
			},
			wantErr: false,
		},
		{
			name: "valid config without path version",
			config: &Config{
				BaseURL:     "https://api.example.com",
				AccessToken: "test-token",
				SiteID:      "test-site",
				APIVersion:  "2026-02-16",
				Timeout:     "30s",
			},
			wantErr: false,
		},
		{
			name: "valid config with trailing slash",
			config: &Config{
				BaseURL:     "https://api.example.com/",
				AccessToken: "test-token",
				Timeout:     "30s",
			},
			wantErr: false,
		},
		{
			name: "invalid timeout",
			config: &Config{
				BaseURL:     "https://api.example.com",
				AccessToken: "test-token",
				Timeout:     "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, "https://api.example.com", client.BaseURL)
				assert.Equal(t, tt.config.AccessToken, client.AccessToken)
				assert.Equal(t, tt.config.SiteID, client.SiteID)
				assert.Equal(t, tt.config.APIVersion, client.APIVersion)
				assert.Equal(t, tt.config.APIPathVersion, client.APIPathVersion)
				assert.Equal(t, tt.config.Role, client.Role)
			}
		})
	}
}

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name           string
		siteID         string
		apiPathVersion string
		endpoint       string
		want           string
	}{
		{
			name:           "no path version",
			siteID:         "default",
			apiPathVersion: "",
			endpoint:       "/folders",
			want:           "/site/default/secrets/folders",
		},
		{
			name:           "with path version",
			siteID:         "default",
			apiPathVersion: "v1",
			endpoint:       "/folders",
			want:           "/site/default/secrets/v1/folders",
		},
		{
			name:           "root endpoint",
			siteID:         "test-site",
			apiPathVersion: "",
			endpoint:       "/",
			want:           "/site/test-site/secrets/",
		},
		{
			name:           "complex path no version",
			siteID:         "prod",
			apiPathVersion: "",
			endpoint:       "/folders/production/secrets",
			want:           "/site/prod/secrets/folders/production/secrets",
		},
		{
			name:           "complex path with version",
			siteID:         "staging",
			apiPathVersion: "v2",
			endpoint:       "/folders/production/secrets",
			want:           "/site/staging/secrets/v2/folders/production/secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				SiteID:         tt.siteID,
				APIPathVersion: tt.apiPathVersion,
			}
			got := client.BuildPath(tt.endpoint)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDoRequest(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		serverBody     interface{}
		wantErr        bool
		checkError     func(t *testing.T, err error)
	}{
		{
			name:           "successful request",
			serverResponse: http.StatusOK,
			serverBody:     map[string]string{"status": "ok"},
			wantErr:        false,
		},
		{
			name:           "not found",
			serverResponse: http.StatusNotFound,
			serverBody:     map[string]string{"message": "not found"},
			wantErr:        true,
			checkError: func(t *testing.T, err error) {
				// Check error message contains status code or text
				errStr := err.Error()
				assert.True(t, strings.Contains(errStr, "404") || strings.Contains(errStr, "not found"),
					"error should contain 404 or not found, got: %s", errStr)
			},
		},
		{
			name:           "unauthorized",
			serverResponse: http.StatusUnauthorized,
			serverBody:     map[string]string{"message": "unauthorized"},
			wantErr:        true,
			checkError: func(t *testing.T, err error) {
				errStr := err.Error()
				assert.True(t, strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized"),
					"error should contain 401 or unauthorized, got: %s", errStr)
			},
		},
		{
			name:           "internal server error",
			serverResponse: http.StatusInternalServerError,
			serverBody:     map[string]string{"message": "internal error"},
			wantErr:        true,
			checkError: func(t *testing.T, err error) {
				errStr := err.Error()
				assert.True(t, strings.Contains(errStr, "500") || strings.Contains(errStr, "internal"),
					"error should contain 500 or internal, got: %s", errStr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers that should always be set
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"), "Authorization header")
				assert.Equal(t, "2026-02-16", r.Header.Get("bt-secrets-api-version"), "API Version header")
				assert.Equal(t, "test-site", r.Header.Get("X-BT-Site-ID"), "Site ID header")
				// Content-Type is only set when there's a body, so we don't check it for GET

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.serverResponse)
				json.NewEncoder(w).Encode(tt.serverBody)
			}))
			defer server.Close()

			// Create client with server URL as base
			client, err := NewClient(&Config{
				BaseURL:     server.URL,
				AccessToken: "test-token",
				SiteID:      "test-site",
				APIVersion:  "2026-02-16",
				Timeout:     "30s",
			})
			require.NoError(t, err)

			// Make request using DoRequest method
			var result map[string]string
			err = client.DoRequest(context.Background(), "GET", "/test", nil, nil, &result)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkError != nil {
					tt.checkError(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestDoRequestWithHeaders(t *testing.T) {
	// Test that all required headers are set when posting with a body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify all headers
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"), "Authorization header")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Content-Type header")
		assert.Equal(t, "2026-02-16", r.Header.Get("bt-secrets-api-version"), "API Version header")
		assert.Equal(t, "test-site", r.Header.Get("X-BT-Site-ID"), "Site ID header")
		assert.Equal(t, "test-role", r.Header.Get("X-BT-Role"), "Role header")
		assert.Equal(t, "CUSTOM-IDP", r.Header.Get("X-BT-Auth-Type"), "Auth Type header")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Role:        "test-role",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	// Use POST with a body so Content-Type gets set
	var result map[string]string
	body := map[string]string{"test": "data"}
	err = client.DoRequest(context.Background(), "POST", "/test", nil, body, &result)
	assert.NoError(t, err)
}

func TestDoRequestWithoutRole(t *testing.T) {
	// Test that auth type header is not set when role is not provided
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "", r.Header.Get("X-BT-Role"), "Role header should not be set")
		assert.Equal(t, "", r.Header.Get("X-BT-Auth-Type"), "Auth Type header should not be set")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Role:        "", // No role
		Timeout:     "30s",
	})
	require.NoError(t, err)

	var result map[string]string
	err = client.DoRequest(context.Background(), "GET", "/test", nil, nil, &result)
	assert.NoError(t, err)
}

func TestAPIError(t *testing.T) {
	// Test error with code (primary use case)
	apiError := &APIError{
		Message: "Resource not found",
		Code:    "NOT_FOUND",
	}
	assert.Equal(t, "Resource not found (code: NOT_FOUND)", apiError.Error())
}

func TestClientInsecureMode(t *testing.T) {
	client, err := NewClient(&Config{
		BaseURL:     "https://api.example.com",
		AccessToken: "test-token",
		Insecure:    true,
		Timeout:     "30s",
	})
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Verify TLS config is set to skip verification
	transport := client.HTTPClient.Transport.(*http.Transport)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestClientSecureMode(t *testing.T) {
	client, err := NewClient(&Config{
		BaseURL:     "https://api.example.com",
		AccessToken: "test-token",
		Insecure:    false,
		Timeout:     "30s",
	})
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Verify TLS config is NOT set to skip verification
	transport := client.HTTPClient.Transport.(*http.Transport)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
}

// TestEnsureCSRFToken_FromHeader validates CSRF token extraction from response header (primary method).
func TestEnsureCSRFToken_FromHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CSRF token in header
		w.Header().Set("X-CSRF-Token", "test-csrf-from-header")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	// Call ensureCSRFToken
	err = client.ensureCSRFToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "test-csrf-from-header", client.csrfToken)
}

// TestEnsureCSRFToken_Caching validates that CSRF token is cached and not re-fetched.
func TestEnsureCSRFToken_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("X-CSRF-Token", "test-csrf-token")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	// First call should fetch token
	err = client.ensureCSRFToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "test-csrf-token", client.csrfToken)
	assert.Equal(t, 1, callCount)

	// Second call should use cached token
	err = client.ensureCSRFToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "test-csrf-token", client.csrfToken)
	assert.Equal(t, 1, callCount, "CSRF token should be cached, no additional API call")
}

// TestEnsureCSRFToken_NoToken validates behavior when no CSRF token is provided.
func TestEnsureCSRFToken_NoToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No CSRF token in any location
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	// Call ensureCSRFToken - should not error even if no token is found
	err = client.ensureCSRFToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "", client.csrfToken)
}

// TestHandleErrorResponse_StructuredJSON validates error parsing from structured JSON.
func TestHandleErrorResponse_StructuredJSON(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody map[string]interface{}
		wantErrMsg   string
	}{
		{
			name:       "error with code and message",
			statusCode: http.StatusBadRequest,
			responseBody: map[string]interface{}{
				"message": "Invalid request",
				"code":    "INVALID_REQUEST",
			},
			wantErrMsg: "Invalid request (code: INVALID_REQUEST)",
		},
		{
			name:       "error with only message",
			statusCode: http.StatusNotFound,
			responseBody: map[string]interface{}{
				"message": "Resource not found",
			},
			wantErrMsg: "Resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer server.Close()

			client, err := NewClient(&Config{
				BaseURL:     server.URL,
				AccessToken: "test-token",
				SiteID:      "test-site",
				APIVersion:  "2026-02-16",
				Timeout:     "30s",
			})
			require.NoError(t, err)

			var result map[string]string
			err = client.DoRequest(context.Background(), "GET", "/test", nil, nil, &result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

// TestHandleErrorResponse_UnstructuredJSON validates error handling for non-JSON responses.
func TestHandleErrorResponse_UnstructuredJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	var result map[string]string
	err = client.DoRequest(context.Background(), "GET", "/test", nil, nil, &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "Internal Server Error")
}

// TestValidateSession_Success validates successful session validation.
func TestValidateSession_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/site/test-site/secrets/session" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "valid"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	err = client.ValidateSession(context.Background())
	assert.NoError(t, err)
}

// TestValidateSession_Unauthorized validates session validation failure.
func TestValidateSession_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/site/test-site/secrets/session" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Invalid access token",
				"code":    "UNAUTHORIZED",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "invalid-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	err = client.ValidateSession(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session validation failed")
}

// TestMergePatchRequest validates merge-patch content type.
func TestMergePatchRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify merge-patch content type
		assert.Equal(t, "application/merge-patch+json", r.Header.Get("Content-Type"))
		assert.Equal(t, "PATCH", r.Method)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	// Use Patch method which should use merge-patch content type
	err = client.Patch(context.Background(), "/test", nil, map[string]string{"key": "value"})
	assert.NoError(t, err)
}

// TestHTTPMethods validates HTTP method convenience functions.
func TestHTTPMethods(t *testing.T) {
	tests := []struct {
		name   string
		method string
		call   func(client *Client, ctx context.Context) error
	}{
		{
			name:   "GET without body",
			method: "GET",
			call: func(client *Client, ctx context.Context) error {
				var result map[string]string
				return client.Get(ctx, "/test", nil, &result)
			},
		},
		{
			name:   "POST with body",
			method: "POST",
			call: func(client *Client, ctx context.Context) error {
				var result map[string]string
				return client.Post(ctx, "/test", nil, map[string]string{"key": "value"}, &result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.method, r.Method)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			}))
			defer server.Close()

			client, err := NewClient(&Config{
				BaseURL:     server.URL,
				AccessToken: "test-token",
				SiteID:      "test-site",
				APIVersion:  "2026-02-16",
				Timeout:     "30s",
			})
			require.NoError(t, err)

			err = tt.call(client, context.Background())
			assert.NoError(t, err)
		})
	}
}

// TestDoRequest_NoContent validates handling of 204 No Content responses.
func TestDoRequest_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	err = client.DoRequest(context.Background(), "DELETE", "/test", nil, nil, nil)
	assert.NoError(t, err)
}

// TestDoRequest_QueryParameters validates query parameter encoding.
func TestDoRequest_QueryParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters
		assert.Equal(t, "test value", r.URL.Query().Get("path"))
		assert.Equal(t, "true", r.URL.Query().Get("permanent"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-02-16",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	query := url.Values{}
	query.Set("path", "test value")
	query.Set("permanent", "true")

	var result map[string]string
	err = client.Get(context.Background(), "/test", query, &result)
	assert.NoError(t, err)
}
