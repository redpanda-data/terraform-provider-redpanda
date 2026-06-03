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

package topic_test

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/topic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const topicAddr = "redpanda_topic.test"

// TestIntegration_Topic exercises redpanda_topic end-to-end against the bufconn-backed
// fake dataplane. Covers default-create, refresh, no-op update, partition-count
// increase, and a configuration-entry add.
func TestIntegration_Topic(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_topic.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockTopicConfig(3, 1, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "mock-topic"),
					resource.TestCheckResourceAttr(addr, "partition_count", "3"),
					resource.TestCheckResourceAttr(addr, "replication_factor", "1"),
					resource.TestCheckResourceAttr(addr, "allow_deletion", "true"),
					resource.TestCheckResourceAttrSet(addr, "id"),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "mock-topic"),
					resource.TestCheckResourceAttr(addr, "partition_count", "3"),
				),
			},
			{
				Config: mockTopicConfig(3, 1, ""),
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
				Config: mockTopicConfig(6, 1, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "partition_count", "6"),
			},
			{
				Config: mockTopicConfig(6, 1, `"retention.ms" = "86400000"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "configuration.retention.ms", "86400000"),
			},
		},
	})
}

func mockTopicConfig(partitionCount, replicationFactor int, configEntries string) string {
	cfgBlock := ""
	if configEntries != "" {
		cfgBlock = fmt.Sprintf("  configuration = {\n    %s\n  }\n", configEntries)
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_topic" "test" {
  name               = "mock-topic"
  partition_count    = %d
  replication_factor = %d
  cluster_api_url    = "bufnet"
  allow_deletion     = true
%s}
`, partitionCount, replicationFactor, cfgBlock)
}

// mockTopicCreateConfig renders an HCL topic resource for the canonical
// (partition_count + replication_factor) shape. Pass an empty configBlock for
// no configuration map; otherwise pass a rendered `configuration = { ... }`
// HCL fragment ending with a newline.
func mockTopicCreateConfig(name, clusterAPIURL string, partitionCount, replicationFactor int, configBlock string, allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_topic" "test" {
  name               = %q
  cluster_api_url    = %q
  partition_count    = %d
  replication_factor = %d
  allow_deletion     = %t
%s}
`, name, clusterAPIURL, partitionCount, replicationFactor, allowDeletion, configBlock)
}

// mockTopicReplicaAssignmentsConfig renders an HCL topic resource for the
// replica_assignments alt-shape. partition_count and replication_factor are
// pinned to -1 as required by the proto-validate mutual-exclusion rule
// ("If manually assigning replicas, both replication_factor and
// partition_count must be -1.").
//
// replica_assignments is a ListNestedAttribute on the framework schema, so
// it uses `= [ { ... }, ... ]` assignment, not block syntax.
func mockTopicReplicaAssignmentsConfig(name string, assignments []topicRAEntry) string {
	var entries string
	for i, a := range assignments {
		var idsStr string
		for j, v := range a.replicaIDs {
			if j > 0 {
				idsStr += ", "
			}
			idsStr += fmt.Sprintf("%d", v)
		}
		sep := ""
		if i > 0 {
			sep = ",\n"
		}
		entries += fmt.Sprintf(`%s    {
      partition_id = %d
      replica_ids  = [%s]
    }`, sep, a.partitionID, idsStr)
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_topic" "test" {
  name               = %q
  cluster_api_url    = "bufnet"
  partition_count    = -1
  replication_factor = -1
  allow_deletion     = true
  replica_assignments = [
%s
  ]
}
`, name, entries)
}

type topicRAEntry struct {
	partitionID int
	replicaIDs  []int
}

// bigFloat returns a *big.Float for the integer i, matching the precision
// the framework uses for NumberAttribute knownvalue comparisons.
func bigFloat(i int) *big.Float { return big.NewFloat(float64(i)) }

