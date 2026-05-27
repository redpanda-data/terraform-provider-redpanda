//go:build live_test && (all || shadowlink)

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

package shadowlink_test

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

// TestAcc_ShadowLink provisions two dedicated AWS clusters and exercises redpanda_secret + redpanda_shadow_link end-to-end.
func TestAcc_ShadowLink(t *testing.T) {
	ctx := context.Background()

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	// Resource group and network names share the test prefix. Cluster names
	// must be distinct (source + shadow). Shadow link name is DNS-1123
	// (lowercase). Secret name is ^[A-Z][A-Z0-9_]*$.
	prefix := strings.ToLower(acc.RandomName(acc.NamePrefix + "shadowlink"))
	resourceGroup := prefix
	sourceName := prefix + "-src"
	shadowName := prefix + "-shd"
	linkName := prefix
	secretName := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(prefix, "-", "_"), ".", "_"))
	password := "Tfrp-Test-Password-" + prefix

	origVars := make(map[string]config.Variable)
	maps.Copy(origVars, acc.ProviderCfgIDSecretVars)
	origVars["resource_group_name"] = config.StringVariable(resourceGroup)
	origVars["source_network_name"] = config.StringVariable(sourceName)
	origVars["shadow_network_name"] = config.StringVariable(shadowName)
	origVars["source_cluster_name"] = config.StringVariable(sourceName)
	origVars["shadow_cluster_name"] = config.StringVariable(shadowName)
	origVars["user_name"] = config.StringVariable(prefix + "-user")
	origVars["user_password"] = config.StringVariable(password)
	origVars["secret_name"] = config.StringVariable(secretName)
	origVars["link_name"] = config.StringVariable(linkName)
	if acc.ThroughputTier != "" {
		origVars["throughput_tier"] = config.StringVariable(acc.ThroughputTier)
	}

	// Partial-update vars: bump only client_options.metadata_max_age_ms.
	partialUpdateVars := make(map[string]config.Variable)
	maps.Copy(partialUpdateVars, origVars)
	partialUpdateVars["metadata_max_age_ms"] = config.IntegerVariable(15000)

	// Shadow-link rename triggers RequiresReplace (delete + create) since
	// `name` is immutable on the proto.
	linkRename := strings.ToLower(acc.RandomName(acc.NamePrefix + "shadowlink-rename"))
	renameVars := make(map[string]config.Variable)
	maps.Copy(renameVars, partialUpdateVars)
	renameVars["link_name"] = config.StringVariable(linkRename)

	// Final destroy step needs cluster_allow_deletion=true. allow_deletion=false
	// on the cluster is the canary per CLAUDE.md.
	destroyVars := make(map[string]config.Variable)
	maps.Copy(destroyVars, renameVars)
	destroyVars["cluster_allow_deletion"] = config.BoolVariable(true)

	// Captured server-managed values from Step 1 used by the partial-update step
	// to assert that untouched blocks were not clobbered.
	var initialPaused bool

	acc.Register(acc.KindShadowLink, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ShadowLink{LinkName: linkName, Client: c}.SweepShadowLinks("")
	}))
	acc.Register(acc.KindShadowLink, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ShadowLink{LinkName: linkRename, Client: c}.SweepShadowLinks("")
	}))
	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: sourceName, Client: c}.SweepCluster("")
	}))
	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: shadowName, Client: c}.SweepCluster("")
	}))
	acc.Register(acc.KindNetwork, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Network{NetworkName: sourceName, Client: c}.SweepNetworks("")
	}))
	acc.Register(acc.KindNetwork, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Network{NetworkName: shadowName, Client: c}.SweepNetworks("")
	}))
	acc.Register(acc.KindResourceGroup, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ResourceGroup{ResourceGroupName: resourceGroup, Client: c}.SweepResourceGroup("")
	}))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			// Step 1: Apply, with out-of-band verification of both clusters and the link.
			{
				ConfigDirectory:          config.StaticDirectory(acc.ShadowLinkDir),
				ConfigVariables:          origVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_cluster.source", "name", sourceName),
					resource.TestCheckResourceAttr("redpanda_cluster.shadow", "name", shadowName),
					resource.TestCheckResourceAttr(acc.ShadowLinkResourceName, "name", linkName),
					resource.TestCheckResourceAttrSet(acc.ShadowLinkResourceName, "id"),
					resource.TestCheckResourceAttrSet(acc.ShadowLinkResourceName, "state"),
					resource.TestCheckResourceAttr(acc.ShadowLinkSecretResourceName, "name", secretName),
					func(s *terraform.State) error {
						sourceID, err := acc.ResourceID(s, "redpanda_cluster.source")
						if err != nil {
							return err
						}
						shadowID, err := acc.ResourceID(s, "redpanda_cluster.shadow")
						if err != nil {
							return err
						}
						linkID, err := acc.ResourceID(s, acc.ShadowLinkResourceName)
						if err != nil {
							return err
						}
						if _, err := c.ClusterForID(ctx, sourceID); err != nil {
							return fmt.Errorf("source cluster %q not found via API: %v", sourceID, err)
						}
						if _, err := c.ClusterForID(ctx, shadowID); err != nil {
							return fmt.Errorf("shadow cluster %q not found via API: %v", shadowID, err)
						}
						sl, err := c.ShadowLinkForID(ctx, linkID)
						if err != nil {
							return fmt.Errorf("shadow link %q not found via API: %v", linkID, err)
						}
						if sl.GetName() != linkName {
							return fmt.Errorf("expected shadow link name %q, got %q", linkName, sl.GetName())
						}
						if sl.GetShadowRedpandaId() != shadowID {
							return fmt.Errorf("expected shadow_redpanda_id %q, got %q", shadowID, sl.GetShadowRedpandaId())
						}
						// Capture server-managed defaults from blocks the user never set in HCL.
						// The partial-update step asserts these are unchanged after the update.
						initialPaused = sl.GetTopicMetadataSyncOptions().GetPaused()
						t.Logf("Shadow link %s active. source=%s shadow=%s state=%s tms.paused=%v",
							sl.GetId(), sourceID, shadowID, sl.GetState(), initialPaused)
						return nil
					},
				),
			},
			// Change ONLY client_options.metadata_max_age_ms.
			// Verifies the field-mask diff fix in BuildUpdateRequest:
			//   * client_options is in the mask (changed),
			//   * the other 4 sync-options blocks (which the user never set in HCL)
			//     are NOT in the mask, so server-managed defaults like
			//     topic_metadata_sync_options.paused stay intact.
			// If the always-on field-mask bug regresses, those defaults would be
			// re-sent as zero/false and clobber server state.
			{
				ConfigDirectory:          config.StaticDirectory(acc.ShadowLinkDir),
				ConfigVariables:          partialUpdateVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ShadowLinkResourceName, "client_options.metadata_max_age_ms", "15000"),
					func(s *terraform.State) error {
						linkID, err := acc.ResourceID(s, acc.ShadowLinkResourceName)
						if err != nil {
							return err
						}
						sl, err := c.ShadowLinkForID(ctx, linkID)
						if err != nil {
							return fmt.Errorf("shadow link %q not found via API: %v", linkID, err)
						}
						if got := sl.GetClientOptions().GetMetadataMaxAgeMs(); got != 15000 {
							return fmt.Errorf("expected client_options.metadata_max_age_ms=15000 server-side, got %d", got)
						}
						if got := sl.GetTopicMetadataSyncOptions().GetPaused(); got != initialPaused {
							return fmt.Errorf("topic_metadata_sync_options.paused was clobbered by partial update: before=%v after=%v", initialPaused, got)
						}
						return nil
					},
				),
			},
			// Step 4: Rename the link to exercise RequiresReplace without rebuilding clusters.
			{
				ConfigDirectory:          config.StaticDirectory(acc.ShadowLinkDir),
				ConfigVariables:          renameVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ShadowLinkResourceName, "name", linkRename),
				),
			},
			// Step 5: Import the renamed link; ignore input-only and write-only attrs.
			{
				ResourceName:             acc.ShadowLinkResourceName,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  []string{"allow_deletion", "client_options", "source_redpanda_id"},
				ConfigDirectory:          config.StaticDirectory(acc.ShadowLinkDir),
				ConfigVariables:          renameVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
			// Step 6: Flip cluster_allow_deletion=true so step 7 can tear down.
			{
				ConfigDirectory:          config.StaticDirectory(acc.ShadowLinkDir),
				ConfigVariables:          destroyVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_cluster.source", "allow_deletion", "true"),
					resource.TestCheckResourceAttr("redpanda_cluster.shadow", "allow_deletion", "true"),
				),
			},
			// Step 7: Destroy.
			{
				ConfigDirectory:          config.StaticDirectory(acc.ShadowLinkDir),
				ConfigVariables:          destroyVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Destroy:                  true,
			},
		},
	})
}
