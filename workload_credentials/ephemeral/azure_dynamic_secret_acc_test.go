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

// The Azure tests create their fixtures per run; grant_helpers_test.go says how and why, and
// why they end with a step that sweeps the leases they leave behind.
func TestAccAzureDynamicSecretEphemeral_generatesAndRevokes(t *testing.T) {
	preCheckGrantedAzure(t)

	env := setupGrantEnv(t)
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	appObjectID := os.Getenv(acctest.EnvTestAzureAppObjectID)

	// resource.Test, not ParallelTest, deliberately.
	//
	// Each of these configures two identities — the admin that authors the grants and the
	// principal that uses them — and every provider configuration costs an OIDC exchange.
	// Run in parallel, the three tests produced transient 403s on the admin's policy reads
	// and 401 "OIDC workload exchange denied" on the principal's writes, varying between
	// tests within a single run while the same code succeeded elsewhere in it. Serialising
	// trades a little wall clock for a suite whose failures mean something.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckGrantedAzure(t) },
		ProtoV6ProviderFactories: ephemeralProviderFactories(),
		CheckDestroy:             testAccCheckEphemeralFixturesDestroyed,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			// See the AWS test for the ordering of fixtures, grants and cleanups.
			{
				PreConfig: func() {
					env.createFixtureIntegration(t, "azure", integrationName, env.azureIntegrationRequest())
					registerDynamicSecretCleanup(t, dynamicSecretName, env.fixtureRoot)
				},
				Config: env.azureSetupConfig(integrationName, dynamicSecretName, appObjectID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_azure_dynamic_secret.test", "name", dynamicSecretName),
				),
			},
			{
				PreConfig: func() {
					grantViaAdmin(t, dynamicSecretName+"-generate", env.generateCedar(dynamicSecretName))
					grantViaAdmin(t, dynamicSecretName+"-revoke", env.revokeCedar(dynamicSecretName))
				},
				Config: env.azureGenerateConfig(integrationName, dynamicSecretName, appObjectID, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					recordRan(t),
					resource.TestCheckResourceAttrSet("echo.azure", "data.client_secret"),
					resource.TestCheckResourceAttrSet("echo.azure", "data.key_id"),
					resource.TestCheckResourceAttrSet("echo.azure", "data.lease_id"),
					resource.TestCheckResourceAttr("echo.azure", "data.tenant_id", os.Getenv(acctest.EnvTestAzureTenantID)),
					resource.TestCheckResourceAttr("echo.azure", "data.revoke_on_close", "true"),
					// Close runs at the end of the apply walk, before checks execute, so
					// by now the lease must already be gone.
					testAccCheckLeaseRevoked("echo.azure", true),
				),
			},
			// Step 3: the fixtures alone again, so no plan walk opens the ephemeral resource,
			// and every lease still open is revoked first. Terraform's destroy follows this
			// step, and a dynamic secret with a live lease cannot be destroyed.
			{
				PreConfig: func() { env.sweepLeases(t, dynamicSecretName) },
				Config:    env.azureSetupConfig(integrationName, dynamicSecretName, appObjectID),
			},
		},
	})
}

// revoke_on_close = false is the escape hatch for handing a credential to something
// that must keep using it after the apply. If it stopped suppressing the revoke, that
// credential would be dead on arrival with nothing to indicate why.
func TestAccAzureDynamicSecretEphemeral_revokeOnCloseDisabled(t *testing.T) {
	preCheckGrantedAzure(t)

	env := setupGrantEnv(t)
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	appObjectID := os.Getenv(acctest.EnvTestAzureAppObjectID)

	// resource.Test, not ParallelTest, deliberately; see the test above.
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
					registerDynamicSecretCleanup(t, dynamicSecretName, env.fixtureRoot)
				},
				Config: env.azureSetupConfig(integrationName, dynamicSecretName, appObjectID),
			},
			{
				PreConfig: func() {
					grantViaAdmin(t, dynamicSecretName+"-generate", env.generateCedar(dynamicSecretName))
					grantViaAdmin(t, dynamicSecretName+"-revoke", env.revokeCedar(dynamicSecretName))
				},
				Config: env.azureGenerateConfig(integrationName, dynamicSecretName, appObjectID, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					recordRan(t),
					resource.TestCheckResourceAttr("echo.azure", "data.revoke_on_close", "false"),
					testAccCheckLeaseRevoked("echo.azure", false),
				),
			},
			// Step 3: sweep the leases revoke_on_close = false deliberately left alive, one per
			// plan walk, so that the destroy that follows can succeed. See the test above.
			{
				PreConfig: func() { env.sweepLeases(t, dynamicSecretName) },
				Config:    env.azureSetupConfig(integrationName, dynamicSecretName, appObjectID),
			},
		},
	})
}

