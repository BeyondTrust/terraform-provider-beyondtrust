package ephemeral

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// privateStateLeaseKey is the private state key under which an ephemeral resource
// records the lease it minted, so Close can revoke it. Close receives only private
// state — not config — so anything Close needs has to be stashed here during Open.
const privateStateLeaseKey = "lease"

// Revoke retry budget. These mirror client.defaultStaleReadRetry() rather than
// inventing a second set of numbers: the window being defended against is the same
// eventual-consistency window, just on a DELETE instead of a GET. The client's own
// retry is GET-only, so a lease revoke has to implement its own.
const (
	revokeInitialBackoff  = 25 * time.Millisecond
	revokeMaxBackoff      = 500 * time.Millisecond
	revokeMaxTotalBackoff = 2 * time.Second
	revokeJitter          = 0.25

	// closeTimeout bounds the whole of Close. The backoff budget above bounds sleep,
	// not wall clock — against a hung backend the default 30s HTTP timeout would let
	// a handful of attempts run for minutes. This is the actual hang guard.
	closeTimeout = 10 * time.Second
)

// leasePrivateState is what Open hands to Close. Kept deliberately small: private
// state has to round-trip as JSON through Terraform core between the two calls.
type leasePrivateState struct {
	LeaseID       string `json:"leaseId"`
	RevokeOnClose bool   `json:"revokeOnClose"`
}

// generateSecretEnvelope is the {"secret": {...}} wrapper every generate response
// arrives in. Generic so each credential type decodes into its own struct: decoding
// into map[string]any would round-trip every value through interface{} and turn any
// future numeric field into a float64.
type generateSecretEnvelope[T any] struct {
	Secret T `json:"secret"`
}

// generateCredential mints a credential from the dynamic secret at name/folder.
//
// The credential is returned exactly once and is never stored server-side; the only
// thing that persists is the lease, identified by the leaseId in the response.
//
// name is passed unescaped on purpose. newRequest builds the URL with u.Path += path
// and lets url.URL.String() do the encoding, so a pre-escaped segment would be
// double-encoded ("^" -> "%255E"). The resource name validator already restricts the
// charset to one that survives this intact.
func generateCredential[T any](ctx context.Context, c *client.Client, name, folder string) (T, error) {
	query := url.Values{}
	if folder != "" {
		query.Set("folder", folder)
	}

	var envelope generateSecretEnvelope[T]
	err := c.Post(ctx, c.BuildPath("/dynamic/"+name+"/generate"), query, nil, &envelope)

	return envelope.Secret, err
}

// revokeLease destroys a lease's external credential ahead of its expiration.
//
// Only revocable credential types reach the provider destroyer; AWS assumed-role
// leases answer 400 lease_not_revocable because the STS credential expires on its
// own. That is a successful outcome here, not a failure.
//
// A 404 immediately after generate is the lease-durability window: the API returns
// the leaseId from an enqueued workflow before the row is committed, so a revoke can
// briefly outrun its own lease. That case retries. Nothing else does — see
// classifyRevokeError.
func revokeLease(ctx context.Context, c *client.Client, leaseID string) error {
	path := c.BuildPath("/leases/id/" + leaseID)

	var totalBackoff time.Duration
	backoff := revokeInitialBackoff

	for {
		err := c.Delete(ctx, path, nil)
		if err == nil || !isLeaseNotYetDurable(err) {
			return normalizeRevokeError(err)
		}

		sleep := jittered(backoff)
		if totalBackoff+sleep > revokeMaxTotalBackoff {
			return normalizeRevokeError(err)
		}

		select {
		case <-ctx.Done():
			return normalizeRevokeError(err)
		case <-time.After(sleep):
		}

		totalBackoff += sleep
		backoff = min(backoff*2, revokeMaxBackoff)
	}
}

// normalizeRevokeError folds the one error that is really a success into a nil.
//
// lease_not_revocable means the credential type has no active revocation path, which
// is the documented behaviour for AWS assumed-role leases rather than a fault.
func normalizeRevokeError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.Code == "lease_not_revocable" {
		return nil
	}

	return err
}

// isLeaseNotYetDurable reports whether err is the transient 404 that a revoke issued
// moments after generate can hit, before the lease row is committed.
//
// Deliberately narrow. A 403 is not included: the API masks "not found" as "forbidden"
// for resources the caller cannot see, so a 403 is ambiguous between this same window
// and a principal that simply lacks can_revoke_lease. The second case is permanent, and
// retrying it would make every under-privileged caller pay the full backoff budget on
// every single apply for a failure that was never going to clear.
func isLeaseNotYetDurable(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// jittered spreads retries so concurrent ephemeral opens do not resynchronise onto the
// same backend at the same instant.
func jittered(d time.Duration) time.Duration {
	delta := float64(d) * revokeJitter
	return time.Duration(float64(d) - delta + rand.Float64()*2*delta) //nolint:gosec // jitter, not a security decision
}

// marshalLeasePrivateState encodes what Close needs. Private state values must be
// valid JSON, so this is always built by the marshaler rather than by hand.
func marshalLeasePrivateState(leaseID string, revokeOnClose bool) ([]byte, error) {
	return json.Marshal(leasePrivateState{LeaseID: leaseID, RevokeOnClose: revokeOnClose})
}
