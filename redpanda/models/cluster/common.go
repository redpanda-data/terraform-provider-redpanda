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

package cluster

import (
	"context"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pkg/errors"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func generateClusterAWSCMR(ctx context.Context, cmrObj types.Object) (*controlplanev1.CustomerManagedResources_AWS, error) {
	awsRet := &controlplanev1.CustomerManagedResources_AWS{}

	aws, err := utils.GetObjectFromAttributes(ctx, "aws", cmrObj.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get AWS object from CustomerManagedResources")
	}

	if agentProfileArn, err := getArnFromAttributes(ctx, "agent_instance_profile", aws.Attributes()); err == nil {
		awsRet.AgentInstanceProfile = &controlplanev1.AWSInstanceProfile{Arn: agentProfileArn}
	}
	if connectorsProfileArn, err := getArnFromAttributes(ctx, "connectors_node_group_instance_profile", aws.Attributes()); err == nil {
		awsRet.ConnectorsNodeGroupInstanceProfile = &controlplanev1.AWSInstanceProfile{Arn: connectorsProfileArn}
	}
	if utilityProfileArn, err := getArnFromAttributes(ctx, "utility_node_group_instance_profile", aws.Attributes()); err == nil {
		awsRet.UtilityNodeGroupInstanceProfile = &controlplanev1.AWSInstanceProfile{Arn: utilityProfileArn}
	}
	if redpandaProfileArn, err := getArnFromAttributes(ctx, "redpanda_node_group_instance_profile", aws.Attributes()); err == nil {
		awsRet.RedpandaNodeGroupInstanceProfile = &controlplanev1.AWSInstanceProfile{Arn: redpandaProfileArn}
	}
	if k8sRoleArn, err := getArnFromAttributes(ctx, "k8s_cluster_role", aws.Attributes()); err == nil {
		awsRet.K8SClusterRole = &controlplanev1.CustomerManagedResources_AWS_Role{Arn: k8sRoleArn}
	}
	if redpandaAgentSgArn, err := getArnFromAttributes(ctx, "redpanda_agent_security_group", aws.Attributes()); err == nil {
		awsRet.RedpandaAgentSecurityGroup = &controlplanev1.AWSSecurityGroup{Arn: redpandaAgentSgArn}
	}
	if connectorsSgArn, err := getArnFromAttributes(ctx, "connectors_security_group", aws.Attributes()); err == nil {
		awsRet.ConnectorsSecurityGroup = &controlplanev1.AWSSecurityGroup{Arn: connectorsSgArn}
	}
	if redpandaNodeGroupSgArn, err := getArnFromAttributes(ctx, "redpanda_node_group_security_group", aws.Attributes()); err == nil {
		awsRet.RedpandaNodeGroupSecurityGroup = &controlplanev1.AWSSecurityGroup{Arn: redpandaNodeGroupSgArn}
	}
	if utilitySgArn, err := getArnFromAttributes(ctx, "utility_security_group", aws.Attributes()); err == nil {
		awsRet.UtilitySecurityGroup = &controlplanev1.AWSSecurityGroup{Arn: utilitySgArn}
	}
	if clusterSgArn, err := getArnFromAttributes(ctx, "cluster_security_group", aws.Attributes()); err == nil {
		awsRet.ClusterSecurityGroup = &controlplanev1.AWSSecurityGroup{Arn: clusterSgArn}
	}
	if nodeSgArn, err := getArnFromAttributes(ctx, "node_security_group", aws.Attributes()); err == nil {
		awsRet.NodeSecurityGroup = &controlplanev1.AWSSecurityGroup{Arn: nodeSgArn}
	}
	if bucketArn, err := getArnFromAttributes(ctx, "cloud_storage_bucket", aws.Attributes()); err == nil {
		awsRet.CloudStorageBucket = &controlplanev1.CustomerManagedAWSCloudStorageBucket{Arn: bucketArn}
	}
	if permissionsBoundaryArn, err := getArnFromAttributes(ctx, "permissions_boundary_policy", aws.Attributes()); err == nil {
		awsRet.PermissionsBoundaryPolicy = &controlplanev1.CustomerManagedResources_AWS_Policy{Arn: permissionsBoundaryArn}
	}

	return awsRet, nil
}

