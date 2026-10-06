//go:build acceptance

package ephemeral_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// A skipped acceptance test reports the same green as a passing one. That matters more
// here than almost anywhere else in this repo: CI supplies no Azure credentials, so the
// Azure suite — which covers the only revocable credential type, and the only Close
// implementation — skips in full on every run. Without accounting, "0 Azure tests ran"
// and "all Azure tests passed" look identical in the log.
//
// So the suite accounts for itself: every skip is recorded with its reason, and TestMain
// prints a summary loud enough to read in a CI log.
//
// Passing a precheck is deliberately not treated as having run. The framework applies its
// own gates after PreCheck returns — TerraformVersionChecks in particular, which skips the
// whole case when the resolved Terraform CLI predates ephemeral resource support. That is
// easy to hit by accident (a tfenv shim resolves .terraform-version relative to the
// working directory, and the framework runs Terraform in a temp dir, so `go test` without
// the Makefile's TFENV_TERRAFORM_VERSION silently drops to whatever tfenv defaults to).
// Only a check that executes inside a test step proves a step actually ran.
var (
	skipMu          sync.Mutex
	skipReasons     = map[string][]string{} // reason -> test names
	precheckedTests = map[string]struct{}{} // test names that got past their precheck
	ranTests        = map[string]struct{}{} // test names that executed at least one step
)

// recordSkip notes why a test is being skipped, then skips it.
func recordSkip(t *testing.T, reason string) {
	t.Helper()
	skipMu.Lock()
	skipReasons[reason] = append(skipReasons[reason], t.Name())
	skipMu.Unlock()
	t.Skipf("skipped: %s", reason)
}

// recordPrechecked notes that a test's credentials were present. Keyed by name because
// PreCheck runs both at setup and via the TestCase hook.
func recordPrechecked(t *testing.T) {
	t.Helper()
	skipMu.Lock()
	precheckedTests[t.Name()] = struct{}{}
	skipMu.Unlock()
}

// recordRan notes that a test executed a step. Unlike recordPrechecked this runs from
// inside a TestStep's Check, which is only reached once Terraform has actually applied.
func recordRan(t *testing.T) resource.TestCheckFunc {
	t.Helper()

	return func(*terraform.State) error {
		skipMu.Lock()
		ranTests[t.Name()] = struct{}{}
		skipMu.Unlock()

		return nil
	}
}

// preCheckGrantedAWS gates the AWS tests, which grant themselves generation rather than
// assuming the environment already has.
//
// On top of the AWS fixtures they need the admin site, where the IAM policy API lives, and
// the Cedar entity of the identity the grant is for. The product-site job supplies all
// three; a job without admin credentials skips them and the accounting records why.
//
// The env-var lists live in acctest so this and the direct prechecks cannot drift.
func preCheckGrantedAWS(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)

	if reason := acctest.AWSSkipReason(); reason != "" {
		recordSkip(t, reason)
		return
	}
	if reason := acctest.GenerateGrantSkipReason(); reason != "" {
		recordSkip(t, reason)
		return
	}

	recordPrechecked(t)
}

// preCheckGrantedAzure is preCheckGrantedAWS for Azure: the same grant prerequisites, plus
// the Azure fixtures.
func preCheckGrantedAzure(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)

	if reason := acctest.AzureSkipReason(); reason != "" {
		recordSkip(t, reason)
		return
	}
	if reason := acctest.GenerateGrantSkipReason(); reason != "" {
		recordSkip(t, reason)
		return
	}

	recordPrechecked(t)
}

// preCheckBase is the accounting-aware form of acctest.PreCheck for tests that need
// nothing beyond base credentials.
func preCheckBase(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)
	recordPrechecked(t)
}

// envRequireAcc turns "everything skipped" into a failure. Printing loudly is enough
// locally, where a developer reads the output; CI does not read, it only checks the
// exit code, so set this in any job that is supposed to provide real coverage.
const envRequireAcc = "BEYONDTRUST_TEST_REQUIRE_ACC"

