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

// Revoke retry budget, matching the client's own read retry. That retry only covers GETs,
// so revoke has its own loop.
const (
	revokeInitialBackoff  = 25 * time.Millisecond
	revokeMaxBackoff      = 500 * time.Millisecond
	revokeMaxTotalBackoff = 2 * time.Second
	revokeJitter          = 0.25

	// closeTimeout bounds the whole of Close. The backoff budget only bounds sleep, so
	// without this a slow API could hold Close for minutes.
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
// A 404 shortly after generate can be transient, because a new lease may not be visible
// yet. That case retries; nothing else does.
func revokeLease(ctx context.Context, c *client.Client, leaseID string) error {
	path := c.BuildPath("/leases/id/" + leaseID)

	var totalBackoff time.Duration
	backoff := revokeInitialBackoff

	for {
		err := c.Delete(ctx, path, nil)
		if err == nil || !isLeaseNotYetVisible(err) {
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

// isLeaseNotYetVisible reports whether err is the transient 404 a revoke can hit moments
// after generate.
//
// A 403 is not retried. The API reports resources the caller cannot see as forbidden, so a
// 403 may just as well mean the caller lacks RevokeLease, which retrying never fixes.
func isLeaseNotYetVisible(err error) bool {
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
// The advice depends on the status: permission guidance under a 500 would send the reader
// looking for a problem that is not there.
func generateFailureDetail(name string, err error) string {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Sprintf("Could not generate credentials from dynamic secret '%s': %s", name, err)
	}

	detail := fmt.Sprintf("Could not generate credentials from dynamic secret '%s': %s", name, apiErr.Message)

	switch {
	case apiErr.StatusCode == http.StatusForbidden || apiErr.IsNotFound():
		// Only here are the two causes genuinely indistinguishable: the API reports a
		// dynamic secret the caller cannot see as forbidden rather than missing.
		detail += "\n\nThis can mean either of two things, which the status does not separate:\n\n" +
			"  - The dynamic secret must already exist when this runs. It is opened while the " +
			"plan is built, so a configuration that creates it in the same apply fails here; " +
			"apply the dynamic secret first. depends_on does not help, because the open " +
			"happens before it takes effect.\n" +
			"  - The caller needs the GenerateDynamicCredential permission on it. Product " +
			"admins hold it already; anyone else needs a policy granting it."
	case apiErr.IsServerError():
		detail += "\n\nThis is a server-side failure rather than anything wrong with the " +
			"configuration. Quote the trace id below when reporting it."
	}

	if apiErr.Code != "" {
		detail += "\n\nAPI error code: " + apiErr.Code
	}
	if apiErr.TraceID != "" {
		detail += "\nTrace ID: " + apiErr.TraceID
	}

	return detail
}

// errUnconfiguredClient is the diagnostic for an Open reached without a configured client,
// so a wiring mistake surfaces as an error rather than a crash.
func errUnconfiguredClient(resourceName string) (string, string) {
	return "Ephemeral Resource Not Configured",
		fmt.Sprintf("%s was opened without a configured API client. "+
			"Please report this issue to the provider developers.", resourceName)
}
