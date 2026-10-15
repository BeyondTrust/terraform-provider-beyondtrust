//go:build acceptance
// +build acceptance

package resources_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

const (
	sharedNameAWS   = "beyondtrust_workload_credentials_aws_integration.test"
	sharedNameAzure = "beyondtrust_workload_credentials_azure_integration.test"
)

// TestAccIntegrationResource_sameNameAcrossProviders asserts an AWS and an Azure integration can
// share a name and stay independent: removing the Azure one leaves the AWS one in place.
// Integration names are unique per provider, not per site.
func TestAccIntegrationResource_sameNameAcrossProviders(t *testing.T) {
	name := acctest.RandomIntegrationName()
	roleArn := getTestRoleArn(t)
	tenantID := acctest.GetAzureTenantID(t)
	clientID := acctest.GetAzureClientID(t)
	clientSecret := acctest.GetAzureClientSecret(t)

	registerAwsIntegrationCleanup(t, name)
	registerAzureIntegrationCleanup(t, name)

	awsConfig := testAccAwsIntegrationResourceConfig_basic(name, roleArn)
	azureConfig := testAccAzureIntegrationResourceConfig_basic(name, tenantID, clientID, clientSecret, 1)

	var awsID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t); acctest.PreCheckAzure(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if err := testAccCheckAwsIntegrationDestroy(s); err != nil {
				return err
			}
			return testAccCheckAzureIntegrationDestroy(s)
		},
		Steps: []resource.TestStep{
			{
				Config: awsConfig + azureConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sharedNameAWS, "name", name),
					resource.TestCheckResourceAttr(sharedNameAzure, "name", name),
					checkDistinctIDs(sharedNameAWS, sharedNameAzure),
					captureID(sharedNameAWS, &awsID),
				),
			},
			{
				// Destroys only the Azure integration.
				Config: awsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(sharedNameAWS, "id", &awsID),
					checkAwsIntegrationReadable(name, &awsID),
					checkAzureIntegrationGone(name),
				),
			},
		},
	})
}

func checkDistinctIDs(first, second string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		a, ok := s.RootModule().Resources[first]
		if !ok {
			return fmt.Errorf("%s not found in state", first)
		}
		b, ok := s.RootModule().Resources[second]
		if !ok {
			return fmt.Errorf("%s not found in state", second)
		}
		if a.Primary.ID == "" || a.Primary.ID == b.Primary.ID {
			return fmt.Errorf("expected distinct ids, got %q and %q", a.Primary.ID, b.Primary.ID)
		}
		return nil
	}
}

func captureID(name string, id *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not found in state", name)
		}
		*id = rs.Primary.ID
		return nil
	}
}

// checkAwsIntegrationReadable reads the AWS integration back through the API, so the check does
// not rely on the provider's own refresh.
func checkAwsIntegrationReadable(name string, wantID *string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		client, err := acctest.NewTestClient()
		if err != nil {
			return err
		}
		var got struct {
			Metadata struct {
				ID string `json:"id"`
			} `json:"metadata"`
		}
		if err := client.Get(context.Background(), client.BuildPath("/integrations/aws/"+name), nil, &got); err != nil {
			return fmt.Errorf("reading AWS integration %q after its Azure namesake was destroyed: %w", name, err)
		}
		if got.Metadata.ID != *wantID {
			return fmt.Errorf("AWS integration %q has id %q, want %q", name, got.Metadata.ID, *wantID)
		}
		return nil
	}
}

// checkAzureIntegrationGone reuses the Azure destroy check against a synthetic state holding only
// the Azure integration, since the real state no longer has it.
func checkAzureIntegrationGone(name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		state := terraform.NewState()
		state.RootModule().Resources[sharedNameAzure] = &terraform.ResourceState{
			Type:    "beyondtrust_workload_credentials_azure_integration",
			Primary: &terraform.InstanceState{Attributes: map[string]string{"name": name}},
		}
		return testAccCheckAzureIntegrationDestroy(state)
	}
}
