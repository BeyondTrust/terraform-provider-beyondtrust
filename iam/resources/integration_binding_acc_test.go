//go:build acceptance

package resources_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

// Integration names are unique per provider, not per site, so an AWS and an Azure integration can
// share one. A policy tells them apart by naming the integration as "<provider>/<name>".
//
// The test seeds a same-named AWS and Azure integration and grants the low-privilege principal
// DeleteIntegration. Only an integration's owner holds that by default, so the baseline is a
// reliable 403 on both.

const (
	providerAWS   = "aws"
	providerAzure = "azure"

	actionCreateIntegration = "CreateIntegration"
	actionDeleteIntegration = "DeleteIntegration"

	integrationGrant = "beyondtrust_iam_policy.grant"
)

type integrationBindingEnv struct {
	adminCfg  *acctest.TestConfig
	admin     *client.Client
	owner     *client.Client
	principal *client.Client

	targetSite      string
	principalEntity string
	ownerEntity     string

	name      string
	grantName string

	// live tracks which integrations exist, so cleanup only waits on those.
	live map[string]bool
}

// TestAccIntegrationPolicyBinding_providerScoped asserts a policy on an integration reaches only
// the integration it names.
func TestAccIntegrationPolicyBinding_providerScoped(t *testing.T) {
	env := setupIntegrationBinding(t)
	ctx := context.Background()

	for _, provider := range []string{providerAWS, providerAzure} {
		status, err := deleteIntegration(ctx, env.principal, provider, env.name)
		if status == http.StatusNoContent {
			env.live[provider] = false
		}
		requireDenied(t, status, err, "baseline delete of the "+provider+" integration")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{
				// The bare name matches no integration, so the grant is accepted but never binds.
				// TODO: assert a 400 here once the policy service rejects bare integration names.
				Config: env.providerBlock() + env.deletePolicyHCL(env.name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(integrationGrant, "status", "WAITING_FOR_RESOURCE"),
					env.checkNotDeletable(providerAWS),
					env.checkNotDeletable(providerAzure),
				),
			},
			{
				Config: env.providerBlock() + env.deletePolicyHCL(providerAWS+"/"+env.name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(integrationGrant, "status", "ACTIVE"),
					env.checkDeletable(t, providerAWS),
					// Checked once the AWS delete has gone through, so the grant is known to be
					// in effect rather than not yet applied.
					env.checkNotDeletable(providerAzure),
				),
			},
		},
	})
}

// setupIntegrationBinding seeds the two integrations.
//
// The fixture owner holds no product-level permissions, so it is granted CreateIntegration for
// the run. Creating an integration does not make it the owner, so it is also granted
// DeleteIntegration on both paths for cleanup. Those two are written before the integrations
// exist and bind once they are created.
func setupIntegrationBinding(t *testing.T) *integrationBindingEnv {
	t.Helper()
	if reason := acctest.PolicyBindingSkipReason(); reason != "" {
		recordSkip(t, reason)
	}
	if reason := acctest.AWSSkipReason(); reason != "" {
		recordSkip(t, reason)
	}
	if reason := acctest.AzureSkipReason(); reason != "" {
		recordSkip(t, reason)
	}
	ownerCfg, err := acctest.LoadPolicyOwnerTestConfig()
	if err != nil {
		t.Fatalf("loading fixture owner config: %v", err)
	}
	if ownerCfg.ServiceName == "" {
		recordSkip(t, fmt.Sprintf("the fixture owner needs a service name to be granted to (set %s)",
			acctest.EnvTestPolicyOwnerServiceName))
	}
	recordRan(t)

	ctx := context.Background()
	adminCfg, err := acctest.LoadAdminTestConfig()
	if err != nil {
		t.Fatalf("loading admin test config: %v", err)
	}
	admin, err := acctest.NewClientForConfig(adminCfg)
	if err != nil {
		t.Fatalf("building admin client: %v", err)
	}
	owner, err := acctest.NewClientForConfig(ownerCfg)
	if err != nil {
		t.Fatalf("building fixture owner client: %v", err)
	}
	principal, err := acctest.NewPrincipalTestClient()
	if err != nil {
		t.Fatalf("building principal client: %v", err)
	}
	ownerEntity, err := acctest.ResolveWorkloadPrincipal(ctx, admin, ownerCfg.ServiceName)
	if err != nil {
		t.Fatalf("resolving the fixture owner's principal: %v", err)
	}

	env := &integrationBindingEnv{
		adminCfg:        adminCfg,
		admin:           admin,
		owner:           owner,
		principal:       principal,
		targetSite:      acctest.PolicyTargetSiteID(),
		principalEntity: os.Getenv(acctest.EnvTestPolicyPrincipal),
		ownerEntity:     ownerEntity,
		name:            acctest.RandomIntegrationName(),
		grantName:       acctest.RandomResourceName("grant"),
		live:            map[string]bool{},
	}
	if env.targetSite == "" {
		t.Fatalf("no target site: set BEYONDTRUST_SITE_ID (or %s)", acctest.EnvTestPolicySiteID)
	}
	registerPolicyCleanup(t, env.grantName)

	createGrant := acctest.RandomResourceName("create-integration")
	env.putGrant(t, createGrant, env.productCedar(actionCreateIntegration))
	for _, provider := range []string{providerAWS, providerAzure} {
		env.putGrant(t, acctest.RandomResourceName("cleanup-"+provider),
			env.integrationCedar(env.ownerEntity, actionDeleteIntegration, provider+"/"+env.name))
	}
	env.waitActive(t, createGrant)

	// Registered after the grants, so it runs before they are removed.
	t.Cleanup(func() { env.deleteFixtures(t) })

	bodies := map[string]any{
		providerAWS: map[string]string{"roleArn": acctest.GetAWSRoleARN(t)},
		providerAzure: map[string]string{
			"tenantId":     acctest.GetAzureTenantID(t),
			"clientId":     acctest.GetAzureClientID(t),
			"clientSecret": acctest.GetAzureClientSecret(t),
		},
	}
	for _, provider := range []string{providerAWS, providerAzure} {
		if err := createIntegrationWithRetry(ctx, owner, provider, env.name, bodies[provider]); err != nil {
			t.Fatalf("seeding %s integration %q: %v", provider, env.name, err)
		}
		env.live[provider] = true
	}

	return env
}

