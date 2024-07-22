package utils

import (
	"context"
	"reflect"
	"testing"
	"time"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/golang/mock/gomock"
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
			want: map[string]string{},
		},
		{
			name: "Single key",
			args: args{tags: mustMap(t, map[string]string{"key": "value"})},
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
