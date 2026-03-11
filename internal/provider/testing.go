package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/beyondtrust/terraform-provider-beyondtrust/internal/acctest"
)

func init() {
	// Register the provider factory for testing
	// This is in a non-test file so it gets loaded when the package is imported
	acctest.RegisterProviderFactory("beyondtrust", providerserver.NewProtocol6WithError(New("test")()))
}
