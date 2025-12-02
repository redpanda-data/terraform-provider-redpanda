package utils

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestAreWeDoneYet(t *testing.T) {
	testCases := []struct {
		name      string
		op        *controlplanev1.Operation
		timeout   time.Duration
		mockSetup func(m *mocks.MockOperationServiceClient)
		wantErr   string
	}{
		{
			name: "Operation completed successfully",
			op:   &controlplanev1.Operation{State: controlplanev1.Operation_STATE_COMPLETED},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_COMPLETED), nil)
			},
			timeout: 5 * time.Minute,
		},
		{
			name: "Operation goes unspecified but then completes",
			op: &controlplanev1.Operation{
				State: controlplanev1.Operation_STATE_IN_PROGRESS,
			},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				gomock.InOrder(
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_IN_PROGRESS), nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_UNSPECIFIED), nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_UNSPECIFIED), nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_COMPLETED), nil),
				)
			},
			timeout: 5 * time.Minute,
		},
		{
			name: "Operation failed with an error",
			op: &controlplanev1.Operation{
				State: controlplanev1.Operation_STATE_IN_PROGRESS,
			},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				gomock.InOrder(
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_IN_PROGRESS), nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&controlplanev1.GetOperationResponse{Operation: &controlplanev1.Operation{
						State: controlplanev1.Operation_STATE_FAILED,
						Result: &controlplanev1.Operation_Error{
							Error: &status.Status{
								Code:    1,
								Message: "operation failed",
							},
						},
					}}, nil))
			},
			timeout: 5 * time.Minute,
			wantErr: "operation failed: operation failed",
		},
		{
			name:    "Operation times out",
			op:      &controlplanev1.Operation{State: controlplanev1.Operation_STATE_IN_PROGRESS},
			timeout: 100 * time.Millisecond,
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_IN_PROGRESS), nil).AnyTimes()
			},
			wantErr: "timed out after 100ms: expected operation to be completed but was in state STATE_IN_PROGRESS",
		},
		{
			name:    "Operation times out with unspecified",
			op:      &controlplanev1.Operation{State: controlplanev1.Operation_STATE_UNSPECIFIED},
			timeout: 100 * time.Millisecond,
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_UNSPECIFIED), nil).AnyTimes()
			},
			wantErr: "timed out after 100ms: expected operation to be completed but was in state STATE_UNSPECIFIED",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockOperationServiceClient(ctrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockClient)
			}

			ctx := context.Background()
			err := AreWeDoneYet(ctx, tc.op, tc.timeout, mockClient)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			} else {
				if err == nil || err.Error() != tc.wantErr {
					t.Errorf("Expected error '%s', got: %v", tc.wantErr, err)
				}
			}
		})
	}
}

func createOpResponse(state controlplanev1.Operation_State) *controlplanev1.GetOperationResponse {
	return &controlplanev1.GetOperationResponse{
		Operation: &controlplanev1.Operation{
			State: state,
		},
	}
}

