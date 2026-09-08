package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

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

// newStaleReadTestClient returns a client pointed at server with the stale-read
// retry budget left at its production defaults.
func newStaleReadTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()

	c, err := NewClient(&Config{
		BaseURL:     serverURL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  DefaultAPIVersion,
		Timeout:     "30s",
	})
	require.NoError(t, err)

	return c
}

// TestStaleReadRetry_ForbiddenThenSuccess verifies a GET that first reads before the
// write is visible (403) succeeds once the read converges.
func TestStaleReadRetry_ForbiddenThenSuccess(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "test-secret"})
	}))
	defer server.Close()

	client := newStaleReadTestClient(t, server.URL)

	var result map[string]string
	err := client.Get(context.Background(), client.BuildPath("/static/test-secret"), nil, &result)

	require.NoError(t, err)
	assert.Equal(t, "test-secret", result["name"])
	assert.Equal(t, int32(2), attempts.Load(), "expected one retry after the 403")
}

// TestStaleReadRetry_ExhaustsBudget verifies a 403 that outlives the backoff budget
// is surfaced as a typed *APIError after exactly 8 attempts (7 retries).
//
// This exercises the production constants, so it sleeps through the full
// 25+50+100+200+400+500+500ms schedule and takes ~1.8s. That is deliberate: it is
// the only test that pins the real retry budget end to end.
func TestStaleReadRetry_ExhaustsBudget(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer server.Close()

	client := newStaleReadTestClient(t, server.URL)

	start := time.Now()
	var result map[string]string
	err := client.Get(context.Background(), client.BuildPath("/static/test-secret"), nil, &result)
	elapsed := time.Since(start)

	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.True(t, apiErr.IsPermissionError(), "terminal 403 stays a permission error")

	// The attempt count is exact even with jitter on, because the budget is
	// accounted against the nominal backoff. Only the wall clock varies, so the
	// timing bound allows for jitter running the full 25% short.
	assert.Equal(t, int32(8), attempts.Load(), "expected 8 attempts (7 retries)")
	assert.GreaterOrEqual(t, elapsed, time.Duration(float64(1775*time.Millisecond)*(1-defaultStaleReadRetry().Jitter)),
		"expected to sleep through the 25+50+100+200+400+500+500ms schedule, less jitter")

	// No upper bound on elapsed: the attempt count above already rules out a
	// runaway loop, and TestStaleReadRetry_NewClientDefaults pins the schedule
	// arithmetic exactly. A wall-clock ceiling would only add flake risk on a
	// contended runner.
}

// TestStaleReadRetry_OnlyRetriesGet verifies write methods are never retried, since
// they do not observe eventually consistent reads.
func TestStaleReadRetry_OnlyRetriesGet(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			var attempts atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
			}))
			defer server.Close()

			client := newStaleReadTestClient(t, server.URL)

			err := client.DoRequest(context.Background(), method, client.BuildPath("/static/test-secret"), nil, map[string]string{"k": "v"}, nil)

			require.Error(t, err)
			assert.Equal(t, int32(1), attempts.Load(), "%s must not be retried", method)
		})
	}
}

// TestStaleReadRetry_OnlyRetriesForbidden verifies other failing statuses are returned
// on the first attempt. 401 in particular must not retry: an invalid token will not
// become valid, so retrying only adds latency.
func TestStaleReadRetry_OnlyRetriesForbidden(t *testing.T) {
	for _, status := range []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusInternalServerError,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var attempts atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "boom"})
			}))
			defer server.Close()

			client := newStaleReadTestClient(t, server.URL)

			var result map[string]string
			err := client.Get(context.Background(), client.BuildPath("/static/test-secret"), nil, &result)

			require.Error(t, err)
			assert.Equal(t, int32(1), attempts.Load(), "status %d must not be retried", status)
		})
	}
}

