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
	"reflect"
	"time"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
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
				Required:      true,
				Description:   "Name of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cluster_type": schema.StringAttribute{
				Required:      true,
				Description:   "Type of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"connection_type": schema.StringAttribute{
				Required:      true,
				Description:   "Connection type of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cloud_provider": schema.StringAttribute{
				Optional:      true,
				Description:   "Must be one of aws or gcp",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"redpanda_version": schema.StringAttribute{
				Optional:      true,
				Description:   "Version of Redpanda to deploy",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"throughput_tier": schema.StringAttribute{
				Required:      true,
				Description:   "Throughput tier of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"region": schema.StringAttribute{
				Optional:      true,
				Description:   "Cloud provider specific region of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"zones": schema.ListAttribute{
				Optional:      true,
				Description:   "Cloud provider specific zones of the cluster",
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Description: "allows deletion of the cluster. defaults to true. should probably be set to false for production use",
			},
			"tags": schema.MapAttribute{
				Optional:      true,
				Description:   "Tags to apply to the cluster",
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"resource_group_id": schema.StringAttribute{
				Required:      true,
				Description:   "The ID of the resource group in which to create the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"network_id": schema.StringAttribute{
				Required:      true,
				Description:   "The ID of the network in which to create the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The ID of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cluster_api_url": schema.StringAttribute{
				Computed:      true,
				Description:   "The URL of the cluster API",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"aws_private_link": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The AWS Private Link configuration",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required: true,
					},
					"allowed_principals": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
						Description: "The ARNs of the allowed principals",
					},
				},
			},
			"gcp_private_service_connect": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The GCP Private Service Connect configuration",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether to enable GCP Private Service Connect",
					},
					"global_access_enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether to enable global access for GCP Private Service Connect",
					},
					"consumer_accept_list": schema.ListNestedAttribute{
						Required:    true,
						Description: "The list of consumers to accept",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"source": schema.StringAttribute{
									Required:    true,
									Description: "The source of the consumer",
								},
							},
						},
					},
				},
			},
			"kafka_api": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The mutual TLS configuration for the Kafka API",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Required:    true,
						Description: "The mutual TLS configuration for the Kafka API",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether to enable mutual TLS for the Kafka API",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "The CA certificates in PEM format",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "The principal mapping rules",
							},
						},
					},
				},
			},
			"http_proxy": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Description: "The mutual TLS configuration for the HTTP Proxy",
						Required:    true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether to enable mutual TLS for the HTTP Proxy",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "The CA certificates in PEM format",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "The principal mapping rules",
							},
						},
					},
				},
			},
			"schema_registry": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Required:    true,
						Description: "The mutual TLS configuration for the Schema Registry",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether to enable mutual TLS for the Schema Registry",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "The CA certificates in PEM format",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "The principal mapping rules",
							},
						},
					},
				},
			},
			"read_replica_cluster_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "The IDs of the read replica clusters",
			},
		},
	}
}

