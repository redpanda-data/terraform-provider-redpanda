package cluster

import (
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
)

func mustCloudProvider(s string) controlplanev1beta2.CloudProvider {
	cp, _ := utils.StringToCloudProvider(s)
	return cp
}

func TestGenerateModelCMR(t *testing.T) {
	type expectedAWS struct {
		agentProfileARN        string
		connectorsProfileARN   string
		utilityProfileARN      string
		redpandaProfileARN     string
		k8sRoleARN             string
		agentSGARN             string
		connectorsSGARN        string
		redpandaSGARN          string
		utilitySGARN           string
		clusterSGARN           string
		nodeSGARN              string
		bucketARN              string
		permissionsBoundaryARN string
	}

	tests := []struct {
		name           string
		cloudProvider  string
		cluster        *controlplanev1beta2.Cluster
		expectedAWS    *expectedAWS
		expectNull     bool
		expectedErrors []string
	}{
		{
			name:       "nil cluster returns null object",
			cluster:    nil,
			expectNull: true,
		},
		{
			name:       "cluster without CMR returns null object",
			cluster:    &controlplanev1beta2.Cluster{},
			expectNull: true,
		},
		{
			name: "non-BYOC cluster with CMR returns error",
			cluster: &controlplanev1beta2.Cluster{
				CloudProvider: mustCloudProvider("aws"),
				Type:          controlplanev1beta2.Cluster_TYPE_DEDICATED,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
					CloudProvider: &controlplanev1beta2.CustomerManagedResources_Aws{},
				},
			},
			expectNull:     true,
			expectedErrors: []string{"Customer Managed Resources with non-BYOC cluster type"},
		},
		{
			name: "cloud provider mismatch returns error",
			cluster: &controlplanev1beta2.Cluster{
				CloudProvider: mustCloudProvider("aws"),
				Type:          controlplanev1beta2.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
					CloudProvider: &controlplanev1beta2.CustomerManagedResources_Gcp{},
				},
			},
			expectNull:     true,
			expectedErrors: []string{"Cloud Provider Mismatch"},
		},
		{
			name: "valid AWS BYOC cluster with complete CMR",
			cluster: &controlplanev1beta2.Cluster{
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
			},
			expectedAWS: &expectedAWS{
				agentProfileARN:        "arn:aws:iam::123456789012:instance-profile/agent",
				connectorsProfileARN:   "arn:aws:iam::123456789012:instance-profile/connectors",
				utilityProfileARN:      "arn:aws:iam::123456789012:instance-profile/utility",
				redpandaProfileARN:     "arn:aws:iam::123456789012:instance-profile/redpanda",
				k8sRoleARN:             "arn:aws:iam::123456789012:role/k8s",
				agentSGARN:             "arn:aws:ec2:region:123456789012:security-group/agent",
				connectorsSGARN:        "arn:aws:ec2:region:123456789012:security-group/connectors",
				redpandaSGARN:          "arn:aws:ec2:region:123456789012:security-group/redpanda",
				utilitySGARN:           "arn:aws:ec2:region:123456789012:security-group/utility",
				clusterSGARN:           "arn:aws:ec2:region:123456789012:security-group/cluster",
				nodeSGARN:              "arn:aws:ec2:region:123456789012:security-group/node",
				bucketARN:              "arn:aws:s3:::my-bucket",
				permissionsBoundaryARN: "arn:aws:iam::123456789012:policy/boundary",
			},
		},
		{
			name: "valid AWS BYOC cluster with partial CMR",
			cluster: &controlplanev1beta2.Cluster{
				CloudProvider: mustCloudProvider("aws"),
				Type:          controlplanev1beta2.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
					CloudProvider: &controlplanev1beta2.CustomerManagedResources_Aws{
						Aws: &controlplanev1beta2.CustomerManagedResources_AWS{
							AgentInstanceProfile: &controlplanev1beta2.CustomerManagedResources_AWS_InstanceProfile{
								Arn: "arn:aws:iam::123456789012:instance-profile/agent",
							},
							CloudStorageBucket: &controlplanev1beta2.CustomerManagedAWSCloudStorageBucket{
								Arn: "arn:aws:s3:::my-bucket",
							},
						},
					},
				},
			},
			expectedAWS: &expectedAWS{
				agentProfileARN: "arn:aws:iam::123456789012:instance-profile/agent",
				bucketARN:       "arn:aws:s3:::my-bucket",
			},
		},
		{
			name: "GCP CMR returns null object (not implemented)",
			cluster: &controlplanev1beta2.Cluster{
				CloudProvider: mustCloudProvider("gcp"),
				Type:          controlplanev1beta2.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
					CloudProvider: &controlplanev1beta2.CustomerManagedResources_Gcp{},
				},
			},
			expectNull: true,
		},
		{
			name: "unknown cloud provider returns null object",
			cluster: &controlplanev1beta2.Cluster{
				CloudProvider:            mustCloudProvider("unknown"),
				Type:                     controlplanev1beta2.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{},
			},
			expectNull: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, diagnostics := generateModelCMR(tt.cluster, diag.Diagnostics{})

			// Check for expected errors
			if len(tt.expectedErrors) > 0 {
				assert.True(t, diagnostics.HasError())
				for _, expectedError := range tt.expectedErrors {
					errorFound := false
					for _, diagnostic := range diagnostics.Errors() {
						if diagnostic.Summary() == expectedError {
							errorFound = true
							break
						}
					}
					assert.True(t, errorFound, "Expected error not found: %s", expectedError)
				}
				return
			}

			// Check if null object is expected
			if tt.expectNull {
				assert.True(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())
				return
			}

			// Verify AWS attributes if expected
			if tt.expectedAWS != nil {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				awsObj, ok := attrs["aws"].(types.Object)
				assert.True(t, ok)
				assert.False(t, awsObj.IsNull())

				awsAttrs := awsObj.Attributes()
				verifyARN(t, awsAttrs, "agent_instance_profile", tt.expectedAWS.agentProfileARN)
				verifyARN(t, awsAttrs, "connectors_node_group_instance_profile", tt.expectedAWS.connectorsProfileARN)
				verifyARN(t, awsAttrs, "utility_node_group_instance_profile", tt.expectedAWS.utilityProfileARN)
				verifyARN(t, awsAttrs, "redpanda_node_group_instance_profile", tt.expectedAWS.redpandaProfileARN)
				verifyARN(t, awsAttrs, "k8s_cluster_role", tt.expectedAWS.k8sRoleARN)
				verifyARN(t, awsAttrs, "redpanda_agent_security_group", tt.expectedAWS.agentSGARN)
				verifyARN(t, awsAttrs, "connectors_security_group", tt.expectedAWS.connectorsSGARN)
				verifyARN(t, awsAttrs, "redpanda_node_group_security_group", tt.expectedAWS.redpandaSGARN)
				verifyARN(t, awsAttrs, "utility_security_group", tt.expectedAWS.utilitySGARN)
				verifyARN(t, awsAttrs, "cluster_security_group", tt.expectedAWS.clusterSGARN)
				verifyARN(t, awsAttrs, "node_security_group", tt.expectedAWS.nodeSGARN)
				verifyARN(t, awsAttrs, "cloud_storage_bucket", tt.expectedAWS.bucketARN)
				verifyARN(t, awsAttrs, "permissions_boundary_policy", tt.expectedAWS.permissionsBoundaryARN)
			}
		})
	}
}

// verifyARN helper function checks if the ARN matches expected value or is null if no value expected
func verifyARN(t *testing.T, attrs map[string]attr.Value, key, expectedARN string) {
	obj, ok := attrs[key].(types.Object)
	assert.True(t, ok)
	if expectedARN == "" {
		assert.True(t, obj.IsNull())
		return
	}
	assert.False(t, obj.IsNull())
	arnStr, ok := obj.Attributes()["arn"].(types.String)
	assert.True(t, ok)
	assert.Equal(t, expectedARN, arnStr.ValueString())
}