func mustMap(t *testing.T, m map[string]string) basetypes.MapValue {
	o, err := types.MapValueFrom(context.TODO(), types.StringType, m)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestTypeMapToStringMap(t *testing.T) {
	type args struct {
		tags types.Map
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Empty map",
			args: args{tags: mustMap(t, map[string]string{})},
			want: nil,
		},
		{
			name: "Single key",
			args: args{tags: mustMap(t, map[string]string{"key": "value"})},
			want: map[string]string{"key": "value"},
		},
		{
			name: "Single key with quotes",
			args: args{tags: mustMap(t, map[string]string{"key": `"value"`})},
			want: map[string]string{"key": "value"},
		},
		{
			name: "Multiple keys",
			args: args{tags: mustMap(t, map[string]string{"key1": "value1", "key2": "value2"})},
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TypeMapToStringMap(tt.args.tags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TypeMapToStringMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypeListToStringSlice(t *testing.T) {
	testCases := []struct {
		name     string
		input    types.List
		expected []string
	}{
		{
			name:     "test conversion",
			input:    StringSliceToTypeList([]string{"a", "b", "c"}),
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "test special character conversion",
			input:    StringSliceToTypeList([]string{"---BEGIN CERTIFICATE---\nhello world\n---END CERTIFICATE---\n"}),
			expected: []string{"---BEGIN CERTIFICATE---\nhello world\n---END CERTIFICATE---\n"},
		},
		{
			name:     "test nil conversion",
			input:    StringSliceToTypeList(nil),
			expected: nil,
		},
		{
			name:     "test empty conversion",
			input:    StringSliceToTypeList([]string{}),
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TypeListToStringSlice(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, but got %v", tc.expected, result)
			}
		})
	}
}

func TestFindUserByName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockUserServiceClient(ctrl)

	testCases := []struct {
		name         string
		setupMock    func()
		inputName    string
		expectedUser *dataplanev1.ListUsersResponse_User
		expectedErr  string
	}{
		{
			name: "User found",
			setupMock: func() {
				mockClient.EXPECT().ListUsers(gomock.Any(), &dataplanev1.ListUsersRequest{
					Filter: &dataplanev1.ListUsersRequest_Filter{
						Name: "alice",
					},
				}).Return(&dataplanev1.ListUsersResponse{
					Users: []*dataplanev1.ListUsersResponse_User{
						{Name: "alice"},
						{Name: "bob"},
					},
				}, nil)
			},
			inputName:    "alice",
			expectedUser: &dataplanev1.ListUsersResponse_User{Name: "alice"},
			expectedErr:  "",
		},
		{
			name: "User not found",
			setupMock: func() {
				mockClient.EXPECT().ListUsers(gomock.Any(), &dataplanev1.ListUsersRequest{
					Filter: &dataplanev1.ListUsersRequest_Filter{
						Name: "charlie",
					},
				}).Return(&dataplanev1.ListUsersResponse{
					Users: []*dataplanev1.ListUsersResponse_User{
						{Name: "alice"},
						{Name: "bob"},
					},
				}, nil)
			},
			inputName:    "charlie",
			expectedUser: nil,
			expectedErr:  `user "charlie" not found`,
		},
		{
			name: "ListUsers error",
			setupMock: func() {
				mockClient.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Return(nil, errors.New("connection error"))
			},
			inputName:    "alice",
			expectedUser: nil,
			expectedErr:  "connection error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			user, err := FindUserByName(context.Background(), tc.inputName, mockClient)

			if tc.expectedErr != "" {
				if err == nil {
					t.Errorf("Expected error %q, but got nil", tc.expectedErr)
				} else if err.Error() != tc.expectedErr {
					t.Errorf("Expected error %q, but got %q", tc.expectedErr, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(user, tc.expectedUser) {
				t.Errorf("Expected user %+v, but got %+v", tc.expectedUser, user)
			}
		})
	}
}

func TestTopicConfigurationToMap(t *testing.T) {
	testCases := []struct {
		name        string
		input       []*dataplanev1.Topic_Configuration
		expected    types.Map
		expectedErr string
	}{
		{
			name:  "Empty configuration",
			input: []*dataplanev1.Topic_Configuration{},
			expected: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{})
				return m
			}(),
			expectedErr: "",
		},
		{
			name: "Single configuration",
			input: []*dataplanev1.Topic_Configuration{
				{Name: "retention.ms", Value: StringToStringPointer("86400000")},
			},
			expected: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{
					"retention.ms": types.StringValue("86400000"),
				})
				return m
			}(),
			expectedErr: "",
		},
		{
			name: "Multiple configurations",
			input: []*dataplanev1.Topic_Configuration{
				{Name: "retention.ms", Value: StringToStringPointer("86400000")},
				{Name: "cleanup.policy", Value: StringToStringPointer("delete")},
				{Name: "max.message.bytes", Value: StringToStringPointer("1000000")},
			},
			expected: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{
					"retention.ms":      types.StringValue("86400000"),
					"cleanup.policy":    types.StringValue("delete"),
					"max.message.bytes": types.StringValue("1000000"),
				})
				return m
			}(),
			expectedErr: "",
		},
		{
			name: "Configuration with nil value",
			input: []*dataplanev1.Topic_Configuration{
				{Name: "retention.ms", Value: StringToStringPointer("86400000")},
				{Name: "cleanup.policy", Value: nil},
			},
			expected: func() types.Map {
				return types.Map{}
			}(),
			expectedErr: "nil value for topic configuration \"cleanup.policy\"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := TopicConfigurationToMap(tc.input)

			if tc.expectedErr != "" {
				if err == nil {
					t.Errorf("Expected error %q, but got nil", tc.expectedErr)
				} else if err.Error() != tc.expectedErr {
					t.Errorf("Expected error %q, but got %q", tc.expectedErr, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %+v, but got %+v", tc.expected, result)
			}
		})
	}
}

func TestMapToCreateTopicConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		input       types.Map
		expected    []*dataplanev1.CreateTopicRequest_Topic_Config
		expectedErr string
	}{
		{
			name: "Empty configuration",
			input: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{})
				return m
			}(),
			expected:    nil,
			expectedErr: "",
		},
		{
			name: "Single configuration",
			input: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{
					"retention.ms": types.StringValue("86400000"),
				})
				return m
			}(),
			expected: []*dataplanev1.CreateTopicRequest_Topic_Config{
				{Name: "retention.ms", Value: StringToStringPointer("86400000")},
			},
			expectedErr: "",
		},
		{
			name: "Multiple configurations",
			input: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{
					"cleanup.policy":    types.StringValue("delete"),
					"retention.ms":      types.StringValue("86400000"),
					"max.message.bytes": types.StringValue("1000000"),
				})
				return m
			}(),
			expected: []*dataplanev1.CreateTopicRequest_Topic_Config{
				{Name: "cleanup.policy", Value: StringToStringPointer("delete")},
				{Name: "retention.ms", Value: StringToStringPointer("86400000")},
				{Name: "max.message.bytes", Value: StringToStringPointer("1000000")},
			},
			expectedErr: "",
		},
		{
			name: "Configuration with null value",
			input: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{
					"retention.ms":   types.StringValue("86400000"),
					"cleanup.policy": types.StringNull(),
				})
				return m
			}(),
			expected:    nil,
			expectedErr: "topic configuration \"cleanup.policy\" must have a value",
		},
		{
			name: "Configuration with unknown value",
			input: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{
					"retention.ms":   types.StringValue("86400000"),
					"cleanup.policy": types.StringUnknown(),
				})
				return m
			}(),
			expected:    nil,
			expectedErr: "topic configuration \"cleanup.policy\" must have a value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := MapToCreateTopicConfiguration(tc.input)
			if tc.expectedErr != "" {
				if err == nil {
					t.Errorf("Expected error %q, but got nil", tc.expectedErr)
				} else if err.Error() != tc.expectedErr {
					t.Errorf("Expected error %q, but got %q", tc.expectedErr, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			sort.Slice(tc.expected, func(i, j int) bool {
				return tc.expected[i].Name < tc.expected[j].Name
			})

			sort.Slice(result, func(i, j int) bool {
				return result[i].Name < result[j].Name
			})

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %+v, but got %+v", tc.expected, result)
			}
		})
	}
}

