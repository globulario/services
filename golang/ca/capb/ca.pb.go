//*
// Admin functionality.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.24.0
// 	protoc        v3.13.0
// source: proto/ca.proto

package capb

import (
	context "context"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

// Take a certificate signing request
type SignCertificateRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Csr string `protobuf:"bytes,1,opt,name=csr,proto3" json:"csr,omitempty"` // Certificate request.
}

func (x *SignCertificateRequest) Reset() {
	*x = SignCertificateRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ca_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignCertificateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignCertificateRequest) ProtoMessage() {}

func (x *SignCertificateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ca_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignCertificateRequest.ProtoReflect.Descriptor instead.
func (*SignCertificateRequest) Descriptor() ([]byte, []int) {
	return file_proto_ca_proto_rawDescGZIP(), []int{0}
}

func (x *SignCertificateRequest) GetCsr() string {
	if x != nil {
		return x.Csr
	}
	return ""
}

// Return a signed certificate.
type SignCertificateResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Crt string `protobuf:"bytes,1,opt,name=crt,proto3" json:"crt,omitempty"`
}

func (x *SignCertificateResponse) Reset() {
	*x = SignCertificateResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ca_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignCertificateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignCertificateResponse) ProtoMessage() {}

func (x *SignCertificateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ca_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignCertificateResponse.ProtoReflect.Descriptor instead.
func (*SignCertificateResponse) Descriptor() ([]byte, []int) {
	return file_proto_ca_proto_rawDescGZIP(), []int{1}
}

func (x *SignCertificateResponse) GetCrt() string {
	if x != nil {
		return x.Crt
	}
	return ""
}

// Return the authority thrust certificate
type GetCaCertificateRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetCaCertificateRequest) Reset() {
	*x = GetCaCertificateRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ca_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetCaCertificateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetCaCertificateRequest) ProtoMessage() {}

func (x *GetCaCertificateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ca_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetCaCertificateRequest.ProtoReflect.Descriptor instead.
func (*GetCaCertificateRequest) Descriptor() ([]byte, []int) {
	return file_proto_ca_proto_rawDescGZIP(), []int{2}
}

type GetCaCertificateResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ca string `protobuf:"bytes,1,opt,name=ca,proto3" json:"ca,omitempty"`
}

func (x *GetCaCertificateResponse) Reset() {
	*x = GetCaCertificateResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ca_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetCaCertificateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetCaCertificateResponse) ProtoMessage() {}

func (x *GetCaCertificateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ca_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetCaCertificateResponse.ProtoReflect.Descriptor instead.
func (*GetCaCertificateResponse) Descriptor() ([]byte, []int) {
	return file_proto_ca_proto_rawDescGZIP(), []int{3}
}

func (x *GetCaCertificateResponse) GetCa() string {
	if x != nil {
		return x.Ca
	}
	return ""
}

var File_proto_ca_proto protoreflect.FileDescriptor

var file_proto_ca_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x02, 0x63, 0x61, 0x22, 0x2a, 0x0a, 0x16, 0x53, 0x69, 0x67, 0x6e, 0x43, 0x65, 0x72, 0x74,
	0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10,
	0x0a, 0x03, 0x63, 0x73, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x63, 0x73, 0x72,
	0x22, 0x2b, 0x0a, 0x17, 0x53, 0x69, 0x67, 0x6e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63,
	0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x63,
	0x72, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x63, 0x72, 0x74, 0x22, 0x19, 0x0a,
	0x17, 0x47, 0x65, 0x74, 0x43, 0x61, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74,
	0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x2a, 0x0a, 0x18, 0x47, 0x65, 0x74, 0x43,
	0x61, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x63, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x02, 0x63, 0x61, 0x32, 0xb1, 0x01, 0x0a, 0x14, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69,
	0x63, 0x61, 0x74, 0x65, 0x41, 0x75, 0x74, 0x68, 0x6f, 0x72, 0x69, 0x74, 0x79, 0x12, 0x4a, 0x0a,
	0x0f, 0x53, 0x69, 0x67, 0x6e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65,
	0x12, 0x1a, 0x2e, 0x63, 0x61, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66,
	0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x63,
	0x61, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74,
	0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4d, 0x0a, 0x10, 0x47, 0x65, 0x74,
	0x43, 0x61, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x12, 0x1b, 0x2e,
	0x63, 0x61, 0x2e, 0x47, 0x65, 0x74, 0x43, 0x61, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63,
	0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1c, 0x2e, 0x63, 0x61, 0x2e,
	0x47, 0x65, 0x74, 0x43, 0x61, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x09, 0x5a, 0x07, 0x63, 0x61, 0x2f, 0x63,
	0x61, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_ca_proto_rawDescOnce sync.Once
	file_proto_ca_proto_rawDescData = file_proto_ca_proto_rawDesc
)

func file_proto_ca_proto_rawDescGZIP() []byte {
	file_proto_ca_proto_rawDescOnce.Do(func() {
		file_proto_ca_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_ca_proto_rawDescData)
	})
	return file_proto_ca_proto_rawDescData
}

