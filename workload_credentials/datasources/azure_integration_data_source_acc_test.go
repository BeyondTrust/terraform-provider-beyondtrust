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

func TestAccAzureIntegrationDataSource_basic(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAzureIntegrationDataSourceConfig_basic(integrationName, tenantID, clientID, clientSecret),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.beyondtrust_workload_credentials_azure_integration.test", "name", integrationName),
					resource.TestCheckResourceAttr("data.beyondtrust_workload_credentials_azure_integration.test", "tenant_id", tenantID),
					resource.TestCheckResourceAttr("data.beyondtrust_workload_credentials_azure_integration.test", "client_id", clientID),
					resource.TestCheckResourceAttrSet("data.beyondtrust_workload_credentials_azure_integration.test", "id"),
					resource.TestCheckResourceAttrSet("data.beyondtrust_workload_credentials_azure_integration.test", "created_at"),
				),
			},
		},
	})
}

func testAccAzureIntegrationDataSourceConfig_basic(name, tenantID, clientID, clientSecret string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_azure_integration" "setup" {
  name                  = %[1]q
  tenant_id             = %[2]q
  client_id             = %[3]q
  client_secret         = %[4]q
  client_secret_version = 1
}

data "beyondtrust_workload_credentials_azure_integration" "test" {
  name = beyondtrust_workload_credentials_azure_integration.setup.name
}
`, name, tenantID, clientID, clientSecret)
}