// TestStaleReadRetry_ContextCancellation verifies a cancelled context cuts the retry
// loop short and still yields the typed *APIError callers classify on.
func TestStaleReadRetry_ContextCancellation(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer server.Close()

	client := newStaleReadTestClient(t, server.URL)

	// Stretch the first backoff so the deadline lands deep inside it. This test
	// is about cancellation semantics, not the production schedule, so the exact
	// durations do not matter — only that the sleep is long relative to a request.
	//
	// The margin matters: if the deadline expires while the request is in flight
	// rather than during the sleep, the transport returns a wrapped context error
	// instead of an *APIError and the assertions below fail. Against the real
	// 25ms first backoff, that needed only a 10ms loopback request to trigger,
	// which a contended runner can easily produce.
	client.StaleReadRetry.InitialBackoff = 1 * time.Second
	client.StaleReadRetry.MaxTotalBackoff = 30 * time.Second
	client.StaleReadRetry.Jitter = 0 // keep the sleep exact so the margin is known

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var result map[string]string
	err := client.Get(ctx, client.BuildPath("/static/test-secret"), nil, &result)

	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, int32(1), attempts.Load(),
		"cancellation during the first backoff must stop the loop after one attempt")
}

// TestStaleReadRetry_Disabled verifies the ways of turning retrying off, which is
// also what keeps directly constructed Client literals behaving as single-shot.
func TestStaleReadRetry_Disabled(t *testing.T) {
	tests := []struct {
		name    string
		disable func(*Client)
	}{
		{
			name:    "zero initial backoff",
			disable: func(c *Client) { c.StaleReadRetry.InitialBackoff = 0 },
		},
		{
			// Pins the sentinel to the contract its name claims, so it cannot
			// drift into something that quietly still retries.
			name:    "disabled sentinel",
			disable: func(c *Client) { c.StaleReadRetry = StaleReadRetryDisabled() },
		},
		{
			name:    "zero-value policy",
			disable: func(c *Client) { c.StaleReadRetry = StaleReadRetry{} },
		},
		{
			// The zero-value case above also zeroes InitialBackoff, so the
			// backoff <= 0 check short-circuits and this path never runs. Keep a
			// valid InitialBackoff to reach the budget comparison itself.
			name: "zero total budget with a valid initial backoff",
			disable: func(c *Client) {
				c.StaleReadRetry.InitialBackoff = 25 * time.Millisecond
				c.StaleReadRetry.MaxTotalBackoff = 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
			}))
			defer server.Close()

			client := newStaleReadTestClient(t, server.URL)
			tt.disable(client)

			var result map[string]string
			err := client.Get(context.Background(), client.BuildPath("/static/test-secret"), nil, &result)

			require.Error(t, err)
			assert.Equal(t, int32(1), attempts.Load(), "retrying must be off")
		})
	}
}