func TestMain(m *testing.M) {
	code := m.Run()
	printSkipSummary(os.Stderr, code)

	if code == 0 && os.Getenv(envRequireAcc) != "" {
		if reason := unverifiedReason(); reason != "" {
			fmt.Fprintf(os.Stderr,
				"FAIL: %s is set, so a run that verified nothing is an error. %s\nSupply the missing "+
					"credentials above, or unset %s if skipping is expected here.\n",
				envRequireAcc, reason, envRequireAcc)
			code = 1
		}
	}
	os.Exit(code)
}

// unverifiedReason returns why the run failed to verify what it claims to, or "" when it
// did. Silently swallowed tests are the case worth catching: a test whose credentials were
// present but which never reached a step was gated by something other than configuration.
// requiredSurface names the tests this gate exists for. The package also holds the
// static-secret ephemeral tests, which need only base credentials and therefore always run;
// counting them would let every dynamic-credential test skip while the gate still passed,
// which is the exact failure it was added to catch.
//
// Matching AWS is enough to prove the surface was exercised. The Azure tests cover the only
// Close and revocation paths, but CI supplies no Azure credentials, so requiring them would
// fail every run; their coverage comes from the unit tests instead.
const requiredSurface = "DynamicSecretEphemeral"

// ranSurfaceLocked reports whether any test naming the given surface executed a step.
// Caller holds skipMu.
func ranSurfaceLocked(surface string) bool {
	for name := range ranTests {
		if strings.Contains(name, surface) {
			return true
		}
	}

	return false
}

func unverifiedReason() string {
	skipMu.Lock()
	defer skipMu.Unlock()

	if len(ranTests) == 0 {
		return "No test executed a single step."
	}
	if !ranSurfaceLocked(requiredSurface) {
		return fmt.Sprintf("No %s test ran; only tests outside the surface this gate covers did.",
			requiredSurface)
	}
	if swallowed := swallowedTestsLocked(); len(swallowed) > 0 {
		return fmt.Sprintf("%d test(s) passed their precheck but never executed a step: %s.",
			len(swallowed), strings.Join(swallowed, ", "))
	}

	return ""
}

// swallowedTestsLocked lists tests whose credentials were present but which never ran.
// Callers must hold skipMu.
func swallowedTestsLocked() []string {
	var swallowed []string
	for name := range precheckedTests {
		if _, ok := ranTests[name]; !ok {
			swallowed = append(swallowed, name)
		}
	}
	sort.Strings(swallowed)

	return swallowed
}

// printSkipSummary reports what the run did and did not cover. code is the suite's exit
// status: a test that failed plainly did run, and its failure already explains itself, so
// the swallowed-test accounting below only applies to an otherwise-green run.
func printSkipSummary(w *os.File, code int) {
	skipMu.Lock()
	defer skipMu.Unlock()

	skipped := 0
	for _, tests := range skipReasons {
		skipped += len(tests)
	}

	var swallowed []string
	if code == 0 {
		swallowed = swallowedTestsLocked()
	}

	if skipped == 0 && len(swallowed) == 0 {
		return
	}

	const rule = "================================================================================"
	fmt.Fprintf(w, "\n%s\n", rule)
	if len(ranTests) == 0 {
		fmt.Fprintf(w, "  DYNAMIC CREDENTIAL EPHEMERAL TESTS: 0 ran — NOTHING WAS VERIFIED (%d skipped, %d dropped)\n",
			skipped, len(swallowed))
	} else {
		fmt.Fprintf(w, "  Dynamic credential ephemeral tests: %d ran, %d skipped, %d dropped\n",
			len(ranTests), skipped, len(swallowed))
	}
	fmt.Fprintf(w, "%s\n", rule)

	reasons := make([]string, 0, len(skipReasons))
	for reason := range skipReasons {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)

	for _, reason := range reasons {
		tests := skipReasons[reason]
		fmt.Fprintf(w, "  %d test(s): %s\n", len(tests), reason)
		fmt.Fprintf(w, "             %s\n", strings.Join(tests, ", "))
	}

	// Credentials were present, so these were dropped by a gate the accounting above
	// cannot see. The Terraform CLI version check is overwhelmingly the likeliest one.
	if len(swallowed) > 0 {
		fmt.Fprintf(w, "  %d test(s): passed their precheck but never executed a step\n", len(swallowed))
		fmt.Fprintf(w, "             %s\n", strings.Join(swallowed, ", "))
		fmt.Fprintf(w, "             Check the resolved Terraform CLI version — ephemeral resources need 1.10+.\n")
		fmt.Fprintf(w, "             Run via `make test-acc`, which pins it, rather than `go test` directly.\n")
	}

	fmt.Fprintf(w, "%s\n\n", rule)
}

