//go:build integration

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

package shadowlink_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestIntegration_ShadowLink exercises redpanda_shadow_link against the bufconn fake.
// UpdateShadowLink honors the FieldMask via proto reflection; the update step
// changes a single client_options field to verify mask paths route correctly
// and unmasked fields survive.
func TestIntegration_ShadowLink(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_shadow_link.test"
	shadowID := "abcdefghij0123456789"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockShadowLinkConfig(shadowID, "1000"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "tfrp-mock-shadow"),
					resource.TestCheckResourceAttr(addr, "shadow_redpanda_id", shadowID),
					resource.TestCheckResourceAttr(addr, "client_options.metadata_max_age_ms", "1000"),
					resource.TestCheckResourceAttrSet(addr, "id"),
				),
			},
			{
				Config: mockShadowLinkConfig(shadowID, "1000"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState: true,
				Check:        resource.TestCheckResourceAttr(addr, "client_options.metadata_max_age_ms", "1000"),
			},
			{
				Config: mockShadowLinkConfig(shadowID, "2000"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "client_options.metadata_max_age_ms", "2000"),
			},
		},
	})
}

func mockShadowLinkConfig(shadowID, metadataMaxAgeMs string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-shadow"
  shadow_redpanda_id = %q

  client_options = {
    bootstrap_servers   = ["mock-source.broker:9092"]
    metadata_max_age_ms = %s
  }

  allow_deletion = true
}
`, shadowID, metadataMaxAgeMs)
}

const shadowLinkAddr = "redpanda_shadow_link.test"

// fullBaselineConfig produces a shadow link HCL config with all optional
// blocks populated.
func fullBaselineConfig(name, shadowID string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = %q
  shadow_redpanda_id = %q
  allow_deletion     = true

  client_options = {
    bootstrap_servers   = ["broker-a:9092", "broker-b:9092"]
    connection_timeout_ms = 5000
    fetch_max_bytes = 1048576
    fetch_min_bytes = 131072
    fetch_partition_max_bytes = 262144
    fetch_wait_max_ms = 250
    metadata_max_age_ms = 5000
    retry_backoff_ms = 50
    source_cluster_id = "src-cluster-id-baseline"
    tls_settings = {
      enabled                 = true
      ca                      = "fake-ca-pem"
      do_not_set_sni_hostname = false
    }
  }

  topic_metadata_sync_options = {
    interval                   = "30s"
    paused                     = false
    exclude_default            = true
    synced_shadow_topic_properties = ["retention.ms", "segment.bytes"]
    start_at_earliest          = true
    auto_create_shadow_topic_filters = [
      {
        filter_type  = "INCLUDE"
        name         = "prod-topic"
        pattern_type = "LITERAL"
      }
    ]
  }

  consumer_offset_sync_options = {
    interval = "30s"
    paused   = false
    group_filters = [
      {
        filter_type  = "INCLUDE"
        name         = "my-consumer-group"
        pattern_type = "LITERAL"
      }
    ]
  }

  security_sync_options = {
    interval = "45s"
    paused   = false
    acl_filters = [
      {
        resource_filter = {
          resource_type = "TOPIC"
          pattern_type  = "LITERAL"
          name          = "my-topic"
        }
        access_filter = {
          operation       = "ANY"
          permission_type = "ALLOW"
          principal       = "User:alice"
          host            = "*"
        }
      }
    ]
  }

  schema_registry_sync_options = {
    shadow_schema_registry_topic = true
  }
}
`, name, shadowID)
}

