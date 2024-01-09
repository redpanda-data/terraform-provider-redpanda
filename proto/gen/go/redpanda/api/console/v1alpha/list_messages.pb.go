// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.32.0
// 	protoc        (unknown)
// source: redpanda/api/console/v1alpha/list_messages.proto

package consolev1alpha

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
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

// ListMessagesRequest is the request for ListMessages call.
type ListMessagesRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Topic string `protobuf:"bytes,1,opt,name=topic,proto3" json:"topic,omitempty"` // Topic name.
}

func (x *ListMessagesRequest) Reset() {
	*x = ListMessagesRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_redpanda_api_console_v1alpha_list_messages_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListMessagesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListMessagesRequest) ProtoMessage() {}

func (x *ListMessagesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_redpanda_api_console_v1alpha_list_messages_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListMessagesRequest.ProtoReflect.Descriptor instead.
func (*ListMessagesRequest) Descriptor() ([]byte, []int) {
	return file_redpanda_api_console_v1alpha_list_messages_proto_rawDescGZIP(), []int{0}
}

func (x *ListMessagesRequest) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

// ListMessagesResponse is the response for ListMessages call.
type ListMessagesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ListMessagesResponse) Reset() {
	*x = ListMessagesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_redpanda_api_console_v1alpha_list_messages_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListMessagesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListMessagesResponse) ProtoMessage() {}

func (x *ListMessagesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_redpanda_api_console_v1alpha_list_messages_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListMessagesResponse.ProtoReflect.Descriptor instead.
func (*ListMessagesResponse) Descriptor() ([]byte, []int) {
	return file_redpanda_api_console_v1alpha_list_messages_proto_rawDescGZIP(), []int{1}
}

var File_redpanda_api_console_v1alpha_list_messages_proto protoreflect.FileDescriptor

var file_redpanda_api_console_v1alpha_list_messages_proto_rawDesc = []byte{
	0x0a, 0x30, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x63,
	0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x2f, 0x6c,
	0x69, 0x73, 0x74, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x1c, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x1a, 0x1b, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2f, 0x76,
	0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x37, 0x0a,
	0x13, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x20, 0x0a, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x42, 0x0a, 0xba, 0x48, 0x07, 0x72, 0x05, 0x10, 0x01, 0x18, 0x80, 0x01, 0x52,
	0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x22, 0x16, 0x0a, 0x14, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0x8b,
	0x01, 0x0a, 0x0e, 0x43, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x12, 0x79, 0x0a, 0x0c, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x73, 0x12, 0x31, 0x2e, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x2e, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x32, 0x2e, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e,
	0x61, 0x70, 0x69, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x2e, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x30, 0x01, 0x42, 0xb7, 0x02, 0x0a,
	0x20, 0x63, 0x6f, 0x6d, 0x2e, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x61, 0x70,
	0x69, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68,
	0x61, 0x42, 0x11, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x50,
	0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x6d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2d, 0x64, 0x61, 0x74, 0x61,
	0x2f, 0x74, 0x65, 0x72, 0x72, 0x61, 0x66, 0x6f, 0x72, 0x6d, 0x2d, 0x70, 0x72, 0x6f, 0x76, 0x69,
	0x64, 0x65, 0x72, 0x2d, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x67, 0x6f, 0x2f, 0x72, 0x65, 0x64, 0x70, 0x61, 0x6e,
	0x64, 0x61, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x2f, 0x76,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x3b, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x76, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0xa2, 0x02, 0x03, 0x52, 0x41, 0x43, 0xaa, 0x02, 0x1c, 0x52, 0x65,
	0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x2e, 0x41, 0x70, 0x69, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x6f,
	0x6c, 0x65, 0x2e, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0xca, 0x02, 0x1c, 0x52, 0x65, 0x64,
	0x70, 0x61, 0x6e, 0x64, 0x61, 0x5c, 0x41, 0x70, 0x69, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x6f, 0x6c,
	0x65, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0xe2, 0x02, 0x28, 0x52, 0x65, 0x64, 0x70,
	0x61, 0x6e, 0x64, 0x61, 0x5c, 0x41, 0x70, 0x69, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65,
	0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61,
	0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x1f, 0x52, 0x65, 0x64, 0x70, 0x61, 0x6e, 0x64, 0x61, 0x3a,
	0x3a, 0x41, 0x70, 0x69, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x3a, 0x3a, 0x56,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_redpanda_api_console_v1alpha_list_messages_proto_rawDescOnce sync.Once
	file_redpanda_api_console_v1alpha_list_messages_proto_rawDescData = file_redpanda_api_console_v1alpha_list_messages_proto_rawDesc
)

func file_redpanda_api_console_v1alpha_list_messages_proto_rawDescGZIP() []byte {
	file_redpanda_api_console_v1alpha_list_messages_proto_rawDescOnce.Do(func() {
		file_redpanda_api_console_v1alpha_list_messages_proto_rawDescData = protoimpl.X.CompressGZIP(file_redpanda_api_console_v1alpha_list_messages_proto_rawDescData)
	})
	return file_redpanda_api_console_v1alpha_list_messages_proto_rawDescData
}

var file_redpanda_api_console_v1alpha_list_messages_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_redpanda_api_console_v1alpha_list_messages_proto_goTypes = []interface{}{
	(*ListMessagesRequest)(nil),  // 0: redpanda.api.console.v1alpha.ListMessagesRequest
	(*ListMessagesResponse)(nil), // 1: redpanda.api.console.v1alpha.ListMessagesResponse
}
var file_redpanda_api_console_v1alpha_list_messages_proto_depIdxs = []int32{
	0, // 0: redpanda.api.console.v1alpha.ConsoleService.ListMessages:input_type -> redpanda.api.console.v1alpha.ListMessagesRequest
	1, // 1: redpanda.api.console.v1alpha.ConsoleService.ListMessages:output_type -> redpanda.api.console.v1alpha.ListMessagesResponse
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_redpanda_api_console_v1alpha_list_messages_proto_init() }
func file_redpanda_api_console_v1alpha_list_messages_proto_init() {
	if File_redpanda_api_console_v1alpha_list_messages_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_redpanda_api_console_v1alpha_list_messages_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListMessagesRequest); i {
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
		file_redpanda_api_console_v1alpha_list_messages_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListMessagesResponse); i {
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
			RawDescriptor: file_redpanda_api_console_v1alpha_list_messages_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_redpanda_api_console_v1alpha_list_messages_proto_goTypes,
		DependencyIndexes: file_redpanda_api_console_v1alpha_list_messages_proto_depIdxs,
		MessageInfos:      file_redpanda_api_console_v1alpha_list_messages_proto_msgTypes,
	}.Build()
	File_redpanda_api_console_v1alpha_list_messages_proto = out.File
	file_redpanda_api_console_v1alpha_list_messages_proto_rawDesc = nil
	file_redpanda_api_console_v1alpha_list_messages_proto_goTypes = nil
	file_redpanda_api_console_v1alpha_list_messages_proto_depIdxs = nil
}