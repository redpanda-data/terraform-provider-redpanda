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

// TODO fix acceptance tests not working when auth_token is passed in instead of ID+SECRET
var providerCfgAuthVars = config.Variables{
	"auth_token": config.StringVariable(os.Getenv("AUTH_TOKEN")),
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"redpanda": providerserver.NewProtocol6WithError(redpanda.New(context.Background(), "ign")()),
}

// testAccPreCheck is used to perform provider validation before running the provider
func testAccPreCheck(t *testing.T) {

}
