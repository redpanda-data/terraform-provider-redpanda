package cluster

import (
	"testing"
	"time"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGenerateModelAWSPrivateLink(t *testing.T) {
	testTime := time.Date(2024, 1, 30, 12, 0, 0, 0, time.UTC)
	testTimestamp := timestamppb.New(testTime)

	tests := []struct {
		name        string
		input       *controlplanev1beta2.Cluster
		expectError bool
		verify      func(*testing.T, types.Object, diag.Diagnostics)
	}{
		{
			name:  "nil cluster returns null object",
			input: nil,
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.True(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())
			},
		},
		{
			name:  "cluster without aws private link returns null object",
			input: &controlplanev1beta2.Cluster{},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.True(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())
			},
		},
		{
			name: "aws private link without status",
			input: &controlplanev1beta2.Cluster{
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkStatus{
					Enabled:           true,
					ConnectConsole:    true,
					AllowedPrincipals: []string{"principal1", "principal2"},
				},
			},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				enabled := attrs["enabled"].(types.Bool)
				assert.True(t, enabled.ValueBool())

				connectConsole := attrs["connect_console"].(types.Bool)
				assert.True(t, connectConsole.ValueBool())

				principals := attrs["allowed_principals"].(types.List)
				assert.Equal(t, 2, len(principals.Elements()))

				status := attrs["status"].(types.Object)
				assert.True(t, status.IsNull())
			},
		},
		{
			name: "aws private link with empty status fields should not error",
			input: &controlplanev1beta2.Cluster{
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkStatus{
					Enabled:        true,
					ConnectConsole: true,
					Status: &controlplanev1beta2.AWSPrivateLinkStatus_Status{
						VpcEndpointConnections: []*controlplanev1beta2.AWSPrivateLinkStatus_Status_VPCEndpointConnection{
							{
								DnsEntries: []*controlplanev1beta2.AWSPrivateLinkStatus_Status_VPCEndpointConnection_DNSEntry{
									{}, // Empty DNS entry
								},
							},
						},
					},
				},
			},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				status := attrs["status"].(types.Object)
				assert.False(t, status.IsNull())

				statusAttrs := status.Attributes()
				conns := statusAttrs["vpc_endpoint_connections"].(types.List)
				assert.Equal(t, 1, len(conns.Elements()))

				firstConn := conns.Elements()[0].(types.Object)
				connAttrs := firstConn.Attributes()

				dnsEntries := connAttrs["dns_entries"].(types.List)
				assert.Equal(t, 1, len(dnsEntries.Elements()))

				firstDNS := dnsEntries.Elements()[0].(types.Object)
				dnsAttrs := firstDNS.Attributes()
				assert.Equal(t, "", dnsAttrs["dns_name"].(types.String).ValueString())
				assert.Equal(t, "", dnsAttrs["hosted_zone_id"].(types.String).ValueString())
			},
		},
		{
			name: "aws private link with partial status fields",
			input: &controlplanev1beta2.Cluster{
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkStatus{
					Enabled:        true,
					ConnectConsole: true,
					Status: &controlplanev1beta2.AWSPrivateLinkStatus_Status{
						ServiceId: "svc-1",
						// Missing timestamps
						VpcEndpointConnections: []*controlplanev1beta2.AWSPrivateLinkStatus_Status_VPCEndpointConnection{
							{
								Id: "vpc-1",
								DnsEntries: []*controlplanev1beta2.AWSPrivateLinkStatus_Status_VPCEndpointConnection_DNSEntry{
									{
										DnsName: "test.dns.com",
										// Missing hosted zone
									},
								},
								// Missing other fields
							},
						},
						KafkaApiSeedPort: 9092,
						// Missing other fields
					},
				},
			},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				status := attrs["status"].(types.Object)
				statusAttrs := status.Attributes()

				assert.Equal(t, "svc-1", statusAttrs["service_id"].(types.String).ValueString())
				assert.True(t, statusAttrs["created_at"].(types.String).IsNull())
				assert.True(t, statusAttrs["deleted_at"].(types.String).IsNull())

				conns := statusAttrs["vpc_endpoint_connections"].(types.List)
				firstConn := conns.Elements()[0].(types.Object)
				connAttrs := firstConn.Attributes()
				assert.Equal(t, "vpc-1", connAttrs["id"].(types.String).ValueString())

				dnsEntries := connAttrs["dns_entries"].(types.List)
				firstDNS := dnsEntries.Elements()[0].(types.Object)
				dnsAttrs := firstDNS.Attributes()
				assert.Equal(t, "test.dns.com", dnsAttrs["dns_name"].(types.String).ValueString())
				assert.Equal(t, "", dnsAttrs["hosted_zone_id"].(types.String).ValueString())

				assert.Equal(t, int64(9092), statusAttrs["kafka_api_seed_port"].(types.Int64).ValueInt64())
			},
		},
		{
			name: "aws private link with complete status",
			input: &controlplanev1beta2.Cluster{
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkStatus{
					Enabled:           true,
					ConnectConsole:    true,
					AllowedPrincipals: []string{"principal1"},
					Status: &controlplanev1beta2.AWSPrivateLinkStatus_Status{
						ServiceId:    "svc-123",
						ServiceName:  "test-service",
						ServiceState: "ACTIVE",
						CreatedAt:    testTimestamp,
						VpcEndpointConnections: []*controlplanev1beta2.AWSPrivateLinkStatus_Status_VPCEndpointConnection{
							{
								Id:               "vpc-123",
								Owner:            "test-owner",
								State:            "available",
								CreatedAt:        testTimestamp,
								ConnectionId:     "conn-123",
								LoadBalancerArns: []string{"arn1", "arn2"},
								DnsEntries: []*controlplanev1beta2.AWSPrivateLinkStatus_Status_VPCEndpointConnection_DNSEntry{
									{
										DnsName:      "test.dns.com",
										HostedZoneId: "Z123456",
									},
								},
							},
						},
						KafkaApiSeedPort:          9092,
						SchemaRegistrySeedPort:    8081,
						RedpandaProxySeedPort:     8082,
						KafkaApiNodeBasePort:      9093,
						RedpandaProxyNodeBasePort: 8083,
						ConsolePort:               8080,
					},
				},
			},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				status := attrs["status"].(types.Object)
				statusAttrs := status.Attributes()

				assert.Equal(t, "svc-123", statusAttrs["service_id"].(types.String).ValueString())
				assert.Equal(t, "test-service", statusAttrs["service_name"].(types.String).ValueString())
				assert.Equal(t, "ACTIVE", statusAttrs["service_state"].(types.String).ValueString())
				assert.Equal(t, testTime.Format(time.RFC3339), statusAttrs["created_at"].(types.String).ValueString())

				conns := statusAttrs["vpc_endpoint_connections"].(types.List)
				firstConn := conns.Elements()[0].(types.Object)
				connAttrs := firstConn.Attributes()
				assert.Equal(t, "vpc-123", connAttrs["id"].(types.String).ValueString())
				assert.Equal(t, "test-owner", connAttrs["owner"].(types.String).ValueString())
				assert.Equal(t, "available", connAttrs["state"].(types.String).ValueString())

				dnsEntries := connAttrs["dns_entries"].(types.List)
				firstDNS := dnsEntries.Elements()[0].(types.Object)
				dnsAttrs := firstDNS.Attributes()
				assert.Equal(t, "test.dns.com", dnsAttrs["dns_name"].(types.String).ValueString())
				assert.Equal(t, "Z123456", dnsAttrs["hosted_zone_id"].(types.String).ValueString())

				// Verify ports
				assert.Equal(t, int64(9092), statusAttrs["kafka_api_seed_port"].(types.Int64).ValueInt64())
				assert.Equal(t, int64(8081), statusAttrs["schema_registry_seed_port"].(types.Int64).ValueInt64())
				assert.Equal(t, int64(8080), statusAttrs["console_port"].(types.Int64).ValueInt64())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, diagnostics := generateModelAWSPrivateLink(tt.input, diag.Diagnostics{})
			if tt.expectError {
				assert.True(t, diagnostics.HasError())
			} else {
				assert.False(t, diagnostics.HasError())
			}
			tt.verify(t, obj, diagnostics)
		})
	}
}
func TestGenerateModelGCPPrivateServiceConnect(t *testing.T) {
	testTime := time.Date(2024, 1, 30, 12, 0, 0, 0, time.UTC)
	testTimestamp := timestamppb.New(testTime)

	tests := []struct {
		name        string
		input       *controlplanev1beta2.Cluster
		expectError bool
		verify      func(*testing.T, types.Object, diag.Diagnostics)
	}{
		{
			name:  "nil cluster returns null object",
			input: nil,
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.True(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())
			},
		},
		{
			name:  "cluster without gcp psc returns null object",
			input: &controlplanev1beta2.Cluster{},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.True(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())
			},
		},
		{
			name: "gcp psc without status",
			input: &controlplanev1beta2.Cluster{
				GcpPrivateServiceConnect: &controlplanev1beta2.GCPPrivateServiceConnectStatus{
					Enabled:             true,
					GlobalAccessEnabled: true,
					ConsumerAcceptList: []*controlplanev1beta2.GCPPrivateServiceConnectConsumer{
						{Source: "project-1"},
						{Source: "project-2"},
					},
				},
			},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				enabled := attrs["enabled"].(types.Bool)
				assert.True(t, enabled.ValueBool())

				globalAccess := attrs["global_access_enabled"].(types.Bool)
				assert.True(t, globalAccess.ValueBool())

				consumers := attrs["consumer_accept_list"].(types.List)
				assert.Equal(t, 2, len(consumers.Elements()))

				status := attrs["status"].(types.Object)
				assert.True(t, status.IsNull())
			},
		},
		{
			name: "gcp psc with empty status fields should not error",
			input: &controlplanev1beta2.Cluster{
				GcpPrivateServiceConnect: &controlplanev1beta2.GCPPrivateServiceConnectStatus{
					Enabled:             true,
					GlobalAccessEnabled: true,
					Status: &controlplanev1beta2.GCPPrivateServiceConnectStatus_Status{
						ConnectedEndpoints: []*controlplanev1beta2.GCPPrivateServiceConnectStatus_Status_ConnectedEndpoint{
							{
								// All fields empty/zero
							},
						},
					},
				},
			},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				status := attrs["status"].(types.Object)
				assert.False(t, status.IsNull())

				statusAttrs := status.Attributes()
				endpoints := statusAttrs["connected_endpoints"].(types.List)
				assert.Equal(t, 1, len(endpoints.Elements()))

				firstEndpoint := endpoints.Elements()[0].(types.Object)
				endpointAttrs := firstEndpoint.Attributes()
				assert.Equal(t, "", endpointAttrs["connection_id"].(types.String).ValueString())
				assert.Equal(t, "", endpointAttrs["status"].(types.String).ValueString())
			},
		},
		{
			name: "gcp psc with partial status fields",
			input: &controlplanev1beta2.Cluster{
				GcpPrivateServiceConnect: &controlplanev1beta2.GCPPrivateServiceConnectStatus{
					Enabled:             true,
					GlobalAccessEnabled: true,
					Status: &controlplanev1beta2.GCPPrivateServiceConnectStatus_Status{
						ServiceAttachment: "service-1",
						// Missing timestamps
						ConnectedEndpoints: []*controlplanev1beta2.GCPPrivateServiceConnectStatus_Status_ConnectedEndpoint{
							{
								ConnectionId: "conn-1",
								// Missing other fields
							},
						},
						// Missing other fields
					},
				},
			},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				status := attrs["status"].(types.Object)
				statusAttrs := status.Attributes()

				assert.Equal(t, "service-1", statusAttrs["service_attachment"].(types.String).ValueString())
				assert.True(t, statusAttrs["created_at"].(types.String).IsNull())
				assert.True(t, statusAttrs["deleted_at"].(types.String).IsNull())

				endpoints := statusAttrs["connected_endpoints"].(types.List)
				firstEndpoint := endpoints.Elements()[0].(types.Object)
				endpointAttrs := firstEndpoint.Attributes()
				assert.Equal(t, "conn-1", endpointAttrs["connection_id"].(types.String).ValueString())
				assert.Equal(t, "", endpointAttrs["consumer_network"].(types.String).ValueString())
			},
		},
		{
			name: "gcp psc with complete status",
			input: &controlplanev1beta2.Cluster{
				GcpPrivateServiceConnect: &controlplanev1beta2.GCPPrivateServiceConnectStatus{
					Enabled:             true,
					GlobalAccessEnabled: true,
					ConsumerAcceptList: []*controlplanev1beta2.GCPPrivateServiceConnectConsumer{
						{Source: "project-1"},
					},
					Status: &controlplanev1beta2.GCPPrivateServiceConnectStatus_Status{
						ServiceAttachment: "service-1",
						CreatedAt:         testTimestamp,
						ConnectedEndpoints: []*controlplanev1beta2.GCPPrivateServiceConnectStatus_Status_ConnectedEndpoint{
							{
								ConnectionId:    "conn-1",
								ConsumerNetwork: "network-1",
								Endpoint:        "endpoint-1",
								Status:          "ACCEPTED",
							},
						},
						KafkaApiSeedPort:          9092,
						SchemaRegistrySeedPort:    8081,
						RedpandaProxySeedPort:     8082,
						KafkaApiNodeBasePort:      9093,
						RedpandaProxyNodeBasePort: 8083,
						DnsARecords:               []string{"record1", "record2"},
						SeedHostname:              "test.host.com",
					},
				},
			},
			verify: func(t *testing.T, obj types.Object, diagnostics diag.Diagnostics) {
				assert.False(t, obj.IsNull())
				assert.False(t, diagnostics.HasError())

				attrs := obj.Attributes()
				status := attrs["status"].(types.Object)
				statusAttrs := status.Attributes()

				assert.Equal(t, "service-1", statusAttrs["service_attachment"].(types.String).ValueString())
				assert.Equal(t, testTime.Format(time.RFC3339), statusAttrs["created_at"].(types.String).ValueString())

				endpoints := statusAttrs["connected_endpoints"].(types.List)
				firstEndpoint := endpoints.Elements()[0].(types.Object)
				endpointAttrs := firstEndpoint.Attributes()
				assert.Equal(t, "conn-1", endpointAttrs["connection_id"].(types.String).ValueString())
				assert.Equal(t, "network-1", endpointAttrs["consumer_network"].(types.String).ValueString())
				assert.Equal(t, "ACCEPTED", endpointAttrs["status"].(types.String).ValueString())

				assert.Equal(t, int64(9092), statusAttrs["kafka_api_seed_port"].(types.Int64).ValueInt64())
				assert.Equal(t, int64(8081), statusAttrs["schema_registry_seed_port"].(types.Int64).ValueInt64())

				dnsRecords := statusAttrs["dns_a_records"].(types.List)
				assert.Equal(t, 2, len(dnsRecords.Elements()))

				assert.Equal(t, "test.host.com", statusAttrs["seed_hostname"].(types.String).ValueString())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, diagnostics := generateModelGCPPrivateServiceConnect(tt.input, diag.Diagnostics{})
			if tt.expectError {
				assert.True(t, diagnostics.HasError())
			} else {
				assert.False(t, diagnostics.HasError())
			}
			tt.verify(t, obj, diagnostics)
		})
	}
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
			name:          "nil cluster returns null object",
			cloudProvider: "aws",
			cluster:       nil,
			expectNull:    true,
		},
		{
			name:          "cluster without CMR returns null object",
			cloudProvider: "aws",
			cluster:       &controlplanev1beta2.Cluster{},
			expectNull:    true,
		},
		{
			name:          "non-BYOC cluster with CMR returns error",
			cloudProvider: "aws",
			cluster: &controlplanev1beta2.Cluster{
				Type: controlplanev1beta2.Cluster_TYPE_DEDICATED,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
					CloudProvider: &controlplanev1beta2.CustomerManagedResources_Aws{},
				},
			},
			expectNull:     true,
			expectedErrors: []string{"Customer Managed Resources with non-BYOC cluster type"},
		},
		{
			name:          "cloud provider mismatch returns error",
			cloudProvider: "aws",
			cluster: &controlplanev1beta2.Cluster{
				Type: controlplanev1beta2.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
					CloudProvider: &controlplanev1beta2.CustomerManagedResources_Gcp{},
				},
			},
			expectNull:     true,
			expectedErrors: []string{"Cloud Provider Mismatch"},
		},
		{
			name:          "valid AWS BYOC cluster with complete CMR",
			cloudProvider: "aws",
			cluster: &controlplanev1beta2.Cluster{
				Type: controlplanev1beta2.Cluster_TYPE_BYOC,
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
			name:          "valid AWS BYOC cluster with partial CMR",
			cloudProvider: "aws",
			cluster: &controlplanev1beta2.Cluster{
				Type: controlplanev1beta2.Cluster_TYPE_BYOC,
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
			name:          "GCP CMR returns null object (not implemented)",
			cloudProvider: "gcp",
			cluster: &controlplanev1beta2.Cluster{
				Type: controlplanev1beta2.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{
					CloudProvider: &controlplanev1beta2.CustomerManagedResources_Gcp{},
				},
			},
			expectNull: true,
		},
		{
			name:          "unknown cloud provider returns null object",
			cloudProvider: "unknown",
			cluster: &controlplanev1beta2.Cluster{
				Type:                     controlplanev1beta2.Cluster_TYPE_BYOC,
				CustomerManagedResources: &controlplanev1beta2.CustomerManagedResources{},
			},
			expectNull: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, diagnostics := generateModelCMR(tt.cloudProvider, tt.cluster, diag.Diagnostics{})

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
				awsObj := attrs["aws"].(types.Object)
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
func verifyARN(t *testing.T, attrs map[string]attr.Value, key string, expectedARN string) {
	obj := attrs[key].(types.Object)
	if expectedARN == "" {
		assert.True(t, obj.IsNull())
		return
	}
	assert.False(t, obj.IsNull())
	assert.Equal(t, expectedARN, obj.Attributes()["arn"].(types.String).ValueString())
}