func generateClusterGCPCMRUpdate(ctx context.Context, cmrObj types.Object) (*controlplanev1.CustomerManagedResourcesUpdate_GCP, error) {
	gcpUpdateRet := &controlplanev1.CustomerManagedResourcesUpdate_GCP{}

	gcp, err := utils.GetObjectFromAttributes(ctx, "gcp", cmrObj.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get GCP object from CustomerManagedResources")
	}

	if pscNatSubnetName, err := utils.GetStringFromAttributes("psc_nat_subnet_name", gcp.Attributes()); err == nil {
		gcpUpdateRet.PscNatSubnetName = pscNatSubnetName
	}
	if redpandaConnectAPISA, err := getEmailFromServiceAccount(ctx, "redpanda_connect_api_service_account", gcp.Attributes()); err == nil {
		gcpUpdateRet.RedpandaConnectApiServiceAccount = &controlplanev1.GCPServiceAccount{Email: redpandaConnectAPISA}
	}
	if redpandaConnectSA, err := getEmailFromServiceAccount(ctx, "redpanda_connect_service_account", gcp.Attributes()); err == nil {
		gcpUpdateRet.RedpandaConnectServiceAccount = &controlplanev1.GCPServiceAccount{Email: redpandaConnectSA}
	}
	if redpandaOperatorSA, err := getEmailFromServiceAccount(ctx, "redpanda_operator_service_account", gcp.Attributes()); err == nil {
		gcpUpdateRet.RedpandaOperatorServiceAccount = &controlplanev1.GCPServiceAccount{Email: redpandaOperatorSA}
	}
	return gcpUpdateRet, nil
}

func generateClusterGCPCMR(ctx context.Context, cmrObj types.Object) (*controlplanev1.CustomerManagedResources_GCP, error) {
	gcpRet := &controlplanev1.CustomerManagedResources_GCP{}

	gcp, err := utils.GetObjectFromAttributes(ctx, "gcp", cmrObj.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get GCP object from CustomerManagedResources")
	}

	if subnet, err := utils.GetObjectFromAttributes(ctx, "subnet", gcp.Attributes()); err == nil {
		subnetName, _ := utils.GetStringFromAttributes("name", subnet.Attributes())
		gcpRet.Subnet = &controlplanev1.CustomerManagedResources_GCP_Subnet{Name: subnetName}
		if podsRange, err := utils.GetObjectFromAttributes(ctx, "secondary_ipv4_range_pods", subnet.Attributes()); err == nil {
			podsRangeName, _ := utils.GetStringFromAttributes("name", podsRange.Attributes())
			gcpRet.Subnet.SecondaryIpv4RangePods = &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{Name: podsRangeName}
		}
		if servicesRange, err := utils.GetObjectFromAttributes(ctx, "secondary_ipv4_range_services", subnet.Attributes()); err == nil {
			servicesRangeName, _ := utils.GetStringFromAttributes("name", servicesRange.Attributes())
			gcpRet.Subnet.SecondaryIpv4RangeServices = &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{Name: servicesRangeName}
		}
		if k8sMasterRange, err := utils.GetStringFromAttributes("k8s_master_ipv4_range", subnet.Attributes()); err == nil {
			gcpRet.Subnet.K8SMasterIpv4Range = k8sMasterRange
		}
	}

	if agentSA, err := getEmailFromServiceAccount(ctx, "agent_service_account", gcp.Attributes()); err == nil {
		gcpRet.AgentServiceAccount = &controlplanev1.GCPServiceAccount{Email: agentSA}
	}
	if consoleSA, err := getEmailFromServiceAccount(ctx, "console_service_account", gcp.Attributes()); err == nil {
		gcpRet.ConsoleServiceAccount = &controlplanev1.GCPServiceAccount{Email: consoleSA}
	}
	if connectorSA, err := getEmailFromServiceAccount(ctx, "connector_service_account", gcp.Attributes()); err == nil {
		gcpRet.ConnectorServiceAccount = &controlplanev1.GCPServiceAccount{Email: connectorSA}
	}
	if redpandaClusterSA, err := getEmailFromServiceAccount(ctx, "redpanda_cluster_service_account", gcp.Attributes()); err == nil {
		gcpRet.RedpandaClusterServiceAccount = &controlplanev1.GCPServiceAccount{Email: redpandaClusterSA}
	}
	if gkeSA, err := getEmailFromServiceAccount(ctx, "gke_service_account", gcp.Attributes()); err == nil {
		gcpRet.GkeServiceAccount = &controlplanev1.GCPServiceAccount{Email: gkeSA}
	}
	if bucket, err := utils.GetObjectFromAttributes(ctx, "tiered_storage_bucket", gcp.Attributes()); err == nil {
		bucketName, _ := utils.GetStringFromAttributes("name", bucket.Attributes())
		gcpRet.TieredStorageBucket = &controlplanev1.CustomerManagedGoogleCloudStorageBucket{Name: bucketName}
	}
	if pscNatSubnetName, err := utils.GetStringFromAttributes("psc_nat_subnet_name", gcp.Attributes()); err == nil {
		gcpRet.PscNatSubnetName = pscNatSubnetName
	}

	return gcpRet, nil
}

