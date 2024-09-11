package cluster

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
)

func TestGenerateClusterRequest(t *testing.T) {
	type args struct {
		model models.Cluster
	}
	rpVersion := "v24.1.1"
	tests := []struct {
		name string
		args args
		want *controlplanev1beta2.ClusterCreate
	}{
		{
			name: "validate_schema",
			args: args{
				model: models.Cluster{
					Name:            types.StringValue("testname"),
					RedpandaVersion: types.StringValue(rpVersion),
					ConnectionType:  types.StringValue("public"),
					CloudProvider:   types.StringValue("gcp"),
					ClusterType:     types.StringValue("dedicated"),
					ThroughputTier:  types.StringValue("tier-2-gcp-um50"),
					Region:          types.StringValue("us-west1"),
					Zones:           utils.TestingOnlyStringSliceToTypeList([]string{"us-west1-a", "us-west1-b"}),
					AllowDeletion:   types.BoolValue(true),
					ResourceGroupID: types.StringValue("testrg"),
					NetworkID:       types.StringValue("testnetwork"),
				},
			},
			want: &controlplanev1beta2.ClusterCreate{
				Name:            "testname",
				RedpandaVersion: &rpVersion,
				ConnectionType:  controlplanev1beta2.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:            controlplanev1beta2.Cluster_TYPE_DEDICATED,
				ThroughputTier:  "tier-2-gcp-um50",
				Region:          "us-west1",
				Zones:           []string{"us-west1-a", "us-west1-b"},
				ResourceGroupId: "testrg",
				NetworkId:       "testnetwork",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := generateClusterRequest(tt.args.model); !reflect.DeepEqual(got, tt.want) {
				fmt.Println("got")
				spew.Dump(got)
				fmt.Println("want")
				spew.Dump(tt.want)
				t.Errorf("generateClusterRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateModel(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name     string
		cfg      models.Cluster
		cluster  *controlplanev1beta2.Cluster
		expected *models.Cluster
		wantErr  bool
	}{
		{
			name: "Basic cluster",
			cfg: models.Cluster{
				Name:            types.StringValue("test-cluster"),
				ConnectionType:  types.StringValue("public"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("dedicated"),
				RedpandaVersion: types.StringValue("v22.3.11"),
				ThroughputTier:  types.StringValue("t1"),
				Region:          types.StringValue("us-west-2"),
				ResourceGroupID: types.StringValue("rg-123"),
				NetworkID:       types.StringValue("net-456"),
				Zones:           utils.TestingOnlyStringSliceToTypeList([]string{"us-west-2a", "us-west-2b"}),
				AllowDeletion:   types.BoolValue(false),
			},
			cluster: &controlplanev1beta2.Cluster{
				Id:              "cl-789",
				Name:            "test-cluster",
				ConnectionType:  controlplanev1beta2.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:            controlplanev1beta2.Cluster_TYPE_DEDICATED,
				RedpandaVersion: "v22.3.11",
				ThroughputTier:  "t1",
				Region:          "us-west-2",
				Zones:           []string{"us-west-2a", "us-west-2b"},
				ResourceGroupId: "rg-123",
				NetworkId:       "net-456",
				DataplaneApi:    &controlplanev1beta2.Cluster_DataplaneAPI{Url: "https://test-cluster.rptest.io:443"},
				KafkaApi: &controlplanev1beta2.Cluster_KafkaAPI{
					Mtls: &controlplanev1beta2.MTLSSpec{
						Enabled:               false,
						CaCertificatesPem:     make([]string, 0),
						PrincipalMappingRules: make([]string, 0),
					},
				},
				HttpProxy: &controlplanev1beta2.Cluster_HTTPProxyStatus{
					Mtls: &controlplanev1beta2.MTLSSpec{
						Enabled:               false,
						CaCertificatesPem:     make([]string, 0),
						PrincipalMappingRules: make([]string, 0),
					},
				},
				SchemaRegistry: &controlplanev1beta2.Cluster_SchemaRegistryStatus{
					Mtls: &controlplanev1beta2.MTLSSpec{
						Enabled:               false,
						CaCertificatesPem:     make([]string, 0),
						PrincipalMappingRules: make([]string, 0),
					},
				},
			},
			expected: &models.Cluster{
				Name:                  types.StringValue("test-cluster"),
				ConnectionType:        types.StringValue("public"),
				CloudProvider:         types.StringValue("aws"),
				ClusterType:           types.StringValue("dedicated"),
				RedpandaVersion:       types.StringValue("v22.3.11"),
				ThroughputTier:        types.StringValue("t1"),
				Region:                types.StringValue("us-west-2"),
				ResourceGroupID:       types.StringValue("rg-123"),
				NetworkID:             types.StringValue("net-456"),
				ID:                    types.StringValue("cl-789"),
				ClusterAPIURL:         types.StringValue("test-cluster.rptest.io:443"),
				ReadReplicaClusterIDs: basetypes.NewListNull(types.StringType),
				Zones:                 utils.TestingOnlyStringSliceToTypeList([]string{"us-west-2a", "us-west-2b"}),
				AllowDeletion:         types.BoolValue(false),
			},
			wantErr: false,
		},
		{
			name: "GCP cluster with private connection",
			cfg: models.Cluster{
				Name:            types.StringValue("gcp-private-cluster"),
				ConnectionType:  types.StringValue("private"),
				CloudProvider:   types.StringValue("gcp"),
				ClusterType:     types.StringValue("dedicated"),
				RedpandaVersion: types.StringValue("v22.3.12"),
				ThroughputTier:  types.StringValue("t2"),
				Region:          types.StringValue("us-central1"),
				ResourceGroupID: types.StringValue("rg-456"),
				NetworkID:       types.StringValue("net-789"),
				Zones:           utils.TestingOnlyStringSliceToTypeList([]string{"us-central1-a", "us-central1-b", "us-central1-c"}),
				AllowDeletion:   types.BoolValue(true),
			},
			cluster: &controlplanev1beta2.Cluster{
				Id:              "cl-101",
				Name:            "gcp-private-cluster",
				ConnectionType:  controlplanev1beta2.Cluster_CONNECTION_TYPE_PRIVATE,
				CloudProvider:   controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:            controlplanev1beta2.Cluster_TYPE_DEDICATED,
				RedpandaVersion: "v22.3.12",
				ThroughputTier:  "t2",
				Region:          "us-central1",
				Zones:           []string{"us-central1-a", "us-central1-b", "us-central1-c"},
				ResourceGroupId: "rg-456",
				NetworkId:       "net-789",
				DataplaneApi:    &controlplanev1beta2.Cluster_DataplaneAPI{Url: "https://gcp-private-cluster.rptest.io:443"},
			},
			expected: &models.Cluster{
				Name:                  types.StringValue("gcp-private-cluster"),
				ConnectionType:        types.StringValue("private"),
				CloudProvider:         types.StringValue("gcp"),
				ClusterType:           types.StringValue("dedicated"),
				RedpandaVersion:       types.StringValue("v22.3.12"),
				ThroughputTier:        types.StringValue("t2"),
				Region:                types.StringValue("us-central1"),
				ResourceGroupID:       types.StringValue("rg-456"),
				NetworkID:             types.StringValue("net-789"),
				ID:                    types.StringValue("cl-101"),
				ClusterAPIURL:         types.StringValue("gcp-private-cluster.rptest.io:443"),
				Zones:                 utils.TestingOnlyStringSliceToTypeList([]string{"us-central1-a", "us-central1-b", "us-central1-c"}),
				AllowDeletion:         types.BoolValue(true),
				ReadReplicaClusterIDs: basetypes.NewListNull(types.StringType),
			},
			wantErr: false,
		},
		{
			name: "AWS cluster with MTLS enabled",
			cfg: models.Cluster{
				Name:           types.StringValue("aws-mtls-cluster"),
				ConnectionType: types.StringValue("public"),
				CloudProvider:  types.StringValue("aws"),
				ClusterType:    types.StringValue("dedicated"),
				ThroughputTier: types.StringValue("t3"),
			},
			cluster: &controlplanev1beta2.Cluster{
				Id:                    "cl-202",
				Name:                  "aws-mtls-cluster",
				ConnectionType:        controlplanev1beta2.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:         controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:                  controlplanev1beta2.Cluster_TYPE_DEDICATED,
				ThroughputTier:        "t3",
				Region:                "eu-west-1",
				Zones:                 []string{"eu-west-1a"},
				ResourceGroupId:       "rg-789",
				NetworkId:             "net-101",
				ReadReplicaClusterIds: []string{""},
				DataplaneApi:          &controlplanev1beta2.Cluster_DataplaneAPI{Url: "https://aws-mtls-cluster.rptest.io:443"},
				KafkaApi: &controlplanev1beta2.Cluster_KafkaAPI{
					Mtls: &controlplanev1beta2.MTLSSpec{
						Enabled:               true,
						CaCertificatesPem:     []string{"cert1", "cert2"},
						PrincipalMappingRules: []string{"rule1", "rule2"},
					},
				},
			},
			expected: &models.Cluster{
				Name:                  types.StringValue("aws-mtls-cluster"),
				ConnectionType:        types.StringValue("public"),
				CloudProvider:         types.StringValue("aws"),
				ClusterType:           types.StringValue("dedicated"),
				ThroughputTier:        types.StringValue("t3"),
				Region:                types.StringValue("eu-west-1"),
				ID:                    types.StringValue("cl-202"),
				ResourceGroupID:       types.StringValue("rg-789"),
				NetworkID:             types.StringValue("net-101"),
				ClusterAPIURL:         types.StringValue("aws-mtls-cluster.rptest.io:443"),
				ReadReplicaClusterIDs: utils.TestingOnlyStringSliceToTypeList([]string{""}),
				Zones:                 utils.TestingOnlyStringSliceToTypeList([]string{"eu-west-1a"}),
				KafkaAPI: &models.KafkaAPI{
					Mtls: &models.Mtls{
						Enabled:               types.BoolValue(true),
						CaCertificatesPem:     utils.TestingOnlyStringSliceToTypeList([]string{"cert1", "cert2"}),
						PrincipalMappingRules: utils.TestingOnlyStringSliceToTypeList([]string{"rule1", "rule2"}),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCP cluster with AWS Private Link",
			cfg: models.Cluster{
				Name:           types.StringValue("gcp-aws-pl-cluster"),
				ConnectionType: types.StringValue("public"),
				CloudProvider:  types.StringValue("gcp"),
				ClusterType:    types.StringValue("dedicated"),
				AwsPrivateLink: &models.AwsPrivateLink{
					Enabled:           types.BoolValue(true),
					AllowedPrincipals: utils.TestingOnlyStringSliceToTypeList([]string{"arn:aws:iam::123456789012:root"}),
				},
			},
			cluster: &controlplanev1beta2.Cluster{
				Id:                    "cl-303",
				Name:                  "gcp-aws-pl-cluster",
				ConnectionType:        controlplanev1beta2.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:         controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:                  controlplanev1beta2.Cluster_TYPE_DEDICATED,
				ThroughputTier:        "t3",
				Region:                "eu-west-1",
				ReadReplicaClusterIds: []string{""},
				NetworkId:             "net-303",
				ResourceGroupId:       "123",
				Zones:                 []string{"eu-west-1a"},
				DataplaneApi:          &controlplanev1beta2.Cluster_DataplaneAPI{Url: "https://gcp-aws-pl-cluster.rptest.io:443"},
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkStatus{
					Enabled:           true,
					AllowedPrincipals: []string{"arn:aws:iam::123456789012:root"},
				},
			},
			expected: &models.Cluster{
				Name:                  types.StringValue("gcp-aws-pl-cluster"),
				ConnectionType:        types.StringValue("public"),
				CloudProvider:         types.StringValue("gcp"),
				ClusterType:           types.StringValue("dedicated"),
				ID:                    types.StringValue("cl-303"),
				ClusterAPIURL:         types.StringValue("gcp-aws-pl-cluster.rptest.io:443"),
				ThroughputTier:        types.StringValue("t3"),
				NetworkID:             types.StringValue("net-303"),
				ResourceGroupID:       types.StringValue("123"),
				Zones:                 utils.TestingOnlyStringSliceToTypeList([]string{"eu-west-1a"}),
				ReadReplicaClusterIDs: utils.TestingOnlyStringSliceToTypeList([]string{""}),
				Region:                types.StringValue("eu-west-1"),
				AwsPrivateLink: &models.AwsPrivateLink{
					Enabled:           types.BoolValue(true),
					AllowedPrincipals: utils.TestingOnlyStringSliceToTypeList([]string{"arn:aws:iam::123456789012:root"}),
					ConnectConsole:    types.BoolValue(false),
				},
			},
			wantErr: false,
		},
		{
			name: "AWS cluster with GCP Private Service Connect",
			cfg: models.Cluster{
				Name:           types.StringValue("aws-gcp-psc-cluster"),
				ConnectionType: types.StringValue("private"),
				CloudProvider:  types.StringValue("aws"),
				ClusterType:    types.StringValue("dedicated"),
				GcpPrivateServiceConnect: &models.GcpPrivateServiceConnect{
					Enabled:             types.BoolValue(true),
					GlobalAccessEnabled: types.BoolValue(false),
					ConsumerAcceptList: []*models.GcpPrivateServiceConnectConsumer{
						{Source: "projects/123456789012/regions/us-central1/serviceAttachments/sa-1"},
					},
				},
			},
			cluster: &controlplanev1beta2.Cluster{
				Id:                    "cl-404",
				Name:                  "aws-gcp-psc-cluster",
				ConnectionType:        controlplanev1beta2.Cluster_CONNECTION_TYPE_PRIVATE,
				CloudProvider:         controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:                  controlplanev1beta2.Cluster_TYPE_DEDICATED,
				Zones:                 []string{"us-central1-a"},
				Region:                "us-central1",
				ThroughputTier:        "t4",
				ResourceGroupId:       "rg-404",
				NetworkId:             "net-404",
				ReadReplicaClusterIds: []string{""},
				DataplaneApi:          &controlplanev1beta2.Cluster_DataplaneAPI{Url: "https://aws-gcp-psc-cluster.rptest.io:443"},
				GcpPrivateServiceConnect: &controlplanev1beta2.GCPPrivateServiceConnectStatus{
					Enabled:             true,
					GlobalAccessEnabled: false,
					ConsumerAcceptList: []*controlplanev1beta2.GCPPrivateServiceConnectConsumer{
						{Source: "projects/123456789012/regions/us-central1/serviceAttachments/sa-1"},
					},
				},
			},
			expected: &models.Cluster{
				Name:                  types.StringValue("aws-gcp-psc-cluster"),
				ConnectionType:        types.StringValue("private"),
				CloudProvider:         types.StringValue("aws"),
				ClusterType:           types.StringValue("dedicated"),
				ID:                    types.StringValue("cl-404"),
				ThroughputTier:        types.StringValue("t4"),
				Region:                types.StringValue("us-central1"),
				ResourceGroupID:       types.StringValue("rg-404"),
				NetworkID:             types.StringValue("net-404"),
				ReadReplicaClusterIDs: utils.TestingOnlyStringSliceToTypeList([]string{""}),
				Zones:                 utils.TestingOnlyStringSliceToTypeList([]string{"us-central1-a"}),
				ClusterAPIURL:         types.StringValue("aws-gcp-psc-cluster.rptest.io:443"),
				GcpPrivateServiceConnect: &models.GcpPrivateServiceConnect{
					Enabled:             types.BoolValue(true),
					GlobalAccessEnabled: types.BoolValue(false),
					ConsumerAcceptList: []*models.GcpPrivateServiceConnectConsumer{
						{Source: "projects/123456789012/regions/us-central1/serviceAttachments/sa-1"},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := generateModel(ctx, tc.cfg, tc.cluster)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestIsMtlsNil(t *testing.T) {
	tests := []struct {
		name      string
		container any
		want      bool
	}{
		{
			name:      "KafkaAPI with nil Mtls",
			container: &models.KafkaAPI{Mtls: nil},
			want:      true,
		},
		{
			name: "KafkaAPI with non-nil Mtls",
			container: &models.KafkaAPI{Mtls: &models.Mtls{
				Enabled: types.BoolValue(true),
			}},
			want: false,
		},
		{
			name:      "HTTPProxy with nil Mtls",
			container: &models.HTTPProxy{Mtls: nil},
			want:      true,
		},
		{
			name: "HTTPProxy with non-nil Mtls",
			container: &models.HTTPProxy{Mtls: &models.Mtls{
				Enabled: types.BoolValue(true),
			}},
			want: false,
		},
		{
			name:      "SchemaRegistry with nil Mtls",
			container: &models.SchemaRegistry{Mtls: nil},
			want:      true,
		},
		{
			name: "SchemaRegistry with non-nil Mtls",
			container: &models.SchemaRegistry{Mtls: &models.Mtls{
				Enabled: types.BoolValue(true),
			}},
			want: false,
		},
		{
			name:      "Nil container",
			container: nil,
			want:      true,
		},
		{
			name:      "Non-struct container",
			container: "not a struct",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMtlsNil(tt.container); got != tt.want {
				t.Errorf("isMtlsNil() = %v, want %v", got, tt.want)
			}
		})
	}
}
