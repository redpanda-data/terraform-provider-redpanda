// Copyright 2023 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

// Package topic contains the implementation of the Topic resource following the Terraform framework interfaces.
package topic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
)

var (
	_ resource.Resource                = &Topic{}
	_ resource.ResourceWithConfigure   = &Topic{}
	_ resource.ResourceWithImportState = &Topic{}
)

// ServiceClientFactory is a function type for creating topic service clients.
// This allows dependency injection for testing.
type ServiceClientFactory func(clusterURL, authToken, providerVersion, terraformVersion string) (dataplanev1grpc.TopicServiceClient, error)

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
	if err := t.createTopicClient(plan.ClusterAPIURL.ValueString()); err != nil {
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
	var topic *dataplanev1.CreateTopicResponse
	err = utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
		var createErr error
		topic, createErr = t.TopicClient.CreateTopic(ctx, req)
		if createErr != nil {
			if isAlreadyExistsError(createErr) {
				return utils.NonRetryableError(createErr)
			}
			if isTransientBrokerError(createErr) {
				// The broker may have created the topic before the error.
				// Check before retrying to avoid orphans.
				if _, findErr := utils.FindTopicByName(ctx, topicName, t.TopicClient); findErr == nil {
					return nil
				}
				return utils.RetryableError(fmt.Errorf("transient broker error, retrying: %w", createErr))
			}
			return utils.NonRetryableError(createErr)
		}
		return nil
	})
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
	var tpCfgRes *dataplanev1.GetTopicConfigurationsResponse
	err = utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
		var cfgErr error
		tpCfgRes, cfgErr = t.TopicClient.GetTopicConfigurations(ctx, &dataplanev1.GetTopicConfigurationsRequest{TopicName: state.Name.ValueString()})
		if cfgErr != nil {
			if isTransientBrokerError(cfgErr) {
				return utils.RetryableError(cfgErr)
			}
			return utils.NonRetryableError(cfgErr)
		}
		return nil
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

	if err := t.createTopicClient(model.ClusterAPIURL.ValueString()); err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "topic", topicName, model.AllowDeletion, err, "create topic client")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	var tp *dataplanev1.ListTopicsResponse_Topic
	err := utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
		var findErr error
		tp, findErr = utils.FindTopicByName(ctx, topicName, t.TopicClient)
		if findErr != nil {
			if isTransientBrokerError(findErr) {
				return utils.RetryableError(findErr)
			}
			return utils.NonRetryableError(findErr)
		}
		return nil
	})
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "topic", topicName, model.AllowDeletion, err, "find topic")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}
	var tpCfgRes *dataplanev1.GetTopicConfigurationsResponse
	err = utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
		var cfgErr error
		tpCfgRes, cfgErr = t.TopicClient.GetTopicConfigurations(ctx, &dataplanev1.GetTopicConfigurationsRequest{TopicName: tp.Name})
		if cfgErr != nil {
			if isTransientBrokerError(cfgErr) {
				return utils.RetryableError(cfgErr)
			}
			return utils.NonRetryableError(cfgErr)
		}
		return nil
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
	err := t.createTopicClient(plan.ClusterAPIURL.ValueString())
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
		err = utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
			_, setErr := t.TopicClient.SetTopicConfigurations(ctx, &dataplanev1.SetTopicConfigurationsRequest{
				TopicName:      plan.Name.ValueString(),
				Configurations: cfgToSet,
			})
			if setErr != nil {
				if isTransientBrokerError(setErr) {
					return utils.RetryableError(setErr)
				}
				return utils.NonRetryableError(setErr)
			}
			return nil
		})
		if err != nil {
			response.Diagnostics.AddError("failed to update topic configuration", utils.DeserializeGrpcError(err))
			return
		}
	}
	to := plan.PartitionCount.ValueBigFloat()
	from := state.PartitionCount.ValueBigFloat()

	if to.Cmp(from) > 0 {
		err := utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
			_, setErr := t.TopicClient.SetTopicPartitions(ctx, &dataplanev1.SetTopicPartitionsRequest{
				TopicName:      plan.Name.ValueString(),
				PartitionCount: *utils.NumberToInt32(plan.PartitionCount),
			})
			if setErr != nil {
				if isTransientBrokerError(setErr) {
					return utils.RetryableError(setErr)
				}
				return utils.NonRetryableError(setErr)
			}
			return nil
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

	var tpCfgRes *dataplanev1.GetTopicConfigurationsResponse
	err = utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
		var cfgErr error
		tpCfgRes, cfgErr = t.TopicClient.GetTopicConfigurations(ctx, &dataplanev1.GetTopicConfigurationsRequest{TopicName: plan.Name.ValueString()})
		if cfgErr != nil {
			if isTransientBrokerError(cfgErr) {
				return utils.RetryableError(cfgErr)
			}
			return utils.NonRetryableError(cfgErr)
		}
		return nil
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
	if err := t.createTopicClient(model.ClusterAPIURL.ValueString()); err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "topic", topicName, model.AllowDeletion, err, "create topic client")
		response.Diagnostics.Append(diags...)
		return
	}

	delReq, expandDiags := topicmodel.ExpandDelete(ctx, &model)
	response.Diagnostics.Append(expandDiags...)
	if response.Diagnostics.HasError() {
		return
	}

	err := utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
		_, delErr := t.TopicClient.DeleteTopic(ctx, delReq)
		if delErr != nil {
			// A retry after a transient broker error may see the topic as
			// already gone; that means the earlier attempt succeeded.
			if isNotFoundError(delErr) {
				return nil
			}
			if isTransientBrokerError(delErr) {
				return utils.RetryableError(delErr)
			}
			return utils.NonRetryableError(delErr)
		}
		return nil
	})
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "topic", topicName, model.AllowDeletion, err, "delete topic")
		response.Diagnostics.Append(diags...)
		return
	}
}

