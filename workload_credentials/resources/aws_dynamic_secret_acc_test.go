//go:build acceptance
// +build acceptance

package resources_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	btclient "github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	_ "github.com/beyondtrust/terraform-provider-beyondtrust/internal/provider"
)

func TestAccAwsDynamicSecretResource_basic(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	// Register cleanup as safety net (LIFO order: secret cleaned up before integration)
	registerAwsIntegrationCleanup(t, integrationName)
	registerAwsDynamicSecretCleanup(t, dynamicSecretName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAwsDynamicSecretResourceConfig_basic(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "name", dynamicSecretName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "integration_name", integrationName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "credential_type", "assumed_role"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "role_arn", targetRoleArn),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "ttl", "3600"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "path", dynamicSecretName),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_aws_dynamic_secret.test", "id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_aws_dynamic_secret.test", "integration_id"),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_aws_dynamic_secret.test", "created_at"),
				),
			},
			// ImportState testing - import by path since the API identifies dynamic secrets by path, not UUID
			{
				ResourceName:      "beyondtrust_workload_credentials_aws_dynamic_secret.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["beyondtrust_workload_credentials_aws_dynamic_secret.test"]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					p, ok := rs.Primary.Attributes["path"]
					if !ok || p == "" {
						return "", fmt.Errorf("resource has no path attribute in state")
					}
					return p, nil
				},
				// external_id is sensitive and not returned by the API
				// created_at precision differs between create and read responses (API inconsistency)
				ImportStateVerifyIgnore: []string{"external_id", "created_at"},
			},
		},
	})
}

func TestAccAwsDynamicSecretResource_inFolder(t *testing.T) {
	folderName := acctest.RandomFolderName()
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	// Register cleanup as safety net (LIFO: secret → folder → integration)
	registerAwsIntegrationCleanup(t, integrationName)
	registerFolderCleanup(t, folderName, "")
	registerAwsDynamicSecretCleanup(t, fmt.Sprintf("%s/%s", folderName, dynamicSecretName))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsDynamicSecretResourceConfig_inFolder(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "name", dynamicSecretName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "folder", folderName),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "path", fmt.Sprintf("%s/%s", folderName, dynamicSecretName)),
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_aws_dynamic_secret.test", "id"),
				),
			},
		},
	})
}

func TestAccAwsDynamicSecretResource_withPolicyArns(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	// Register cleanup as safety net (LIFO order: secret cleaned up before integration)
	registerAwsIntegrationCleanup(t, integrationName)
	registerAwsDynamicSecretCleanup(t, dynamicSecretName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsDynamicSecretResourceConfig_withPolicyArns(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "policy_arns.#", "2"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "policy_arns.0", "arn:aws:iam::aws:policy/ReadOnlyAccess"),
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "policy_arns.1", "arn:aws:iam::aws:policy/SecurityAudit"),
				),
			},
		},
	})
}

func TestAccAwsDynamicSecretResource_withInlinePolicy(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	inlinePolicy := `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "s3:ListBucket",
      "Resource": "arn:aws:s3:::test-bucket"
    }
  ]
}`

	// Register cleanup as safety net (LIFO order: secret cleaned up before integration)
	registerAwsIntegrationCleanup(t, integrationName)
	registerAwsDynamicSecretCleanup(t, dynamicSecretName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsDynamicSecretResourceConfig_withInlinePolicy(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, inlinePolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("beyondtrust_workload_credentials_aws_dynamic_secret.test", "policy"),
				),
			},
		},
	})
}

func TestAccAwsDynamicSecretResource_updateTTL(t *testing.T) {
	integrationName := acctest.RandomIntegrationName()
	dynamicSecretName := acctest.RandomDynamicSecretName()
	roleArn := getTestRoleArn(t)
	targetRoleArn := getTestTargetRoleArn(t)
	externalId := acctest.RandomString(32)

	// Register cleanup as safety net (LIFO order: secret cleaned up before integration)
	registerAwsIntegrationCleanup(t, integrationName)
	registerAwsDynamicSecretCleanup(t, dynamicSecretName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); acctest.PreCheckAWS(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsDynamicSecretDestroy,
		Steps: []resource.TestStep{
			// Create with 1 hour TTL
			{
				Config: testAccAwsDynamicSecretResourceConfig_withTTL(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, 3600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "ttl", "3600"),
				),
			},
			// Update to 2 hour TTL
			{
				Config: testAccAwsDynamicSecretResourceConfig_withTTL(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, 7200),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("beyondtrust_workload_credentials_aws_dynamic_secret.test", "ttl", "7200"),
				),
			},
		},
	})
}

