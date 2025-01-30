package cluster

import (
	"context"
	"fmt"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// generateClusterRequest was pulled out to enable unit testing
func generateClusterRequest(model models.Cluster) (*controlplanev1beta2.ClusterCreate, error) {
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

	if !model.KafkaAPI.IsNull() {
		output.KafkaApi.
	}

	if !isAwsPrivateLinkStructNil(model.AwsPrivateLink) {
		output.AwsPrivateLink = &controlplanev1beta2.AWSPrivateLinkSpec{
			Enabled:           model.AwsPrivateLink.Enabled.ValueBool(),
			AllowedPrincipals: utils.TypeListToStringSlice(model.AwsPrivateLink.AllowedPrincipals),
			ConnectConsole:    model.AwsPrivateLink.ConnectConsole.ValueBool(),
		}
	}
	if !isGcpPrivateServiceConnectStructNil(model.GcpPrivateServiceConnect) {
		output.GcpPrivateServiceConnect = &controlplanev1beta2.GCPPrivateServiceConnectSpec{
			Enabled:             model.GcpPrivateServiceConnect.Enabled.ValueBool(),
			GlobalAccessEnabled: model.GcpPrivateServiceConnect.GlobalAccessEnabled.ValueBool(),
			ConsumerAcceptList:  gcpConnectConsumerModelToStruct(model.GcpPrivateServiceConnect.ConsumerAcceptList),
		}
	}

	if !isAzurePrivateLinkStructNil(model.AzurePrivateLink) {
		output.AzurePrivateLink = &controlplanev1beta2.AzurePrivateLinkSpec{
			Enabled:              model.AzurePrivateLink.Enabled.ValueBool(),
			AllowedSubscriptions: utils.TypeListToStringSlice(model.AzurePrivateLink.AllowedSubscriptions),
			ConnectConsole:       model.AzurePrivateLink.ConnectConsole.ValueBool(),
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
	if !model.ReadReplicaClusterIDs.IsNull() {
		output.ReadReplicaClusterIds = utils.TypeListToStringSlice(model.ReadReplicaClusterIDs)
	}

	if !model.CustomerManagedResources.IsNull() {
		cmr, d := generateClusterCMR(context.Background(), model, diag.Diagnostics{})
		if d.HasError() {
			return nil, fmt.Errorf("failed to generate CustomerManagedResources: %v", d)
		}
		output.CustomerManagedResources = cmr
	}

	return output, nil
}

// generateClusterUpdate generates a *controlplanev1beta2.ClusterUpdate for a given cluster
// model, which is then used by generateUpdateRequest to compare ClusterUpdates for plan
// and state and generate an efficient diff and updatemask.
func generateClusterUpdate(cluster models.Cluster) *controlplanev1beta2.ClusterUpdate {
	update := &controlplanev1beta2.ClusterUpdate{
		Id:                    cluster.ID.ValueString(),
		Name:                  cluster.Name.ValueString(),
		ReadReplicaClusterIds: utils.TypeListToStringSlice(cluster.ReadReplicaClusterIDs),
	}

	if !isAwsPrivateLinkStructNil(cluster.AwsPrivateLink) {
		update.AwsPrivateLink = &controlplanev1beta2.AWSPrivateLinkSpec{
			Enabled:           cluster.AwsPrivateLink.Enabled.ValueBool(),
			AllowedPrincipals: utils.TypeListToStringSlice(cluster.AwsPrivateLink.AllowedPrincipals),
			ConnectConsole:    cluster.AwsPrivateLink.ConnectConsole.ValueBool(),
		}
	}

	if !isAzurePrivateLinkStructNil(cluster.AzurePrivateLink) {
		update.AzurePrivateLink = &controlplanev1beta2.AzurePrivateLinkSpec{
			Enabled:              cluster.AzurePrivateLink.Enabled.ValueBool(),
			AllowedSubscriptions: utils.TypeListToStringSlice(cluster.AzurePrivateLink.AllowedSubscriptions),
			ConnectConsole:       cluster.AzurePrivateLink.ConnectConsole.ValueBool(),
		}
	}

	if !isGcpPrivateServiceConnectStructNil(cluster.GcpPrivateServiceConnect) {
		update.GcpPrivateServiceConnect = &controlplanev1beta2.GCPPrivateServiceConnectSpec{
			Enabled:             cluster.GcpPrivateServiceConnect.Enabled.ValueBool(),
			GlobalAccessEnabled: cluster.GcpPrivateServiceConnect.GlobalAccessEnabled.ValueBool(),
			ConsumerAcceptList:  gcpConnectConsumerModelToStruct(cluster.GcpPrivateServiceConnect.ConsumerAcceptList),
		}
	}

	if !isMtlsNil(cluster.KafkaAPI) {
		update.KafkaApi = &controlplanev1beta2.KafkaAPISpec{
			Mtls: toMtlsSpec(cluster.KafkaAPI.Mtls),
		}
	}

	if !isMtlsNil(cluster.HTTPProxy) {
		update.HttpProxy = &controlplanev1beta2.HTTPProxySpec{
			Mtls: toMtlsSpec(cluster.HTTPProxy.Mtls),
		}
	}

	if !isMtlsNil(cluster.SchemaRegistry) {
		update.SchemaRegistry = &controlplanev1beta2.SchemaRegistrySpec{
			Mtls: toMtlsSpec(cluster.SchemaRegistry.Mtls),
		}
	}
	return update
}


func getMtlsSpec(ctx context.Context, mtls types.Object, diags diag.Diagnostics) (*controlplanev1beta2.MTLSSpec, diag.Diagnostics) {
	if mtls.IsNull() {
		return nil, diags
	}
	m, d := getObjectFromAttributes(ctx, "mtls", mtls.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	en, d := getBoolFromAttributes("enabled", m.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	caCerts, d := getListFromAttributes("ca_certificates_pem", m.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	pr, d := getListFromAttributes("principal_mapping_rules", m.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	return &controlplanev1beta2.MTLSSpec{
		Enabled:               en,
		CaCertificatesPem:     utils.TypeListToStringSlice(caCerts),
		PrincipalMappingRules: utils.TypeListToStringSlice(pr),
	}, diags
}

func generateClusterCMR(ctx context.Context, model models.Cluster, diags diag.Diagnostics) (*controlplanev1beta2.CustomerManagedResources, diag.Diagnostics) {
	cmr := &controlplanev1beta2.CustomerManagedResources{}

	if model.CustomerManagedResources.IsNull() {
		return nil, nil
	}

	// If CustomerManagedResources is not null, process it
	switch model.CloudProvider.ValueString() {
	case "aws":
		awsRet := &controlplanev1beta2.CustomerManagedResources_AWS{
			AgentInstanceProfile:               &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{},
			ConnectorsNodeGroupInstanceProfile: &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{},
			UtilityNodeGroupInstanceProfile:    &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{},
			RedpandaNodeGroupInstanceProfile:   &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{},
			K8SClusterRole:                     &controlplanev1beta2.CustomerManagedResources_AWS_Role{},
			RedpandaAgentSecurityGroup:         &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{},
			ConnectorsSecurityGroup:            &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{},
			RedpandaNodeGroupSecurityGroup:     &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{},
			UtilitySecurityGroup:               &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{},
			ClusterSecurityGroup:               &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{},
			NodeSecurityGroup:                  &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{},
			CloudStorageBucket:                 &controlplanev1beta2.CustomerManagedAWSCloudStorageBucket{},
			PermissionsBoundaryPolicy:          &controlplanev1beta2.CustomerManagedResources_AWS_Policy{},
		}

		// Get the AWS object from CustomerManagedResources
		var cmrObj types.Object
		if d := model.CustomerManagedResources.As(context.Background(), &cmrObj, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		}); d.HasError() {
			return nil, d
		}

		aws, d := getObjectFromAttributes(ctx, "aws", cmrObj.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}

		// Agent instance profile
		agentProfileArn, d := getStringFromAttributes("agent_instance_profile", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.AgentInstanceProfile.Arn = agentProfileArn

		// Connectors node group instance profile
		connectorsProfileArn, d := getStringFromAttributes("connectors_node_group_instance_profile", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.ConnectorsNodeGroupInstanceProfile.Arn = connectorsProfileArn

		// Utility node group instance profile
		utilityProfileArn, d := getStringFromAttributes("utility_node_group_instance_profile", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.UtilityNodeGroupInstanceProfile.Arn = utilityProfileArn

		// Redpanda node group instance profile
		redpandaProfileArn, d := getStringFromAttributes("redpanda_node_group_instance_profile", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.RedpandaNodeGroupInstanceProfile.Arn = redpandaProfileArn

		// K8s cluster role
		k8sRoleArn, d := getStringFromAttributes("k8s_cluster_role", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.K8SClusterRole.Arn = k8sRoleArn

		policyArn, d := getStringFromAttributes("permissions_boundary_policy", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.PermissionsBoundaryPolicy.Arn = policyArn

		// Security groups
		agentSecurityGroupArn, d := getStringFromAttributes("redpanda_agent_security_group", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.RedpandaAgentSecurityGroup.Arn = agentSecurityGroupArn

		connectorsSecurityGroupArn, d := getStringFromAttributes("connectors_security_group", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.ConnectorsSecurityGroup.Arn = connectorsSecurityGroupArn

		redpandaNodeGroupSecurityGroupArn, d := getStringFromAttributes("redpanda_node_group_security_group", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.RedpandaNodeGroupSecurityGroup.Arn = redpandaNodeGroupSecurityGroupArn

		utilitySecurityGroupArn, d := getStringFromAttributes("utility_security_group", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.UtilitySecurityGroup.Arn = utilitySecurityGroupArn

		clusterSecurityGroupArn, d := getStringFromAttributes("cluster_security_group", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.ClusterSecurityGroup.Arn = clusterSecurityGroupArn

		nodeSecurityGroupArn, d := getStringFromAttributes("node_security_group", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.NodeSecurityGroup.Arn = nodeSecurityGroupArn

		// Cloud storage bucket
		bucketArn, d := getStringFromAttributes("cloud_storage_bucket", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.CloudStorageBucket.Arn = bucketArn

		cmr.CloudProvider = &controlplanev1beta2.CustomerManagedResources_Aws{
			Aws: awsRet,
		}
		return cmr, nil
	case "gcp":
		// TODO: Implement GCP support
		return nil, nil
	default:
		return nil, nil
	}
}