func TestFindTopicByName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockTopicServiceClient(ctrl)

	testCases := []struct {
		name          string
		setupMock     func()
		inputName     string
		expectedTopic *dataplanev1.ListTopicsResponse_Topic
		expectedErr   string
	}{
		{
			name: "Topic found",
			setupMock: func() {
				mockClient.EXPECT().ListTopics(gomock.Any(), &dataplanev1.ListTopicsRequest{
					Filter: &dataplanev1.ListTopicsRequest_Filter{
						NameContains: "test-topic",
					},
				}).Return(&dataplanev1.ListTopicsResponse{
					Topics: []*dataplanev1.ListTopicsResponse_Topic{
						{Name: "test-topic"},
						{Name: "another-topic"},
					},
				}, nil)
			},
			inputName:     "test-topic",
			expectedTopic: &dataplanev1.ListTopicsResponse_Topic{Name: "test-topic"},
			expectedErr:   "",
		},
		{
			name: "Topic not found",
			setupMock: func() {
				mockClient.EXPECT().ListTopics(gomock.Any(), &dataplanev1.ListTopicsRequest{
					Filter: &dataplanev1.ListTopicsRequest_Filter{
						NameContains: "non-existent-topic",
					},
				}).Return(&dataplanev1.ListTopicsResponse{
					Topics: []*dataplanev1.ListTopicsResponse_Topic{
						{Name: "test-topic"},
						{Name: "another-topic"},
					},
				}, nil)
			},
			inputName:     "non-existent-topic",
			expectedTopic: nil,
			expectedErr:   "topic non-existent-topic not found",
		},
		{
			name: "ListTopics error",
			setupMock: func() {
				mockClient.EXPECT().ListTopics(gomock.Any(), gomock.Any()).Return(nil, errors.New("connection error"))
			},
			inputName:     "test-topic",
			expectedTopic: nil,
			expectedErr:   "connection error",
		},
		{
			name: "Topic found on second page",
			setupMock: func() {
				// First page with token
				mockClient.EXPECT().ListTopics(gomock.Any(), &dataplanev1.ListTopicsRequest{
					Filter: &dataplanev1.ListTopicsRequest_Filter{
						NameContains: "xyz",
					},
					PageToken: "",
				}).Return(&dataplanev1.ListTopicsResponse{
					Topics: []*dataplanev1.ListTopicsResponse_Topic{
						{Name: "app1_xyz_logs"},
						{Name: "app2_xyz_metrics"},
					},
					NextPageToken: "page2",
				}, nil)

				// Second page without token
				mockClient.EXPECT().ListTopics(gomock.Any(), &dataplanev1.ListTopicsRequest{
					Filter: &dataplanev1.ListTopicsRequest_Filter{
						NameContains: "xyz",
					},
					PageToken: "page2",
				}).Return(&dataplanev1.ListTopicsResponse{
					Topics: []*dataplanev1.ListTopicsResponse_Topic{
						{Name: "xyz"},
						{Name: "xyz_internal_data"},
					},
					NextPageToken: "",
				}, nil)
			},
			inputName:     "xyz",
			expectedTopic: &dataplanev1.ListTopicsResponse_Topic{Name: "xyz"},
			expectedErr:   "",
		},
		{
			name: "Topic not found after multiple pages",
			setupMock: func() {
				// First page
				mockClient.EXPECT().ListTopics(gomock.Any(), &dataplanev1.ListTopicsRequest{
					Filter: &dataplanev1.ListTopicsRequest_Filter{
						NameContains: "missing",
					},
					PageToken: "",
				}).Return(&dataplanev1.ListTopicsResponse{
					Topics: []*dataplanev1.ListTopicsResponse_Topic{
						{Name: "missing_topic1"},
						{Name: "missing_topic2"},
					},
					NextPageToken: "page2",
				}, nil)

				// Second page
				mockClient.EXPECT().ListTopics(gomock.Any(), &dataplanev1.ListTopicsRequest{
					Filter: &dataplanev1.ListTopicsRequest_Filter{
						NameContains: "missing",
					},
					PageToken: "page2",
				}).Return(&dataplanev1.ListTopicsResponse{
					Topics: []*dataplanev1.ListTopicsResponse_Topic{
						{Name: "missing_topic3"},
					},
					NextPageToken: "",
				}, nil)
			},
			inputName:     "missing",
			expectedTopic: nil,
			expectedErr:   "topic missing not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			topic, err := FindTopicByName(context.Background(), tc.inputName, mockClient)

			if tc.expectedErr != "" {
				if err == nil {
					t.Errorf("Expected error %q, but got nil", tc.expectedErr)
				} else if err.Error() != tc.expectedErr {
					t.Errorf("Expected error %q, but got %q", tc.expectedErr, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(topic, tc.expectedTopic) {
				t.Errorf("Expected topic %+v, but got %+v", tc.expectedTopic, topic)
			}
		})
	}
}

func normalizeString(s string) string {
	// Replace all whitespace with single spaces
	s = strings.Join(strings.Fields(s), " ")
	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return s
}

func TestDeserializeGrpcError(t *testing.T) {
	detailedStatus, _ := grpcstatus.New(codes.InvalidArgument, "invalid parameter").WithDetails(
		&errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "user_id",
					Description: "must be positive integer",
				},
			},
		},
	)
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "regular error",
			err:      errors.New("standard error"),
			expected: "standard error",
		},
		{
			name:     "basic grpc error",
			err:      grpcstatus.Error(codes.NotFound, "resource not found"),
			expected: "NotFound : resource not found",
		},
		{
			name:     "grpc error",
			err:      detailedStatus.Err(),
			expected: "InvalidArgument : invalid parameter\n[field_violations:{field:\"user_id\" description:\"must be positive integer\"}]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeserializeGrpcError(tt.err)
			if normalizeString(got) != normalizeString(tt.expected) {
				t.Errorf("DeserializeGrpcError() got:\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}

func TestRetryGetCluster(t *testing.T) {
	testCases := []struct {
		name            string
		timeout         time.Duration
		mockSetup       func(m *mocks.MockCpClientSet)
		retryFunc       func(cluster *controlplanev1.Cluster) *RetryError
		expectedCluster *controlplanev1.Cluster
		expectedErr     error
	}{
		{
			name:    "Cluster is ready immediately",
			timeout: 5 * time.Minute,
			mockSetup: func(m *mocks.MockCpClientSet) {
				m.EXPECT().
					ClusterForID(gomock.Any(), "test-cluster-id").
					Return(&controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_READY}, nil)
			},
			retryFunc: func(cluster *controlplanev1.Cluster) *RetryError {
				if cluster.GetState() == controlplanev1.Cluster_STATE_READY {
					return nil
				}
				return RetryableError(fmt.Errorf("unexpected state: %v", cluster.GetState()))
			},
			expectedCluster: &controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_READY},
			expectedErr:     nil,
		},
		{
			name:    "Cluster requires retries",
			timeout: 5 * time.Minute,
			mockSetup: func(m *mocks.MockCpClientSet) {
				gomock.InOrder(
					m.EXPECT().
						ClusterForID(gomock.Any(), "test-cluster-id").
						Return(&controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_CREATING}, nil),
					m.EXPECT().
						ClusterForID(gomock.Any(), "test-cluster-id").
						Return(&controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_READY}, nil),
				)
			},
			retryFunc: func(cluster *controlplanev1.Cluster) *RetryError {
				if cluster.GetState() == controlplanev1.Cluster_STATE_READY {
					return nil
				}
				return RetryableError(fmt.Errorf("unexpected state: %v", cluster.GetState()))
			},
			expectedCluster: &controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_READY},
			expectedErr:     nil,
		},
		{
			name:    "Cluster fails to become ready (timeout)",
			timeout: 100 * time.Millisecond,
			mockSetup: func(m *mocks.MockCpClientSet) {
				m.EXPECT().
					ClusterForID(gomock.Any(), "test-cluster-id").
					Return(&controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_CREATING}, nil).
					AnyTimes()
			},
			retryFunc: func(_ *controlplanev1.Cluster) *RetryError {
				return RetryableError(errors.New("cluster not ready"))
			},
			expectedCluster: &controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_CREATING},
			expectedErr:     &TimeoutError{Timeout: 100 * time.Millisecond, Wrapped: errors.New("cluster not ready")},
		},
		{
			name:    "Cluster fails to become ready (non-retryable error)",
			timeout: 5 * time.Minute,
			mockSetup: func(m *mocks.MockCpClientSet) {
				m.EXPECT().
					ClusterForID(gomock.Any(), "test-cluster-id").
					Return(nil, errors.New("cluster failed"))
			},
			retryFunc: func(_ *controlplanev1.Cluster) *RetryError {
				return NonRetryableError(errors.New("cluster failed"))
			},
			expectedCluster: nil,
			expectedErr:     errors.New("cluster failed"),
		},
		{
			name:    "Cluster not found",
			timeout: 5 * time.Minute,
			mockSetup: func(m *mocks.MockCpClientSet) {
				m.EXPECT().
					ClusterForID(gomock.Any(), "test-cluster-id").
					Return(nil, NotFoundError{Message: "test-cluster-id not found"})
			},
			retryFunc: func(_ *controlplanev1.Cluster) *RetryError {
				return nil
			},
			expectedCluster: nil,
			expectedErr:     nil,
		},
		{
			name:    "Cluster goes through BYOC state",
			timeout: 5 * time.Minute,
			mockSetup: func(m *mocks.MockCpClientSet) {
				gomock.InOrder(
					m.EXPECT().
						ClusterForID(gomock.Any(), "test-cluster-id").
						Return(&controlplanev1.Cluster{
							State: controlplanev1.Cluster_STATE_CREATING_AGENT,
							Type:  controlplanev1.Cluster_TYPE_BYOC,
						}, nil),
					m.EXPECT().
						ClusterForID(gomock.Any(), "test-cluster-id").
						Return(&controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_READY}, nil),
				)
			},
			retryFunc: func(cluster *controlplanev1.Cluster) *RetryError {
				if cluster.GetState() == controlplanev1.Cluster_STATE_CREATING_AGENT {
					return RetryableError(errors.New("cluster in BYOC state"))
				}
				return nil
			},
			expectedCluster: &controlplanev1.Cluster{State: controlplanev1.Cluster_STATE_READY},
			expectedErr:     nil,
		},
		{
			name:    "Unhandled cluster state",
			timeout: 5 * time.Minute,
			mockSetup: func(m *mocks.MockCpClientSet) {
				m.EXPECT().
					ClusterForID(gomock.Any(), "test-cluster-id").
					Return(nil, errors.New("invalid state"))
			},
			retryFunc: func(_ *controlplanev1.Cluster) *RetryError {
				return NonRetryableError(errors.New("unhandled state"))
			},
			expectedCluster: nil,
			expectedErr:     errors.New("invalid state"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockCpClientSet(ctrl)
			tc.mockSetup(mockClient)

			ctx := context.Background()
			cluster, err := RetryGetCluster(ctx, tc.timeout, "test-cluster-id", mockClient, tc.retryFunc)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				var timeoutErr *TimeoutError
				var notFoundErr NotFoundError
				switch {
				case errors.As(tc.expectedErr, &timeoutErr):
					// For timeout errors, verify the type and properties
					var actualTimeoutErr *TimeoutError
					if assert.True(t, errors.As(err, &actualTimeoutErr), "expected TimeoutError") {
						assert.Equal(t, timeoutErr.Timeout, actualTimeoutErr.Timeout)
						assert.Equal(t, timeoutErr.Wrapped.Error(), actualTimeoutErr.Wrapped.Error())
					}
				case errors.As(tc.expectedErr, &notFoundErr):
					// For NotFoundError, verify the type and message
					var actualNotFoundErr NotFoundError
					if assert.True(t, errors.As(err, &actualNotFoundErr), "expected NotFoundError") {
						assert.Equal(t, notFoundErr.Message, actualNotFoundErr.Message)
					}
				default:
					// For other errors, compare the error messages
					assert.Equal(t, tc.expectedErr.Error(), err.Error())
				}
			}

			if tc.expectedCluster == nil {
				assert.Nil(t, cluster, "expected nil cluster")
			} else {
				assert.Equal(t, tc.expectedCluster.State, cluster.State)
			}
		})
	}
}

