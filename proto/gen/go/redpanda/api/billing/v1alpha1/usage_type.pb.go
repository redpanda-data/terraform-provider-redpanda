// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.32.0
// 	protoc        (unknown)
// source: redpanda/api/billing/v1alpha1/usage_type.proto

package billingv1alpha1

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	_ "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/common/v1alpha1"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ProductType int32

const (
	ProductType_PRODUCT_TYPE_UNSPECIFIED ProductType = 0
	ProductType_PRODUCT_TYPE_DEDICATED   ProductType = 1
	ProductType_PRODUCT_TYPE_BYOC        ProductType = 2
	ProductType_PRODUCT_TYPE_SERVERLESS  ProductType = 3
)

// Enum value maps for ProductType.
var (
	ProductType_name = map[int32]string{
		0: "PRODUCT_TYPE_UNSPECIFIED",
		1: "PRODUCT_TYPE_DEDICATED",
		2: "PRODUCT_TYPE_BYOC",
		3: "PRODUCT_TYPE_SERVERLESS",
	}
	ProductType_value = map[string]int32{
		"PRODUCT_TYPE_UNSPECIFIED": 0,
		"PRODUCT_TYPE_DEDICATED":   1,
		"PRODUCT_TYPE_BYOC":        2,
		"PRODUCT_TYPE_SERVERLESS":  3,
	}
)

func (x ProductType) Enum() *ProductType {
	p := new(ProductType)
	*p = x
	return p
}

func (x ProductType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ProductType) Descriptor() protoreflect.EnumDescriptor {
	return file_redpanda_api_billing_v1alpha1_usage_type_proto_enumTypes[0].Descriptor()
}

func (ProductType) Type() protoreflect.EnumType {
	return &file_redpanda_api_billing_v1alpha1_usage_type_proto_enumTypes[0]
}

func (x ProductType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ProductType.Descriptor instead.
func (ProductType) EnumDescriptor() ([]byte, []int) {
	return file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescGZIP(), []int{0}
}

type ListUsageTypesRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PageSize  int32  `protobuf:"varint,1,opt,name=page_size,json=pageSize,proto3" json:"page_size,omitempty"`
	PageToken string `protobuf:"bytes,2,opt,name=page_token,json=pageToken,proto3" json:"page_token,omitempty"`
}

func (x *ListUsageTypesRequest) Reset() {
	*x = ListUsageTypesRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListUsageTypesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListUsageTypesRequest) ProtoMessage() {}

func (x *ListUsageTypesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListUsageTypesRequest.ProtoReflect.Descriptor instead.
func (*ListUsageTypesRequest) Descriptor() ([]byte, []int) {
	return file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescGZIP(), []int{0}
}

func (x *ListUsageTypesRequest) GetPageSize() int32 {
	if x != nil {
		return x.PageSize
	}
	return 0
}

func (x *ListUsageTypesRequest) GetPageToken() string {
	if x != nil {
		return x.PageToken
	}
	return ""
}

type ListUsageTypesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	UsageTypes    []*UsageType `protobuf:"bytes,1,rep,name=usage_types,json=usageTypes,proto3" json:"usage_types,omitempty"`
	NextPageToken string       `protobuf:"bytes,2,opt,name=next_page_token,json=nextPageToken,proto3" json:"next_page_token,omitempty"`
}

func (x *ListUsageTypesResponse) Reset() {
	*x = ListUsageTypesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListUsageTypesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListUsageTypesResponse) ProtoMessage() {}

func (x *ListUsageTypesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListUsageTypesResponse.ProtoReflect.Descriptor instead.
func (*ListUsageTypesResponse) Descriptor() ([]byte, []int) {
	return file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescGZIP(), []int{1}
}

func (x *ListUsageTypesResponse) GetUsageTypes() []*UsageType {
	if x != nil {
		return x.UsageTypes
	}
	return nil
}

func (x *ListUsageTypesResponse) GetNextPageToken() string {
	if x != nil {
		return x.NextPageToken
	}
	return ""
}

type UsageType struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id          string      `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Name        string      `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Group       string      `protobuf:"bytes,3,opt,name=group,proto3" json:"group,omitempty"`
	ProductType ProductType `protobuf:"varint,4,opt,name=product_type,json=productType,proto3,enum=redpanda.api.billing.v1alpha1.ProductType" json:"product_type,omitempty"`
}

