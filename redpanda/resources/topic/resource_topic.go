// Copyright 2023 Redpanda Data, Inc.
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

// Package topic contains the implementation of the Topic resource following the Terraform framework interfaces.
package topic

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/numberplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	topicmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/topic"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"golang.org/x/oauth2"
)

var (
	_ resource.Resource                 = &Topic{}
	_ resource.ResourceWithConfigure    = &Topic{}
	_ resource.ResourceWithImportState  = &Topic{}
	_ resource.ResourceWithUpgradeState = &Topic{}
)

// ServiceClientFactory is a function type for creating topic service clients.
// This allows dependency injection for testing.
type ServiceClientFactory func(clusterURL string, ts oauth2.TokenSource, providerVersion, terraformVersion string) (dataplanev1grpc.TopicServiceClient, error)

// Topic represents the Topic Terraform resource.
type Topic struct {
	base.ResourceBase

	TopicClient dataplanev1grpc.TopicServiceClient

	resData       config.Resource
	clientFactory ServiceClientFactory
}

// NewTopic constructs a Topic resource.
func NewTopic() *Topic {
	t := &Topic{}
	t.ResourceBase = base.NewResourceBase(
		"redpanda_topic",
		ResourceTopicSchema,
		func(p config.Resource) { t.resData = p },
	)
	return t
}

// UpgradeState migrates v0 state to v1, normalizing the legacy host:443
// cluster_api_url form to the canonical https://host the control plane now
// returns so the RequiresReplace plan modifier does not fire on the format
// change alone.
func (*Topic) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	prior := ResourceTopicSchema(ctx)
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &prior,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var model topicmodel.ResourceModel
				resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
				if resp.Diagnostics.HasError() {
					return
				}
				if !model.ClusterAPIURL.IsNull() && !model.ClusterAPIURL.IsUnknown() {
					model.ClusterAPIURL = types.StringValue(utils.NormalizeClusterAPIURL(model.ClusterAPIURL.ValueString()))
				}
				resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
			},
		},
	}
}

