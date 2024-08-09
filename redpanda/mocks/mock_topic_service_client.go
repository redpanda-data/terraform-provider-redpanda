// Code generated by MockGen. DO NOT EDIT.
// Source: buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1alpha1/dataplanev1alpha1grpc (interfaces: TopicServiceClient)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	dataplanev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1alpha1"
	gomock "github.com/golang/mock/gomock"
	grpc "google.golang.org/grpc"
)

// MockTopicServiceClient is a mock of TopicServiceClient interface.
type MockTopicServiceClient struct {
	ctrl     *gomock.Controller
	recorder *MockTopicServiceClientMockRecorder
}

// MockTopicServiceClientMockRecorder is the mock recorder for MockTopicServiceClient.
type MockTopicServiceClientMockRecorder struct {
	mock *MockTopicServiceClient
}

// NewMockTopicServiceClient creates a new mock instance.
func NewMockTopicServiceClient(ctrl *gomock.Controller) *MockTopicServiceClient {
	mock := &MockTopicServiceClient{ctrl: ctrl}
	mock.recorder = &MockTopicServiceClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTopicServiceClient) EXPECT() *MockTopicServiceClientMockRecorder {
	return m.recorder
}

// CreateTopic mocks base method.
func (m *MockTopicServiceClient) CreateTopic(arg0 context.Context, arg1 *dataplanev1alpha1.CreateTopicRequest, arg2 ...grpc.CallOption) (*dataplanev1alpha1.CreateTopicResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CreateTopic", varargs...)
	ret0, _ := ret[0].(*dataplanev1alpha1.CreateTopicResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTopic indicates an expected call of CreateTopic.
func (mr *MockTopicServiceClientMockRecorder) CreateTopic(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTopic", reflect.TypeOf((*MockTopicServiceClient)(nil).CreateTopic), varargs...)
}

// DeleteTopic mocks base method.
func (m *MockTopicServiceClient) DeleteTopic(arg0 context.Context, arg1 *dataplanev1alpha1.DeleteTopicRequest, arg2 ...grpc.CallOption) (*dataplanev1alpha1.DeleteTopicResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteTopic", varargs...)
	ret0, _ := ret[0].(*dataplanev1alpha1.DeleteTopicResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteTopic indicates an expected call of DeleteTopic.
func (mr *MockTopicServiceClientMockRecorder) DeleteTopic(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteTopic", reflect.TypeOf((*MockTopicServiceClient)(nil).DeleteTopic), varargs...)
}

// GetTopicConfigurations mocks base method.
func (m *MockTopicServiceClient) GetTopicConfigurations(arg0 context.Context, arg1 *dataplanev1alpha1.GetTopicConfigurationsRequest, arg2 ...grpc.CallOption) (*dataplanev1alpha1.GetTopicConfigurationsResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetTopicConfigurations", varargs...)
	ret0, _ := ret[0].(*dataplanev1alpha1.GetTopicConfigurationsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTopicConfigurations indicates an expected call of GetTopicConfigurations.
func (mr *MockTopicServiceClientMockRecorder) GetTopicConfigurations(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTopicConfigurations", reflect.TypeOf((*MockTopicServiceClient)(nil).GetTopicConfigurations), varargs...)
}

// ListTopics mocks base method.
func (m *MockTopicServiceClient) ListTopics(arg0 context.Context, arg1 *dataplanev1alpha1.ListTopicsRequest, arg2 ...grpc.CallOption) (*dataplanev1alpha1.ListTopicsResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListTopics", varargs...)
	ret0, _ := ret[0].(*dataplanev1alpha1.ListTopicsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTopics indicates an expected call of ListTopics.
func (mr *MockTopicServiceClientMockRecorder) ListTopics(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTopics", reflect.TypeOf((*MockTopicServiceClient)(nil).ListTopics), varargs...)
}

// SetTopicConfigurations mocks base method.
func (m *MockTopicServiceClient) SetTopicConfigurations(arg0 context.Context, arg1 *dataplanev1alpha1.SetTopicConfigurationsRequest, arg2 ...grpc.CallOption) (*dataplanev1alpha1.SetTopicConfigurationsResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SetTopicConfigurations", varargs...)
	ret0, _ := ret[0].(*dataplanev1alpha1.SetTopicConfigurationsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SetTopicConfigurations indicates an expected call of SetTopicConfigurations.
func (mr *MockTopicServiceClientMockRecorder) SetTopicConfigurations(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTopicConfigurations", reflect.TypeOf((*MockTopicServiceClient)(nil).SetTopicConfigurations), varargs...)
}

// UpdateTopicConfigurations mocks base method.
func (m *MockTopicServiceClient) UpdateTopicConfigurations(arg0 context.Context, arg1 *dataplanev1alpha1.UpdateTopicConfigurationsRequest, arg2 ...grpc.CallOption) (*dataplanev1alpha1.UpdateTopicConfigurationsResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpdateTopicConfigurations", varargs...)
	ret0, _ := ret[0].(*dataplanev1alpha1.UpdateTopicConfigurationsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateTopicConfigurations indicates an expected call of UpdateTopicConfigurations.
func (mr *MockTopicServiceClientMockRecorder) UpdateTopicConfigurations(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateTopicConfigurations", reflect.TypeOf((*MockTopicServiceClient)(nil).UpdateTopicConfigurations), varargs...)
}