// TestIntegration_Topic_CreateAndRefresh exercises the canonical create + noop cycle.
// Full leaf assertions on the canonical (partition_count + replication_factor)
// shape. UseStateForUnknown on id is proven via CompareValue ValuesSame across
// Create and Noop (id = topic name per conv_gen.go:59).
func TestIntegration_Topic_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)
	cfg := mockTopicCreateConfig("tfrp-mock-topic-create", "bufnet", 3, 1, "", true)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	baseChecks := func() []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-topic-create")),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("partition_count"), knownvalue.NumberExact(bigFloat(3))),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replication_factor"), knownvalue.NumberExact(bigFloat(1))),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("id"), knownvalue.StringExact("tfrp-mock-topic-create")),
			idPreserved.AddStateValue(topicAddr, tfjsonpath.New("id")),
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfg, baseChecks()),
			integration.NoopReapplyStep(topicAddr, cfg, baseChecks()),
		},
	})
}

// TestIntegration_Topic_CreateAndRefresh_WithConfiguration creates a topic
// that ships with a basic custom configuration (retention.ms) from the start,
// then re-applies the same config. The no-op reapply guard is the load-bearing
// assertion: it proves that the GetTopicConfigurations → mergeWithPlannedConfig
// → state cycle produces a map that re-plans cleanly. retention.ms is a
// kafka-side key (not redpanda.*) so the strip branch is not involved here —
// this is the plain happy-path counterpart to the no-config CreateAndRefresh.
func TestIntegration_Topic_CreateAndRefresh_WithConfiguration(t *testing.T) {
	_, factories := integration.Setup(t)
	cfg := mockTopicCreateConfig("tfrp-mock-topic-create-cfg", "bufnet", 3, 1,
		"  configuration = {\n    \"retention.ms\" = \"86400000\"\n  }\n", true)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	baseChecks := func() []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-topic-create-cfg")),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("partition_count"), knownvalue.NumberExact(bigFloat(3))),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replication_factor"), knownvalue.NumberExact(bigFloat(1))),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("configuration").AtMapKey("retention.ms"), knownvalue.StringExact("86400000")),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("id"), knownvalue.StringExact("tfrp-mock-topic-create-cfg")),
			idPreserved.AddStateValue(topicAddr, tfjsonpath.New("id")),
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfg, baseChecks()),
			integration.NoopReapplyStep(topicAddr, cfg, baseChecks()),
		},
	})
}

// TestIntegration_Topic_UpdateLeaf_Configuration proves that adding a configuration
// entry triggers in-place Update (SetTopicConfigurations RPC). id is stable
// (id = name; name unchanged across the update).
func TestIntegration_Topic_UpdateLeaf_Configuration(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-topic-cfg"
	cfgEmpty := mockTopicCreateConfig(name, "bufnet", 3, 1, "", true)
	cfgWithCfg := mockTopicCreateConfig(name, "bufnet", 3, 1, "  configuration = {\n    \"retention.ms\" = \"86400000\"\n  }\n", true)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgEmpty, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(topicAddr, cfgWithCfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("configuration").AtMapKey("retention.ms"), knownvalue.StringExact("86400000")),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Topic_MergeWithPlannedConfig_StripsServerInjectedRedpandaKey
// proves the broker-injected redpanda.* strip branch of mergeWithPlannedConfig
// (resource_topic.go:526) end-to-end. The fake is configured to inject
// redpanda.storage.mode="unset" on every GetTopicConfigurations response —
// mirroring the post-v26.1.1 broker. Without the strip, plan-twice would see
// the key "appear" in state and try to remove it on the next apply, producing
// a perpetual diff. The NoopReapplyStep is the load-bearing assertion: if the
// strip regresses, the second apply would not be a no-op.
func TestIntegration_Topic_MergeWithPlannedConfig_StripsServerInjectedRedpandaKey(t *testing.T) {
	srv, factories := integration.Setup(t)
	srv.Topic.SetServerInjectedConfig("redpanda.storage.mode", "unset")

	const name = "tfrp-mock-topic-strip-rp"
	cfg := mockTopicCreateConfig(name, "bufnet", 3, 1, "", true)

	checks := func() []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("configuration"), knownvalue.MapSizeExact(0)),
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfg, checks()),
			integration.NoopReapplyStep(topicAddr, cfg, checks()),
		},
	})
}

