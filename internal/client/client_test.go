package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
				APIVersion:     "DefaultAPIVersion",
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
				APIVersion:  "DefaultAPIVersion",
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
			name: "rejects URL with fragment",
			config: &Config{
				BaseURL:     "https://api.example.com#",
				AccessToken: "test-token",
				Timeout:     "30s",
			},
			wantErr: true,
		},
		{
			name: "rejects URL with fragment and content",
			config: &Config{
				BaseURL:     "https://api.example.com#frag",
				AccessToken: "test-token",
				Timeout:     "30s",
			},
			wantErr: true,
		},
		{
			name: "rejects URL with query string",
			config: &Config{
				BaseURL:     "https://api.example.com?x=",
				AccessToken: "test-token",
				Timeout:     "30s",
			},
			wantErr: true,
		},
		{
			name: "rejects URL with bare question mark",
			config: &Config{
				BaseURL:     "https://api.example.com?",
				AccessToken: "test-token",
				Timeout:     "30s",
			},
			wantErr: true,
		},
		{
			name: "rejects URL without scheme",
			config: &Config{
				BaseURL:     "api.example.com",
				AccessToken: "test-token",
				Timeout:     "30s",
			},
			wantErr: true,
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
				var apiErr *APIError
				require.True(t, errors.As(err, &apiErr), "error should be *client.APIError, got: %T", err)
				assert.Equal(t, http.StatusNotFound, apiErr.StatusCode, "status code should be 404")
				assert.True(t, apiErr.IsNotFound(), "IsNotFound() should return true")
			},
		},
		{
			name:           "unauthorized",
			serverResponse: http.StatusUnauthorized,
			serverBody:     map[string]string{"message": "unauthorized"},
			wantErr:        true,
			checkError: func(t *testing.T, err error) {
				var apiErr *APIError
				require.True(t, errors.As(err, &apiErr), "error should be *client.APIError, got: %T", err)
				assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode, "status code should be 401")
			},
		},
		{
			name:           "internal server error",
			serverResponse: http.StatusInternalServerError,
			serverBody:     map[string]string{"message": "internal error"},
			wantErr:        true,
			checkError: func(t *testing.T, err error) {
				var apiErr *APIError
				require.True(t, errors.As(err, &apiErr), "error should be *client.APIError, got: %T", err)
				assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode, "status code should be 500")
				assert.True(t, apiErr.IsServerError(), "IsServerError() should return true")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers that should always be set
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"), "Authorization header")
				assert.Equal(t, "DefaultAPIVersion", r.Header.Get("bt-secrets-api-version"), "API Version header")
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
				APIVersion:  "DefaultAPIVersion",
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
		assert.Equal(t, "DefaultAPIVersion", r.Header.Get("bt-secrets-api-version"), "API Version header")
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
		APIVersion:  "DefaultAPIVersion",
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
		APIVersion:  "DefaultAPIVersion",
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
			wantErrMsg: "Resource not found (status: 404)",
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
				APIVersion:  "DefaultAPIVersion",
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
		APIVersion:  "DefaultAPIVersion",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	var result map[string]string
	err = client.DoRequest(context.Background(), "GET", "/test", nil, nil, &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Internal Server Error")
	assert.Contains(t, err.Error(), "status: 500")
}

// TestValidateSession_Success validates successful session validation.
func TestValidateSession_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/site/test-site/secrets/session" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "DefaultAPIVersion",
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
		APIVersion:  "DefaultAPIVersion",
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
		APIVersion:  "DefaultAPIVersion",
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
				APIVersion:  "DefaultAPIVersion",
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
		APIVersion:  "DefaultAPIVersion",
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
		APIVersion:  "DefaultAPIVersion",
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

// TestAPIError_HelperMethods validates all APIError helper methods
func TestAPIError_HelperMethods(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		wantNotFound    bool
		wantConflict    bool
		wantBadRequest  bool
		wantServerError bool
	}{
		{
			name:            "404 Not Found",
			statusCode:      http.StatusNotFound,
			wantNotFound:    true,
			wantConflict:    false,
			wantBadRequest:  false,
			wantServerError: false,
		},
		{
			name:            "409 Conflict",
			statusCode:      http.StatusConflict,
			wantNotFound:    false,
			wantConflict:    true,
			wantBadRequest:  false,
			wantServerError: false,
		},
		{
			name:            "400 Bad Request",
			statusCode:      http.StatusBadRequest,
			wantNotFound:    false,
			wantConflict:    false,
			wantBadRequest:  true,
			wantServerError: false,
		},
		{
			name:            "500 Internal Server Error",
			statusCode:      http.StatusInternalServerError,
			wantNotFound:    false,
			wantConflict:    false,
			wantBadRequest:  false,
			wantServerError: true,
		},
		{
			name:            "503 Service Unavailable",
			statusCode:      http.StatusServiceUnavailable,
			wantNotFound:    false,
			wantConflict:    false,
			wantBadRequest:  false,
			wantServerError: true,
		},
		{
			name:            "599 Edge of 5xx range",
			statusCode:      599,
			wantNotFound:    false,
			wantConflict:    false,
			wantBadRequest:  false,
			wantServerError: true,
		},
		{
			name:            "600 Outside 5xx range",
			statusCode:      600,
			wantNotFound:    false,
			wantConflict:    false,
			wantBadRequest:  false,
			wantServerError: false,
		},
		{
			name:            "401 Unauthorized",
			statusCode:      http.StatusUnauthorized,
			wantNotFound:    false,
			wantConflict:    false,
			wantBadRequest:  false,
			wantServerError: false,
		},
		{
			name:            "403 Forbidden",
			statusCode:      http.StatusForbidden,
			wantNotFound:    false,
			wantConflict:    false,
			wantBadRequest:  false,
			wantServerError: false,
		},
		{
			name:            "200 OK (no error)",
			statusCode:      http.StatusOK,
			wantNotFound:    false,
			wantConflict:    false,
			wantBadRequest:  false,
			wantServerError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := &APIError{
				Message:    "test error",
				StatusCode: tt.statusCode,
			}

			assert.Equal(t, tt.wantNotFound, apiErr.IsNotFound(), "IsNotFound()")
			assert.Equal(t, tt.wantConflict, apiErr.IsConflict(), "IsConflict()")
			assert.Equal(t, tt.wantBadRequest, apiErr.IsBadRequest(), "IsBadRequest()")
			assert.Equal(t, tt.wantServerError, apiErr.IsServerError(), "IsServerError()")
		})
	}
}

