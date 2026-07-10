//go:build acceptance
// +build acceptance

package resources_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	btclient "github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

func TestAccAzureIntegrationResource_basic(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)

	registerAzureIntegrationCleanup(t, integrationName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureIntegrationDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAzureIntegrationResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_integration.test", "name", integrationName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_integration.test", "tenant_id", tenantID),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_integration.test", "client_id", clientID),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_integration.test", "client_secret_version", "1"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_azure_integration.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_azure_integration.test", "created_at"),
				),
			},
			// ImportState testing — import by name
			{
				ResourceName:      "beyondtrust_workload_credentials_azure_integration.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["beyondtrust_workload_credentials_azure_integration.test"]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					name, ok := rs.Primary.Attributes["name"]
					if !ok || name == "" {
						return "", fmt.Errorf("resource has no name attribute in state")
					}
					return name, nil
				},
				// client_secret is write-only (never in state); created_at precision differs
				ImportStateVerifyIgnore: []string{"client_secret", "client_secret_version", "created_at"},
			},
		},
	})
}

func TestAccAzureIntegrationResource_updateCredentials(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)

	registerAzureIntegrationCleanup(t, integrationName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureIntegrationDestroy,
		Steps: []resource.TestStep{
			// Create with initial credentials
			{
				Config: testAccAzureIntegrationResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_integration.test", "client_secret_version", "1"),
				),
			},
			// Rotate client secret by bumping version
			{
				Config: testAccAzureIntegrationResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_integration.test", "client_secret_version", "2"),
				),
			},
		},
	})
}

func TestAccAzureIntegrationResource_nameImmutable(t *testing.T) {
	integrationName1 := acctest.RandomIntegrationName()
	integrationName2 := acctest.RandomIntegrationName()
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)

	registerAzureIntegrationCleanup(t, integrationName1)
	registerAzureIntegrationCleanup(t, integrationName2)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAzureIntegrationResourceConfig_basic(integrationName1, tenantID, clientID, clientSecret, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_integration.test", "name", integrationName1),
				),
			},
			// Changing name triggers replacement
			{
				Config: testAccAzureIntegrationResourceConfig_basic(integrationName2, tenantID, clientID, clientSecret, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_integration.test", "name", integrationName2),
				),
			},
		},
	})
}

func testAccCheckAzureIntegrationDestroy(s *terraform.State) error {
	client, err := acctest.NewTestClient()
	if err != nil {
		return fmt.Errorf("failed to create test client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_workload_credentials_azure_integration" {
			continue
		}

		name := rs.Primary.Attributes["name"]
		if name == "" {
			return fmt.Errorf("integration name not found in state")
		}

		apiPath := client.BuildPath(fmt.Sprintf("/integrations/azure/%s", name))
		var result interface{}
		err := client.Get(context.Background(), apiPath, nil, &result)

		if err == nil {
			return fmt.Errorf("Azure integration %s still exists after destroy", name)
		}

		var apiErr *btclient.APIError
		if errors.As(err, &apiErr) {
			if apiErr.IsGone() {
				continue
			}
			if apiErr.IsPermissionError() {
				continue
			}
		}

		return fmt.Errorf("unexpected error checking Azure integration deletion: %w", err)
	}

	return nil
}

func testAccAzureIntegrationResourceConfig_basic(name, tenantID, clientID, clientSecret string, secretVersion int) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_azure_integration" "test" {
  name                  = %[1]q
  tenant_id             = %[2]q
  client_id             = %[3]q
  client_secret         = %[4]q
  client_secret_version = %[5]d
}
`, name, tenantID, clientID, clientSecret, secretVersion)
}
