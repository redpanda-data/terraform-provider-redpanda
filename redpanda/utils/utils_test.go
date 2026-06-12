package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// TestMain shrinks Retry's backoff floor + ceiling to microseconds so the
// AreWeDoneYet burst tests (25-50 retries) run in milliseconds instead of
// the production timing (1s floor, 60s ceiling) that would push them past
// any reasonable unit-test budget.
func TestMain(m *testing.M) {
	retryInitialWait.Store(int64(time.Microsecond))
	retryMaxWait.Store(int64(10 * time.Microsecond))
	m.Run()
}

func TestAreWeDoneYet(t *testing.T) {
	testCases := []struct {
		name          string
		op            *controlplanev1.Operation
		timeout       time.Duration
		mockSetup     func(m *mocks.MockOperationServiceClient)
		wantErr       string
		wantErrPrefix string // for assertions where the message contains a runtime-measured value
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
		{
			// Long-running async ops can see 10+ consecutive Internals on
			// GetOperation while the server-side mutation completes — the
			// old hard count cap (10) tripped in production on a tag
			// mutation that succeeded server-side. The time-based stuck
			// cap (min(5m, timeout/6)) lets the 25 errors run through
			// TestMain's microsecond backoff well within the 5m window
			// before COMPLETED arrives. Cap doesn't fire.
			name: "Transient burst within stuck cap then completes",
			op:   &controlplanev1.Operation{State: controlplanev1.Operation_STATE_IN_PROGRESS},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				calls := []any{}
				for range 25 {
					calls = append(calls, m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(nil, grpcstatus.Error(codes.Internal, "internal error")))
				}
				calls = append(calls, m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_COMPLETED), nil))
				gomock.InOrder(calls...)
			},
			timeout: 5 * time.Minute,
		},
		{
			// Reset-on-success: 25 transients → IN_PROGRESS → 25 more
			// transients → COMPLETED. Each successful poll resets the stuck
			// window so the second burst is measured independently.
			name: "Reset stuck window on successful poll between bursts",
			op:   &controlplanev1.Operation{State: controlplanev1.Operation_STATE_IN_PROGRESS},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				calls := []any{}
				for range 25 {
					calls = append(calls, m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(nil, grpcstatus.Error(codes.Internal, "internal error")))
				}
				calls = append(calls, m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_IN_PROGRESS), nil))
				for range 25 {
					calls = append(calls, m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(nil, grpcstatus.Error(codes.Internal, "internal error")))
				}
				calls = append(calls, m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(createOpResponse(controlplanev1.Operation_STATE_COMPLETED), nil))
				gomock.InOrder(calls...)
			},
			timeout: 5 * time.Minute,
		},
		{
			// Sustained transients past the stuck cap with no successful
			// poll: bail with "server unresponsive for ...". Test uses a
			// short timeout (10ms → stuckCap ≈ 1.6ms) plus a slow-down
			// pause inside the mock so the retry loop exceeds stuckCap.
			// .AnyTimes() because the exact retry count depends on backoff
			// jitter; we only assert the message format.
			name: "Sustained transients past stuck cap bails",
			op:   &controlplanev1.Operation{State: controlplanev1.Operation_STATE_IN_PROGRESS},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ *controlplanev1.GetOperationRequest, _ ...any) (*controlplanev1.GetOperationResponse, error) {
						time.Sleep(2 * time.Millisecond)
						return nil, grpcstatus.Error(codes.Internal, "internal error")
					},
				).AnyTimes()
			},
			timeout: 10 * time.Millisecond,
			// Match the prefix; the full message includes a measured duration that varies per run.
			wantErrPrefix: "server unresponsive for ",
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
			switch {
			case tc.wantErrPrefix != "":
				if err == nil || !strings.HasPrefix(err.Error(), tc.wantErrPrefix) {
					t.Errorf("Expected error prefixed %q, got: %v", tc.wantErrPrefix, err)
				}
			case tc.wantErr != "":
				if err == nil || err.Error() != tc.wantErr {
					t.Errorf("Expected error %q, got: %v", tc.wantErr, err)
				}
			default:
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
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
	s = strings.Join(strings.Fields(s), " ")
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
			name:     "grpc error with empty message",
			err:      grpcstatus.Error(codes.Internal, ""),
			expected: "Internal (raw: rpc error: code = Internal desc = )",
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
		{
			name:    "Transient error succeeds after retries",
			timeout: 5 * time.Minute,
			mockSetup: func(m *mocks.MockCpClientSet) {
				gomock.InOrder(
					// First 3 calls return unavailable
					m.EXPECT().ClusterForID(gomock.Any(), "test-cluster-id").
						Return(nil, grpcstatus.Error(codes.Unavailable, "service unavailable")),
					m.EXPECT().ClusterForID(gomock.Any(), "test-cluster-id").
						Return(nil, grpcstatus.Error(codes.Unavailable, "service unavailable")),
					m.EXPECT().ClusterForID(gomock.Any(), "test-cluster-id").
						Return(nil, grpcstatus.Error(codes.Unavailable, "service unavailable")),
					// 4th call succeeds
					m.EXPECT().ClusterForID(gomock.Any(), "test-cluster-id").
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
			name:    "Transient error exceeds max retries",
			timeout: 5 * time.Minute,
			mockSetup: func(m *mocks.MockCpClientSet) {
				// Return unavailable 11 times (1 more than max of 10)
				m.EXPECT().ClusterForID(gomock.Any(), "test-cluster-id").
					Return(nil, grpcstatus.Error(codes.Unavailable, "service unavailable")).
					Times(10)
			},
			retryFunc: func(_ *controlplanev1.Cluster) *RetryError {
				return nil
			},
			expectedCluster: nil,
			expectedErr:     errors.New("max transient retries exceeded: rpc error: code = Unavailable desc = service unavailable"),
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
					var actualTimeoutErr *TimeoutError
					if assert.True(t, errors.As(err, &actualTimeoutErr), "expected TimeoutError") {
						assert.Equal(t, timeoutErr.Timeout, actualTimeoutErr.Timeout)
						assert.Equal(t, timeoutErr.Wrapped.Error(), actualTimeoutErr.Wrapped.Error())
					}
				case errors.As(tc.expectedErr, &notFoundErr):
					var actualNotFoundErr NotFoundError
					if assert.True(t, errors.As(err, &actualNotFoundErr), "expected NotFoundError") {
						assert.Equal(t, notFoundErr.Message, actualNotFoundErr.Message)
					}
				default:
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

// TestRetry_NoThunderingHerd
func TestRetry_NoThunderingHerd(t *testing.T) {
	const (
		workers      = 50
		serverCap    = 5
		serviceTime  = 20 * time.Millisecond
		totalTimeout = 20 * time.Second
	)

	sem := make(chan struct{}, serverCap)
	var (
		inFlight     atomic.Int64
		peakInFlight atomic.Int64
		successes    atomic.Int64
	)

	work := func() *RetryError {
		select {
		case sem <- struct{}{}:
		default:
			return RetryableError(errors.New("transient broker error: client closed"))
		}
		defer func() { <-sem }()

		cur := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			prev := peakInFlight.Load()
			if cur <= prev || peakInFlight.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(serviceTime)
		successes.Add(1)
		return nil
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			_ = Retry(context.Background(), totalTimeout, work)
		}()
	}
	close(start)
	wg.Wait()

	require.Equal(t, int64(workers), successes.Load(),
		"all workers should finish within %s; peak in-flight = %d",
		totalTimeout, peakInFlight.Load())
}

func TestIsNil(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
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

func TestIsNilGenericTypes(t *testing.T) {
	var strPtr *string
	assert.True(t, IsNil(strPtr), "nil string pointer should be nil")

	nonNilStrPtr := new(string)
	assert.False(t, IsNil(nonNilStrPtr), "non-nil string pointer should not be nil")

	type CustomStruct struct {
		Field string
	}
	var customPtr *CustomStruct
	assert.True(t, IsNil(customPtr), "nil custom struct pointer should be nil")

	nonNilCustomPtr := &CustomStruct{}
	assert.False(t, IsNil(nonNilCustomPtr), "non-nil custom struct pointer should not be nil")

	var nilInterface any = (*int)(nil)
	assert.True(t, IsNil(nilInterface), "interface containing nil pointer should be nil")

	var trueNilInterface any
	assert.True(t, IsNil(trueNilInterface), "truly nil interface should be nil")
}

func TestIsNilEdgeCases(t *testing.T) {
	var nestedPtr **int
	assert.True(t, IsNil(nestedPtr), "nil nested pointer should be nil")

	var innerPtr *int
	nestedPtrToNil := &innerPtr
	assert.False(t, IsNil(nestedPtrToNil), "pointer to nil pointer should not be nil itself")

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

func TestIsUnavailable(t *testing.T) {
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
			name:     "gRPC Unavailable error",
			err:      grpcstatus.Error(codes.Unavailable, "service unavailable"),
			expected: true,
		},
		{
			name:     "HTTP 503 error",
			err:      errors.New("unexpected status code 503"),
			expected: true,
		},
		{
			name:     "service unavailable string",
			err:      errors.New("Service Unavailable"),
			expected: true,
		},
		{
			name:     "unavailable in error message",
			err:      errors.New("server is currently unavailable"),
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "gRPC NotFound error (not unavailable)",
			err:      grpcstatus.Error(codes.NotFound, "not found"),
			expected: false,
		},
		{
			name:     "gRPC Internal error (not unavailable)",
			err:      grpcstatus.Error(codes.Internal, "internal error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsUnavailable(tt.err)
			assert.Equal(t, tt.expected, result, "IsUnavailable(%v) = %v, want %v", tt.err, result, tt.expected)
		})
	}
}

// TestIsTransientServerError covers the AreWeDoneYet retry classifier. It
// extends IsUnavailable to also include gRPC Internal, which the v6/v7 live
// cycles hit three times during serverless tag-mutation operation polling
// (mutation succeeded; read transiently glitched). Pin: Unavailable stays
// in, Internal now also in, NotFound stays out.
func TestIsTransientServerError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"gRPC Unavailable", grpcstatus.Error(codes.Unavailable, "x"), true},
		{"gRPC Internal", grpcstatus.Error(codes.Internal, "x"), true},
		{"HTTP 503 string", errors.New("got 503"), true},
		{"gRPC NotFound", grpcstatus.Error(codes.NotFound, "x"), false},
		{"gRPC InvalidArgument", grpcstatus.Error(codes.InvalidArgument, "x"), false},
		{"generic error", errors.New("plain"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsTransientServerError(tt.err))
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

func TestParseCPUToMillicores(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError bool
	}{
		{"100m", "100m", 100, false},
		{"500m", "500m", 500, false},
		{"1000m", "1000m", 1000, false},
		{"1 core", "1", 1000, false},
		{"2 cores", "2", 2000, false},
		{"0.5 cores", "0.5", 500, false},
		{"1.5 cores", "1.5", 1500, false},
		{"empty", "", 0, true},
		{"invalid", "abc", 0, true},
		{"invalid millicores", "abcm", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCPUToMillicores(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseMemoryToBytes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError bool
	}{
		{"256Ki", "256Ki", 256 * 1024, false},
		{"512Mi", "512Mi", 512 * 1024 * 1024, false},
		{"1Gi", "1Gi", 1024 * 1024 * 1024, false},
		{"2Gi", "2Gi", 2 * 1024 * 1024 * 1024, false},
		{"128M", "128M", 128 * 1000 * 1000, false},
		{"1G", "1G", 1000 * 1000 * 1000, false},
		{"500k", "500k", 500 * 1000, false},
		{"1024 bytes", "1024", 1024, false},
		{"1048576 bytes", "1048576", 1048576, false},
		{"1.5Gi", "1.5Gi", int64(1.5 * 1024 * 1024 * 1024), false},
		{"2.5M", "2.5M", int64(2.5 * 1000 * 1000), false},
		{"empty", "", 0, true},
		{"invalid", "abc", 0, true},
		{"invalid suffix", "100X", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMemoryToBytes(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetEffectivePassword(t *testing.T) {
	tests := []struct {
		name       string
		password   types.String
		passwordWO types.String
		expected   string
	}{
		{
			name:       "password_wo takes precedence over password",
			password:   types.StringValue("legacy-password"),
			passwordWO: types.StringValue("write-only-password"),
			expected:   "write-only-password",
		},
		{
			name:       "falls back to password when password_wo is null",
			password:   types.StringValue("legacy-password"),
			passwordWO: types.StringNull(),
			expected:   "legacy-password",
		},
		{
			name:       "falls back to password when password_wo is unknown",
			password:   types.StringValue("legacy-password"),
			passwordWO: types.StringUnknown(),
			expected:   "legacy-password",
		},
		{
			name:       "returns empty string when both are null",
			password:   types.StringNull(),
			passwordWO: types.StringNull(),
			expected:   "",
		},
		{
			name:       "returns empty password_wo when explicitly set to empty",
			password:   types.StringValue("legacy-password"),
			passwordWO: types.StringValue(""),
			expected:   "",
		},
		{
			name:       "returns password_wo even if password is null",
			password:   types.StringNull(),
			passwordWO: types.StringValue("write-only-password"),
			expected:   "write-only-password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEffectivePassword(tt.password, tt.passwordWO)
			assert.Equal(t, tt.expected, result, "GetEffectivePassword() = %q, expected %q", result, tt.expected)
		})
	}
}

func TestStringValueOrNull(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		isNull bool
		want   string
	}{
		{name: "empty string yields null", input: "", isNull: true},
		{name: "non-empty string yields value", input: "foo", want: "foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringValueOrNull(tt.input)
			assert.Equal(t, tt.isNull, got.IsNull())
			if !tt.isNull {
				assert.Equal(t, tt.want, got.ValueString())
			}
		})
	}
}

func TestPointerOrNil(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		assert.Nil(t, PointerOrNil(types.StringNull(), types.String.ValueString))
		assert.Nil(t, PointerOrNil(types.StringUnknown(), types.String.ValueString))
		got := PointerOrNil(types.StringValue("hello"), types.String.ValueString)
		require.NotNil(t, got)
		assert.Equal(t, "hello", *got)
		empty := PointerOrNil(types.StringValue(""), types.String.ValueString)
		require.NotNil(t, empty)
		assert.Equal(t, "", *empty)
	})
	t.Run("Bool", func(t *testing.T) {
		assert.Nil(t, PointerOrNil(types.BoolNull(), types.Bool.ValueBool))
		assert.Nil(t, PointerOrNil(types.BoolUnknown(), types.Bool.ValueBool))
		tr := PointerOrNil(types.BoolValue(true), types.Bool.ValueBool)
		require.NotNil(t, tr)
		assert.True(t, *tr)
		fa := PointerOrNil(types.BoolValue(false), types.Bool.ValueBool)
		require.NotNil(t, fa)
		assert.False(t, *fa)
	})
	t.Run("Int32", func(t *testing.T) {
		assert.Nil(t, PointerOrNil(types.Int32Null(), types.Int32.ValueInt32))
		assert.Nil(t, PointerOrNil(types.Int32Unknown(), types.Int32.ValueInt32))
		got := PointerOrNil(types.Int32Value(42), types.Int32.ValueInt32)
		require.NotNil(t, got)
		assert.Equal(t, int32(42), *got)
		zero := PointerOrNil(types.Int32Value(0), types.Int32.ValueInt32)
		require.NotNil(t, zero)
		assert.Equal(t, int32(0), *zero)
	})
	t.Run("Int64", func(t *testing.T) {
		assert.Nil(t, PointerOrNil(types.Int64Null(), types.Int64.ValueInt64))
		assert.Nil(t, PointerOrNil(types.Int64Unknown(), types.Int64.ValueInt64))
		got := PointerOrNil(types.Int64Value(99), types.Int64.ValueInt64)
		require.NotNil(t, got)
		assert.Equal(t, int64(99), *got)
	})
	t.Run("Float64", func(t *testing.T) {
		assert.Nil(t, PointerOrNil(types.Float64Null(), types.Float64.ValueFloat64))
		assert.Nil(t, PointerOrNil(types.Float64Unknown(), types.Float64.ValueFloat64))
		got := PointerOrNil(types.Float64Value(1.5), types.Float64.ValueFloat64)
		require.NotNil(t, got)
		assert.Equal(t, 1.5, *got)
	})
}

func TestStringSliceToTypeListOrNull(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		isNull bool
		want   []string
	}{
		{name: "nil yields null", input: nil, isNull: true},
		{name: "empty slice yields null", input: []string{}, isNull: true},
		{name: "non-empty slice yields list", input: []string{"a", "b"}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringSliceToTypeListOrNull(tt.input)
			assert.Equal(t, tt.isNull, got.IsNull())
			if !tt.isNull {
				assert.Equal(t, tt.want, TypeListToStringSlice(got))
			}
		})
	}
}

func TestNormalizeClusterAPIURL(t *testing.T) {
	const canonical = "https://api-abc.cid.byoc.prd.cloud.redpanda.com"
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{"host_port_443", "api-abc.cid.byoc.prd.cloud.redpanda.com:443", canonical},
		{"bare_host", "api-abc.cid.byoc.prd.cloud.redpanda.com", canonical},
		{"already_canonical", canonical, canonical},
		{"https_with_443", "https://api-abc.cid.byoc.prd.cloud.redpanda.com:443", canonical},
		{"https_trailing_slash", "https://api-abc.cid.byoc.prd.cloud.redpanda.com/", canonical},
		{"non_standard_port_kept", "api-abc.cid.byoc.prd.cloud.redpanda.com:9092", "https://api-abc.cid.byoc.prd.cloud.redpanda.com:9092"},
		{"empty", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeClusterAPIURL(tt.in))
		})
	}
}