// TestStaleReadRetry_NewClientDefaults verifies NewClient wires the production budget.
func TestStaleReadRetry_NewClientDefaults(t *testing.T) {
	c, err := NewClient(&Config{
		BaseURL:     "https://api.example.com",
		AccessToken: "test-token",
		SiteID:      "test-site",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	retry := defaultStaleReadRetry()
	assert.Equal(t, retry, c.StaleReadRetry, "NewClient must install the default policy")

	// Derive the nominal schedule the loop will follow: double until the cap,
	// stopping before the budget is exceeded.
	var elapsed time.Duration
	var sleeps []time.Duration
	for b := retry.InitialBackoff; elapsed+b <= retry.MaxTotalBackoff; {
		sleeps = append(sleeps, b)
		elapsed += b
		if b *= 2; b > retry.MaxBackoff {
			b = retry.MaxBackoff
		}
	}

	assert.Equal(t,
		[]time.Duration{
			25 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond,
			200 * time.Millisecond, 400 * time.Millisecond, 500 * time.Millisecond, 500 * time.Millisecond,
		},
		sleeps, "nominal backoff schedule")
	assert.Equal(t, 1775*time.Millisecond, elapsed, "final probe lands at 1775ms")
	assert.Len(t, sleeps, 7, "7 retries, so 8 attempts")

	for _, s := range sleeps {
		assert.LessOrEqual(t, s, retry.MaxBackoff, "no nominal sleep may exceed the cap")
	}

	// The assertion above is about the nominal schedule, so on its own it says
	// nothing about how long a sleep actually lasts. Pin the real bounds, which
	// jitter puts above MaxBackoff and MaxTotalBackoff rather than at them.
	assert.Equal(t, 625*time.Millisecond,
		time.Duration(float64(retry.MaxBackoff)*(1+retry.Jitter)),
		"longest single sleep once jitter is applied")
	assert.Equal(t, 2218750*time.Microsecond,
		time.Duration(float64(elapsed)*(1+retry.Jitter)),
		"longest total sleep once jitter is applied, which exceeds MaxTotalBackoff")

	// The final probe must stay far enough out to absorb outlier convergence
	// times even when jitter runs short.
	assert.Greater(t, time.Duration(float64(elapsed)*(1-retry.Jitter)), time.Second,
		"worst-case jittered coverage must stay above one second")
}

// TestJitterBackoff verifies jitter stays within its band, actually varies, and can
// be switched off for an exact schedule.
func TestJitterBackoff(t *testing.T) {
	const d = 400 * time.Millisecond
	jitter := defaultStaleReadRetry().Jitter

	t.Run("stays within band and varies", func(t *testing.T) {
		lo := time.Duration(float64(d) * (1 - jitter))
		hi := time.Duration(float64(d) * (1 + jitter))

		seen := make(map[time.Duration]struct{})
		for range 200 {
			got := jitterBackoff(d, jitter)
			assert.GreaterOrEqual(t, got, lo, "jittered sleep below band")
			assert.LessOrEqual(t, got, hi, "jittered sleep above band")
			seen[got] = struct{}{}
		}

		// Lockstep retries are the whole reason jitter exists, so a constant
		// value would defeat the purpose even while staying inside the band.
		assert.Greater(t, len(seen), 100, "jitter should spread values, not return a constant")
	})

	t.Run("disabled by zero or negative jitter", func(t *testing.T) {
		assert.Equal(t, d, jitterBackoff(d, 0))
		assert.Equal(t, d, jitterBackoff(d, -1))
	})

	t.Run("clamped above one", func(t *testing.T) {
		// Unclamped, a fraction above 1 puts the low end of the band below zero,
		// and a negative duration makes the timer fire at once — retrying with no
		// backoff at all. Clamping keeps the sleep positive for any input.
		for _, j := range []float64{1.5, 2, 100} {
			for range 200 {
				got := jitterBackoff(d, j)
				assert.Positive(t, got, "jitter %v must not produce a non-positive sleep", j)
				assert.LessOrEqual(t, got, 2*d, "jitter %v must stay within the clamped band", j)
			}
		}
	})

	t.Run("non-positive duration passes through", func(t *testing.T) {
		assert.Zero(t, jitterBackoff(0, jitter))
	})
}

// TestStaleReadRetry_UncappedWhenMaxBackoffZero verifies a zero cap leaves the
// backoff doubling unbounded, so a hand-built Client keeps working.
func TestStaleReadRetry_UncappedWhenMaxBackoffZero(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer server.Close()

	client := newStaleReadTestClient(t, server.URL)
	client.StaleReadRetry.InitialBackoff = 10 * time.Millisecond
	client.StaleReadRetry.MaxBackoff = 0 // uncapped
	client.StaleReadRetry.MaxTotalBackoff = 70 * time.Millisecond
	client.StaleReadRetry.Jitter = 0

	var result map[string]string
	err := client.Get(context.Background(), client.BuildPath("/static/test-secret"), nil, &result)

	require.Error(t, err)
	// Pure doubling: 10+20+40 = 70ms fits, the next 80ms would not.
	assert.Equal(t, int32(4), attempts.Load(), "uncapped doubling should give 4 attempts")
}

// TestStaleReadRetryApplies covers the retry gate directly, across both the
// method and path dimensions.
func TestStaleReadRetryApplies(t *testing.T) {
	c := &Client{SiteID: "test-site"}
	versioned := &Client{SiteID: "test-site", APIPathVersion: "v1"}

	tests := []struct {
		name   string
		client *Client
		method string
		path   string
		want   bool
	}{
		{"secrets GET", c, "GET", c.BuildPath("/static/name"), true},
		{"secrets GET, bare prefix", c, "GET", c.BuildPath(""), true},
		{"secrets GET, versioned path", versioned, "GET", versioned.BuildPath("/static/name"), true},
		{"secrets GET, nested path", c, "GET", c.BuildPath("/folders/a/metadata"), true},

		// Writes reach the same service but do not observe stale reads.
		{"secrets POST", c, "POST", c.BuildPath("/static/name"), false},
		{"secrets PUT", c, "PUT", c.BuildPath("/static/name"), false},
		{"secrets PATCH", c, "PATCH", c.BuildPath("/static/name"), false},
		{"secrets DELETE", c, "DELETE", c.BuildPath("/static/name"), false},

		// The auth service does not share the behaviour, so its 403s are real.
		{"auth GET", c, "GET", c.BuildAuthPath("/workload-identities"), false},
		{"auth GET, nested", c, "GET", c.BuildAuthPath("/workload-identities/id"), false},

		// Unrecognised paths fall through to a single attempt.
		{"unrelated path", c, "GET", "/test", false},
		{"root", c, "GET", "/", false},
		{"empty", c, "GET", "", false},
		{"another site", c, "GET", "/site/other-site/secrets/static/name", false},
		{"prefix not at segment boundary", c, "GET", "/site/test-site/secretsomething", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.client.staleReadRetryApplies(tt.method, tt.path),
				"%s %s", tt.method, tt.path)
		})
	}
}

