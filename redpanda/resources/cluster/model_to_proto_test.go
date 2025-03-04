package cluster

import (
	"context"
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
)

func TestGenerateClusterCMRUpdate(t *testing.T) {
	// Create a basic GCP cluster proto object
	gcpCluster := &controlplanev1beta2.Cluster{
		CloudProvider: mustCloudProvider("gcp"),
		Type:          controlplanev1beta2.Cluster_TYPE_BYOC,
		CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
			CloudProvider: &controlplanev1beta2.CustomerManagedResources_Gcp{
				Gcp: &controlplanev1beta2.CustomerManagedResources_GCP{
					Subnet: &controlplanev1beta2.CustomerManagedResources_GCP_Subnet{
						Name: "subnet-name",
						SecondaryIpv4RangePods: &controlplanev1beta2.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
							Name: "pods-range",
						},
						SecondaryIpv4RangeServices: &controlplanev1beta2.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
							Name: "services-range",
						},
						K8SMasterIpv4Range: "10.0.0.0/24",
					},
					AgentServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "agent@example.com",
					},
					ConsoleServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "console@example.com",
					},
					ConnectorServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "connector@example.com",
					},
					RedpandaClusterServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "redpanda@example.com",
					},
					GkeServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "gke@example.com",
					},
					TieredStorageBucket: &controlplanev1beta2.CustomerManagedGoogleCloudStorageBucket{
						Name: "test-bucket",
					},
				},
			},
		},
	}

	// Create a GCP cluster with PSC NAT subnet name
	gcpClusterWithSubnet := &controlplanev1beta2.Cluster{
		CloudProvider: mustCloudProvider("gcp"),
		Type:          controlplanev1beta2.Cluster_TYPE_BYOC,
		CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
			CloudProvider: &controlplanev1beta2.CustomerManagedResources_Gcp{
				Gcp: &controlplanev1beta2.CustomerManagedResources_GCP{
					Subnet: &controlplanev1beta2.CustomerManagedResources_GCP_Subnet{
						Name: "subnet-name",
						SecondaryIpv4RangePods: &controlplanev1beta2.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
							Name: "pods-range",
						},
						SecondaryIpv4RangeServices: &controlplanev1beta2.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
							Name: "services-range",
						},
						K8SMasterIpv4Range: "10.0.0.0/24",
					},
					AgentServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "agent@example.com",
					},
					ConsoleServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "console@example.com",
					},
					ConnectorServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "connector@example.com",
					},
					RedpandaClusterServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "redpanda@example.com",
					},
					GkeServiceAccount: &controlplanev1beta2.GCPServiceAccount{
						Email: "gke@example.com",
					},
					TieredStorageBucket: &controlplanev1beta2.CustomerManagedGoogleCloudStorageBucket{
						Name: "test-bucket",
					},
					PscNatSubnetName: "test-subnet",
				},
			},
		},
	}

	// Create an AWS cluster
	awsCluster := &controlplanev1beta2.Cluster{
		CloudProvider: mustCloudProvider("aws"),
		Type:          controlplanev1beta2.Cluster_TYPE_BYOC,
		CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
			CloudProvider: &controlplanev1beta2.CustomerManagedResources_Aws{
				Aws: &controlplanev1beta2.CustomerManagedResources_AWS{
					AgentInstanceProfile: &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{
						Arn: "arn:aws:iam::123456789012:instance-profile/agent",
					},
					ConnectorsNodeGroupInstanceProfile: &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{
						Arn: "arn:aws:iam::123456789012:instance-profile/connectors",
					},
					UtilityNodeGroupInstanceProfile: &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{
						Arn: "arn:aws:iam::123456789012:instance-profile/utility",
					},
					RedpandaNodeGroupInstanceProfile: &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{
						Arn: "arn:aws:iam::123456789012:instance-profile/redpanda",
					},
					K8SClusterRole: &controlplanev1beta2.CustomerManagedResources_AWS_Role{
						Arn: "arn:aws:iam::123456789012:role/k8s",
					},
					RedpandaAgentSecurityGroup: &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{
						Arn: "arn:aws:ec2:region:123456789012:security-group/agent",
					},
					ConnectorsSecurityGroup: &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{
						Arn: "arn:aws:ec2:region:123456789012:security-group/connectors",
					},
					RedpandaNodeGroupSecurityGroup: &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{
						Arn: "arn:aws:ec2:region:123456789012:security-group/redpanda",
					},
					UtilitySecurityGroup: &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{
						Arn: "arn:aws:ec2:region:123456789012:security-group/utility",
					},
					ClusterSecurityGroup: &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{
						Arn: "arn:aws:ec2:region:123456789012:security-group/cluster",
					},
					NodeSecurityGroup: &controlplanev1beta2.CustomerManagedResources_AWS_SecurityGroup{
						Arn: "arn:aws:ec2:region:123456789012:security-group/node",
					},
					CloudStorageBucket: &controlplanev1beta2.CustomerManagedAWSCloudStorageBucket{
						Arn: "arn:aws:s3:::my-bucket",
					},
					PermissionsBoundaryPolicy: &controlplanev1beta2.CustomerManagedResources_AWS_Policy{
						Arn: "arn:aws:iam::123456789012:policy/boundary",
					},
				},
			},
		},
	}

	// Generate model objects using generateModelCMR function
	diagContext := diag.Diagnostics{}
	gcpCMR, diagGcp := generateModelCMR(gcpCluster, diagContext)
	gcpCMRWithSubnet, diagGcpWithSubnet := generateModelCMR(gcpClusterWithSubnet, diagContext)
	awsCMR, diagAws := generateModelCMR(awsCluster, diagContext)

	// Make sure all the generations worked correctly
	if diagGcp.HasError() || diagGcpWithSubnet.HasError() || diagAws.HasError() {
		t.Fatalf("Error generating CMR models for test setup: %v, %v, %v", diagGcp, diagGcpWithSubnet, diagAws)
	}

	tests := []struct {
		name           string
		cluster        models.Cluster
		expectNil      bool
		expectedUpdate *controlplanev1beta2.CustomerManagedResourcesUpdate
		expectError    bool
	}{
		{
			name: "null customer managed resources",
			cluster: models.Cluster{
				CustomerManagedResources: types.ObjectNull(cmrType),
				CloudProvider:            types.StringValue(utils.CloudProviderStringGcp),
			},
			expectNil: true,
		},
		{
			name: "non-GCP cloud provider",
			cluster: models.Cluster{
				CustomerManagedResources: awsCMR,
				CloudProvider:            types.StringValue(utils.CloudProviderStringAws),
			},
			expectNil: true,
		},
		{
			name: "GCP with no PSC NAT subnet name",
			cluster: models.Cluster{
				CustomerManagedResources: gcpCMR,
				CloudProvider:            types.StringValue(utils.CloudProviderStringGcp),
			},
			expectedUpdate: &controlplanev1beta2.CustomerManagedResourcesUpdate{
				CloudProvider: &controlplanev1beta2.CustomerManagedResourcesUpdate_Gcp{
					Gcp: &controlplanev1beta2.CustomerManagedResourcesUpdate_GCP{},
				},
			},
		},
		{
			name: "GCP with PSC NAT subnet name",
			cluster: models.Cluster{
				CustomerManagedResources: gcpCMRWithSubnet,
				CloudProvider:            types.StringValue(utils.CloudProviderStringGcp),
			},
			expectedUpdate: &controlplanev1beta2.CustomerManagedResourcesUpdate{
				CloudProvider: &controlplanev1beta2.CustomerManagedResourcesUpdate_Gcp{
					Gcp: &controlplanev1beta2.CustomerManagedResourcesUpdate_GCP{
						PscNatSubnetName: "test-subnet",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxTest := context.Background()
			diags := diag.Diagnostics{}

			result, resultDiags := generateClusterCMRUpdate(ctxTest, tt.cluster, diags)

			// Check for expected errors
			if tt.expectError {
				assert.True(t, resultDiags.HasError())
			} else {
				assert.False(t, resultDiags.HasError())
			}

			// Check if nil result expected
			if tt.expectNil {
				assert.Nil(t, result)
				return
			}

			// Verify the result
			assert.NotNil(t, result)

			// Check the GCP provider type
			assert.NotNil(t, result.GetGcp())

			// Check PSC NAT subnet name if provided
			if tt.expectedUpdate != nil && tt.expectedUpdate.GetGcp() != nil {
				expectedSubnet := tt.expectedUpdate.GetGcp().PscNatSubnetName
				actualSubnet := result.GetGcp().PscNatSubnetName
				assert.Equal(t, expectedSubnet, actualSubnet)
			}
		})
	}
}

func TestGetGcpPrivateServiceConnect(t *testing.T) {
	tests := []struct {
		name        string
		input       types.Object
		want        *controlplanev1beta2.GCPPrivateServiceConnectSpec
		expectError bool
		errorMsg    string
	}{
		{
			name:  "null object returns nil",
			input: types.ObjectNull(gcpPrivateServiceConnectType),
			want:  nil,
		},
		{
			name: "basic valid configuration",
			input: types.ObjectValueMust(
				gcpPrivateServiceConnectType,
				map[string]attr.Value{
					"enabled":               types.BoolValue(true),
					"global_access_enabled": types.BoolValue(false),
					"consumer_accept_list": types.ListValueMust(
						types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}},
						[]attr.Value{
							types.ObjectValueMust(
								map[string]attr.Type{"source": types.StringType},
								map[string]attr.Value{"source": types.StringValue("project-123")},
							),
						},
					),
					"status": types.ObjectNull(gcpPrivateServiceConnectStatusType),
				},
			),
			want: &controlplanev1beta2.GCPPrivateServiceConnectSpec{
				Enabled:             true,
				GlobalAccessEnabled: false,
				ConsumerAcceptList: []*controlplanev1beta2.GCPPrivateServiceConnectConsumer{
					{Source: "project-123"},
				},
			},
		},
		{
			name: "multiple consumers",
			input: types.ObjectValueMust(
				gcpPrivateServiceConnectType,
				map[string]attr.Value{
					"enabled":               types.BoolValue(true),
					"global_access_enabled": types.BoolValue(true),
					"consumer_accept_list": types.ListValueMust(
						types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}},
						[]attr.Value{
							types.ObjectValueMust(
								map[string]attr.Type{"source": types.StringType},
								map[string]attr.Value{"source": types.StringValue("project-123")},
							),
							types.ObjectValueMust(
								map[string]attr.Type{"source": types.StringType},
								map[string]attr.Value{"source": types.StringValue("project-456")},
							),
						},
					),
					"status": types.ObjectNull(gcpPrivateServiceConnectStatusType),
				},
			),
			want: &controlplanev1beta2.GCPPrivateServiceConnectSpec{
				Enabled:             true,
				GlobalAccessEnabled: true,
				ConsumerAcceptList: []*controlplanev1beta2.GCPPrivateServiceConnectConsumer{
					{Source: "project-123"},
					{Source: "project-456"},
				},
			},
		},
		{
			name: "empty consumer list",
			input: types.ObjectValueMust(
				gcpPrivateServiceConnectType,
				map[string]attr.Value{
					"enabled":               types.BoolValue(true),
					"global_access_enabled": types.BoolValue(false),
					"consumer_accept_list":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}}),
					"status":                types.ObjectNull(gcpPrivateServiceConnectStatusType),
				},
			),
			want: &controlplanev1beta2.GCPPrivateServiceConnectSpec{
				Enabled:             true,
				GlobalAccessEnabled: false,
				ConsumerAcceptList:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := getGcpPrivateServiceConnect(context.Background(), tt.input, diag.Diagnostics{})

			if tt.expectError {
				assert.True(t, diags.HasError())
				assert.Contains(t, diags.Errors()[0].Summary(), tt.errorMsg)
				return
			}

			assert.False(t, diags.HasError())
			assert.Equal(t, tt.want, got)
		})
	}
}
