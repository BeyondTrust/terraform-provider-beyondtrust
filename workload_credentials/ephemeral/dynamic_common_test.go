//go:build !acceptance

package ephemeral

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// newTestClient builds a real client pointed at a test server. The client is not
// mocked anywhere in this repo; exercising the genuine request path is the point.
func newTestClient(t *testing.T, serverURL string) *client.Client {
	t.Helper()

	c, err := client.NewClient(&client.Config{
		BaseURL:     serverURL,
		AccessToken: "test-token",
		SiteID:      "test-site",
		APIVersion:  "2026-04-28",
		Timeout:     "30s",
	})
	require.NoError(t, err)

	return c
}

// writeAPIError writes the {code, message} envelope every error response uses.
func writeAPIError(t *testing.T, w http.ResponseWriter, status int, code string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write([]byte(`{"code":"` + code + `","message":"test error"}`))
	require.NoError(t, err)
}

func TestGenerateCredential_AWS(t *testing.T) {
	t.Parallel()

	var gotPath, gotQuery, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotMethod = r.URL.Path, r.URL.RawQuery, r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"secret":{
			"leaseId":"lease-abc","type":"aws","credentialType":"assumed_role",
			"accessKeyId":"ASIAEXAMPLE","secretAccessKey":"wJalrEXAMPLE",
			"sessionToken":"FwoGZXIvYXdz","expiration":"2026-06-17T15:00:00-06:00"}}`))
	}))
	defer srv.Close()

	secret, err := generateCredential[awsGeneratedSecret](context.Background(), newTestClient(t, srv.URL), "my-secret", "production/aws")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/site/test-site/wlc/dynamic/my-secret/generate", gotPath)
	assert.Equal(t, "folder=production%2Faws", gotQuery)

	assert.Equal(t, "lease-abc", secret.LeaseID)
	assert.Equal(t, "assumed_role", secret.CredentialType)
	assert.Equal(t, "ASIAEXAMPLE", secret.AccessKeyID)
	assert.Equal(t, "wJalrEXAMPLE", secret.SecretAccessKey)
	assert.Equal(t, "FwoGZXIvYXdz", secret.SessionToken)
	assert.Equal(t, "2026-06-17T15:00:00-06:00", secret.Expiration)
}

func TestGenerateCredential_Azure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"secret":{
			"leaseId":"lease-xyz","type":"azure","credentialType":"service_principal_password",
			"clientId":"33333333-3333-3333-3333-333333333333","clientSecret":"generated-password",
			"tenantId":"00000000-0000-0000-0000-000000000000","keyId":"44444444-4444-4444-4444-444444444444"}}`))
	}))
	defer srv.Close()

	secret, err := generateCredential[azureGeneratedSecret](context.Background(), newTestClient(t, srv.URL), "my-secret", "")
	require.NoError(t, err)

	assert.Equal(t, "lease-xyz", secret.LeaseID)
	assert.Equal(t, "generated-password", secret.ClientSecret)
	assert.Equal(t, "33333333-3333-3333-3333-333333333333", secret.ClientID)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", secret.TenantID)
	assert.Equal(t, "44444444-4444-4444-4444-444444444444", secret.KeyID)
}

