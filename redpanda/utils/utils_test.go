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
	"github.com/stretchr/testify/assert"
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
			wantErr: "timeout reached",
		},
		{
			name:    "Operation times out with unspecified",
			op:      &controlplanev1beta2.Operation{State: controlplanev1beta2.Operation_STATE_UNSPECIFIED},
			timeout: 100 * time.Millisecond,
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1beta2.Operation_STATE_UNSPECIFIED), nil).AnyTimes()
			},
			wantErr: "timeout reached",
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
			err := AreWeDoneYet(ctx, tc.op, tc.timeout, time.Second, mockClient)
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
			input:    TestingOnlyStringSliceToTypeList([]string{"a", "b", "c"}),
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "test empty conversion",
			input:    TestingOnlyStringSliceToTypeList([]string{}),
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

func TestSplitSchemeDefPort(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		defaultPort string
		expected    string
		expectError bool
	}{
		{
			name:        "URL with scheme and port",
			url:         "http://example.com:8080",
			defaultPort: "80",
			expected:    "example.com:8080",
			expectError: false,
		},
		{
			name:        "URL with scheme, no port",
			url:         "https://example.com",
			defaultPort: "443",
			expected:    "example.com:443",
			expectError: false,
		},
		{
			name:        "URL without scheme, with port",
			url:         "example.com:9090",
			defaultPort: "80",
			expected:    "example.com:9090",
			expectError: false,
		},
		{
			name:        "URL without scheme, no port",
			url:         "example.com",
			defaultPort: "80",
			expected:    "example.com:80",
			expectError: false,
		},
		{
			name:        "IP address with port",
			url:         "192.168.1.1:8080",
			defaultPort: "80",
			expected:    "192.168.1.1:8080",
			expectError: false,
		},
		{
			name:        "IP address without port",
			url:         "192.168.1.1",
			defaultPort: "80",
			expected:    "192.168.1.1:80",
			expectError: false,
		},
		{
			name:        "Invalid URL",
			url:         "http://[invalid",
			defaultPort: "80",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty URL",
			url:         "",
			defaultPort: "80",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := SplitSchemeDefPort(tc.url, tc.defaultPort)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("Expected %q, but got %q", tc.expected, result)
				}
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

func TestValidateThroughputTier(t *testing.T) {
	tests := []struct {
		name           string
		throughputTier string
		cloudProvider  string
		clusterType    string
		region         string
		mockSetup      func(*mocks.MockThroughputTierClient)
		expectedError  string
	}{
		{
			name:           "Valid throughput tier",
			throughputTier: "tier-1",
			cloudProvider:  "aws",
			clusterType:    "dedicated",
			region:         "us-east-1",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.ListThroughputTiersResponse{
					ThroughputTiers: []*controlplanev1beta2.ThroughputTier{
						{Name: "tier-1"},
						{Name: "tier-2"},
					},
				}, nil)
			},
			expectedError: "",
		},
		{
			name:           "Invalid throughput tier",
			throughputTier: "tier-3",
			cloudProvider:  "aws",
			clusterType:    "dedicated",
			region:         "us-east-1",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.ListThroughputTiersResponse{
					ThroughputTiers: []*controlplanev1beta2.ThroughputTier{
						{Name: "tier-1"},
						{Name: "tier-2"},
					},
				}, nil)
			},
			expectedError: "invalid throughput tier tier-3, please select a valid throughput tier",
		},
		{
			name:           "API error",
			throughputTier: "tier-1",
			cloudProvider:  "aws",
			clusterType:    "dedicated",
			region:         "us-east-1",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("API error"))
			},
			expectedError: "API error",
		},
		{
			name:           "Invalid cloud provider",
			throughputTier: "tier-1",
			cloudProvider:  "invalid",
			clusterType:    "dedicated",
			region:         "us-east-1",
			mockSetup:      func(_ *mocks.MockThroughputTierClient) {},
			expectedError:  "provider \"invalid\" not supported",
		},
		{
			name:           "Invalid cluster type",
			throughputTier: "tier-1",
			cloudProvider:  "aws",
			clusterType:    "invalid",
			region:         "us-east-1",
			mockSetup:      func(_ *mocks.MockThroughputTierClient) {},
			expectedError:  "cluster type \"invalid\" not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockThroughputTierClient(ctrl)
			tt.mockSetup(mockClient)

			err := ValidateThroughputTier(context.Background(), mockClient, tt.throughputTier, tt.cloudProvider, tt.clusterType, tt.region)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
