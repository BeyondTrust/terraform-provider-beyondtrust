//go:build acceptance
// +build acceptance

package ephemeral_test

import (
	"context"
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
// runs in the admin-site job, the only environment the admin identity federates from (see
// main_test.go). The default provider there is the product-site identity that seeds the IAM
// binding tests' fixtures: a workload identity created with no product permissions. So the
// create permissions are granted here too, at Product scope, because an integration can
// only be created there and the dynamic secret sits beside it at the root. Every grant is
// written through the admin API before the step that needs it and removed when the test ends.

// grantEnv holds the identities and fixtures a granted-generation test needs.
type grantEnv struct {
	principal     string
	siteID        string
	roleArn       string
	targetRoleArn string
}

func setupGrantEnv(t *testing.T) *grantEnv {
	t.Helper()

	admin, err := acctest.NewAdminTestClient()
	if err != nil {
		t.Fatalf("admin client: %v", err)
	}
	principal, err := acctest.ResolveGeneratePrincipal(context.Background(), admin)
	if err != nil {
		t.Fatalf("resolving the principal the grants name: %v", err)
	}

	return &grantEnv{
		principal:     principal,
		siteID:        acctest.PolicyTargetSiteID(),
		roleArn:       os.Getenv(acctest.EnvTestAWSRoleARN),
		targetRoleArn: os.Getenv(acctest.EnvTestAWSTargetRoleARN),
	}
}

// generateCedar grants the principal GenerateDynamicCredential on one dynamic secret. The
// secret sits at the product root, so its path is its name.
func (e *grantEnv) generateCedar(dynamicSecretName string) string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == WorkloadCredentials::Action::"GenerateDynamicCredential",
  resource == WorkloadCredentials::DynamicSecret::"/%s"
);
`, e.siteID, e.principal, dynamicSecretName)
}

// createIntegrationCedar grants the principal CreateIntegration. Integrations live at the
// product root, so the only scope the action accepts is the Product itself.
func (e *grantEnv) createIntegrationCedar() string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == WorkloadCredentials::Action::"CreateIntegration",
  resource is WorkloadCredentials::Product
);
`, e.siteID, e.principal)
}

// createDynamicSecretCedar grants the principal CreateDynamicSecret at the product root,
// where the fixture is created so that its Cedar path is just its name.
func (e *grantEnv) createDynamicSecretCedar() string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == WorkloadCredentials::Action::"CreateDynamicSecret",
  resource is WorkloadCredentials::Product
);
`, e.siteID, e.principal)
}

// grantFixtureCreation writes the two grants the fixture step needs and waits for both to be
// ACTIVE. Named after the dynamic secret so concurrent runs cannot collide on policy names.
func (e *grantEnv) grantFixtureCreation(t *testing.T, dynamicSecretName string) {
	t.Helper()
	grantViaAdmin(t, dynamicSecretName+"-create-integration", e.createIntegrationCedar())
	grantViaAdmin(t, dynamicSecretName+"-create-dynamic-secret", e.createDynamicSecretCedar())
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
			// Step 1: the fixtures, with no ephemeral resource present. They must exist before
			// anything generates: an ephemeral resource is opened while the plan is built, so a
			// plan holding all of it would call generate before the secret and the grant it
			// depends on exist. The create grants go in first, for the identity explained above.
			{
				PreConfig: func() { env.grantFixtureCreation(t, dynamicSecretName) },
				Config:    env.setupConfig(integrationName, dynamicSecretName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "name", dynamicSecretName),
				),
			},
			// Step 2: grant generation, then generate. The grant is written through the admin
			// API and awaited ACTIVE here, before the plan that opens the ephemeral resource —
			// not as a resource in the configuration. See main_test.go for why there is no
			// second provider.
			{
				PreConfig: func() {
					grantViaAdmin(t, dynamicSecretName+"-generate", env.generateCedar(dynamicSecretName))
				},
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

// setupConfig creates the fixtures. Just those: the grant is not a resource here.
func (e *grantEnv) setupConfig(integrationName, dynamicSecretName string) string {
	return fmt.Sprintf(`
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
`, integrationName, dynamicSecretName, e.roleArn, e.targetRoleArn)
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