func getArnFromAttributes(ctx context.Context, key string, att map[string]attr.Value) (string, error) {
	a, err := utils.GetObjectFromAttributes(ctx, key, att)
	if err != nil {
		return "", err
	}
	return utils.GetStringFromAttributes("arn", a.Attributes())
}

func getEmailFromServiceAccount(ctx context.Context, key string, att map[string]attr.Value) (string, error) {
	sa, err := utils.GetObjectFromAttributes(ctx, key, att)
	if err != nil {
		return "", err
	}
	return utils.GetStringFromAttributes("email", sa.Attributes())
}

func generateModelClusterAWSCMR(_ context.Context, awsData *controlplanev1.CustomerManagedResources_AWS) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	awsVal := getAwsCmrNullAttributes()

	createArnObject := func(arn string) (types.Object, diag.Diagnostics) {
		return types.ObjectValue(GetArnContainerType(), map[string]attr.Value{
			"arn": types.StringValue(arn),
		})
	}

	if awsData.HasAgentInstanceProfile() {
		if obj, d := createArnObject(awsData.GetAgentInstanceProfile().GetArn()); !d.HasError() {
			awsVal["agent_instance_profile"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasConnectorsNodeGroupInstanceProfile() {
		if obj, d := createArnObject(awsData.GetConnectorsNodeGroupInstanceProfile().GetArn()); !d.HasError() {
			awsVal["connectors_node_group_instance_profile"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasUtilityNodeGroupInstanceProfile() {
		if obj, d := createArnObject(awsData.GetUtilityNodeGroupInstanceProfile().GetArn()); !d.HasError() {
			awsVal["utility_node_group_instance_profile"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasRedpandaNodeGroupInstanceProfile() {
		if obj, d := createArnObject(awsData.GetRedpandaNodeGroupInstanceProfile().GetArn()); !d.HasError() {
			awsVal["redpanda_node_group_instance_profile"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasK8SClusterRole() {
		if obj, d := createArnObject(awsData.GetK8SClusterRole().GetArn()); !d.HasError() {
			awsVal["k8s_cluster_role"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasRedpandaAgentSecurityGroup() {
		if obj, d := createArnObject(awsData.GetRedpandaAgentSecurityGroup().GetArn()); !d.HasError() {
			awsVal["redpanda_agent_security_group"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasConnectorsSecurityGroup() {
		if obj, d := createArnObject(awsData.GetConnectorsSecurityGroup().GetArn()); !d.HasError() {
			awsVal["connectors_security_group"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasRedpandaNodeGroupSecurityGroup() {
		if obj, d := createArnObject(awsData.GetRedpandaNodeGroupSecurityGroup().GetArn()); !d.HasError() {
			awsVal["redpanda_node_group_security_group"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasUtilitySecurityGroup() {
		if obj, d := createArnObject(awsData.GetUtilitySecurityGroup().GetArn()); !d.HasError() {
			awsVal["utility_security_group"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasClusterSecurityGroup() {
		if obj, d := createArnObject(awsData.GetClusterSecurityGroup().GetArn()); !d.HasError() {
			awsVal["cluster_security_group"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasNodeSecurityGroup() {
		if obj, d := createArnObject(awsData.GetNodeSecurityGroup().GetArn()); !d.HasError() {
			awsVal["node_security_group"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasCloudStorageBucket() {
		if obj, d := createArnObject(awsData.GetCloudStorageBucket().GetArn()); !d.HasError() {
			awsVal["cloud_storage_bucket"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if awsData.HasPermissionsBoundaryPolicy() {
		if obj, d := createArnObject(awsData.GetPermissionsBoundaryPolicy().GetArn()); !d.HasError() {
			awsVal["permissions_boundary_policy"] = obj
		} else {
			diags.Append(d...)
		}
	}

	awsObj, d := types.ObjectValue(GetAwsCmrType(), awsVal)
	if d.HasError() {
		diags.AddError("failed to create AWS Customer Managed Resources object", "could not create AWS Customer Managed Resources object")
		diags.Append(d...)
		return getClusterAWSCMRNull(), diags
	}

	return awsObj, diags
}

func generateModelClusterGCPCMR(_ context.Context, gcpData *controlplanev1.CustomerManagedResources_GCP) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	gcpVal := getGcpCmrNullAttributes()

	createServiceAccountObject := func(email string) (types.Object, diag.Diagnostics) {
		return types.ObjectValue(GetGcpServiceAccountType(), map[string]attr.Value{
			"email": types.StringValue(email),
		})
	}

	if gcpData.HasSubnet() {
		subnet := gcpData.GetSubnet()
		subnetVal := map[string]attr.Value{
			"name":                          types.StringValue(subnet.GetName()),
			"secondary_ipv4_range_pods":     types.ObjectNull(GetGcpSecondaryIPv4RangeType()),
			"secondary_ipv4_range_services": types.ObjectNull(GetGcpSecondaryIPv4RangeType()),
			"k8s_master_ipv4_range":         types.StringNull(),
		}

		if subnet.HasSecondaryIpv4RangePods() {
			if obj, d := types.ObjectValue(GetGcpSecondaryIPv4RangeType(), map[string]attr.Value{
				"name": types.StringValue(subnet.GetSecondaryIpv4RangePods().GetName()),
			}); !d.HasError() {
				subnetVal["secondary_ipv4_range_pods"] = obj
			} else {
				diags.Append(d...)
			}
		}

		if subnet.HasSecondaryIpv4RangeServices() {
			if obj, d := types.ObjectValue(GetGcpSecondaryIPv4RangeType(), map[string]attr.Value{
				"name": types.StringValue(subnet.GetSecondaryIpv4RangeServices().GetName()),
			}); !d.HasError() {
				subnetVal["secondary_ipv4_range_services"] = obj
			} else {
				diags.Append(d...)
			}
		}

		if subnet.GetK8SMasterIpv4Range() != "" {
			subnetVal["k8s_master_ipv4_range"] = types.StringValue(subnet.GetK8SMasterIpv4Range())
		}

		if subnetObj, d := types.ObjectValue(GetGcpSubnetType(), subnetVal); !d.HasError() {
			gcpVal["subnet"] = subnetObj
		} else {
			diags.Append(d...)
		}
	}

	if gcpData.HasAgentServiceAccount() {
		if obj, d := createServiceAccountObject(gcpData.GetAgentServiceAccount().GetEmail()); !d.HasError() {
			gcpVal["agent_service_account"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if gcpData.HasConsoleServiceAccount() {
		if obj, d := createServiceAccountObject(gcpData.GetConsoleServiceAccount().GetEmail()); !d.HasError() {
			gcpVal["console_service_account"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if gcpData.HasConnectorServiceAccount() {
		if obj, d := createServiceAccountObject(gcpData.GetConnectorServiceAccount().GetEmail()); !d.HasError() {
			gcpVal["connector_service_account"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if gcpData.HasRedpandaClusterServiceAccount() {
		if obj, d := createServiceAccountObject(gcpData.GetRedpandaClusterServiceAccount().GetEmail()); !d.HasError() {
			gcpVal["redpanda_cluster_service_account"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if gcpData.HasGkeServiceAccount() {
		if obj, d := createServiceAccountObject(gcpData.GetGkeServiceAccount().GetEmail()); !d.HasError() {
			gcpVal["gke_service_account"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if gcpData.HasTieredStorageBucket() {
		if obj, d := types.ObjectValue(GetGcpBucketType(), map[string]attr.Value{
			"name": types.StringValue(gcpData.GetTieredStorageBucket().GetName()),
		}); !d.HasError() {
			gcpVal["tiered_storage_bucket"] = obj
		} else {
			diags.Append(d...)
		}
	}

	if gcpData.GetPscNatSubnetName() != "" {
		gcpVal["psc_nat_subnet_name"] = types.StringValue(gcpData.GetPscNatSubnetName())
	}

	gcpObj, d := types.ObjectValue(GetGcpCmrType(), gcpVal)
	if d.HasError() {
		diags.AddError("failed to create GCP Customer Managed Resources object", "could not create GCP Customer Managed Resources object")
		diags.Append(d...)
		return getClusterGCPCMRNull(), diags
	}

	return gcpObj, diags
}

func getClusterAWSCMRNull() basetypes.ObjectValue {
	return types.ObjectNull(GetAwsCmrType())
}

func getClusterGCPCMRNull() basetypes.ObjectValue {
	return types.ObjectNull(GetGcpCmrType())
}
