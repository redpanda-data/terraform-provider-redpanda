package topic

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	dataplanev1alpha1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/dataplane/v1alpha1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type Topic struct {
	TopicClient dataplanev1alpha1.TopicServiceClient
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

func (t Topic) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
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
	client, err := clients.NewTopicServiceClient(ctx, p.Version, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create topic client", err.Error())
		return
	}
	t.TopicClient = client
}

func (t Topic) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_topic"
}

func ResourceTopicSchema() schema.Schema {
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
		},
	}
}

func (t Topic) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = ResourceTopicSchema()
}

func (t Topic) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	cfg, err := utils.SliceToTopicConfiguration(model.Configuration)
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to convert topic configuration for %s", model.Name), err.Error())
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

func (t Topic) Read(_ context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.State.Get(context.Background(), &model)...)
	tp, err := utils.FindTopicByName(context.Background(), model.Name.ValueString(), t.TopicClient)
	if err != nil {
		if utils.IsNotFound(err) {
			response.State.RemoveResource(context.Background())
			return
		} else {
			response.Diagnostics.AddError(fmt.Sprintf("failed receive response from topic api for topic %s", model.Name), err.Error())
			return
		}
	}
	response.Diagnostics.Append(response.State.Set(context.Background(), models.Topic{
		Name:              types.StringValue(tp.Name),
		PartitionCount:    utils.Int32ToNumber(tp.PartitionCount),
		ReplicationFactor: utils.Int32ToNumber(tp.ReplicationFactor),
		Configuration:     utils.TopicConfigurationToSlice(tp.Configuration),
		AllowDeletion:     model.AllowDeletion,
	})...)
}

func (t Topic) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

func (t Topic) Delete(_ context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.Topic
	response.Diagnostics.Append(request.State.Get(context.Background(), &model)...)
	if !model.AllowDeletion.ValueBool() {
		response.Diagnostics.AddError(fmt.Sprintf("topic %s does not allow deletion", model.Name), "")
		return
	}
	_, err := t.TopicClient.DeleteTopic(context.Background(), &dataplanev1alpha1.DeleteTopicRequest{
		Name: model.Name.ValueString(),
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to delete topic %s", model.Name), err.Error())
	}
}

func (t Topic) ImportState(_ context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	response.Diagnostics.Append(response.State.Set(context.Background(), models.Topic{Name: types.StringValue(request.ID)})...)
}

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Topic{}
	_ resource.ResourceWithConfigure   = &Topic{}
	_ resource.ResourceWithImportState = &Topic{}
)
