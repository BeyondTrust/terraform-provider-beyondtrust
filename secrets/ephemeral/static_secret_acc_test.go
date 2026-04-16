//go:build acceptance
// +build acceptance

package ephemeral_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
)

func TestAccStaticSecretEphemeral_basic(t *testing.T) {
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccStaticSecretEphemeralConfig_basic(secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.beyondtrust_secrets_static_secret.test", "path", secretName),
					resource.TestCheckResourceAttr("data.beyondtrust_secrets_static_secret.test", "value", secretValue),
					resource.TestCheckResourceAttrSet("data.beyondtrust_secrets_static_secret.test", "id"),
					resource.TestCheckResourceAttrSet("data.beyondtrust_secrets_static_secret.test", "version"),
					resource.TestCheckResourceAttrSet("data.beyondtrust_secrets_static_secret.test", "created_at"),
				),
			},
		},
	})
}

func TestAccStaticSecretEphemeral_inFolder(t *testing.T) {
	folderName := acctest.RandomFolderName()
	secretName := acctest.RandomSecretName()
	secretValue := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccStaticSecretEphemeralConfig_inFolder(folderName, secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.beyondtrust_secrets_static_secret.test", "path", fmt.Sprintf("%s/%s", folderName, secretName)),
					resource.TestCheckResourceAttr("data.beyondtrust_secrets_static_secret.test", "value", secretValue),
				),
			},
		},
	})
}

func TestAccStaticSecretEphemeral_specificVersion(t *testing.T) {
	secretName := acctest.RandomSecretName()
	secretValue1 := acctest.RandomString(32)
	secretValue2 := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			{
				// Read version 1
				Config: testAccStaticSecretEphemeralConfig_withVersion(secretName, secretValue1, secretValue2, "1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.beyondtrust_secrets_static_secret.test_v1", "version", "1"),
					resource.TestCheckResourceAttr("data.beyondtrust_secrets_static_secret.test_v1", "value", secretValue1),
				),
			},
		},
	})
}

// testAccStaticSecretEphemeralConfig_basic returns a basic ephemeral secret configuration
func testAccStaticSecretEphemeralConfig_basic(name, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_static_secret" "setup" {
  name  = %[1]q
  value = %[2]q
}

ephemeral "beyondtrust_secrets_static_secret" "test" {
  path = beyondtrust_secrets_static_secret.setup.path
}

data "beyondtrust_secrets_static_secret" "test" {
  path    = ephemeral.beyondtrust_secrets_static_secret.test.path
  value   = ephemeral.beyondtrust_secrets_static_secret.test.value
  id      = ephemeral.beyondtrust_secrets_static_secret.test.id
  version = ephemeral.beyondtrust_secrets_static_secret.test.version
  created_at = ephemeral.beyondtrust_secrets_static_secret.test.created_at
}
`, name, value)
}

// testAccStaticSecretEphemeralConfig_inFolder returns a configuration with ephemeral secret in a folder
func testAccStaticSecretEphemeralConfig_inFolder(folderName, secretName, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "setup" {
  name = %[1]q
}

resource "beyondtrust_secrets_static_secret" "setup" {
  name   = %[2]q
  folder = beyondtrust_secrets_folder.setup.path
  value  = %[3]q
}

ephemeral "beyondtrust_secrets_static_secret" "test" {
  path = beyondtrust_secrets_static_secret.setup.path
}

data "beyondtrust_secrets_static_secret" "test" {
  path    = ephemeral.beyondtrust_secrets_static_secret.test.path
  value   = ephemeral.beyondtrust_secrets_static_secret.test.value
  id      = ephemeral.beyondtrust_secrets_static_secret.test.id
  version = ephemeral.beyondtrust_secrets_static_secret.test.version
  created_at = ephemeral.beyondtrust_secrets_static_secret.test.created_at
}
`, folderName, secretName, value)
}

// testAccStaticSecretEphemeralConfig_withVersion returns a configuration that reads a specific version
func testAccStaticSecretEphemeralConfig_withVersion(name, value1, value2, version string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_static_secret" "setup" {
  name  = %[1]q
  value = %[2]q
}

# Update to create version 2
resource "beyondtrust_secrets_static_secret" "setup_v2" {
  name  = %[1]q
  value = %[3]q
  depends_on = [beyondtrust_secrets_static_secret.setup]
}

ephemeral "beyondtrust_secrets_static_secret" "test_v1" {
  path    = beyondtrust_secrets_static_secret.setup_v2.path
  version = %[4]q
}

data "beyondtrust_secrets_static_secret" "test_v1" {
  path    = ephemeral.beyondtrust_secrets_static_secret.test_v1.path
  value   = ephemeral.beyondtrust_secrets_static_secret.test_v1.value
  id      = ephemeral.beyondtrust_secrets_static_secret.test_v1.id
  version = ephemeral.beyondtrust_secrets_static_secret.test_v1.version
  created_at = ephemeral.beyondtrust_secrets_static_secret.test_v1.created_at
}
`, name, value1, value2, version)
}