// Create creates a Topic resource.
func (t *Topic) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan topicmodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	cfg, err := utils.MapToCreateTopicConfiguration(plan.Configuration)
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to parse topic configuration for %s", plan.Name), utils.DeserializeGrpcError(err))
		return
	}
	if err := t.createTopicClient(ctx, plan.ClusterAPIURL.ValueString()); err != nil {
		response.Diagnostics.AddError("failed to create topic client", utils.DeserializeGrpcError(err))
		return
	}

	// Generator-emitted Expand builds the request envelope from the plan.
	// `configs` is excluded from the schema (config CRUD goes through the
	// separate SetTopicConfigurations / GetTopicConfigurations RPCs), so we
	// splice in the parsed configs after Expand.
	req, expandDiags := topicmodel.ExpandCreate(ctx, &plan)
	response.Diagnostics.Append(expandDiags...)
	if response.Diagnostics.HasError() {
		return
	}
	req.Topic.Configs = cfg

	topicName := plan.Name.ValueString()
	topic, err := utils.DataplaneCall(ctx,
		func(ctx context.Context) (*dataplanev1.CreateTopicResponse, error) {
			return t.TopicClient.CreateTopic(ctx, req)
		},
		// The broker may have created the topic before failing. Checking before
		// a retry avoids orphaning it. DataplaneCall only consults this from the
		// second attempt on, so a topic that predates this resource still fails
		// with the import hint below rather than being adopted.
		utils.WithProbe(func(ctx context.Context) (*dataplanev1.CreateTopicResponse, bool) {
			_, findErr := utils.FindTopicByName(ctx, topicName, t.TopicClient)
			return nil, findErr == nil
		}),
	)
	if err != nil {
		if isAlreadyExistsError(err) {
			response.Diagnostics.AddError(
				fmt.Sprintf("Failed to create topic; topic %q already exists", plan.Name.ValueString()),
				"Topic resource can be imported using 'terraform import redpanda_topic.<resource_name> <topic_name>,<cluster_id>'",
			)
			return
		}
		response.Diagnostics.AddError(fmt.Sprintf("failed to create topic %q", plan.Name.ValueString()), utils.DeserializeGrpcError(err))
		return
	}

	// Resolve final partition/replication numbers. CreateTopic returns them
	// directly on success; on a retry after a transient broker error topic
	// is nil but FindTopicByName confirmed the topic exists, so read it back.
	flat, err := t.flattenInputAfterCreate(ctx, topic, topicName)
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to read topic %q after create", topicName), utils.DeserializeGrpcError(err))
		return
	}
	state, flatDiags := topicmodel.Flatten(ctx, flat, &plan)
	response.Diagnostics.Append(flatDiags...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, state)...)
	if response.Diagnostics.HasError() {
		return
	}

	// Configuration sync — separate Get-after-Create RPC, then update state.
	tpCfgRes, err := utils.DataplaneCall(ctx, func(ctx context.Context) (*dataplanev1.GetTopicConfigurationsResponse, error) {
		return t.TopicClient.GetTopicConfigurations(ctx, &dataplanev1.GetTopicConfigurationsRequest{TopicName: state.Name.ValueString()})
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to retrieve %q topic configuration", state.Name.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	tpCfg := mergeWithPlannedConfig(filterDynamicConfig(tpCfgRes.Configurations), tpCfgRes.Configurations, plan.Configuration)
	tpCfgMap, err := utils.TopicConfigurationToMap(tpCfg)
	if err != nil {
		response.Diagnostics.AddError("unable to parse the topic configuration", utils.DeserializeGrpcError(err))
		return
	}
	state.Configuration = tpCfgMap
	response.Diagnostics.Append(response.State.Set(ctx, state)...)
}

// Read reads the state of the Topic resource.
func (t *Topic) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model topicmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)

	if model.ClusterAPIURL.IsNull() || model.ClusterAPIURL.IsUnknown() || model.ClusterAPIURL.ValueString() == "" {
		response.State.RemoveResource(ctx)
		return
	}

	topicName := model.Name.ValueString()

	if err := t.createTopicClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "topic", topicName, model.AllowDeletion, err, "create topic client")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	tp, err := utils.DataplaneCall(ctx, func(ctx context.Context) (*dataplanev1.ListTopicsResponse_Topic, error) {
		return utils.FindTopicByName(ctx, topicName, t.TopicClient)
	})
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "topic", topicName, model.AllowDeletion, err, "find topic")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}
	tpCfgRes, err := utils.DataplaneCall(ctx, func(ctx context.Context) (*dataplanev1.GetTopicConfigurationsResponse, error) {
		return t.TopicClient.GetTopicConfigurations(ctx, &dataplanev1.GetTopicConfigurationsRequest{TopicName: tp.Name})
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to retrieve %q topic configuration", tp.Name), utils.DeserializeGrpcError(err))
		return
	}
	tpCfg := mergeWithPlannedConfig(filterDynamicConfig(tpCfgRes.Configurations), tpCfgRes.Configurations, model.Configuration)
	topicCfg, err := utils.TopicConfigurationToMap(tpCfg)
	if err != nil {
		response.Diagnostics.AddError("unable to parse the topic configuration", utils.DeserializeGrpcError(err))
		return
	}
	state, flatDiags := topicmodel.Flatten(ctx, listTopicToFlattenInput(tp), &model)
	response.Diagnostics.Append(flatDiags...)
	if response.Diagnostics.HasError() {
		return
	}
	state.Configuration = topicCfg
	response.Diagnostics.Append(response.State.Set(ctx, state)...)
}