var file_proto_ca_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_proto_ca_proto_goTypes = []interface{}{
	(*SignCertificateRequest)(nil),   // 0: ca.SignCertificateRequest
	(*SignCertificateResponse)(nil),  // 1: ca.SignCertificateResponse
	(*GetCaCertificateRequest)(nil),  // 2: ca.GetCaCertificateRequest
	(*GetCaCertificateResponse)(nil), // 3: ca.GetCaCertificateResponse
}
var file_proto_ca_proto_depIdxs = []int32{
	0, // 0: ca.CertificateAuthority.SignCertificate:input_type -> ca.SignCertificateRequest
	2, // 1: ca.CertificateAuthority.GetCaCertificate:input_type -> ca.GetCaCertificateRequest
	1, // 2: ca.CertificateAuthority.SignCertificate:output_type -> ca.SignCertificateResponse
	3, // 3: ca.CertificateAuthority.GetCaCertificate:output_type -> ca.GetCaCertificateResponse
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_proto_ca_proto_init() }
func file_proto_ca_proto_init() {
	if File_proto_ca_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_ca_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignCertificateRequest); i {
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
		file_proto_ca_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignCertificateResponse); i {
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
		file_proto_ca_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetCaCertificateRequest); i {
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
		file_proto_ca_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetCaCertificateResponse); i {
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
			RawDescriptor: file_proto_ca_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_ca_proto_goTypes,
		DependencyIndexes: file_proto_ca_proto_depIdxs,
		MessageInfos:      file_proto_ca_proto_msgTypes,
	}.Build()
	File_proto_ca_proto = out.File
	file_proto_ca_proto_rawDesc = nil
	file_proto_ca_proto_goTypes = nil
	file_proto_ca_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// CertificateAuthorityClient is the client API for CertificateAuthority service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type CertificateAuthorityClient interface {
	// Signed a certificate request.
	SignCertificate(ctx context.Context, in *SignCertificateRequest, opts ...grpc.CallOption) (*SignCertificateResponse, error)
	// Return the Authority Trust Certificate.
	GetCaCertificate(ctx context.Context, in *GetCaCertificateRequest, opts ...grpc.CallOption) (*GetCaCertificateResponse, error)
}

type certificateAuthorityClient struct {
	cc grpc.ClientConnInterface
}

func NewCertificateAuthorityClient(cc grpc.ClientConnInterface) CertificateAuthorityClient {
	return &certificateAuthorityClient{cc}
}

func (c *certificateAuthorityClient) SignCertificate(ctx context.Context, in *SignCertificateRequest, opts ...grpc.CallOption) (*SignCertificateResponse, error) {
	out := new(SignCertificateResponse)
	err := c.cc.Invoke(ctx, "/ca.CertificateAuthority/SignCertificate", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *certificateAuthorityClient) GetCaCertificate(ctx context.Context, in *GetCaCertificateRequest, opts ...grpc.CallOption) (*GetCaCertificateResponse, error) {
	out := new(GetCaCertificateResponse)
	err := c.cc.Invoke(ctx, "/ca.CertificateAuthority/GetCaCertificate", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CertificateAuthorityServer is the server API for CertificateAuthority service.
type CertificateAuthorityServer interface {
	// Signed a certificate request.
	SignCertificate(context.Context, *SignCertificateRequest) (*SignCertificateResponse, error)
	// Return the Authority Trust Certificate.
	GetCaCertificate(context.Context, *GetCaCertificateRequest) (*GetCaCertificateResponse, error)
}

// UnimplementedCertificateAuthorityServer can be embedded to have forward compatible implementations.
type UnimplementedCertificateAuthorityServer struct {
}

func (*UnimplementedCertificateAuthorityServer) SignCertificate(context.Context, *SignCertificateRequest) (*SignCertificateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SignCertificate not implemented")
}
func (*UnimplementedCertificateAuthorityServer) GetCaCertificate(context.Context, *GetCaCertificateRequest) (*GetCaCertificateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCaCertificate not implemented")
}

func RegisterCertificateAuthorityServer(s *grpc.Server, srv CertificateAuthorityServer) {
	s.RegisterService(&_CertificateAuthority_serviceDesc, srv)
}

func _CertificateAuthority_SignCertificate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SignCertificateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CertificateAuthorityServer).SignCertificate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ca.CertificateAuthority/SignCertificate",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CertificateAuthorityServer).SignCertificate(ctx, req.(*SignCertificateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CertificateAuthority_GetCaCertificate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetCaCertificateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CertificateAuthorityServer).GetCaCertificate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ca.CertificateAuthority/GetCaCertificate",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CertificateAuthorityServer).GetCaCertificate(ctx, req.(*GetCaCertificateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _CertificateAuthority_serviceDesc = grpc.ServiceDesc{
	ServiceName: "ca.CertificateAuthority",
	HandlerType: (*CertificateAuthorityServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SignCertificate",
			Handler:    _CertificateAuthority_SignCertificate_Handler,
		},
		{
			MethodName: "GetCaCertificate",
			Handler:    _CertificateAuthority_GetCaCertificate_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/ca.proto",
}
