// Package tests includes the acceptance tests for the Redpanda Terraform
// Provider.
package tests

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

var providerCfgIDSecretVars = config.Variables{
	"client_id":     config.StringVariable(os.Getenv("CLIENT_ID")),
	"client_secret": config.StringVariable(os.Getenv("CLIENT_SECRET")),
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"redpanda": providerserver.NewProtocol6WithError(redpanda.New(context.Background(), "ign")()),
}

// testAccPreCheck is a test helper function used to perform provider validation
// before running the provider
func testAccPreCheck(t testing.TB) {
	if v := os.Getenv("CLIENT_ID"); v == "" {
		t.Fatal("CLIENT_ID must be set for acceptance tests")
	}
	if v := os.Getenv("CLIENT_SECRET"); v == "" {
		t.Fatal("CLIENT_SECRET must be set for acceptance tests")
	}
}