// Lease reads are only conclusive when they answer 200.
//
// A single probe cannot tell the two "unreadable" cases apart: the API answers 404 both
// for a lease that was revoked and for one whose create_lease job has not committed the
// row yet (the durability window revokeLease itself retries against, see
// dynamic_common.go), and it masks an invisible resource as 403 rather than 404. So both
// assertions below are built around waiting for an unambiguous answer instead of
// interpreting the first one that arrives.
const (
	// leasePresentTimeout bounds the wait for a lease that is expected to exist. It has
	// to outlast the durability window by a wide margin, since concluding "revoked" from
	// a lease that simply had not committed yet is the failure this replaces.
	leasePresentTimeout = 30 * time.Second

	// leaseAbsentSettle is how long a lease expected to be gone must stay unreadable
	// before absence is accepted as evidence of revocation. A revoke that silently failed
	// leaves a lease with the dynamic secret's full TTL on it, so it surfaces within this
	// window once the row commits.
	leaseAbsentSettle = 10 * time.Second

	leaseProbeInterval = time.Second
)

// leaseReadable reports whether the lease answers a read. Any status other than the two
// that mean "not visible" is returned as an error: a test that cannot read leases at all
// must say so rather than score the failure as a revocation.
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

// testAccCheckLeaseRevoked asserts whether the lease recorded by the echo resource has
// been destroyed.
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

// requireLeaseBecomesReadable waits for positive proof that the lease survived, which is
// the only conclusive outcome available: a 200. Timing out is reported as a failure to
// verify rather than as a revocation, because an unreadable lease is equally consistent
// with a backend that never committed the row or never served lease reads at all.
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
			return fmt.Errorf("lease %s was never readable within %s, so revoke_on_close = false "+
				"could not be confirmed to have preserved it. Either it was revoked anyway, or the "+
				"lease read endpoint is unavailable to this principal", leaseID, leasePresentTimeout)
		}
		time.Sleep(leaseProbeInterval)
	}
}

// requireLeaseStaysAbsent holds the absence assertion open for long enough that a lease
// which was merely uncommitted has time to appear. A revoke that failed leaves the lease
// alive for the whole TTL, so it shows up well inside the window.
//
// Absence is still the weaker of the two signals: a backend that serves no lease reads at
// all would satisfy this trivially. TestAccAzureDynamicSecretEphemeral_revokeOnCloseDisabled
// is what rules that out — it fails outright when lease reads never succeed — so the two
// tests are only meaningful as a pair.
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

// revokeCedar grants the principal RevokeLease, ReadLease and ListLeases on one dynamic
// secret, which Owner on the fixture folder already includes; it is written all the same so
// that a run without a fixture root has the permissions the checks and the sweep need spelled
// out. Without RevokeLease, Close only warns while the password lives to its TTL.
func (e *grantEnv) revokeCedar(dynamicSecretName string) string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action in [
    WorkloadCredentials::Action::"RevokeLease",
    WorkloadCredentials::Action::"ReadLease",
    WorkloadCredentials::Action::"ListLeases"
  ],
  resource == WorkloadCredentials::DynamicSecret::%q
);
`, e.siteID, e.principal, e.dsPath(dynamicSecretName))
}

// azureIntegrationRequest is the body that creates the Azure integration fixture, from the
// same environment the Azure precheck requires.
func (e *grantEnv) azureIntegrationRequest() resources.AzureIntegrationCreateRequest {
	return resources.AzureIntegrationCreateRequest{
		TenantID:     os.Getenv(acctest.EnvTestAzureTenantID),
		ClientID:     os.Getenv(acctest.EnvTestAzureClientID),
		ClientSecret: os.Getenv(acctest.EnvTestAzureClientSecret),
	}
}

// azureSetupConfig declares the dynamic secret fixture. The integration it names already
// exists; see the AWS test's file header.
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
