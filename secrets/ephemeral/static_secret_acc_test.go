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
			// Step 1: Create the managed resource
			{
				Config: testAccStaticSecretResourceConfig_setup(secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.setup", "name", secretName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.setup", "path", secretName),
				),
			},
			// Step 2: Read with ephemeral resource (verify no errors = success)
			{
				Config: testAccStaticSecretEphemeralConfig_basic(secretName, secretValue),
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
			// Step 1: Create folder and secret
			{
				Config: testAccStaticSecretResourceConfig_inFolder(folderName, secretName, secretValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_folder.setup", "name", folderName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.setup", "name", secretName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.setup", "path", fmt.Sprintf("%s/%s", folderName, secretName)),
				),
			},
			// Step 2: Read with ephemeral resource (verify no errors = success)
			{
				Config: testAccStaticSecretEphemeralConfig_inFolder(folderName, secretName, secretValue),
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
			// Step 1: Create initial secret (version 1)
			{
				Config: testAccStaticSecretResourceConfig_setup(secretName, secretValue1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.setup", "name", secretName),
				),
			},
			// Step 2: Update secret to create version 2
			{
				Config: testAccStaticSecretResourceConfig_setup(secretName, secretValue2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_static_secret.setup", "name", secretName),
				),
			},
			// Step 3: Read version 1 with ephemeral resource (verify no errors = success)
			{
				Config: testAccStaticSecretEphemeralConfig_withVersion(secretName, secretValue2, "1"),
			},
		},
	})
}

// testAccStaticSecretResourceConfig_setup returns a managed resource configuration (no ephemeral)
func testAccStaticSecretResourceConfig_setup(name, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_static_secret" "setup" {
  name = %[1]q
  secret_wo = {
    value = %[2]q
  }
}
`, name, value)
}

// testAccStaticSecretResourceConfig_inFolder returns folder + managed resource configuration (no ephemeral)
func testAccStaticSecretResourceConfig_inFolder(folderName, secretName, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "setup" {
  name = %[1]q
}

resource "beyondtrust_secrets_static_secret" "setup" {
  name   = %[2]q
  folder = beyondtrust_secrets_folder.setup.path
  secret_wo = {
    value = %[3]q
  }
}
`, folderName, secretName, value)
}

// testAccStaticSecretEphemeralConfig_basic returns managed + ephemeral resource configuration
// The test verifies the ephemeral resource can be read without errors
func testAccStaticSecretEphemeralConfig_basic(name, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_static_secret" "setup" {
  name = %[1]q
  secret_wo = {
    value = %[2]q
  }
}

ephemeral "beyondtrust_secrets_static_secret" "test" {
  name = %[1]q
}
`, name, value)
}

// testAccStaticSecretEphemeralConfig_inFolder returns folder + managed + ephemeral configuration
func testAccStaticSecretEphemeralConfig_inFolder(folderName, secretName, value string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "setup" {
  name = %[1]q
}

resource "beyondtrust_secrets_static_secret" "setup" {
  name   = %[2]q
  folder = beyondtrust_secrets_folder.setup.path
  secret_wo = {
    value = %[3]q
  }
}

ephemeral "beyondtrust_secrets_static_secret" "test" {
  name   = %[2]q
  folder = %[1]q
}
`, folderName, secretName, value)
}

// testAccStaticSecretEphemeralConfig_withVersion returns managed + ephemeral with version configuration
func testAccStaticSecretEphemeralConfig_withVersion(name, currentValue, version string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_static_secret" "setup" {
  name = %[1]q
  secret_wo = {
    value = %[2]q
  }
}

ephemeral "beyondtrust_secrets_static_secret" "test_v1" {
  name    = %[1]q
  version = %[3]q
}
`, name, currentValue, version)
}