func (e *integrationBindingEnv) providerBlock() string {
	return e.adminCfg.ProviderConfig()
}

// deletePolicyHCL grants the principal DeleteIntegration on one integration path.
func (e *integrationBindingEnv) deletePolicyHCL(path string) string {
	return fmt.Sprintf(`
resource "beyondtrust_iam_policy" "grant" {
  name  = %[1]q
  cedar = <<-EOT
    @siteId(%[2]q)
    permit(
      principal == %[3]s,
      action == %[4]s::Action::%[5]q,
      resource == %[4]s::Integration::%[6]q
    );
  EOT
}
`, e.grantName, e.targetSite, e.principalEntity, cedarNamespace, actionDeleteIntegration, path)
}

func (e *integrationBindingEnv) integrationCedar(principal, action, path string) string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == %s::Action::%q,
  resource == %s::Integration::%q
);`, e.targetSite, principal, cedarNamespace, action, cedarNamespace, path)
}

// productCedar grants the fixture owner a product-level action.
func (e *integrationBindingEnv) productCedar(action string) string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == %s::Action::%q,
  resource is %s::Product
);`, e.targetSite, e.ownerEntity, cedarNamespace, action, cedarNamespace)
}

// putGrant writes a policy through the admin client and removes it when the test ends.
func (e *integrationBindingEnv) putGrant(t *testing.T, name, cedar string) {
	t.Helper()
	body := client.RawBody{ContentType: "text/plain", Data: []byte(cedar)}
	if err := e.admin.Put(context.Background(), e.admin.BuildIAMPath("/policies/"+name), nil, body, nil); err != nil {
		t.Fatalf("writing grant %q: %v", name, err)
	}
	registerPolicyCleanup(t, name)
}

func (e *integrationBindingEnv) waitActive(t *testing.T, name string) {
	t.Helper()
	err := waitForBinding(t, "grant "+name, func() (bool, error) {
		pol, err := getPolicyForTest(e.admin, name)
		if err != nil {
			return false, err
		}
		return pol.Status == "ACTIVE", nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// createIntegrationWithRetry retries while the provider's credentials are still propagating,
// as the integration resources do.
func createIntegrationWithRetry(ctx context.Context, c *client.Client, provider, name string, body any) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if err = createIntegration(ctx, c, provider, name, body); err == nil {
			return nil
		}
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) ||
			!(apiErr.IsAWSCredentialValidationError() || apiErr.IsAzureCredentialValidationError()) {
			return err
		}
		time.Sleep(time.Duration(5*attempt) * time.Second)
	}
	return err
}

// deleteFixtures removes the integrations that still exist. A 403 here can mean the cleanup
// grant has not bound yet, so each delete is retried until the bind timeout.
func (e *integrationBindingEnv) deleteFixtures(t *testing.T) {
	for _, provider := range []string{providerAWS, providerAzure} {
		if !e.live[provider] {
			continue
		}
		err := waitForBinding(t, "cleanup of the "+provider+" integration", func() (bool, error) {
			status, err := deleteIntegration(context.Background(), e.owner, provider, e.name)
			return statusSettled(status, http.StatusNoContent, err)
		})
		if err != nil {
			t.Logf("cleanup: %s integration %q not deleted (leaked): %v", provider, e.name, err)
		}
	}
}

// checkDeletable polls until the principal's delete of the integration succeeds.
func (e *integrationBindingEnv) checkDeletable(t *testing.T, provider string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		err := waitForBinding(t, "delete of the "+provider+" integration", func() (bool, error) {
			status, err := deleteIntegration(context.Background(), e.principal, provider, e.name)
			return statusSettled(status, http.StatusNoContent, err)
		})
		if err == nil {
			e.live[provider] = false
		}
		return err
	}
}

func (e *integrationBindingEnv) checkNotDeletable(provider string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		status, err := deleteIntegration(context.Background(), e.principal, provider, e.name)
		if status == 0 {
			return err
		}
		if status == http.StatusNoContent {
			e.live[provider] = false
		}
		if status != http.StatusForbidden {
			return fmt.Errorf("expected the principal's delete of the %s integration %q to be refused (403), "+
				"got %d; a grant meant for another integration reached it", provider, e.name, status)
		}
		return nil
	}
}