// ephemeralProviderFactories adds the echo provider alongside the provider under test.
//
// Ephemeral values never reach state, so the only way to assert on a generated
// credential is to route it through echo, which copies its provider configuration into
// a managed resource attribute. Without this an ephemeral test can only assert "did not
// error", which would not catch returning the wrong field.
func ephemeralProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	factories := make(map[string]func() (tfprotov6.ProviderServer, error), len(acctest.ProtoV6ProviderFactories)+1)
	for name, factory := range acctest.ProtoV6ProviderFactories {
		factories[name] = factory
	}
	factories["echo"] = echoprovider.NewProviderServer()

	return factories
}

// --- grants via the admin API, not via a second Terraform provider -------------------------
//
// These tests write their Cedar grant through a raw admin-site client rather than as a
// beyondtrust_iam_policy resource on an aliased provider. Two reasons.
//
// The one that matters is how the edge authenticates an OIDC caller: it resolves the exchange
// by (site id in the URL path, X-BT-Service-Name), a DynamoDB GetItem on exactly that pair.
// A single service name therefore cannot reach two sites, and a Terraform provider block can
// only carry one. A raw client built from LoadAdminTestConfig can carry the admin identity's
// own name (BEYONDTRUST_ADMIN_SERVICE_NAME) while the provider keeps the product one. The IAM
// policy binding tests do the same for the opposite direction. The name only finds the record;
// the edge then checks the token's claims against that identity's trust conditions, so the
// admin identity must also trust the environment this job runs in.
//
// The other is ordering: an ephemeral resource is opened while the plan is built, so the grant
// has to exist before that plan runs. Writing it in PreConfig and awaiting ACTIVE there is the
// only place that is guaranteed.

const (
	grantStatusActive  = "ACTIVE"
	grantBindTimeout   = 3 * time.Minute
	envGrantBindTimout = "BEYONDTRUST_TEST_POLICY_BIND_TIMEOUT"
)

// grantPolicy is the IAM policy API's read shape; only status is consulted.
type grantPolicy struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// grantViaAdmin writes a Cedar policy on the admin site and blocks until it is ACTIVE, so a
// step that depends on the grant cannot outrun it. The policy is removed when the test ends.
//
// A grant the service accepts but never binds parks in WAITING_FOR_RESOURCE and reports no
// error, so waiting on status is the only way to know the permission is in effect.
func grantViaAdmin(t *testing.T, name, cedar string) {
	t.Helper()

	admin, err := acctest.NewAdminTestClient()
	if err != nil {
		t.Fatalf("admin client: %v", err)
	}

	ctx := context.Background()
	path := admin.BuildIAMPath("/policies/" + name)
	body := client.RawBody{ContentType: "text/plain", Data: []byte(cedar)}

	var written grantPolicy
	if err := admin.Put(ctx, path, nil, body, &written); err != nil {
		t.Fatalf("writing grant %q: %v", name, err)
	}
	t.Cleanup(func() {
		if err := admin.Delete(context.Background(), path, nil); err != nil {
			t.Logf("Cleanup: could not delete grant %q: %v", name, err)
		}
	})

	timeout := grantBindTimeout
	if raw := os.Getenv(envGrantBindTimout); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}
	deadline := time.Now().Add(timeout)
	for {
		var current grantPolicy
		if err := admin.Get(ctx, path, nil, &current); err != nil {
			t.Fatalf("reading grant %q: %v", name, err)
		}
		if current.Status == grantStatusActive {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("grant %q did not become ACTIVE within %s (last status %q); it was accepted but "+
				"the service did not finish binding it", name, timeout, current.Status)
		}
		time.Sleep(2 * time.Second)
	}
}
