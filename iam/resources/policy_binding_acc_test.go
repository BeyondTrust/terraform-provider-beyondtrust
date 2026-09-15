//go:build acceptance

package resources_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

// These tests prove the thing the resource exists for: that granting a Cedar policy actually
// changes what the Workload Credentials API returns for a specific user.
//
// Each test walks three rungs:
//
//	1. baseline  — the principal is denied (403). Asserted, never assumed: a test whose
//	               baseline is already open reports success no matter what the policy does.
//	2. grant     — Terraform creates the policy against the ADMIN site.
//	3. verify    — the principal's probe flips, observed against the PRODUCT site.
//
// The credential split is forced by the platform: tokens carry a single audience and are scoped
// to one site, so the admin-site policy author, the product-site owner that seeds fixtures, and
// the low-privilege principal are three separate identities.
//
// Verification happens in Go rather than HCL on purpose. Under the acceptance-test harness a
// single provider server instance is shared across every provider block, and its configure data
// is overwritten by whichever block configures last — so two aliased blocks would silently share
// one token. Holding the clients directly keeps the identities honest and lets assertions read
// the exact HTTP status.

// Binding is asynchronous: the policy service accepts the write, then a separate pipeline
// applies it. Sixty seconds is typical; the ceiling is a deliberate fail-fast rather than a
// generous wait.
const (
	defaultBindTimeout  = 3 * time.Minute
	defaultBindInterval = 5 * time.Second

	envBindTimeoutSeconds = "BEYONDTRUST_TEST_POLICY_BIND_TIMEOUT"
)

const (
	secretAlpha   = "alpha"
	secretBravo   = "bravo"
	secretCharlie = "charlie"
)

// envPolicySchema selects which generation of the WLC Cedar vocabulary to write.
//
// The schema is mid-migration: the namespace becomes WorkloadCredentials and action ids become
// PascalCase display names. Until that ships, the
// deployed schema uses the WorkloadCreds namespace with snake_case relation names — and has no
// list gate at all, which is why the scoping test cannot run against it.
//
// Default to what is deployed so the suite works today, and set this to "current" once the new
// schema rolls out. Neither the provider nor the resource cares: action names are validated
// server-side precisely so this kind of change does not require a provider release.
const envPolicySchema = "BEYONDTRUST_TEST_POLICY_SCHEMA"

// wlcVocab is the Cedar vocabulary for one schema generation. listAction is empty when the
// schema has no grantable list gate.
type wlcVocab struct {
	namespace    string
	listAction   string
	readAction   string
	updateAction string
	// Actions valid on a Folder, used by the resource lifecycle tests, which only need Cedar the
	// service will accept — they assert on Terraform behaviour, not on enforcement.
	folderAction    string
	folderAltAction string
}

func vocabulary() wlcVocab {
	if strings.EqualFold(os.Getenv(envPolicySchema), "current") {
		return wlcVocab{
			namespace:       "WorkloadCredentials",
			listAction:      "ListFolderContents",
			readAction:      "ReadSecret",
			updateAction:    "UpdateSecret",
			folderAction:    "Owner",
			folderAltAction: "ReadFolderMetadata",
		}
	}
	return wlcVocab{
		namespace:       "WorkloadCreds",
		listAction:      "", // no can_list_folder_contents in the deployed schema
		readAction:      "can_read_secret",
		updateAction:    "can_update_secret",
		folderAction:    "owner",
		folderAltAction: "can_read_folder_metadata",
	}
}

// policyBindingEnv holds the three identities and the fixture names for one test run.
type policyBindingEnv struct {
	adminCfg  *acctest.TestConfig
	owner     *client.Client // product-site owner: seeds and cleans up fixtures
	principal *client.Client // low-privilege subject under test

	targetSite     string // goes in the policy's @siteId annotation
	principalEmail string
	folder         string
	gateName       string
	grantName      string
}

// setupPolicyBinding checks credentials, builds the three clients, and seeds a folder holding
// three secrets. Cleanup is registered before anything is created, so a mid-test failure still
// tears the fixtures down.
func setupPolicyBinding(t *testing.T) *policyBindingEnv {
	t.Helper()
	if reason := acctest.PolicyBindingSkipReason(); reason != "" {
		recordSkip(t, reason)
	}
	recordRan(t)

	adminCfg, err := acctest.LoadAdminTestConfig()
	if err != nil {
		t.Fatalf("loading admin test config: %v", err)
	}
	owner, err := acctest.NewTestClient()
	if err != nil {
		t.Fatalf("building product-site owner client: %v", err)
	}
	principal, err := acctest.NewPrincipalTestClient()
	if err != nil {
		t.Fatalf("building principal client: %v", err)
	}

	env := &policyBindingEnv{
		adminCfg:       adminCfg,
		owner:          owner,
		principal:      principal,
		targetSite:     acctest.PolicyTargetSiteID(),
		principalEmail: os.Getenv(acctest.EnvTestPolicyPrincipalEmail),
		folder:         acctest.RandomFolderName(),
		gateName:       acctest.RandomResourceName("gate"),
		grantName:      acctest.RandomResourceName("grant"),
	}
	if env.targetSite == "" {
		t.Fatalf("no target site: set BEYONDTRUST_SITE_ID (or %s)", acctest.EnvTestPolicySiteID)
	}

	ctx := context.Background()
	registerPolicyCleanup(t, env.gateName)
	registerPolicyCleanup(t, env.grantName)
	t.Cleanup(func() {
		if err := deleteFolderRecursive(context.Background(), env.owner, env.folder); err != nil {
			t.Logf("cleanup: folder %q not deleted (leaked): %v", env.folder, err)
		}
	})

	if err := createFolder(ctx, env.owner, env.folder, ""); err != nil {
		t.Fatalf("seeding folder %q: %v", env.folder, err)
	}
	for _, name := range []string{secretAlpha, secretBravo, secretCharlie} {
		if err := createSecret(ctx, env.owner, name, env.folder, "tf-acc-"+name); err != nil {
			t.Fatalf("seeding secret %q in %q: %v", name, env.folder, err)
		}
	}

	return env
}

