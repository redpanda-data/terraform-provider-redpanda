package topic

import (
	"context"
	"errors"
	"testing"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	topicmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/topic"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTopic_Create(t *testing.T) {
	partitionCount := int32(3)
	replicationFactor := int32(1)

	tests := []struct {
		name    string
		input   topicmodel.ResourceModel
		setup   func(*mocks.MockTopicServiceClient)
		wantErr bool
	}{
		{
			name: "basic topic creation",
			input: topicmodel.ResourceModel{
				Name:              types.StringValue("test-topic"),
				PartitionCount:    utils.Int32ToNumber(partitionCount),
				ReplicationFactor: utils.Int32ToNumber(replicationFactor),
				Configuration:     types.MapNull(types.StringType),
				ClusterAPIURL:     types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion:     types.BoolValue(true),
				ReplicaAssignments: types.ListNull(types.ObjectType{
					AttrTypes: replicaAssignmentAttrTypes(),
				}),
			},
			setup: func(m *mocks.MockTopicServiceClient) {
				m.EXPECT().
					CreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&dataplanev1.CreateTopicResponse{
						TopicName:         "test-topic",
						PartitionCount:    partitionCount,
						ReplicationFactor: replicationFactor,
					}, nil)
				m.EXPECT().
					GetTopicConfigurations(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&dataplanev1.GetTopicConfigurationsResponse{
						Configurations: []*dataplanev1.Topic_Configuration{},
					}, nil)
			},
		},
		{
			name: "topic with configuration",
			input: topicmodel.ResourceModel{
				Name:              types.StringValue("configured-topic"),
				PartitionCount:    utils.Int32ToNumber(partitionCount),
				ReplicationFactor: utils.Int32ToNumber(replicationFactor),
				Configuration: types.MapValueMust(types.StringType, map[string]attr.Value{
					"cleanup.policy": types.StringValue("delete"),
					"retention.ms":   types.StringValue("604800000"),
				}),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ReplicaAssignments: types.ListNull(types.ObjectType{
					AttrTypes: replicaAssignmentAttrTypes(),
				}),
			},
			setup: func(m *mocks.MockTopicServiceClient) {
				m.EXPECT().
					CreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&dataplanev1.CreateTopicResponse{
						TopicName:         "configured-topic",
						PartitionCount:    partitionCount,
						ReplicationFactor: replicationFactor,
					}, nil)
				m.EXPECT().
					GetTopicConfigurations(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&dataplanev1.GetTopicConfigurationsResponse{
						Configurations: []*dataplanev1.Topic_Configuration{
							{
								Name:   "cleanup.policy",
								Value:  strPtr("delete"),
								Source: dataplanev1.ConfigSource_CONFIG_SOURCE_DYNAMIC_TOPIC_CONFIG,
							},
							{
								Name:   "retention.ms",
								Value:  strPtr("604800000"),
								Source: dataplanev1.ConfigSource_CONFIG_SOURCE_DYNAMIC_TOPIC_CONFIG,
							},
						},
					}, nil)
			},
		},
		{
			name: "create fails - API error",
			input: topicmodel.ResourceModel{
				Name:              types.StringValue("failing-topic"),
				PartitionCount:    utils.Int32ToNumber(partitionCount),
				ReplicationFactor: utils.Int32ToNumber(replicationFactor),
				Configuration:     types.MapNull(types.StringType),
				ClusterAPIURL:     types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion:     types.BoolValue(true),
				ReplicaAssignments: types.ListNull(types.ObjectType{
					AttrTypes: replicaAssignmentAttrTypes(),
				}),
			},
			setup: func(m *mocks.MockTopicServiceClient) {
				m.EXPECT().
					CreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("API error"))
			},
			wantErr: true,
		},
		{
			name: "state persisted when GetTopicConfigurations fails after create",
			input: topicmodel.ResourceModel{
				Name:              types.StringValue("orphan-topic"),
				PartitionCount:    utils.Int32ToNumber(partitionCount),
				ReplicationFactor: utils.Int32ToNumber(replicationFactor),
				Configuration: types.MapValueMust(types.StringType, map[string]attr.Value{
					"cleanup.policy": types.StringValue("compact"),
				}),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ReplicaAssignments: types.ListNull(types.ObjectType{
					AttrTypes: replicaAssignmentAttrTypes(),
				}),
			},
			setup: func(m *mocks.MockTopicServiceClient) {
				m.EXPECT().
					CreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&dataplanev1.CreateTopicResponse{
						TopicName:         "orphan-topic",
						PartitionCount:    partitionCount,
						ReplicationFactor: replicationFactor,
					}, nil)
				m.EXPECT().
					GetTopicConfigurations(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("timeout"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockTopicServiceClient(ctrl)
			tt.setup(mockClient)

			topic := &Topic{
				clientFactory: func(_, _, _, _ string) (dataplanev1grpc.TopicServiceClient, error) {
					return mockClient, nil
				},
				resData: config.Resource{
					AuthToken:        "test-token",
					ProviderVersion:  "1.0.0",
					TerraformVersion: "1.5.0",
				},
			}

			schemaResp := resource.SchemaResponse{}
			topic.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			req := resource.CreateRequest{
				Plan: tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := req.Plan.Set(ctx, &tt.input)
			require.False(t, diags.HasError(), "Plan.Set should not error: %v", diags)

			resp := resource.CreateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			topic.Create(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")

				// Even on error, if CreateTopic succeeded, state should be persisted.
				if tt.name == "state persisted when GetTopicConfigurations fails after create" {
					var state topicmodel.ResourceModel
					diags = resp.State.Get(ctx, &state)
					require.False(t, diags.HasError(), "State.Get should not error")
					assert.Equal(t, "orphan-topic", state.ID.ValueString(), "ID should be set even after GetTopicConfigurations failure")
					assert.Equal(t, "orphan-topic", state.Name.ValueString(), "Name should be set")
					assert.False(t, state.Configuration.IsNull(), "Configuration should be set from plan values")
				}
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Create should not error: %v", resp.Diagnostics)

			var state topicmodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")

			assert.Equal(t, tt.input.Name, state.Name)
			assert.Equal(t, tt.input.ClusterAPIURL, state.ClusterAPIURL)
			assert.Equal(t, tt.input.AllowDeletion, state.AllowDeletion)
			assert.False(t, state.ID.IsNull())
			assert.Equal(t, tt.input.Name.ValueString(), state.ID.ValueString())
		})
	}
}

func TestTopic_CreateStatePersistence(t *testing.T) {
	partitionCount := int32(3)
	replicationFactor := int32(1)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockTopicServiceClient(ctrl)

	// CreateTopic succeeds
	mockClient.EXPECT().
		CreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&dataplanev1.CreateTopicResponse{
			TopicName:         "bulk-topic",
			PartitionCount:    partitionCount,
			ReplicationFactor: replicationFactor,
		}, nil)

	// GetTopicConfigurations fails (simulates timeout under bulk creation)
	mockClient.EXPECT().
		GetTopicConfigurations(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("context deadline exceeded"))

	topic := &Topic{
		clientFactory: func(_, _, _, _ string) (dataplanev1grpc.TopicServiceClient, error) {
			return mockClient, nil
		},
		resData: config.Resource{
			AuthToken:        "test-token",
			ProviderVersion:  "1.0.0",
			TerraformVersion: "1.5.0",
		},
	}

	schemaResp := resource.SchemaResponse{}
	topic.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	input := topicmodel.ResourceModel{
		Name:              types.StringValue("bulk-topic"),
		PartitionCount:    utils.Int32ToNumber(partitionCount),
		ReplicationFactor: utils.Int32ToNumber(replicationFactor),
		Configuration: types.MapValueMust(types.StringType, map[string]attr.Value{
			"retention.ms": types.StringValue("86400000"),
		}),
		ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
		AllowDeletion: types.BoolValue(true),
		ReplicaAssignments: types.ListNull(types.ObjectType{
			AttrTypes: replicaAssignmentAttrTypes(),
		}),
	}

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema},
	}
	diags := req.Plan.Set(ctx, &input)
	require.False(t, diags.HasError())

	resp := resource.CreateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	topic.Create(ctx, req, &resp)

	// Should have an error from GetTopicConfigurations failure
	require.True(t, resp.Diagnostics.HasError(), "expected error diagnostic")

	// But state should still be persisted with plan values
	var state topicmodel.ResourceModel
	diags = resp.State.Get(ctx, &state)
	require.False(t, diags.HasError(), "State.Get should succeed")

	assert.Equal(t, "bulk-topic", state.ID.ValueString(), "ID must be persisted")
	assert.Equal(t, "bulk-topic", state.Name.ValueString(), "Name must be persisted")
	assert.Equal(t, input.ClusterAPIURL, state.ClusterAPIURL, "ClusterAPIURL must be persisted")
	assert.Equal(t, input.AllowDeletion, state.AllowDeletion, "AllowDeletion must be persisted")

	// Configuration should contain the planned values
	configMap := make(map[string]string)
	diags = state.Configuration.ElementsAs(ctx, &configMap, false)
	require.False(t, diags.HasError())
	assert.Equal(t, "86400000", configMap["retention.ms"], "planned config should be in state")
}

