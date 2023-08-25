// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v4.23.4
// source: proto/github/ayonli/UserService.proto

package ayonli

import (
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

type UserQuery struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id    *string `protobuf:"bytes,1,opt,name=id,proto3,oneof" json:"id,omitempty"`
	Email *string `protobuf:"bytes,2,opt,name=email,proto3,oneof" json:"email,omitempty"`
}

func (x *UserQuery) Reset() {
	*x = UserQuery{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_github_ayonli_UserService_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserQuery) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserQuery) ProtoMessage() {}

func (x *UserQuery) ProtoReflect() protoreflect.Message {
	mi := &file_proto_github_ayonli_UserService_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserQuery.ProtoReflect.Descriptor instead.
func (*UserQuery) Descriptor() ([]byte, []int) {
	return file_proto_github_ayonli_UserService_proto_rawDescGZIP(), []int{0}
}

func (x *UserQuery) GetId() string {
	if x != nil && x.Id != nil {
		return *x.Id
	}
	return ""
}

func (x *UserQuery) GetEmail() string {
	if x != nil && x.Email != nil {
		return *x.Email
	}
	return ""
}

type PostQueryResult struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Posts []*Post `protobuf:"bytes,1,rep,name=posts,proto3" json:"posts,omitempty"`
}

func (x *PostQueryResult) Reset() {
	*x = PostQueryResult{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_github_ayonli_UserService_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PostQueryResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PostQueryResult) ProtoMessage() {}

func (x *PostQueryResult) ProtoReflect() protoreflect.Message {
	mi := &file_proto_github_ayonli_UserService_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PostQueryResult.ProtoReflect.Descriptor instead.
func (*PostQueryResult) Descriptor() ([]byte, []int) {
	return file_proto_github_ayonli_UserService_proto_rawDescGZIP(), []int{1}
}

func (x *PostQueryResult) GetPosts() []*Post {
	if x != nil {
		return x.Posts
	}
	return nil
}

var File_proto_github_ayonli_UserService_proto protoreflect.FileDescriptor

var file_proto_github_ayonli_UserService_proto_rawDesc = []byte{
	0x0a, 0x25, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2f, 0x61,
	0x79, 0x6f, 0x6e, 0x6c, 0x69, 0x2f, 0x55, 0x73, 0x65, 0x72, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x16, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x73, 0x2e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x61, 0x79, 0x6f, 0x6e, 0x6c, 0x69, 0x1a,
	0x20, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2f, 0x61, 0x79,
	0x6f, 0x6e, 0x6c, 0x69, 0x2f, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x4c, 0x0a, 0x09, 0x55, 0x73, 0x65, 0x72, 0x51, 0x75, 0x65, 0x72, 0x79, 0x12, 0x13,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x02, 0x69, 0x64,
	0x88, 0x01, 0x01, 0x12, 0x19, 0x0a, 0x05, 0x65, 0x6d, 0x61, 0x69, 0x6c, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x48, 0x01, 0x52, 0x05, 0x65, 0x6d, 0x61, 0x69, 0x6c, 0x88, 0x01, 0x01, 0x42, 0x05,
	0x0a, 0x03, 0x5f, 0x69, 0x64, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x65, 0x6d, 0x61, 0x69, 0x6c, 0x22,
	0x45, 0x0a, 0x0f, 0x50, 0x6f, 0x73, 0x74, 0x51, 0x75, 0x65, 0x72, 0x79, 0x52, 0x65, 0x73, 0x75,
	0x6c, 0x74, 0x12, 0x32, 0x0a, 0x05, 0x70, 0x6f, 0x73, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x1c, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x61, 0x79, 0x6f, 0x6e, 0x6c, 0x69, 0x2e, 0x50, 0x6f, 0x73, 0x74, 0x52,
	0x05, 0x70, 0x6f, 0x73, 0x74, 0x73, 0x32, 0xb7, 0x01, 0x0a, 0x0b, 0x55, 0x73, 0x65, 0x72, 0x53,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x4c, 0x0a, 0x07, 0x47, 0x65, 0x74, 0x55, 0x73, 0x65,
	0x72, 0x12, 0x21, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x61, 0x79, 0x6f, 0x6e, 0x6c, 0x69, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x51,
	0x75, 0x65, 0x72, 0x79, 0x1a, 0x1c, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x61, 0x79, 0x6f, 0x6e, 0x6c, 0x69, 0x2e, 0x55, 0x73,
	0x65, 0x72, 0x22, 0x00, 0x12, 0x5a, 0x0a, 0x0a, 0x47, 0x65, 0x74, 0x4d, 0x79, 0x50, 0x6f, 0x73,
	0x74, 0x73, 0x12, 0x21, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x61, 0x79, 0x6f, 0x6e, 0x6c, 0x69, 0x2e, 0x55, 0x73, 0x65, 0x72,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x1a, 0x27, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73,
	0x2e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x61, 0x79, 0x6f, 0x6e, 0x6c, 0x69, 0x2e, 0x50,
	0x6f, 0x73, 0x74, 0x51, 0x75, 0x65, 0x72, 0x79, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x22, 0x00,
	0x42, 0x0f, 0x5a, 0x0d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2f, 0x61, 0x79, 0x6f, 0x6e, 0x6c,
	0x69, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_github_ayonli_UserService_proto_rawDescOnce sync.Once
	file_proto_github_ayonli_UserService_proto_rawDescData = file_proto_github_ayonli_UserService_proto_rawDesc
)

func file_proto_github_ayonli_UserService_proto_rawDescGZIP() []byte {
	file_proto_github_ayonli_UserService_proto_rawDescOnce.Do(func() {
		file_proto_github_ayonli_UserService_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_github_ayonli_UserService_proto_rawDescData)
	})
	return file_proto_github_ayonli_UserService_proto_rawDescData
}

var file_proto_github_ayonli_UserService_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto_github_ayonli_UserService_proto_goTypes = []interface{}{
	(*UserQuery)(nil),       // 0: services.github.ayonli.UserQuery
	(*PostQueryResult)(nil), // 1: services.github.ayonli.PostQueryResult
	(*Post)(nil),            // 2: services.github.ayonli.Post
	(*User)(nil),            // 3: services.github.ayonli.User
}
var file_proto_github_ayonli_UserService_proto_depIdxs = []int32{
	2, // 0: services.github.ayonli.PostQueryResult.posts:type_name -> services.github.ayonli.Post
	0, // 1: services.github.ayonli.UserService.GetUser:input_type -> services.github.ayonli.UserQuery
	0, // 2: services.github.ayonli.UserService.GetMyPosts:input_type -> services.github.ayonli.UserQuery
	3, // 3: services.github.ayonli.UserService.GetUser:output_type -> services.github.ayonli.User
	1, // 4: services.github.ayonli.UserService.GetMyPosts:output_type -> services.github.ayonli.PostQueryResult
	3, // [3:5] is the sub-list for method output_type
	1, // [1:3] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_proto_github_ayonli_UserService_proto_init() }
func file_proto_github_ayonli_UserService_proto_init() {
	if File_proto_github_ayonli_UserService_proto != nil {
		return
	}
	file_proto_github_ayonli_struct_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_proto_github_ayonli_UserService_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserQuery); i {
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
		file_proto_github_ayonli_UserService_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PostQueryResult); i {
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
	file_proto_github_ayonli_UserService_proto_msgTypes[0].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_github_ayonli_UserService_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_github_ayonli_UserService_proto_goTypes,
		DependencyIndexes: file_proto_github_ayonli_UserService_proto_depIdxs,
		MessageInfos:      file_proto_github_ayonli_UserService_proto_msgTypes,
	}.Build()
	File_proto_github_ayonli_UserService_proto = out.File
	file_proto_github_ayonli_UserService_proto_rawDesc = nil
	file_proto_github_ayonli_UserService_proto_goTypes = nil
	file_proto_github_ayonli_UserService_proto_depIdxs = nil
}
