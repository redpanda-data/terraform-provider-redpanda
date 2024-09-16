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

// Package cluster contains the implementation of the Cluster resource
// following the Terraform framework interfaces.
package cluster

import (
	"context"
	"fmt"
	"time"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Cluster{}
	_ resource.ResourceWithConfigure   = &Cluster{}
	_ resource.ResourceWithImportState = &Cluster{}
)

// Cluster represents a cluster managed resource.
type Cluster struct {
	CpCl *cloud.ControlPlaneClientSet
	Byoc *utils.ByocClient
}

// Metadata returns the full name of the Cluster resource.
func (*Cluster) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_cluster"
}

// Configure uses provider level data to configure Cluster's clients.
func (c *Cluster) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(config.Resource)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	c.Byoc = p.ByocClient
	c.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the Cluster resource.
func (*Cluster) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceClusterSchema()
}

func resourceClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Unique name of the cluster.",
			},
			"cluster_type": schema.StringAttribute{
				Required:      true,
				Description:   "Cluster type. Type is immutable and can only be set on cluster creation.",
				Validators:    validators.ClusterTypes(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"connection_type": schema.StringAttribute{
				Required:      true,
				Description:   "Cluster connection type. Private clusters are not exposed to the internet. For BYOC clusters, Private is best-practice.",
				Validators:    validators.ConnectionTypes(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cloud_provider": schema.StringAttribute{
				Optional:      true,
				Description:   "Cloud provider where resources are created.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    validators.CloudProviders(),
			},
			"redpanda_version": schema.StringAttribute{
				Optional:      true,
				Description:   "Current Redpanda version of the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"throughput_tier": schema.StringAttribute{
				Required:      true,
				Description:   "Throughput tier of the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"region": schema.StringAttribute{
				Optional:      true,
				Description:   "Cloud provider region. Region represents the name of the region where the cluster will be provisioned.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"zones": schema.ListAttribute{
				Optional:      true,
				Description:   "Zones of the cluster. Must be valid zones within the selected region. If multiple zones are used, the cluster is a multi-AZ cluster.",
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Description: "Allows deletion of the cluster. Defaults to true. Should probably be set to false for production use.",
			},
			"tags": schema.MapAttribute{
				Optional:      true,
				Description:   "Tags placed on cloud resources. If the cloud provider is GCP and the name of a tag has the prefix \"gcp.network-tag.\", the tag is a network tag that will be added to the Redpanda cluster GKE nodes. Otherwise, the tag is a normal tag. For example, if the name of a tag is \"gcp.network-tag.network-tag-foo\", the network tag named \"network-tag-foo\" will be added to the Redpanda cluster GKE nodes. Note: The value of a network tag will be ignored. See the details on network tags at https://cloud.google.com/vpc/docs/add-remove-network-tags.",
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"resource_group_id": schema.StringAttribute{
				Required:      true,
				Description:   "Resource group ID of the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"network_id": schema.StringAttribute{
				Required:      true,
				Description:   "Network ID where cluster is placed.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "ID of the cluster. ID is an output from the Create Cluster endpoint and cannot be set by the caller.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cluster_api_url": schema.StringAttribute{
				Computed:      true,
				Description:   "The URL of the cluster API.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"aws_private_link": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The AWS Private Link configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Redpanda AWS Private Link Endpoint Service is enabled.",
					},
					"connect_console": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Console is connected in Redpanda AWS Private Link Service.",
					},
					"allowed_principals": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
						Description: "The ARN of the principals that can access the Redpanda AWS PrivateLink Endpoint Service. To grant permissions to all principals, use an asterisk (*).",
					},
				},
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "aws_private_link",
						CloudProvider: "aws",
					},
				},
			},
			"azure_private_link": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The Azure Private Link configuration.",
				Attributes: map[string]schema.Attribute{
					"allowed_subscriptions": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
						Description: "The subscriptions that can access the Redpanda Azure PrivateLink Endpoint Service. To grant permissions to all principals, use an asterisk (*).",
					},
					"connect_console": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Console is connected in Redpanda Azure Private Link Service.",
					},
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Redpanda Azure Private Link Endpoint Service is enabled.",
					},
				},
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "azure_private_link",
						CloudProvider: "azure",
					},
				},
			},
			"gcp_private_service_connect": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The GCP Private Service Connect configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Redpanda GCP Private Service Connect is enabled.",
					},
					"global_access_enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether global access is enabled.",
					},
					"consumer_accept_list": schema.ListNestedAttribute{
						Required:    true,
						Description: "List of consumers that are allowed to connect to Redpanda GCP PSC (Private Service Connect) service attachment.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"source": schema.StringAttribute{
									Required:    true,
									Description: "Either the GCP project number or its alphanumeric ID.",
								},
							},
						},
					},
				},
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "gcp_private_service_connect",
						CloudProvider: "gcp",
					},
				},
			},
			"kafka_api": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Cluster's Kafka API properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Required:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"http_proxy": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "HTTP Proxy properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Required:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"schema_registry": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Cluster's Schema Registry properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Required:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"read_replica_cluster_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "IDs of clusters which may create read-only topics from this cluster.",
			},
		},
	}
}