// Create creates a new Cluster resource. It updates the state if the resource
// is successfully created.
func (c *Cluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	clusterReq, err := GenerateClusterRequest(model)
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
	var metadata controlplanev1beta2.CreateClusterMetadata
	if err := op.Metadata.UnmarshalTo(&metadata); err != nil {
		resp.Diagnostics.AddError("failed to unmarshal cluster metadata", err.Error())
		return
	}
	if err := utils.AreWeDoneYet(ctx, op, 60*time.Minute, time.Minute, c.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("operation error while creating cluster", err.Error())
		return
	}
	cluster, err := c.CpCl.ClusterForID(ctx, metadata.GetClusterId())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("successfully created the cluster with ID %q, but failed to read the cluster configuration: %v", model.ID.ValueString(), err), err.Error())
		return
	}
	persist, err := GenerateModel(ctx, model, cluster)
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
	persist, err := GenerateModel(ctx, model, cluster)
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

	updateReq := &controlplanev1beta2.UpdateClusterRequest{
		Cluster: &controlplanev1beta2.ClusterUpdate{
			Id:   plan.ID.ValueString(),
			Name: plan.Name.ValueString(),
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: make([]string, 0),
		},
	}

	if plan.AwsPrivateLink != nil {
		updateReq.Cluster.AwsPrivateLink = &controlplanev1beta2.AWSPrivateLinkSpec{
			Enabled:           plan.AwsPrivateLink.Enabled.ValueBool(),
			AllowedPrincipals: utils.TypeListToStringSlice(plan.AwsPrivateLink.AllowedPrincipals),
		}
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "aws_private_link")
	}

	if plan.GcpPrivateServiceConnect != nil {
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

	if !plan.ReadReplicaClusterIds.IsNull() {
		updateReq.Cluster.ReadReplicaClusterIds = utils.TypeListToStringSlice(plan.ReadReplicaClusterIds)
		updateReq.UpdateMask.Paths = append(updateReq.UpdateMask.Paths, "read_replica_cluster_ids")
	}
	op, err := c.CpCl.Cluster.UpdateCluster(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to send cluster update request", err.Error())
		return
	}

	if err := utils.AreWeDoneYet(ctx, op.GetOperation(), 90*time.Minute, time.Minute, c.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("failed while waiting to update cluster", err.Error())
		return
	}

	cluster, err := c.CpCl.ClusterForID(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", plan.ID), err.Error())
		return
	}

	var cfg models.Cluster
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)

	persist, err := GenerateModel(ctx, cfg, cluster)
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

	// We need to wait for the cluster to be in a running state before we can delete it
	_, err := utils.GetClusterUntilRunningState(ctx, 0, 30, model.Name.ValueString(), c.CpCl)
	if err != nil {
		return
	}

	clResp, err := c.CpCl.Cluster.DeleteCluster(ctx, &controlplanev1beta2.DeleteClusterRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to delete cluster", err.Error())
		return
	}

	if err := utils.AreWeDoneYet(ctx, clResp.Operation, 90*time.Minute, time.Minute, c.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("failed to delete cluster", err.Error())
		return
	}
}

// ImportState imports and update the state of the cluster resource.
func (*Cluster) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// GenerateClusterRequest was pulled out to enable unit testing
func GenerateClusterRequest(model models.Cluster) (*controlplanev1beta2.ClusterCreate, error) {
	provider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		return nil, fmt.Errorf("unable to parse cloud provider: %v", err)
	}
	clusterType, err := utils.StringToClusterType(model.ClusterType.ValueString())
	if err != nil {
		return nil, fmt.Errorf("unable to parse cluster type: %v", err)
	}
	rpVersion := model.RedpandaVersion.ValueString()

	output := &controlplanev1beta2.ClusterCreate{
		Name:              model.Name.ValueString(),
		ConnectionType:    utils.StringToConnectionType(model.ConnectionType.ValueString()),
		CloudProvider:     provider,
		RedpandaVersion:   &rpVersion,
		ThroughputTier:    model.ThroughputTier.ValueString(),
		Region:            model.Region.ValueString(),
		Zones:             utils.TypeListToStringSlice(model.Zones),
		ResourceGroupId:   model.ResourceGroupID.ValueString(),
		NetworkId:         model.NetworkID.ValueString(),
		Type:              clusterType,
		CloudProviderTags: utils.TypeMapToStringMap(model.Tags),
	}
	if model.AwsPrivateLink != nil {
		if !model.AwsPrivateLink.AllowedPrincipals.IsNull() {
			output.AwsPrivateLink = &controlplanev1beta2.AWSPrivateLinkSpec{
				Enabled:           model.AwsPrivateLink.Enabled.ValueBool(),
				AllowedPrincipals: utils.TypeListToStringSlice(model.AwsPrivateLink.AllowedPrincipals),
			}
		}
	}
	if model.GcpPrivateServiceConnect != nil {
		if len(model.GcpPrivateServiceConnect.ConsumerAcceptList) > 0 {
			output.GcpPrivateServiceConnect = &controlplanev1beta2.GCPPrivateServiceConnectSpec{
				Enabled:             model.GcpPrivateServiceConnect.Enabled.ValueBool(),
				GlobalAccessEnabled: model.GcpPrivateServiceConnect.GlobalAccessEnabled.ValueBool(),
				ConsumerAcceptList:  gcpConnectConsumerModelToStruct(model.GcpPrivateServiceConnect.ConsumerAcceptList),
			}
		}
	}
	if model.KafkaAPI != nil {
		output.KafkaApi = &controlplanev1beta2.KafkaAPISpec{
			Mtls: toMtlsSpec(model.KafkaAPI.Mtls),
		}
	}
	if model.HTTPProxy != nil {
		output.HttpProxy = &controlplanev1beta2.HTTPProxySpec{
			Mtls: toMtlsSpec(model.HTTPProxy.Mtls),
		}
	}
	if model.SchemaRegistry != nil {
		output.SchemaRegistry = &controlplanev1beta2.SchemaRegistrySpec{
			Mtls: toMtlsSpec(model.SchemaRegistry.Mtls),
		}
	}
	if !model.ReadReplicaClusterIds.IsNull() {
		output.ReadReplicaClusterIds = utils.TypeListToStringSlice(model.ReadReplicaClusterIds)
	}

	return output, nil
}

