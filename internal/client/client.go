package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAPIVersion is the default API version header value
const DefaultAPIVersion = "2026-04-28"

// DefaultAPIURL is the public BeyondTrust API base URL, used when api_url is not set.
const DefaultAPIURL = "https://api.beyondtrust.io"

// Reads on the BeyondTrust API are eventually consistent: an object that was
// just written may not yet be visible to an immediately following GET, and the
// API reports that as 403 rather than 404. A 403 on a GET is therefore
// ambiguous — it may be a stale read rather than a permission failure — so GET
// is retried briefly to give the write time to become visible. A 403 that
// outlives the budget is surfaced to the caller unchanged. Writes do not observe
// this, so only GET is retried.
// StaleReadRetry is the backoff policy for the stale-read retry described above.
//
// The zero value disables retrying, so a Client built without one behaves as
// single-shot. Individually: a zero InitialBackoff disables retrying, a zero
// MaxBackoff leaves the backoff uncapped, and a zero Jitter makes the schedule
// exact, which is useful in tests.
type StaleReadRetry struct {
	// InitialBackoff is the first sleep before a retry. Each subsequent attempt
	// doubles until MaxBackoff.
	InitialBackoff time.Duration
	// MaxBackoff caps any single sleep. Uncapped doubling would spend the last
	// 800ms of the budget on one sleep; capping fits another probe into the same
	// budget instead, which both reaches further and leaves a smaller worst-case
	// gap between probes.
	MaxBackoff time.Duration
	// MaxElapsed caps the time spent sleeping between retries. It governs the
	// nominal schedule: Jitter is applied to each sleep, so actual wall time
	// varies by up to that fraction either way.
	MaxElapsed time.Duration
	// Jitter is the fraction by which each sleep is randomly scaled up or down.
	// Terraform applies resources in parallel, so without jitter every worker
	// that hits a stale read retries on an identical schedule and the attempts
	// arrive in synchronised bursts.
	//
	// This is proportional rather than full jitter (a uniform value in [0, d))
	// deliberately: full jitter would halve the expected schedule and with it
	// the tail coverage the budget was sized for, whereas scaling around the
	// nominal value decorrelates workers while preserving it.
	Jitter float64
}

// defaultStaleReadRetry is the policy NewClient installs. It probes at 0, 25,
// 75, 175, 375, 775, 1275 and 1775ms — 8 attempts across 7 retries.
//
// The schedule is front-loaded because reads normally converge quickly, so
// nearly every stale read clears on the second or third attempt. The long tail
// exists only to absorb rare outliers rather than fail an apply, and costs
// nothing in the common case. The bounds were chosen from internal service
// latency measurements; consult those before narrowing them.
var defaultStaleReadRetry = StaleReadRetry{
	InitialBackoff: 25 * time.Millisecond,
	MaxBackoff:     500 * time.Millisecond,
	MaxElapsed:     2 * time.Second,
	Jitter:         0.25,
}

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

	// StaleRead tunes the 403 stale-read retry on GET requests.
	StaleRead StaleReadRetry
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

// IsAzureCredentialValidationError returns true if the error is an Azure credential validation failure
func (e *APIError) IsAzureCredentialValidationError() bool {
	return e.Code == "azure_integration_test_failed"
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

		StaleRead: defaultStaleReadRetry,
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

// BuildAuthPath constructs a path for the BeyondTrust auth service (workload identities).
// Format: /site/{site-id}/platform/auth/endpoint
// SiteID here is the operator's admin site — the site CRUD is performed against. The
// site a workload identity grants access to is carried separately in the request body.
func (c *Client) BuildAuthPath(endpoint string) string {
	return fmt.Sprintf("/site/%s/platform/auth%s", c.SiteID, endpoint)
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

// isStaleReadForbidden reports whether err is (or wraps) a 403 from the API,
// which on a GET may mean a just-written object is not yet visible to reads.
// This deliberately checks 403 alone rather than using IsPermissionError, which
// also matches 401 — retrying an invalid token only adds latency.
func isStaleReadForbidden(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden
}

// DoRequest performs a request and unmarshals the response.
//
// GET requests that fail with 403 are retried with exponential backoff to ride
// out eventually consistent reads (see StaleReadRetry). A 403 that
// outlives the backoff budget is returned unchanged, as are all other errors.
func (c *Client) DoRequest(ctx context.Context, method, path string, query url.Values, body interface{}, result interface{}) error {
	if method != http.MethodGet {
		return c.doRequestOnce(ctx, method, path, query, body, result)
	}

	var elapsed time.Duration
	retry := c.StaleRead
	backoff := retry.InitialBackoff

	for {
		err := c.doRequestOnce(ctx, method, path, query, body, result)
		if err == nil || !isStaleReadForbidden(err) {
			return err
		}

		// Stop once another sleep would exceed the budget, and surface the 403.
		// Accounting uses the nominal backoff rather than the jittered sleep, so
		// the attempt count stays fixed and only the timing varies.
		if backoff <= 0 || elapsed+backoff > retry.MaxElapsed {
			return err
		}

		timer := time.NewTimer(jitterBackoff(backoff, retry.Jitter))
		select {
		case <-ctx.Done():
			timer.Stop()
			// Return the API error rather than ctx.Err() so callers keep the
			// typed *APIError they use to classify failures.
			return err
		case <-timer.C:
		}

		// select chooses pseudo-randomly when both cases are ready, and the
		// context can also be cancelled in the gap between the sleep and the
		// next attempt. Re-check so cancellation deterministically returns the
		// typed *APIError above, rather than spending a doomed request that
		// fails with a wrapped context error instead.
		if ctx.Err() != nil {
			return err
		}

		elapsed += backoff

		backoff *= 2
		if retry.MaxBackoff > 0 && backoff > retry.MaxBackoff {
			backoff = retry.MaxBackoff
		}
	}
}

// jitterBackoff scales d by a random factor uniform in [1-jitter, 1+jitter), so
// that clients retrying concurrently do not stay in lockstep. A non-positive
// jitter returns d unchanged.
func jitterBackoff(d time.Duration, jitter float64) time.Duration {
	if d <= 0 || jitter <= 0 {
		return d
	}

	return time.Duration(float64(d) * (1 + jitter*(2*rand.Float64()-1)))
}

// doRequestOnce performs a single request attempt and unmarshals the response.
func (c *Client) doRequestOnce(ctx context.Context, method, path string, query url.Values, body interface{}, result interface{}) error {
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
