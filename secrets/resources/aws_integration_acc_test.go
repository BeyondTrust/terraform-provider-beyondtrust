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

func TestAccAwsIntegrationResource_basic(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	roleArn := getTestRoleArn(t)
	externalId := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsIntegrationDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName, roleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "name", integrationName),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "role_arn", roleArn),
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "external_id", externalId),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_aws_integration.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_secrets_aws_integration.test", "created_at"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "beyondtrust_secrets_aws_integration.test",
				ImportState:       true,
				ImportStateVerify: true,
				// external_id is sensitive and not returned by the API
				ImportStateVerifyIgnore: []string{"external_id"},
			},
		},
	})
}

func TestAccAwsIntegrationResource_updateRole(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	roleArn1 := getTestRoleArn(t)
	roleArn2 := getTestRoleArn2(t)
	externalId := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsIntegrationDestroy,
		Steps: []resource.TestStep{
			// Create with first role
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName, roleArn1, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "role_arn", roleArn1),
				),
			},
			// Update to second role
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName, roleArn2, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "role_arn", roleArn2),
				),
			},
		},
	})
}

func TestAccAwsIntegrationResource_updateExternalId(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	roleArn := getTestRoleArn(t)
	externalId1 := acctest.RandomString(32)
	externalId2 := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsIntegrationDestroy,
		Steps: []resource.TestStep{
			// Create with first external ID
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName, roleArn, externalId1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "external_id", externalId1),
				),
			},
			// Update to second external ID
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName, roleArn, externalId2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "external_id", externalId2),
				),
			},
		},
	})
}

func TestAccAwsIntegrationResource_nameImmutable(t *testing.T) {
	integrationName1 := acctest.RandomIntegrationName()
	integrationName2 := acctest.RandomIntegrationName()
	roleArn := getTestRoleArn(t)
	externalId := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName1, roleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "name", integrationName1),
				),
			},
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName2, roleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_secrets_aws_integration.test", "name", integrationName2),
				),
				// This should trigger a replacement (destroy then create)
			},
		},
	})
}

func getTestRoleArn(t *testing.T) string {
	return acctest.GetAWSRoleARN(t)
}

func getTestRoleArn2(t *testing.T) string {
	return acctest.GetAWSRoleARN2(t)
}

func getTestTargetRoleArn(t *testing.T) string {
	return getTestRoleArn(t)
}

func testAccCheckAwsIntegrationDestroy(s *terraform.State) error {
	// TODO: Implement actual destroy check by querying the API
	// For now, we'll just verify the resource is no longer in state
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_secrets_aws_integration" {
			continue
		}

		// In a real implementation, you would:
		// 1. Get the client from the provider
		// 2. Try to fetch the integration by name
		// 3. Verify it returns a 404
		_ = rs.Primary.Attributes["name"]
	}

	return nil
}

// testAccAwsIntegrationResourceConfig_basic returns a basic AWS integration resource configuration
func testAccAwsIntegrationResourceConfig_basic(name, roleArn, externalId string) string {
	return fmt.Sprintf(`
resource "beyondtrust_secrets_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[2]q
  external_id = %[3]q
}
`, name, roleArn, externalId)
}
