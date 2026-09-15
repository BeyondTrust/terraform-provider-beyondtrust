//go:build !acceptance
// +build !acceptance

package resources

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

const validCedar = `@siteId("11111111-2222-3333-4444-555555555555")
permit(
  principal == Pathfinder::User::Email::"devops@example.com",
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Folder::"/production"
);`

func TestValidateCedar(t *testing.T) {
	tests := []struct {
		name        string
		cedar       string
		wantSummary string
		description string
	}{
		{
			name:        "valid single permit",
			cedar:       validCedar,
			description: "the canonical policy shape the service accepts",
		},
		{
			name: "valid product grant using is",
			cedar: `@siteId("11111111-2222-3333-4444-555555555555")
permit(
  principal == Pathfinder::Group::"platform-admins",
  action == WorkloadCredentials::Action::"Lister",
  resource is WorkloadCredentials::Product
);`,
			description: "product grants use `resource is`, which carries no entity id",
		},
		{
			name: "valid multi action",
			cedar: `@siteId("11111111-2222-3333-4444-555555555555")
permit(
  principal == Pathfinder::User::Email::"dev@example.com",
  action in [WorkloadCredentials::Action::"ReadFolderMetadata", WorkloadCredentials::Action::"ListFolderContents"],
  resource == WorkloadCredentials::Folder::"/production"
);`,
			description: "action sets are supported by the service",
		},
		{
			name:        "unparseable",
			cedar:       "this is not cedar",
			wantSummary: "Invalid Cedar Syntax",
			description: "a parse failure should be reported at plan time",
		},
		{
			name: "two statements",
			cedar: validCedar + "\n" + `@siteId("11111111-2222-3333-4444-555555555555")
permit(
  principal == Pathfinder::User::Email::"other@example.com",
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Folder::"/staging"
);`,
			wantSummary: "Expected Exactly One Cedar Statement",
			description: "the service stores exactly one statement per policy name",
		},
		{
			name: "missing siteId annotation",
			cedar: `permit(
  principal == Pathfinder::User::Email::"devops@example.com",
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Folder::"/production"
);`,
			wantSummary: "Missing @siteId Annotation",
			description: "the site is taken only from the annotation, never inferred",
		},
		{
			name: "siteId is not a uuid",
			cedar: `@siteId("production")
permit(
  principal == Pathfinder::User::Email::"devops@example.com",
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Folder::"/production"
);`,
			wantSummary: "Invalid @siteId Annotation",
			description: "the annotation value must parse as a UUID",
		},
		{
			name: "forbid effect",
			cedar: `@siteId("11111111-2222-3333-4444-555555555555")
forbid(
  principal == Pathfinder::User::Email::"devops@example.com",
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Folder::"/production"
);`,
			wantSummary: "Unsupported Cedar Effect",
			description: "phase 1 accepts permit only",
		},
		{
			name: "when condition",
			cedar: `@siteId("11111111-2222-3333-4444-555555555555")
permit(
  principal == Pathfinder::User::Email::"devops@example.com",
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Folder::"/production"
) when { context.source_ip == "10.0.0.1" };`,
			wantSummary: "Unsupported Cedar Condition",
			description: "phase 1 rejects when/unless clauses",
		},
		{
			name: "unqualified principal type",
			cedar: `@siteId("11111111-2222-3333-4444-555555555555")
permit(
  principal == User::"devops@example.com",
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Folder::"/production"
);`,
			wantSummary: "Unqualified Cedar Entity Type",
			description: "the service's Cedar schema namespaces every type",
		},
		{
			name: "unqualified resource type",
			cedar: `@siteId("11111111-2222-3333-4444-555555555555")
permit(
  principal == Pathfinder::User::Email::"devops@example.com",
  action == WorkloadCredentials::Action::"Owner",
  resource == Folder::"/production"
);`,
			wantSummary: "Unqualified Cedar Entity Type",
			description: "resource types must carry their product namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCedar(tt.cedar)
			if tt.wantSummary == "" {
				assert.Nil(t, err, tt.description)
				return
			}
			require.NotNil(t, err, tt.description)
			assert.Equal(t, tt.wantSummary, err.Summary, tt.description)
			assert.NotEmpty(t, err.Detail, "every validation error should explain itself")
		})
	}
}