func GenerateModel(ctx context.Context, cfg models.Cluster, cluster *controlplanev1beta2.Cluster) (*models.Cluster, error) {
	output := &models.Cluster{
		Name:            types.StringValue(cluster.Name),
		ConnectionType:  types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:   types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion: cfg.RedpandaVersion,
		ThroughputTier:  types.StringValue(cluster.ThroughputTier),
		Region:          types.StringValue(cluster.Region),
		AllowDeletion:   cfg.AllowDeletion,
		Tags:            cfg.Tags,
		ResourceGroupID: types.StringValue(cluster.ResourceGroupId),
		NetworkID:       types.StringValue(cluster.NetworkId),
		ID:              types.StringValue(cluster.Id),
	}

	clusterZones, d := types.ListValueFrom(ctx, types.StringType, cluster.Zones)
	if d.HasError() {
		return nil, fmt.Errorf("failed to parse cluster zones: %v", d)
	}
	output.Zones = clusterZones

	if cluster.GetDataplaneApi() != nil {
		clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
		if err != nil {
			return nil, fmt.Errorf("unable to parse Cluster API URL: %v", err)
		}
		output.ClusterAPIURL = basetypes.NewStringValue(clusterURL)
	}

	rr, d := types.ListValueFrom(ctx, types.StringType, cluster.ReadReplicaClusterIds)
	if d.HasError() {
		return nil, fmt.Errorf("failed to parse read replica cluster IDs: %v", d)
	}
	output.ReadReplicaClusterIds = rr

	if cluster.AwsPrivateLink != nil {
		pl, err := awsPrivateLinkStructToModel(ctx, cluster.GetAwsPrivateLink())
		if err.HasError() {
			return nil, fmt.Errorf("failed to parse AWS Private Link: %v", err)
		}
		output.AwsPrivateLink = pl
	}
	if cluster.GcpPrivateServiceConnect != nil {
		output.GcpPrivateServiceConnect = &models.GcpPrivateServiceConnect{
			Enabled:             types.BoolValue(cluster.GcpPrivateServiceConnect.Enabled),
			GlobalAccessEnabled: types.BoolValue(cluster.GcpPrivateServiceConnect.GlobalAccessEnabled),
			ConsumerAcceptList:  gcpConnectConsumerStructToModel(cluster.GcpPrivateServiceConnect.ConsumerAcceptList),
		}
	}
	kApi, err := toMtlsModel(ctx, cluster.GetKafkaApi().GetMtls())
	if err != nil {
		return nil, fmt.Errorf("failed to parse Kafka API MTLS: %v", err)
	}
	if kApi != nil {
		output.KafkaAPI = &models.KafkaAPI{
			Mtls: kApi,
		}
	}
	ht, err := toMtlsModel(ctx, cluster.GetHttpProxy().GetMtls())
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTTP Proxy MTLS: %v", err)
	}
	if ht != nil {
		output.HTTPProxy = &models.HTTPProxy{
			Mtls: ht,
		}
	}
	sr, err := toMtlsModel(ctx, cluster.GetSchemaRegistry().GetMtls())
	if err != nil {
		return nil, fmt.Errorf("failed to parse Schema Registry MTLS: %v", err)
	}
	if sr != nil {
		output.SchemaRegistry = &models.SchemaRegistry{
			Mtls: sr,
		}
	}
	return output, nil
}