// An empty folder must not send the query parameter at all. Sending folder="" would
// scope the lookup to the root and silently miss a foldered dynamic secret.
func TestGenerateCredential_OmitsEmptyFolder(t *testing.T) {
	t.Parallel()

	var gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"secret":{"leaseId":"l"}}`))
	}))
	defer srv.Close()

	_, err := generateCredential[awsGeneratedSecret](context.Background(), newTestClient(t, srv.URL), "my-secret", "")
	require.NoError(t, err)
	assert.Empty(t, gotRawQuery)
}

func TestGenerateCredential_PropagatesError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(t, w, http.StatusNotFound, "dynamic_secret_not_found")
	}))
	defer srv.Close()

	_, err := generateCredential[awsGeneratedSecret](context.Background(), newTestClient(t, srv.URL), "missing", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dynamic_secret_not_found")
}

func TestRevokeLease_Success(t *testing.T) {
	t.Parallel()

	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	require.NoError(t, revokeLease(context.Background(), newTestClient(t, srv.URL), "lease-abc"))
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/site/test-site/wlc/leases/id/lease-abc", gotPath)
}

// Only the Azure resource implements Close, so this path is reached for a revocable
// credential type. lease_not_revocable there means the password was not deleted and is
// still live, which must surface rather than be swallowed as success.
func TestRevokeLease_NotRevocableIsReported(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeAPIError(t, w, http.StatusBadRequest, "lease_not_revocable")
	}))
	defer srv.Close()

	err := revokeLease(context.Background(), newTestClient(t, srv.URL), "lease-abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lease_not_revocable")
	assert.Equal(t, int32(1), calls.Load(), "a non-revocable lease must not be retried")
}

// The API returns a leaseId from an enqueued workflow before the row is committed, so
// a revoke can briefly outrun its own lease.
func TestRevokeLease_RetriesTransientNotFound(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			writeAPIError(t, w, http.StatusNotFound, "lease_not_found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	require.NoError(t, revokeLease(context.Background(), newTestClient(t, srv.URL), "lease-abc"))
	assert.Equal(t, int32(3), calls.Load())
}

// A lease that is still absent after the retry budget is gone, which is the state a revoke
// is trying to reach. Revocation is idempotent, so this is a success rather than a warning
// about a credential that no longer exists.
func TestRevokeLease_PersistentNotFoundIsSuccess(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeAPIError(t, w, http.StatusNotFound, "lease_not_found")
	}))
	defer srv.Close()

	require.NoError(t, revokeLease(context.Background(), newTestClient(t, srv.URL), "lease-abc"))
	assert.Greater(t, calls.Load(), int32(1), "a transient status should have been retried first")
}

// A 403 is ambiguous between the durability window and a principal that simply lacks
// can_revoke_lease. The second case never clears, so retrying it would make every
// under-privileged caller pay the full backoff budget on every apply.
func TestRevokeLease_DoesNotRetryForbidden(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeAPIError(t, w, http.StatusForbidden, "forbidden")
	}))
	defer srv.Close()

	require.Error(t, revokeLease(context.Background(), newTestClient(t, srv.URL), "lease-abc"))
	assert.Equal(t, int32(1), calls.Load(), "403 must not be retried")
}

// A failed or timed-out revoke may have partially succeeded upstream, so it is not
// safe to replay blindly.
func TestRevokeLease_DoesNotRetryUpstreamFailures(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		status int
		code   string
	}{
		"revoke failed":  {http.StatusBadGateway, "lease_revoke_failed"},
		"revoke timeout": {http.StatusGatewayTimeout, "lease_revoke_timeout"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				writeAPIError(t, w, tc.status, tc.code)
			}))
			defer srv.Close()

			require.Error(t, revokeLease(context.Background(), newTestClient(t, srv.URL), "lease-abc"))
			assert.Equal(t, int32(1), calls.Load())
		})
	}
}

func TestMarshalLeasePrivateState_RoundTrips(t *testing.T) {
	t.Parallel()

	encoded, err := marshalLeasePrivateState("lease-abc", false)
	require.NoError(t, err)

	var state leasePrivateState
	require.NoError(t, json.Unmarshal(encoded, &state))
	assert.Equal(t, "lease-abc", state.LeaseID)
	assert.False(t, state.RevokeOnClose)
}

// Close can be reached during a graceful shutdown, when the context Terraform hands the
// provider is already cancelled. Cleanup that gives up at that point is cleanup that does
// not happen when it matters most, so the revoke has to run on a detached context.
func TestRevokeWithTimeout_RunsOnCancelledContext(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, revokeWithTimeout(ctx, c, "lease-abc"))
	assert.Equal(t, int32(1), calls.Load(), "a cancelled caller context must not skip the revoke")
}

// A response that decodes cleanly but carries no credential must fail rather than succeed
// with empty values. Unknown fields are discarded and a 204 is never parsed, so neither
// produces a JSON error — and for Azure an empty lease id would make Close silently skip a
// password that is still live on the app registration.
func TestGenerateCredential_RejectsResponseWithoutCredential(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"unwrapped secret":  `{"leaseId":"l","accessKeyId":"A"}`,
		"empty envelope":    `{"secret":{}}`,
		"renamed lease key": `{"secret":{"lease_id":"l","accessKeyId":"A"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			_, err := generateCredential[awsGeneratedSecret](context.Background(), newTestClient(t, srv.URL), "s", "")
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrMalformedGenerateResponse)
		})
	}
}

// A 204 decodes to nothing at all, which must not read as a successful generation.
func TestGenerateCredential_RejectsNoContent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	_, err := generateCredential[azureGeneratedSecret](context.Background(), newTestClient(t, srv.URL), "s", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedGenerateResponse)
}

// The two causes a refusal might have are only indistinguishable for a 403 or 404. Printing
// that advice under a 500 sends the reader hunting a permission problem that is not there,
// which is what an earlier version of this message did.
func TestGenerateFailureDetail_AdviceMatchesStatus(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		err          error
		wantSubstr   []string
		notWantSubst []string
	}{
		"forbidden": {
			err:          &client.APIError{StatusCode: http.StatusForbidden, Code: "forbidden", Message: "nope", TraceID: "tr-1"},
			wantSubstr:   []string{"GenerateDynamicCredential", "depends_on does not help", "API error code: forbidden", "Trace ID: tr-1"},
			notWantSubst: []string{"server-side failure"},
		},
		"not found": {
			err:        &client.APIError{StatusCode: http.StatusNotFound, Code: "dynamic_secret_not_found", Message: "gone"},
			wantSubstr: []string{"GenerateDynamicCredential"},
		},
		"internal error": {
			err:          &client.APIError{StatusCode: http.StatusInternalServerError, Code: "internal_error", Message: "boom", TraceID: "tr-2"},
			wantSubstr:   []string{"server-side failure", "Trace ID: tr-2"},
			notWantSubst: []string{"GenerateDynamicCredential", "depends_on"},
		},
		"non-api error": {
			err:          errors.New("dial tcp: refused"),
			wantSubstr:   []string{"dial tcp: refused"},
			notWantSubst: []string{"GenerateDynamicCredential", "server-side failure"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := generateFailureDetail("my-secret", tc.err)
			assert.Contains(t, got, "my-secret")
			for _, want := range tc.wantSubstr {
				assert.Contains(t, got, want)
			}
			for _, unwanted := range tc.notWantSubst {
				assert.NotContains(t, got, unwanted)
			}
		})
	}
}
