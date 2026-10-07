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
	btclient "github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider" // Import to trigger init()
	"github.com/beyondtrust/terraform-provider-beyondtrust/workload_credentials/resources"
)

// Generation is gated on can_generate_dynamic_credential, which resolves through the
// operator role. The identity these tests run as does not hold it, and neither owning a
// dynamic secret nor having created one confers it, so the tests grant it to themselves
// rather than assuming the environment already has.
//
// Writing that grant needs the admin site, where the IAM policy API lives, so the suite runs
// in the admin-site job, the only environment the admin identity federates from (see
// main_test.go). The default provider there is the product-site identity that seeds the IAM
// binding tests' fixtures: a workload identity with no product permissions beyond Owner on one
// folder, BEYONDTRUST_TEST_POLICY_FIXTURE_ROOT. The fixtures are arranged around that.
//
// The dynamic secret is created inside that folder, so Owner cascades to it: create, read,
// destroy, and reading and revoking its leases. Generating is not part of Owner, which is the
// whole point.
//
// Integrations sit outside the folder tree, so the integration is created up front through
// the API as the provider identity, after a product-scoped CreateIntegration grant, and the
// identity is then made its Owner by path. Both grants bind at once, because the product and,
// by then, the integration exist. A grant on a path that does not exist yet binds only after
// the resource is created, by a background pipeline, which is why Terraform does not create
// the integration here: its first refresh would race that pipeline. The integration resource
// has its own tests in the product-site job.
//
// Every grant is written through the admin API before the step that needs it and removed when
// the test ends. Without a fixture root the dynamic secret goes to the product root, which
// needs an identity already allowed to manage secrets there, such as a product admin
// authenticating with a personal access token locally.

// grantEnv holds the identities and fixtures a granted-generation test needs.
type grantEnv struct {
	principal     string
	siteID        string
	fixtureRoot   string
	roleArn       string
	targetRoleArn string
	// owner is the product-site client for the identity the provider runs as.
	owner *btclient.Client
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
	owner, err := acctest.NewTestClient()
	if err != nil {
		t.Fatalf("product-site client: %v", err)
	}

	return &grantEnv{
		principal:     principal,
		siteID:        acctest.PolicyTargetSiteID(),
		fixtureRoot:   acctest.PolicyFixtureRoot(),
		roleArn:       os.Getenv(acctest.EnvTestAWSRoleARN),
		targetRoleArn: os.Getenv(acctest.EnvTestAWSTargetRoleARN),
		owner:         owner,
	}
}

// dsPath is the dynamic secret's Cedar path: under the fixture root when there is one.
func (e *grantEnv) dsPath(dynamicSecretName string) string {
	if e.fixtureRoot == "" {
		return "/" + dynamicSecretName
	}
	return "/" + e.fixtureRoot + "/" + dynamicSecretName
}

// folderAttr is the folder attribute line for a dynamic secret block, or nothing at the root.
func (e *grantEnv) folderAttr() string {
	if e.fixtureRoot == "" {
		return ""
	}
	return fmt.Sprintf("  folder = %q\n", e.fixtureRoot)
}

// generateCedar grants the principal GenerateDynamicCredential on one dynamic secret.
func (e *grantEnv) generateCedar(dynamicSecretName string) string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == WorkloadCredentials::Action::"GenerateDynamicCredential",
  resource == WorkloadCredentials::DynamicSecret::%q
);
`, e.siteID, e.principal, e.dsPath(dynamicSecretName))
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

// ownIntegrationCedar makes the principal Owner of one integration, which is how it gets to
// read the integration back and delete it. Integration paths are the provider then the name.
func (e *grantEnv) ownIntegrationCedar(provider, integrationName string) string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Integration::%q
);
`, e.siteID, e.principal, "/"+provider+"/"+integrationName)
}

// createFixtureIntegration creates the integration a dynamic secret fixture needs, as the
// provider identity, and makes that identity its Owner. See the file header for why this is
// not a Terraform resource in the test configuration.
//
// The integration's cleanup is registered after the Owner grant's, so that it runs first:
// deleting the integration needs the grant to still be in effect.
func (e *grantEnv) createFixtureIntegration(t *testing.T, provider, integrationName string, body any) {
	t.Helper()

	grantViaAdmin(t, integrationName+"-create", e.createIntegrationCedar())

	path := e.owner.BuildPath("/integrations/" + provider + "/" + integrationName)
	if err := e.owner.Post(context.Background(), path, nil, body, nil); err != nil {
		t.Fatalf("creating %s integration %q: %v", provider, integrationName, err)
	}

	grantViaAdmin(t, integrationName+"-own", e.ownIntegrationCedar(provider, integrationName))
	registerIntegrationCleanup(t, provider, integrationName)
}

func TestAccAwsDynamicSecretEphemeral_generatesWithGrant(t *testing.T) {
	preCheckGrantedAWS(t)

	env := setupGrantEnv(t)
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()

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
		CheckDestroy:             testAccCheckEphemeralFixturesDestroyed,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			// Step 1: the fixtures, with no ephemeral resource present. They must exist before
			// anything generates: an ephemeral resource is opened while the plan is built, so a
			// plan holding all of it would call generate before the secret and the grant it
			// depends on exist. The integration goes in first, through the API, for the
			// reasons in the file header. The dynamic secret's cleanup is registered after it
			// so that, if the test fails midway, the secret is removed before the integration
			// it references.
			{
				PreConfig: func() {
					env.createFixtureIntegration(t, "aws", integrationName, resources.AwsIntegrationCreateRequest{
						RoleArn: env.roleArn,
					})
					registerDynamicSecretCleanup(t, dynamicSecretName, env.fixtureRoot)
				},
				Config: env.setupConfig(integrationName, dynamicSecretName),
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

// setupConfig declares the dynamic secret fixture. The integration it names already exists;
// the grant is not a resource here either.
func (e *grantEnv) setupConfig(integrationName, dynamicSecretName string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[1]q
  integration_name = %[2]q
  credential_type  = "assumed_role"
  role_arn         = %[3]q
  ttl              = 3600
%[4]s}
`, dynamicSecretName, integrationName, e.targetRoleArn, e.folderAttr())
}

// generateConfig adds the ephemeral resource, which runs as the default provider — the
// identity the grant above names.
func (e *grantEnv) generateConfig(integrationName, dynamicSecretName string) string {
	return e.setupConfig(integrationName, dynamicSecretName) + fmt.Sprintf(`
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name = beyondtrust_workload_credentials_aws_dynamic_secret.test.name
%[1]s}

provider "echo" {
  data = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.test
}

resource "echo" "aws" {}
`, e.folderAttr())
}