// TestGetARNListFromAttributes_ErrorMessage pins that the not-found errors are
// well-formed. They previously used fmt.Errorf(fmt.Sprintf(...), suffix), which
// dropped the suffix as a stray EXTRA arg; go vet can't catch it because the
// format string is a function call rather than a literal.
func TestGetARNListFromAttributes_ErrorMessage(t *testing.T) {
	_, err := GetARNListFromAttributes("subnet", map[string]attr.Value{})
	require.Error(t, err)
	require.Equal(t, "subnet not found: object is missing or malformed for network resource", err.Error())

	obj, d := types.ObjectValue(
		map[string]attr.Type{"other": types.StringType},
		map[string]attr.Value{"other": types.StringValue("x")},
	)
	require.False(t, d.HasError())
	_, err = GetARNListFromAttributes("subnet", map[string]attr.Value{"subnet": obj})
	require.Error(t, err)
	require.Equal(t, "subnet not found: list is missing or malformed for network resource", err.Error())
}

// TestRunSubprocess_RemovesTempDir pins that runSubprocess cleans up the temp
// working directory it creates. It previously leaked one dir per invocation
// (no defer os.RemoveAll).
func TestRunSubprocess_RemovesTempDir(t *testing.T) {
	pattern := filepath.Join(os.TempDir(), "terraform-provider-redpanda-byoc*")
	before, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NoError(t, runSubprocess(context.Background(), nil, "echo", "hello"))
	after, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.Len(t, after, len(before), "runSubprocess leaked a temp dir: before=%v after=%v", before, after)
}
