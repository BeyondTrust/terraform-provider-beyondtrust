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

// The AWS test generates from a dynamic secret that outlives the run; grant_helpers_test.go
// says why. Nothing is created or destroyed here: the test grants itself generation on that
// secret, opens the ephemeral resource, and checks that what came back is an STS session.
func TestAccAwsDynamicSecretEphemeral_generatesWithGrant(t *testing.T) {
	preCheckGrantedAWS(t)

	env := setupGrantEnv(t)
	dynamicSecretName := os.Getenv(acctest.EnvTestAWSDynamicSecret)

	// resource.Test, not ParallelTest: tests that write grants are more reliable run serially.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckGrantedAWS(t) },
		ProtoV6ProviderFactories: ephemeralProviderFactories(),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		Steps: []resource.TestStep{
			// The grant must be active before the plan that opens the ephemeral resource; see
			// main_test.go. The policy name is random because the secret is shared between runs.
			{
				PreConfig: func() {
					grantViaAdmin(t, acctest.RandomResourceName("tf-acc-generate"), env.generateCedar(dynamicSecretName))
				},
				Config: env.awsGenerateConfig(dynamicSecretName),
				Check: resource.ComposeAggregateTestCheckFunc(
					recordRan(t),
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

// awsGenerateConfig opens the ephemeral resource against the persistent dynamic secret and
// echoes the result so it can be asserted on.
func (e *grantEnv) awsGenerateConfig(dynamicSecretName string) string {
	return fmt.Sprintf(`
ephemeral "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name = %[1]q
%[2]s}

provider "echo" {
  data = ephemeral.beyondtrust_workload_credentials_aws_dynamic_secret.test
}

resource "echo" "aws" {}
`, dynamicSecretName, e.folderAttr())
}