// TestIntegration_Topic_MergeWithPlannedConfig_PreservesUserNamedRedpandaKey
// proves the other half of the strip branch: when the user explicitly names
// a redpanda.* key in their plan, mergeWithPlannedConfig must keep it. The
// fake is also configured to inject the same key — exercising the "user
// supersedes server injection" path in TopicFake.GetTopicConfigurations
// (the key sits in rec.configs, so the injection is skipped) and confirming
// the merged state reports the user's value.
func TestIntegration_Topic_MergeWithPlannedConfig_PreservesUserNamedRedpandaKey(t *testing.T) {
	srv, factories := integration.Setup(t)
	srv.Topic.SetServerInjectedConfig("redpanda.storage.mode", "unset")

	const name = "tfrp-mock-topic-keep-rp"
	cfg := mockTopicCreateConfig(name, "bufnet", 3, 1,
		"  configuration = {\n    \"redpanda.storage.mode\" = \"tiered\"\n  }\n", true)

	checks := func() []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("configuration").AtMapKey("redpanda.storage.mode"), knownvalue.StringExact("tiered")),
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfg, checks()),
			integration.NoopReapplyStep(topicAddr, cfg, checks()),
		},
	})
}

// TestIntegration_Topic_UpdateLeaf_AllowDeletion flips allow_deletion true→false→true.
// The Update path skips both SetTopicConfigurations (configuration unchanged)
// and SetTopicPartitions (partition_count unchanged) on these flips; only
// GetTopicConfigurations runs at the tail. id stable. Ends with
// allow_deletion=true so the terminal cleanup destroy succeeds.
func TestIntegration_Topic_UpdateLeaf_AllowDeletion(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-topic-allow-del"
	cfgTrue := mockTopicCreateConfig(name, "bufnet", 3, 1, "", true)
	cfgFalse := mockTopicCreateConfig(name, "bufnet", 3, 1, "", false)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgTrue, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(topicAddr, cfgFalse, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(topicAddr, cfgTrue, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Topic_RequiresReplace_Name mutates the topic name. Since id=name,
// the id DIFFERS across the replace — both the plancheck
// (DestroyBeforeCreate) and the ValuesDiffer id comparer pin the behavior.
func TestIntegration_Topic_RequiresReplace_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	cfgA := mockTopicCreateConfig("tfrp-mock-topic-rr-name-a", "bufnet", 3, 1, "", true)
	cfgB := mockTopicCreateConfig("tfrp-mock-topic-rr-name-b", "bufnet", 3, 1, "", true)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgA, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-topic-rr-name-a")),
				idChanged.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(topicAddr, cfgB, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-topic-rr-name-b")),
				idChanged.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Topic_RequiresReplace_ClusterApiUrl mutates cluster_api_url
// bufnet→bufnet2. The plancheck (DestroyBeforeCreate) is the load-bearing
// proof of replacement — id is unchanged because id=name and the name is
// unchanged. ValuesSame on id confirms that property.
func TestIntegration_Topic_RequiresReplace_ClusterApiUrl(t *testing.T) {
	_, factories := integration.Setup(t)

	cfgA := mockTopicCreateConfig("tfrp-mock-topic-rr-url", "bufnet", 3, 1, "", true)
	cfgB := mockTopicCreateConfig("tfrp-mock-topic-rr-url", "bufnet2", 3, 1, "", true)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgA, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(topicAddr, cfgB, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet2")),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Topic_RequiresReplace_ReplicationFactor mutates replication_factor
// 1→2. Both values satisfy the -1..5 proto-validate constraint. id stays
// the same because id=name and name is unchanged; the plancheck is the
// load-bearing proof of replacement.
func TestIntegration_Topic_RequiresReplace_ReplicationFactor(t *testing.T) {
	_, factories := integration.Setup(t)

	cfgA := mockTopicCreateConfig("tfrp-mock-topic-rr-rf", "bufnet", 3, 1, "", true)
	cfgB := mockTopicCreateConfig("tfrp-mock-topic-rr-rf", "bufnet", 3, 2, "", true)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgA, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replication_factor"), knownvalue.NumberExact(bigFloat(1))),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(topicAddr, cfgB, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replication_factor"), knownvalue.NumberExact(bigFloat(2))),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Topic_RequiresReplaceIf_PartitionCount_Shrink is THE canonical
// positive-branch test for the partitionRequiresReplaceWhenShrinking
// predicate. Step 1 creates with partition_count=5; Step 2 reduces to 3.
// The predicate fires (plan<state) → DestroyBeforeCreate. id stays the
// same because id=name and name is unchanged; the PreApply plancheck is
// the sole load-bearing proof of replacement.
//
// Two-axis mutation discipline (verified manually before commit):
//
//	(a) Flipping plancheck.ResourceActionDestroyBeforeCreate to
//	    ResourceActionUpdate fails the test — confirming the predicate
//	    truly fires on shrink.
//	(b) Flipping the step-2 partition_count from 3 to 7 (a grow relative
//	    to state=5) also fails the DestroyBeforeCreate assertion —
//	    confirming the assertion is keyed on the partition_count
//	    direction, not the resource-action keyword.
func TestIntegration_Topic_RequiresReplaceIf_PartitionCount_Shrink(t *testing.T) {
	_, factories := integration.Setup(t)

	cfgA := mockTopicCreateConfig("tfrp-mock-topic-rr-shrink", "bufnet", 5, 1, "", true)
	cfgB := mockTopicCreateConfig("tfrp-mock-topic-rr-shrink", "bufnet", 3, 1, "", true)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgA, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("partition_count"), knownvalue.NumberExact(bigFloat(5))),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceIfStep(topicAddr, cfgB, true, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("partition_count"), knownvalue.NumberExact(bigFloat(3))),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Topic_RequiresReplaceIf_PartitionCount_Grow is THE canonical
// negative-branch test for the partitionRequiresReplaceWhenShrinking
// predicate. Step 1 creates with partition_count=3; Step 2 grows to 5.
// The predicate does NOT fire (plan>state) → ResourceActionUpdate
// (in-place). The Update path calls SetTopicPartitions on the fake.
//
// Two-axis mutation discipline (verified manually before commit):
//
//	(a) Flipping plancheck.ResourceActionUpdate to
//	    ResourceActionDestroyBeforeCreate fails the test — confirming
//	    the predicate truly does not fire on grow.
//	(b) Flipping the step-2 partition_count from 5 to 1 (a shrink
//	    relative to state=3) also fails the ResourceActionUpdate
//	    assertion — confirming the assertion is keyed on the
//	    partition_count direction, not the resource-action keyword.
func TestIntegration_Topic_RequiresReplaceIf_PartitionCount_Grow(t *testing.T) {
	_, factories := integration.Setup(t)

	cfgA := mockTopicCreateConfig("tfrp-mock-topic-rr-grow", "bufnet", 3, 1, "", true)
	cfgB := mockTopicCreateConfig("tfrp-mock-topic-rr-grow", "bufnet", 5, 1, "", true)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgA, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("partition_count"), knownvalue.NumberExact(bigFloat(3))),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceIfStep(topicAddr, cfgB, false, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("partition_count"), knownvalue.NumberExact(bigFloat(5))),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Topic_CreateAndRefresh_ReplicaAssignments exercises the
// replica_assignments variant. partition_count and replication_factor are
// pinned to -1 as required by the proto-validate mutual-exclusion rule.
// Two replica_assignments entries cover (partition_id, replica_ids).
//
// The TopicFake does not store replica_assignments (the topicRecord struct
// has no RA field). The Flatten prev-state guard at conv_gen.go:65 restores
// ReplicaAssignments from the plan model on Create and from prior state on
// Read, which is what allows this scenario to pass without fake changes.
func TestIntegration_Topic_CreateAndRefresh_ReplicaAssignments(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := mockTopicReplicaAssignmentsConfig("tfrp-mock-topic-ra", []topicRAEntry{
		{partitionID: 0, replicaIDs: []int{1, 2}},
		{partitionID: 1, replicaIDs: []int{1, 2}},
	})

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	checks := func() []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-topic-ra")),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("partition_count"), knownvalue.NumberExact(bigFloat(-1))),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replication_factor"), knownvalue.NumberExact(bigFloat(-1))),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replica_assignments"), knownvalue.ListSizeExact(2)),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replica_assignments").AtSliceIndex(0).AtMapKey("partition_id"), knownvalue.Int32Exact(0)),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replica_assignments").AtSliceIndex(0).AtMapKey("replica_ids"), knownvalue.ListExact([]knownvalue.Check{
				knownvalue.Int32Exact(1),
				knownvalue.Int32Exact(2),
			})),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replica_assignments").AtSliceIndex(1).AtMapKey("partition_id"), knownvalue.Int32Exact(1)),
			statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("id"), knownvalue.StringExact("tfrp-mock-topic-ra")),
			idPreserved.AddStateValue(topicAddr, tfjsonpath.New("id")),
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfg, checks()),
			integration.NoopReapplyStep(topicAddr, cfg, checks()),
		},
	})
}

