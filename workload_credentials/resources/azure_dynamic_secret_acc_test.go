//go:build acceptance
// +build acceptance

package resources_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	btclient "github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

func TestAccAzureDynamicSecretResource_basic(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)
	appObjectID := acctest.GetAzureAppObjectID(t)

	registerAzureIntegrationCleanup(t, integrationName)
	registerAzureDynamicSecretCleanup(t, dynamicSecretName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureDynamicSecretDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAzureDynamicSecretResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID, 3600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "name", dynamicSecretName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "integration_name", integrationName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "credential_type", "service_principal_password"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "application_object_id", appObjectID),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "ttl", "3600"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "path", dynamicSecretName),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_azure_dynamic_secret.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_azure_dynamic_secret.test", "integration_id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_azure_dynamic_secret.test", "created_at"),
				),
			},
			// ImportState testing — import by "integration-name:path"
			{
				ResourceName:      "beyondtrust_workload_credentials_azure_dynamic_secret.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["beyondtrust_workload_credentials_azure_dynamic_secret.test"]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					intName := rs.Primary.Attributes["integration_name"]
					p := rs.Primary.Attributes["path"]
					if intName == "" || p == "" {
						return "", fmt.Errorf("resource missing integration_name or path in state")
					}
					return intName + ":" + p, nil
				},
				ImportStateVerifyIgnore: []string{"created_at"},
			},
		},
	})
}

func TestAccAzureDynamicSecretResource_inFolder(t *testing.T) {
	folderName := acctest.RandomFolderName()
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)
	appObjectID := acctest.GetAzureAppObjectID(t)

	registerAzureIntegrationCleanup(t, integrationName)
	registerFolderCleanup(t, folderName, "")
	registerAzureDynamicSecretCleanup(t, fmt.Sprintf("%s/%s", folderName, dynamicSecretName))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureDynamicSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAzureDynamicSecretResourceConfig_inFolder(folderName, integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "name", dynamicSecretName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "folder", folderName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "path", fmt.Sprintf("%s/%s", folderName, dynamicSecretName)),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_azure_dynamic_secret.test", "id"),
				),
			},
		},
	})
}

func TestAccAzureDynamicSecretResource_updateTTL(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)
	appObjectID := acctest.GetAzureAppObjectID(t)

	registerAzureIntegrationCleanup(t, integrationName)
	registerAzureDynamicSecretCleanup(t, dynamicSecretName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureDynamicSecretDestroy,
		Steps: []resource.TestStep{
			// Create with 1 hour TTL
			{
				Config: testAccAzureDynamicSecretResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID, 3600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "ttl", "3600"),
				),
			},
			// Update to 4 hour TTL
			{
				Config: testAccAzureDynamicSecretResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID, 14400),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "ttl", "14400"),
				),
			},
		},
	})
}

func TestAccAzureDynamicSecretResource_updateAppObjectID(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)
	appObjectID := acctest.GetAzureAppObjectID(t)

	// For this test we reuse the same app object ID for both steps
	// since we only have one test app — the important thing is that the PATCH path is exercised
	registerAzureIntegrationCleanup(t, integrationName)
	registerAzureDynamicSecretCleanup(t, dynamicSecretName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureDynamicSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAzureDynamicSecretResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID, 3600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "application_object_id", appObjectID),
				),
			},
			// Update TTL alongside (application_object_id same value to keep test simple)
			{
				Config: testAccAzureDynamicSecretResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID, 7200),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "application_object_id", appObjectID),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "ttl", "7200"),
				),
			},
		},
	})
}

func testAccCheckAzureDynamicSecretDestroy(s *terraform.State) error {
	client, err := acctest.NewTestClient()
	if err != nil {
		return fmt.Errorf("failed to create test client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_workload_credentials_azure_dynamic_secret" {
			continue
		}

		name := rs.Primary.Attributes["name"]
		folder := rs.Primary.Attributes["folder"]

		if name == "" {
			return fmt.Errorf("dynamic secret name not found in state")
		}

		apiPath := client.BuildPath(fmt.Sprintf("/dynamic/%s", name))
		query := url.Values{}
		if folder != "" {
			query.Set("folder", folder)
		}

		var result interface{}
		err := client.Get(context.Background(), apiPath, query, &result)

		if err == nil {
			return fmt.Errorf("Azure dynamic secret %s still exists after destroy", name)
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

		return fmt.Errorf("unexpected error checking Azure dynamic secret deletion: %w", err)
	}

	return nil
}

func testAccAzureDynamicSecretResourceConfig_basic(integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID string, ttl int) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_azure_integration" "test" {
  name                  = %[1]q
  tenant_id             = %[2]q
  client_id             = %[3]q
  client_secret         = %[4]q
  client_secret_version = 1
}

resource "beyondtrust_workload_credentials_azure_dynamic_secret" "test" {
  name                 = %[5]q
  integration_name     = beyondtrust_workload_credentials_azure_integration.test.name
  credential_type      = "service_principal_password"
  application_object_id = %[6]q
  ttl                  = %[7]d
}
`, integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID, ttl)
}

func testAccAzureDynamicSecretResourceConfig_inFolder(folderName, integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_folder" "test" {
  name = %[1]q
}

resource "beyondtrust_workload_credentials_azure_integration" "test" {
  name                  = %[2]q
  tenant_id             = %[3]q
  client_id             = %[4]q
  client_secret         = %[5]q
  client_secret_version = 1
}

resource "beyondtrust_workload_credentials_azure_dynamic_secret" "test" {
  name                 = %[6]q
  folder               = beyondtrust_workload_credentials_folder.test.path
  integration_name     = beyondtrust_workload_credentials_azure_integration.test.name
  credential_type      = "service_principal_password"
  application_object_id = %[7]q
  ttl                  = 3600
}
`, folderName, integrationName, tenantID, clientID, clientSecret, dynamicSecretName, appObjectID)
}