func (x *UsageType) Reset() {
	*x = UsageType{}
	if protoimpl.UnsafeEnabled {
		mi := &file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UsageType) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UsageType) ProtoMessage() {}

func (x *UsageType) ProtoReflect() protoreflect.Message {
	mi := &file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UsageType.ProtoReflect.Descriptor instead.
func (*UsageType) Descriptor() ([]byte, []int) {
	return file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescGZIP(), []int{2}
}

func (x *UsageType) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *UsageType) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *UsageType) GetGroup() string {
	if x != nil {
		return x.Group
	}
	return ""
}

func (x *UsageType) GetProductType() ProductType {
	if x != nil {
		return x.ProductType
	}
	return ProductType_PRODUCT_TYPE_UNSPECIFIED
}

var File_redpanda_api_billing_v1alpha1_usage_type_proto protoreflect.FileDescriptor

var file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDesc = []byte{
	0x0a, 0x2e, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x62,
	0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f,
	0x75, 0x73, 0x61, 0x67, 0x65, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x1d, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x62,
	0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x1a,
	0x1b, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x61,
	0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x2d, 0x67, 0x65, 0x6e, 0x2d, 0x6f, 0x70, 0x65, 0x6e, 0x61, 0x70, 0x69, 0x76, 0x32,
	0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x2a, 0x72, 0x65, 0x64, 0x70,
	0x61, 0x6e, 0x64, 0x61, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2f,
	0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x5e, 0x0a, 0x15, 0x4c, 0x69, 0x73, 0x74, 0x55, 0x73,
	0x61, 0x67, 0x65, 0x54, 0x79, 0x70, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x26, 0x0a, 0x09, 0x70, 0x61, 0x67, 0x65, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x05, 0x42, 0x09, 0xba, 0x48, 0x06, 0x1a, 0x04, 0x18, 0x64, 0x28, 0x00, 0x52, 0x08, 0x70,
	0x61, 0x67, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x70, 0x61, 0x67, 0x65, 0x5f,
	0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x70, 0x61, 0x67,
	0x65, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x8b, 0x01, 0x0a, 0x16, 0x4c, 0x69, 0x73, 0x74, 0x55,
	0x73, 0x61, 0x67, 0x65, 0x54, 0x79, 0x70, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x49, 0x0a, 0x0b, 0x75, 0x73, 0x61, 0x67, 0x65, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x73,
	0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x28, 0x2e, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64,
	0x61, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x55, 0x73, 0x61, 0x67, 0x65, 0x54, 0x79, 0x70, 0x65,
	0x52, 0x0a, 0x75, 0x73, 0x61, 0x67, 0x65, 0x54, 0x79, 0x70, 0x65, 0x73, 0x12, 0x26, 0x0a, 0x0f,
	0x6e, 0x65, 0x78, 0x74, 0x5f, 0x70, 0x61, 0x67, 0x65, 0x5f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x6e, 0x65, 0x78, 0x74, 0x50, 0x61, 0x67, 0x65, 0x54,
	0x6f, 0x6b, 0x65, 0x6e, 0x22, 0xb9, 0x01, 0x0a, 0x09, 0x55, 0x73, 0x61, 0x67, 0x65, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x1b, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0b,
	0xba, 0x48, 0x08, 0xc8, 0x01, 0x01, 0x72, 0x03, 0xb0, 0x01, 0x01, 0x52, 0x02, 0x69, 0x64, 0x12,
	0x1a, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x06, 0xba,
	0x48, 0x03, 0xc8, 0x01, 0x01, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x05, 0x67,
	0x72, 0x6f, 0x75, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x42, 0x06, 0xba, 0x48, 0x03, 0xc8,
	0x01, 0x01, 0x52, 0x05, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x12, 0x55, 0x0a, 0x0c, 0x70, 0x72, 0x6f,
	0x64, 0x75, 0x63, 0x74, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x2a, 0x2e, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x62,
	0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e,
	0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x54, 0x79, 0x70, 0x65, 0x42, 0x06, 0xba, 0x48, 0x03,
	0xc8, 0x01, 0x01, 0x52, 0x0b, 0x70, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x54, 0x79, 0x70, 0x65,
	0x2a, 0x7b, 0x0a, 0x0b, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x54, 0x79, 0x70, 0x65, 0x12,
	0x1c, 0x0a, 0x18, 0x50, 0x52, 0x4f, 0x44, 0x55, 0x43, 0x54, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f,
	0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x1a, 0x0a,
	0x16, 0x50, 0x52, 0x4f, 0x44, 0x55, 0x43, 0x54, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x44, 0x45,
	0x44, 0x49, 0x43, 0x41, 0x54, 0x45, 0x44, 0x10, 0x01, 0x12, 0x15, 0x0a, 0x11, 0x50, 0x52, 0x4f,
	0x44, 0x55, 0x43, 0x54, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x42, 0x59, 0x4f, 0x43, 0x10, 0x02,
	0x12, 0x1b, 0x0a, 0x17, 0x50, 0x52, 0x4f, 0x44, 0x55, 0x43, 0x54, 0x5f, 0x54, 0x59, 0x50, 0x45,
	0x5f, 0x53, 0x45, 0x52, 0x56, 0x45, 0x52, 0x4c, 0x45, 0x53, 0x53, 0x10, 0x03, 0x32, 0x95, 0x03,
	0x0a, 0x10, 0x55, 0x73, 0x61, 0x67, 0x65, 0x54, 0x79, 0x70, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x80, 0x03, 0x0a, 0x0e, 0x4c, 0x69, 0x73, 0x74, 0x55, 0x73, 0x61, 0x67, 0x65,
	0x54, 0x79, 0x70, 0x65, 0x73, 0x12, 0x34, 0x2e, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61,
	0x2e, 0x61, 0x70, 0x69, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x55, 0x73, 0x61, 0x67, 0x65, 0x54,
	0x79, 0x70, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x35, 0x2e, 0x72, 0x65,
	0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69,
	0x6e, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74,
	0x55, 0x73, 0x61, 0x67, 0x65, 0x54, 0x79, 0x70, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x80, 0x02, 0x92, 0x41, 0xc3, 0x01, 0x12, 0x10, 0x4c, 0x69, 0x73, 0x74, 0x20,
	0x75, 0x73, 0x61, 0x67, 0x65, 0x20, 0x74, 0x79, 0x70, 0x65, 0x73, 0x1a, 0x11, 0x4c, 0x69, 0x73,
	0x74, 0x20, 0x75, 0x73, 0x61, 0x67, 0x65, 0x20, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x4a, 0x46,
	0x0a, 0x03, 0x32, 0x30, 0x30, 0x12, 0x3f, 0x0a, 0x02, 0x4f, 0x4b, 0x12, 0x39, 0x0a, 0x37, 0x1a,
	0x35, 0x2e, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x62,
	0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e,
	0x4c, 0x69, 0x73, 0x74, 0x55, 0x73, 0x61, 0x67, 0x65, 0x54, 0x79, 0x70, 0x65, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x4a, 0x54, 0x0a, 0x03, 0x35, 0x30, 0x30, 0x12, 0x4d, 0x0a,
	0x33, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x20, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72,
	0x20, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x2e, 0x20, 0x50, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x20, 0x72,
	0x65, 0x61, 0x63, 0x68, 0x20, 0x6f, 0x75, 0x74, 0x20, 0x74, 0x6f, 0x20, 0x73, 0x75, 0x70, 0x70,
	0x6f, 0x72, 0x74, 0x2e, 0x12, 0x16, 0x0a, 0x14, 0x1a, 0x12, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0xb2, 0xbf, 0x07, 0x18,
	0x6d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x3a, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69, 0x7a, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x2d, 0x69, 0x6e, 0x66, 0x6f, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x17, 0x12, 0x15,
	0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x75, 0x73, 0x61, 0x67, 0x65, 0x2d,
	0x74, 0x79, 0x70, 0x65, 0x73, 0x42, 0xbb, 0x02, 0x0a, 0x21, 0x63, 0x6f, 0x6d, 0x2e, 0x72, 0x65,
	0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69,
	0x6e, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x42, 0x0e, 0x55, 0x73, 0x61,
	0x67, 0x65, 0x54, 0x79, 0x70, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x6f, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e,
	0x64, 0x61, 0x2d, 0x64, 0x61, 0x74, 0x61, 0x2f, 0x74, 0x65, 0x72, 0x72, 0x61, 0x66, 0x6f, 0x72,
	0x6d, 0x2d, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x2d, 0x72, 0x65, 0x64, 0x70, 0x61,
	0x6e, 0x64, 0x61, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x67, 0x6f,
	0x2f, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x62, 0x69,
	0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x3b, 0x62,
	0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xa2, 0x02,
	0x03, 0x52, 0x41, 0x42, 0xaa, 0x02, 0x1d, 0x52, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e,
	0x41, 0x70, 0x69, 0x2e, 0x42, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x56, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0xca, 0x02, 0x1d, 0x52, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x5c,
	0x41, 0x70, 0x69, 0x5c, 0x42, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x5c, 0x56, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0xe2, 0x02, 0x29, 0x52, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x5c,
	0x41, 0x70, 0x69, 0x5c, 0x42, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x5c, 0x56, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61,
	0xea, 0x02, 0x20, 0x52, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x3a, 0x3a, 0x41, 0x70, 0x69,
	0x3a, 0x3a, 0x42, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x3a, 0x3a, 0x56, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescOnce sync.Once
	file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescData = file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDesc
)