// TestIntegration_Topic_RequiresReplace_ReplicaAssignments mutates a
// replica_assignments entry's replica_ids. The ListNestedAttribute carries
// RequiresReplace, so any change triggers DestroyBeforeCreate. id is
// unchanged (id=name, name unchanged); the plancheck is the proof.
func TestIntegration_Topic_RequiresReplace_ReplicaAssignments(t *testing.T) {
	_, factories := integration.Setup(t)

	cfgA := mockTopicReplicaAssignmentsConfig("tfrp-mock-topic-rr-ra", []topicRAEntry{
		{partitionID: 0, replicaIDs: []int{1, 2}},
	})
	cfgB := mockTopicReplicaAssignmentsConfig("tfrp-mock-topic-rr-ra", []topicRAEntry{
		{partitionID: 0, replicaIDs: []int{1, 3}},
	})

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgA, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replica_assignments").AtSliceIndex(0).AtMapKey("replica_ids"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.Int32Exact(1),
					knownvalue.Int32Exact(2),
				})),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(topicAddr, cfgB, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("replica_assignments").AtSliceIndex(0).AtMapKey("replica_ids"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.Int32Exact(1),
					knownvalue.Int32Exact(3),
				})),
				idStable.AddStateValue(topicAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Topic_ImportRoundTrip verifies that topic's ImportState correctly
// reconstructs resource state when the cluster_id resolves through the
// bufconn-backed ClusterFake.
func TestIntegration_Topic_ImportRoundTrip(t *testing.T) {
	srv, factories := integration.Setup(t)
	srv.Cluster.SetClusterByID("cgg5hmkar1m4l0pjg6tg", "bufnet")

	cfg := mockTopicCreateConfig("tfrp-mock-topic-import", "bufnet", 3, 1, "", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfg, nil),
			integration.ImportRoundTripStep(topicAddr, func(s *terraform.State) (string, error) {
				rs, ok := s.RootModule().Resources[topicAddr]
				if !ok {
					return "", fmt.Errorf("resource %q not found in state", topicAddr)
				}
				return rs.Primary.Attributes["name"] + ",cgg5hmkar1m4l0pjg6tg", nil
			}, []string{"allow_deletion"}), // ImportState hardcodes allow_deletion=false; pre-import is true.
		},
	})
}

