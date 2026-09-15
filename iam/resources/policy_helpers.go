package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cedar-policy/cedar-go"
	expast "github.com/cedar-policy/cedar-go/x/exp/ast"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/validators"
)

// Policy lifecycle states reported by the IAM policy service. Writes are asynchronous: the API
// returns 202 with a non-terminal status, and the service advances the policy to ACTIVE once
// the grant it compiles to has actually been applied.
const (
	statusResourceLookup      = "RESOURCE_LOOKUP"
	statusResourceLookupError = "RESOURCE_LOOKUP_ERROR"
	statusPendingBind         = "PENDING_BIND"
	statusPendingUnbind       = "PENDING_UNBIND"
	statusWaitingForResource  = "WAITING_FOR_RESOURCE"
	statusNoProductAccess     = "NO_PRODUCT_ACCESS"
	statusActive              = "ACTIVE"
	statusDeleted             = "DELETED"
)

// Polling cadence used while waiting for a policy to reach a terminal state.
const (
	initialPollInterval = 2 * time.Second
	maxPollInterval     = 15 * time.Second
	pollBackoffFactor   = 1.5
)

// siteIDAnnotation is the Cedar annotation carrying the site a policy applies to. The service
// treats it as authoritative — the site is deliberately not derived from the token or the
// request path, so that tenant scoping cannot be made conditional.
const siteIDAnnotation = "siteId"