func file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescGZIP() []byte {
	file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescOnce.Do(func() {
		file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescData = protoimpl.X.CompressGZIP(file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescData)
	})
	return file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDescData
}

var file_redpanda_api_billing_v1alpha1_usage_type_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_redpanda_api_billing_v1alpha1_usage_type_proto_goTypes = []interface{}{
	(ProductType)(0),               // 0: redpanda.api.billing.v1alpha1.ProductType
	(*ListUsageTypesRequest)(nil),  // 1: redpanda.api.billing.v1alpha1.ListUsageTypesRequest
	(*ListUsageTypesResponse)(nil), // 2: redpanda.api.billing.v1alpha1.ListUsageTypesResponse
	(*UsageType)(nil),              // 3: redpanda.api.billing.v1alpha1.UsageType
}
var file_redpanda_api_billing_v1alpha1_usage_type_proto_depIdxs = []int32{
	3, // 0: redpanda.api.billing.v1alpha1.ListUsageTypesResponse.usage_types:type_name -> redpanda.api.billing.v1alpha1.UsageType
	0, // 1: redpanda.api.billing.v1alpha1.UsageType.product_type:type_name -> redpanda.api.billing.v1alpha1.ProductType
	1, // 2: redpanda.api.billing.v1alpha1.UsageTypeService.ListUsageTypes:input_type -> redpanda.api.billing.v1alpha1.ListUsageTypesRequest
	2, // 3: redpanda.api.billing.v1alpha1.UsageTypeService.ListUsageTypes:output_type -> redpanda.api.billing.v1alpha1.ListUsageTypesResponse
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_redpanda_api_billing_v1alpha1_usage_type_proto_init() }
func file_redpanda_api_billing_v1alpha1_usage_type_proto_init() {
	if File_redpanda_api_billing_v1alpha1_usage_type_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListUsageTypesRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListUsageTypesResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UsageType); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_redpanda_api_billing_v1alpha1_usage_type_proto_goTypes,
		DependencyIndexes: file_redpanda_api_billing_v1alpha1_usage_type_proto_depIdxs,
		EnumInfos:         file_redpanda_api_billing_v1alpha1_usage_type_proto_enumTypes,
		MessageInfos:      file_redpanda_api_billing_v1alpha1_usage_type_proto_msgTypes,
	}.Build()
	File_redpanda_api_billing_v1alpha1_usage_type_proto = out.File
	file_redpanda_api_billing_v1alpha1_usage_type_proto_rawDesc = nil
	file_redpanda_api_billing_v1alpha1_usage_type_proto_goTypes = nil
	file_redpanda_api_billing_v1alpha1_usage_type_proto_depIdxs = nil
}