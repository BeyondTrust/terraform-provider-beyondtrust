//go:build acceptance
// +build acceptance

package resources_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccFolderResource_basic(t *testing.T) {
	// Load test configuration
	cfg, err := acctest.LoadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	folderName := acctest.RandomFolderName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create folder
			{
				Config: testAccFolderConfig_basic(cfg, folderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "name", folderName),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_folder.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_folder.test", "created_at"),
				),
			},
			// Step 2: Import
			{
				ResourceName:      "beyondtrust_workload_credentials_folder.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     folderName, // Import by path, not ID
				// Ignore folder attribute - empty string vs null are semantically equivalent
				ImportStateVerifyIgnore: []string{"folder"},
			},
		},
	})
}

func TestAccFolderResource_update(t *testing.T) {
	cfg, err := acctest.LoadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	folderName := acctest.RandomFolderName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with tags
			{
				Config: testAccFolderConfig_withTags(cfg, folderName, map[string]string{
					"env": "dev",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "name", folderName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.env", "dev"),
				),
			},
			// Step 2: Update tags
			{
				Config: testAccFolderConfig_withTags(cfg, folderName, map[string]string{
					"env":  "staging",
					"team": "platform",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "name", folderName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.env", "staging"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.team", "platform"),
				),
			},
			// Step 3: Remove all tags
			{
				Config: testAccFolderConfig_basic(cfg, folderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "name", folderName),
					resource.TestCheckNoResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.env"),
					resource.TestCheckNoResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.team"),
				),
			},
		},
	})
}

func TestAccFolderResource_tags(t *testing.T) {
	cfg, err := acctest.LoadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	folderName := acctest.RandomFolderName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with tags
			{
				Config: testAccFolderConfig_withTags(cfg, folderName, map[string]string{
					"env":  "staging",
					"team": "platform",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.env", "staging"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.team", "platform"),
				),
			},
			// Step 2: Update tags
			{
				Config: testAccFolderConfig_withTags(cfg, folderName, map[string]string{
					"env":     "production",
					"team":    "platform",
					"managed": "terraform",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.env", "production"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.team", "platform"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.managed", "terraform"),
				),
			},
			// Step 3: Remove all tags
			{
				Config: testAccFolderConfig_basic(cfg, folderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.env"),
					resource.TestCheckNoResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.team"),
					resource.TestCheckNoResourceAttr("beyondtrust_workload_credentials_folder.test", "tags.managed"),
				),
			},
		},
	})
}

func TestAccFolderResource_nested(t *testing.T) {
	cfg, err := acctest.LoadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	parentName := acctest.RandomFolderName()
	childName := acctest.RandomFolderName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccFolderConfig_nested(cfg, parentName, childName),
				Check: resource.ComposeTestCheckFunc(
					// Check parent folder (root level - folder attribute not set)
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.parent", "name", parentName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.parent", "path", parentName),
					// Check child folder (nested under parent)
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.child", "name", childName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.child", "folder", parentName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_folder.child", "path", fmt.Sprintf("%s/%s", parentName, childName)),
				),
			},
		},
	})
}

// Helper functions for test configurations

func testAccFolderConfig_basic(cfg *acctest.TestConfig, name string) string {
	return cfg.ProviderConfig() + fmt.Sprintf(`
resource "beyondtrust_workload_credentials_folder" "test" {
  name = %q
}
`, name)
}

func testAccFolderConfig_withTags(cfg *acctest.TestConfig, name string, tags map[string]string) string {
	config := cfg.ProviderConfig() + fmt.Sprintf(`
resource "beyondtrust_workload_credentials_folder" "test" {
  name = %q
  tags = {
`, name)

	for k, v := range tags {
		config += fmt.Sprintf("    %q = %q\n", k, v)
	}

	config += "  }\n}\n"
	return config
}

func testAccFolderConfig_nested(cfg *acctest.TestConfig, parentName, childName string) string {
	return cfg.ProviderConfig() + fmt.Sprintf(`
resource "beyondtrust_workload_credentials_folder" "parent" {
  name = %q
}

resource "beyondtrust_workload_credentials_folder" "child" {
  name   = %q
  folder = beyondtrust_workload_credentials_folder.parent.name
}
`, parentName, childName)
}

func testAccCheckFolderDestroy(s *terraform.State) error {
	// Create a test client to verify resources are destroyed
	client, err := acctest.NewTestClient()
	if err != nil {
		return fmt.Errorf("failed to create test client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_workload_credentials_folder" {
			continue
		}

		// Get the folder name and parent folder from state
		name := rs.Primary.Attributes["name"]
		folder := rs.Primary.Attributes["folder"]

		if name == "" {
			return fmt.Errorf("folder name not found in state")
		}

		// Build API path and query parameters
		apiPath := client.BuildPath(fmt.Sprintf("/folders/%s/metadata", name))
		query := url.Values{}
		if folder != "" {
			query.Set("folder", folder)
		}

		// Try to fetch the folder metadata - should return 404 if properly deleted
		var result interface{}
		err := client.Get(context.Background(), apiPath, query, &result)

		// If no error, resource still exists - test should fail
		if err == nil {
			return fmt.Errorf("folder %s still exists after destroy", name)
		}

		// Verify it's a 404 error (resource properly deleted)
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			// Expected - resource is properly deleted
			continue
		}

		// Any other error is unexpected
		return fmt.Errorf("unexpected error checking folder deletion: %w", err)
	}

	return nil
}
