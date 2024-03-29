// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: redpanda/api/billing/v1alpha1/usage_type.proto

package billingv1alpha1

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
	UsageTypeService_ListUsageTypes_FullMethodName = "/redpanda.api.billing.v1alpha1.UsageTypeService/ListUsageTypes"
)

// UsageTypeServiceClient is the client API for UsageTypeService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type UsageTypeServiceClient interface {
	ListUsageTypes(ctx context.Context, in *ListUsageTypesRequest, opts ...grpc.CallOption) (*ListUsageTypesResponse, error)
}

type usageTypeServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewUsageTypeServiceClient(cc grpc.ClientConnInterface) UsageTypeServiceClient {
	return &usageTypeServiceClient{cc}
}

func (c *usageTypeServiceClient) ListUsageTypes(ctx context.Context, in *ListUsageTypesRequest, opts ...grpc.CallOption) (*ListUsageTypesResponse, error) {
	out := new(ListUsageTypesResponse)
	err := c.cc.Invoke(ctx, UsageTypeService_ListUsageTypes_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UsageTypeServiceServer is the server API for UsageTypeService service.
// All implementations should embed UnimplementedUsageTypeServiceServer
// for forward compatibility
type UsageTypeServiceServer interface {
	ListUsageTypes(context.Context, *ListUsageTypesRequest) (*ListUsageTypesResponse, error)
}

// UnimplementedUsageTypeServiceServer should be embedded to have forward compatible implementations.
type UnimplementedUsageTypeServiceServer struct {
}

func (UnimplementedUsageTypeServiceServer) ListUsageTypes(context.Context, *ListUsageTypesRequest) (*ListUsageTypesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListUsageTypes not implemented")
}

// UnsafeUsageTypeServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to UsageTypeServiceServer will
// result in compilation errors.
type UnsafeUsageTypeServiceServer interface {
	mustEmbedUnimplementedUsageTypeServiceServer()
}

func RegisterUsageTypeServiceServer(s grpc.ServiceRegistrar, srv UsageTypeServiceServer) {
	s.RegisterService(&UsageTypeService_ServiceDesc, srv)
}

func _UsageTypeService_ListUsageTypes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListUsageTypesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UsageTypeServiceServer).ListUsageTypes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UsageTypeService_ListUsageTypes_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UsageTypeServiceServer).ListUsageTypes(ctx, req.(*ListUsageTypesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// UsageTypeService_ServiceDesc is the grpc.ServiceDesc for UsageTypeService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var UsageTypeService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "redpanda.api.billing.v1alpha1.UsageTypeService",
	HandlerType: (*UsageTypeServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListUsageTypes",
			Handler:    _UsageTypeService_ListUsageTypes_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "redpanda/api/billing/v1alpha1/usage_type.proto",
}
