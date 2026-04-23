//go:build acceptance
// +build acceptance

package datasources_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
)

func TestAccAwsIntegrationDataSource_basic(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	roleArn := getTestRoleArn(t)
	externalId := acctest.RandomString(32)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsIntegrationDataSourceConfig_basic(integrationName, roleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.beyondtrust_workload_credentials_aws_integration.test", "name", integrationName),
					resource.TestCheckResourceAttr("data.beyondtrust_workload_credentials_aws_integration.test", "role_arn", roleArn),
					resource.TestCheckResourceAttrSet("data.beyondtrust_workload_credentials_aws_integration.test", "id"),
					resource.TestCheckResourceAttrSet("data.beyondtrust_workload_credentials_aws_integration.test", "created_at"),
				),
			},
		},
	})
}

func getTestRoleArn(t *testing.T) string {
	return acctest.GetAWSRoleARN(t)
}

// testAccAwsIntegrationDataSourceConfig_basic returns a basic AWS integration data source configuration
func testAccAwsIntegrationDataSourceConfig_basic(name, roleArn, externalId string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "setup" {
  name        = %[1]q
  role_arn    = %[2]q
  external_id = %[3]q
}

data "beyondtrust_workload_credentials_aws_integration" "test" {
  name = beyondtrust_workload_credentials_aws_integration.setup.name
}
`, name, roleArn, externalId)
}
