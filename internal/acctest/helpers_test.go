package acctest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/client"
	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRandomGeneration validates random string and integer generation.
func TestRandomGeneration(t *testing.T) {
	t.Run("string generation", func(t *testing.T) {
		result := RandomString(10)
		assert.Equal(t, 10, len(result), "string length should match")

		// Verify all characters are lowercase letters or numbers
		for _, char := range result {
			assert.True(t,
				(char >= 'a' && char <= 'z') || (char >= '0' && char <= '9'),
				"character should be lowercase letter or number")
		}
	})

	t.Run("integer generation", func(t *testing.T) {
		// Run multiple times to verify range boundaries
		for i := 0; i < 10; i++ {
			result := RandomInt(1, 10)
			assert.GreaterOrEqual(t, result, 1, "result should be >= min")
			assert.LessOrEqual(t, result, 10, "result should be <= max")
		}
	})
}

// TestRandomNaming validates resource name generation patterns.
func TestRandomNaming(t *testing.T) {
	tests := []struct {
		name      string
		fn        func() string
		wantPfx   string
		suffixLen int
	}{
		{
			name:      "resource name",
			fn:        func() string { return RandomResourceName("folder") },
			wantPfx:   "tf-acc-test-folder-",
			suffixLen: 8,
		},
		{
			name:      "folder path",
			fn:        RandomFolderPath,
			wantPfx:   "tf-acc-test/",
			suffixLen: 8,
		},
		{
			name:      "folder name",
			fn:        RandomFolderName,
			wantPfx:   "tf-acc-test-",
			suffixLen: 8,
		},
		{
			name:      "secret name",
			fn:        RandomSecretName,
			wantPfx:   "tf-acc-test-secret-",
			suffixLen: 8,
		},
		{
			name:      "integration name",
			fn:        RandomIntegrationName,
			wantPfx:   "tf-acc-test-integration-",
			suffixLen: 8,
		},
		{
			name:      "dynamic secret name",
			fn:        RandomDynamicSecretName,
			wantPfx:   "tf-acc-test-dynamic-",
			suffixLen: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn()
			assert.True(t, strings.HasPrefix(result, tt.wantPfx), "should start with %s", tt.wantPfx)
			suffix := result[len(tt.wantPfx):]
			assert.Equal(t, tt.suffixLen, len(suffix), "suffix should be %d characters", tt.suffixLen)
		})
	}
}

// TestRandomAWS validates AWS-specific random generation.
func TestRandomAWS(t *testing.T) {
	t.Run("ARN generation", func(t *testing.T) {
		result := RandomARN("iam", "role")

		// Verify ARN format components
		assert.True(t, strings.HasPrefix(result, "arn:aws:"), "should start with arn:aws:")
		assert.Contains(t, result, "iam", "should contain service name")
		assert.Contains(t, result, "role", "should contain resource type")
		assert.Contains(t, result, "us-east-1", "should contain region")
		assert.Contains(t, result, "123456789012", "should contain account ID")
		assert.Contains(t, result, "tf-acc-test-", "should contain test prefix")
	})

	t.Run("role ARN", func(t *testing.T) {
		result := RandomRoleARN()
		assert.True(t, strings.HasPrefix(result, "arn:aws:iam:"), "should be an IAM ARN")
		assert.Contains(t, result, "role", "should contain 'role'")
	})

	t.Run("tags generation", func(t *testing.T) {
		result := RandomTags()

		// Verify expected keys exist
		assert.Contains(t, result, "Environment")
		assert.Contains(t, result, "ManagedBy")
		assert.Contains(t, result, "TestRun")

		// Verify expected values
		assert.Equal(t, "test", result["Environment"])
		assert.Equal(t, "terraform", result["ManagedBy"])
		assert.Equal(t, 8, len(result["TestRun"]), "TestRun should be 8 characters")
	})
}

// The edge keys an OIDC exchange on (URL site id, X-BT-Service-Name), so the admin client must
// be able to carry the admin identity's name even when the job's default names a product one.
// A silent regression here reappears as a generic 401 "no trust record" that took six CI runs
// to attribute, so the precedence is pinned.
func TestLoadAdminTestConfig_ServiceNamePrecedence(t *testing.T) {
	t.Setenv(EnvAdminSiteID, "admin-site")
	t.Setenv(EnvAdminAccessToken, "tok")

	t.Run("admin override wins over the job default", func(t *testing.T) {
		t.Setenv(constants.EnvServiceName, "product-identity")
		t.Setenv(EnvAdminServiceName, "admin-identity")
		cfg, err := LoadAdminTestConfig()
		require.NoError(t, err)
		assert.Equal(t, "admin-identity", cfg.ServiceName)
	})

	t.Run("falls back to the job default when unset", func(t *testing.T) {
		t.Setenv(constants.EnvServiceName, "admin-identity-in-admin-job")
		t.Setenv(EnvAdminServiceName, "")
		cfg, err := LoadAdminTestConfig()
		require.NoError(t, err)
		assert.Equal(t, "admin-identity-in-admin-job", cfg.ServiceName)
	})
}

func TestResolveGeneratePrincipal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/site/admin-site/platform/auth/workload-identities" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"issuers":[
			{"identityId":"11111111-1111-1111-1111-111111111111","serviceName":"Other-Identity"},
			{"identityId":"22222222-2222-2222-2222-222222222222","serviceName":"Fixture-Owner"}
		],"totalCount":2}`)
	}))
	defer srv.Close()

	admin, err := client.NewClient(&client.Config{
		BaseURL: srv.URL, SiteID: "admin-site", AccessToken: "tok", APIVersion: "2026-04-28", Timeout: "30s",
	})
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("explicit entity wins without a lookup", func(t *testing.T) {
		t.Setenv(EnvTestGeneratePrincipal, `Pathfinder::User::Email::"x@example.com"`)
		t.Setenv(constants.EnvServiceName, "fixture-owner")
		got, err := ResolveGeneratePrincipal(ctx, admin)
		require.NoError(t, err)
		assert.Equal(t, `Pathfinder::User::Email::"x@example.com"`, got)
	})

	t.Run("resolves the service name without regard to case", func(t *testing.T) {
		t.Setenv(EnvTestGeneratePrincipal, "")
		t.Setenv(constants.EnvServiceName, "fixture-owner")
		got, err := ResolveGeneratePrincipal(ctx, admin)
		require.NoError(t, err)
		assert.Equal(t, `Pathfinder::Workload::Id::"22222222-2222-2222-2222-222222222222"`, got)
	})

	t.Run("an unknown service name is an error that names it", func(t *testing.T) {
		t.Setenv(EnvTestGeneratePrincipal, "")
		t.Setenv(constants.EnvServiceName, "nobody")
		_, err := ResolveGeneratePrincipal(ctx, admin)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"nobody"`)
	})

	t.Run("nothing to resolve from is an error that names the override", func(t *testing.T) {
		t.Setenv(EnvTestGeneratePrincipal, "")
		t.Setenv(constants.EnvServiceName, "")
		_, err := ResolveGeneratePrincipal(ctx, admin)
		require.Error(t, err)
		assert.Contains(t, err.Error(), EnvTestGeneratePrincipal)
	})
}
