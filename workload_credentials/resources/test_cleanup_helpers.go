//go:build acceptance
// +build acceptance

package resources_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
)

// registerAwsIntegrationCleanup registers a cleanup function that attempts to delete
// an AWS integration via API if Terraform's destroy somehow failed. This is a safety
// net for test failures, panics, or bugs in the provider's destroy logic.
func registerAwsIntegrationCleanup(t *testing.T, name string) {
	t.Cleanup(func() {
		if t.Skipped() {
			return
		}

		client, err := acctest.NewTestClient()
		if err != nil {
			t.Logf("Cleanup: failed to create client: %v", err)
			return
		}

		apiPath := client.BuildPath(fmt.Sprintf("/integrations/%s", name))
		err = client.Delete(context.Background(), apiPath, nil)

		if err == nil {
			t.Logf("WARNING: Cleanup deleted AWS integration %s (Terraform destroy didn't work)", name)
			return
		}

		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			// Expected - already deleted by Terraform
			return
		}

		t.Logf("Cleanup: unexpected error deleting AWS integration %s: %v", name, err)
	})
}

// registerAwsDynamicSecretCleanup registers a cleanup function that attempts to delete
// an AWS dynamic secret via API if Terraform's destroy somehow failed.
func registerAwsDynamicSecretCleanup(t *testing.T, path string) {
	t.Cleanup(func() {
		if t.Skipped() {
			return
		}

		client, err := acctest.NewTestClient()
		if err != nil {
			t.Logf("Cleanup: failed to create client: %v", err)
			return
		}

		apiPath := client.BuildPath(fmt.Sprintf("/dynamic/%s", path))
		err = client.Delete(context.Background(), apiPath, nil)

		if err == nil {
			t.Logf("WARNING: Cleanup deleted AWS dynamic secret %s (Terraform destroy didn't work)", path)
			return
		}

		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			// Expected - already deleted by Terraform
			return
		}

		t.Logf("Cleanup: unexpected error deleting AWS dynamic secret %s: %v", path, err)
	})
}

// registerFolderCleanup registers a cleanup function that attempts to delete
// a folder via API if Terraform's destroy somehow failed.
func registerFolderCleanup(t *testing.T, name, folder string) {
	t.Cleanup(func() {
		if t.Skipped() {
			return
		}

		client, err := acctest.NewTestClient()
		if err != nil {
			t.Logf("Cleanup: failed to create client: %v", err)
			return
		}

		apiPath := client.BuildPath(fmt.Sprintf("/folders/%s", name))
		query := url.Values{}
		if folder != "" {
			query.Set("folder", folder)
		}

		err = client.Delete(context.Background(), apiPath, query)

		if err == nil {
			t.Logf("WARNING: Cleanup deleted folder %s (Terraform destroy didn't work)", name)
			return
		}

		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			// Expected - already deleted by Terraform
			return
		}

		t.Logf("Cleanup: unexpected error deleting folder %s: %v", name, err)
	})
}

// registerStaticSecretCleanup registers a cleanup function that attempts to delete
// a static secret via API if Terraform's destroy somehow failed.
func registerStaticSecretCleanup(t *testing.T, name, folder string) {
	t.Cleanup(func() {
		if t.Skipped() {
			return
		}

		client, err := acctest.NewTestClient()
		if err != nil {
			t.Logf("Cleanup: failed to create client: %v", err)
			return
		}

		apiPath := client.BuildPath(fmt.Sprintf("/static/%s", name))
		query := url.Values{}
		if folder != "" {
			query.Set("folder", folder)
		}

		err = client.Delete(context.Background(), apiPath, query)

		if err == nil {
			t.Logf("WARNING: Cleanup deleted static secret %s (Terraform destroy didn't work)", name)
			return
		}

		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			// Expected - already deleted by Terraform
			return
		}

		t.Logf("Cleanup: unexpected error deleting static secret %s: %v", name, err)
	})
}
