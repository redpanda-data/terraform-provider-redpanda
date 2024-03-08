package mocks

import (
	"context"
	"fmt"
	"reflect"

	"github.com/golang/mock/gomock"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"google.golang.org/grpc"
)

// MockOperationServiceClient is a mock of OperationServiceClient interface.
type MockOperationServiceClient struct {
	ctrl     *gomock.Controller
	recorder *MockOperationServiceClientMockRecorder
}

// MockOperationServiceClientMockRecorder is the mock recorder for MockOperationServiceClient.
type MockOperationServiceClientMockRecorder struct {
	mock *MockOperationServiceClient
}

// NewMockOperationServiceClient creates a new mock instance.
func NewMockOperationServiceClient(ctrl *gomock.Controller) *MockOperationServiceClient {
	mock := &MockOperationServiceClient{ctrl: ctrl}
	mock.recorder = &MockOperationServiceClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOperationServiceClient) EXPECT() *MockOperationServiceClientMockRecorder {
	return m.recorder
}

// GetOperation mocks base method.
func (m *MockOperationServiceClient) GetOperation(ctx context.Context, in *cloudv1beta1.GetOperationRequest, opts ...grpc.CallOption) (*cloudv1beta1.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetOperation", varargs...)
	ret0, ok := ret[0].(*cloudv1beta1.Operation)
	if !ok {
		fmt.Println("unexpected type")
	}
	ret1, ok := ret[1].(error)
	if !ok {
		fmt.Print("unexpected type")
	}
	return ret0, ret1
}

// GetOperation indicates an expected call of GetOperation.
func (mr *MockOperationServiceClientMockRecorder) GetOperation(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOperation", reflect.TypeOf((*MockOperationServiceClient)(nil).GetOperation), varargs...)
}
