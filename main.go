package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

// TODO: We should update this on build. Part of the release process
var version = "0.0.0-alpha"

const defaultCloudEnv = "prod"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(
		context.Background(),
		redpanda.New(context.Background(), defaultCloudEnv, version),
		providerserver.ServeOpts{
			Address: "registry.terraform.io/redpanda-data/redpanda",
			Debug:   debug,
		})
	if err != nil {
		log.Fatal(err.Error())
	}
}
