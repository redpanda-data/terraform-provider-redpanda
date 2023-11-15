package main

import (
	"context"
	"flag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"log"
)

var version = "dev"
var debug = false

func main() {
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	log.Println("starting")
	// Setup connection

	if err := providerserver.Serve(context.Background(), redpanda.New(context.Background(), version), providerserver.ServeOpts{
		Address: "registry.terraform.io/redpanda-data/redpanda",
		Debug:   debug,
	}); err != nil {
		log.Fatal(err.Error()) // TODO dont
	}
}