func TestIsNil(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		// Nil-able types that are nil
		{
			name:     "nil pointer",
			value:    (*int)(nil),
			expected: true,
		},
		{
			name:     "nil interface",
			value:    any(nil),
			expected: true,
		},
		{
			name:     "nil map",
			value:    map[string]int(nil),
			expected: true,
		},
		{
			name:     "nil slice",
			value:    []int(nil),
			expected: true,
		},
		{
			name:     "nil function",
			value:    (func())(nil),
			expected: true,
		},
		{
			name:     "nil channel",
			value:    chan int(nil),
			expected: true,
		},

		// Nil-able types that are not nil
		{
			name:     "non-nil pointer",
			value:    new(int),
			expected: false,
		},
		{
			name:     "non-nil interface with value",
			value:    any(42),
			expected: false,
		},
		{
			name:     "empty map (not nil)",
			value:    make(map[string]int),
			expected: false,
		},
		{
			name:     "empty slice (not nil)",
			value:    make([]int, 0),
			expected: false,
		},
		{
			name:     "non-nil function",
			value:    func() {},
			expected: false,
		},
		{
			name:     "non-nil channel",
			value:    make(chan int),
			expected: false,
		},

		// Non-nil-able types (should always return false)
		{
			name:     "int value",
			value:    42,
			expected: false,
		},
		{
			name:     "string value",
			value:    "hello",
			expected: false,
		},
		{
			name:     "empty string",
			value:    "",
			expected: false,
		},
		{
			name:     "boolean true",
			value:    true,
			expected: false,
		},
		{
			name:     "boolean false",
			value:    false,
			expected: false,
		},
		{
			name:     "float value",
			value:    3.14,
			expected: false,
		},
		{
			name:     "struct value",
			value:    struct{ Name string }{Name: "test"},
			expected: false,
		},
		{
			name:     "array value",
			value:    [3]int{1, 2, 3},
			expected: false,
		},
		{
			name:     "empty string",
			value:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNil(tt.value)
			assert.Equal(t, tt.expected, result, "IsNil(%T) = %v, expected %v", tt.value, result, tt.expected)
		})
	}
}

