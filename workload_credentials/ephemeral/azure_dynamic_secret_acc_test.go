//go:build acceptance
// +build acceptance

package ephemeral_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	btclient "github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
	"github.com/beyondtrust/terraform-provider-beyondtrust/workload_credentials/resources"
)

func TestAccAzureDynamicSecretEphemeral_generatesAndRevokes(t *testing.T) {
	testAccAzureDynamicSecretEphemeral(t, true,
		resource.TestCheckResourceAttrSet("echo.azure", "data.client_secret"),
		resource.TestCheckResourceAttrSet("echo.azure", "data.key_id"),
		resource.TestCheckResourceAttrSet("echo.azure", "data.lease_id"),
		resource.TestCheckResourceAttr("echo.azure", "data.tenant_id", os.Getenv(acctest.EnvTestAzureTenantID)),
		resource.TestCheckResourceAttr("echo.azure", "data.revoke_on_close", "true"),
		// Close runs at the end of the apply, before checks, so the lease is already gone.
		testAccCheckLeaseRevoked("echo.azure", true),
	)
}

// revoke_on_close = false must leave the credential usable after the apply.
func TestAccAzureDynamicSecretEphemeral_revokeOnCloseDisabled(t *testing.T) {
	testAccAzureDynamicSecretEphemeral(t, false,
		resource.TestCheckResourceAttr("echo.azure", "data.revoke_on_close", "false"),
		testAccCheckLeaseRevoked("echo.azure", false),
	)
}

// testAccAzureDynamicSecretEphemeral creates the fixtures, generates from them, and sweeps the
// leases left behind so that Terraform can destroy the dynamic secret.
//
// resource.Test rather than ParallelTest: each test authenticates two identities, and running
// them in parallel produced intermittent authentication failures.
func testAccAzureDynamicSecretEphemeral(t *testing.T, revokeOnClose bool, checks ...resource.TestCheckFunc) {
	preCheckGrantedAzure(t)

	env := setupGrantEnv(t)
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	appObjectID := os.Getenv(acctest.EnvTestAzureAppObjectID)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckGrantedAzure(t) },
		ProtoV6ProviderFactories: ephemeralProviderFactories(),
		CheckDestroy:             testAccCheckEphemeralFixturesDestroyed,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					env.createFixtureIntegration(t, "azure", integrationName, env.azureIntegrationRequest())
					// Registered after the integration's cleanup so it runs first: the
					// integration cannot be deleted while the secret uses it.
					env.registerFixtureSecretCleanup(t, dynamicSecretName)
				},
				Config: env.azureSetupConfig(integrationName, dynamicSecretName, appObjectID),
				Check: resource.TestCheckResourceAttr(
					"beyondtrust_workload_credentials_azure_dynamic_secret.test", "name", dynamicSecretName),
			},
			{
				PreConfig: func() {
					grantViaAdmin(t, dynamicSecretName+"-generate", env.generateCedar(dynamicSecretName))
					grantViaAdmin(t, dynamicSecretName+"-revoke", env.revokeCedar(dynamicSecretName))
				},
				Config: env.azureGenerateConfig(integrationName, dynamicSecretName, appObjectID, revokeOnClose),
				Check:  resource.ComposeAggregateTestCheckFunc(append([]resource.TestCheckFunc{recordRan(t)}, checks...)...),
			},
			// The fixtures alone, so no plan opens the ephemeral resource, with every open lease
			// revoked first. Terraform's destroy follows.
			{
				PreConfig: func() { env.requireSwept(t, dynamicSecretName) },
				Config:    env.azureSetupConfig(integrationName, dynamicSecretName, appObjectID),
			},
		},
	})
}

// A lease read is only conclusive when it succeeds. A 404 can mean revoked or not yet
// visible, and the API reports a resource the caller cannot see as 403. So the checks below
// wait for a clear answer instead of trusting the first one.
const (
	// leasePresentTimeout bounds the wait for a lease that should exist.
	leasePresentTimeout = 30 * time.Second

	// leaseAbsentSettle is how long a lease must stay unreadable before it counts as revoked.
	leaseAbsentSettle = 10 * time.Second

	leaseProbeInterval = time.Second
)

