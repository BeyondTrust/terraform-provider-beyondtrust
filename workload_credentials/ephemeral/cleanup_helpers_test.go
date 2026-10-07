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

// Terraform destroys the fixtures at the end of a successful run. These cleanups catch the
// runs where it does not, such as a panic or a failed destroy. They only log, because a
// fixture Terraform already removed is the normal case.

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
		if errors.As(err, &apiErr) && (apiErr.IsGone() || apiErr.IsPermissionError()) {
			return
		}

		t.Logf("Cleanup: unexpected error deleting %s %s: %v", kind, path, err)
	})
}

func registerDynamicSecretCleanup(t *testing.T, name, folder string) {
	t.Helper()
	registerCleanup(t, "dynamic secret", "/dynamic/"+name, deleteQuery(folder))
}

// folderQuery scopes a request to a folder.
func folderQuery(folder string) url.Values {
	if folder == "" {
		return nil
	}

	query := url.Values{}
	query.Set("folder", folder)

	return query
}

// deleteQuery is folderQuery plus permanent=true, as the provider's own Delete sends.
// Without it the fixture is soft-deleted and keeps holding its path.
func deleteQuery(folder string) url.Values {
	query := folderQuery(folder)
	if query == nil {
		query = url.Values{}
	}
	query.Set("permanent", "true")

	return query
}

// testAccCheckEphemeralFixturesDestroyed verifies Terraform removed the fixtures the
// ephemeral tests generate against.
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
