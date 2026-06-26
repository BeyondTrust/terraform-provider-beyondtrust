//go:build acceptance
// +build acceptance

package resources_test

import (
	"fmt"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const wiResourceName = "beyondtrust_auth_workload_identity.test"

func TestAccWorkloadIdentityResource_basicAndUpdate(t *testing.T) {
	cfg, err := acctest.LoadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	serviceName := acctest.RandomFolderName()
	issuerURL := "https://token.actions.githubusercontent.com"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: testAccWorkloadIdentityConfig(cfg, serviceName, issuerURL, "initial description"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(wiResourceName, "service_name", serviceName),
					resource.TestCheckResourceAttr(wiResourceName, "idp_category", "GitHubActions"),
					resource.TestCheckResourceAttr(wiResourceName, "description", "initial description"),
					resource.TestCheckResourceAttrSet(wiResourceName, "id"),
					resource.TestCheckResourceAttrSet(wiResourceName, "organization_id"),
					resource.TestCheckResourceAttrSet(wiResourceName, "expected_aud"),
				),
			},
			// Update a mutable field (description) in place
			{
				Config: testAccWorkloadIdentityConfig(cfg, serviceName, issuerURL, "updated description"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(wiResourceName, "description", "updated description"),
				),
			},
			// Import by identity id
			{
				ResourceName:      wiResourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// Immutable identity fields are kept from state on read (not echoed from the
				// API, which lower-cases them), so don't verify them on import.
				ImportStateVerifyIgnore: []string{"service_name", "issuer_url", "idp_category"},
			},
		},
	})
}

func testAccWorkloadIdentityConfig(cfg *acctest.TestConfig, serviceName, issuerURL, description string) string {
	return cfg.ProviderConfig() + fmt.Sprintf(`
resource "beyondtrust_auth_workload_identity" "test" {
  service_name      = %[1]q
  issuer_url        = %[2]q
  idp_category      = "GitHubActions"
  registered_scopes = ["admin"]
  description       = %[3]q
  conditions = {
    sub        = ["repo:myorg/myrepo:ref:refs/heads/main"]
    repository = ["myorg/myrepo"]
  }
}
`, serviceName, issuerURL, description)
}
