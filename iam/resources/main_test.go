//go:build acceptance

package resources_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
)

// A skipped acceptance test reports the same green as a passing one. `go test` prints "ok" for a
// package whose every test skipped, and CI shows a green check for a run that verified nothing —
// which is exactly how a whole suite can sit broken for weeks without anyone noticing.
//
// So the suite accounts for itself: every skip is recorded with the reason, and TestMain prints a
// summary loud enough to read in a CI log.

var (
	skipMu      sync.Mutex
	skipReasons = map[string][]string{} // reason -> test names
	ranTests    = map[string]struct{}{} // test names that got past their precheck
)

// recordSkip notes why a test is being skipped, then skips it.
func recordSkip(t *testing.T, reason string) {
	t.Helper()
	skipMu.Lock()
	skipReasons[reason] = append(skipReasons[reason], t.Name())
	skipMu.Unlock()
	t.Skipf("skipped: %s", reason)
}

// recordRan notes that a test got past its precheck and did real work. Keyed by name because a
// test may check its preconditions more than once (setup plus the TestCase PreCheck hook).
func recordRan(t *testing.T) {
	t.Helper()
	skipMu.Lock()
	ranTests[t.Name()] = struct{}{}
	skipMu.Unlock()
}

// envRequireAcc turns "everything skipped" into a failure. Printing loudly is enough locally,
// where a developer reads the output; CI does not read, it only checks the exit code, so set
// this in any job that is supposed to provide real coverage.
const envRequireAcc = "BEYONDTRUST_TEST_REQUIRE_ACC"

func TestMain(m *testing.M) {
	code := m.Run()
	printSkipSummary(os.Stderr)

	if code == 0 && os.Getenv(envRequireAcc) != "" && len(ranTests) == 0 && len(skipReasons) > 0 {
		fmt.Fprintf(os.Stderr,
			"FAIL: %s is set, so a run that verified nothing is an error. Supply the missing "+
				"credentials above, or unset %s if skipping is expected here.\n",
			envRequireAcc, envRequireAcc)
		code = 1
	}
	os.Exit(code)
}

func printSkipSummary(w *os.File) {
	skipMu.Lock()
	defer skipMu.Unlock()

	skipped := 0
	for _, tests := range skipReasons {
		skipped += len(tests)
	}
	if skipped == 0 {
		return
	}

	const rule = "================================================================================"
	fmt.Fprintf(w, "\n%s\n", rule)
	if len(ranTests) == 0 {
		fmt.Fprintf(w, "  IAM POLICY ACCEPTANCE TESTS: %d skipped, 0 ran — NOTHING WAS VERIFIED\n", skipped)
	} else {
		fmt.Fprintf(w, "  IAM policy acceptance tests: %d ran, %d skipped\n", len(ranTests), skipped)
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
	fmt.Fprintf(w, "%s\n\n", rule)
}