// Update updates the state of the Topic resource.
func (t *Topic) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan, state topicmodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	err := t.createTopicClient(ctx, plan.ClusterAPIURL.ValueString())
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", utils.DeserializeGrpcError(err))
		return
	}

	if !plan.Configuration.Equal(state.Configuration) {
		cfgToSet, err := utils.MapToSetTopicConfiguration(plan.Configuration)
		if err != nil {
			response.Diagnostics.AddError("unable to parse the plan topic configuration", utils.DeserializeGrpcError(err))
			return
		}
		_, err = utils.DataplaneCall(ctx, func(ctx context.Context) (*dataplanev1.SetTopicConfigurationsResponse, error) {
			return t.TopicClient.SetTopicConfigurations(ctx, &dataplanev1.SetTopicConfigurationsRequest{
				TopicName:      plan.Name.ValueString(),
				Configurations: cfgToSet,
			})
		})
		if err != nil {
			response.Diagnostics.AddError("failed to update topic configuration", utils.DeserializeGrpcError(err))
			return
		}
	}
	to := plan.PartitionCount.ValueBigFloat()
	from := state.PartitionCount.ValueBigFloat()

	if to.Cmp(from) > 0 {
		_, err = utils.DataplaneCall(ctx, func(ctx context.Context) (*dataplanev1.SetTopicPartitionsResponse, error) {
			return t.TopicClient.SetTopicPartitions(ctx, &dataplanev1.SetTopicPartitionsRequest{
				TopicName:      plan.Name.ValueString(),
				PartitionCount: *utils.NumberToInt32(plan.PartitionCount),
			})
		})
		if err != nil {
			response.Diagnostics.AddError("failed to update partition count", utils.DeserializeGrpcError(err))
			return
		}
	}

	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	tpCfgRes, err := utils.DataplaneCall(ctx, func(ctx context.Context) (*dataplanev1.GetTopicConfigurationsResponse, error) {
		return t.TopicClient.GetTopicConfigurations(ctx, &dataplanev1.GetTopicConfigurationsRequest{TopicName: plan.Name.ValueString()})
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to retrieve %q topic configuration after update", plan.Name.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	tpCfg := mergeWithPlannedConfig(filterDynamicConfig(tpCfgRes.Configurations), tpCfgRes.Configurations, plan.Configuration)
	tpCfgMap, err := utils.TopicConfigurationToMap(tpCfg)
	if err != nil {
		response.Diagnostics.AddError("unable to parse the topic configuration", utils.DeserializeGrpcError(err))
		return
	}
	plan.Configuration = tpCfgMap
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

// Delete deletes the Topic resource.
func (t *Topic) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model topicmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)

	topicName := model.Name.ValueString()

	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		response.Diagnostics.AddError(fmt.Sprintf("topic %s does not allow deletion", topicName), "allow_deletion is set to false")
		return
	}
	if err := t.createTopicClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "topic", topicName, model.AllowDeletion, err, "create topic client")
		response.Diagnostics.Append(diags...)
		return
	}

	delReq, expandDiags := topicmodel.ExpandDelete(ctx, &model)
	response.Diagnostics.Append(expandDiags...)
	if response.Diagnostics.HasError() {
		return
	}

	_, err := utils.DataplaneCall(ctx, func(ctx context.Context) (*dataplanev1.DeleteTopicResponse, error) {
		resp, delErr := t.TopicClient.DeleteTopic(ctx, delReq)
		// A retry may see the topic already gone, which means the earlier
		// attempt landed.
		if isNotFoundError(delErr) {
			return resp, nil
		}
		return resp, delErr
	})
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "topic", topicName, model.AllowDeletion, err, "delete topic")
		response.Diagnostics.Append(diags...)
		return
	}
}

// ImportState imports the state of the Topic resource.
func (t *Topic) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	topicName, clusterID, ok := utils.SplitImportID(req.ID, ",")
	if !ok {
		resp.Diagnostics.AddError(fmt.Sprintf("wrong ID format: %v", req.ID), "ID format is <topic_name>,<cluster_id>")
		return
	}
	dataplaneURL, err := t.CpCl.DataplaneURLForCluster(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("failed to resolve dataplane URL for cluster %q; make sure ID format is <topic_name>,<cluster_id>", clusterID),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(topicName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(topicName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), types.StringValue(dataplaneURL))...)
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceTopicSchema(ctx), &resp.State, "allow_deletion")...)
}

func (t *Topic) createTopicClient(ctx context.Context, clusterURL string) error {
	if t.TopicClient != nil {
		return nil
	}
	if t.clientFactory != nil {
		client, err := t.clientFactory(clusterURL, t.resData.TokenSource, t.resData.ProviderVersion, t.resData.TerraformVersion)
		if err != nil {
			return fmt.Errorf("unable to open a connection with the cluster API: %v", utils.DeserializeGrpcError(err))
		}
		t.TopicClient = client
		return nil
	}
	client, err := utils.NewDataplaneClient(ctx, t.resData.DataplaneConnPool, clusterURL, dataplanev1grpc.NewTopicServiceClient)
	if err != nil {
		return err
	}
	t.TopicClient = client
	return nil
}

