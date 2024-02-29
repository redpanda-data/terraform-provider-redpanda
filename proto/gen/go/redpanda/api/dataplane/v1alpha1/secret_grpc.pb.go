// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: redpanda/api/dataplane/v1alpha1/secret.proto

package dataplanev1alpha1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	SecretService_GetSecret_FullMethodName           = "/redpanda.api.dataplane.v1alpha1.SecretService/GetSecret"
	SecretService_ListSecrets_FullMethodName         = "/redpanda.api.dataplane.v1alpha1.SecretService/ListSecrets"
	SecretService_CreateSecret_FullMethodName        = "/redpanda.api.dataplane.v1alpha1.SecretService/CreateSecret"
	SecretService_UpdateSecret_FullMethodName        = "/redpanda.api.dataplane.v1alpha1.SecretService/UpdateSecret"
	SecretService_DeleteSecret_FullMethodName        = "/redpanda.api.dataplane.v1alpha1.SecretService/DeleteSecret"
	SecretService_GetConnectSecret_FullMethodName    = "/redpanda.api.dataplane.v1alpha1.SecretService/GetConnectSecret"
	SecretService_ListConnectSecrets_FullMethodName  = "/redpanda.api.dataplane.v1alpha1.SecretService/ListConnectSecrets"
	SecretService_CreateConnectSecret_FullMethodName = "/redpanda.api.dataplane.v1alpha1.SecretService/CreateConnectSecret"
	SecretService_UpdateConnectSecret_FullMethodName = "/redpanda.api.dataplane.v1alpha1.SecretService/UpdateConnectSecret"
	SecretService_DeleteConnectSecret_FullMethodName = "/redpanda.api.dataplane.v1alpha1.SecretService/DeleteConnectSecret"
)

// SecretServiceClient is the client API for SecretService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SecretServiceClient interface {
	// GetSecret retrieves the specific secret.
	GetSecret(ctx context.Context, in *GetSecretRequest, opts ...grpc.CallOption) (*GetSecretResponse, error)
	// ListSecrets lists the secrets based on optional filter.
	ListSecrets(ctx context.Context, in *ListSecretsRequest, opts ...grpc.CallOption) (*ListSecretsResponse, error)
	// CreateSecret creates the secret.
	CreateSecret(ctx context.Context, in *CreateSecretRequest, opts ...grpc.CallOption) (*CreateSecretResponse, error)
	// UpdateSecret updates the secret.
	UpdateSecret(ctx context.Context, in *UpdateSecretRequest, opts ...grpc.CallOption) (*UpdateSecretResponse, error)
	// DeleteSecret deletes the secret.
	DeleteSecret(ctx context.Context, in *DeleteSecretRequest, opts ...grpc.CallOption) (*DeleteSecretResponse, error)
	// GetConnectSecret retrieves the specific secret for a specific Connect.
	GetConnectSecret(ctx context.Context, in *GetConnectSecretRequest, opts ...grpc.CallOption) (*GetConnectSecretResponse, error)
	// ListConnectSecrets lists the Connect secrets based on optional filter.
	ListConnectSecrets(ctx context.Context, in *ListConnectSecretsRequest, opts ...grpc.CallOption) (*ListConnectSecretsResponse, error)
	// CreateConnectSecret creates the secret for a Connect.
	CreateConnectSecret(ctx context.Context, in *CreateConnectSecretRequest, opts ...grpc.CallOption) (*CreateConnectSecretResponse, error)
	// UpdateConnectSecret updates the Connect secret.
	UpdateConnectSecret(ctx context.Context, in *UpdateConnectSecretRequest, opts ...grpc.CallOption) (*UpdateConnectSecretResponse, error)
	// DeleteSecret deletes the secret.
	DeleteConnectSecret(ctx context.Context, in *DeleteConnectSecretRequest, opts ...grpc.CallOption) (*DeleteConnectSecretResponse, error)
}

type secretServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSecretServiceClient(cc grpc.ClientConnInterface) SecretServiceClient {
	return &secretServiceClient{cc}
}

