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

// The dynamic credential tests grant themselves GenerateDynamicCredential, since their
// identity holds only Owner on the fixture folder and Owner does not include generation.
//
// The AWS test generates from a dynamic secret that already exists rather than creating one.
// AWS credentials cannot be revoked early, and a dynamic secret cannot be deleted while it has
// active leases, so a secret created during the run could not be cleaned up afterwards.
// docs/TESTING_AWS.md describes the setup.
//
// Azure credentials can be revoked, so the Azure tests create their fixtures per run and
// revoke any leases left open before Terraform destroys the dynamic secret. The integration is
// created through the API rather than by Terraform, because a grant on it only takes effect
// once the integration exists. The dynamic secret is created by Terraform inside the fixture
// folder.

// grantEnv holds what the granted-generation tests need to know about their environment.
type grantEnv struct {
	principal   string
	siteID      string
	fixtureRoot string
	// owner is a client for the identity the provider runs as.
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

// revokeCedar grants the principal what Close and the lease checks need on one dynamic secret.
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

// createIntegrationCedar grants the principal CreateIntegration. Integrations live at the
// product root, so the grant is scoped to the product.
func (e *grantEnv) createIntegrationCedar() string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == WorkloadCredentials::Action::"CreateIntegration",
  resource is WorkloadCredentials::Product
);
`, e.siteID, e.principal)
}

// ownIntegrationCedar makes the principal Owner of one integration. An integration's path in
// a policy is its bare name.
func (e *grantEnv) ownIntegrationCedar(integrationName string) string {
	return fmt.Sprintf(`@siteId(%q)
permit(
  principal == %s,
  action == WorkloadCredentials::Action::"Owner",
  resource == WorkloadCredentials::Integration::%q
);
`, e.siteID, e.principal, integrationName)
}

// createFixtureIntegration creates an integration as the provider identity and makes that
// identity its Owner.
//
// Its cleanup is registered after the Owner grant's, so it runs first: deleting the
// integration needs the grant still in effect.
func (e *grantEnv) createFixtureIntegration(t *testing.T, provider, integrationName string, body any) {
	t.Helper()

	grantViaAdmin(t, integrationName+"-create", e.createIntegrationCedar())

	path := e.owner.BuildPath("/integrations/" + provider + "/" + integrationName)
	if err := e.owner.Post(context.Background(), path, nil, body, nil); err != nil {
		t.Fatalf("creating %s integration %q: %v", provider, integrationName, err)
	}

	grantViaAdmin(t, integrationName+"-own", e.ownIntegrationCedar(integrationName))

	// Unlike registerCleanup, a 403 here is a leak worth failing on, not "already gone".
	t.Cleanup(func() {
		if t.Skipped() {
			return
		}
		err := e.owner.Delete(context.Background(), path, nil)
		var apiErr *btclient.APIError
		if err == nil || (errors.As(err, &apiErr) && apiErr.IsGone()) {
			return
		}
		t.Errorf("Cleanup: %s integration %q was not deleted: %v", provider, integrationName, err)
	})
}

// leasePage is the lease list response, reduced to what a sweep needs.
type leasePage struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// sweepLeases revokes every open lease on a dynamic secret and reports how many there were.
// A dynamic secret with an open lease cannot be destroyed.
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

// requireSwept is sweepLeases for a PreConfig.
func (e *grantEnv) requireSwept(t *testing.T, dynamicSecretName string) {
	t.Helper()
	n, err := e.sweepLeases(dynamicSecretName)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("swept %d lease(s) on %q", n, dynamicSecretName)
}

// registerFixtureSecretCleanup registers the dynamic secret's cleanup, preceded by a lease
// sweep in case the test failed before its own sweep ran.
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
