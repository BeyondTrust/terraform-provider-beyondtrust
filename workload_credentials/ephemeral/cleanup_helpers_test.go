//go:build acceptance
// +build acceptance

package ephemeral_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	btclient "github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// The ephemeral tests stand up real integrations, dynamic secrets and folders to generate
// against. Terraform destroys them at the end of a successful run, but a panic, an
// interrupt, or a destroy that itself fails would otherwise leave them behind on a shared
// test site forever. The managed-resource suites carry the same safety net; these mirror
// it rather than inventing a second convention.
//
// Cleanup is best effort by design: it only logs, because a fixture that was already
// removed by Terraform is the normal case and must not turn a passing test red.

// registerCleanup deletes a fixture by API path once the test finishes.
func registerCleanup(t *testing.T, kind, path string, query url.Values) {
	t.Cleanup(func() {
		if t.Skipped() {
			return
		}

		c, err := acctest.NewTestClient()
		if err != nil {
			t.Logf("Cleanup: failed to create client: %v", err)
			return
		}

		err = c.Delete(context.Background(), c.BuildPath(path), query)
		if err == nil {
			t.Logf("WARNING: Cleanup deleted %s %s (Terraform destroy didn't work)", kind, path)
			return
		}

		var apiErr *btclient.APIError
		if errors.As(err, &apiErr) {
			// Already gone, or gone and no longer visible to this principal.
			if apiErr.IsGone() || apiErr.IsPermissionError() {
				return
			}
		}

		t.Logf("Cleanup: unexpected error deleting %s %s: %v", kind, path, err)
	})
}

func registerIntegrationCleanup(t *testing.T, provider, name string) {
	t.Helper()
	registerCleanup(t, provider+" integration", fmt.Sprintf("/integrations/%s/%s", provider, name), nil)
}

func registerDynamicSecretCleanup(t *testing.T, name, folder string) {
	t.Helper()
	registerCleanup(t, "dynamic secret", "/dynamic/"+name, folderQuery(folder))
}

func registerFolderCleanup(t *testing.T, name string) {
	t.Helper()
	registerCleanup(t, "folder", "/folders/"+name, nil)
}

func folderQuery(folder string) url.Values {
	if folder == "" {
		return nil
	}

	query := url.Values{}
	query.Set("folder", folder)

	return query
}

// testAccCheckEphemeralFixturesDestroyed verifies Terraform removed the fixtures the
// ephemeral tests generate against. Without it a destroy that silently no-ops would leave
// the suite green while leaking an integration and a dynamic secret per run.
func testAccCheckEphemeralFixturesDestroyed(s *terraform.State) error {
	c, err := acctest.NewDestroyCheckClient()
	if err != nil {
		return fmt.Errorf("failed to create test client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		var path string
		var query url.Values

		name := rs.Primary.Attributes["name"]
		if name == "" {
			continue
		}

		switch rs.Type {
		case "beyondtrust_workload_credentials_aws_dynamic_secret",
			"beyondtrust_workload_credentials_azure_dynamic_secret":
			path, query = "/dynamic/"+name, folderQuery(rs.Primary.Attributes["folder"])
		case "beyondtrust_workload_credentials_aws_integration":
			path = "/integrations/aws/" + name
		case "beyondtrust_workload_credentials_azure_integration":
			path = "/integrations/azure/" + name
		case "beyondtrust_workload_credentials_folder":
			path = "/folders/" + name
		default:
			continue
		}

		err := c.Get(context.Background(), c.BuildPath(path), query, nil)
		if err == nil {
			return fmt.Errorf("%s %s still exists after destroy", rs.Type, name)
		}

		var apiErr *btclient.APIError
		if errors.As(err, &apiErr) && (apiErr.IsGone() || apiErr.IsPermissionError()) {
			continue
		}

		return fmt.Errorf("unexpected error checking %s %s was destroyed: %w", rs.Type, name, err)
	}

	return nil
}
