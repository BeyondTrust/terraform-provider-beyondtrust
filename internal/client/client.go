package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAPIVersion is the default API version header value
const DefaultAPIVersion = "2026-04-28"

// Client is the BeyondTrust API client
type Client struct {
	BaseURL        string
	AccessToken    string
	SiteID         string
	APIVersion     string // Header version (date-based, YYYY-MM-DD)
	APIPathVersion string // Optional path version (e.g., "v1" or empty string)
	Role           string // X-BT-Role header value (when set, auth type is always CUSTOM-IDP)
	HTTPClient     *http.Client
	ServiceName    string // Optional service name for user agent
}

// Config holds the client configuration
type Config struct {
	BaseURL        string
	AccessToken    string
	SiteID         string
	APIVersion     string // Header version (date-based)
	APIPathVersion string // Optional path version
	Role           string // X-BT-Role header value (when set, auth type is always CUSTOM-IDP)
	ServiceName    string // X-BT-Service-Name header value (for GitHub OIDC authentication)
	Insecure       bool
	Timeout        string
}

// APIError represents an error response from the API
type APIError struct {
	Message    string                 `json:"message"`
	Code       string                 `json:"code,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	StatusCode int                    // HTTP status code
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s (code: %s)", e.Message, e.Code)
	}
	// Include status code for unstructured responses (when there's no error code)
	if e.StatusCode >= 400 {
		return fmt.Sprintf("%s (status: %d)", e.Message, e.StatusCode)
	}
	return e.Message
}

// IsNotFound returns true if the error is a 404 Not Found
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsGone returns true if the error indicates the resource no longer exists
func (e *APIError) IsGone() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsPermissionError returns true for 403 Forbidden or 401 Unauthorized
func (e *APIError) IsPermissionError() bool {
	return e.StatusCode == http.StatusForbidden ||
		e.StatusCode == http.StatusUnauthorized
}

// IsConflict returns true if the error is a 409 Conflict
func (e *APIError) IsConflict() bool {
	return e.StatusCode == http.StatusConflict
}

// IsBadRequest returns true if the error is a 400 Bad Request
func (e *APIError) IsBadRequest() bool {
	return e.StatusCode == http.StatusBadRequest
}

// IsServerError returns true if the error is a 5xx Server Error
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= http.StatusInternalServerError && e.StatusCode < 600
}

// IsAWSCredentialValidationError returns true if the error is an AWS credential validation failure
func (e *APIError) IsAWSCredentialValidationError() bool {
	return e.Code == "aws_integration_test_failed" ||
		e.Code == "aws_credential_validation_failed" ||
		strings.Contains(strings.ToLower(e.Message), "failed to validate aws integration credentials")
}

// NewClient creates a new BeyondTrust API client
func NewClient(cfg *Config) (*Client, error) {
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration: %w", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
		},
	}

	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	// Ensure BaseURL doesn't have trailing slash
	baseURL := cfg.BaseURL
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	// Parse and validate the base URL to prevent SSRF via fragment or query injection.
	// Raw string checks catch cases like bare "#" that url.Parse silently discards.
	if strings.Contains(baseURL, "#") {
		return nil, errors.New("api_url must not contain a URL fragment (#)")
	}
	if strings.Contains(baseURL, "?") {
		return nil, errors.New("api_url must not contain a query string (?)")
	}
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid api_url: %w", err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, errors.New("api_url must include a scheme and host")
	}

	return &Client{
		BaseURL:        baseURL,
		AccessToken:    cfg.AccessToken,
		SiteID:         cfg.SiteID,
		APIVersion:     cfg.APIVersion,
		APIPathVersion: cfg.APIPathVersion,
		Role:           cfg.Role,
		ServiceName:    cfg.ServiceName,
		HTTPClient:     httpClient,
	}, nil
}

// BuildPath constructs an API path with optional version segment
// Format: /site/{site-id}/secrets[/version]/endpoint
func (c *Client) BuildPath(endpoint string) string {
	if c.APIPathVersion == "" {
		return fmt.Sprintf("/site/%s/secrets%s", c.SiteID, endpoint)
	}
	return fmt.Sprintf("/site/%s/secrets/%s%s", c.SiteID, c.APIPathVersion, endpoint)
}

// ValidateSession validates the access token by calling GET /session.
// A 200 response indicates the credentials are valid.
func (c *Client) ValidateSession(ctx context.Context) error {
	path := c.BuildPath("/session")

	req, err := c.newRequest(ctx, "GET", path, nil, nil)
	if err != nil {
		return fmt.Errorf("error creating session validation request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("session validation failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// newRequest creates a new HTTP request with standard headers
func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body interface{}) (*http.Request, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL: %w", err)
	}
	u.Path += path

	if query != nil {
		u.RawQuery = query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set standard headers
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("bt-secrets-api-version", c.APIVersion)
	req.Header.Set("Accept", "application/json")

	// Set optional headers if provided
	// When role is provided, auth type is always CUSTOM-IDP
	if c.Role != "" {
		req.Header.Set("X-BT-Role", c.Role)
		req.Header.Set("X-BT-Auth-Type", "CUSTOM-IDP")
	}

	// Set service name header for GitHub OIDC authentication
	if c.ServiceName != "" {
		req.Header.Set("X-BT-Service-Name", c.ServiceName)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// newMergePatchRequest creates a new HTTP PATCH request with merge-patch+json content type
func (c *Client) newMergePatchRequest(ctx context.Context, path string, query url.Values, body interface{}) (*http.Request, error) {
	req, err := c.newRequest(ctx, "PATCH", path, query, body)
	if err != nil {
		return nil, err
	}

	// Override content type for merge patch
	req.Header.Set("Content-Type", "application/merge-patch+json")

	return req, nil
}

// do performs the HTTP request
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return resp, c.handleErrorResponse(resp)
	}

	return resp, nil
}

// handleErrorResponse parses and returns an error from the API response
func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "failed to read error response",
		}
	}

	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		// If we can't parse the error, return the raw response as an APIError
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	// Capture the HTTP status code
	apiErr.StatusCode = resp.StatusCode

	return &apiErr
}

// DoRequest performs a request and unmarshals the response
func (c *Client) DoRequest(ctx context.Context, method, path string, query url.Values, body interface{}, result interface{}) error {
	var req *http.Request
	var err error

	if method == "PATCH" && body != nil {
		req, err = c.newMergePatchRequest(ctx, path, query, body)
	} else {
		req, err = c.newRequest(ctx, method, path, query, body)
	}

	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle 204 No Content
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	// Parse response if result is provided
	if result != nil {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response body: %w", err)
		}

		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("error unmarshaling response: %w", err)
		}
	}

	return nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, query url.Values, result interface{}) error {
	return c.DoRequest(ctx, "GET", path, query, nil, result)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, query url.Values, body interface{}, result interface{}) error {
	return c.DoRequest(ctx, "POST", path, query, body, result)
}

// Put performs a PUT request and either creates a resource or replaces an existing one with what is provided
func (c *Client) Put(ctx context.Context, path string, query url.Values, body interface{}, result interface{}) error {
	return c.DoRequest(ctx, "PUT", path, query, body, result)
}

// Patch performs a PATCH request with merge-patch+json semantics
func (c *Client) Patch(ctx context.Context, path string, query url.Values, body interface{}) error {
	return c.DoRequest(ctx, "PATCH", path, query, body, nil)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string, query url.Values) error {
	return c.DoRequest(ctx, "DELETE", path, query, nil, nil)
}