// TestIntegration_Topic_ErrorPath_GetNotFound proves that a NotFound on the
// post-create Refresh causes the provider to remove the resource from
// state and re-plan a Create. After Create, the topic is deleted out-of-band
// directly from the fake's store via srv.Topic.DeleteTopic. The next plan's
// Refresh calls utils.FindTopicByName — confirmed at redpanda/utils/utils.go:452
// to issue client.ListTopics with a name-contains filter — which returns an
// empty list and FindTopicByName returns a NotFound error. The provider's
// Read routes that error through utils.HandleGracefulRemoval, which
// recognizes NotFound and removes the resource from state regardless of
// allow_deletion. TF then plans a re-create.
func TestIntegration_Topic_ErrorPath_GetNotFound(t *testing.T) {
	srv, factories := integration.Setup(t)
	const name = "tfrp-mock-topic-notfound"
	cfg := mockTopicCreateConfig(name, "bufnet", 3, 1, "", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
			}),
			{
				PreConfig: func() {
					if _, err := srv.Topic.DeleteTopic(context.Background(),
						&dataplanev1.DeleteTopicRequest{TopicName: name}); err != nil {
						t.Fatalf("PreConfig: delete topic %q from fake: %v", name, err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(topicAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(topicAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				},
			},
		},
	})
}

