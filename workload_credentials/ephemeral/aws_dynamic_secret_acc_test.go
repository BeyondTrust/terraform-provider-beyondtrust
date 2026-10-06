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
// operator role. Product admins hold it implicitly; CI does not, so these tests grant it to
// themselves rather than assuming the environment already has.
//
// That needs two identities in one configuration — the admin that writes the grant, and the
// principal the grant names, which creates the fixtures and generates from them — so these
// run in the admin-site job beside the IAM policy tests that need the same thing. In the
// product-site job they skip for want of admin credentials.
//
// What the suite cannot grant itself is can_create_integration. That action exists only at
// Product scope in the Cedar schema, and this caller is not permitted to write Product-scoped
// grants: attempting it fails with "forbidden: caller cannot perform this action" while
// Folder-scoped grants from the same identity succeed. So it is a standing grant on the test
// site, and the one prerequisite this file cannot describe in Terraform. Everything else is
// Folder- or resource-scoped and written below.

// grantEnv holds the identities and fixtures a granted-generation test needs.
type grantEnv struct {
	principalProvider string
	adminProvider     string
	principal         string
	siteID            string
	fixtureRoot       string
	roleArn           string
	targetRoleArn     string
}

func setupGrantEnv(t *testing.T) *grantEnv {
	t.Helper()

	adminCfg, err := acctest.LoadAdminTestConfig()
	if err != nil {
		t.Fatalf("loading admin config: %v", err)
	}
	principalCfg, err := acctest.LoadPrincipalTestConfig()
	if err != nil {
		t.Fatalf("loading principal config: %v", err)
	}

	return &grantEnv{
		// The principal is the default provider: it creates the fixtures and generates from
		// them, so everything under test runs as the identity the grants name.
		principalProvider: principalCfg.ProviderConfig(),
		adminProvider:     adminCfg.AliasedProviderConfig("platform"),
		principal:         os.Getenv(acctest.EnvTestPolicyPrincipal),
		siteID:            acctest.PolicyTargetSiteID(),
		fixtureRoot:       acctest.PolicyFixtureRoot(),
		roleArn:           os.Getenv(acctest.EnvTestAWSRoleARN),
		targetRoleArn:     os.Getenv(acctest.EnvTestAWSTargetRoleARN),
	}
}

// createSecretGrant lets the principal create a dynamic secret inside the fixture root.
//
// Folder-scoped rather than Product-scoped on purpose: CreateDynamicSecret accepts either,
// and only the Folder form can be written by this caller. It also keeps the grant narrow —
// the principal gains nothing outside the folder the suite already seeds into.
func (e *grantEnv) createSecretGrant(name string) string {
	return fmt.Sprintf(`
resource "beyondtrust_iam_policy" "create_dynamic_secret" {
  provider = beyondtrust.platform
  name     = "tf-acc-%[3]s-create-ds"

  cedar = <<-EOT
    @siteId(%[1]q)
    permit(
      principal == %[2]s,
      action == WorkloadCredentials::Action::"CreateDynamicSecret",
      resource == WorkloadCredentials::Folder::"/%[4]s"
    );
  EOT
}
`, e.siteID, e.principal, name, e.fixtureRoot)
}

func TestAccAwsDynamicSecretEphemeral_generatesWithGrant(t *testing.T) {
	preCheckGrantedAWS(t)

	env := setupGrantEnv(t)
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()

	registerIntegrationCleanup(t, "aws", integrationName)
	registerDynamicSecretCleanup(t, dynamicSecretName, env.fixtureRoot)

	resource.ParallelTest(t, resource.TestCase{
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

// setupConfig creates the fixtures as the principal and grants it what the API will ask for.
func (e *grantEnv) setupConfig(integrationName, dynamicSecretName string) string {
	return e.principalProvider + e.adminProvider + e.createSecretGrant(dynamicSecretName) + fmt.Sprintf(`
# No depends_on, and no grant written for this: can_create_integration is a standing grant
# on the test site, for the reason given at the top of this file.
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name     = %[1]q
  role_arn = %[3]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[2]q
  folder           = %[7]q
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600

  # The provider waits for a grant to bind before returning, so depending on the policy is
  # what makes this ordering real rather than hopeful.
  depends_on = [beyondtrust_iam_policy.create_dynamic_secret]
}

# Creating a dynamic secret makes the principal its owner, which deliberately does NOT
# confer generation: that resolves through operator, not owner. So this grant is load
# bearing, and the test would fail without it even though the principal owns the secret.
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
`, integrationName, dynamicSecretName, e.roleArn, e.targetRoleArn, e.siteID, e.principal, e.fixtureRoot)
}

// generateConfig adds the ephemeral resource, which runs as the default provider — the same
// principal every grant above names.
func (e *grantEnv) generateConfig(integrationName, dynamicSecretName string) string {
	return e.setupConfig(integrationName, dynamicSecretName) + `
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name   = beyondtrust_workload_credentials_aws_dynamic_secret.test.name
  folder = beyondtrust_workload_credentials_aws_dynamic_secret.test.folder
}

provider "echo" {
  data = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.test
}

resource "echo" "aws" {}
`
}
