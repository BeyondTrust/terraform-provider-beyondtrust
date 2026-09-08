//go:build acceptance

package resources_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// This file is the single point of coupling to the Workload Credentials secrets API.
// Everything the binding tests know about that service's URL shapes, query parameters, request
// bodies and status codes lives here, so a change on their side breaks one file rather than
// several. Paths are relative to client.BuildPath, i.e. /site/{site-id}/secrets{endpoint}.

// wlcListItem is one row of a list response. Only the fields the assertions need are modeled.
type wlcListItem struct {
	Path     string `json:"path"`
	Type     string `json:"type"`
	Metadata struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	} `json:"metadata"`
}

// wlcListResponse is the envelope every WLC list endpoint returns.
type wlcListResponse struct {
	Data []wlcListItem `json:"data"`
}

// statusOf reports the HTTP status an API call produced: 200 for success, otherwise the status
// carried by the APIError. A transport-level failure returns 0 along with the error, so callers
// can tell "denied" apart from "never reached the service".
func statusOf(err error) (int, error) {
	if err == nil {
		return http.StatusOK, nil
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode, nil
	}
	return 0, err
}

// errorCodeOf returns the service's stable error code, or "" when the error is not an APIError.
// Assertions use this rather than the message, which WLC documents as unstable.
func errorCodeOf(err error) string {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}

// --- fixture seeding (runs as the product-site owner) ---

// createFolder creates a single folder segment. The API takes one segment as {name}, with the
// parent supplied as ?folder=, so a nested path needs one call per level.
func createFolder(ctx context.Context, c *client.Client, name, parent string) error {
	return c.Post(ctx, c.BuildPath("/folders/"+name), folderQuery(parent), nil, nil)
}

// createSecret creates a static secret with a single value key.
func createSecret(ctx context.Context, c *client.Client, name, folder, value string) error {
	body := map[string]any{"secret": map[string]string{"value": value}}
	return c.Post(ctx, c.BuildPath("/static/"+name), folderQuery(folder), body, nil)
}

// deleteFolderRecursive removes a folder and everything under it. Used for cleanup, so it is
// deliberately tolerant: WLC masks a missing resource as 403, and delete is idempotent anyway.
func deleteFolderRecursive(ctx context.Context, c *client.Client, name string) error {
	query := url.Values{}
	query.Set("permanent", "true")
	query.Set("recursive", "true")
	return c.Delete(ctx, c.BuildPath("/folders/"+name), query)
}

// --- permission probes (run as the low-privilege principal) ---

// listStaticSecrets calls the list endpoint scoped to a folder. Note the parameter is `path`,
// not `folder` — list endpoints and single-resource endpoints differ here.
//
// The endpoint applies two independent authorization layers: one permission gates the call, then
// every row is filtered by the caller's access to that individual secret. So a 200 with an empty
// list means "allowed to list, nothing visible", which is a distinct state from 403.
func listStaticSecrets(ctx context.Context, c *client.Client, folder string) (*wlcListResponse, int, error) {
	query := url.Values{}
	query.Set("path", folder)

	var out wlcListResponse
	err := c.Get(ctx, c.BuildPath("/static"), query, &out)
	status, transportErr := statusOf(err)
	if transportErr != nil {
		return nil, 0, transportErr
	}
	if status != http.StatusOK {
		return nil, status, err
	}
	return &out, status, nil
}

// readSecret fetches a single secret's value. Gated on can_read_secret.
func readSecret(ctx context.Context, c *client.Client, name, folder string) (int, error) {
	var out map[string]any
	err := c.Get(ctx, c.BuildPath("/static/"+name), folderQuery(folder), &out)
	status, transportErr := statusOf(err)
	if transportErr != nil {
		return 0, transportErr
	}
	return status, nil
}

// updateSecret merge-patches a secret. Gated on can_update_secret, which is owner-level — a
// standard user does not have it, which is what makes it a reliable denied-before signal.
// Success is 204 No Content.
func updateSecret(ctx context.Context, c *client.Client, name, folder, probe string) (int, error) {
	body := map[string]any{"secret": map[string]string{"tf-acc-probe": probe}}
	err := c.Patch(ctx, c.BuildPath("/static/"+name), folderQuery(folder), body)
	status, transportErr := statusOf(err)
	if transportErr != nil {
		return 0, transportErr
	}
	// The client short-circuits 204 to a nil error, which statusOf reports as 200.
	if status == http.StatusOK {
		return http.StatusNoContent, nil
	}
	return status, nil
}

func folderQuery(folder string) url.Values {
	query := url.Values{}
	if folder != "" {
		query.Set("folder", folder)
	}
	return query
}

// --- assertion helpers ---

// requireDenied fails unless the probe was refused with 403. WLC returns 403 for unknown
// resources as well as for denied ones, so this asserts denial-or-absence, never 404.
//
// A 401 is called out separately: it means the token expired, and polling an expired credential
// for the full timeout is the worst possible debugging experience.
func requireDenied(t *testing.T, status int, err error, what string) {
	t.Helper()
	switch status {
	case http.StatusForbidden:
		if code := errorCodeOf(err); code != "" && code != "forbidden" {
			t.Fatalf("%s: expected error code \"forbidden\", got %q", what, code)
		}
	case http.StatusUnauthorized:
		t.Fatalf("%s: token rejected (401) — the principal token is likely EXPIRED, mint a fresh one", what)
	case http.StatusOK, http.StatusNoContent:
		t.Fatalf(`%s: expected 403 but got %d.

The principal is over-privileged, so this test cannot prove anything: it would report success
whether or not the policy bound. Either

  (a) the identity behind %s is not a low-privilege user on the product site, or
  (b) it is a REUSED token that still carries access from when it was minted — a token's
      authorization can outlive the resources this test cleans up.

Mint a FRESH principal token and re-run.`, what, status, "BEYONDTRUST_TEST_POLICY_PRINCIPAL_TOKEN")
	default:
		t.Fatalf("%s: expected 403, got %d (err: %v)", what, status, err)
	}
}

// secretBaseNames reduces list rows to their trailing path segment, which is what the test
// names its fixtures by.
func secretBaseNames(items []wlcListItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, path.Base(strings.TrimSuffix(item.Path, "/")))
	}
	return names
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// describeItems renders a list response for failure messages.
func describeItems(items []wlcListItem) string {
	if len(items) == 0 {
		return "(empty)"
	}
	return fmt.Sprintf("%v", secretBaseNames(items))
}
