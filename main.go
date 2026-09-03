// Copyright 2026 Redpanda Data, Inc.
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
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

	ctx := context.Background()

	muxed, err := provider.NewMuxedServer(ctx, cloudEnv, version)()
	if err != nil {
		log.Fatal(err.Error())
	}

	var serveOpts []tf6server.ServeOpt
	if debug {
		serveOpts = append(serveOpts, tf6server.WithManagedDebug())
	}

	err = tf6server.Serve(
		"registry.terraform.io/redpanda-data/redpanda",
		func() tfprotov6.ProviderServer { return muxed },
		serveOpts...,
	)
	if err != nil {
		log.Fatal(err.Error())
	}
}
