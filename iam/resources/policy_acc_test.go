//go:build acceptance
// +build acceptance

package resources_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

const policyResourceName = "beyondtrust_iam_policy.test"

// testAccPolicyConfig renders a provider block pointed at the admin site plus a policy whose
// @siteId annotation names the target site. Those are two different sites: the IAM API is hosted
// on the admin site, while the policy grants access on the target site.
func testAccPolicyConfig(cfg *acctest.TestConfig, name, siteID, principal, action, folder string) string {
	return cfg.ProviderConfig() + fmt.Sprintf(`
resource "beyondtrust_iam_policy" "test" {
  name  = %[1]q
  cedar = <<-EOT
    @siteId(%[2]q)
    permit(
      principal == Pathfinder::User::Email::%[3]q,
      action == %[6]s::Action::%[4]q,
      resource == %[6]s::Folder::%[5]q
    );
  EOT
}
`, name, siteID, principal, action, folder, vocabulary().namespace)
}

// checkStatusHealthy asserts the policy settled somewhere that means the write succeeded.
//
// Deliberately not an exact ACTIVE match. This suite exercises Terraform behaviour — create,
// update, replace, import, drift — and must not depend on whether the policy's target resource
// happens to exist in the tenant: a grant naming a resource that is not there parks in
// WAITING_FOR_RESOURCE, which is a documented success, not a failure. TestAccPolicyBinding_*
// covers whether the grant actually takes effect.
func checkStatusHealthy(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("%s not found in state", resourceName)
		}
		switch got := rs.Primary.Attributes["status"]; got {
		case "ACTIVE", "WAITING_FOR_RESOURCE":
			return nil
		default:
			return fmt.Errorf("policy settled in unhealthy state %q", got)
		}
	}
}

func TestAccPolicyResource_basic(t *testing.T) {
	env := preCheckPolicyEnv(t)
	name := acctest.RandomResourceName("policy")
	registerPolicyCleanup(t, name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckPolicyEnv(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig(env.cfg, name, env.siteID, env.principal, vocabulary().folderAction, env.folder),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(policyResourceName, "name", name),
					resource.TestCheckResourceAttr(policyResourceName, "site", env.siteID),
					checkStatusHealthy(policyResourceName),
					resource.TestCheckResourceAttrSet(policyResourceName, "product"),
					resource.TestCheckResourceAttrSet(policyResourceName, "created_at"),
					resource.TestCheckResourceAttrSet(policyResourceName, "updated_at"),
				),
			},
		},
	})
}

// TestAccPolicyResource_noDrift is the important one: it proves the Cedar text round-trips
// byte for byte and that none of the server-derived computed attributes (notably created_at,
// which the service regenerates on every write) produce a perpetual diff.
func TestAccPolicyResource_noDrift(t *testing.T) {
	env := preCheckPolicyEnv(t)
	name := acctest.RandomResourceName("policy")
	registerPolicyCleanup(t, name)

	config := testAccPolicyConfig(env.cfg, name, env.siteID, env.principal, vocabulary().folderAction, env.folder)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckPolicyEnv(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{Config: config},
			{
				Config:   config,
				PlanOnly: true,
				// A non-empty plan here means the provider is not writing back what the service
				// returned, and every apply would show spurious changes.
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccPolicyResource_updateCedar(t *testing.T) {
	env := preCheckPolicyEnv(t)
	name := acctest.RandomResourceName("policy")
	registerPolicyCleanup(t, name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckPolicyEnv(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig(env.cfg, name, env.siteID, env.principal, vocabulary().folderAction, env.folder),
				Check:  checkStatusHealthy(policyResourceName),
			},
			{
				// Changing the action is an in-place replace (PUT), not a destroy/create.
				Config: testAccPolicyConfig(env.cfg, name, env.siteID, env.principal, vocabulary().folderAltAction, env.folder),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(policyResourceName, "name", name),
					checkStatusHealthy(policyResourceName),
					resource.TestMatchResourceAttr(policyResourceName, "cedar", regexpUpdatedAction()),
				),
			},
		},
	})
}

func TestAccPolicyResource_nameImmutable(t *testing.T) {
	env := preCheckPolicyEnv(t)
	name := acctest.RandomResourceName("policy")
	renamed := acctest.RandomResourceName("policy")
	registerPolicyCleanup(t, name)
	registerPolicyCleanup(t, renamed)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckPolicyEnv(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig(env.cfg, name, env.siteID, env.principal, vocabulary().folderAction, env.folder),
				Check:  resource.TestCheckResourceAttr(policyResourceName, "name", name),
			},
			{
				Config: testAccPolicyConfig(env.cfg, renamed, env.siteID, env.principal, vocabulary().folderAction, env.folder),
				Check:  resource.TestCheckResourceAttr(policyResourceName, "name", renamed),
			},
		},
	})
}

func TestAccPolicyResource_import(t *testing.T) {
	env := preCheckPolicyEnv(t)
	name := acctest.RandomResourceName("policy")
	registerPolicyCleanup(t, name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckPolicyEnv(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig(env.cfg, name, env.siteID, env.principal, vocabulary().folderAction, env.folder),
			},
			{
				ResourceName:      policyResourceName,
				ImportState:       true,
				ImportStateId:     name,
				ImportStateVerify: true,
				// The policy's identity is its name — there is no `id` attribute, which is what
				// ImportStateVerify compares by default.
				ImportStateVerifyIdentifierAttribute: "name",
				// timeouts is config-only and never returned by the API.
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}

func TestAccPolicyResource_invalidCedarFailsAtPlan(t *testing.T) {
	env := preCheckPolicyEnv(t)
	name := acctest.RandomResourceName("policy")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckPolicyEnv(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// No @siteId annotation — ValidateConfig should reject this before any API call.
				Config: env.cfg.ProviderConfig() + fmt.Sprintf(`
resource "beyondtrust_iam_policy" "test" {
  name  = %[1]q
  cedar = <<-EOT
    permit(
      principal == Pathfinder::User::Email::%[2]q,
      action == WorkloadCreds::Action::"owner",
      resource == WorkloadCreds::Folder::%[3]q
    );
  EOT
}
`, name, env.principal, env.folder),
				ExpectError: regexpMissingSiteID(),
			},
		},
	})
}

// testAccCheckPolicyDestroy verifies the policy is gone or on its way out.
//
// Deletion is asynchronous: the service marks the policy PENDING_UNBIND and keeps serving it (then
// a short-lived DELETED tombstone) until the downstream pipeline removes the row, so a plain 404
// assertion would be flaky.
func testAccCheckPolicyDestroy(s *terraform.State) error {
	c, err := newAdminTestClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_iam_policy" {
			continue
		}
		name := rs.Primary.Attributes["name"]

		pol, err := getPolicyForTest(c, name)
		if err != nil {
			if isNotFoundForTest(err) || isPermissionErrorForTest(err) {
				continue
			}
			return fmt.Errorf("unexpected error checking policy %q was destroyed: %w", name, err)
		}

		switch pol.Status {
		case "PENDING_UNBIND", "DELETED":
			// Unbind is in flight — acceptable.
		default:
			return fmt.Errorf("policy %q still exists with status %s", name, pol.Status)
		}
	}
	return nil
}