// TestIsNilGenericTypes tests the function with specific generic type constraints
func TestIsNilGenericTypes(t *testing.T) {
	// Test with specific types to ensure generics work correctly

	// Test with string pointer
	var strPtr *string
	assert.True(t, IsNil(strPtr), "nil string pointer should be nil")

	nonNilStrPtr := new(string)
	assert.False(t, IsNil(nonNilStrPtr), "non-nil string pointer should not be nil")

	// Test with custom struct pointer
	type CustomStruct struct {
		Field string
	}
	var customPtr *CustomStruct
	assert.True(t, IsNil(customPtr), "nil custom struct pointer should be nil")

	nonNilCustomPtr := &CustomStruct{}
	assert.False(t, IsNil(nonNilCustomPtr), "non-nil custom struct pointer should not be nil")

	// Test with interface containing nil pointer
	var nilInterface any = (*int)(nil)
	assert.True(t, IsNil(nilInterface), "interface containing nil pointer should be nil")

	// Test with truly nil interface
	var trueNilInterface any
	assert.True(t, IsNil(trueNilInterface), "truly nil interface should be nil")
}

// TestIsNilEdgeCases tests edge cases and reflect.Invalid scenarios
func TestIsNilEdgeCases(t *testing.T) {
	// Test with reflect.Value of Invalid kind (though this is hard to create directly)
	// The function handles reflect.Invalid by returning true

	// Test with nested pointers
	var nestedPtr **int
	assert.True(t, IsNil(nestedPtr), "nil nested pointer should be nil")

	// Test with pointer to nil pointer
	var innerPtr *int
	nestedPtrToNil := &innerPtr
	assert.False(t, IsNil(nestedPtrToNil), "pointer to nil pointer should not be nil itself")

	// Test with slice of pointers
	var sliceOfPtrs []*int
	assert.True(t, IsNil(sliceOfPtrs), "nil slice of pointers should be nil")

	emptySliceOfPtrs := make([]*int, 0)
	assert.False(t, IsNil(emptySliceOfPtrs), "empty slice of pointers should not be nil")
}

