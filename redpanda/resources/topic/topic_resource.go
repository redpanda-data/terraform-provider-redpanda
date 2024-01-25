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

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	dataplanev1alpha1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/dataplane/v1alpha1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Topic{}
	_ resource.ResourceWithConfigure   = &Topic{}
	_ resource.ResourceWithImportState = &Topic{}
)

// Topic represents the Topic Terraform resource.
type Topic struct {
	TopicClient dataplanev1alpha1.TopicServiceClient

	resData utils.ResourceData
}

var sourceValidator = stringvalidator.OneOf(
	"SOURCE_UNSPECIFIED",
	"DYNAMIC_TOPIC_CONFIG",
	"DYNAMIC_BROKER_CONFIG",
	"DYNAMIC_DEFAULT_BROKER_CONFIG",
	"STATIC_BROKER_CONFIG",
	"DEFAULT_CONFIG",
	"DYNAMIC_BROKER_LOGGER_CONFIG",
)

// Configure configures the Topic resource.
func (t *Topic) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		response.Diagnostics.AddWarning("provider data not set", "provider data not set at topic.Configure")
		return
	}
	p, ok := request.ProviderData.(utils.ResourceData)
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

func resourceTopicSchema() schema.Schema {
	return schema.Schema{
		Description: "Topic represents a Kafka topic configuration",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the topic.",
				Required:    true,
			},
			"partition_count": schema.NumberAttribute{
				Description: "The number of partitions for the topic. This determines how the data is distributed across brokers.",
				Required:    true,
			},
			"replication_factor": schema.NumberAttribute{
				Description: "The replication factor for the topic, which defines how many copies of the data are kept across different brokers for fault tolerance.",
				Required:    true,
			},
			"allow_deletion": schema.BoolAttribute{
				Description: "Indicates whether the topic can be deleted.",
				Optional:    true,
			},
			"configuration": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the configuration parameter.",
							Required:    true,
						},
						"type": schema.StringAttribute{
							Description: "The type of the configuration parameter.",
							Required:    true,
						},
						"value": schema.StringAttribute{
							Description: "The value of the configuration parameter.",
							Required:    true,
						},
						"source": schema.StringAttribute{
							Description: "The source of the configuration parameter, indicating how the configuration was set.",
							Required:    true,
							Validators: []validator.String{
								sourceValidator,
							},
						},
						"is_read_only": schema.BoolAttribute{
							Description: "Indicates whether the configuration parameter is read-only.",
							Required:    true,
						},
						"is_sensitive": schema.BoolAttribute{
							Description: "Indicates whether the configuration parameter is sensitive and should be handled securely.",
							Required:    true,
						},
						"config_synonyms": schema.SetNestedAttribute{
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Description: "The synonym name for the configuration parameter.",
										Required:    true,
									},
									"value": schema.StringAttribute{
										Description: "The synonym value for the configuration parameter.",
										Required:    true,
									},
									"source": schema.StringAttribute{
										Description: "The source of the synonym, indicating how the synonym was set.",
										Required:    true,
										Validators: []validator.String{
											sourceValidator,
										},
									},
								},
							},
							Required: true,
						},
						"documentation": schema.StringAttribute{
							Description: "Documentation for the configuration parameter, providing additional context or information.",
							Required:    true,
						},
					},
				},
				Required: true,
			},
			"cluster_api_url": schema.StringAttribute{
				Required: true,
				Description: "The cluster API URL. Changing this will prevent deletion of the resource on the existing " +
					"cluster. It is generally a better idea to delete an existing resource and create a new one than to " +
					"change this value unless you are planning to do state imports",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

// Schema returns the schema for the Topic resource.
func (*Topic) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = resourceTopicSchema()
}

// Create creates a Topic resource.
func (t *Topic) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	cfg, err := utils.SliceToTopicConfiguration(model.Configuration)
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to convert topic configuration for %s", model.Name), err.Error())
		return
	}
	err = t.createTopicClient(ctx, model.ClusterAPIURL.ValueString())
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", err.Error())
		return
	}
	tp, err := t.TopicClient.CreateTopic(ctx, &dataplanev1alpha1.CreateTopicRequest{
		Topic: &dataplanev1alpha1.Topic{
			Name:              model.Name.ValueString(),
			PartitionCount:    utils.NumberToInt32(model.PartitionCount),
			ReplicationFactor: utils.NumberToInt32(model.ReplicationFactor),
			Configuration:     cfg,
		},
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to create topic %s", model.Name.ValueString()), err.Error())
	}

	response.Diagnostics.Append(response.State.Set(ctx, models.Topic{
		Name:              types.StringValue(tp.Topic.Name),
		PartitionCount:    utils.Int32ToNumber(tp.Topic.PartitionCount),
		ReplicationFactor: utils.Int32ToNumber(tp.Topic.ReplicationFactor),
		Configuration:     utils.TopicConfigurationToSlice(tp.Topic.Configuration),
		AllowDeletion:     model.AllowDeletion,
	})...)
}

// Read reads the state of the Topic resource.
func (t *Topic) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	err := t.createTopicClient(ctx, model.ClusterAPIURL.ValueString())
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", err.Error())
		return
	}
	tp, err := utils.FindTopicByName(ctx, model.Name.ValueString(), t.TopicClient)
	if err != nil {
		if utils.IsNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError(fmt.Sprintf("failed receive response from topic api for topic %s", model.Name), err.Error())
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, models.Topic{
		Name:              types.StringValue(tp.Name),
		PartitionCount:    utils.Int32ToNumber(tp.PartitionCount),
		ReplicationFactor: utils.Int32ToNumber(tp.ReplicationFactor),
		Configuration:     utils.TopicConfigurationToSlice(tp.Configuration),
		AllowDeletion:     model.AllowDeletion,
	})...)
}

// Update updates the state of the Topic resource.
func (*Topic) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete deletes the Topic resource.
func (t *Topic) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	if !model.AllowDeletion.ValueBool() {
		response.Diagnostics.AddError(fmt.Sprintf("topic %s does not allow deletion", model.Name), "")
		return
	}
	err := t.createTopicClient(ctx, model.ClusterAPIURL.ValueString())
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", err.Error())
		return
	}
	_, err = t.TopicClient.DeleteTopic(ctx, &dataplanev1alpha1.DeleteTopicRequest{
		Name: model.Name.ValueString(),
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to delete topic %s", model.Name), err.Error())
	}
}

// ImportState imports the state of the Topic resource.
func (*Topic) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	response.Diagnostics.Append(response.State.Set(ctx, models.Topic{Name: types.StringValue(request.ID)})...)
}

func (t *Topic) createTopicClient(ctx context.Context, clusterURL string) error {
	if t.TopicClient != nil { // Client already started, no need to create another one.
		return nil
	}
	client, err := clients.NewTopicServiceClient(ctx, t.resData.CloudEnv, clusterURL, clients.ClientRequest{
		ClientID:     t.resData.ClientID,
		ClientSecret: t.resData.ClientSecret,
	})
	if err != nil {
		return fmt.Errorf("unable to create Topic client: %v", err)
	}
	t.TopicClient = client
	return nil
}