// newPolicyResource wires a PolicyResource to a test server.
func newPolicyResource(t *testing.T, handler http.HandlerFunc) (*PolicyResource, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	c, err := client.NewClient(&client.Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
		SiteID:      "admin-site",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	return &PolicyResource{client: c}, server
}

func writePolicyJSON(t *testing.T, w http.ResponseWriter, status string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(policy{
		Name:      "example",
		Cedar:     validCedar,
		Product:   "wlc",
		Site:      "11111111-2222-3333-4444-555555555555",
		Status:    status,
		CreatedAt: "2026-08-14T10:00:00Z",
		UpdatedAt: "2026-08-14T10:00:00Z",
	})
}

func TestWaitForPolicy(t *testing.T) {
	t.Run("settles on ACTIVE after transient states", func(t *testing.T) {
		var calls int32
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			n := atomic.AddInt32(&calls, 1)
			status := statusResourceLookup
			switch {
			case n == 2:
				status = statusPendingBind
			case n >= 3:
				status = statusActive
			}
			writePolicyJSON(t, w, status)
		})

		pol, err := r.waitForPolicy(context.Background(), "example", 30*time.Second)
		require.NoError(t, err)
		assert.Equal(t, statusActive, pol.Status)
		assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3))
	})

	t.Run("returns terminal error on RESOURCE_LOOKUP_ERROR", func(t *testing.T) {
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			writePolicyJSON(t, w, statusResourceLookupError)
		})

		pol, err := r.waitForPolicy(context.Background(), "example", 30*time.Second)
		require.Error(t, err)

		var terminal *terminalStatusError
		require.True(t, errors.As(err, &terminal), "should be a terminalStatusError")
		assert.Equal(t, statusResourceLookupError, terminal.status)
		// The policy is still returned so callers can persist the failing status.
		require.NotNil(t, pol)
		assert.Equal(t, statusResourceLookupError, pol.Status)
	})

	t.Run("returns terminal error on NO_PRODUCT_ACCESS", func(t *testing.T) {
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			writePolicyJSON(t, w, statusNoProductAccess)
		})

		_, err := r.waitForPolicy(context.Background(), "example", 30*time.Second)
		var terminal *terminalStatusError
		require.True(t, errors.As(err, &terminal))
		assert.Contains(t, terminal.Error(), "product access")
	})

	t.Run("WAITING_FOR_RESOURCE returns immediately rather than blocking", func(t *testing.T) {
		var calls int32
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			writePolicyJSON(t, w, statusWaitingForResource)
		})

		// Deliberately generous: granting ahead of a resource is a documented pattern, and the
		// grant sits in this state until that resource appears. Treating it as transient would
		// burn the whole timeout and then report that nothing was wrong.
		start := time.Now()
		pol, err := r.waitForPolicy(context.Background(), "example", 30*time.Second)
		elapsed := time.Since(start)

		require.Error(t, err)
		var pending *pendingStateError
		require.True(t, errors.As(err, &pending), "should be a pendingStateError so callers warn instead of failing")

		assert.Less(t, elapsed, 5*time.Second, "must not wait out the timeout on a steady state")
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "nothing to poll for; one read is enough")

		require.NotNil(t, pol)
		assert.Equal(t, statusWaitingForResource, pol.Status)
	})

	t.Run("timing out while still settling is a failure, not a warning", func(t *testing.T) {
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			writePolicyJSON(t, w, statusResourceLookup)
		})

		pol, err := r.waitForPolicy(context.Background(), "example", 100*time.Millisecond)
		require.Error(t, err)

		// The distinction is load-bearing: callers warn on pendingStateError and fail on anything
		// else, so if a timeout were pending the timeouts block could never fail an apply.
		var pending *pendingStateError
		assert.False(t, errors.As(err, &pending), "a timeout must not be reported as a pending state")

		var timedOut *waitTimeoutError
		require.True(t, errors.As(err, &timedOut))
		assert.Equal(t, statusResourceLookup, timedOut.status)
		assert.Contains(t, timedOut.Error(), "timed out")
		require.NotNil(t, pol)
	})

	t.Run("timing out with no successful read explains why", func(t *testing.T) {
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"internal_error","message":"boom"}`))
		})

		_, err := r.waitForPolicy(context.Background(), "example", 100*time.Millisecond)
		require.Error(t, err)

		var timedOut *waitTimeoutError
		require.True(t, errors.As(err, &timedOut))
		assert.Empty(t, timedOut.status, "no status was ever read")
		assert.Contains(t, timedOut.Error(), "boom", "the underlying failure must not be swallowed")
	})

	t.Run("retries connection failures", func(t *testing.T) {
		var calls int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&calls, 1) == 1 {
				// Hijack and drop the connection to produce a transport-level error.
				hj, ok := w.(http.Hijacker)
				if ok {
					conn, _, err := hj.Hijack()
					if err == nil {
						_ = conn.Close()
						return
					}
				}
			}
			writePolicyJSON(t, w, statusActive)
		}))
		t.Cleanup(server.Close)

		c, err := client.NewClient(&client.Config{
			BaseURL: server.URL, AccessToken: "t", SiteID: "s", Timeout: "5s",
		})
		require.NoError(t, err)
		r := &PolicyResource{client: c}

		pol, err := r.waitForPolicy(context.Background(), "example", 30*time.Second)
		require.NoError(t, err, "a dropped connection mid-poll must not fail the apply")
		assert.Equal(t, statusActive, pol.Status)
	})

	t.Run("retries transient server errors", func(t *testing.T) {
		var calls int32
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&calls, 1) == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"code": "principal_lookup_failed", "message": "upstream unavailable",
				})
				return
			}
			writePolicyJSON(t, w, statusActive)
		})

		pol, err := r.waitForPolicy(context.Background(), "example", 30*time.Second)
		require.NoError(t, err, "a 502 principal_lookup_failed should be retried, not fatal")
		assert.Equal(t, statusActive, pol.Status)
	})

	t.Run("gives up on non-retryable errors", func(t *testing.T) {
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "forbidden", "message": "denied"})
		})

		_, err := r.waitForPolicy(context.Background(), "example", 30*time.Second)
		require.Error(t, err)

		var apiErr *client.APIError
		require.True(t, errors.As(err, &apiErr), "a 403 should surface as-is rather than being polled through")
		assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	})
}

func TestPutPolicy(t *testing.T) {
	t.Run("sends raw cedar as text/plain and accepts 202", func(t *testing.T) {
		var gotPath, gotMethod, gotContentType, gotBody string
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, req *http.Request) {
			gotPath, gotMethod = req.URL.Path, req.Method
			gotContentType = req.Header.Get("Content-Type")
			buf, _ := io.ReadAll(req.Body)
			gotBody = string(buf)

			w.WriteHeader(http.StatusAccepted)
			writePolicyJSON(t, w, statusResourceLookup)
		})

		pol, err := r.putPolicy(context.Background(), "example", validCedar)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPut, gotMethod)
		assert.Equal(t, "/site/admin-site/platform/iam/policies/example", gotPath)
		assert.Equal(t, "text/plain", gotContentType)
		assert.Equal(t, validCedar, gotBody, "cedar must be sent verbatim")
		assert.Equal(t, statusResourceLookup, pol.Status)
	})

	t.Run("retries transient server errors", func(t *testing.T) {
		var calls int32
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&calls, 1) < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":"internal_error","message":"boom"}`))
				return
			}
			w.WriteHeader(http.StatusAccepted)
			writePolicyJSON(t, w, statusResourceLookup)
		})

		_, err := r.putPolicy(context.Background(), "example", validCedar)
		require.NoError(t, err)
		assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
	})

	t.Run("does not retry client errors", func(t *testing.T) {
		var calls int32
		r, _ := newPolicyResource(t, func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":"invalid_cedar_syntax","message":"bad"}`))
		})

		_, err := r.putPolicy(context.Background(), "example", validCedar)
		require.Error(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a 400 is not retryable")
	})
}

