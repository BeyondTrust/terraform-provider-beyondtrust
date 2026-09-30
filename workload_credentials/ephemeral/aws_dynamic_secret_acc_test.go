//go:build acceptance
// +build acceptance

package ephemeral_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
)

func TestAccAwsDynamicSecretEphemeral_generatesCredentials(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := acctest.GetAWSRoleARN(t)
	targetRoleArn := acctest.GetAWSTargetRoleARN(t)

	// Safety net (LIFO: secret cleaned up before the integration it references).
	registerIntegrationCleanup(t, "aws", integrationName)
	registerDynamicSecretCleanup(t, dynamicSecretName, "")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { preCheckAWS(t) },
		ProtoV6ProviderFactories: ephemeralProviderFactories(),
		CheckDestroy:             testAccCheckEphemeralFixturesDestroyed,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			// Step 1: create the dynamic secret definition on its own. The ephemeral
			// resource cannot be opened until this exists.
			{
				Config: testAccAwsEphemeralConfig_setup(integrationName, dynamicSecretName, roleArn, targetRoleArn),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "name", dynamicSecretName),
				),
			},
			// Step 2: generate credentials from it and assert on what came back.
			{
				Config: testAccAwsEphemeralConfig_generate(integrationName, dynamicSecretName, roleArn, targetRoleArn),
				Check: resource.ComposeAggregateTestCheckFunc(
					recordRan(t),
					// STS session keys are distinguishable from long-lived IAM keys
					// (AKIA...) by their ASIA prefix, so this confirms the credential
					// really was assumed rather than echoed back from the integration.
					resource.TestMatchResourceAttr("echo.aws", "data.access_key_id", regexp.MustCompile(`^ASIA`)),
					resource.TestCheckResourceAttrSet("echo.aws", "data.secret_access_key"),
					resource.TestCheckResourceAttrSet("echo.aws", "data.session_token"),
					resource.TestCheckResourceAttrSet("echo.aws", "data.lease_id"),
					// AWS credentials carry a server-authoritative expiration.
					resource.TestMatchResourceAttr("echo.aws", "data.expiration", regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`)),
				),
			},
		},
	})
}

// A dynamic secret inside a folder must be addressed with the folder query parameter.
// Omitting it resolves against the root and silently fails to find the secret.
func TestAccAwsDynamicSecretEphemeral_inFolder(t *testing.T) {
	folderName := acctest.RandomFolderName()
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := acctest.GetAWSRoleARN(t)
	targetRoleArn := acctest.GetAWSTargetRoleARN(t)

	// Safety net (LIFO: secret, then folder, then integration).
	registerIntegrationCleanup(t, "aws", integrationName)
	registerFolderCleanup(t, folderName)
	registerDynamicSecretCleanup(t, dynamicSecretName, folderName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { preCheckAWS(t) },
		ProtoV6ProviderFactories: ephemeralProviderFactories(),
		CheckDestroy:             testAccCheckEphemeralFixturesDestroyed,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccAwsEphemeralConfig_inFolderSetup(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn),
			},
			{
				Config: testAccAwsEphemeralConfig_inFolderGenerate(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn),
				Check: resource.ComposeAggregateTestCheckFunc(
					recordRan(t),
					resource.TestMatchResourceAttr("echo.aws", "data.access_key_id", regexp.MustCompile(`^ASIA`)),
					resource.TestCheckResourceAttrSet("echo.aws", "data.session_token"),
				),
			},
		},
	})
}

func testAccAwsEphemeralConfig_setup(integrationName, dynamicSecretName, roleArn, targetRoleArn string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name     = %[1]q
  role_arn = %[3]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn)
}

func testAccAwsEphemeralConfig_generate(integrationName, dynamicSecretName, roleArn, targetRoleArn string) string {
	return testAccAwsEphemeralConfig_setup(integrationName, dynamicSecretName, roleArn, targetRoleArn) + `
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name = beyondtrust_workload_credentials_aws_dynamic_secret.test.name

  depends_on = [beyondtrust_workload_credentials_aws_dynamic_secret.test]
}

provider "echo" {
  data = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.test
}

resource "echo" "aws" {}
`
}

func testAccAwsEphemeralConfig_inFolderSetup(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_folder" "test" {
  name = %[1]q
}

resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name     = %[2]q
  role_arn = %[4]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[3]q
  folder           = beyondtrust_workload_credentials_folder.test.name
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[5]q
  ttl              = 3600
}
`, folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn)
}

func testAccAwsEphemeralConfig_inFolderGenerate(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn string) string {
	return testAccAwsEphemeralConfig_inFolderSetup(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn) + `
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name   = beyondtrust_workload_credentials_aws_dynamic_secret.test.name
  folder = beyondtrust_workload_credentials_folder.test.name

  depends_on = [beyondtrust_workload_credentials_aws_dynamic_secret.test]
}

provider "echo" {
  data = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.test
}

resource "echo" "aws" {}
`
}
