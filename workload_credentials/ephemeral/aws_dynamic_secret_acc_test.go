//go:build acceptance
// +build acceptance

package ephemeral_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
)

// Generation is gated on can_generate_dynamic_credential, which resolves through the
// operator role. The identity these tests run as does not hold it — creating a dynamic
// secret makes it the owner, and ownership deliberately does not confer generating from it —
// so the tests grant it to themselves rather than assuming the environment already has.
//
// Writing that grant needs the admin site, where the IAM policy API lives, so the suite
// needs an admin-audience token alongside the product-site one it runs on. Fixtures are
// created by the default provider, the identity every other product-site test uses, because
// it can already create integrations and dynamic secrets.
//
// Only the generate grant is written here. The create permissions are not, because that
// identity demonstrably has them: the integration and dynamic secret resource tests create
// both, at the product root, in the same job.

// grantEnv holds the identities and fixtures a granted-generation test needs.
type grantEnv struct {
	adminProvider string
	principal     string
	siteID        string
	roleArn       string
	targetRoleArn string
}

func setupGrantEnv(t *testing.T) *grantEnv {
	t.Helper()

	adminCfg, err := acctest.LoadAdminTestConfig()
	if err != nil {
		t.Fatalf("loading admin config: %v", err)
	}

	return &grantEnv{
		// Aliased, so the default provider stays the product-site one configured from the
		// environment — the same configuration every other test in this job runs on.
		adminProvider: adminCfg.AliasedProviderConfig("platform"),
		principal:     os.Getenv(acctest.EnvTestGeneratePrincipal),
		siteID:        acctest.PolicyTargetSiteID(),
		roleArn:       os.Getenv(acctest.EnvTestAWSRoleARN),
		targetRoleArn: os.Getenv(acctest.EnvTestAWSTargetRoleARN),
	}
}

func TestAccAwsDynamicSecretEphemeral_generatesWithGrant(t *testing.T) {
	preCheckGrantedAWS(t)

	env := setupGrantEnv(t)
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()

	registerIntegrationCleanup(t, "aws", integrationName)
	registerDynamicSecretCleanup(t, dynamicSecretName, "")

	// resource.Test, not ParallelTest, deliberately.
	//
	// Each of these configures two identities — the admin that authors the grants and the
	// principal that uses them — and every provider configuration costs an OIDC exchange.
	// Run in parallel, the three tests produced transient 403s on the admin's policy reads
	// and 401 "OIDC workload exchange denied" on the principal's writes, varying between
	// tests within a single run while the same code succeeded elsewhere in it. Serialising
	// trades a little wall clock for a suite whose failures mean something.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckGrantedAWS(t) },
		ProtoV6ProviderFactories: ephemeralProviderFactories(),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			// Step 1: the fixtures and the grants, with no ephemeral resource present. Both
			// must exist before anything generates: an ephemeral resource is opened while the
			// plan is built, so a plan holding all of it would call generate before the secret
			// and the grant it depends on exist.
			{
				Config: env.setupConfig(integrationName, dynamicSecretName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "name", dynamicSecretName),
					// The provider waits for a grant to bind and reports one that has not as a
					// warning rather than an error, so the status is the only thing separating
					// a live grant from an inert one.
					resource.TestCheckResourceAttr("beyondtrust_iam_policy.generate", "status", "ACTIVE"),
				),
			},
			// Step 2: generate, as the principal the grant names.
			{
				Config: env.generateConfig(integrationName, dynamicSecretName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// STS session keys carry an ASIA prefix where long-lived IAM keys carry
					// AKIA, so this fails if the integration's own credentials came back
					// instead of an assumed session.
					resource.TestMatchResourceAttr("echo.aws", "data.access_key_id", regexp.MustCompile(`^ASIA`)),
					resource.TestCheckResourceAttrSet("echo.aws", "data.secret_access_key"),
					resource.TestCheckResourceAttrSet("echo.aws", "data.session_token"),
					resource.TestCheckResourceAttrSet("echo.aws", "data.lease_id"),
					resource.TestMatchResourceAttr("echo.aws", "data.expiration", regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`)),
				),
			},
		},
	})
}

// setupConfig creates the fixtures and grants the identity permission to generate.
func (e *grantEnv) setupConfig(integrationName, dynamicSecretName string) string {
	return e.adminProvider + fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name     = %[1]q
  role_arn = %[3]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600
}

# Creating a dynamic secret makes this identity its owner, which deliberately does NOT
# confer generation: that resolves through operator, not owner. So this grant is load
# bearing, and the test would fail without it even though the caller owns the secret.
#
# Written against the admin site, where the IAM policy API lives, hence the alias.
resource "beyondtrust_iam_policy" "generate" {
  provider = beyondtrust.platform
  name     = "tf-acc-%[2]s-generate"

  cedar = <<-EOT
    @siteId(%[5]q)
    permit(
      principal == %[6]s,
      action == WorkloadCredentials::Action::"GenerateDynamicCredential",
      resource == WorkloadCredentials::DynamicSecret::"/${beyondtrust_workload_credentials_aws_dynamic_secret.test.path}"
    );
  EOT
}
`, integrationName, dynamicSecretName, e.roleArn, e.targetRoleArn, e.siteID, e.principal)
}

// generateConfig adds the ephemeral resource, which runs as the default provider — the
// identity the grant above names.
func (e *grantEnv) generateConfig(integrationName, dynamicSecretName string) string {
	return e.setupConfig(integrationName, dynamicSecretName) + `
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name = beyondtrust_workload_credentials_aws_dynamic_secret.test.name
}

provider "echo" {
  data = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.test
}

resource "echo" "aws" {}
`
}