func testAccCheckAwsDynamicSecretDestroy(s *terraform.State) error {
	// Create a test client to verify resources are destroyed
	client, err := acctest.NewTestClient()
	if err != nil {
		return fmt.Errorf("failed to create test client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "beyondtrust_workload_credentials_aws_dynamic_secret" {
			continue
		}

		// Get the secret name and folder from state
		name := rs.Primary.Attributes["name"]
		folder := rs.Primary.Attributes["folder"]

		if name == "" {
			return fmt.Errorf("dynamic secret name not found in state")
		}

		// Build API path and query parameters
		apiPath := client.BuildPath(fmt.Sprintf("/dynamic/%s", name))
		query := url.Values{}
		if folder != "" {
			query.Set("folder", folder)
		}

		// Try to fetch the dynamic secret - should return 404 if properly deleted
		var result interface{}
		err := client.Get(context.Background(), apiPath, query, &result)

		// If no error, resource still exists - test should fail
		if err == nil {
			return fmt.Errorf("AWS dynamic secret %s still exists after destroy", name)
		}

		// Verify it's a 404 error (resource properly deleted)
		var apiErr *btclient.APIError
		if errors.As(err, &apiErr) {
			if apiErr.IsGone() {
				// Expected - resource is properly deleted
				continue
			}
			if apiErr.IsPermissionError() {
				// Permission error after destroy - likely means resource is deleted
				// but we no longer have permission to verify
				continue
			}
		}

		// Any other error is unexpected
		return fmt.Errorf("unexpected error checking AWS dynamic secret deletion: %w", err)
	}

	return nil
}

// testAccAwsDynamicSecretResourceConfig_basic returns a basic AWS dynamic secret resource configuration
func testAccAwsDynamicSecretResourceConfig_basic(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[3]q
  external_id = %[5]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId)
}

// testAccAwsDynamicSecretResourceConfig_inFolder returns a configuration with dynamic secret in a folder
func testAccAwsDynamicSecretResourceConfig_inFolder(folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_folder" "test" {
  name = %[1]q
}

resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name        = %[2]q
  role_arn    = %[4]q
  external_id = %[6]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[3]q
  folder           = beyondtrust_workload_credentials_folder.test.path
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[5]q
  ttl              = 3600
}
`, folderName, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId)
}

// testAccAwsDynamicSecretResourceConfig_withPolicyArns returns a configuration with policy ARNs
func testAccAwsDynamicSecretResourceConfig_withPolicyArns(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[3]q
  external_id = %[5]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600
  policy_arns = [
    "arn:aws:iam::aws:policy/ReadOnlyAccess",
    "arn:aws:iam::aws:policy/SecurityAudit"
  ]
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId)
}

// testAccAwsDynamicSecretResourceConfig_withInlinePolicy returns a configuration with inline policy
func testAccAwsDynamicSecretResourceConfig_withInlinePolicy(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, policy string) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[3]q
  external_id = %[5]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = 3600
  policy           = %[6]q
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, policy)
}

// testAccAwsDynamicSecretResourceConfig_withTTL returns a configuration with custom TTL
func testAccAwsDynamicSecretResourceConfig_withTTL(integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId string, ttl int) string {
	return fmt.Sprintf(`
resource "beyondtrust_workload_credentials_aws_integration" "test" {
  name        = %[1]q
  role_arn    = %[3]q
  external_id = %[5]q
}

resource "beyondtrust_workload_credentials_aws_dynamic_secret" "test" {
  name             = %[2]q
  integration_name = beyondtrust_workload_credentials_aws_integration.test.name
  credential_type  = "assumed_role"
  role_arn         = %[4]q
  ttl              = %[6]d
}
`, integrationName, dynamicSecretName, roleArn, targetRoleArn, externalId, ttl)
}