func awsPrivateLinkStructToModel(ctx context.Context, accept *controlplanev1beta2.AWSPrivateLinkStatus) (*models.AwsPrivateLink, diag.Diagnostics) {
	ap, d := types.ListValueFrom(ctx, types.StringType, accept.AllowedPrincipals)
	return &models.AwsPrivateLink{
		Enabled:           types.BoolValue(accept.Enabled),
		AllowedPrincipals: ap,
	}, d
}

func gcpConnectConsumerModelToStruct(accept []*models.GcpPrivateServiceConnectConsumer) []*controlplanev1beta2.GCPPrivateServiceConnectConsumer {
	var output []*controlplanev1beta2.GCPPrivateServiceConnectConsumer
	for _, a := range accept {
		output = append(output, &controlplanev1beta2.GCPPrivateServiceConnectConsumer{
			Source: a.Source,
		})
	}
	return output
}

func gcpConnectConsumerStructToModel(accept []*controlplanev1beta2.GCPPrivateServiceConnectConsumer) []*models.GcpPrivateServiceConnectConsumer {
	var output []*models.GcpPrivateServiceConnectConsumer
	for _, a := range accept {
		output = append(output, &models.GcpPrivateServiceConnectConsumer{
			Source: a.Source,
		})
	}
	return output
}

func toMtlsModel(ctx context.Context, mtls *controlplanev1beta2.MTLSSpec) (*models.Mtls, diag.Diagnostics) {
	if isMtlsSpecNil(mtls) {
		return nil, nil
	}

	capem, err := types.ListValueFrom(ctx, types.StringType, mtls.GetCaCertificatesPem())
	if err != nil {
		return nil, err
	}
	maprules, err := types.ListValueFrom(ctx, types.StringType, mtls.GetPrincipalMappingRules())
	if err != nil {
		return nil, err
	}
	return &models.Mtls{
		Enabled:               types.BoolValue(mtls.GetEnabled()),
		CaCertificatesPem:     capem,
		PrincipalMappingRules: maprules,
	}, nil
}

func toMtlsSpec(mtls *models.Mtls) *controlplanev1beta2.MTLSSpec {
	if isMtlsStructNil(mtls) {
		return emptyMtlsSpec()
	}
	return &controlplanev1beta2.MTLSSpec{
		Enabled:               mtls.Enabled.ValueBool(),
		CaCertificatesPem:     utils.TypeListToStringSlice(mtls.CaCertificatesPem),
		PrincipalMappingRules: utils.TypeListToStringSlice(mtls.PrincipalMappingRules),
	}
}

func isMtlsNil(container any) bool {
	v := reflect.ValueOf(container)
	if !v.IsValid() || v.IsNil() {
		return true
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return true
	}
	mtlsField := v.FieldByName("Mtls")
	if !mtlsField.IsValid() || mtlsField.IsNil() {
		return true
	}
	return isMtlsStructNil(mtlsField.Interface().(*models.Mtls))
}

func isMtlsStructNil(m *models.Mtls) bool {
	return m == nil || (m.Enabled.IsNull() && m.CaCertificatesPem.IsNull() && m.PrincipalMappingRules.IsNull())
}

func isMtlsSpecNil(m *controlplanev1beta2.MTLSSpec) bool {
	return m == nil || (m.GetEnabled() == false && len(m.GetCaCertificatesPem()) == 0 && len(m.GetPrincipalMappingRules()) == 0)
}

func emptyMtlsSpec() *controlplanev1beta2.MTLSSpec {
	return &controlplanev1beta2.MTLSSpec{
		Enabled:               false,
		CaCertificatesPem:     make([]string, 0),
		PrincipalMappingRules: make([]string, 0),
	}
}
