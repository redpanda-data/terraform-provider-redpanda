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
	"fmt"
	"strings"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Topic{}
	_ resource.ResourceWithConfigure   = &Topic{}
	_ resource.ResourceWithImportState = &Topic{}
)

// Topic represents the Topic Terraform resource.
type Topic struct {
	TopicClient dataplanev1grpc.TopicServiceClient

	resData       config.Resource
	dataplaneConn *grpc.ClientConn
}

// Configure configures the Topic resource.
func (t *Topic) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	p, ok := request.ProviderData.(config.Resource)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	t.resData = p
}

// Metadata returns the metadata for the Topic resource.
func (*Topic) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_topic"
}

// Schema returns the schema for the Topic resource.
func (*Topic) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = resourceTopicSchema()
}

// Create creates a Topic resource.
func (t *Topic) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	cfg, err := utils.MapToCreateTopicConfiguration(model.Configuration)
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to parse topic configuration for %s", model.Name), utils.DeserializeGrpcError(err))
		return
	}
	err = t.createTopicClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", utils.DeserializeGrpcError(err))
		return
	}
	defer t.dataplaneConn.Close()
	var p, rf *int32
	if !model.PartitionCount.IsUnknown() {
		p = utils.NumberToInt32(model.PartitionCount)
	}
	if !model.ReplicationFactor.IsUnknown() {
		rf = utils.NumberToInt32(model.ReplicationFactor)
	}
	topic, err := t.TopicClient.CreateTopic(ctx, &dataplanev1.CreateTopicRequest{
		Topic: &dataplanev1.CreateTopicRequest_Topic{
			Name:              model.Name.ValueString(),
			PartitionCount:    p,
			ReplicationFactor: rf,
			Configs:           cfg,
		},
	})
	if err != nil {
		if isAlreadyExistsError(err) {
			response.Diagnostics.AddError(
				fmt.Sprintf("Failed to create topic; topic %q already exists", model.Name.ValueString()),
				"Topic resource can be imported using 'terraform import redpanda_topic.<resource_name> <topic_name>,<cluster_id>'",
			)
			return
		}
		response.Diagnostics.AddError(fmt.Sprintf("failed to create topic %q", model.Name.ValueString()), utils.DeserializeGrpcError(err))
		return
	}

	tpCfgRes, err := t.TopicClient.GetTopicConfigurations(ctx, &dataplanev1.GetTopicConfigurationsRequest{TopicName: topic.GetTopicName()})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to retrieve %q topic configuration", topic.GetTopicName()), utils.DeserializeGrpcError(err))
		return
	}
	tpCfg := filterDynamicConfig(tpCfgRes.Configurations)
	tpCfgMap, err := utils.TopicConfigurationToMap(tpCfg)
	if err != nil {
		response.Diagnostics.AddError("unable to parse the topic configuration", utils.DeserializeGrpcError(err))
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, models.Topic{
		Name:              types.StringValue(topic.GetTopicName()),
		PartitionCount:    utils.Int32ToNumber(topic.GetPartitionCount()),
		ReplicationFactor: utils.Int32ToNumber(topic.GetReplicationFactor()),
		Configuration:     tpCfgMap,
		AllowDeletion:     model.AllowDeletion,
		ClusterAPIURL:     model.ClusterAPIURL,
		ID:                types.StringValue(topic.GetTopicName()),
	})...)
}

// Read reads the state of the Topic resource.
func (t *Topic) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	err := t.createTopicClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", utils.DeserializeGrpcError(err))
		return
	}
	defer t.dataplaneConn.Close()
	tp, err := utils.FindTopicByName(ctx, model.Name.ValueString(), t.TopicClient)
	if err != nil {
		if utils.IsNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError(fmt.Sprintf("failed receive response from topic api for topic %s", model.Name), utils.DeserializeGrpcError(err))
		return
	}
	tpCfgRes, err := t.TopicClient.GetTopicConfigurations(ctx, &dataplanev1.GetTopicConfigurationsRequest{TopicName: tp.Name})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to retrieve %q topic configuration", tp.Name), utils.DeserializeGrpcError(err))
		return
	}
	tpCfg := filterDynamicConfig(tpCfgRes.Configurations)
	topicCfg, err := utils.TopicConfigurationToMap(tpCfg)
	if err != nil {
		response.Diagnostics.AddError("unable to parse the topic configuration", utils.DeserializeGrpcError(err))
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, models.Topic{
		Name:              types.StringValue(tp.Name),
		PartitionCount:    utils.Int32ToNumber(tp.PartitionCount),
		ReplicationFactor: utils.Int32ToNumber(tp.ReplicationFactor),
		Configuration:     topicCfg,
		AllowDeletion:     model.AllowDeletion,
		ClusterAPIURL:     model.ClusterAPIURL,
		ID:                types.StringValue(tp.Name),
	})...)
}

