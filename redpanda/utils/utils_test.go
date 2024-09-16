package utils

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	dataplanev1alpha2 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1alpha2"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	"google.golang.org/genproto/googleapis/rpc/status"
)

func TestAreWeDoneYet(t *testing.T) {
	testCases := []struct {
		name      string
		op        *controlplanev1beta2.Operation
		timeout   time.Duration
		mockSetup func(m *mocks.MockOperationServiceClient)
		wantErr   string
	}{
		{
			name: "Operation completed successfully",
			op:   &controlplanev1beta2.Operation{State: controlplanev1beta2.Operation_STATE_COMPLETED},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_COMPLETED), nil)
			},
			timeout: 5 * time.Minute,
		},
		{
			name: "Operation goes unspecified but then completes",
			op: &controlplanev1beta2.Operation{
				State: controlplanev1beta2.Operation_STATE_IN_PROGRESS,
			},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				gomock.InOrder(
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_IN_PROGRESS), nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_UNSPECIFIED), nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_UNSPECIFIED), nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_COMPLETED), nil),
				)
			},
			timeout: 5 * time.Minute,
		},
		{
			name: "Operation failed with an error",
			op: &controlplanev1beta2.Operation{
				State: controlplanev1beta2.Operation_STATE_IN_PROGRESS,
			},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				gomock.InOrder(
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_IN_PROGRESS), nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.GetOperationResponse{Operation: &controlplanev1beta2.Operation{
						State: controlplanev1beta2.Operation_STATE_FAILED,
						Result: &controlplanev1beta2.Operation_Error{
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
			op:      &controlplanev1beta2.Operation{State: controlplanev1beta2.Operation_STATE_IN_PROGRESS},
			timeout: 100 * time.Millisecond,
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_IN_PROGRESS), nil).AnyTimes()
			},
			wantErr: "timed out after 100ms: expected operation to be completed but was in state STATE_IN_PROGRESS",
		},
		{
			name:    "Operation times out with unspecified",
			op:      &controlplanev1beta2.Operation{State: controlplanev1beta2.Operation_STATE_UNSPECIFIED},
			timeout: 100 * time.Millisecond,
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_UNSPECIFIED), nil).AnyTimes()
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

func createOpResponse(state controlplanev1beta2.Operation_State) *controlplanev1beta2.GetOperationResponse {
	return &controlplanev1beta2.GetOperationResponse{
		Operation: &controlplanev1beta2.Operation{
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
			name:     "test empty conversion",
			input:    StringSliceToTypeList([]string{}),
			expected: nil,
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
		expectedUser *dataplanev1alpha2.ListUsersResponse_User
		expectedErr  string
	}{
		{
			name: "User found",
			setupMock: func() {
				mockClient.EXPECT().ListUsers(gomock.Any(), &dataplanev1alpha2.ListUsersRequest{
					Filter: &dataplanev1alpha2.ListUsersRequest_Filter{
						Name: "alice",
					},
				}).Return(&dataplanev1alpha2.ListUsersResponse{
					Users: []*dataplanev1alpha2.ListUsersResponse_User{
						{Name: "alice"},
						{Name: "bob"},
					},
				}, nil)
			},
			inputName:    "alice",
			expectedUser: &dataplanev1alpha2.ListUsersResponse_User{Name: "alice"},
			expectedErr:  "",
		},
		{
			name: "User not found",
			setupMock: func() {
				mockClient.EXPECT().ListUsers(gomock.Any(), &dataplanev1alpha2.ListUsersRequest{
					Filter: &dataplanev1alpha2.ListUsersRequest_Filter{
						Name: "charlie",
					},
				}).Return(&dataplanev1alpha2.ListUsersResponse{
					Users: []*dataplanev1alpha2.ListUsersResponse_User{
						{Name: "alice"},
						{Name: "bob"},
					},
				}, nil)
			},
			inputName:    "charlie",
			expectedUser: nil,
			expectedErr:  "user not found",
		},
		{
			name: "ListUsers error",
			setupMock: func() {
				mockClient.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("connection error"))
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
		input       []*dataplanev1alpha2.Topic_Configuration
		expected    types.Map
		expectedErr string
	}{
		{
			name:  "Empty configuration",
			input: []*dataplanev1alpha2.Topic_Configuration{},
			expected: func() types.Map {
				m, _ := types.MapValue(types.StringType, map[string]attr.Value{})
				return m
			}(),
			expectedErr: "",
		},
		{
			name: "Single configuration",
			input: []*dataplanev1alpha2.Topic_Configuration{
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
			input: []*dataplanev1alpha2.Topic_Configuration{
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
			input: []*dataplanev1alpha2.Topic_Configuration{
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
		expected    []*dataplanev1alpha2.CreateTopicRequest_Topic_Config
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
			expected: []*dataplanev1alpha2.CreateTopicRequest_Topic_Config{
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
			expected: []*dataplanev1alpha2.CreateTopicRequest_Topic_Config{
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
		expectedTopic *dataplanev1alpha2.ListTopicsResponse_Topic
		expectedErr   string
	}{
		{
			name: "Topic found",
			setupMock: func() {
				mockClient.EXPECT().ListTopics(gomock.Any(), &dataplanev1alpha2.ListTopicsRequest{
					Filter: &dataplanev1alpha2.ListTopicsRequest_Filter{
						NameContains: "test-topic",
					},
				}).Return(&dataplanev1alpha2.ListTopicsResponse{
					Topics: []*dataplanev1alpha2.ListTopicsResponse_Topic{
						{Name: "test-topic"},
						{Name: "another-topic"},
					},
				}, nil)
			},
			inputName:     "test-topic",
			expectedTopic: &dataplanev1alpha2.ListTopicsResponse_Topic{Name: "test-topic"},
			expectedErr:   "",
		},
		{
			name: "Topic not found",
			setupMock: func() {
				mockClient.EXPECT().ListTopics(gomock.Any(), &dataplanev1alpha2.ListTopicsRequest{
					Filter: &dataplanev1alpha2.ListTopicsRequest_Filter{
						NameContains: "non-existent-topic",
					},
				}).Return(&dataplanev1alpha2.ListTopicsResponse{
					Topics: []*dataplanev1alpha2.ListTopicsResponse_Topic{
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
				mockClient.EXPECT().ListTopics(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("connection error"))
			},
			inputName:     "test-topic",
			expectedTopic: nil,
			expectedErr:   "connection error",
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