// policy mirrors the IAM policy service response payload. These seven fields are the entire
// public model: there is no id, description, or version.
type policy struct {
	Name      string `json:"name"`
	Cedar     string `json:"cedar"`
	Product   string `json:"product"`
	Site      string `json:"site"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// isNotFound reports whether err is a 404 from the API.
func isNotFound(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsNotFound()
	}
	return false
}

// isRetryable reports whether err is a transient server-side failure worth retrying. The policy
// service returns 502 principal_lookup_failed and 500 internal_error for conditions that clear
// on their own.
func isRetryable(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsServerError()
	}
	return false
}

// policyErrorDetail renders an API error into a diagnostic detail, surfacing the service's
// stable error code and the request trace id when present so failures can be handed to support.
func policyErrorDetail(verb, name string, err error) string {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Sprintf("Could not %s IAM policy %q: %s", verb, name, err)
	}

	detail := fmt.Sprintf("Could not %s IAM policy %q: %s", verb, name, apiErr.Message)
	if apiErr.Code != "" {
		detail += "\n\nAPI error code: " + apiErr.Code
	}
	if apiErr.TraceID != "" {
		detail += "\nTrace ID: " + apiErr.TraceID
	}
	return detail
}

// terminalStatusError reports that a policy settled into a state other than ACTIVE.
type terminalStatusError struct {
	status string
}

func (e *terminalStatusError) Error() string {
	switch e.status {
	case statusResourceLookupError:
		return "the policy's principal or resource could not be resolved (status RESOURCE_LOOKUP_ERROR). " +
			"Check that the principal exists and that the resource path in the Cedar text is correct."
	case statusNoProductAccess:
		return "the policy's principal does not have access to the product the resource belongs to " +
			"(status NO_PRODUCT_ACCESS). Grant the principal product access, then re-apply."
	case statusDeleted:
		return "the policy was deleted while waiting for it to become active (status DELETED)."
	default:
		return fmt.Sprintf("the policy settled in unexpected state %s.", e.status)
	}
}

// pendingStateError reports that the policy was accepted and is valid, but is not enforced yet
// for a reason that resolves on its own with no further action. Callers surface it as a warning:
// the write succeeded, so failing the apply — and tearing the policy back down — would be wrong.
//
// This covers exactly one case. A wait that simply ran out of time is waitTimeoutError, which
// callers must treat as a failure; conflating the two would make the timeouts block decorative.
type pendingStateError struct{}

func (e *pendingStateError) Error() string {
	return "the policy is valid but its target resource does not exist yet (status " +
		"WAITING_FOR_RESOURCE). The grant will bind automatically once the resource is created; " +
		"no further action is required here."
}

// waitTimeoutError reports that the configured wait elapsed before the policy settled. Unlike
// pendingStateError this is a real failure: the practitioner asked to wait for the policy to
// become active and it did not.
type waitTimeoutError struct {
	timeout time.Duration
	status  string // last status successfully read; empty if the policy was never readable
	lastErr error  // last read failure, when the reads themselves were failing
}

func (e *waitTimeoutError) Error() string {
	switch {
	case e.lastErr != nil:
		return fmt.Sprintf("timed out after %s; the policy could not be read (last error: %s).",
			e.timeout, e.lastErr)
	case e.status != "":
		return fmt.Sprintf("timed out after %s waiting for the policy to become active; it was "+
			"still %s. The write itself succeeded, so the service has not finished applying it.",
			e.timeout, e.status)
	default:
		return fmt.Sprintf("timed out after %s and the policy was never readable.", e.timeout)
	}
}

func (e *waitTimeoutError) Unwrap() error { return e.lastErr }

// getPolicy fetches a single policy by name.
func (r *PolicyResource) getPolicy(ctx context.Context, name string) (*policy, error) {
	var out policy
	if err := r.client.Get(ctx, r.client.BuildIAMPath("/policies/"+name), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// putPolicy creates or replaces a policy. The body is raw Cedar sent as text/plain; the service
// has no JSON envelope and no separate create call. Transient server errors are retried.
func (r *PolicyResource) putPolicy(ctx context.Context, name, cedarText string) (*policy, error) {
	path := r.client.BuildIAMPath("/policies/" + name)
	body := client.RawBody{ContentType: "text/plain", Data: []byte(cedarText)}

	var out policy
	var err error
	for attempt := 1; attempt <= maxPutAttempts; attempt++ {
		err = r.client.Put(ctx, path, nil, body, &out)
		if err == nil {
			return &out, nil
		}
		if !isRetryable(err) {
			return nil, err
		}
		if attempt < maxPutAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
	}
	return nil, err
}

const maxPutAttempts = 3

// waitForPolicy polls until the policy reaches a terminal state or timeout elapses.
//
// The returned policy is non-nil whenever the service was reachable, even alongside an error, so
// callers can still persist the latest known state. A *pendingStateError means the write
// succeeded and the policy is simply not bound yet — callers should warn rather than fail.
func (r *PolicyResource) waitForPolicy(ctx context.Context, name string, timeout time.Duration) (*policy, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var last *policy
	var lastErr error
	delay := initialPollInterval

	for {
		pol, err := r.getPolicy(ctx, name)
		switch {
		case err == nil:
			last, lastErr = pol, nil
			switch pol.Status {
			case statusActive:
				return pol, nil
			case statusResourceLookup, statusPendingBind:
				// Still settling — keep polling.
			case statusWaitingForResource:
				// Not transient. Granting ahead of a resource is a documented pattern, and the
				// service parks the grant here until that resource shows up — which may be never,
				// or long after this apply. Polling would just burn the whole timeout and then
				// report the same thing, so return now and let the caller warn.
				return pol, &pendingStateError{}
			default:
				return pol, &terminalStatusError{status: pol.Status}
			}
		case ctx.Err() != nil:
			// Deadline hit mid-request; fall through and report it as a timeout below.
		case isRetryable(err), isNotFound(err), isTransport(err):
			// Retried until the deadline: 5xx, the window before a write reaches the read path,
			// and connection-level failures — over a multi-minute poll a single reset or DNS blip
			// is expected, and aborting on the first one would make applies flaky. Keep the error
			// so a timeout can say what was actually going wrong.
			lastErr = err
		default:
			// 4xx other than 404 (a revoked token, a lost permission) will not resolve by
			// waiting, so fail now rather than burning the timeout.
			return last, err
		}

		select {
		case <-ctx.Done():
			return last, &waitTimeoutError{timeout: timeout, status: lastStatus(last), lastErr: lastErr}
		case <-time.After(delay):
		}

		if delay = time.Duration(float64(delay) * pollBackoffFactor); delay > maxPollInterval {
			delay = maxPollInterval
		}
	}
}

// lastStatus returns the status of the most recent successful read, or "" if there was none.
func lastStatus(p *policy) string {
	if p == nil {
		return ""
	}
	return p.Status
}

// isTransport reports whether err is a connection-level failure rather than a response from the
// API. Those are worth retrying during a long poll; an API error carries a status that tells us
// whether waiting could possibly help.
func isTransport(err error) bool {
	var apiErr *client.APIError
	return err != nil && !errors.As(err, &apiErr)
}

// cedarValidationError carries a plan-time Cedar problem as a diagnostic summary and detail.
type cedarValidationError struct {
	Summary string
	Detail  string
}

func (e *cedarValidationError) Error() string {
	return e.Summary + ": " + e.Detail
}

// validateCedar checks policy text against the rules the IAM policy service enforces, so
// practitioners get actionable errors at plan time instead of a 400 at apply.
//
// It deliberately stops short of the service's full validation. The set of valid actions and the
// Cedar schema are owned by the service and change independently of this provider, so
// replicating them here would go stale. Only the structural rules below are checked.
func validateCedar(cedarText string) *cedarValidationError {
	ps, err := cedar.NewPolicySetFromBytes("policy.cedar", []byte(cedarText))
	if err != nil {
		return &cedarValidationError{
			Summary: "Invalid Cedar Syntax",
			Detail:  fmt.Sprintf("The policy text could not be parsed as Cedar:\n\n%s", err),
		}
	}

	var pol *cedar.Policy
	count := 0
	for _, p := range ps.All() {
		pol = p
		count++
	}
	if count != 1 {
		return &cedarValidationError{
			Summary: "Expected Exactly One Cedar Statement",
			Detail: fmt.Sprintf(
				"The IAM policy service stores one Cedar statement per policy, but the text contains %d. "+
					"Split the statements into separate beyondtrust_iam_policy resources.", count),
		}
	}

	ast := (*expast.Policy)(pol.AST())

	// The site a policy applies to comes from this annotation and nothing else, so a policy
	// without it cannot be scoped to a tenant at all.
	if err := validateSiteIDAnnotation(ast.Annotations); err != nil {
		return err
	}

	// Restrictions the service does not accept yet. It is expected to allow forbid and
	// when/unless conditions in a later release; remove these two checks when it does.
	if ast.Effect != expast.EffectPermit {
		return &cedarValidationError{
			Summary: "Unsupported Cedar Effect",
			Detail: "Only permit policies are supported. forbid is not yet accepted by the IAM " +
				"policy service.",
		}
	}
	if len(ast.Conditions) > 0 {
		return &cedarValidationError{
			Summary: "Unsupported Cedar Condition",
			Detail: "when and unless clauses are not yet accepted by the IAM policy service. " +
				"Express the grant without conditions.",
		}
	}

	return validateQualifiedTypes(ast)
}

// validateSiteIDAnnotation requires a well-formed @siteId annotation.
func validateSiteIDAnnotation(annotations []expast.AnnotationType) *cedarValidationError {
	for _, a := range annotations {
		if string(a.Key) != siteIDAnnotation {
			continue
		}
		if !validators.IsValidUUID(string(a.Value)) {
			return &cedarValidationError{
				Summary: "Invalid @siteId Annotation",
				Detail: fmt.Sprintf("The @siteId annotation must be a UUID, got %q.",
					string(a.Value)),
			}
		}
		return nil
	}
	return &cedarValidationError{
		Summary: "Missing @siteId Annotation",
		Detail: "The policy must carry an @siteId(\"<uuid>\") annotation naming the site it " +
			"applies to. The service does not infer the site from the provider configuration or " +
			"the access token.\n\nExample:\n\n" +
			"    @siteId(\"11111111-2222-3333-4444-555555555555\")\n" +
			"    permit(...);",
	}
}

// validateQualifiedTypes requires namespace-qualified entity types. The service validates against
// a Cedar schema whose types are all namespaced, so a bare `User::"..."` is rejected at apply.
// Scope forms other than `==` and `is` are left to the service to judge.
func validateQualifiedTypes(ast *expast.Policy) *cedarValidationError {
	if eq, ok := ast.Principal.(expast.ScopeTypeEq); ok {
		if err := requireQualified("principal", string(eq.Entity.Type)); err != nil {
			return err
		}
	}

	switch res := ast.Resource.(type) {
	case expast.ScopeTypeEq:
		return requireQualified("resource", string(res.Entity.Type))
	case expast.ScopeTypeIs:
		return requireQualified("resource", string(res.Type))
	}
	return nil
}

func requireQualified(scope, entityType string) *cedarValidationError {
	if strings.Contains(entityType, "::") {
		return nil
	}
	return &cedarValidationError{
		Summary: "Unqualified Cedar Entity Type",
		Detail: fmt.Sprintf(
			"The %s type %q must be namespace-qualified. The IAM policy service validates against "+
				"a Cedar schema in which every type carries its namespace.\n\nValid %s types: %s.",
			scope, entityType, scope, exampleTypes(scope)),
	}
}

// exampleTypes lists the namespace-qualified types valid in a scope. They are spelled out rather
// than derived from what the practitioner wrote, because a bare name like "User" has no correct
// qualified form to suggest — the schema's principal types are Email, Group, Role and Id.
func exampleTypes(scope string) string {
	if scope == "principal" {
		return `Pathfinder::User::Email, Pathfinder::Group, Pathfinder::Role, ` +
			`Pathfinder::Workload::Id`
	}
	return `WorkloadCredentials::Folder, WorkloadCredentials::Secret, ` +
		`WorkloadCredentials::DynamicSecret, WorkloadCredentials::Integration, ` +
		`WorkloadCredentials::Product`
}