func (c *secretServiceClient) GetSecret(ctx context.Context, in *GetSecretRequest, opts ...grpc.CallOption) (*GetSecretResponse, error) {
	out := new(GetSecretResponse)
	err := c.cc.Invoke(ctx, SecretService_GetSecret_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) ListSecrets(ctx context.Context, in *ListSecretsRequest, opts ...grpc.CallOption) (*ListSecretsResponse, error) {
	out := new(ListSecretsResponse)
	err := c.cc.Invoke(ctx, SecretService_ListSecrets_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) CreateSecret(ctx context.Context, in *CreateSecretRequest, opts ...grpc.CallOption) (*CreateSecretResponse, error) {
	out := new(CreateSecretResponse)
	err := c.cc.Invoke(ctx, SecretService_CreateSecret_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) UpdateSecret(ctx context.Context, in *UpdateSecretRequest, opts ...grpc.CallOption) (*UpdateSecretResponse, error) {
	out := new(UpdateSecretResponse)
	err := c.cc.Invoke(ctx, SecretService_UpdateSecret_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) DeleteSecret(ctx context.Context, in *DeleteSecretRequest, opts ...grpc.CallOption) (*DeleteSecretResponse, error) {
	out := new(DeleteSecretResponse)
	err := c.cc.Invoke(ctx, SecretService_DeleteSecret_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) GetConnectSecret(ctx context.Context, in *GetConnectSecretRequest, opts ...grpc.CallOption) (*GetConnectSecretResponse, error) {
	out := new(GetConnectSecretResponse)
	err := c.cc.Invoke(ctx, SecretService_GetConnectSecret_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) ListConnectSecrets(ctx context.Context, in *ListConnectSecretsRequest, opts ...grpc.CallOption) (*ListConnectSecretsResponse, error) {
	out := new(ListConnectSecretsResponse)
	err := c.cc.Invoke(ctx, SecretService_ListConnectSecrets_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) CreateConnectSecret(ctx context.Context, in *CreateConnectSecretRequest, opts ...grpc.CallOption) (*CreateConnectSecretResponse, error) {
	out := new(CreateConnectSecretResponse)
	err := c.cc.Invoke(ctx, SecretService_CreateConnectSecret_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) UpdateConnectSecret(ctx context.Context, in *UpdateConnectSecretRequest, opts ...grpc.CallOption) (*UpdateConnectSecretResponse, error) {
	out := new(UpdateConnectSecretResponse)
	err := c.cc.Invoke(ctx, SecretService_UpdateConnectSecret_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *secretServiceClient) DeleteConnectSecret(ctx context.Context, in *DeleteConnectSecretRequest, opts ...grpc.CallOption) (*DeleteConnectSecretResponse, error) {
	out := new(DeleteConnectSecretResponse)
	err := c.cc.Invoke(ctx, SecretService_DeleteConnectSecret_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SecretServiceServer is the server API for SecretService service.
// All implementations should embed UnimplementedSecretServiceServer
// for forward compatibility
type SecretServiceServer interface {
	// GetSecret retrieves the specific secret.
	GetSecret(context.Context, *GetSecretRequest) (*GetSecretResponse, error)
	// ListSecrets lists the secrets based on optional filter.
	ListSecrets(context.Context, *ListSecretsRequest) (*ListSecretsResponse, error)
	// CreateSecret creates the secret.
	CreateSecret(context.Context, *CreateSecretRequest) (*CreateSecretResponse, error)
	// UpdateSecret updates the secret.
	UpdateSecret(context.Context, *UpdateSecretRequest) (*UpdateSecretResponse, error)
	// DeleteSecret deletes the secret.
	DeleteSecret(context.Context, *DeleteSecretRequest) (*DeleteSecretResponse, error)
	// GetConnectSecret retrieves the specific secret for a specific Connect.
	GetConnectSecret(context.Context, *GetConnectSecretRequest) (*GetConnectSecretResponse, error)
	// ListConnectSecrets lists the Connect secrets based on optional filter.
	ListConnectSecrets(context.Context, *ListConnectSecretsRequest) (*ListConnectSecretsResponse, error)
	// CreateConnectSecret creates the secret for a Connect.
	CreateConnectSecret(context.Context, *CreateConnectSecretRequest) (*CreateConnectSecretResponse, error)
	// UpdateConnectSecret updates the Connect secret.
	UpdateConnectSecret(context.Context, *UpdateConnectSecretRequest) (*UpdateConnectSecretResponse, error)
	// DeleteSecret deletes the secret.
	DeleteConnectSecret(context.Context, *DeleteConnectSecretRequest) (*DeleteConnectSecretResponse, error)
}

// UnimplementedSecretServiceServer should be embedded to have forward compatible implementations.
type UnimplementedSecretServiceServer struct {
}

func (UnimplementedSecretServiceServer) GetSecret(context.Context, *GetSecretRequest) (*GetSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSecret not implemented")
}
func (UnimplementedSecretServiceServer) ListSecrets(context.Context, *ListSecretsRequest) (*ListSecretsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSecrets not implemented")
}
func (UnimplementedSecretServiceServer) CreateSecret(context.Context, *CreateSecretRequest) (*CreateSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateSecret not implemented")
}
func (UnimplementedSecretServiceServer) UpdateSecret(context.Context, *UpdateSecretRequest) (*UpdateSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateSecret not implemented")
}
func (UnimplementedSecretServiceServer) DeleteSecret(context.Context, *DeleteSecretRequest) (*DeleteSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSecret not implemented")
}
func (UnimplementedSecretServiceServer) GetConnectSecret(context.Context, *GetConnectSecretRequest) (*GetConnectSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetConnectSecret not implemented")
}
func (UnimplementedSecretServiceServer) ListConnectSecrets(context.Context, *ListConnectSecretsRequest) (*ListConnectSecretsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListConnectSecrets not implemented")
}
func (UnimplementedSecretServiceServer) CreateConnectSecret(context.Context, *CreateConnectSecretRequest) (*CreateConnectSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateConnectSecret not implemented")
}
func (UnimplementedSecretServiceServer) UpdateConnectSecret(context.Context, *UpdateConnectSecretRequest) (*UpdateConnectSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateConnectSecret not implemented")
}
func (UnimplementedSecretServiceServer) DeleteConnectSecret(context.Context, *DeleteConnectSecretRequest) (*DeleteConnectSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteConnectSecret not implemented")
}

// UnsafeSecretServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SecretServiceServer will
// result in compilation errors.
type UnsafeSecretServiceServer interface {
	mustEmbedUnimplementedSecretServiceServer()
}

func RegisterSecretServiceServer(s grpc.ServiceRegistrar, srv SecretServiceServer) {
	s.RegisterService(&SecretService_ServiceDesc, srv)
}

func _SecretService_GetSecret_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).GetSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_GetSecret_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).GetSecret(ctx, req.(*GetSecretRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_ListSecrets_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSecretsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).ListSecrets(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_ListSecrets_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).ListSecrets(ctx, req.(*ListSecretsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_CreateSecret_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).CreateSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_CreateSecret_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).CreateSecret(ctx, req.(*CreateSecretRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_UpdateSecret_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).UpdateSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_UpdateSecret_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).UpdateSecret(ctx, req.(*UpdateSecretRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_DeleteSecret_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).DeleteSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_DeleteSecret_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).DeleteSecret(ctx, req.(*DeleteSecretRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_GetConnectSecret_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetConnectSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).GetConnectSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_GetConnectSecret_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).GetConnectSecret(ctx, req.(*GetConnectSecretRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_ListConnectSecrets_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListConnectSecretsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).ListConnectSecrets(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_ListConnectSecrets_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).ListConnectSecrets(ctx, req.(*ListConnectSecretsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_CreateConnectSecret_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateConnectSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).CreateConnectSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_CreateConnectSecret_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).CreateConnectSecret(ctx, req.(*CreateConnectSecretRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_UpdateConnectSecret_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateConnectSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).UpdateConnectSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_UpdateConnectSecret_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).UpdateConnectSecret(ctx, req.(*UpdateConnectSecretRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecretService_DeleteConnectSecret_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteConnectSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecretServiceServer).DeleteConnectSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SecretService_DeleteConnectSecret_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecretServiceServer).DeleteConnectSecret(ctx, req.(*DeleteConnectSecretRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// SecretService_ServiceDesc is the grpc.ServiceDesc for SecretService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SecretService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "redpanda.api.dataplane.v1alpha1.SecretService",
	HandlerType: (*SecretServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetSecret",
			Handler:    _SecretService_GetSecret_Handler,
		},
		{
			MethodName: "ListSecrets",
			Handler:    _SecretService_ListSecrets_Handler,
		},
		{
			MethodName: "CreateSecret",
			Handler:    _SecretService_CreateSecret_Handler,
		},
		{
			MethodName: "UpdateSecret",
			Handler:    _SecretService_UpdateSecret_Handler,
		},
		{
			MethodName: "DeleteSecret",
			Handler:    _SecretService_DeleteSecret_Handler,
		},
		{
			MethodName: "GetConnectSecret",
			Handler:    _SecretService_GetConnectSecret_Handler,
		},
		{
			MethodName: "ListConnectSecrets",
			Handler:    _SecretService_ListConnectSecrets_Handler,
		},
		{
			MethodName: "CreateConnectSecret",
			Handler:    _SecretService_CreateConnectSecret_Handler,
		},
		{
			MethodName: "UpdateConnectSecret",
			Handler:    _SecretService_UpdateConnectSecret_Handler,
		},
		{
			MethodName: "DeleteConnectSecret",
			Handler:    _SecretService_DeleteConnectSecret_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "redpanda/api/dataplane/v1alpha1/secret.proto",
}