// Update updates the state of the Topic resource.
func (t *Topic) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan, state models.Topic
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	err := t.createTopicClient(plan.ClusterAPIURL.ValueString())
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", utils.DeserializeGrpcError(err))
		return
	}
	defer t.dataplaneConn.Close()
	if !plan.Configuration.Equal(state.Configuration) {
		cfgToSet, err := utils.MapToSetTopicConfiguration(plan.Configuration)
		if err != nil {
			response.Diagnostics.AddError("unable to parse the plan topic configuration", utils.DeserializeGrpcError(err))
			return
		}
		_, err = t.TopicClient.SetTopicConfigurations(ctx, &dataplanev1.SetTopicConfigurationsRequest{
			TopicName:      plan.Name.ValueString(),
			Configurations: cfgToSet,
		})
		if err != nil {
			response.Diagnostics.AddError("failed to update topic configuration", utils.DeserializeGrpcError(err))
			return
		}
	}
	// ValueBigFloat returns 0.0 if the value is unknown or null
	to := plan.PartitionCount.ValueBigFloat()
	from := state.PartitionCount.ValueBigFloat()

	// we can only increase the partition count
	if to.Cmp(from) > 0 {
		_, err := t.TopicClient.SetTopicPartitions(ctx, &dataplanev1.SetTopicPartitionsRequest{
			TopicName:      plan.Name.ValueString(),
			PartitionCount: *utils.NumberToInt32(plan.PartitionCount),
		})
		if err != nil {
			response.Diagnostics.AddError("failed to update partition count", utils.DeserializeGrpcError(err))
			return
		}
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

// Delete deletes the Topic resource.
func (t *Topic) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	if !model.AllowDeletion.ValueBool() {
		response.Diagnostics.AddError(fmt.Sprintf("topic %s does not allow deletion", model.Name), "")
		return
	}
	err := t.createTopicClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", utils.DeserializeGrpcError(err))
		return
	}
	defer t.dataplaneConn.Close()
	_, err = t.TopicClient.DeleteTopic(ctx, &dataplanev1.DeleteTopicRequest{
		TopicName: model.Name.ValueString(),
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to delete topic %s", model.Name), utils.DeserializeGrpcError(err))
	}
}

// ImportState imports the state of the Topic resource.
func (t *Topic) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	split := strings.SplitN(req.ID, ",", 2)
	if len(split) != 2 {
		resp.Diagnostics.AddError(fmt.Sprintf("wrong ADDR ID format: %v", req.ID), "ADDR ID format is <topic_name>,<cluster_id>")
		return
	}
	topicName, clusterID := split[0], split[1]
	client := cloud.NewControlPlaneClientSet(t.resData.ControlPlaneConnection)
	cluster, err := client.ClusterForID(ctx, clusterID)
	var dataplaneURL string

	if err == nil && cluster != nil {
		dataplaneURL = cluster.DataplaneApi.Url
	} else {
		serverlessCluster, serr := client.ServerlessClusterForID(ctx, clusterID)
		if serr != nil || serverlessCluster == nil {
			resp.Diagnostics.AddError(fmt.Sprintf("failed to find cluster with ID %q; make sure ADDR ID format is <topic_name>,<cluster_id>", clusterID), utils.DeserializeGrpcError(err)+utils.DeserializeGrpcError(serr))
			return
		}
		dataplaneURL = serverlessCluster.DataplaneApi.Url
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(topicName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(topicName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), types.StringValue(dataplaneURL))...)
}

func (t *Topic) createTopicClient(clusterURL string) error {
	if t.TopicClient != nil { // Client already started, no need to create another one.
		return nil
	}
	if t.dataplaneConn == nil {
		conn, err := cloud.SpawnConn(clusterURL, t.resData.AuthToken, t.resData.ProviderVersion, t.resData.TerraformVersion)
		if err != nil {
			return fmt.Errorf("unable to open a connection with the cluster API: %v", utils.DeserializeGrpcError(err))
		}
		t.dataplaneConn = conn
	}
	t.TopicClient = dataplanev1grpc.NewTopicServiceClient(t.dataplaneConn)
	return nil
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

func isAlreadyExistsError(err error) bool {
	return strings.Contains(utils.DeserializeGrpcError(err), "TOPIC_ALREADY_EXISTS") || strings.Contains(utils.DeserializeGrpcError(err), "The topic has already been created")
}