// TestIntegration_Topic_ErrorPath_CreateAlreadyExists injects an AlreadyExists
// gRPC status on CreateTopic with a "TOPIC_ALREADY_EXISTS" message. The
// provider's isAlreadyExistsError matches that prefix and routes to a
// targeted diagnostic with the text "already exists". The gRPC message
// and the diagnostic regex differ here, so we don't use ErrorPathStep
// (which injects the regex string as the gRPC message); instead we hand-
// wire OverrideOnce + ExpectError.
func TestIntegration_Topic_ErrorPath_CreateAlreadyExists(t *testing.T) {
	srv, factories := integration.Setup(t)
	cfg := mockTopicCreateConfig("tfrp-mock-topic-exists", "bufnet", 3, 1, "", true)

	srv.OverrideOnce(dataplanev1grpc.TopicService_CreateTopic_FullMethodName,
		status.Error(codes.AlreadyExists, "TOPIC_ALREADY_EXISTS: tfrp-mock-topic-exists"))

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile("already exists"),
			},
		},
	})
}

// TestIntegration_Topic_ErrorPath_UpdateFailed injects Internal on
// SetTopicConfigurations. Internal is non-retryable per
// utils.Retry's isTransientBrokerError guard, so a single override
// consumes and the error surfaces as a TF diagnostic.
func TestIntegration_Topic_ErrorPath_UpdateFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-topic-upd-fail"
	cfgEmpty := mockTopicCreateConfig(name, "bufnet", 3, 1, "", true)
	cfgWithCfg := mockTopicCreateConfig(name, "bufnet", 3, 1, "  configuration = {\n    \"retention.ms\" = \"86400000\"\n  }\n", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfgEmpty, nil),
			{
				PreConfig: func() {
					srv.OverrideOnce(dataplanev1grpc.TopicService_SetTopicConfigurations_FullMethodName,
						status.Error(codes.Internal, "synthetic update failure"))
				},
				Config:      cfgWithCfg,
				ExpectError: regexp.MustCompile("synthetic update failure"),
			},
		},
	})
}

// TestIntegration_Topic_ErrorPath_DeleteFailed injects Internal on DeleteTopic.
// The provider routes the DeleteTopic error through
// utils.HandleGracefulRemoval, which swallows NotFound /
// ClusterUnreachable / PermissionDenied but surfaces all other errors —
// codes.Internal is in the surface set.
func TestIntegration_Topic_ErrorPath_DeleteFailed(t *testing.T) {
	srv, factories := integration.Setup(t)
	cfg := mockTopicCreateConfig("tfrp-mock-topic-del-fail", "bufnet", 3, 1, "", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(topicAddr, cfg, nil),
			{
				PreConfig: func() {
					srv.OverrideOnce(dataplanev1grpc.TopicService_DeleteTopic_FullMethodName,
						status.Error(codes.Internal, "synthetic delete failure"))
				},
				Config:      cfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile("synthetic delete failure"),
			},
		},
	})
}

// TestIntegration_Topic_UpgradeState_NormalizesClusterApiUrl drives the v0->v1
// state upgrade through the provider server's UpgradeResourceState RPC and
// asserts the legacy host:443 cluster_api_url is rewritten to https://host so
// the format change alone no longer forces replacement.
func TestIntegration_Topic_UpgradeState_NormalizesClusterApiUrl(t *testing.T) {
	_, factories := integration.Setup(t)
	ctx := context.Background()
	schemaType := topic.ResourceTopicSchema(ctx).Type().TerraformType(ctx)

	const priorState = `{` +
		`"allow_deletion":true,` +
		`"cluster_api_url":"bufnet:443",` +
		`"configuration":null,` +
		`"id":"app",` +
		`"name":"app",` +
		`"partition_count":null,` +
		`"replica_assignments":null,` +
		`"replication_factor":null` +
		`}`

	upgraded := integration.UpgradeState(t, factories, "redpanda_topic", 0, priorState, schemaType)

	var obj map[string]tftypes.Value
	require.NoError(t, upgraded.As(&obj))
	var got string
	require.NoError(t, obj["cluster_api_url"].As(&got))
	assert.Equal(t, "https://bufnet", got)
}
