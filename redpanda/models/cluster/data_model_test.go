package cluster

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/genproto/googleapis/type/dayofweek"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDataModel_GetId(t *testing.T) {
	model := &DataModel{
		ID: types.StringValue("test-cluster-id"),
	}

	assert.Equal(t, "test-cluster-id", model.GetID())
}

func TestDataModel_GetUpdatedModel_BasicFields(t *testing.T) {
	ctx := context.Background()

	createdAt := timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC))
	cluster := &controlplanev1.Cluster{
		Id:                    "test-id",
		Name:                  "test-cluster",
		ConnectionType:        controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
		CloudProvider:         controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
		Type:                  controlplanev1.Cluster_TYPE_DEDICATED,
		ThroughputTier:        "tier-1",
		Region:                "us-east-1",
		ResourceGroupId:       "rg-123",
		NetworkId:             "net-456",
		State:                 controlplanev1.Cluster_STATE_READY,
		Zones:                 []string{"us-east-1a", "us-east-1b"},
		ReadReplicaClusterIds: []string{"replica-1", "replica-2"},
		CreatedAt:             createdAt,
		DataplaneApi: &controlplanev1.Cluster_DataplaneAPI{
			Url: "https://dataplane.example.com",
		},
	}

	contingent := ContingentFields{
		RedpandaVersion:       types.StringValue("v23.1.1"),
		AllowDeletion:         types.BoolValue(true),
		Tags:                  types.MapValueMust(types.StringType, map[string]attr.Value{"env": types.StringValue("test")}),
		GcpGlobalAccessConfig: types.BoolNull(),
	}

	model := &DataModel{}
	result, diags := model.GetUpdatedModel(ctx, cluster, contingent)

	require.False(t, diags.HasError(), "GetUpdatedModel should not have errors")
	require.NotNil(t, result)
	assert.Equal(t, "test-id", result.ID.ValueString())
	assert.Equal(t, "test-cluster", result.Name.ValueString())
	assert.Equal(t, "public", result.ConnectionType.ValueString())
	assert.Equal(t, "aws", result.CloudProvider.ValueString())
	assert.Equal(t, "dedicated", result.ClusterType.ValueString())
	assert.Equal(t, "tier-1", result.ThroughputTier.ValueString())
	assert.Equal(t, "us-east-1", result.Region.ValueString())
	assert.Equal(t, "rg-123", result.ResourceGroupID.ValueString())
	assert.Equal(t, "net-456", result.NetworkID.ValueString())
	assert.Equal(t, "STATE_READY", result.State.ValueString())
	assert.Equal(t, "v23.1.1", result.RedpandaVersion.ValueString())
	assert.True(t, result.AllowDeletion.ValueBool())
	assert.Equal(t, "2023-01-01T12:00:00Z", result.CreatedAt.ValueString())
	assert.Equal(t, "https://dataplane.example.com", result.ClusterAPIURL.ValueString())
}

