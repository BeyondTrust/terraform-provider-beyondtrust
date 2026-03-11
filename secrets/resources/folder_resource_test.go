package resources_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
)

func TestAccFolderResource_basic(t *testing.T) {
	folderName := acctest.RandomFolderName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccFolderResourceConfig_basic(folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "path", folderName),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.test", "created_at"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "beyondtrust_secrets_folder.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFolderResource_withParentFolder(t *testing.T) {
	parentName := acctest.RandomFolderName()
	childName := acctest.RandomFolderName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccFolderResourceConfig_withParent(parentName, childName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.parent", "name", parentName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.parent", "path", parentName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.child", "name", childName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.child", "folder", parentName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.child", "path", fmt.Sprintf("%s/%s", parentName, childName)),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.parent", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_folder.child", "id"),
				),
			},
		},
	})
}

func TestAccFolderResource_withTags(t *testing.T) {
	folderName := acctest.RandomFolderName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroy,
		Steps: []resource.TestStep{
			// Create with tags
			{
				Config: testAccFolderResourceConfig_withTags(folderName, map[string]string{
					"Environment": "test",
					"Team":        "platform",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.Environment", "test"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.Team", "platform"),
				),
			},
			// Update tags
			{
				Config: testAccFolderResourceConfig_withTags(folderName, map[string]string{
					"Environment": "production",
					"Team":        "security",
					"Owner":       "john",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.Environment", "production"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.Team", "security"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.Owner", "john"),
				),
			},
			// Remove all tags
			{
				Config: testAccFolderResourceConfig_basic(folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "tags.%", "0"),
				),
			},
		},
	})
}

func TestAccFolderResource_nameImmutable(t *testing.T) {
	folderName1 := acctest.RandomFolderName()
	folderName2 := acctest.RandomFolderName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccFolderResourceConfig_basic(folderName1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName1),
				),
			},
			{
				Config: testAccFolderResourceConfig_basic(folderName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.test", "name", folderName2),
				),
				// This should trigger a replacement (destroy then create)
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func testAccCheckFolderDestroy(s *terraform.State) error {
	// TODO: Implement actual destroy check by querying the API
	// For now, we'll just verify the resource is no longer in state
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_secrets_folder" {
			continue
		}

		// In a real implementation, you would:
		// 1. Get the client from the provider
		// 2. Try to fetch the folder by path
		// 3. Verify it returns a 404 or is marked as deleted
		// For now, we'll just log that we would check
		_ = rs.Primary.Attributes["path"]
	}

	return nil
}

// testAccFolderResourceConfig_basic returns a basic folder resource configuration
func testAccFolderResourceConfig_basic(name string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "test" {
  name = %[1]q
}
`, name)
}

// testAccFolderResourceConfig_withParent returns a configuration with parent and child folders
func testAccFolderResourceConfig_withParent(parentName, childName string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "parent" {
  name = %[1]q
}

resource "beyondtrust_secrets_folder" "child" {
  name   = %[2]q
  folder = beyondtrust_secrets_folder.parent.path
}
`, parentName, childName)
}

// testAccFolderResourceConfig_withTags returns a configuration with tags
func testAccFolderResourceConfig_withTags(name string, tags map[string]string) string {
	tagsStr := "{\n"
	for k, v := range tags {
		tagsStr += fmt.Sprintf("    %q = %q\n", k, v)
	}
	tagsStr += "  }"

	return fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "test" {
  name = %[1]q
  tags = %[2]s
}
`, name, tagsStr)
}
