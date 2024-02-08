// Copyright 2023 Redpanda Data, Inc.
//
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
	"client_id":     config.StringVariable(os.Getenv(redpanda.ClientIDEnv)),
	"client_secret": config.StringVariable(os.Getenv(redpanda.ClientSecretEnv)),
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"redpanda": providerserver.NewProtocol6WithError(redpanda.New(context.Background(), "ign", "test")()),
}

// testAccPreCheck is a test helper function used to perform provider validation
// before running the provider
func testAccPreCheck(t testing.TB) {
	if v := os.Getenv(redpanda.ClientIDEnv); v == "" {
		t.Fatalf("environment variable %v must be set for acceptance tests", redpanda.ClientIDEnv)
	}
	if v := os.Getenv(redpanda.ClientSecretEnv); v == "" {
		t.Fatalf("environment variable %v must be set for acceptance tests", redpanda.ClientSecretEnv)
	}
}
