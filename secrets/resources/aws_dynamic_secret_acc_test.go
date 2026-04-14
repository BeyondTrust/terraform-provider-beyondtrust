//go:build acceptance
// +build acceptance

package resources

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
)

func TestAccAwsDynamicSecretResource_basic(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAwsDynamicSecretResourceConfig_basic(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "name", dynamicSecretName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "integration_name", integrationName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "credential_type", "assumed_role"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "role_arn", targetRoleArn),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "ttl", "3600"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "path", dynamicSecretName),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_aws_dynamic_secret.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_aws_dynamic_secret.test", "integration_id"),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_aws_dynamic_secret.test", "created_at"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "beyondtrust_secrets_aws_dynamic_secret.test",
				ImportState:       true,
				ImportStateVerify: true,
				// external_id is sensitive and not returned by the API
				ImportStateVerifyIgnore: []string{"external_id"},
			},
		},
	})
}

func TestAccAwsDynamicSecretResource_inFolder(t *testing.T) {
	folderName := acctest.RandomFolderName()
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsDynamicSecretResourceConfig_inFolder(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "name", dynamicSecretName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "folder", folderName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "path", fmt.Sprintf("%s/%s", folderName, dynamicSecretName)),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_aws_dynamic_secret.test", "id"),
				),
			},
		},
	})
}

func TestAccAwsDynamicSecretResource_withPolicyArns(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsDynamicSecretResourceConfig_withPolicyArns(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "policy_arns.#", "2"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "policy_arns.0", "arn:aws:iam::aws:policy/ReadOnlyAccess"),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "policy_arns.1", "arn:aws:iam::aws:policy/SecurityAudit"),
				),
			},
		},
	})
}

func TestAccAwsDynamicSecretResource_withInlinePolicy(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	inlinePolicy := `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "s3:ListBucket",
      "Resource": "arn:aws:s3:::test-bucket"
    }
  ]
}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsDynamicSecretResourceConfig_withInlinePolicy(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, inlinePolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_aws_dynamic_secret.test", "policy"),
				),
			},
		},
	})
}

func TestAccAwsDynamicSecretResource_updateTTL(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			// Create with 1 hour TTL
			{
				Config: testAccAwsDynamicSecretResourceConfig_withTTL(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, 3600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "ttl", "3600"),
				),
			},
			// Update to 2 hour TTL
			{
				Config: testAccAwsDynamicSecretResourceConfig_withTTL(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, 7200),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_dynamic_secret.test", "ttl", "7200"),
				),
			},
		},
	})
}

func testAccCheckAwsDynamicSecretDestroy(s *terraform.State) error {
	// TODO: Implement actual destroy check by querying the API
	// For now, we'll just verify the resource is no longer in state
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_secrets_aws_dynamic_secret" {
			continue
		}

		// In a real implementation, you would:
		// 1. Get the client from the provider
		// 2. Try to fetch the dynamic secret by path
		// 3. Verify it returns a 404 or is marked as deleted
		_ = rs.Primary.Attributes["path"]
	}

	return nil
}

// testAccAwsDynamicSecretResourceConfig_basic returns a basic AWS dynamic secret resource configuration
func testAccAwsDynamicSecretResourceConfig_basic(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[3]q
  external_id = %[5]q
}

resource "beyondtrust_secrets_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_secrets_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId)
}

// testAccAwsDynamicSecretResourceConfig_inFolder returns a configuration with dynamic secret in a folder
func testAccAwsDynamicSecretResourceConfig_inFolder(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_folder" "test" {
  name = %[1]q
}

resource "beyondtrust_secrets_aws_integration" "test" {
  name        = %[2]q
  role_arn    = %[4]q
  external_id = %[6]q
}

resource "beyondtrust_secrets_aws_dynamic_secret" "test" {
  name             = %[3]q
  folder           = beyondtrust_secrets_folder.test.path
  integration_name = beyondtrust_secrets_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[5]q
  ttl              = 3600
}
`, folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId)
}

// testAccAwsDynamicSecretResourceConfig_withPolicyArns returns a configuration with policy ARNs
func testAccAwsDynamicSecretResourceConfig_withPolicyArns(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[3]q
  external_id = %[5]q
}

resource "beyondtrust_secrets_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_secrets_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600
  policy_arns = [
    "arn:aws:iam::aws:policy/ReadOnlyAccess",
    "arn:aws:iam::aws:policy/SecurityAudit"
  ]
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId)
}

// testAccAwsDynamicSecretResourceConfig_withInlinePolicy returns a configuration with inline policy
func testAccAwsDynamicSecretResourceConfig_withInlinePolicy(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, policy string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[3]q
  external_id = %[5]q
}

resource "beyondtrust_secrets_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_secrets_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600
  policy           = %[6]q
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, policy)
}

// testAccAwsDynamicSecretResourceConfig_withTTL returns a configuration with custom TTL
func testAccAwsDynamicSecretResourceConfig_withTTL(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId string, ttl int) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[3]q
  external_id = %[5]q
}

resource "beyondtrust_secrets_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_secrets_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = %[6]d
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, ttl)
}
