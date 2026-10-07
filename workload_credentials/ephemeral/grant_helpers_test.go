//go:build acceptance
// +build acceptance

package ephemeral_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	btclient "github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// --- fixtures and grants shared by the dynamic credential tests -----------------------------
//
// Generation is gated on can_generate_dynamic_credential, which resolves through the
// operator role. The identity these tests run as does not hold it, and neither owning a
// dynamic secret nor having created one confers it, so the tests grant it to themselves
// rather than assuming the environment already has.
//
// Writing that grant needs the admin site, where the IAM policy API lives, so the suite runs
// in the admin-site job, the only environment the admin identity federates from (see
// main_test.go). The default provider there is the product-site identity that seeds the IAM
// binding tests' fixtures: a workload identity with no product permissions beyond Owner on one
// folder, BEYONDTRUST_TEST_POLICY_FIXTURE_ROOT. The fixtures are arranged around that, and
// around one more fact: a dynamic secret cannot be destroyed while a lease references it.
//
// Dynamic secrets live inside that folder, so Owner cascades to them: create, read, destroy,
// and listing, reading and revoking their leases. Generating is not part of Owner, which is
// the whole point.
//
// Azure leases are revocable, so the Azure tests create their dynamic secret per run and
// sweep its leases before Terraform destroys it. AWS leases are not: an STS session cannot be
// recalled, so its lease row stays until the TTL expires and the secret cannot be destroyed
// until then. The AWS test therefore generates from a dynamic secret that outlives the run,
// named by BEYONDTRUST_TEST_AWS_DYNAMIC_SECRET and created once, like the fixture folder.
//
// Integrations sit outside the folder tree, so the Azure tests create theirs up front through
// the API as the provider identity, after a product-scoped CreateIntegration grant, and make
// the identity its Owner by path. Both grants bind at once, because the product and, by then,
// the integration exist. A grant on a path that does not exist yet binds only after the
// resource is created, by a background pipeline, which is why Terraform does not create the
// integration here: its first refresh would race that pipeline. The integration resources
// have their own tests in the product-site job.
//
// Every grant is written through the admin API before the step that needs it and removed when
// the test ends. Without a fixture root the Azure dynamic secret goes to the product root,
// which needs an identity already allowed to manage secrets there, such as a product admin
// authenticating with a personal access token locally.

// grantEnv holds the identities and fixture settings a granted-generation test needs.
type grantEnv struct {
	principal   string
	siteID      string
	fixtureRoot string
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
		principal:   principal,
		siteID:      acctest.PolicyTargetSiteID(),
		fixtureRoot: acctest.PolicyFixtureRoot(),
		owner:       owner,
	}
}

// dsPath is a dynamic secret's Cedar path: under the fixture root when there is one.
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
// read the integration back and delete it. An integration's Cedar path is its bare name: the
// RBAC guide's "/aws/prod" example never bound (the policy sat in WAITING_FOR_RESOURCE), while
// every integration policy that has bound in this org names the integration alone.
func (e *grantEnv) ownIntegrationCedar(integrationName string) string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Integration::%q
);
`, e.siteID, e.principal, integrationName)
}

// createFixtureIntegration creates the integration a dynamic secret fixture needs, as the
// provider identity, and makes that identity its Owner. See the header above for why this is
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

	grantViaAdmin(t, integrationName+"-own", e.ownIntegrationCedar(integrationName))

	// Not registerCleanup: that treats a 403 as "already gone", which for a fixture created
	// outside Terraform would hide exactly the leak this suite has produced before.
	t.Cleanup(func() {
		if t.Skipped() {
			return
		}
		err := e.owner.Delete(context.Background(), path, nil)
		var apiErr *btclient.APIError
		if err == nil || (errors.As(err, &apiErr) && apiErr.IsGone()) {
			return
		}
		t.Errorf("Cleanup: %s integration %q was not deleted and is leaked: %v", provider, integrationName, err)
	})
}

// leasePage is the shape of GET /leases/{name}, reduced to what a sweep needs.
type leasePage struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// sweepLeases revokes every lease still open on a dynamic secret and reports how many there
// were. A dynamic secret with a live lease cannot be destroyed; revoke_on_close = false leaves
// one per plan walk by design, and a Close that failed leaves one by accident.
func (e *grantEnv) sweepLeases(dynamicSecretName string) (int, error) {
	ctx := context.Background()

	var page leasePage
	if err := e.owner.Get(ctx, e.owner.BuildPath("/leases/"+dynamicSecretName), folderQuery(e.fixtureRoot), &page); err != nil {
		return 0, fmt.Errorf("listing leases on %q: %w", dynamicSecretName, err)
	}
	for _, lease := range page.Data {
		err := e.owner.Delete(ctx, e.owner.BuildPath("/leases/id/"+lease.ID), nil)
		var apiErr *btclient.APIError
		if err != nil && !(errors.As(err, &apiErr) && apiErr.IsGone()) {
			return 0, fmt.Errorf("revoking lease %s on %q: %w", lease.ID, dynamicSecretName, err)
		}
	}
	return len(page.Data), nil
}

// requireSwept is sweepLeases for a PreConfig: Terraform destroys the secret after the step.
func (e *grantEnv) requireSwept(t *testing.T, dynamicSecretName string) {
	t.Helper()
	n, err := e.sweepLeases(dynamicSecretName)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("swept %d lease(s) on %q", n, dynamicSecretName)
}

// registerFixtureSecretCleanup registers the dynamic secret's safety-net deletion, preceded
// by a lease sweep. A step that fails before the sweeping step runs leaves the leases earlier
// plan walks opened, and the deletion would otherwise fail on them exactly as Terraform's
// destroy did, leaking the secret and the integration it holds in use.
func (e *grantEnv) registerFixtureSecretCleanup(t *testing.T, dynamicSecretName string) {
	t.Helper()
	registerDynamicSecretCleanup(t, dynamicSecretName, e.fixtureRoot)
	t.Cleanup(func() {
		if t.Skipped() {
			return
		}
		if _, err := e.sweepLeases(dynamicSecretName); err != nil {
			t.Errorf("Cleanup: %v", err)
		}
	})
}