func TestIsPermissionDenied(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "gRPC PermissionDenied error",
			err:      grpcstatus.Error(codes.PermissionDenied, "access denied"),
			expected: true,
		},
		{
			name:     "HTTP 403 error",
			err:      errors.New("HTTP 403 Forbidden"),
			expected: true,
		},
		{
			name:     "Schema Registry forbidden error",
			err:      errors.New("Forbidden (missing required ACLs)"),
			expected: true,
		},
		{
			name:     "case insensitive forbidden check",
			err:      errors.New("request FORBIDDEN by server"),
			expected: true,
		},
		{
			name:     "missing required ACLs check",
			err:      errors.New("Error: Missing Required ACLs for operation"),
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "gRPC NotFound error (not permission)",
			err:      grpcstatus.Error(codes.NotFound, "not found"),
			expected: false,
		},
		{
			name:     "gRPC Internal error (not permission)",
			err:      grpcstatus.Error(codes.Internal, "internal error"),
			expected: false,
		},
		{
			name:     "Exact error message from schema read failure",
			err:      errors.New("Unable to read schema for subject test-topic-value: Forbidden (missing required ACLs)"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPermissionDenied(tt.err)
			assert.Equal(t, tt.expected, result, "IsPermissionDenied(%v) = %v, want %v", tt.err, result, tt.expected)
		})
	}
}

func TestConvertToConsoleURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard cluster API URL",
			input:    "https://api-12345.cluster-id.byoc.prd.cloud.redpanda.com",
			expected: "https://console-12345.cluster-id.byoc.prd.cloud.redpanda.com",
		},
		{
			name:     "URL with different cluster ID",
			input:    "https://api-abcdef.d110a6bu3l09un9dm4jg.byoc.prd.cloud.redpanda.com",
			expected: "https://console-abcdef.d110a6bu3l09un9dm4jg.byoc.prd.cloud.redpanda.com",
		},
		{
			name:     "URL without api- prefix should not change",
			input:    "https://console-12345.cluster-id.byoc.prd.cloud.redpanda.com",
			expected: "https://console-12345.cluster-id.byoc.prd.cloud.redpanda.com",
		},
		{
			name:     "http protocol",
			input:    "http://api-12345.cluster-id.byoc.prd.cloud.redpanda.com",
			expected: "http://console-12345.cluster-id.byoc.prd.cloud.redpanda.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToConsoleURL(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertToConsoleURL(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
