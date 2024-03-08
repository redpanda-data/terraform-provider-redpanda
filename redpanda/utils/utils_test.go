package utils

import (
	"context"
	"github.com/golang/mock/gomock"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	"google.golang.org/genproto/googleapis/rpc/status"
	"testing"
	"time"
)

func TestAreWeDoneYet(t *testing.T) {
	testCases := []struct {
		name      string
		op        *cloudv1beta1.Operation
		timeout   time.Duration
		mockSetup func(m *mocks.MockOperationServiceClient)
		wantErr   string
	}{
		{
			name: "Operation completed successfully",
			op:   &cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_COMPLETED},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_COMPLETED}, nil)
			},
			timeout: 5 * time.Minute,
		},
		{
			name: "Operation goes unspecified but then completes",
			op: &cloudv1beta1.Operation{
				State: cloudv1beta1.Operation_STATE_IN_PROGRESS,
			},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				gomock.InOrder(
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_IN_PROGRESS}, nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_UNSPECIFIED}, nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_UNSPECIFIED}, nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_COMPLETED}, nil),
				)
			},
			timeout: 5 * time.Minute,
			wantErr: "",
		},
		{
			name: "Operation failed with an error",
			op: &cloudv1beta1.Operation{
				State: cloudv1beta1.Operation_STATE_IN_PROGRESS,
			},
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				gomock.InOrder(
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_IN_PROGRESS}, nil),
					m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{
						State: cloudv1beta1.Operation_STATE_FAILED,
						Result: &cloudv1beta1.Operation_Error{
							Error: &status.Status{
								Code:    1,
								Message: "operation failed",
							},
						},
					}, nil))
			},
			timeout: 5 * time.Minute,
			wantErr: "operation failed: operation failed",
		},
		{
			name:    "Operation times out",
			op:      &cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_IN_PROGRESS},
			timeout: 100 * time.Millisecond,
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_IN_PROGRESS}, nil).AnyTimes()
			},
			wantErr: "timeout reached",
		},
		{
			name:    "Operation times out with unspecified",
			op:      &cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_UNSPECIFIED},
			timeout: 100 * time.Millisecond,
			mockSetup: func(m *mocks.MockOperationServiceClient) {
				m.EXPECT().GetOperation(gomock.Any(), gomock.Any()).Return(&cloudv1beta1.Operation{State: cloudv1beta1.Operation_STATE_UNSPECIFIED}, nil).AnyTimes()
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
