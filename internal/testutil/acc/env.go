// Copyright 2026 Redpanda Data, Inc.
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

package acc

import (
	"fmt"
	"os"

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

// NamePrefix is prepended to every generated test-resource name.
var NamePrefix = "tfrp-acc-"

// ClientID and ClientSecret carry the Redpanda Cloud OAuth credentials
// derived from the standard provider env vars.
var (
	ClientID     = os.Getenv(redpanda.ClientIDEnv)
	ClientSecret = os.Getenv(redpanda.ClientSecretEnv)
)

// TestAgainstExistingCluster gates the datasource matrix runner.
var TestAgainstExistingCluster = os.Getenv("TEST_AGAINST_EXISTING_CLUSTER")

// RedpandaVersion is an optional pin for the cluster version under test.
var RedpandaVersion = os.Getenv("REDPANDA_VERSION")

// CloudEnv selects the Redpanda Cloud environment ("pre", "ign", "prod").
var CloudEnv string

// ThroughputTier is the cluster throughput-tier override for tests.
var ThroughputTier string

// CloudLabel* are short test-name fragments injected into generated
// resource names so different cloud variants do not collide.
var (
	CloudLabelAWS       = "testaws"
	CloudLabelAWSRename = "testaws-rename"
)

func init() {
	if v := os.Getenv("REDPANDA_CLOUD_ENVIRONMENT"); v != "" {
		CloudEnv = v
	} else {
		CloudEnv = "pre"
	}

	if CloudEnv == "ign" {
		if os.Getenv("GOOGLE_PROJECT") != "" {
			fmt.Println("cloud environment ign but provider gcp, setting throughput tier to nothing")
			ThroughputTier = ""
		} else {
			fmt.Println("cloud environment ign, setting throughput tier to test")
			ThroughputTier = "test"
		}
	} else if v := os.Getenv("THROUGHPUT_TIER"); v != "" {
		fmt.Println("setting throughput tier to", v)
		ThroughputTier = v
	}
}