func TestPolicyErrorDetail(t *testing.T) {
	t.Run("includes code and trace id", func(t *testing.T) {
		err := &client.APIError{
			Message:    "cedar policy must include an @siteId annotation",
			Code:       "invalid_site",
			StatusCode: http.StatusBadRequest,
			TraceID:    "trace-abc-123",
		}
		detail := policyErrorDetail("create", "example", err)

		assert.Contains(t, detail, `Could not create IAM policy "example"`)
		assert.Contains(t, detail, "invalid_site")
		assert.Contains(t, detail, "trace-abc-123")
	})

	t.Run("handles plain errors", func(t *testing.T) {
		detail := policyErrorDetail("read", "example", errors.New("connection refused"))
		assert.Contains(t, detail, "connection refused")
	})
}

func TestIsNotFoundAndIsRetryable(t *testing.T) {
	notFound := &client.APIError{StatusCode: http.StatusNotFound, Code: "policy_not_found"}
	serverErr := &client.APIError{StatusCode: http.StatusBadGateway, Code: "principal_lookup_failed"}
	badRequest := &client.APIError{StatusCode: http.StatusBadRequest, Code: "invalid_cedar_syntax"}

	assert.True(t, isNotFound(notFound))
	assert.False(t, isNotFound(serverErr))
	assert.False(t, isNotFound(errors.New("plain")))

	assert.True(t, isRetryable(serverErr))
	assert.False(t, isRetryable(badRequest))
	assert.False(t, isRetryable(errors.New("plain")))
}
