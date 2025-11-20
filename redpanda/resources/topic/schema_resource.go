package topic

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/numberplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceTopicSchema() schema.Schema {
	return schema.Schema{
		Description: "Topic represents a Kafka topic configuration",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "The name of the topic.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"partition_count": schema.NumberAttribute{
				Description: "The number of partitions for the topic. This determines how the data is distributed across brokers. Increases are fully supported without data loss. Decreases will destroy and recreate the topic if allow_deletion is set to true (defaults to false).",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Number{
					numberplanmodifier.UseStateForUnknown(),
					numberplanmodifier.RequiresReplaceIf(
						partitionRequiresReplaceWhenShrinking,
						"Decreasing partition count requires recreating the topic",
						"Decreasing partition count requires recreating the topic",
					),
				},
			},
			"replication_factor": schema.NumberAttribute{
				Description: "The replication factor for the topic, which defines how many copies of the data are kept across different brokers for fault tolerance.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Number{
					numberplanmodifier.RequiresReplace(),
					numberplanmodifier.UseStateForUnknown(),
				},
			},
			"allow_deletion": schema.BoolAttribute{
				Description: "Indicates whether the topic can be deleted.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"configuration": schema.MapAttribute{
				ElementType:   types.StringType,
				Description:   "A map of string key/value pairs of topic configurations.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"cluster_api_url": schema.StringAttribute{
				Required: true,
				Description: "The cluster API URL. Changing this will prevent deletion of the resource on the existing " +
					"cluster. It is generally a better idea to delete an existing resource and create a new one than to " +
					"change this value unless you are planning to do state imports",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

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