func TestDataModel_GetUpdatedModel_ComprehensiveExamples(t *testing.T) {
	ctx := context.Background()
	createdTime := time.Now()

	tests := []struct {
		name       string
		cluster    *controlplanev1.Cluster
		contingent ContingentFields
		validate   func(t *testing.T, result *DataModel)
	}{
		{
			name: "aws_dedicated_public_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-abc123def",
				Name:            "testname",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:            controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier:  "tier-1-aws-v2-arm",
				Region:          "us-east-2",
				Zones:           []string{"use2-az1", "use2-az2", "use2-az3"},
				ResourceGroupId: "rg-123456",
				NetworkId:       "net-789012",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				DataplaneApi: &controlplanev1.Cluster_DataplaneAPI{
					Url: "https://testname.us-east-2.aws.redpanda.cloud:443",
				},
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"key": types.StringValue("value"),
				}),
			},
			validate: func(t *testing.T, result *DataModel) {
				require.Equal(t, "rp-abc123def", result.ID.ValueString())
				require.Equal(t, "testname", result.Name.ValueString())
				require.Equal(t, "public", result.ConnectionType.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "dedicated", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-aws-v2-arm", result.ThroughputTier.ValueString())
				require.Equal(t, "us-east-2", result.Region.ValueString())
				require.Equal(t, "https://testname.us-east-2.aws.redpanda.cloud:443", result.ClusterAPIURL.ValueString())
				// Dedicated clusters should not have CustomerManagedResources
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "gcp_byovpc_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-gcp789xyz",
				Name:            "testname",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:            controlplanev1.Cluster_TYPE_BYOC,
				ThroughputTier:  "tier-1-gcp-um4g",
				Region:          "us-central1",
				Zones:           []string{"us-central1-a", "us-central1-b", "us-central1-c"},
				ResourceGroupId: "rg-456789",
				NetworkId:       "net-345678",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.CustomerManagedResources_GCP{
							Subnet: &controlplanev1.CustomerManagedResources_GCP_Subnet{
								Name: "redpanda-subnet-testname",
								SecondaryIpv4RangePods: &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
									Name: "redpanda-pods-testname",
								},
								SecondaryIpv4RangeServices: &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
									Name: "redpanda-services-testname",
								},
								K8SMasterIpv4Range: "10.0.7.240/28",
							},
							AgentServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-agent-testname@project.iam.gserviceaccount.com",
							},
							ConsoleServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-console-testname@project.iam.gserviceaccount.com",
							},
							ConnectorServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-connector-testname@project.iam.gserviceaccount.com",
							},
							RedpandaClusterServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-cluster-testname@project.iam.gserviceaccount.com",
							},
							GkeServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-gke-testname@project.iam.gserviceaccount.com",
							},
							TieredStorageBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
								Name: "redpanda-storage-testname",
							},
						},
					},
				},
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"environment": types.StringValue("dev"),
					"managed-by":  types.StringValue("terraform"),
				}),
			},
			validate: func(t *testing.T, result *DataModel) {
				require.Equal(t, "rp-gcp789xyz", result.ID.ValueString())
				require.Equal(t, "testname", result.Name.ValueString())
				require.Equal(t, "private", result.ConnectionType.ValueString())
				require.Equal(t, "gcp", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-gcp-um4g", result.ThroughputTier.ValueString())
				require.Equal(t, "us-central1", result.Region.ValueString())
				// BYOC clusters with private connection should have CustomerManagedResources
				require.False(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "azure_dedicated_public_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-azure123def",
				Name:            "azure-testname",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
				Type:            controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier:  "tier-1-azure-v3-x86",
				Region:          "eastus",
				Zones:           []string{"eastus-az1", "eastus-az2", "eastus-az3"},
				ResourceGroupId: "rg-azure123",
				NetworkId:       "net-azure123",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"environment": types.StringValue("production"),
				}),
			},
			validate: func(t *testing.T, result *DataModel) {
				require.Equal(t, "rp-azure123def", result.ID.ValueString())
				require.Equal(t, "azure-testname", result.Name.ValueString())
				require.Equal(t, "public", result.ConnectionType.ValueString())
				require.Equal(t, "azure", result.CloudProvider.ValueString())
				require.Equal(t, "dedicated", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-azure-v3-x86", result.ThroughputTier.ValueString())
				require.Equal(t, "eastus", result.Region.ValueString())
				// Dedicated clusters should not have CustomerManagedResources
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "aws_byoc_public_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-awsbyoc123",
				Name:            "aws-byoc-cluster",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:            controlplanev1.Cluster_TYPE_BYOC,
				ThroughputTier:  "tier-1-aws-v2-x86",
				Region:          "us-east-1",
				Zones:           []string{"use1-az2", "use1-az4", "use1-az6"},
				ResourceGroupId: "rg-byoc123",
				NetworkId:       "net-byoc123",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(false),
				Tags:            types.MapNull(types.StringType),
			},
			validate: func(t *testing.T, result *DataModel) {
				require.Equal(t, "rp-awsbyoc123", result.ID.ValueString())
				require.Equal(t, "aws-byoc-cluster", result.Name.ValueString())
				require.Equal(t, "public", result.ConnectionType.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-aws-v2-x86", result.ThroughputTier.ValueString())
				require.Equal(t, "us-east-1", result.Region.ValueString())
				require.True(t, result.Tags.IsNull())
				// Public BYOC clusters should not have CustomerManagedResources
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "gcp_dedicated_public_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-gcpdedicated",
				Name:            "gcp-dedicated-cluster",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:            controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier:  "tier-1-gcp-um4g",
				Region:          "us-central1",
				Zones:           []string{"us-central1-a", "us-central1-b", "us-central1-c"},
				ResourceGroupId: "rg-gcpdedicated",
				NetworkId:       "net-gcpdedicated",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"team":        types.StringValue("data-platform"),
					"cost-center": types.StringValue("engineering"),
				}),
			},
			validate: func(t *testing.T, result *DataModel) {
				require.Equal(t, "rp-gcpdedicated", result.ID.ValueString())
				require.Equal(t, "gcp-dedicated-cluster", result.Name.ValueString())
				require.Equal(t, "public", result.ConnectionType.ValueString())
				require.Equal(t, "gcp", result.CloudProvider.ValueString())
				require.Equal(t, "dedicated", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-gcp-um4g", result.ThroughputTier.ValueString())
				require.Equal(t, "us-central1", result.Region.ValueString())
				// Dedicated clusters should not have CustomerManagedResources
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "aws_byovpc_private_cluster_with_full_cmr",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-awsbyovpc123",
				Name:            "aws-byovpc-cluster",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:            controlplanev1.Cluster_TYPE_BYOC,
				ThroughputTier:  "tier-1-aws-v2-arm",
				Region:          "us-east-2",
				Zones:           []string{"use2-az1", "use2-az2", "use2-az3"},
				ResourceGroupId: "rg-awsbyovpc",
				NetworkId:       "net-awsbyovpc",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Aws{
						Aws: &controlplanev1.CustomerManagedResources_AWS{
							AgentInstanceProfile: &controlplanev1.AWSInstanceProfile{
								Arn: "arn:aws:iam::123456789012:instance-profile/redpanda-byovpc-agent-instance-profile",
							},
							CloudStorageBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
								Arn: "arn:aws:s3:::redpanda-byovpc-cloud-storage-bucket",
							},
							K8SClusterRole: &controlplanev1.CustomerManagedResources_AWS_Role{
								Arn: "arn:aws:iam::123456789012:role/redpanda-byovpc-k8s-cluster-role",
							},
							RedpandaAgentSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:us-east-2:123456789012:security-group/sg-agent123",
							},
							ClusterSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:us-east-2:123456789012:security-group/sg-cluster123",
							},
						},
					},
				},
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"deployment": types.StringValue("byovpc"),
					"managed-by": types.StringValue("terraform"),
				}),
			},
			validate: func(t *testing.T, result *DataModel) {
				require.Equal(t, "rp-awsbyovpc123", result.ID.ValueString())
				require.Equal(t, "aws-byovpc-cluster", result.Name.ValueString())
				require.Equal(t, "private", result.ConnectionType.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-aws-v2-arm", result.ThroughputTier.ValueString())
				require.Equal(t, "us-east-2", result.Region.ValueString())
				// Private BYOC clusters should have CustomerManagedResources
				require.False(t, result.CustomerManagedResources.IsNull())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &DataModel{}
			result, diags := model.GetUpdatedModel(ctx, tt.cluster, tt.contingent)

			require.False(t, diags.HasError())
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestDataModel_GenerateStateDescription(t *testing.T) {
	model := &DataModel{}

	t.Run("with state description", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			StateDescription: &status.Status{
				Message: "Cluster is ready",
				Code:    200,
			},
		}

		result, diags := model.generateModelStateDescription(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())
		attrs := result.Attributes()
		messageAttr, ok := attrs["message"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "Cluster is ready", messageAttr.ValueString())
		codeAttr, ok := attrs["code"].(types.Int32)
		require.True(t, ok)
		assert.Equal(t, int32(200), codeAttr.ValueInt32())
	})

	t.Run("without state description", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}
		result, diags := model.generateModelStateDescription(cluster)
		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateKafkaAPI(t *testing.T) {
	model := &DataModel{}

	t.Run("with kafka api", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			KafkaApi: &controlplanev1.Cluster_KafkaAPI{
				SeedBrokers: []string{"broker1:9092", "broker2:9092"},
				Mtls: &controlplanev1.MTLSSpec{
					Enabled:               true,
					CaCertificatesPem:     []string{"cert1", "cert2"},
					PrincipalMappingRules: []string{"rule1", "rule2"},
				},
			},
		}

		result, diags := model.generateModelKafkaAPI(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		seedBrokers, ok := attrs["seed_brokers"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 2, len(seedBrokers.Elements()))

		mtls, ok := attrs["mtls"].(types.Object)
		require.True(t, ok)
		assert.False(t, mtls.IsNull())
	})

	t.Run("without kafka api", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelKafkaAPI(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateMtlsModel(t *testing.T) {
	model := &DataModel{}

	t.Run("with mtls spec", func(t *testing.T) {
		mtls := &controlplanev1.MTLSSpec{
			Enabled:               true,
			CaCertificatesPem:     []string{"cert1", "cert2"},
			PrincipalMappingRules: []string{"rule1", "rule2"},
		}

		result, diags := model.generateModelMtls(mtls)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		enabledAttr, ok := attrs["enabled"].(types.Bool)
		require.True(t, ok)
		assert.True(t, enabledAttr.ValueBool())

		caCerts, ok := attrs["ca_certificates_pem"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 2, len(caCerts.Elements()))

		rules, ok := attrs["principal_mapping_rules"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 2, len(rules.Elements()))
	})

	t.Run("with nil mtls", func(t *testing.T) {
		result, diags := model.generateModelMtls(nil)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateKafkaConnect(t *testing.T) {
	model := &DataModel{}

	t.Run("with enabled kafka connect", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			KafkaConnect: &controlplanev1.KafkaConnect{
				Enabled: true,
			},
		}

		result, diags := model.generateModelKafkaConnect(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		enabledAttr, ok := attrs["enabled"].(types.Bool)
		require.True(t, ok)
		assert.True(t, enabledAttr.ValueBool())
	})

	t.Run("with disabled kafka connect", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			KafkaConnect: &controlplanev1.KafkaConnect{
				Enabled: false,
			},
		}

		result, diags := model.generateModelKafkaConnect(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GeneratePrometheus(t *testing.T) {
	model := &DataModel{}

	t.Run("with prometheus", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			Prometheus: &controlplanev1.Cluster_Prometheus{
				Url: "https://prometheus.example.com",
			},
		}

		result, diags := model.generateModelPrometheus(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		urlAttr, ok := attrs["url"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "https://prometheus.example.com", urlAttr.ValueString())
	})

	t.Run("without prometheus", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelPrometheus(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateRedpandaConsole(t *testing.T) {
	model := &DataModel{}

	t.Run("with console", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			RedpandaConsole: &controlplanev1.Cluster_RedpandaConsole{
				Url: "https://console.example.com",
			},
		}

		result, diags := model.generateModelRedpandaConsole(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		urlAttr, ok := attrs["url"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "https://console.example.com", urlAttr.ValueString())
	})

	t.Run("without console", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelRedpandaConsole(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateSchemaRegistry(t *testing.T) {
	model := &DataModel{}

	t.Run("with schema registry", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			SchemaRegistry: &controlplanev1.Cluster_SchemaRegistryStatus{
				Url: "https://schema.example.com",
				Mtls: &controlplanev1.MTLSSpec{
					Enabled: true,
				},
			},
		}

		result, diags := model.generateModelSchemaRegistry(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		urlAttr, ok := attrs["url"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "https://schema.example.com", urlAttr.ValueString())

		mtls, ok := attrs["mtls"].(types.Object)
		require.True(t, ok)
		assert.False(t, mtls.IsNull())
	})

	t.Run("without schema registry", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelSchemaRegistry(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateHTTPProxy(t *testing.T) {
	model := &DataModel{}

	t.Run("with http proxy", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			HttpProxy: &controlplanev1.Cluster_HTTPProxyStatus{
				Url: "https://proxy.example.com",
				Mtls: &controlplanev1.MTLSSpec{
					Enabled: true,
				},
			},
		}

		result, diags := model.generateModelHTTPProxy(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		urlAttr, ok := attrs["url"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "https://proxy.example.com", urlAttr.ValueString())

		mtls, ok := attrs["mtls"].(types.Object)
		require.True(t, ok)
		assert.False(t, mtls.IsNull())
	})

	t.Run("without http proxy", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelHTTPProxy(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateMaintenanceWindow(t *testing.T) {
	model := &DataModel{}

	t.Run("with day hour window", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			MaintenanceWindowConfig: &controlplanev1.MaintenanceWindowConfig{
				Window: &controlplanev1.MaintenanceWindowConfig_DayHour_{
					DayHour: &controlplanev1.MaintenanceWindowConfig_DayHour{
						HourOfDay: 14,
						DayOfWeek: dayofweek.DayOfWeek_MONDAY,
					},
				},
			},
		}
		result, diags := model.generateModelMaintenanceWindow(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		dayHour, ok := attrs["day_hour"].(types.Object)
		require.True(t, ok)
		assert.False(t, dayHour.IsNull())

		dayHourAttrs := dayHour.Attributes()
		hourAttr, ok := dayHourAttrs["hour_of_day"].(types.Int32)
		require.True(t, ok)
		assert.Equal(t, int32(14), hourAttr.ValueInt32())
		dayAttr, ok := dayHourAttrs["day_of_week"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "MONDAY", dayAttr.ValueString())

		anytimeAttr, ok := attrs["anytime"].(types.Bool)
		require.True(t, ok)
		assert.True(t, anytimeAttr.IsNull())
		unspecifiedAttr, ok := attrs["unspecified"].(types.Bool)
		require.True(t, ok)
		assert.True(t, unspecifiedAttr.IsNull())
	})

	t.Run("with anytime window", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			MaintenanceWindowConfig: &controlplanev1.MaintenanceWindowConfig{
				Window: &controlplanev1.MaintenanceWindowConfig_Anytime_{
					Anytime: &controlplanev1.MaintenanceWindowConfig_Anytime{},
				},
			},
		}

		result, diags := model.generateModelMaintenanceWindow(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		dayHourAttr, ok := attrs["day_hour"].(types.Object)
		require.True(t, ok)
		assert.True(t, dayHourAttr.IsNull())
		anytimeAttr, ok := attrs["anytime"].(types.Bool)
		require.True(t, ok)
		assert.True(t, anytimeAttr.ValueBool())
		unspecifiedAttr, ok := attrs["unspecified"].(types.Bool)
		require.True(t, ok)
		assert.True(t, unspecifiedAttr.IsNull())
	})

	t.Run("without maintenance window", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelMaintenanceWindow(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateAWSPrivateLink(t *testing.T) {
	model := &DataModel{}

	t.Run("with complete AWS private link", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			AwsPrivateLink: &controlplanev1.Cluster_AWSPrivateLink{
				Enabled:        true,
				ConnectConsole: true,
				AllowedPrincipals: []string{
					"arn:aws:iam::123456789012:user/test-user",
					"arn:aws:iam::123456789012:role/test-role",
				},
				Status: &controlplanev1.Cluster_AWSPrivateLink_Status{
					ServiceId:    "vpce-svc-12345",
					ServiceName:  "com.amazonaws.vpce.us-east-1.vpce-svc-12345",
					ServiceState: "Available",
					CreatedAt:    timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
					DeletedAt:    timestamppb.New(time.Date(2023, 12, 31, 12, 0, 0, 0, time.UTC)),
					VpcEndpointConnections: []*controlplanev1.Cluster_AWSPrivateLink_Status_VPCEndpointConnection{
						{
							Id:           "vpce-12345",
							Owner:        "123456789012",
							State:        "available",
							CreatedAt:    timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
							ConnectionId: "vpce-conn-12345",
							LoadBalancerArns: []string{
								"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-lb/1234567890abcdef",
							},
							DnsEntries: []*controlplanev1.Cluster_AWSPrivateLink_Status_VPCEndpointConnection_DNSEntry{
								{
									DnsName:      "test.dns.com",
									HostedZoneId: "Z123456789",
								},
							},
						},
					},
					KafkaApiSeedPort:          9092,
					SchemaRegistrySeedPort:    8081,
					RedpandaProxySeedPort:     8082,
					KafkaApiNodeBasePort:      31092,
					RedpandaProxyNodeBasePort: 31082,
					ConsolePort:               8080,
				},
			},
		}

		result, diags := model.generateModelAWSPrivateLink(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		enabledAttr, ok := attrs["enabled"].(types.Bool)
		require.True(t, ok)
		assert.True(t, enabledAttr.ValueBool())
		connectConsoleAttr, ok := attrs["connect_console"].(types.Bool)
		require.True(t, ok)
		assert.True(t, connectConsoleAttr.ValueBool())

		allowedPrincipals, ok := attrs["allowed_principals"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 2, len(allowedPrincipals.Elements()))

		status, ok := attrs["status"].(types.Object)
		require.True(t, ok)
		assert.False(t, status.IsNull())
		statusAttrs := status.Attributes()
		serviceIDAttr, ok := statusAttrs["service_id"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "vpce-svc-12345", serviceIDAttr.ValueString())
		serviceNameAttr, ok := statusAttrs["service_name"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "com.amazonaws.vpce.us-east-1.vpce-svc-12345", serviceNameAttr.ValueString())
		serviceStateAttr, ok := statusAttrs["service_state"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "Available", serviceStateAttr.ValueString())
		kafkaAPISeedPortAttr, ok := statusAttrs["kafka_api_seed_port"].(types.Int32)
		require.True(t, ok)
		assert.Equal(t, int32(9092), kafkaAPISeedPortAttr.ValueInt32())

		vpcConnections, ok := statusAttrs["vpc_endpoint_connections"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 1, len(vpcConnections.Elements()))
	})

	t.Run("with disabled AWS private link", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			AwsPrivateLink: &controlplanev1.Cluster_AWSPrivateLink{
				Enabled: false,
			},
		}

		result, diags := model.generateModelAWSPrivateLink(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})

	t.Run("without AWS private link", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelAWSPrivateLink(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateGCPPrivateServiceConnect(t *testing.T) {
	model := &DataModel{}

	t.Run("with complete GCP private service connect", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			GcpPrivateServiceConnect: &controlplanev1.Cluster_GCPPrivateServiceConnect{
				Enabled:             true,
				GlobalAccessEnabled: true,
				ConsumerAcceptList: []*controlplanev1.GCPPrivateServiceConnectConsumer{
					{Source: "projects/123456789012"},
					{Source: "projects/210987654321"},
				},
				Status: &controlplanev1.Cluster_GCPPrivateServiceConnect_Status{
					ServiceAttachment:         "projects/test-project/regions/us-central1/serviceAttachments/test-sa",
					CreatedAt:                 timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
					DeletedAt:                 timestamppb.New(time.Date(2023, 12, 31, 12, 0, 0, 0, time.UTC)),
					KafkaApiSeedPort:          9092,
					SchemaRegistrySeedPort:    8081,
					RedpandaProxySeedPort:     8082,
					KafkaApiNodeBasePort:      31092,
					RedpandaProxyNodeBasePort: 31082,
					ConnectedEndpoints: []*controlplanev1.Cluster_GCPPrivateServiceConnect_Status_ConnectedEndpoint{
						{
							ConnectionId:    "conn-12345",
							ConsumerNetwork: "projects/consumer-project/global/networks/default",
							Endpoint:        "10.0.0.100",
							Status:          "ACCEPTED",
						},
					},
					DnsARecords:  []string{"10.0.0.100", "10.0.0.101"},
					SeedHostname: "seed.redpanda.com",
				},
			},
		}

		result, diags := model.generateModelGCPPrivateServiceConnect(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		enabledAttr, ok := attrs["enabled"].(types.Bool)
		require.True(t, ok)
		assert.True(t, enabledAttr.ValueBool())
		globalAccessEnabledAttr, ok := attrs["global_access_enabled"].(types.Bool)
		require.True(t, ok)
		assert.True(t, globalAccessEnabledAttr.ValueBool())

		consumerAcceptList, ok := attrs["consumer_accept_list"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 2, len(consumerAcceptList.Elements()))

		status, ok := attrs["status"].(types.Object)
		require.True(t, ok)
		assert.False(t, status.IsNull())
		statusAttrs := status.Attributes()
		serviceAttachmentAttr, ok := statusAttrs["service_attachment"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "projects/test-project/regions/us-central1/serviceAttachments/test-sa", serviceAttachmentAttr.ValueString())
		kafkaAPISeedPortAttr, ok := statusAttrs["kafka_api_seed_port"].(types.Int32)
		require.True(t, ok)
		assert.Equal(t, int32(9092), kafkaAPISeedPortAttr.ValueInt32())
		seedHostnameAttr, ok := statusAttrs["seed_hostname"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "seed.redpanda.com", seedHostnameAttr.ValueString())

		connectedEndpoints, ok := statusAttrs["connected_endpoints"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 1, len(connectedEndpoints.Elements()))

		dnsRecords, ok := statusAttrs["dns_a_records"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 2, len(dnsRecords.Elements()))
	})

	t.Run("with disabled GCP private service connect", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			GcpPrivateServiceConnect: &controlplanev1.Cluster_GCPPrivateServiceConnect{
				Enabled: false,
			},
		}

		result, diags := model.generateModelGCPPrivateServiceConnect(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})

	t.Run("without GCP private service connect", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelGCPPrivateServiceConnect(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateAzurePrivateLink(t *testing.T) {
	model := &DataModel{}

	t.Run("with complete Azure private link", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			AzurePrivateLink: &controlplanev1.Cluster_AzurePrivateLink{
				Enabled:        true,
				ConnectConsole: true,
				AllowedSubscriptions: []string{
					"12345678-1234-1234-1234-123456789012",
					"87654321-4321-4321-4321-210987654321",
				},
				Status: &controlplanev1.Cluster_AzurePrivateLink_Status{
					ServiceId:   "test-service-id",
					ServiceName: "test-service-name",
					CreatedAt:   timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
					DeletedAt:   timestamppb.New(time.Date(2023, 12, 31, 12, 0, 0, 0, time.UTC)),
					PrivateEndpointConnections: []*controlplanev1.Cluster_AzurePrivateLink_Status_PrivateEndpointConnection{
						{
							PrivateEndpointName: "test-endpoint",
							PrivateEndpointId:   "endpoint-12345",
							ConnectionName:      "test-connection",
							ConnectionId:        "conn-12345",
							Status:              "Approved",
							CreatedAt:           timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
						},
					},
					DnsARecord:                "10.0.0.100",
					ApprovedSubscriptions:     []string{"12345678-1234-1234-1234-123456789012"},
					KafkaApiSeedPort:          9092,
					SchemaRegistrySeedPort:    8081,
					RedpandaProxySeedPort:     8082,
					KafkaApiNodeBasePort:      31092,
					RedpandaProxyNodeBasePort: 31082,
					ConsolePort:               8080,
				},
			},
		}

		result, diags := model.generateModelAzurePrivateLink(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		enabledAttr, ok := attrs["enabled"].(types.Bool)
		require.True(t, ok)
		assert.True(t, enabledAttr.ValueBool())
		connectConsoleAttr, ok := attrs["connect_console"].(types.Bool)
		require.True(t, ok)
		assert.True(t, connectConsoleAttr.ValueBool())

		allowedSubscriptions, ok := attrs["allowed_subscriptions"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 2, len(allowedSubscriptions.Elements()))

		status, ok := attrs["status"].(types.Object)
		require.True(t, ok)
		assert.False(t, status.IsNull())
		statusAttrs := status.Attributes()
		serviceIDAttr, ok := statusAttrs["service_id"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "test-service-id", serviceIDAttr.ValueString())
		serviceNameAttr, ok := statusAttrs["service_name"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "test-service-name", serviceNameAttr.ValueString())
		dnsARecordAttr, ok := statusAttrs["dns_a_record"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "10.0.0.100", dnsARecordAttr.ValueString())
		kafkaAPISeedPortAttr, ok := statusAttrs["kafka_api_seed_port"].(types.Int32)
		require.True(t, ok)
		assert.Equal(t, int32(9092), kafkaAPISeedPortAttr.ValueInt32())

		privateEndpointConnections, ok := statusAttrs["private_endpoint_connections"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 1, len(privateEndpointConnections.Elements()))

		approvedSubscriptions, ok := statusAttrs["approved_subscriptions"].(types.List)
		require.True(t, ok)
		assert.Equal(t, 1, len(approvedSubscriptions.Elements()))
	})

	t.Run("with disabled Azure private link", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			AzurePrivateLink: &controlplanev1.Cluster_AzurePrivateLink{
				Enabled: false,
			},
		}

		result, diags := model.generateModelAzurePrivateLink(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})

	t.Run("without Azure private link", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelAzurePrivateLink(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}

func TestDataModel_GenerateCustomerManagedResources(t *testing.T) {
	model := &DataModel{}

	t.Run("with AWS CMR for BYOC cluster", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
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
		}

		result, diags := model.generateModelCustomerManagedResources(cluster)

		require.False(t, diags.HasError())
		require.False(t, result.IsNull())

		attrs := result.Attributes()

		awsObj, ok := attrs["aws"].(types.Object)
		require.True(t, ok)
		assert.False(t, awsObj.IsNull())
		awsAttrs := awsObj.Attributes()

		agentProfile, ok := awsAttrs["agent_instance_profile"].(types.Object)
		require.True(t, ok)
		assert.False(t, agentProfile.IsNull())
		agentArnAttr, ok := agentProfile.Attributes()["arn"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "arn:aws:iam::123456789012:instance-profile/agent",
			agentArnAttr.ValueString())

		bucket, ok := awsAttrs["cloud_storage_bucket"].(types.Object)
		require.True(t, ok)
		assert.False(t, bucket.IsNull())
		bucketArnAttr, ok := bucket.Attributes()["arn"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "arn:aws:s3:::my-bucket",
			bucketArnAttr.ValueString())

		gcpObj, ok := attrs["gcp"].(types.Object)
		require.True(t, ok)
		assert.True(t, gcpObj.IsNull())
	})

	t.Run("with GCP CMR for BYOC cluster", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
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
						PscNatSubnetName: "psc-nat-subnet",
					},
				},
			},
		}

		result, diags := model.generateModelCustomerManagedResources(cluster)

		require.False(t, diags.HasError())
		require.False(t, result.IsNull())

		attrs := result.Attributes()

		gcpObj, ok := attrs["gcp"].(types.Object)
		require.True(t, ok)
		assert.False(t, gcpObj.IsNull())
		gcpAttrs := gcpObj.Attributes()

		subnet, ok := gcpAttrs["subnet"].(types.Object)
		require.True(t, ok)
		assert.False(t, subnet.IsNull())
		subnetAttrs := subnet.Attributes()
		subnetNameAttr, ok := subnetAttrs["name"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "test-subnet", subnetNameAttr.ValueString())
		k8sMasterIpv4RangeAttr, ok := subnetAttrs["k8s_master_ipv4_range"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "10.0.0.0/28", k8sMasterIpv4RangeAttr.ValueString())

		podsRange, ok := subnetAttrs["secondary_ipv4_range_pods"].(types.Object)
		require.True(t, ok)
		assert.False(t, podsRange.IsNull())
		podsRangeNameAttr, ok := podsRange.Attributes()["name"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "pods-range", podsRangeNameAttr.ValueString())

		servicesRange, ok := subnetAttrs["secondary_ipv4_range_services"].(types.Object)
		require.True(t, ok)
		assert.False(t, servicesRange.IsNull())
		servicesRangeNameAttr, ok := servicesRange.Attributes()["name"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "services-range", servicesRangeNameAttr.ValueString())

		agentSA, ok := gcpAttrs["agent_service_account"].(types.Object)
		require.True(t, ok)
		assert.False(t, agentSA.IsNull())
		agentSAEmailAttr, ok := agentSA.Attributes()["email"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "agent-sa@project-id.iam.gserviceaccount.com",
			agentSAEmailAttr.ValueString())

		bucket, ok := gcpAttrs["tiered_storage_bucket"].(types.Object)
		require.True(t, ok)
		assert.False(t, bucket.IsNull())
		bucketNameAttr, ok := bucket.Attributes()["name"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "redpanda-tiered-storage-bucket",
			bucketNameAttr.ValueString())

		pscNatSubnetNameAttr, ok := gcpAttrs["psc_nat_subnet_name"].(types.String)
		require.True(t, ok)
		assert.Equal(t, "psc-nat-subnet", pscNatSubnetNameAttr.ValueString())

		awsObj, ok := attrs["aws"].(types.Object)
		require.True(t, ok)
		assert.True(t, awsObj.IsNull())
	})

	t.Run("without CMR", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelCustomerManagedResources(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})

	t.Run("non-BYOC cluster with CMR returns error", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
			Type:          controlplanev1.Cluster_TYPE_DEDICATED,
			CustomerManagedResources: &controlplanev1.CustomerManagedResources{
				CloudProvider: &controlplanev1.CustomerManagedResources_Aws{},
			},
		}

		result, diags := model.generateModelCustomerManagedResources(cluster)

		assert.True(t, diags.HasError())
		assert.True(t, result.IsNull())
		assert.Contains(t, diags.Errors()[0].Summary(), "Customer Managed Resources with non-BYOC cluster type")
	})
}

func TestDataModel_GenerateClusterConfiguration(t *testing.T) {
	model := &DataModel{}

	t.Run("with cluster configuration and custom properties", func(t *testing.T) {
		customProps, err := structpb.NewStruct(map[string]any{
			"auto.create.topics.enable": true,
			"log.segment.bytes":         "1073741824",
			"retention.ms":              "604800000",
			"compaction.type":           "cleanup",
		})
		require.NoError(t, err)

		cluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{
				CustomProperties: customProps,
			},
		}

		result, diags := model.generateModelClusterConfiguration(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		customPropsJSONAttr, ok := attrs["custom_properties_json"].(types.String)
		require.True(t, ok)

		// Parse the JSON to verify it contains expected properties
		jsonStr := customPropsJSONAttr.ValueString()
		assert.Contains(t, jsonStr, "auto.create.topics.enable")
		assert.Contains(t, jsonStr, "log.segment.bytes")
		assert.Contains(t, jsonStr, "retention.ms")
		assert.Contains(t, jsonStr, "compaction.type")

		// Verify it's valid JSON
		var parsedJSON map[string]any
		err = json.Unmarshal([]byte(jsonStr), &parsedJSON)
		require.NoError(t, err)

		// Verify specific values
		assert.Equal(t, true, parsedJSON["auto.create.topics.enable"])
		assert.Equal(t, "1073741824", parsedJSON["log.segment.bytes"])
		assert.Equal(t, "604800000", parsedJSON["retention.ms"])
		assert.Equal(t, "cleanup", parsedJSON["compaction.type"])
	})

	t.Run("with cluster configuration but no custom properties", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{
				// No custom properties set
			},
		}

		result, diags := model.generateModelClusterConfiguration(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})

	t.Run("with cluster configuration and empty custom properties", func(t *testing.T) {
		customProps, err := structpb.NewStruct(map[string]any{})
		require.NoError(t, err)

		cluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{
				CustomProperties: customProps,
			},
		}

		result, diags := model.generateModelClusterConfiguration(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull(), "Empty custom properties should return null to avoid plan/apply consistency issues")
	})

	t.Run("with complex nested custom properties", func(t *testing.T) {
		customProps, err := structpb.NewStruct(map[string]any{
			"redpanda.enable_transactions":    true,
			"redpanda.transaction_timeout_ms": 60000,
			"kafka_batch_max_bytes":           1048576,
			"cluster_id":                      "test-cluster-123",
			"security": map[string]any{
				"enable_sasl":     true,
				"mechanism_scram": "SCRAM-SHA-256",
				"mechanism_plain": "PLAIN",
			},
			"quotas": map[string]any{
				"producer_byte_rate": 1048576,
				"consumer_byte_rate": 2097152,
			},
		})
		require.NoError(t, err)

		cluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{
				CustomProperties: customProps,
			},
		}

		result, diags := model.generateModelClusterConfiguration(cluster)

		assert.False(t, diags.HasError())
		assert.False(t, result.IsNull())

		attrs := result.Attributes()
		customPropsJSONAttr, ok := attrs["custom_properties_json"].(types.String)
		require.True(t, ok)

		// Parse the JSON to verify nested structure
		jsonStr := customPropsJSONAttr.ValueString()
		var parsedJSON map[string]any
		err = json.Unmarshal([]byte(jsonStr), &parsedJSON)
		require.NoError(t, err)

		// Verify top-level properties
		assert.Equal(t, true, parsedJSON["redpanda.enable_transactions"])
		assert.Equal(t, float64(60000), parsedJSON["redpanda.transaction_timeout_ms"])
		assert.Equal(t, float64(1048576), parsedJSON["kafka_batch_max_bytes"])
		assert.Equal(t, "test-cluster-123", parsedJSON["cluster_id"])

		// Verify nested objects exist
		security, ok := parsedJSON["security"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, security["enable_sasl"])
		assert.Equal(t, "SCRAM-SHA-256", security["mechanism_scram"])
		assert.Equal(t, "PLAIN", security["mechanism_plain"])

		quotas, ok := parsedJSON["quotas"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(1048576), quotas["producer_byte_rate"])
		assert.Equal(t, float64(2097152), quotas["consumer_byte_rate"])
	})

	t.Run("without cluster configuration", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			// No cluster configuration
		}

		result, diags := model.generateModelClusterConfiguration(cluster)

		assert.False(t, diags.HasError())
		assert.True(t, result.IsNull())
	})
}
