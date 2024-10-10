package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

var version = "dev"

const (
	defaultCloudEnv = "prod"
	// CloudEnvironmentEnv is the Redpanda cloud environment.
	cloudEnvironmentEnv = "REDPANDA_CLOUD_ENVIRONMENT"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	// handled here rather than in the provider config so it's easier to switch for tests
	cloudEnv := os.Getenv(cloudEnvironmentEnv)
	if cloudEnv == "" {
		cloudEnv = defaultCloudEnv
	}

	err := providerserver.Serve(
		context.Background(),
		redpanda.New(context.Background(), cloudEnv, version),
		providerserver.ServeOpts{
			Address: "registry.terraform.io/redpanda-data/redpanda",
			Debug:   debug,
		})
	if err != nil {
		log.Fatal(err.Error())
	}
}
