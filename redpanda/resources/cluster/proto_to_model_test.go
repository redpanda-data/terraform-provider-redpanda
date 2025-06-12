package cluster

import (
	"fmt"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
)

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
		cluster        *controlplanev1.Cluster
		expectedAWS    *expectedAWS
		expectNull     bool
		expectedErrors []string
		expectedGCP    *expectedGCP
	}{
		{
			name:       "nil cluster returns null object",
			cluster:    nil,
			expectNull: true,
		},
		{
			name:       "cluster without CMR returns null object",
			cluster:    &controlplanev1.Cluster{},
			expectNull: true,
		},
		{
			name: "non-BYOC cluster with CMR returns error",
			cluster: &controlplanev1.Cluster{
				CloudProvider: mustCloudProvider("aws"),
				Type:          controlplanev1.Cluster_TYPE_DEDICATED,
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Aws{},
				},
			},
			expectNull:     true,
			expectedErrors: []string{"Customer Managed Resources with non-BYOC cluster type"},
		},
		{
			name: "cloud provider mismatch returns error",
			cluster: &controlplanev1.Cluster{
				CloudProvider: mustCloudProvider("aws"),
				Type:          controlplanev1.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Gcp{},
				},
			},
			expectNull:     true,
			expectedErrors: []string{"Cloud Provider Mismatch"},
		},
		{
			name: "valid AWS BYOC cluster with complete CMR",
			cluster: &controlplanev1.Cluster{
				CloudProvider: mustCloudProvider("aws"),
				Type:          controlplanev1.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Aws{
						Aws: &controlplanev1.CustomerManagedResources_AWS{
							AgentInstanceProfile: &controlplanev1.AWSInstanceProfile{
								Arn: "arn:aws:iam::123456789012:instance-profile/agent",
							},
							ConnectorsNodeGroupInstanceProfile: &controlplanev1.AWSInstanceProfile{
								Arn: "arn:aws:iam::123456789012:instance-profile/connectors",
							},
							UtilityNodeGroupInstanceProfile: &controlplanev1.AWSInstanceProfile{
								Arn: "arn:aws:iam::123456789012:instance-profile/utility",
							},
							RedpandaNodeGroupInstanceProfile: &controlplanev1.AWSInstanceProfile{
								Arn: "arn:aws:iam::123456789012:instance-profile/redpanda",
							},
							K8SClusterRole: &controlplanev1.CustomerManagedResources_AWS_Role{
								Arn: "arn:aws:iam::123456789012:role/k8s",
							},
							RedpandaAgentSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:region:123456789012:security-group/agent",
							},
							ConnectorsSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:region:123456789012:security-group/connectors",
							},
							RedpandaNodeGroupSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:region:123456789012:security-group/redpanda",
							},
							UtilitySecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:region:123456789012:security-group/utility",
							},
							ClusterSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:region:123456789012:security-group/cluster",
							},
							NodeSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:region:123456789012:security-group/node",
							},
							CloudStorageBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
								Arn: "arn:aws:s3:::my-bucket",
							},
							PermissionsBoundaryPolicy: &controlplanev1.CustomerManagedResources_AWS_Policy{
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
			cluster: &controlplanev1.Cluster{
				CloudProvider: mustCloudProvider("aws"),
				Type:          controlplanev1.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Aws{
						Aws: &controlplanev1.CustomerManagedResources_AWS{
							AgentInstanceProfile: &controlplanev1.AWSInstanceProfile{
								Arn: "arn:aws:iam::123456789012:instance-profile/agent",
							},
							CloudStorageBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
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
			name: "unknown cloud provider returns null object",
			cluster: &controlplanev1.Cluster{
				CloudProvider:            mustCloudProvider("unknown"),
				Type:                     controlplanev1.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{},
			},
			expectNull: true,
		},
		{
			name: "valid GCP BYOC cluster with complete CMR",
			cluster: &controlplanev1.Cluster{
				CloudProvider: mustCloudProvider("gcp"),
				Type:          controlplanev1.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.CustomerManagedResources_GCP{
							Subnet: &controlplanev1.CustomerManagedResources_GCP_Subnet{
								Name: "test-subnet",
								SecondaryIpv4RangePods: &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
									Name: "pods-range",
								},
								SecondaryIpv4RangeServices: &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
									Name: "services-range",
								},
								K8SMasterIpv4Range: "10.0.0.0/28",
							},
							AgentServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "agent-sa@project-id.iam.gserviceaccount.com",
							},
							ConsoleServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "console-sa@project-id.iam.gserviceaccount.com",
							},
							ConnectorServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "connector-sa@project-id.iam.gserviceaccount.com",
							},
							RedpandaClusterServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-sa@project-id.iam.gserviceaccount.com",
							},
							GkeServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "gke-sa@project-id.iam.gserviceaccount.com",
							},
							TieredStorageBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
								Name: "redpanda-tiered-storage-bucket",
							},
							PscNatSubnetName: "psc-nat-subnet",
						},
					},
				},
			},
			expectedGCP: &expectedGCP{
				subnetName:                         "test-subnet",
				secondaryIPv4RangePodsName:         "pods-range",
				secondaryIPv4RangeServicesName:     "services-range",
				k8sMasterIPv4Range:                 "10.0.0.0/28",
				agentServiceAccountEmail:           "agent-sa@project-id.iam.gserviceaccount.com",
				consoleServiceAccountEmail:         "console-sa@project-id.iam.gserviceaccount.com",
				connectorServiceAccountEmail:       "connector-sa@project-id.iam.gserviceaccount.com",
				redpandaClusterServiceAccountEmail: "redpanda-sa@project-id.iam.gserviceaccount.com",
				gkeServiceAccountEmail:             "gke-sa@project-id.iam.gserviceaccount.com",
				tieredStorageBucketName:            "redpanda-tiered-storage-bucket",
				pscNatSubnetName:                   "psc-nat-subnet",
			},
		},
		{
			name: "valid GCP BYOC cluster with partial CMR",
			cluster: &controlplanev1.Cluster{
				CloudProvider: mustCloudProvider("gcp"),
				Type:          controlplanev1.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.CustomerManagedResources_GCP{
							Subnet: &controlplanev1.CustomerManagedResources_GCP_Subnet{
								Name: "test-subnet",
								SecondaryIpv4RangePods: &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
									Name: "pods-range",
								},
								SecondaryIpv4RangeServices: &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
									Name: "services-range",
								},
								K8SMasterIpv4Range: "10.0.0.0/28",
							},
							AgentServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "agent-sa@project-id.iam.gserviceaccount.com",
							},
							TieredStorageBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
								Name: "redpanda-tiered-storage-bucket",
							},
						},
					},
				},
			},
			expectedGCP: &expectedGCP{
				subnetName:                     "test-subnet",
				secondaryIPv4RangePodsName:     "pods-range",
				secondaryIPv4RangeServicesName: "services-range",
				k8sMasterIPv4Range:             "10.0.0.0/28",
				agentServiceAccountEmail:       "agent-sa@project-id.iam.gserviceaccount.com",
				tieredStorageBucketName:        "redpanda-tiered-storage-bucket",
			},
		},
		{
			name: "GCP BYOC cluster with empty CMR",
			cluster: &controlplanev1.Cluster{
				CloudProvider: mustCloudProvider("gcp"),
				Type:          controlplanev1.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.CustomerManagedResources_GCP{},
					},
				},
			},
			expectNull:  false, // We expect an empty object, not null
			expectedGCP: &expectedGCP{},
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

			// Verify attributes based on cloud provider
			assert.False(t, obj.IsNull())
			assert.False(t, diagnostics.HasError())

			attrs := obj.Attributes()

			// Verify AWS attributes if expected
			if tt.expectedAWS != nil {
				awsObj, ok := attrs["aws"].(types.Object)
				assert.True(t, ok, "aws should be an Object")
				assert.False(t, awsObj.IsNull(), "aws should not be null")

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

			// Verify GCP attributes if expected
			if tt.expectedGCP != nil {
				gcpObj, ok := attrs["gcp"].(types.Object)
				assert.True(t, ok, "gcp should be an Object")
				assert.False(t, gcpObj.IsNull(), "gcp should not be null")

				gcpAttrs := gcpObj.Attributes()
				verifyGCPAttributes(t, gcpAttrs, tt.expectedGCP)
			}
		})
	}
}

