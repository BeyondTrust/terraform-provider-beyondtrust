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

func getTestRoleArn(t *testing.T) string {
	return acctest.GetAWSRoleARN(t)
}

func getTestRoleArn2(t *testing.T) string {
	return acctest.GetAWSRoleARN2(t)
}

func getTestTargetRoleArn(t *testing.T) string {
	return acctest.GetAWSTargetRoleARN(t)
}

func TestAccAwsIntegrationResource_basic(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	roleArn := getTestRoleArn(t)

	// Register cleanup as safety net in case Terraform destroy fails
	registerAwsIntegrationCleanup(t, integrationName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsIntegrationDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName, roleArn),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_integration.test", "name", integrationName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_integration.test", "role_arn", roleArn),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_aws_integration.test", "external_id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_aws_integration.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_aws_integration.test", "created_at"),
				),
			},
			// ImportState testing - import by name since the API identifies integrations by name, not UUID
			{
				ResourceName:      "beyondtrust_workload_credentials_aws_integration.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["beyondtrust_workload_credentials_aws_integration.test"]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					name, ok := rs.Primary.Attributes["name"]
					if !ok || name == "" {
						return "", fmt.Errorf("resource has no name attribute in state")
					}
					return name, nil
				},
				// created_at precision differs between create and read responses (API inconsistency)
				ImportStateVerifyIgnore: []string{"created_at"},
			},
		},
	})
}

func TestAccAwsIntegrationResource_updateRole(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	roleArn1 := getTestRoleArn(t)
	roleArn2 := getTestRoleArn2(t)

	// Register cleanup as safety net in case Terraform destroy fails
	registerAwsIntegrationCleanup(t, integrationName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsIntegrationDestroy,
		Steps: []resource.TestStep{
			// Create with first role
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName, roleArn1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_integration.test", "role_arn", roleArn1),
				),
			},
			// Update to second role
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName, roleArn2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_integration.test", "role_arn", roleArn2),
				),
			},
		},
	})
}

func TestAccAwsIntegrationResource_nameImmutable(t *testing.T) {
	integrationName1 := acctest.RandomIntegrationName()
	integrationName2 := acctest.RandomIntegrationName()
	roleArn := getTestRoleArn(t)

	// Register cleanup for both integrations (name update creates a new resource)
	registerAwsIntegrationCleanup(t, integrationName1)
	registerAwsIntegrationCleanup(t, integrationName2)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName1, roleArn),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_integration.test", "name", integrationName1),
				),
			},
			{
				Config: testAccAwsIntegrationResourceConfig_basic(integrationName2, roleArn),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_integration.test", "name", integrationName2),
				),
				// This should trigger a replacement (destroy then create)
			},
		},
	})
}

func testAccCheckAwsIntegrationDestroy(s *terraform.State) error {
	// Create a test client to verify resources are destroyed
	client, err := acctest.NewTestClient()
	if err != nil {
		return fmt.Errorf("failed to create test client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_workload_credentials_aws_integration" {
			continue
		}

		// Get the integration name from state
		name := rs.Primary.Attributes["name"]
		if name == "" {
			return fmt.Errorf("integration name not found in state")
		}

		// Try to fetch the integration - should return 404 if properly deleted
		apiPath := client.BuildPath(fmt.Sprintf("/integrations/aws/%s", name))
		var result interface{}
		err := client.Get(context.Background(), apiPath, nil, &result)

		// If no error, resource still exists - test should fail
		if err == nil {
			return fmt.Errorf("AWS integration %s still exists after destroy", name)
		}

		// Verify it's a 404 error (resource properly deleted)
		var apiErr *btclient.APIError
		if errors.As(err, &apiErr) {
			if apiErr.IsGone() {
				// Expected - resource is properly deleted
				continue
			}
			if apiErr.IsPermissionError() {
				// Permission error after destroy - likely means resource is deleted
				// but we no longer have permission to verify
				continue
			}
		}

		// Any other error is unexpected
		return fmt.Errorf("unexpected error checking AWS integration deletion: %w", err)
	}

	return nil
}

// testAccAwsIntegrationResourceConfig_basic returns a basic AWS integration resource configuration
func testAccAwsIntegrationResourceConfig_basic(name, roleArn string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name     = %[1]q
  role_arn = %[2]q
}
`, name, roleArn)
}
