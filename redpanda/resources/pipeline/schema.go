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

package pipeline

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourcePipelineSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Manages a Redpanda Connect pipeline. Redpanda Connect is a declarative data streaming service that connects various data sources and sinks.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the pipeline.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cluster_api_url": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The cluster API URL. Changing this will prevent deletion of the resource on the existing " +
					"cluster. It is generally a better idea to delete an existing resource and create a new one than to " +
					"change this value unless you are planning to do state imports.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User-friendly display name for the pipeline.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional description of the pipeline.",
			},
			"config_yaml": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Redpanda Connect pipeline configuration in YAML format. " +
					"See https://docs.redpanda.com/redpanda-cloud/develop/connect/configuration/about for configuration details.",
			},
			"start_after_create": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether to start the pipeline immediately after creation. Defaults to false to avoid unplanned resource overhead.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current state of the pipeline (starting, running, stopping, stopped, error, completed).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URL to connect to the pipeline's HTTP server, if applicable.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tags": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Key-value pairs to tag the pipeline for organization and filtering.",
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"resources": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Resource allocation for the pipeline.",
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"memory_shares": schema.StringAttribute{
						Optional: true,
						MarkdownDescription: "Amount of memory to allocate for the pipeline. Follows Kubernetes quantity format " +
							"(e.g., '256Mi', '1Gi'). If not specified, uses platform defaults.",
					},
					"cpu_shares": schema.StringAttribute{
						Optional: true,
						MarkdownDescription: "Amount of CPU to allocate for the pipeline. Follows Kubernetes quantity format " +
							"(e.g., '500m' for 500 millicores, '2' for 2 cores). If not specified, uses platform defaults.",
					},
				},
			},
		},
	}
}