// TestIntegration_ShadowLink_CreateAndRefresh creates a shadow link with all optional
// blocks populated
func TestIntegration_ShadowLink_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name     = "tfrp-mock-sl-create"
		shadowID = "shadow-cluster-id-create"
	)
	cfg := fullBaselineConfig(name, shadowID)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("shadow_redpanda_id"), knownvalue.StringExact(shadowID)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("state"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("bootstrap_servers").AtSliceIndex(0), knownvalue.StringExact("broker-a:9092")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("connection_timeout_ms"), knownvalue.Int64Exact(5000)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("fetch_max_bytes"), knownvalue.Int64Exact(1048576)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("fetch_min_bytes"), knownvalue.Int64Exact(131072)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("fetch_partition_max_bytes"), knownvalue.Int64Exact(262144)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("fetch_wait_max_ms"), knownvalue.Int64Exact(250)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("metadata_max_age_ms"), knownvalue.Int64Exact(5000)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("retry_backoff_ms"), knownvalue.Int64Exact(50)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("source_cluster_id"), knownvalue.StringExact("src-cluster-id-baseline")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("ca"), knownvalue.StringExact("fake-ca-pem")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("do_not_set_sni_hostname"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("interval"), knownvalue.StringExact("30s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("paused"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("exclude_default"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("start_at_earliest"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(0).AtMapKey("filter_type"), knownvalue.StringExact("INCLUDE")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("prod-topic")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(0).AtMapKey("pattern_type"), knownvalue.StringExact("LITERAL")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("interval"), knownvalue.StringExact("30s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("paused"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("group_filters").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("my-consumer-group")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("interval"), knownvalue.StringExact("45s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("paused"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("resource_filter").AtMapKey("resource_type"), knownvalue.StringExact("TOPIC")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("resource_filter").AtMapKey("pattern_type"), knownvalue.StringExact("LITERAL")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("resource_filter").AtMapKey("name"), knownvalue.StringExact("my-topic")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("access_filter").AtMapKey("operation"), knownvalue.StringExact("ANY")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("access_filter").AtMapKey("permission_type"), knownvalue.StringExact("ALLOW")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("access_filter").AtMapKey("principal"), knownvalue.StringExact("User:alice")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("access_filter").AtMapKey("host"), knownvalue.StringExact("*")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("schema_registry_sync_options").AtMapKey("shadow_schema_registry_topic"), knownvalue.Bool(true)),
				idPreserved.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("metadata_max_age_ms"), knownvalue.Int64Exact(5000)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("start_at_earliest"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("schema_registry_sync_options").AtMapKey("shadow_schema_registry_topic"), knownvalue.Bool(true)),
				idPreserved.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_CreateAndRefresh_MinimalConfig creates with only required
// fields, verifying the Optional+Computed blocks settle without error.
func TestIntegration_ShadowLink_CreateAndRefresh_MinimalConfig(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-minimal"
  shadow_redpanda_id = "shadow-cluster-id-minimal"
  source_redpanda_id = "source-cluster-id-minimal"
  allow_deletion     = true
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-sl-minimal")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("shadow_redpanda_id"), knownvalue.StringExact("shadow-cluster-id-minimal")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("source_redpanda_id"), knownvalue.StringExact("source-cluster-id-minimal")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("state"), knownvalue.NotNull()),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_MetadataMaxAgeMs mutates
// client_options.metadata_max_age_ms in-place. Validates FieldMask correctness:
// other blocks survive the partial update unchanged.
func TestIntegration_ShadowLink_UpdateLeaf_MetadataMaxAgeMs(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-upd-meta"
  shadow_redpanda_id = "shadow-cluster-id-meta"
  allow_deletion     = true
  client_options = {
    bootstrap_servers   = ["broker:9092"]
    metadata_max_age_ms = 1000
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-upd-meta"
  shadow_redpanda_id = "shadow-cluster-id-meta"
  allow_deletion     = true
  client_options = {
    bootstrap_servers   = ["broker:9092"]
    metadata_max_age_ms = 2000
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())
	metadataChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("metadata_max_age_ms"), knownvalue.Int64Exact(1000)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
				metadataChanged.AddStateValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("metadata_max_age_ms")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("metadata_max_age_ms"), knownvalue.Int64Exact(2000)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
				metadataChanged.AddStateValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("metadata_max_age_ms")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_TopicMetadataPaused mutates
// topic_metadata_sync_options.paused from false to true.
func TestIntegration_ShadowLink_UpdateLeaf_TopicMetadataPaused(t *testing.T) {
	_, factories := integration.Setup(t)

	makeCfg := func(paused bool) string {
		return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-tm-paused"
  shadow_redpanda_id = "shadow-cluster-id-tm-paused"
  source_redpanda_id = "source-cluster-id-tm-paused"
  allow_deletion     = true
  topic_metadata_sync_options = {
    interval = "30s"
    paused   = %t
  }
}
`, paused)
	}

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, makeCfg(false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("paused"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, makeCfg(true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("paused"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_ConsumerOffsetPaused mutates
// consumer_offset_sync_options.paused from false to true.
func TestIntegration_ShadowLink_UpdateLeaf_ConsumerOffsetPaused(t *testing.T) {
	_, factories := integration.Setup(t)

	makeCfg := func(paused bool) string {
		return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-co-paused"
  shadow_redpanda_id = "shadow-cluster-id-co-paused"
  source_redpanda_id = "source-cluster-id-co-paused"
  allow_deletion     = true
  consumer_offset_sync_options = {
    interval = "30s"
    paused   = %t
  }
}
`, paused)
	}

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, makeCfg(false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("paused"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, makeCfg(true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("paused"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_SecuritySyncPaused mutates
// security_sync_options.paused from false to true.
func TestIntegration_ShadowLink_UpdateLeaf_SecuritySyncPaused(t *testing.T) {
	_, factories := integration.Setup(t)

	makeCfg := func(paused bool) string {
		return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-sec-paused"
  shadow_redpanda_id = "shadow-cluster-id-sec-paused"
  source_redpanda_id = "source-cluster-id-sec-paused"
  allow_deletion     = true
  security_sync_options = {
    interval = "45s"
    paused   = %t
    acl_filters = [
      {
        resource_filter = {
          resource_type = "TOPIC"
          pattern_type  = "LITERAL"
        }
        access_filter = {
          operation       = "ANY"
          permission_type = "ALLOW"
        }
      }
    ]
  }
}
`, paused)
	}

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, makeCfg(false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("paused"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, makeCfg(true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("paused"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_AutoCreateFilters mutates
// topic_metadata_sync_options.auto_create_shadow_topic_filters from one to two.
func TestIntegration_ShadowLink_UpdateLeaf_AutoCreateFilters(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-filters"
  shadow_redpanda_id = "shadow-cluster-id-filters"
  source_redpanda_id = "source-cluster-id-filters"
  allow_deletion     = true
  topic_metadata_sync_options = {
    auto_create_shadow_topic_filters = [
      {
        filter_type  = "INCLUDE"
        name         = "my-topic"
        pattern_type = "LITERAL"
      }
    ]
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-filters"
  shadow_redpanda_id = "shadow-cluster-id-filters"
  source_redpanda_id = "source-cluster-id-filters"
  allow_deletion     = true
  topic_metadata_sync_options = {
    auto_create_shadow_topic_filters = [
      {
        filter_type  = "INCLUDE"
        name         = "my-topic"
        pattern_type = "LITERAL"
      },
      {
        filter_type  = "EXCLUDE"
        name         = "other-topic"
        pattern_type = "PREFIX"
      }
    ]
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("my-topic")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(1).AtMapKey("filter_type"), knownvalue.StringExact("EXCLUDE")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(1).AtMapKey("name"), knownvalue.StringExact("other-topic")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(1).AtMapKey("pattern_type"), knownvalue.StringExact("PREFIX")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_GroupFilters mutates
// consumer_offset_sync_options.group_filters from empty to one filter.
func TestIntegration_ShadowLink_UpdateLeaf_GroupFilters(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-gf"
  shadow_redpanda_id = "shadow-cluster-id-gf"
  source_redpanda_id = "source-cluster-id-gf"
  allow_deletion     = true
  consumer_offset_sync_options = {
    interval = "30s"
    paused   = false
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-gf"
  shadow_redpanda_id = "shadow-cluster-id-gf"
  source_redpanda_id = "source-cluster-id-gf"
  allow_deletion     = true
  consumer_offset_sync_options = {
    interval = "30s"
    paused   = false
    group_filters = [
      {
        filter_type  = "INCLUDE"
        name         = "my-group"
        pattern_type = "LITERAL"
      }
    ]
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("interval"), knownvalue.StringExact("30s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("group_filters").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("my-group")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("group_filters").AtSliceIndex(0).AtMapKey("filter_type"), knownvalue.StringExact("INCLUDE")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("group_filters").AtSliceIndex(0).AtMapKey("pattern_type"), knownvalue.StringExact("LITERAL")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_AclFilters mutates
// security_sync_options.acl_filters from one to two entries.
func TestIntegration_ShadowLink_UpdateLeaf_AclFilters(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-acl"
  shadow_redpanda_id = "shadow-cluster-id-acl"
  source_redpanda_id = "source-cluster-id-acl"
  allow_deletion     = true
  security_sync_options = {
    interval = "45s"
    paused   = false
    acl_filters = [
      {
        resource_filter = {
          resource_type = "TOPIC"
          pattern_type  = "LITERAL"
          name          = "topic-a"
        }
        access_filter = {
          operation       = "READ"
          permission_type = "ALLOW"
          principal       = "User:alice"
          host            = "*"
        }
      }
    ]
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-acl"
  shadow_redpanda_id = "shadow-cluster-id-acl"
  source_redpanda_id = "source-cluster-id-acl"
  allow_deletion     = true
  security_sync_options = {
    interval = "45s"
    paused   = false
    acl_filters = [
      {
        resource_filter = {
          resource_type = "TOPIC"
          pattern_type  = "LITERAL"
          name          = "topic-a"
        }
        access_filter = {
          operation       = "READ"
          permission_type = "ALLOW"
          principal       = "User:alice"
          host            = "*"
        }
      },
      {
        resource_filter = {
          resource_type = "GROUP"
          pattern_type  = "PREFIXED"
          name          = "grp-"
        }
        access_filter = {
          operation       = "WRITE"
          permission_type = "DENY"
          principal       = "User:bob"
          host            = "10.0.0.1"
        }
      }
    ]
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("resource_filter").AtMapKey("name"), knownvalue.StringExact("topic-a")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(1).AtMapKey("resource_filter").AtMapKey("resource_type"), knownvalue.StringExact("GROUP")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(1).AtMapKey("resource_filter").AtMapKey("pattern_type"), knownvalue.StringExact("PREFIXED")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(1).AtMapKey("resource_filter").AtMapKey("name"), knownvalue.StringExact("grp-")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(1).AtMapKey("access_filter").AtMapKey("operation"), knownvalue.StringExact("WRITE")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(1).AtMapKey("access_filter").AtMapKey("permission_type"), knownvalue.StringExact("DENY")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(1).AtMapKey("access_filter").AtMapKey("principal"), knownvalue.StringExact("User:bob")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(1).AtMapKey("access_filter").AtMapKey("host"), knownvalue.StringExact("10.0.0.1")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_StartAtEarliest adds start_at_earliest=true.
// Flatten drops false to null, so true is the only stable end-to-end value.
func TestIntegration_ShadowLink_UpdateLeaf_StartAtEarliest(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-earliest"
  shadow_redpanda_id = "shadow-cluster-id-earliest"
  source_redpanda_id = "source-cluster-id-earliest"
  allow_deletion     = true
  topic_metadata_sync_options = {
    interval = "30s"
    paused   = false
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-earliest"
  shadow_redpanda_id = "shadow-cluster-id-earliest"
  source_redpanda_id = "source-cluster-id-earliest"
  allow_deletion     = true
  topic_metadata_sync_options = {
    interval          = "30s"
    paused            = false
    start_at_earliest = true
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("start_at_earliest"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_StartAtEarliest_FalseRejected proves the boolvalidator
// rejects `start_at_earliest = false` at config-validation time. The proto
// backing the field is a oneof variant (presence-only) — `false` has no
// representable state on the wire, so the schema-level validator forces
// users to either set `true` or omit the field.
func TestIntegration_ShadowLink_StartAtEarliest_FalseRejected(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-earliest-rej"
  shadow_redpanda_id = "shadow-cluster-id-earliest-rej"
  source_redpanda_id = "source-cluster-id-earliest-rej"
  allow_deletion     = true
  topic_metadata_sync_options = {
    start_at_earliest = false
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?s)start_at_earliest.*Value\s+must be "true",\s+got:\s+false`),
			},
		},
	})
}

// TestIntegration_ShadowLink_StartAtLatest_FalseRejected — same oneof-presence
// rejection as start_at_earliest, but for start_at_latest.
func TestIntegration_ShadowLink_StartAtLatest_FalseRejected(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-latest-rej"
  shadow_redpanda_id = "shadow-cluster-id-latest-rej"
  source_redpanda_id = "source-cluster-id-latest-rej"
  allow_deletion     = true
  topic_metadata_sync_options = {
    start_at_latest = false
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?s)start_at_latest.*Value\s+must be "true",\s+got:\s+false`),
			},
		},
	})
}

// TestIntegration_ShadowLink_ShadowSchemaRegistryTopic_FalseRejected — same
// oneof-presence rejection as start_at_earliest, but for the schema-registry
// sync option.
func TestIntegration_ShadowLink_ShadowSchemaRegistryTopic_FalseRejected(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-sr-rej"
  shadow_redpanda_id = "shadow-cluster-id-sr-rej"
  source_redpanda_id = "source-cluster-id-sr-rej"
  allow_deletion     = true
  schema_registry_sync_options = {
    shadow_schema_registry_topic = false
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?s)shadow_schema_registry_topic.*Value\s+must be "true",\s+got:\s+false`),
			},
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_StartAtTimestamp sets start_at_timestamp.
// Uses an RFC3339 string; starts with no start_at_* to avoid oneof conflicts.
func TestIntegration_ShadowLink_UpdateLeaf_StartAtTimestamp(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-ts"
  shadow_redpanda_id = "shadow-cluster-id-ts"
  source_redpanda_id = "source-cluster-id-ts"
  allow_deletion     = true
  topic_metadata_sync_options = {
    interval = "30s"
    paused   = false
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-ts"
  shadow_redpanda_id = "shadow-cluster-id-ts"
  source_redpanda_id = "source-cluster-id-ts"
  allow_deletion     = true
  topic_metadata_sync_options = {
    interval           = "30s"
    paused             = false
    start_at_timestamp = "2024-01-15T00:00:00Z"
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("start_at_timestamp"), knownvalue.StringExact("2024-01-15T00:00:00Z")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_TLSSettings updates tls_settings (ca, cert,
// enabled, do_not_set_sni_hostname) without the sensitive key field
// (Flatten emits sensitive fields inconsistently).
func TestIntegration_ShadowLink_UpdateLeaf_TLSSettings(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-tls"
  shadow_redpanda_id = "shadow-cluster-id-tls"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    tls_settings = {
      enabled = false
    }
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-tls"
  shadow_redpanda_id = "shadow-cluster-id-tls"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    tls_settings = {
      enabled                 = true
      ca                      = "fake-ca-pem"
      do_not_set_sni_hostname = true
    }
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("enabled"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("ca"), knownvalue.StringExact("fake-ca-pem")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("do_not_set_sni_hostname"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_TLSKey exercises the sensitive
// tls_settings.key write path. The proto requires the key to be a
// `${secrets.<SECRET_ID>}` reference (not raw PEM), and "key and cert must
// both be provided or both be empty" — so we provide both in the same
// matched-pair format.
func TestIntegration_ShadowLink_UpdateLeaf_TLSKey(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-tlskey"
  shadow_redpanda_id = "shadow-cluster-id-tlskey"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    tls_settings = {
      enabled = true
      key     = "$${secrets.tls_key_initial}"
      cert    = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+w==\n-----END CERTIFICATE-----"
    }
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-tlskey"
  shadow_redpanda_id = "shadow-cluster-id-tlskey"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    tls_settings = {
      enabled = true
      key     = "$${secrets.tls_key_updated}"
      cert    = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+w==\n-----END CERTIFICATE-----"
    }
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, nil),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("key"), knownvalue.NotNull()),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_ScramPassword exercises the sensitive
// scram.password write path.
func TestIntegration_ShadowLink_UpdateLeaf_ScramPassword(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-scram"
  shadow_redpanda_id = "shadow-cluster-id-scram"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    authentication_configuration = {
      scram_configuration = {
        username         = "user1"
        scram_mechanism  = "SCRAM_SHA_256"
        password         = "$${secrets.scram_pw_initial}"
      }
    }
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-scram"
  shadow_redpanda_id = "shadow-cluster-id-scram"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    authentication_configuration = {
      scram_configuration = {
        username         = "user1"
        scram_mechanism  = "SCRAM_SHA_256"
        password         = "$${secrets.scram_pw_updated}"
      }
    }
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, nil),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("authentication_configuration").AtMapKey("scram_configuration").AtMapKey("username"), knownvalue.StringExact("user1")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_PlainPassword exercises the sensitive
// plain.password write path.
func TestIntegration_ShadowLink_UpdateLeaf_PlainPassword(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg1 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-plain"
  shadow_redpanda_id = "shadow-cluster-id-plain"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    authentication_configuration = {
      plain_configuration = {
        username = "user1"
        password = "$${secrets.plain_pw_initial}"
      }
    }
  }
}
`

	cfg2 := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-plain"
  shadow_redpanda_id = "shadow-cluster-id-plain"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    authentication_configuration = {
      plain_configuration = {
        username = "user1"
        password = "$${secrets.plain_pw_updated}"
      }
    }
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg1, nil),
			integration.UpdateLeafStep(shadowLinkAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("authentication_configuration").AtMapKey("plain_configuration").AtMapKey("username"), knownvalue.StringExact("user1")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_RequiresReplace_Name mutates name (RequiresReplace).
// id changes on replace.
func TestIntegration_ShadowLink_RequiresReplace_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	makeCfg := func(name string) string {
		return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = %q
  shadow_redpanda_id = "shadow-cluster-id-rr-name"
  source_redpanda_id = "source-cluster-id-rr-name"
  allow_deletion     = true
}
`, name)
	}

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, makeCfg("tfrp-mock-sl-rr-name-a"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-sl-rr-name-a")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(shadowLinkAddr, makeCfg("tfrp-mock-sl-rr-name-b"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-sl-rr-name-b")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_RequiresReplace_ShadowRedpandaId mutates
// shadow_redpanda_id (RequiresReplace). id changes on replace.
func TestIntegration_ShadowLink_RequiresReplace_ShadowRedpandaId(t *testing.T) {
	_, factories := integration.Setup(t)

	makeCfg := func(shadowRedpandaID string) string {
		return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-rr-shadow"
  shadow_redpanda_id = %q
  source_redpanda_id = "source-cluster-id-rr-shadow"
  allow_deletion     = true
}
`, shadowRedpandaID)
	}

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, makeCfg("shadow-cluster-id-aaa"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("shadow_redpanda_id"), knownvalue.StringExact("shadow-cluster-id-aaa")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(shadowLinkAddr, makeCfg("shadow-cluster-id-bbb"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("shadow_redpanda_id"), knownvalue.StringExact("shadow-cluster-id-bbb")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_RequiresReplace_SourceRedpandaId mutates
// source_redpanda_id (extra TF-only RequiresReplace field). id changes on replace.
func TestIntegration_ShadowLink_RequiresReplace_SourceRedpandaId(t *testing.T) {
	_, factories := integration.Setup(t)

	makeCfg := func(sourceRedpandaID string) string {
		return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-rr-source"
  shadow_redpanda_id = "shadow-cluster-id-rr-source"
  source_redpanda_id = %q
  allow_deletion     = true
}
`, sourceRedpandaID)
	}

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, makeCfg("source-cluster-id-aaa"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("source_redpanda_id"), knownvalue.StringExact("source-cluster-id-aaa")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(shadowLinkAddr, makeCfg("source-cluster-id-bbb"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("source_redpanda_id"), knownvalue.StringExact("source-cluster-id-bbb")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_NestedMatrix_ClientOptions_Dense creates with a fully
// populated client_options block (all non-sensitive leaves). Noop step
// validates UseStateForUnknown preservation.
func TestIntegration_ShadowLink_NestedMatrix_ClientOptions_Dense(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-co-dense"
  shadow_redpanda_id = "shadow-cluster-id-co-dense"
  allow_deletion     = true
  client_options = {
    bootstrap_servers         = ["broker-a:9092", "broker-b:9092"]
    connection_timeout_ms     = 500
    fetch_max_bytes           = 1048576
    fetch_min_bytes           = 131072
    fetch_partition_max_bytes = 262144
    fetch_wait_max_ms         = 250
    metadata_max_age_ms       = 5000
    retry_backoff_ms          = 50
    source_cluster_id         = "src-cluster-id-dense"
    tls_settings = {
      enabled                 = true
      ca                      = "ca-pem"
      do_not_set_sni_hostname = true
    }
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("connection_timeout_ms"), knownvalue.Int64Exact(500)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("fetch_max_bytes"), knownvalue.Int64Exact(1048576)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("fetch_min_bytes"), knownvalue.Int64Exact(131072)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("fetch_partition_max_bytes"), knownvalue.Int64Exact(262144)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("fetch_wait_max_ms"), knownvalue.Int64Exact(250)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("metadata_max_age_ms"), knownvalue.Int64Exact(5000)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("retry_backoff_ms"), knownvalue.Int64Exact(50)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("source_cluster_id"), knownvalue.StringExact("src-cluster-id-dense")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("ca"), knownvalue.StringExact("ca-pem")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("do_not_set_sni_hostname"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("connection_timeout_ms"), knownvalue.Int64Exact(500)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("source_cluster_id"), knownvalue.StringExact("src-cluster-id-dense")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_NestedMatrix_ClientOptions_Null omits client_options.
// When source_redpanda_id is used without bootstrap_servers, the fake stores
// nil ClientOptions; Flatten emits null for the block; UseStateForUnknown
// preserves null on re-plan.
func TestIntegration_ShadowLink_NestedMatrix_ClientOptions_Null(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-co-null"
  shadow_redpanda_id = "shadow-cluster-id-co-null"
  source_redpanda_id = "source-cluster-id-co-null"
  allow_deletion     = true
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options"), knownvalue.Null()),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
		},
	})
}

// TestIntegration_ShadowLink_NestedMatrix_TopicMetadata_Dense creates with fully
// populated topic_metadata_sync_options. Noop step validates UseStateForUnknown.
func TestIntegration_ShadowLink_NestedMatrix_TopicMetadata_Dense(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-tm-dense"
  shadow_redpanda_id = "shadow-cluster-id-tm-dense"
  source_redpanda_id = "source-cluster-id-tm-dense"
  allow_deletion     = true
  topic_metadata_sync_options = {
    interval                       = "30s"
    paused                         = true
    exclude_default                = true
    synced_shadow_topic_properties = ["retention.ms", "segment.bytes"]
    start_at_earliest              = true
    auto_create_shadow_topic_filters = [
      {
        filter_type  = "INCLUDE"
        name         = "prod-*"
        pattern_type = "PREFIX"
      }
    ]
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("interval"), knownvalue.StringExact("30s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("paused"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("exclude_default"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("synced_shadow_topic_properties").AtSliceIndex(0), knownvalue.StringExact("retention.ms")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("synced_shadow_topic_properties").AtSliceIndex(1), knownvalue.StringExact("segment.bytes")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("start_at_earliest"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(0).AtMapKey("filter_type"), knownvalue.StringExact("INCLUDE")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("prod-*")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("auto_create_shadow_topic_filters").AtSliceIndex(0).AtMapKey("pattern_type"), knownvalue.StringExact("PREFIX")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("paused"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("topic_metadata_sync_options").AtMapKey("start_at_earliest"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_NestedMatrix_ConsumerOffset_Dense creates with fully
// populated consumer_offset_sync_options.
func TestIntegration_ShadowLink_NestedMatrix_ConsumerOffset_Dense(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-co-d"
  shadow_redpanda_id = "shadow-cluster-id-co-d"
  source_redpanda_id = "source-cluster-id-co-d"
  allow_deletion     = true
  consumer_offset_sync_options = {
    interval = "30s"
    paused   = true
    group_filters = [
      {
        filter_type  = "INCLUDE"
        name         = "my-consumer-group"
        pattern_type = "LITERAL"
      }
    ]
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("interval"), knownvalue.StringExact("30s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("paused"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("group_filters").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("my-consumer-group")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("group_filters").AtSliceIndex(0).AtMapKey("filter_type"), knownvalue.StringExact("INCLUDE")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("group_filters").AtSliceIndex(0).AtMapKey("pattern_type"), knownvalue.StringExact("LITERAL")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("interval"), knownvalue.StringExact("30s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("consumer_offset_sync_options").AtMapKey("group_filters").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("my-consumer-group")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_NestedMatrix_SecuritySync_Dense creates with fully
// populated security_sync_options including all Required + Optional sub-fields.
func TestIntegration_ShadowLink_NestedMatrix_SecuritySync_Dense(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-sec-d"
  shadow_redpanda_id = "shadow-cluster-id-sec-d"
  source_redpanda_id = "source-cluster-id-sec-d"
  allow_deletion     = true
  security_sync_options = {
    interval = "45s"
    paused   = false
    acl_filters = [
      {
        resource_filter = {
          resource_type = "TOPIC"
          pattern_type  = "LITERAL"
          name          = "my-topic"
        }
        access_filter = {
          operation       = "ANY"
          permission_type = "ALLOW"
          principal       = "User:alice"
          host            = "*"
        }
      }
    ]
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("interval"), knownvalue.StringExact("45s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("paused"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("resource_filter").AtMapKey("resource_type"), knownvalue.StringExact("TOPIC")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("resource_filter").AtMapKey("pattern_type"), knownvalue.StringExact("LITERAL")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("resource_filter").AtMapKey("name"), knownvalue.StringExact("my-topic")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("access_filter").AtMapKey("operation"), knownvalue.StringExact("ANY")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("access_filter").AtMapKey("permission_type"), knownvalue.StringExact("ALLOW")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("access_filter").AtMapKey("principal"), knownvalue.StringExact("User:alice")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("access_filter").AtMapKey("host"), knownvalue.StringExact("*")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("interval"), knownvalue.StringExact("45s")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("security_sync_options").AtMapKey("acl_filters").AtSliceIndex(0).AtMapKey("resource_filter").AtMapKey("name"), knownvalue.StringExact("my-topic")),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_NestedMatrix_SchemaRegistry_Dense creates with
// shadow_schema_registry_topic=true. Noop step validates the bool is preserved.
func TestIntegration_ShadowLink_NestedMatrix_SchemaRegistry_Dense(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-sr-dense"
  shadow_redpanda_id = "shadow-cluster-id-sr-dense"
  source_redpanda_id = "source-cluster-id-sr-dense"
  allow_deletion     = true
  schema_registry_sync_options = {
    shadow_schema_registry_topic = true
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("schema_registry_sync_options").AtMapKey("shadow_schema_registry_topic"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("schema_registry_sync_options").AtMapKey("shadow_schema_registry_topic"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_ImportRoundTrip verifies that import correctly
// reconstructs source_redpanda_id from the composite "<id>|<source_redpanda_id>"
// import ID format.
func TestIntegration_ShadowLink_ImportRoundTrip(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name     = "tfrp-mock-sl-import"
		shadowID = "shadow-cluster-id-import"
		sourceID = "source-cluster-id-import"
	)
	cfg := fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = %q
  shadow_redpanda_id = %q
  source_redpanda_id = %q
  allow_deletion     = true
}
`, name, shadowID, sourceID)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("source_redpanda_id"), knownvalue.StringExact(sourceID)),
			}),
			integration.ImportRoundTripStep(shadowLinkAddr, func(s *terraform.State) (string, error) {
				rs, ok := s.RootModule().Resources[shadowLinkAddr]
				if !ok {
					return "", fmt.Errorf("resource %q not found", shadowLinkAddr)
				}
				attrs := rs.Primary.Attributes
				return attrs["id"] + "|" + attrs["source_redpanda_id"], nil
			}, []string{"allow_deletion"}),
		},
	})
}

// TestIntegration_ShadowLink_ErrorPath_CreateFails injects Internal on CreateShadowLink.
func TestIntegration_ShadowLink_ErrorPath_CreateFails(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-create-fail"
  shadow_redpanda_id = "shadow-cluster-id-create-fail"
  source_redpanda_id = "source-cluster-id-create-fail"
  allow_deletion     = true
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				controlplanev1grpc.ShadowLinkService_CreateShadowLink_FullMethodName,
				codes.Internal,
				cfg,
				"create failed",
			),
		},
	})
}

// TestIntegration_ShadowLink_ErrorPath_ReadFails injects Internal on GetShadowLink
// after a successful Create. The provider's Read sees a non-NotFound error
// and emits a "failed to read" diagnostic.
func TestIntegration_ShadowLink_ErrorPath_ReadFails(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-read-fail"
  shadow_redpanda_id = "shadow-cluster-id-read-fail"
  source_redpanda_id = "source-cluster-id-read-fail"
  allow_deletion     = true
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ShadowLinkService_GetShadowLink_FullMethodName,
						status.Error(codes.Internal, "synthetic read failure"),
					)
				},
				Config:      cfg,
				ExpectError: regexp.MustCompile("failed to read"),
			},
		},
	})
}

// TestIntegration_ShadowLink_ErrorPath_AllowDeletionBlocked creates with
// allow_deletion=false and then attempts destroy. The provider's Delete
// blocks before issuing the RPC and emits "shadow link deletion not allowed".
// A final update step flips allow_deletion=true so the framework's implicit
// post-test destroy can succeed.
func TestIntegration_ShadowLink_ErrorPath_AllowDeletionBlocked(t *testing.T) {
	_, factories := integration.Setup(t)

	cfgBlocked := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-del-blocked"
  shadow_redpanda_id = "shadow-cluster-id-del-blocked"
  source_redpanda_id = "source-cluster-id-del-blocked"
  allow_deletion     = false
}
`

	cfgUnblocked := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-del-blocked"
  shadow_redpanda_id = "shadow-cluster-id-del-blocked"
  source_redpanda_id = "source-cluster-id-del-blocked"
  allow_deletion     = true
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfgBlocked, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				Config:      cfgBlocked,
				Destroy:     true,
				ExpectError: regexp.MustCompile("shadow link deletion not allowed"),
			},
			integration.UpdateLeafStep(shadowLinkAddr, cfgUnblocked, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
			}),
		},
	})
}

// TestIntegration_ShadowLink_UpdateLeaf_TLSKey_PreservedAcrossMaskedRead
// exercises the preserveSensitiveFromPrev hook end-to-end against the fake.
//
// The fake's GetShadowLink masks tls_settings.key to "" (mirroring real
// backend behavior). preserveSensitiveFromPrev must restore the value from
// prior plan/state on every Read; otherwise the NoopReapply step detects
// drift and plans an update.
//
// Sequence:
//  1. Create with key = A — fake stores A, returns "" on Read, hook restores A
//     from plan → state holds A. ExpectEmptyPlan confirms no drift.
//  2. UpdateLeaf key A → B — fake stores B, returns "" on Read, hook restores B
//     from plan → state holds B. ExpectEmptyPlan confirms no drift.
//  3. NoopReapply — fake still returns "" on Read, hook restores B from state
//     → plan is empty. This step is the regression assertion: if the hook is
//     removed, this step fails because config has B but Read returned "".
func TestIntegration_ShadowLink_UpdateLeaf_TLSKey_PreservedAcrossMaskedRead(t *testing.T) {
	_, factories := integration.Setup(t)

	// In HCL $${...} is the escape sequence that produces a literal ${...} string.
	// The state therefore holds the single-$ form of each reference.
	const (
		keyAState = `${secrets.tls_key_initial}`
		keyBState = `${secrets.tls_key_updated}`
	)

	cfgA := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-tlskey-hook"
  shadow_redpanda_id = "shadow-cluster-id-tlskey-hook"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    tls_settings = {
      enabled = true
      key     = "$${secrets.tls_key_initial}"
      cert    = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+w==\n-----END CERTIFICATE-----"
    }
  }
}
`
	cfgB := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-tlskey-hook"
  shadow_redpanda_id = "shadow-cluster-id-tlskey-hook"
  allow_deletion     = true
  client_options = {
    bootstrap_servers = ["broker:9092"]
    tls_settings = {
      enabled = true
      key     = "$${secrets.tls_key_updated}"
      cert    = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+w==\n-----END CERTIFICATE-----"
    }
  }
}
`

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfgA, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("key"), knownvalue.StringExact(keyAState)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(shadowLinkAddr, cfgB, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("key"), knownvalue.StringExact(keyBState)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(shadowLinkAddr, cfgB, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("client_options").AtMapKey("tls_settings").AtMapKey("key"), knownvalue.StringExact(keyBState)),
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(shadowLinkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ShadowLink_ErrorPath_DeleteFails injects Internal on
// DeleteShadowLink after a successful Create.
func TestIntegration_ShadowLink_ErrorPath_DeleteFails(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_shadow_link" "test" {
  name               = "tfrp-mock-sl-del-fail"
  shadow_redpanda_id = "shadow-cluster-id-del-fail"
  source_redpanda_id = "source-cluster-id-del-fail"
  allow_deletion     = true
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(shadowLinkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(shadowLinkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ShadowLinkService_DeleteShadowLink_FullMethodName,
						status.Error(codes.Internal, "synthetic delete failure"),
					)
				},
				Config:      cfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile("synthetic delete failure"),
			},
		},
	})
}