// gatePolicyHCL grants only the list gate on the folder.
//
// ListFolderContents is deliberate: granting it directly does NOT confer `lister`, so it opens
// the list endpoint without cascading read access to the folder's children. Granting Lister or
// Viewer here instead would grant access to all three secrets and the scoping assertion would
// pass while proving nothing.
func (e *policyBindingEnv) gatePolicyHCL() string {
	v := vocabulary()
	return fmt.Sprintf(`
resource "beyondtrust_iam_policy" "gate" {
  name  = %[1]q
  cedar = <<-EOT
    @siteId(%[2]q)
    permit(
      principal == Pathfinder::User::Email::%[3]q,
      action == %[4]s::Action::%[5]q,
      resource == %[4]s::Folder::%[6]q
    );
  EOT
}
`, e.gateName, e.targetSite, e.principalEmail, v.namespace, v.listAction, "/"+e.folder)
}

// secretPolicyHCL grants one action on one secret.
func (e *policyBindingEnv) secretPolicyHCL(action, secret string) string {
	v := vocabulary()
	return fmt.Sprintf(`
resource "beyondtrust_iam_policy" "grant" {
  name  = %[1]q
  cedar = <<-EOT
    @siteId(%[2]q)
    permit(
      principal == Pathfinder::User::Email::%[3]q,
      action == %[4]s::Action::%[5]q,
      resource == %[4]s::Secret::%[6]q
    );
  EOT
}
`, e.grantName, e.targetSite, e.principalEmail, v.namespace, action, "/"+e.folder+"/"+secret)
}

func (e *policyBindingEnv) providerBlock() string {
	return e.adminCfg.ProviderConfig()
}

// TestAccPolicyBinding_listScoping is the test that proves the feature: with a list gate on the
// folder and a read grant on one secret, listing returns that secret and nothing else — even
// though the folder holds three.
func TestAccPolicyBinding_listScoping(t *testing.T) {
	v := vocabulary()
	if v.listAction == "" {
		recordSkip(t, fmt.Sprintf("the deployed Cedar schema has no grantable list gate; set "+
			"%s=current once the newer schema is deployed", envPolicySchema))
	}

	env := setupPolicyBinding(t)
	ctx := context.Background()

	_, status, err := listStaticSecrets(ctx, env.principal, env.folder)
	requireDenied(t, status, err, "baseline list of "+env.folder)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock() + env.gatePolicyHCL() + env.secretPolicyHCL(v.readAction, secretAlpha),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_iam_policy.gate", "status", "ACTIVE"),
					resource.TestCheckResourceAttr("beyondtrust_iam_policy.grant", "status", "ACTIVE"),
					env.checkListReturnsOnly(t, secretAlpha),
					env.checkSecretReadable(t, secretAlpha),
					env.checkSecretNotReadable(t, secretBravo),
				),
			},
		},
	})
}

// TestAccPolicyBinding_updateFlip asserts an owner-gated action flips from denied to allowed.
//
// This is the more robust of the two signals. can_update_secret is owner-level, so no ordinary
// role confers it — unlike read, which some product roles grant blanket. If the list-scoping
// test ever goes ambiguous because of a role model change, this one still holds.
func TestAccPolicyBinding_updateFlip(t *testing.T) {
	v := vocabulary()
	env := setupPolicyBinding(t)
	ctx := context.Background()

	status, err := updateSecret(ctx, env.principal, secretAlpha, env.folder, "baseline")
	requireDenied(t, status, err, "baseline update of "+secretAlpha)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock() + env.secretPolicyHCL(v.updateAction, secretAlpha),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_iam_policy.grant", "status", "ACTIVE"),
					env.checkSecretUpdatable(t, secretAlpha),
				),
			},
		},
	})
}