func TestTopic_UpdateStatePersistence(t *testing.T) {
	partitionCount := int32(3)
	replicationFactor := int32(1)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockTopicServiceClient(ctrl)

	// SetTopicConfigurations succeeds
	mockClient.EXPECT().
		SetTopicConfigurations(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&dataplanev1.SetTopicConfigurationsResponse{}, nil)

	// Re-read fails
	mockClient.EXPECT().
		GetTopicConfigurations(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("context deadline exceeded"))

	topic := &Topic{
		clientFactory: func(_, _, _, _ string) (dataplanev1grpc.TopicServiceClient, error) {
			return mockClient, nil
		},
		resData: config.Resource{
			AuthToken:        "test-token",
			ProviderVersion:  "1.0.0",
			TerraformVersion: "1.5.0",
		},
	}

	schemaResp := resource.SchemaResponse{}
	topic.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := topicmodel.ResourceModel{
		Name:              types.StringValue("update-topic"),
		PartitionCount:    utils.Int32ToNumber(partitionCount),
		ReplicationFactor: utils.Int32ToNumber(replicationFactor),
		Configuration: types.MapValueMust(types.StringType, map[string]attr.Value{
			"retention.ms": types.StringValue("86400000"),
		}),
		ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
		AllowDeletion: types.BoolValue(true),
		ID:            types.StringValue("update-topic"),
		ReplicaAssignments: types.ListNull(types.ObjectType{
			AttrTypes: replicaAssignmentAttrTypes(),
		}),
	}

	plan := topicmodel.ResourceModel{
		Name:              types.StringValue("update-topic"),
		PartitionCount:    utils.Int32ToNumber(partitionCount),
		ReplicationFactor: utils.Int32ToNumber(replicationFactor),
		Configuration: types.MapValueMust(types.StringType, map[string]attr.Value{
			"retention.ms": types.StringValue("172800000"),
		}),
		ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
		AllowDeletion: types.BoolValue(true),
		ID:            types.StringValue("update-topic"),
		ReplicaAssignments: types.ListNull(types.ObjectType{
			AttrTypes: replicaAssignmentAttrTypes(),
		}),
	}

	req := resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
	}
	diags := req.State.Set(ctx, &state)
	require.False(t, diags.HasError())
	diags = req.Plan.Set(ctx, &plan)
	require.False(t, diags.HasError())

	resp := resource.UpdateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	topic.Update(ctx, req, &resp)

	// Should have an error from GetTopicConfigurations failure
	require.True(t, resp.Diagnostics.HasError(), "expected error diagnostic")

	// But state should reflect the plan (mutation was applied)
	var result topicmodel.ResourceModel
	diags = resp.State.Get(ctx, &result)
	require.False(t, diags.HasError(), "State.Get should succeed")

	assert.Equal(t, "update-topic", result.ID.ValueString(), "ID must be persisted")

	configMap := make(map[string]string)
	diags = result.Configuration.ElementsAs(ctx, &configMap, false)
	require.False(t, diags.HasError())
	assert.Equal(t, "172800000", configMap["retention.ms"], "updated config should be in state")
}

func strPtr(s string) *string {
	return &s
}

func replicaAssignmentAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"partition_id": types.Int32Type,
		"replica_ids":  types.ListType{ElemType: types.Int32Type},
	}
}