// Create creates a new Cluster resource. It updates the state if the resource
// is successfully created.
func (c *Cluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	clusterReq, err := generateClusterRequest(model)
	if err != nil {
		resp.Diagnostics.AddError("unable to parse CreateCluster request", err.Error())
		return
	}

	clResp, err := c.CpCl.Cluster.CreateCluster(ctx, &controlplanev1beta2.CreateClusterRequest{Cluster: clusterReq})
	if err != nil {
		resp.Diagnostics.AddError("failed to create cluster", err.Error())
		return
	}
	op := clResp.Operation
	clusterID := op.GetResourceId()

	// write initial state so that if cluster creation fails, we can still track and delete it
	resp.Diagnostics.Append(resp.State.Set(ctx, generateMinimalModel(clusterID))...)
	if resp.Diagnostics.HasError() {
		return
	}

	// wait for creation to complete, running "byoc apply" if we see STATE_CREATING_AGENT
	ranByoc := false
	cluster, err := utils.RetryGetCluster(ctx, 90*time.Minute, clusterID, c.CpCl, func(cluster *controlplanev1beta2.Cluster) *utils.RetryError {
		if cluster.GetState() == controlplanev1beta2.Cluster_STATE_CREATING {
			return utils.RetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cluster.GetState()))
		}
		if cluster.GetState() == controlplanev1beta2.Cluster_STATE_CREATING_AGENT {
			if cluster.Type == controlplanev1beta2.Cluster_TYPE_BYOC && !ranByoc {
				err = c.Byoc.RunByoc(ctx, clusterID, "apply")
				if err != nil {
					return utils.NonRetryableError(err)
				}
				ranByoc = true
			}
			return utils.RetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cluster.GetState()))
		}
		if cluster.GetState() == controlplanev1beta2.Cluster_STATE_READY {
			return nil
		}
		if cluster.GetState() == controlplanev1beta2.Cluster_STATE_FAILED {
			return utils.NonRetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cluster.GetState()))
		}
		return utils.NonRetryableError(fmt.Errorf("unhandled state %v. please report this issue to the provider developers", cluster.GetState()))
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to create cluster with ID %q", clusterID), err.Error())
		return
	}
	persist, err := generateModel(model, cluster)
	if err != nil {
		resp.Diagnostics.AddError("failed to generate model for state during cluster.Create", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Read reads Cluster resource's values and updates the state.
func (c *Cluster) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	cluster, err := c.CpCl.ClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			// Treat HTTP 404 Not Found status as a signal to recreate resource and return early
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), err.Error())
		return
	}

	if cluster.GetState() == controlplanev1beta2.Cluster_STATE_DELETING || cluster.GetState() == controlplanev1beta2.Cluster_STATE_DELETING_AGENT {
		// null out the state, force it to be destroyed and recreated
		resp.Diagnostics.Append(resp.State.Set(ctx, generateMinimalModel(cluster.Id))...)
		resp.Diagnostics.AddWarning(fmt.Sprintf("cluster %s is in state %s", model.ID.ValueString(), cluster.GetState()), "")
		return
	}

	persist, err := generateModel(model, cluster)
	if err != nil {
		resp.Diagnostics.AddError("failed to generate model for state during cluster.Read", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update all cluster updates are currently delete and recreate.
func (c *Cluster) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan models.Cluster
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state models.Cluster
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	updateReq := &controlplanev1beta2.UpdateClusterRequest{
		Cluster: &controlplanev1beta2.ClusterUpdate{
			Id: plan.ID.ValueString(),
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: make([]string, 0),
		},
	}

	if !plan.Name.Equal(state.Name) {
		updateReq.Cluster.Name = plan.Name.ValueString()
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "name")
	}

	if !isAwsPrivateLinkStructNil(plan.AwsPrivateLink) {
		updateReq.Cluster.AwsPrivateLink = &controlplanev1beta2.AWSPrivateLinkSpec{
			Enabled:           plan.AwsPrivateLink.Enabled.ValueBool(),
			AllowedPrincipals: utils.TypeListToStringSlice(plan.AwsPrivateLink.AllowedPrincipals),
			ConnectConsole:    plan.AwsPrivateLink.ConnectConsole.ValueBool(),
		}
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "aws_private_link")
	}

	if !isGcpPrivateServiceConnectStructNil(plan.GcpPrivateServiceConnect) {
		updateReq.Cluster.GcpPrivateServiceConnect = &controlplanev1beta2.GCPPrivateServiceConnectSpec{
			Enabled:             plan.GcpPrivateServiceConnect.Enabled.ValueBool(),
			GlobalAccessEnabled: plan.GcpPrivateServiceConnect.GlobalAccessEnabled.ValueBool(),
			ConsumerAcceptList:  gcpConnectConsumerModelToStruct(plan.GcpPrivateServiceConnect.ConsumerAcceptList),
		}
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "gcp_private_service_connect")
	}

	if !isMtlsNil(plan.KafkaAPI) {
		updateReq.Cluster.KafkaApi = &controlplanev1beta2.KafkaAPISpec{
			Mtls: toMtlsSpec(plan.KafkaAPI.Mtls),
		}
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "kafka_api")
	}

	if !isMtlsNil(plan.HTTPProxy) {
		updateReq.Cluster.HttpProxy = &controlplanev1beta2.HTTPProxySpec{
			Mtls: toMtlsSpec(plan.HTTPProxy.Mtls),
		}
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "http_proxy")
	}
	if !isMtlsNil(plan.SchemaRegistry) {
		updateReq.Cluster.SchemaRegistry = &controlplanev1beta2.SchemaRegistrySpec{
			Mtls: toMtlsSpec(plan.SchemaRegistry.Mtls),
		}
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "schema_registry")
	}

	if !plan.ReadReplicaClusterIDs.IsNull() {
		updateReq.Cluster.ReadReplicaClusterIds = utils.TypeListToStringSlice(plan.ReadReplicaClusterIDs)
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "read_replica_cluster_ids")
	}

	if len(updateReq.UpdateMask.Paths) != 0 {
		op, err := c.CpCl.Cluster.UpdateCluster(ctx, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("failed to send cluster update request", err.Error())
			return
		}

		if err := utils.AreWeDoneYet(ctx, op.GetOperation(), 90*time.Minute, c.CpCl.Operation); err != nil {
			resp.Diagnostics.AddError("failed while waiting to update cluster", err.Error())
			return
		}
	}

	cluster, err := c.CpCl.ClusterForID(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", plan.ID), err.Error())
		return
	}

	var cfg models.Cluster
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)

	persist, err := generateModel(cfg, cluster)
	if err != nil {
		resp.Diagnostics.AddError("failed to generate model for state during cluster.Update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Delete deletes the Cluster resource.
func (c *Cluster) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	if !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("cluster deletion not allowed", "allow_deletion is set to false")
		return
	}

	clusterID := model.ID.ValueString()
	cluster, err := c.CpCl.ClusterForID(ctx, clusterID)
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), err.Error())
		return
	}

	// call Delete on the cluser, if it's not already in progress. calling Delete on a cluster in
	// STATE_DELETING_AGENT seems to destroy it immediately and we don't want to do that if we haven't
	// cleaned up yet
	if !(cluster.GetState() == controlplanev1beta2.Cluster_STATE_DELETING || cluster.GetState() == controlplanev1beta2.Cluster_STATE_DELETING_AGENT) {
		_, err = c.CpCl.Cluster.DeleteCluster(ctx, &controlplanev1beta2.DeleteClusterRequest{
			Id: clusterID,
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to delete cluster", err.Error())
			return
		}
	}

	// wait for creation to complete, running "byoc apply" if we see STATE_DELETING_AGENT
	ranByoc := false
	_, err = utils.RetryGetCluster(ctx, 90*time.Minute, clusterID, c.CpCl, func(cluster *controlplanev1beta2.Cluster) *utils.RetryError {
		if cluster.GetState() == controlplanev1beta2.Cluster_STATE_DELETING {
			return utils.RetryableError(fmt.Errorf("expected cluster to be deleted but was in state %v", cluster.GetState()))
		}
		if cluster.GetState() == controlplanev1beta2.Cluster_STATE_DELETING_AGENT {
			if cluster.Type == controlplanev1beta2.Cluster_TYPE_BYOC && !ranByoc {
				err = c.Byoc.RunByoc(ctx, clusterID, "destroy")
				if err != nil {
					return utils.NonRetryableError(err)
				}
				ranByoc = true
			}
			return utils.RetryableError(fmt.Errorf("expected cluster to be deleted but was in state %v", cluster.GetState()))
		}

		return utils.NonRetryableError(fmt.Errorf("unhandled state %v. please report this issue to the provider developers", cluster.GetState()))
	})
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete cluster %s", model.ID), err.Error())
		return
	}
}

// ImportState imports and update the state of the cluster resource.
func (*Cluster) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
