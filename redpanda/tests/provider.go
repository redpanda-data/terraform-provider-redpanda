package tests

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"os"
	"testing"
)

var providerCfgIdSecretVars = config.Variables{
	"client_id":     config.StringVariable(os.Getenv("CLIENT_ID")),
	"client_secret": config.StringVariable(os.Getenv("CLIENT_SECRET")),
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"redpanda": providerserver.NewProtocol6WithError(redpanda.New(context.Background(), "ign")()),
}

// testAccPreCheck is used to perform provider validation before running the provider
func testAccPreCheck(t *testing.T) {

}