// TestStaleReadRetry_OnlyRetriesSecretsPath verifies the gate end to end: an
// identical 403 is retried on the secrets path and returned immediately on the
// auth path, which does not have eventually consistent reads.
func TestStaleReadRetry_OnlyRetriesSecretsPath(t *testing.T) {
	tests := []struct {
		name         string
		path         func(c *Client) string
		wantAttempts int32
	}{
		{"secrets path retries", func(c *Client) string { return c.BuildPath("/static/name") }, 8},
		{"auth path does not", func(c *Client) string { return c.BuildAuthPath("/workload-identities") }, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
			}))
			defer server.Close()

			client := newStaleReadTestClient(t, server.URL)
			// Keep the schedule short; the attempt count is what matters here.
			client.StaleReadRetry.InitialBackoff = time.Millisecond
			client.StaleReadRetry.MaxBackoff = time.Millisecond
			client.StaleReadRetry.MaxTotalBackoff = 7 * time.Millisecond
			client.StaleReadRetry.Jitter = 0

			var result map[string]string
			err := client.Get(context.Background(), tt.path(client), nil, &result)

			require.Error(t, err)

			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
			assert.Equal(t, tt.wantAttempts, attempts.Load())
		})
	}
}

// TestStaleReadRetry_CancelledMidRequest verifies cancellation reports the typed
// *APIError even when it lands while a retry request is in flight, rather than the
// wrapped context error the transport produces for that attempt.
func TestStaleReadRetry_CancelledMidRequest(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Stall every attempt after the first, so the context deadline expires
		// with a request outstanding instead of during a backoff sleep.
		if attempts.Add(1) > 1 {
			time.Sleep(400 * time.Millisecond)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer server.Close()

	client := newStaleReadTestClient(t, server.URL)
	// Reach the second attempt almost immediately, so the deadline below lands
	// inside the stalled request rather than inside a sleep.
	client.StaleReadRetry.InitialBackoff = time.Millisecond
	client.StaleReadRetry.MaxTotalBackoff = time.Second
	client.StaleReadRetry.Jitter = 0

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	var result map[string]string
	err := client.Get(ctx, client.BuildPath("/static/test-secret"), nil, &result)

	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr,
		"cancellation during an in-flight retry must still yield the typed error")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.GreaterOrEqual(t, attempts.Load(), int32(2), "expected to reach a retry")
}
