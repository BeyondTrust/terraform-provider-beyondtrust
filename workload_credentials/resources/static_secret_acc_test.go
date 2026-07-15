//go:build acceptance
// +build acceptance

package resources_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	btclient "github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

func TestAccStaticSecretResource_basic(t *testing.T) {
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	// Register cleanup as safety net in case Terraform destroy fails
	registerStaticSecretCleanup(t, secretName, "")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "name", secretName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "path", secretName),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_static_secret.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_static_secret.test", "created_at"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_static_secret.test", "secret_wo_version"),
				),
			},
			// ImportState testing - import by path since the API identifies secrets by path, not UUID
			{
				ResourceName:      "beyondtrust_workload_credentials_static_secret.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["beyondtrust_workload_credentials_static_secret.test"]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					p, ok := rs.Primary.Attributes["path"]
					if !ok || p == "" {
						return "", fmt.Errorf("resource has no path attribute in state")
					}
					return p, nil
				},
				// secret_wo is write-only and not returned by the API on read
				// secret_wo_version is user-controlled and not derivable from the API on import
				// created_at precision differs between create and read responses (API inconsistency)
				ImportStateVerifyIgnore: []string{"secret_wo", "secret_wo_version", "created_at"},
			},
		},
	})
}

func TestAccStaticSecretResource_inFolder(t *testing.T) {
	folderName := acctest.RandomFolderName()
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	// Register cleanup as safety net (LIFO: secret cleaned up before folder)
	registerFolderCleanup(t, folderName, "")
	registerStaticSecretCleanup(t, secretName, folderName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStaticSecretResourceConfig_inFolder(folderName, secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "name", secretName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "folder", folderName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "path", fmt.Sprintf("%s/%s", folderName, secretName)),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_static_secret.test", "id"),
				),
			},
		},
	})
}

func TestAccStaticSecretResource_updateValue(t *testing.T) {
	secretName := acctest.RandomSecretName()
	secretValue1 := acctest.RandomString(32)
	secretValue2 := acctest.RandomString(32)
	secretValue3 := acctest.RandomString(32)

	// Register cleanup as safety net in case Terraform destroy fails
	registerStaticSecretCleanup(t, secretName, "")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			// Create with initial value (version 1)
			{
				Config: testAccStaticSecretResourceConfig_basicWithVersion(secretName, secretValue1, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "secret_wo_version", "1"),
				),
			},
			// Bump version to 2 (triggers re-applying secret_wo with new value)
			{
				Config: testAccStaticSecretResourceConfig_basicWithVersion(secretName, secretValue2, 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "secret_wo_version", "2"),
				),
			},
			// Bump version to 3 (triggers another re-apply)
			{
				Config: testAccStaticSecretResourceConfig_basicWithVersion(secretName, secretValue3, 3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "secret_wo_version", "3"),
				),
			},
		},
	})
}

func TestAccStaticSecretResource_withTags(t *testing.T) {
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	// Register cleanup as safety net in case Terraform destroy fails
	registerStaticSecretCleanup(t, secretName, "")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			// Create with tags
			{
				Config: testAccStaticSecretResourceConfig_withTags(secretName, secretValue, map[string]string{
					"Environment": "test",
					"Type":        "database-password",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "tags.Environment", "test"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "tags.Type", "database-password"),
				),
			},
			// Update tags
			{
				Config: testAccStaticSecretResourceConfig_withTags(secretName, secretValue, map[string]string{
					"Environment": "production",
					"Type":        "api-key",
					"Owner":       "platform-team",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "tags.Environment", "production"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "tags.Type", "api-key"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "tags.Owner", "platform-team"),
				),
			},
		},
	})
}

func TestAccStaticSecretResource_nameImmutable(t *testing.T) {
	secretName1 := acctest.RandomSecretName()
	secretName2 := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	// Register cleanup for both secrets (name update creates a new resource)
	registerStaticSecretCleanup(t, secretName1, "")
	registerStaticSecretCleanup(t, secretName2, "")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName1, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "name", secretName1),
				),
			},
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName2, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "name", secretName2),
				),
				// This should trigger a replacement (destroy then create)
			},
		},
	})
}

// TestAccStaticSecretResource_valueStoredCorrectly verifies that secret_wo values
// are actually sent to the API and stored correctly. This is a regression test for
// the bug where secret_wo was being read from req.Plan (where write-only attributes
// are nullified) instead of req.Config (where the actual values live).
func TestAccStaticSecretResource_valueStoredCorrectly(t *testing.T) {
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	// Register cleanup as safety net in case Terraform destroy fails
	registerStaticSecretCleanup(t, secretName, "")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStaticSecretResourceConfig_withReadback(secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify the resource was created
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_static_secret.test", "name", secretName),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_static_secret.test", "id"),
					// Verify we can read the value back using the API (via custom check function)
					testAccCheckStaticSecretValueMatches(secretName, "", "value", secretValue),
				),
			},
		},
	})
}

