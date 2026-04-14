//go:build acceptance
// +build acceptance

package resources

import (
	"fmt"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
		Steps: []resource.TestStep{
			// Step 1: Create folder
			{
				Config: testAccFolderConfig_basic(cfg, folderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.test", "created_at"),
				),
			},
			// Step 2: Import
			{
				ResourceName:      "beyondtrust_secrets_folder.test",
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
		Steps: []resource.TestStep{
			// Step 1: Create with tags
			{
				Config: testAccFolderConfig_withTags(cfg, folderName, map[string]string{
					"env": "dev",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.env", "dev"),
				),
			},
			// Step 2: Update tags
			{
				Config: testAccFolderConfig_withTags(cfg, folderName, map[string]string{
					"env":  "staging",
					"team": "platform",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.env", "staging"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.team", "platform"),
				),
			},
			// Step 3: Remove all tags
			{
				Config: testAccFolderConfig_basic(cfg, folderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName),
					resource.TestCheckNoResourceAttr("beyondtrust_secrets_folder.test", "tags.env"),
					resource.TestCheckNoResourceAttr("beyondtrust_secrets_folder.test", "tags.team"),
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
		Steps: []resource.TestStep{
			// Step 1: Create with tags
			{
				Config: testAccFolderConfig_withTags(cfg, folderName, map[string]string{
					"env":  "staging",
					"team": "platform",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.env", "staging"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.team", "platform"),
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
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.env", "production"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.team", "platform"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.managed", "terraform"),
				),
			},
			// Step 3: Remove all tags
			{
				Config: testAccFolderConfig_basic(cfg, folderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("beyondtrust_secrets_folder.test", "tags.env"),
					resource.TestCheckNoResourceAttr("beyondtrust_secrets_folder.test", "tags.team"),
					resource.TestCheckNoResourceAttr("beyondtrust_secrets_folder.test", "tags.managed"),
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
		Steps: []resource.TestStep{
			{
				Config: testAccFolderConfig_nested(cfg, parentName, childName),
				Check: resource.ComposeTestCheckFunc(
					// Check parent folder (root level - folder attribute not set)
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.parent", "name", parentName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.parent", "path", parentName),
					// Check child folder (nested under parent)
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.child", "name", childName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.child", "folder", parentName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.child", "path", fmt.Sprintf("%s/%s", parentName, childName)),
				),
			},
		},
	})
}

// Helper functions for test configurations

func testAccFolderConfig_basic(cfg *acctest.TestConfig, name string) string {
	return cfg.ProviderConfig() + fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "test" {
  name = %q
}
`, name)
}

func testAccFolderConfig_withTags(cfg *acctest.TestConfig, name string, tags map[string]string) string {
	config := cfg.ProviderConfig() + fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "test" {
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
resource "beyondtrust_secrets_folder" "parent" {
  name = %q
}

resource "beyondtrust_secrets_folder" "child" {
  name   = %q
  folder = beyondtrust_secrets_folder.parent.name
}
`, parentName, childName)
}
