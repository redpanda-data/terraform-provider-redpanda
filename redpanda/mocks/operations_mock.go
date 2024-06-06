package mocks

import (
	"context"
	"fmt"
	"reflect"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/golang/mock/gomock"
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
func (m *MockOperationServiceClient) GetOperation(ctx context.Context, in *controlplanev1beta2.GetOperationRequest, opts ...grpc.CallOption) (*controlplanev1beta2.GetOperationResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetOperation", varargs...)
	ret0, ok := ret[0].(*controlplanev1beta2.GetOperationResponse)
	if !ok {
		fmt.Println("unexpected type")
	}
	ret1, ok := ret[1].(error)
	if !ok {
		fmt.Print("unexpected type")
	}
	return ret0, ret1
}

// ListOperations mocks base method.
func (m *MockOperationServiceClient) ListOperations(ctx context.Context, in *controlplanev1beta2.ListOperationsRequest, opts ...grpc.CallOption) (*controlplanev1beta2.ListOperationsResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListOperations", varargs...)
	ret0, ok := ret[0].(*controlplanev1beta2.ListOperationsResponse)
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

// ListOperations indicates an expected call of ListOperations.
func (mr *MockOperationServiceClientMockRecorder) ListOperations(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListOperation", reflect.TypeOf((*MockOperationServiceClient)(nil).ListOperations), varargs...)
}
