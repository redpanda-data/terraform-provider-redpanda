package cluster

import (
	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// generateModel populates the Cluster model to be persisted to state for Create, Read and Update operations. It is also indirectly used by Import
func generateModel(cfg models.Cluster, cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (*models.Cluster, diag.Diagnostics) {
	output := &models.Cluster{
		Name:                  types.StringValue(cluster.Name),
		ConnectionType:        types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:         types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:           types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion:       cfg.RedpandaVersion,
		ThroughputTier:        types.StringValue(cluster.ThroughputTier),
		Region:                types.StringValue(cluster.Region),
		AllowDeletion:         cfg.AllowDeletion,
		Tags:                  cfg.Tags,
		ResourceGroupID:       types.StringValue(cluster.ResourceGroupId),
		NetworkID:             types.StringValue(cluster.NetworkId),
		ID:                    types.StringValue(cluster.Id),
		ReadReplicaClusterIDs: utils.StringSliceToTypeList(cluster.ReadReplicaClusterIds),
		Zones:                 utils.StringSliceToTypeList(cluster.Zones),
	}

	if cluster.HasDataplaneApi() {
		output.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}

	kafkaAPI, d := generateModelKafkaAPI(cluster, output, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.KafkaAPI = kafkaAPI

	if cluster.CustomerManagedResources != nil {
		cmr, dg := generateModelCMR(cluster.CloudProvider.String(), cluster, diagnostics)
		if dg.HasError() {
			diagnostics.Append(d...)
			return nil, diagnostics
		}
		output.CustomerManagedResources = cmr
	}

	return output, nil
}

func getMtlsModel(mtls *controlplanev1beta2.MTLSSpec, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	mtlsValue := map[string]attr.Value{
		"enabled":                 types.BoolNull(),
		"ca_certificates_pem":     types.ListNull(types.StringType),
		"principal_mapping_rules": types.ListNull(types.StringType),
	}
	if mtls != nil {
		mtlsValue["enabled"] = types.BoolValue(mtls.GetEnabled())
		mtlsValue["ca_certificates_pem"] = utils.StringSliceToTypeList(mtls.GetCaCertificatesPem())
		mtlsValue["principal_mapping_rules"] = utils.StringSliceToTypeList(mtls.GetPrincipalMappingRules())
	}
	out, d := types.ObjectValue(mtlsType, mtlsValue)
	if d.HasError() {
		diagnostics.Append(d...)
		return types.ObjectNull(mtlsType), diagnostics
	}
	return out, diagnostics
}

func generateModelKafkaAPI(cluster *controlplanev1beta2.Cluster, output *models.Cluster, diags diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasKafkaApi() {
		output.KafkaAPI = types.ObjectNull(kafkaAPIType)
		return types.ObjectNull(kafkaAPIType), diags
	}

	kafkaApi := cluster.GetKafkaApi()

	var seedBrokers types.List
	if sb := kafkaApi.GetSeedBrokers(); sb != nil {
		seedBrokers = utils.StringSliceToTypeList(sb)
	}
	mtls, d := getMtlsModel(kafkaApi.GetMtls(), diags)
	if d.HasError() {
		return types.ObjectNull(kafkaAPIType), d
	}

	obj, d := types.ObjectValue(kafkaAPIType, map[string]attr.Value{
		"mtls":         mtls,
		"seed_brokers": seedBrokers,
	})
	if d.HasError() {
		return types.ObjectNull(kafkaAPIType), d
	}
	return obj, diags
}

// generateUpdateRequest populates an UpdateClusterRequest that will update a cluster from the
// current state to a new state matching the plan.
func generateUpdateRequest(plan, state models.Cluster) *controlplanev1beta2.UpdateClusterRequest {
	planUpdate := generateClusterUpdate(plan)
	stateUpdate := generateClusterUpdate(state)

	update, fieldmask := utils.GenerateProtobufDiffAndUpdateMask(planUpdate, stateUpdate)
	update.Id = planUpdate.Id
	return &controlplanev1beta2.UpdateClusterRequest{
		Cluster:    update,
		UpdateMask: fieldmask,
	}
}

func generateModelCMR(cloudProvider string, cluster *controlplanev1beta2.Cluster, diags diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasCustomerManagedResources() {
		return types.ObjectNull(cmrType), diags
	}

	if cluster.Type != controlplanev1beta2.Cluster_TYPE_BYOC {
		diags.AddError("Customer Managed Resources with non-BYOC cluster type", "Customer Managed Resources are only supported for BYOC clusters")
		return types.ObjectNull(cmrType), diags
	}

	switch cloudProvider {
	case "aws":
		if !cluster.CustomerManagedResources.HasAws() {
			diags.AddError("Cloud Provider Mismatch", "AWS customer managed resources are missing for AWS BYOVPC Cluster")
			return types.ObjectNull(cmrType), diags
		}

		awsData := cluster.GetCustomerManagedResources().GetAws()
		retVal := make(map[string]attr.Value, len(awsValue))
		for k, v := range awsValue {
			retVal[k] = v
		}

		if awsData.HasAgentInstanceProfile() {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.AgentInstanceProfile.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["agent_instance_profile"] = ov
		}

		if awsData.ConnectorsNodeGroupInstanceProfile != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.ConnectorsNodeGroupInstanceProfile.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["connectors_node_group_instance_profile"] = ov
		}

		if awsData.UtilityNodeGroupInstanceProfile != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.UtilityNodeGroupInstanceProfile.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["utility_node_group_instance_profile"] = ov
		}

		if awsData.RedpandaNodeGroupInstanceProfile != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.RedpandaNodeGroupInstanceProfile.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["redpanda_node_group_instance_profile"] = ov
		}

		// Roles
		if awsData.K8SClusterRole != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.K8SClusterRole.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["k8s_cluster_role"] = ov
		}

		if awsData.PermissionsBoundaryPolicy != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.PermissionsBoundaryPolicy.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["permissions_boundary_policy"] = ov
		}

		// Security Groups
		if awsData.RedpandaAgentSecurityGroup != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.RedpandaAgentSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["redpanda_agent_security_group"] = ov
		}

		if awsData.ConnectorsSecurityGroup != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.ConnectorsSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["connectors_security_group"] = ov
		}

		if awsData.RedpandaNodeGroupSecurityGroup != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.RedpandaNodeGroupSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["redpanda_node_group_security_group"] = ov
		}

		if awsData.UtilitySecurityGroup != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.UtilitySecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["utility_security_group"] = ov
		}

		if awsData.ClusterSecurityGroup != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.ClusterSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["cluster_security_group"] = ov
		}

		if awsData.NodeSecurityGroup != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.NodeSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["node_security_group"] = ov
		}

		// Cloud Storage Bucket
		if awsData.CloudStorageBucket != nil {
			ov, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.CloudStorageBucket.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			retVal["cloud_storage_bucket"] = ov
		}

		crmV := make(map[string]attr.Value, len(crmVal))
		for k, v := range awsValue {
			retVal[k] = v
		}
		crmV["aws"] = basetypes.NewObjectValueMust(awsType, retVal)
		return types.ObjectValue(cmrType, crmV)
	case "gcp":
		// TODO: Implement GCP support
		return types.ObjectNull(cmrType), diags
	}
	return types.ObjectNull(cmrType), diags
}
