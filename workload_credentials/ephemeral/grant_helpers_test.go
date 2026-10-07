//go:build acceptance
// +build acceptance

package ephemeral_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
)

// The dynamic credential test grants itself GenerateDynamicCredential, since its identity
// holds only Owner on the fixture folder and Owner does not include generation.
//
// It generates from a dynamic secret that already exists rather than creating one. AWS
// credentials cannot be revoked early, and a dynamic secret cannot be deleted while it has
// active leases, so a secret created during the run could not be cleaned up afterwards.
// docs/TESTING_AWS.md describes the setup.

// grantEnv holds what the granted-generation test needs to know about its environment.
type grantEnv struct {
	principal   string
	siteID      string
	fixtureRoot string
}

func setupGrantEnv(t *testing.T) *grantEnv {
	t.Helper()

	admin, err := acctest.NewAdminTestClient()
	if err != nil {
		t.Fatalf("admin client: %v", err)
	}
	principal, err := acctest.ResolveGeneratePrincipal(context.Background(), admin)
	if err != nil {
		t.Fatalf("resolving the principal the grant names: %v", err)
	}

	return &grantEnv{
		principal:   principal,
		siteID:      acctest.PolicyTargetSiteID(),
		fixtureRoot: acctest.PolicyFixtureRoot(),
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