// ImportState imports the state of the Topic resource.
func (t *Topic) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	split := strings.SplitN(req.ID, ",", 2)
	if len(split) != 2 {
		resp.Diagnostics.AddError(fmt.Sprintf("wrong ID format: %v", req.ID), "ID format is <topic_name>,<cluster_id>")
		return
	}
	topicName, clusterID := split[0], split[1]
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

func (t *Topic) createTopicClient(clusterURL string) error {
	if t.TopicClient != nil {
		return nil
	}
	if t.clientFactory != nil {
		client, err := t.clientFactory(clusterURL, t.resData.AuthToken, t.resData.ProviderVersion, t.resData.TerraformVersion)
		if err != nil {
			return fmt.Errorf("unable to open a connection with the cluster API: %v", utils.DeserializeGrpcError(err))
		}
		t.TopicClient = client
		return nil
	}
	if t.resData.DataplaneConnPool == nil {
		return errors.New("provider not configured: dataplane connection pool is nil")
	}
	conn, err := t.resData.DataplaneConnPool.GetConnection(clusterURL)
	if err != nil {
		return fmt.Errorf("unable to open a connection with the cluster API: %v", utils.DeserializeGrpcError(err))
	}
	t.TopicClient = dataplanev1grpc.NewTopicServiceClient(conn)
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
	var tp *dataplanev1.ListTopicsResponse_Topic
	if err := utils.Retry(ctx, 2*time.Minute, func() *utils.RetryError {
		var e error
		tp, e = utils.FindTopicByName(ctx, topicName, t.TopicClient)
		if e != nil {
			if isTransientBrokerError(e) {
				return utils.RetryableError(e)
			}
			return utils.NonRetryableError(e)
		}
		return nil
	}); err != nil {
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
	return filtered
}

func isAlreadyExistsError(err error) bool {
	return strings.Contains(utils.DeserializeGrpcError(err), "TOPIC_ALREADY_EXISTS") || strings.Contains(utils.DeserializeGrpcError(err), "The topic has already been created")
}

func isTransientBrokerError(err error) bool {
	msg := utils.DeserializeGrpcError(err)
	return strings.Contains(msg, "broker struct chosen to issue this request has died") ||
		strings.Contains(msg, "client closed") ||
		strings.Contains(msg, "context canceled")
}

func isNotFoundError(err error) bool {
	msg := utils.DeserializeGrpcError(err)
	return strings.Contains(msg, "NOT_FOUND") ||
		strings.Contains(msg, "TOPIC_DOES_NOT_EXIST") ||
		strings.Contains(msg, "does not exist")
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