// TestAccPolicyBinding_revoked asserts destroying the policy actually withdraws the grant.
// Nothing else in the suite proves an unbind happens.
func TestAccPolicyBinding_revoked(t *testing.T) {
	v := vocabulary()
	env := setupPolicyBinding(t)
	ctx := context.Background()

	status, err := updateSecret(ctx, env.principal, secretAlpha, env.folder, "baseline")
	requireDenied(t, status, err, "baseline update of "+secretAlpha)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock() + env.secretPolicyHCL(v.updateAction, secretAlpha),
				Check:  env.checkSecretUpdatable(t, secretAlpha),
			},
			{
				// Removing the resource from the config destroys the policy. The grant should
				// unbind and the principal should be refused again.
				Config: env.providerBlock(),
				Check:  env.checkSecretNotUpdatable(t, secretAlpha),
			},
		},
	})
}

// --- checks ---

// checkListReturnsOnly polls until the list endpoint admits the principal, then asserts the
// response contains exactly the named secret.
func (e *policyBindingEnv) checkListReturnsOnly(t *testing.T, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		ctx := context.Background()

		var last *wlcListResponse
		err := waitForBinding(t, "list of "+e.folder, func() (bool, error) {
			resp, status, err := listStaticSecrets(ctx, e.principal, e.folder)
			switch status {
			case http.StatusOK:
				last = resp
				return true, nil
			case http.StatusForbidden:
				return false, nil // gate not bound yet
			case http.StatusUnauthorized:
				return false, fmt.Errorf("principal token rejected (401, likely expired)")
			default:
				return false, fmt.Errorf("unexpected status %d: %v", status, err)
			}
		})
		if err != nil {
			return err
		}

		names := secretBaseNames(last.Data)
		if len(names) != 1 || !contains(names, want) {
			return fmt.Errorf(
				"expected the list to contain exactly [%s], got %s.\n\n"+
					"The folder holds %d secrets and only %q was granted, so a longer list means the "+
					"grant cascaded. Check that the gate policy grants ListFolderContents (not Lister "+
					"or Viewer) — those confer access to every child.",
				want, describeItems(last.Data), 3, want)
		}
		return nil
	}
}

func (e *policyBindingEnv) checkSecretReadable(t *testing.T, name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		ctx := context.Background()
		return waitForBinding(t, "read of "+name, func() (bool, error) {
			status, err := readSecret(ctx, e.principal, name, e.folder)
			return statusSettled(status, http.StatusOK, err)
		})
	}
}

func (e *policyBindingEnv) checkSecretNotReadable(t *testing.T, name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		status, err := readSecret(context.Background(), e.principal, name, e.folder)
		if err != nil {
			return err
		}
		if status != http.StatusForbidden {
			return fmt.Errorf(
				"expected %q to stay denied (403), got %d — the grant on another secret leaked to it",
				name, status)
		}
		return nil
	}
}

func (e *policyBindingEnv) checkSecretUpdatable(t *testing.T, name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		ctx := context.Background()
		return waitForBinding(t, "update of "+name, func() (bool, error) {
			status, err := updateSecret(ctx, e.principal, name, e.folder, "bound")
			return statusSettled(status, http.StatusNoContent, err)
		})
	}
}

// checkSecretNotUpdatable polls for the access to go away, since unbinding is asynchronous too.
func (e *policyBindingEnv) checkSecretNotUpdatable(t *testing.T, name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		ctx := context.Background()
		return waitForBinding(t, "revocation of update on "+name, func() (bool, error) {
			status, err := updateSecret(ctx, e.principal, name, e.folder, "revoked")
			switch status {
			case http.StatusForbidden:
				return true, nil
			case http.StatusOK, http.StatusNoContent:
				return false, nil // still bound
			case http.StatusUnauthorized:
				return false, fmt.Errorf("principal token rejected (401, likely expired)")
			default:
				return false, fmt.Errorf("unexpected status %d: %v", status, err)
			}
		})
	}
}

// statusSettled maps a probe result onto the (done, err) contract waitForBinding expects.
func statusSettled(status, want int, err error) (bool, error) {
	switch status {
	case want:
		return true, nil
	case http.StatusForbidden:
		return false, nil // not bound yet
	case http.StatusUnauthorized:
		return false, fmt.Errorf("principal token rejected (401, likely expired)")
	default:
		return false, fmt.Errorf("unexpected status %d: %v", status, err)
	}
}

// waitForBinding polls until done, the deadline passes, or the probe reports a fatal error.
//
// A timeout is a hard failure with a specific diagnosis, not a flake: the policy was accepted,
// so if the permission never took effect the service did not finish applying the grant.
func waitForBinding(t *testing.T, what string, probe func() (bool, error)) error {
	t.Helper()

	timeout := defaultBindTimeout
	if raw := os.Getenv(envBindTimeoutSeconds); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		done, err := probe()
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		if done {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(
				"%s did not take effect within %s. The policy was accepted, so the service did not "+
					"finish applying the grant (most likely it is parked in WAITING_FOR_RESOURCE). "+
					"Raise %s to wait longer.",
				what, timeout, envBindTimeoutSeconds)
		}
		t.Logf("waiting for %s to take effect...", what)
		time.Sleep(defaultBindInterval)
	}
}