// TestAPIError_IsAWSCredentialValidationError validates AWS credential validation error detection
func TestAPIError_IsAWSCredentialValidationError(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		want    bool
	}{
		{
			name:    "aws_integration_test_failed code",
			code:    "aws_integration_test_failed",
			message: "some error message",
			want:    true,
		},
		{
			name:    "aws_credential_validation_failed code",
			code:    "aws_credential_validation_failed",
			message: "some error message",
			want:    true,
		},
		{
			name:    "message contains validation failure",
			code:    "",
			message: "Failed to validate AWS integration credentials",
			want:    true,
		},
		{
			name:    "message contains validation failure lowercase",
			code:    "",
			message: "failed to validate aws integration credentials",
			want:    true,
		},
		{
			name:    "message contains validation failure mixed case",
			code:    "",
			message: "FAILED TO VALIDATE AWS INTEGRATION CREDENTIALS",
			want:    true,
		},
		{
			name:    "different error code",
			code:    "some_other_error",
			message: "some error message",
			want:    false,
		},
		{
			name:    "different error message",
			code:    "",
			message: "some other error message",
			want:    false,
		},
		{
			name:    "empty error",
			code:    "",
			message: "",
			want:    false,
		},
		{
			name:    "partial match should not trigger",
			code:    "",
			message: "aws integration error",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := &APIError{
				Code:    tt.code,
				Message: tt.message,
			}

			result := apiErr.IsAWSCredentialValidationError()
			assert.Equal(t, tt.want, result)
		})
	}
}