func mustCloudProvider(s string) controlplanev1.CloudProvider {
	cp, _ := utils.StringToCloudProvider(s)
	return cp
}

type expectedGCP struct {
	subnetName                         string
	secondaryIPv4RangePodsName         string
	secondaryIPv4RangeServicesName     string
	k8sMasterIPv4Range                 string
	agentServiceAccountEmail           string
	consoleServiceAccountEmail         string
	connectorServiceAccountEmail       string
	redpandaClusterServiceAccountEmail string
	gkeServiceAccountEmail             string
	tieredStorageBucketName            string
	pscNatSubnetName                   string
}

// verifyGCPAttributes helper function checks if GCP CMR attributes match expected values
func verifyGCPAttributes(t *testing.T, attrs map[string]attr.Value, expected *expectedGCP) {
	// Skip verification if no expectations are provided
	if expected == nil {
		return
	}

	// Verify subnet attributes
	subnetObj, ok := attrs["subnet"].(types.Object)
	if expected.subnetName != "" {
		assert.True(t, ok, "subnet should be an Object")
		assert.False(t, subnetObj.IsNull(), "subnet should not be null")

		subnetAttrs := subnetObj.Attributes()

		// Verify subnet name
		nameAttr, ok := subnetAttrs["name"].(types.String)
		assert.True(t, ok, "subnet name should be a String")
		assert.Equal(t, expected.subnetName, nameAttr.ValueString(), "subnet name should match")

		// Verify secondary IPv4 range for pods
		podsRangeObj, ok := subnetAttrs["secondary_ipv4_range_pods"].(types.Object)
		if expected.secondaryIPv4RangePodsName != "" {
			assert.True(t, ok, "pods range should be an Object")
			assert.False(t, podsRangeObj.IsNull(), "pods range should not be null")

			podsRangeAttrs := podsRangeObj.Attributes()
			nameAttr, ok := podsRangeAttrs["name"].(types.String)
			assert.True(t, ok, "pods range name should be a String")
			assert.Equal(t, expected.secondaryIPv4RangePodsName, nameAttr.ValueString(), "pods range name should match")
		}

		// Verify secondary IPv4 range for services
		servicesRangeObj, ok := subnetAttrs["secondary_ipv4_range_services"].(types.Object)
		if expected.secondaryIPv4RangeServicesName != "" {
			assert.True(t, ok, "services range should be an Object")
			assert.False(t, servicesRangeObj.IsNull(), "services range should not be null")

			servicesRangeAttrs := servicesRangeObj.Attributes()
			nameAttr, ok := servicesRangeAttrs["name"].(types.String)
			assert.True(t, ok, "services range name should be a String")
			assert.Equal(t, expected.secondaryIPv4RangeServicesName, nameAttr.ValueString(), "services range name should match")
		}

		// Verify k8s master IPv4 range
		if expected.k8sMasterIPv4Range != "" {
			k8sMasterIPv4RangeAttr, ok := subnetAttrs["k8s_master_ipv4_range"].(types.String)
			assert.True(t, ok, "k8s master IPv4 range should be a String")
			assert.Equal(t, expected.k8sMasterIPv4Range, k8sMasterIPv4RangeAttr.ValueString(), "k8s master IPv4 range should match")
		}
	}

	// Verify service account emails
	verifyServiceAccount(t, attrs, "agent_service_account", expected.agentServiceAccountEmail)
	verifyServiceAccount(t, attrs, "console_service_account", expected.consoleServiceAccountEmail)
	verifyServiceAccount(t, attrs, "connector_service_account", expected.connectorServiceAccountEmail)
	verifyServiceAccount(t, attrs, "redpanda_cluster_service_account", expected.redpandaClusterServiceAccountEmail)
	verifyServiceAccount(t, attrs, "gke_service_account", expected.gkeServiceAccountEmail)

	// Verify tiered storage bucket
	if expected.tieredStorageBucketName != "" {
		bucketObj, ok := attrs["tiered_storage_bucket"].(types.Object)
		assert.True(t, ok, "tiered storage bucket should be an Object")
		assert.False(t, bucketObj.IsNull(), "tiered storage bucket should not be null")

		bucketAttrs := bucketObj.Attributes()
		nameAttr, ok := bucketAttrs["name"].(types.String)
		assert.True(t, ok, "bucket name should be a String")
		assert.Equal(t, expected.tieredStorageBucketName, nameAttr.ValueString(), "bucket name should match")
	}

	// Verify PSC NAT subnet name
	if expected.pscNatSubnetName != "" {
		pscNatSubnetNameAttr, ok := attrs["psc_nat_subnet_name"].(types.String)
		assert.True(t, ok, "PSC NAT subnet name should be a String")
		assert.Equal(t, expected.pscNatSubnetName, pscNatSubnetNameAttr.ValueString(), "PSC NAT subnet name should match")
	}
}

// Helper function to verify service account emails
func verifyServiceAccount(t *testing.T, attrs map[string]attr.Value, key, expectedEmail string) {
	if expectedEmail == "" {
		return
	}

	serviceAccountObj, ok := attrs[key].(types.Object)
	assert.True(t, ok, fmt.Sprintf("%s should be an Object", key))
	assert.False(t, serviceAccountObj.IsNull(), fmt.Sprintf("%s should not be null", key))

	serviceAccountAttrs := serviceAccountObj.Attributes()
	emailAttr, ok := serviceAccountAttrs["email"].(types.String)
	assert.True(t, ok, fmt.Sprintf("%s email should be a String", key))
	assert.Equal(t, expectedEmail, emailAttr.ValueString(), fmt.Sprintf("%s email should match", key))
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
