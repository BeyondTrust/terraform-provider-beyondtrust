//go:build acceptance
// +build acceptance

package resources_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/constants"
)

// Environment variables specific to the IAM policy acceptance tests. The policy service needs a
// real principal and a real target site, neither of which the test can create for itself.
const (
	envPolicySiteID    = "BEYONDTRUST_TEST_POLICY_SITE_ID"
	envPolicyPrincipal = "BEYONDTRUST_TEST_POLICY_PRINCIPAL_EMAIL"
	envPolicyFolder    = "BEYONDTRUST_TEST_POLICY_FOLDER_PATH"
)

type policyTestEnv struct {
	cfg       *acctest.TestConfig
	siteID    string
	principal string
	folder    string
}

// preCheckPolicyEnv skips the test unless admin-site credentials and the policy-specific
// fixtures are configured.
func preCheckPolicyEnv(t *testing.T) *policyTestEnv {
	t.Helper()

	if _, err := acctest.LoadAdminTestConfig(); err != nil {
		recordSkip(t, fmt.Sprintf("%v (set %s and %s)", err, acctest.EnvAdminSiteID, acctest.EnvAdminAccessToken))
	}

	// The target site defaults to the product site, so a normally-configured environment does
	// not need a policy-specific override. Requiring one here made this whole suite skip
	// silently for anyone who had not set a variable they did not need.
	siteID := acctest.PolicyTargetSiteID()
	if siteID == "" {
		recordSkip(t, fmt.Sprintf("%s is not set (or %s)", constants.EnvSiteID, envPolicySiteID))
	}
	principal := os.Getenv(envPolicyPrincipal)
	if principal == "" {
		recordSkip(t, fmt.Sprintf("%s is not set", envPolicyPrincipal))
	}
	recordRan(t)

	folder := os.Getenv(envPolicyFolder)
	if folder == "" {
		folder = "/"
	}

	cfg, err := acctest.LoadAdminTestConfig()
	if err != nil {
		t.Fatalf("Failed to load admin test config: %v", err)
	}

	return &policyTestEnv{cfg: cfg, siteID: siteID, principal: principal, folder: folder}
}

// newAdminTestClient builds a client pointed at the admin site, which is where the IAM policy
// API lives. acctest.NewTestClient targets the regular product site instead.
func newAdminTestClient() (*client.Client, error) {
	return acctest.NewAdminTestClient()
}

// testPolicy mirrors the fields of the API response the tests care about.
type testPolicy struct {
	Name   string `json:"name"`
	Cedar  string `json:"cedar"`
	Status string `json:"status"`
}

func getPolicyForTest(c *client.Client, name string) (*testPolicy, error) {
	var pol testPolicy
	if err := c.Get(context.Background(), c.BuildIAMPath("/policies/"+name), nil, &pol); err != nil {
		return nil, err
	}
	return &pol, nil
}

func isNotFoundForTest(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.IsNotFound()
}

func isPermissionErrorForTest(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.IsPermissionError()
}

// registerPolicyCleanup deletes the policy directly through the API after the test, as a safety
// net for runs that fail before Terraform can destroy it.
func registerPolicyCleanup(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		c, err := newAdminTestClient()
		if err != nil {
			t.Logf("cleanup: could not build admin client for policy %q: %v", name, err)
			return
		}
		// DELETE is idempotent and never 404s, so an error here is a real problem worth logging.
		if err := c.Delete(context.Background(), c.BuildIAMPath("/policies/"+name), nil); err != nil {
			if !isNotFoundForTest(err) {
				t.Logf("cleanup: could not delete policy %q: %v", name, err)
			}
		}
	})
}

func regexpMissingSiteID() *regexp.Regexp {
	return regexp.MustCompile(`Missing @siteId Annotation`)
}

func regexpUpdatedAction() *regexp.Regexp {
	return regexp.MustCompile(`Action::"` + regexp.QuoteMeta(vocabulary().folderAltAction) + `"`)
}
