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
// That needs three identities in one configuration — the owner that seeds the dynamic
// secret, the admin that writes the policy, and the principal the policy names and that runs
// the ephemeral resource — so these run in the admin-site job, beside the IAM policy tests
// that need the same thing. In the product-site job they skip for want of admin credentials.
//
// The grant is setup rather than the subject here: what is under test is the ephemeral
// resource, exercised as an identity holding exactly the one permission it documents.

// grantEnv holds the identities a granted-generation test needs.
type grantEnv struct {
	principalProvider string
	adminProvider     string
	principal         string
	siteID            string
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
		// them, so everything the test does runs as the identity the grants name.
		principalProvider: principalCfg.ProviderConfig(),
		adminProvider:     adminCfg.AliasedProviderConfig("platform"),
		principal:         os.Getenv(acctest.EnvTestPolicyPrincipal),
		siteID:            acctest.PolicyTargetSiteID(),
		roleArn:           os.Getenv(acctest.EnvTestAWSRoleARN),
		targetRoleArn:     os.Getenv(acctest.EnvTestAWSTargetRoleARN),
	}
}

// productGrant renders a policy granting the principal a product-scoped action.
//
// Creating an integration is product-scoped and has no folder to be contained by, so it
// cannot be reached by a folder grant the way secrets can. Granting it here rather than
// relying on a standing policy keeps the suite self-describing: everything it needs is
// visible in the configuration it applies.
func (e *grantEnv) productGrant(label, action, name string) string {
	return fmt.Sprintf(`
resource "beyondtrust_iam_policy" %[1]q {
  provider = beyondtrust.platform
  name     = "tf-acc-%[4]s-%[1]s"

  cedar = <<-EOT
    @siteId(%[2]q)
    permit(
      principal == %[3]s,
      action == WorkloadCredentials::Action::%[5]q,
      resource == WorkloadCredentials::Product::%[2]q
    );
  EOT
}
`, label, e.siteID, e.principal, name, action)
}

func TestAccAwsDynamicSecretEphemeral_generatesWithGrant(t *testing.T) {
	preCheckGrantedAWS(t)

	env := setupGrantEnv(t)
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()

	registerIntegrationCleanup(t, "aws", integrationName)
	registerDynamicSecretCleanup(t, dynamicSecretName, "")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { preCheckGrantedAWS(t) },
		ProtoV6ProviderFactories: ephemeralProviderFactories(),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			// Step 1: the dynamic secret and the grant, with no ephemeral resource in the
			// configuration. Both must exist before anything generates: an ephemeral resource
			// is opened while the plan is built, so a plan holding all three would call
			// generate before either the secret or the policy it depends on exists.
			{
				Config: env.setupConfig(integrationName, dynamicSecretName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "name", dynamicSecretName),
					// The provider waits for the grant to bind, and reports a policy that has
					// not as a warning rather than an error — so the status is the only thing
					// that distinguishes a live grant from an inert one.
					resource.TestCheckResourceAttr("beyondtrust_iam_policy.generate", "status", "ACTIVE"),
				),
			},
			// Step 2: generate as the principal the grant names.
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

// setupConfig grants the principal everything it needs, then creates the fixtures as that
// principal. No ephemeral resource yet: it is opened while the plan is built, so a plan
// holding it too would call generate before the secret and the grant exist.
func (e *grantEnv) setupConfig(integrationName, dynamicSecretName string) string {
	return e.principalProvider + e.adminProvider +
		e.productGrant("create_integration", "CreateIntegration", dynamicSecretName) +
		e.productGrant("create_dynamic_secret", "CreateDynamicSecret", dynamicSecretName) +
		fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name     = %[1]q
  role_arn = %[3]q

  # The provider waits for a grant to bind before returning, so depending on the policy is
  # what makes this ordering real rather than hopeful.
  depends_on = [beyondtrust_iam_policy.create_integration]
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600

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
`, integrationName, dynamicSecretName, e.roleArn, e.targetRoleArn, e.siteID, e.principal)
}

// generateConfig adds the ephemeral resource, which runs as the default provider — the same
// principal every grant above names.
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