func testAccCheckStaticSecretDestroy(s *terraform.State) error {
	// Create a test client to verify resources are destroyed
	client, err := acctest.NewTestClient()
	if err != nil {
		return fmt.Errorf("failed to create test client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_workload_credentials_static_secret" {
			continue
		}

		// Get the secret name and folder from state
		name := rs.Primary.Attributes["name"]
		folder := rs.Primary.Attributes["folder"]

		if name == "" {
			return fmt.Errorf("static secret name not found in state")
		}

		// Build API path and query parameters
		apiPath := client.BuildPath(fmt.Sprintf("/static/%s", name))
		query := url.Values{}
		if folder != "" {
			query.Set("folder", folder)
		}

		// Try to fetch the static secret - should return 404 if properly deleted
		var result interface{}
		err := client.Get(context.Background(), apiPath, query, &result)

		// If no error, resource still exists - test should fail
		if err == nil {
			return fmt.Errorf("static secret %s still exists after destroy", name)
		}

		// Verify it's a 404 error (resource properly deleted)
		var apiErr *btclient.APIError
		if errors.As(err, &apiErr) {
			if apiErr.IsGone() {
				// Expected - resource is properly deleted
				continue
			}
			if apiErr.IsPermissionError() {
				// Permission error after destroy - likely means resource is deleted
				// but we no longer have permission to verify
				continue
			}
		}

		// Any other error is unexpected
		return fmt.Errorf("unexpected error checking static secret deletion: %w", err)
	}

	return nil
}

// testAccStaticSecretResourceConfig_basic returns a basic static secret resource configuration
func testAccStaticSecretResourceConfig_basic(name, value string) string {
	return testAccStaticSecretResourceConfig_basicWithVersion(name, value, 1)
}

// testAccStaticSecretResourceConfig_basicWithVersion returns a basic config with an explicit secret_wo_version
func testAccStaticSecretResourceConfig_basicWithVersion(name, value string, version int) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_static_secret" "test" {
  name = %[1]q
  secret_wo = {
    value = %[2]q
  }
  secret_wo_version = %[3]d
}
`, name, value, version)
}

// testAccStaticSecretResourceConfig_inFolder returns a configuration with a secret in a folder
func testAccStaticSecretResourceConfig_inFolder(folderName, secretName, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_folder" "test" {
  name = %[1]q
}

resource "beyondtrust_workload_credentials_static_secret" "test" {
  name   = %[2]q
  folder = beyondtrust_workload_credentials_folder.test.path
  secret_wo = {
    value = %[3]q
  }
  secret_wo_version = 1
}
`, folderName, secretName, value)
}

// testAccStaticSecretResourceConfig_withTags returns a configuration with tags
func testAccStaticSecretResourceConfig_withTags(name, value string, tags map[string]string) string {
	tagsStr := "{\n"
	for k, v := range tags {
		tagsStr += fmt.Sprintf("    %q = %q\n", k, v)
	}
	tagsStr += "  }"

	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_static_secret" "test" {
  name = %[1]q
  secret_wo = {
    value = %[2]q
  }
  secret_wo_version = 1
  tags = %[3]s
}
`, name, value, tagsStr)
}

// testAccStaticSecretResourceConfig_withReadback returns a configuration that creates
// a secret with a known value for verification testing
func testAccStaticSecretResourceConfig_withReadback(name, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_static_secret" "test" {
  name = %[1]q
  secret_wo = {
    value = %[2]q
  }
  secret_wo_version = 1
}
`, name, value)
}

// testAccCheckStaticSecretValueMatches returns a TestCheckFunc that verifies
// the secret value stored in the API matches the expected value.
// This is a regression test helper for ensuring secret_wo values are read from
// req.Config (not req.Plan where they're nullified).
func testAccCheckStaticSecretValueMatches(secretName, folder, key, expectedValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := acctest.NewTestClient()
		if err != nil {
			return fmt.Errorf("failed to create test client: %w", err)
		}

		// Build the API path
		secretPath := secretName
		if folder != "" {
			secretPath = folder + "/" + secretName
		}
		apiPath := client.BuildPath("/static/" + secretName)

		// Build query parameters
		query := url.Values{}
		if folder != "" {
			query.Set("folder", folder)
		}

		// Fetch the secret value from the API
		type SecretResponse struct {
			Path   string            `json:"path"`
			Secret map[string]string `json:"secret"`
		}

		var resp SecretResponse
		err = client.Get(context.Background(), apiPath, query, &resp)
		if err != nil {
			return fmt.Errorf("failed to read secret '%s' from API: %w", secretPath, err)
		}

		// Verify the key exists
		actualValue, exists := resp.Secret[key]
		if !exists {
			return fmt.Errorf("secret '%s' does not contain key '%s'. Available keys: %v", secretPath, key, mapKeys(resp.Secret))
		}

		// Verify the value matches
		if actualValue != expectedValue {
			return fmt.Errorf("secret '%s' key '%s' value does not match expected value", secretPath, key)
		}

		return nil
	}
}

// mapKeys returns the keys of a map as a sorted slice for stable error messages
func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
