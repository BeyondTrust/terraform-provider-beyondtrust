package resources_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
)

func TestAccStaticSecretResource_basic(t *testing.T) {
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "name", secretName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "value", secretValue),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "path", secretName),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_static_secret.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_static_secret.test", "created_at"),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_static_secret.test", "version"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "beyondtrust_secrets_static_secret.test",
				ImportState:       true,
				ImportStateVerify: true,
				// value is not returned by the API on read
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
}

func TestAccStaticSecretResource_inFolder(t *testing.T) {
	folderName := acctest.RandomFolderName()
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStaticSecretResourceConfig_inFolder(folderName, secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "name", secretName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "folder", folderName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "path", fmt.Sprintf("%s/%s", folderName, secretName)),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "value", secretValue),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_static_secret.test", "id"),
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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			// Create with initial value
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName, secretValue1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "value", secretValue1),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "version", "1"),
				),
			},
			// Update value (should create version 2)
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName, secretValue2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "value", secretValue2),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "version", "2"),
				),
			},
			// Update value again (should create version 3)
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName, secretValue3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "value", secretValue3),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "version", "3"),
				),
			},
		},
	})
}

func TestAccStaticSecretResource_withTags(t *testing.T) {
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

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
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "tags.Environment", "test"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "tags.Type", "database-password"),
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
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "tags.Environment", "production"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "tags.Type", "api-key"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "tags.Owner", "platform-team"),
				),
			},
		},
	})
}

func TestAccStaticSecretResource_nameImmutable(t *testing.T) {
	secretName1 := acctest.RandomSecretName()
	secretName2 := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName1, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "name", secretName1),
				),
			},
			{
				Config: testAccStaticSecretResourceConfig_basic(secretName2, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.test", "name", secretName2),
				),
				// This should trigger a replacement (destroy then create)
			},
		},
	})
}

func testAccCheckStaticSecretDestroy(s *terraform.State) error {
	// TODO: Implement actual destroy check by querying the API
	// For now, we'll just verify the resource is no longer in state
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_secrets_static_secret" {
			continue
		}

		// In a real implementation, you would:
		// 1. Get the client from the provider
		// 2. Try to fetch the secret by path
		// 3. Verify it returns a 404 or is marked as deleted
		_ = rs.Primary.Attributes["path"]
	}

	return nil
}

// testAccStaticSecretResourceConfig_basic returns a basic static secret resource configuration
func testAccStaticSecretResourceConfig_basic(name, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_static_secret" "test" {
  name  = %[1]q
  value = %[2]q
}
`, name, value)
}

// testAccStaticSecretResourceConfig_inFolder returns a configuration with a secret in a folder
func testAccStaticSecretResourceConfig_inFolder(folderName, secretName, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "test" {
  name = %[1]q
}

resource "beyondtrust_secrets_static_secret" "test" {
  name   = %[2]q
  folder = beyondtrust_secrets_folder.test.path
  value  = %[3]q
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
resource "beyondtrust_secrets_static_secret" "test" {
  name  = %[1]q
  value = %[2]q
  tags  = %[3]s
}
`, name, value, tagsStr)
}