// flattenInputAfterCreate normalizes the post-create proto state into a
// *CreateTopicRequest_Topic — the type the generated Flatten consumes.
// Uses the CreateTopic response when available; otherwise re-reads the
// topic via FindTopicByName (the CreateTopic call may have succeeded on
// the server but failed the client retry).
func (t *Topic) flattenInputAfterCreate(ctx context.Context, topic *dataplanev1.CreateTopicResponse, topicName string) (*dataplanev1.CreateTopicRequest_Topic, error) {
	if topic != nil {
		pc := topic.GetPartitionCount()
		rf := topic.GetReplicationFactor()
		return &dataplanev1.CreateTopicRequest_Topic{
			Name:              topic.GetTopicName(),
			PartitionCount:    &pc,
			ReplicationFactor: &rf,
		}, nil
	}
	tp, err := utils.DataplaneCall(ctx, func(ctx context.Context) (*dataplanev1.ListTopicsResponse_Topic, error) {
		return utils.FindTopicByName(ctx, topicName, t.TopicClient)
	})
	if err != nil {
		return nil, err
	}
	return listTopicToFlattenInput(tp), nil
}

// listTopicToFlattenInput bridges ListTopicsResponse_Topic (what
// FindTopicByName returns) into the *CreateTopicRequest_Topic shape that
// the generated Flatten expects. The two types share Name +
// PartitionCount + ReplicationFactor; replica_assignments is preserved
// from prev via flatten_skip in schema.yaml.
func listTopicToFlattenInput(tp *dataplanev1.ListTopicsResponse_Topic) *dataplanev1.CreateTopicRequest_Topic {
	pc := tp.GetPartitionCount()
	rf := tp.GetReplicationFactor()
	return &dataplanev1.CreateTopicRequest_Topic{
		Name:              tp.GetName(),
		PartitionCount:    &pc,
		ReplicationFactor: &rf,
	}
}

// filterDynamicConfig filters the configs and returns only the one with a
// DYNAMIC_TOPIC_CONFIG source.
func filterDynamicConfig(configs []*dataplanev1.Topic_Configuration) []*dataplanev1.Topic_Configuration {
	var filtered []*dataplanev1.Topic_Configuration
	for _, cfg := range configs {
		if cfg != nil {
			if cfg.Source == dataplanev1.ConfigSource_CONFIG_SOURCE_DYNAMIC_TOPIC_CONFIG {
				filtered = append(filtered, cfg)
			}
		}
	}
	return filtered
}

// brokerNoopConfigs mirrors Redpanda's allowlist_topic_noop_confs: topic
// configs the broker accepts, ignores, and may echo back with a normalized
// value (e.g. max.compaction.lag.ms = LONG_MAX clamped to the max chrono
// duration in ms). For these keys mergeWithPlannedConfig reports the user's
// planned value so Terraform doesn't raise "inconsistent result after apply".
//
// Source (SHA-pinned): https://github.com/redpanda-data/redpanda/blob/62cf52867a72dd291a35b8728794ac5800d4b4b8/src/v/kafka/server/handlers/topics/types.h#L128-L147
var brokerNoopConfigs = []string{
	"unclean.leader.election.enable",
	"message.downconversion.enable",
	"segment.index.bytes",
	"segment.jitter.ms",
	"min.insync.replicas",
	"min.compaction.lag.ms",
	"message.timestamp.difference.max.ms",
	"message.format.version",
	"max.compaction.lag.ms",
	"leader.replication.throttled.replicas",
	"index.interval.bytes",
	"follower.replication.throttled.replicas",
	"flush.messages",
	"file.delete.delay.ms",
	"preallocate",
}