// leaseReadable reports whether the lease answers a read. Errors other than not found or
// forbidden are returned, so a test that cannot read leases at all says so.
func leaseReadable(c *btclient.Client, leaseID string) (bool, error) {
	err := c.Get(context.Background(), c.BuildPath("/leases/id/"+leaseID), nil, nil)
	if err == nil {
		return true, nil
	}

	var apiErr *btclient.APIError
	if errors.As(err, &apiErr) && (apiErr.IsGone() || apiErr.IsPermissionError()) {
		return false, nil
	}

	return false, err
}

// testAccCheckLeaseRevoked asserts whether the lease recorded by the echo resource has been
// revoked.
func testAccCheckLeaseRevoked(resourceName string, wantRevoked bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}

		leaseID := rs.Primary.Attributes["data.lease_id"]
		if leaseID == "" {
			return fmt.Errorf("no lease_id recorded on %s", resourceName)
		}

		c, err := acctest.NewDestroyCheckClient()
		if err != nil {
			return fmt.Errorf("creating destroy check client: %w", err)
		}

		if wantRevoked {
			return requireLeaseStaysAbsent(c, leaseID)
		}

		return requireLeaseBecomesReadable(c, leaseID)
	}
}

// requireLeaseBecomesReadable waits for the lease to answer a read.
func requireLeaseBecomesReadable(c *btclient.Client, leaseID string) error {
	deadline := time.Now().Add(leasePresentTimeout)

	for {
		readable, err := leaseReadable(c, leaseID)
		if err != nil {
			return fmt.Errorf("lease %s: unexpected error reading lease: %w", leaseID, err)
		}
		if readable {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("lease %s was not readable within %s, so revoke_on_close = false could not be "+
				"confirmed to have kept it. Either it was revoked anyway, or this principal cannot read leases",
				leaseID, leasePresentTimeout)
		}
		time.Sleep(leaseProbeInterval)
	}
}

// requireLeaseStaysAbsent checks the lease stays unreadable for long enough that one which
// was only not yet visible would have appeared.
//
// On its own this would also pass if lease reads never worked.
// TestAccAzureDynamicSecretEphemeral_revokeOnCloseDisabled rules that out.
func requireLeaseStaysAbsent(c *btclient.Client, leaseID string) error {
	deadline := time.Now().Add(leaseAbsentSettle)

	for {
		readable, err := leaseReadable(c, leaseID)
		if err != nil {
			return fmt.Errorf("lease %s: unexpected error reading lease: %w", leaseID, err)
		}
		if readable {
			return fmt.Errorf("lease %s still exists: revoke_on_close did not revoke the credential", leaseID)
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(leaseProbeInterval)
	}
}

// azureIntegrationRequest is the body that creates the Azure integration fixture.
func (e *grantEnv) azureIntegrationRequest() resources.AzureIntegrationCreateRequest {
	return resources.AzureIntegrationCreateRequest{
		TenantID:     os.Getenv(acctest.EnvTestAzureTenantID),
		ClientID:     os.Getenv(acctest.EnvTestAzureClientID),
		ClientSecret: os.Getenv(acctest.EnvTestAzureClientSecret),
	}
}

// azureSetupConfig declares the dynamic secret fixture. Its integration already exists.
func (e *grantEnv) azureSetupConfig(integrationName, dynamicSecretName, appObjectID string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_azure_dynamic_secret" "test" {
  name                  = %[1]q
  integration_name      = %[2]q
  credential_type       = "service_principal_password"
  application_object_id = %[3]q
  ttl                   = 3600
%[4]s}
`, dynamicSecretName, integrationName, appObjectID, e.folderAttr())
}

func (e *grantEnv) azureGenerateConfig(integrationName, dynamicSecretName, appObjectID string, revokeOnClose bool) string {
	return e.azureSetupConfig(integrationName, dynamicSecretName, appObjectID) + fmt.Sprintf(`
ephemeral "beyondtrust_workload_credentials_azure_dynamic_secret" "test" {
  name            = beyondtrust_workload_credentials_azure_dynamic_secret.test.name
  revoke_on_close = %[1]t
%[2]s}

provider "echo" {
  data = ephemeral.beyondtrust_workload_credentials_azure_dynamic_secret.test
}

resource "echo" "azure" {}
`, revokeOnClose, e.folderAttr())
}
