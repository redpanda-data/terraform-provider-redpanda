package main

import (
	"context"
	"flag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"log"
)

var version string
var debug bool

func main() {
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.StringVar(&version, "version", "ign", "version of the provider")
	flag.Parse()

	if err := providerserver.Serve(context.Background(), redpanda.New(context.Background(), version), providerserver.ServeOpts{
		Address: "registry.terraform.io/redpanda-data/redpanda",
		Debug:   debug,
	}); err != nil {
		log.Fatal(err.Error()) // TODO dont
	}
}
