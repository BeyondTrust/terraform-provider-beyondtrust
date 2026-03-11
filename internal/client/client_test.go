package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		apiPathVersion string
		endpoint       string
		want           string
	}{
		{
			name:           "no path version",
			apiPathVersion: "",
			endpoint:       "/folders",
			want:           "/secrets/folders",
		},
		{
			name:           "with path version",
			apiPathVersion: "v1",
			endpoint:       "/folders",
			want:           "/secrets/v1/folders",
		},
		{
			name:           "root endpoint",
			apiPathVersion: "",
			endpoint:       "/",
			want:           "/secrets/",
		},
		{
			name:           "complex path no version",
			apiPathVersion: "",
			endpoint:       "/folders/production/secrets",
			want:           "/secrets/folders/production/secrets",
		},
		{
			name:           "complex path with version",
			apiPathVersion: "v2",
			endpoint:       "/folders/production/secrets",
			want:           "/secrets/v2/folders/production/secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
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
	tests := []struct {
		name     string
		apiError *APIError
		want     string
	}{
		{
			name: "error with code",
			apiError: &APIError{
				Message: "Resource not found",
				Code:    "NOT_FOUND",
			},
			want: "Resource not found (code: NOT_FOUND)",
		},
		{
			name: "error without code",
			apiError: &APIError{
				Message: "Internal server error",
			},
			want: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.apiError.Error()
			assert.Equal(t, tt.want, got)
		})
	}
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
