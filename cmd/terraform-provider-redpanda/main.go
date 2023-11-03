package main

import (
	"context"
	"flag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/provider"
	"log"
)

var version = "dev"
var debug = false

func main() {
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	log.Println("starting")
	// Setup connection

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/redpanda-data/redpanda",
		Debug:   debug,
	}
	if err := providerserver.Serve(context.Background(), provider.New(context.Background(), version), opts); err != nil {
		log.Fatal(err.Error()) // TODO dont
	}

}