// mergeWithPlannedConfig ensures that any configuration keys the user
// explicitly set in their Terraform config are preserved in the result, even
// if the server reports them with a non-dynamic source (e.g. when the user-set
// value matches the server default). Without this, Terraform sees the key
// "vanish" and reports an inconsistent result after apply.
//
// Also strips server-injected `redpanda.*` config keys the user did not name
// in their plan. After v26.1.1, the broker injects redpanda.storage.mode =
// "unset" on every topic; left in state, plan-twice would try to remove the
// key and the server rejects (the property has no null representation, only
// local/tiered/cloud/unset). Same shape as tagsFromProto in
// redpanda/models/cluster/conv.go.
func mergeWithPlannedConfig(dynamicConfigs, allConfigs []*dataplanev1.Topic_Configuration, planned types.Map) []*dataplanev1.Topic_Configuration {
	plannedKeys := make(map[string]bool, len(planned.Elements()))
	if !planned.IsNull() && !planned.IsUnknown() {
		for key := range planned.Elements() {
			plannedKeys[key] = true
		}
	}

	filtered := make([]*dataplanev1.Topic_Configuration, 0, len(dynamicConfigs))
	for _, cfg := range dynamicConfigs {
		if cfg == nil {
			continue
		}
		if strings.HasPrefix(cfg.Name, "redpanda.") && !plannedKeys[cfg.Name] {
			continue
		}
		filtered = append(filtered, cfg)
	}

	if len(plannedKeys) == 0 {
		return filtered
	}

	present := make(map[string]bool, len(filtered))
	for _, cfg := range filtered {
		present[cfg.Name] = true
	}

	for key, planVal := range planned.Elements() {
		if present[key] {
			continue
		}
		var matched bool
		for _, cfg := range allConfigs {
			if cfg != nil && cfg.Name == key {
				filtered = append(filtered, cfg)
				present[key] = true
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// Broker didn't echo the key in either the dynamic or full config
		// response (some keys like min.insync.replicas are server-silent on
		// reflection). Synthesize a topic-config entry from the plan so the
		// state contains what the user asked for — without this Terraform
		// reports "element has vanished from configurations".
		planString, ok := planVal.(types.String)
		if !ok || planString.IsNull() || planString.IsUnknown() {
			continue
		}
		val := planString.ValueString()
		filtered = append(filtered, &dataplanev1.Topic_Configuration{
			Name:   key,
			Value:  &val,
			Source: dataplanev1.ConfigSource_CONFIG_SOURCE_DYNAMIC_TOPIC_CONFIG,
		})
		present[key] = true
	}

	// prevent inconsistency by keeping broker noop configs to user value
	for i, cfg := range filtered {
		if cfg == nil {
			continue
		}
		if !slices.Contains(brokerNoopConfigs, cfg.Name) {
			continue
		}
		planString, ok := planned.Elements()[cfg.Name].(types.String)
		if !ok || planString.IsNull() || planString.IsUnknown() {
			continue
		}
		val := planString.ValueString()
		if cfg.Value != nil && *cfg.Value == val {
			continue
		}
		filtered[i] = &dataplanev1.Topic_Configuration{
			Name:   cfg.Name,
			Value:  &val,
			Source: cfg.Source,
		}
	}
	return filtered
}

func isAlreadyExistsError(err error) bool {
	return strings.Contains(utils.DeserializeGrpcError(err), "TOPIC_ALREADY_EXISTS") || strings.Contains(utils.DeserializeGrpcError(err), "The topic has already been created")
}

// isTransientBrokerError defers to the shared dataplane classifier, which owns
// the broker-churn tokens this used to match on its own.
func isTransientBrokerError(err error) bool {
	return utils.IsTransientDataplaneError(err)
}

func isNotFoundError(err error) bool {
	if utils.IsNotFound(err) {
		return true
	}
	// Topic-specific server codes that IsNotFound's generic tokens don't cover.
	msg := utils.DeserializeGrpcError(err)
	return strings.Contains(msg, "NOT_FOUND") ||
		strings.Contains(msg, "TOPIC_DOES_NOT_EXIST")
}

// partitionRequiresReplaceWhenShrinking is the RequiresReplaceIf predicate
// referenced by the generated schema's partition_count plan modifier.
// Recreating the topic is the only way to reduce its partition count in
// Kafka, so a plan-time decrease triggers replacement (and a warning).
func partitionRequiresReplaceWhenShrinking(_ context.Context, req planmodifier.NumberRequest, resp *numberplanmodifier.RequiresReplaceIfFuncResponse) {
	if !req.PlanValue.IsNull() && !req.StateValue.IsNull() {
		to := req.PlanValue.ValueBigFloat()
		from := req.StateValue.ValueBigFloat()
		if to.Cmp(from) < 0 {
			resp.RequiresReplace = true
			resp.Diagnostics.AddWarning("Partition count decrease detected", "Decreasing partition count requires recreating the topic")
		}
	}
}
