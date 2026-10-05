package ephemeral

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// ErrMalformedGenerateResponse reports a generate response that decoded without error but
// does not carry a credential. Treated as a failure rather than an empty success, because
// the silent form leaves a real credential alive with nothing tracking it.
var ErrMalformedGenerateResponse = errors.New("malformed generate response")

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
	if err := c.Post(ctx, c.BuildPath("/dynamic/"+name+"/generate"), query, nil, &envelope); err != nil {
		var zero T
		return zero, err
	}

	if leaseID := leaseIDOf(envelope.Secret); leaseID == "" {
		var zero T
		return zero, fmt.Errorf("generate returned no leaseId for %q: %w", name, ErrMalformedGenerateResponse)
	}

	return envelope.Secret, nil
}

// leaseIDOf reads the lease id a generated credential must carry.
//
// Every credential type returns one, so its absence means the response was not the object
// this code expects — an unwrapped body, a renamed field, or a 204, none of which produce a
// JSON error because unknown fields are discarded and an empty body is never parsed.
func leaseIDOf(secret any) string {
	type leaseCarrier interface{ leaseID() string }
	if c, ok := secret.(leaseCarrier); ok {
		return c.leaseID()
	}

	return ""
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

		if totalBackoff+backoff > revokeMaxTotalBackoff {
			return normalizeRevokeError(err)
		}

		select {
		case <-ctx.Done():
			return normalizeRevokeError(err)
		case <-time.After(jittered(backoff)):
		}

		// The nominal backoff is charged, not the jittered sleep, matching DoRequest: the
		// attempt count then stays fixed and only the timing varies.
		totalBackoff += backoff
		backoff = min(backoff*2, revokeMaxBackoff)
	}
}

// normalizeRevokeError folds the outcomes that are really successes into a nil.
//
// A 404 after the retry budget is one: the lease is gone, which is the state a revoke is
// trying to reach. Revocation is idempotent, so a lease already removed — by an earlier
// Close, by expiry, or by the backend — is not a failure to report.
//
// lease_not_revocable is deliberately NOT folded. Only the Azure resource implements Close,
// and for a revocable credential type that code means the password was not deleted and is
// still live, which the practitioner needs to hear. The AWS resource, whose leases really
// are non-revocable, never calls this.
func normalizeRevokeError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
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

// revokeWithTimeout runs a revoke on a context detached from the caller's.
//
// Close can be reached during a graceful shutdown with an already-cancelled context, and
// cleanup that gives up the moment the user presses Ctrl-C is cleanup that does not happen
// when it matters most. The timeout keeps that from becoming an unbounded wait.
func revokeWithTimeout(ctx context.Context, c *client.Client, leaseID string) error {
	revokeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()

	return revokeLease(revokeCtx, c, leaseID)
}

// generateFailureDetail builds the diagnostic for a failed generate.
//
// A 403 is genuinely ambiguous here, so the message names both causes rather than guessing:
// the API reports a dynamic secret the caller cannot see as forbidden rather than missing.
func generateFailureDetail(name string, err error) string {
	return fmt.Sprintf("Could not generate credentials from dynamic secret '%s': %s\n\n"+
		"A 403 here can mean either outcome: the API reports a dynamic secret you cannot see "+
		"as forbidden rather than missing. Check both.\n\n"+
		"  - The dynamic secret must already exist when this runs. It is opened during the "+
		"plan, so a configuration that creates it in the same apply fails here; apply the "+
		"dynamic secret first. depends_on does not help, because the open happens before it "+
		"takes effect.\n"+
		"  - The caller needs the GenerateDynamicCredential permission on it. Product admins "+
		"hold it already; anyone else needs a policy granting it.", name, err.Error())
}

// errUnconfiguredClient is the diagnostic for an Open reached without Configure having
// supplied a client. The framework always configures before opening, so this is a guard
// against a provider-wiring mistake surfacing as a nil dereference and a plugin crash.
func errUnconfiguredClient(resourceName string) (string, string) {
	return "Ephemeral Resource Not Configured",
		fmt.Sprintf("%s was opened without a configured API client. "+
			"Please report this issue to the provider developers.", resourceName)
}
