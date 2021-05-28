//*
// Management of load on a cluster.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.13.0
// source: lb.proto

package lbpb

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

//*
// That structure contain necessary information to get
type ServerInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	//* The service instance id unique on the domain *
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	//* The service name, multiple instance can share the same name *
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	//* The domain of the service *
	Domain string `protobuf:"bytes,3,opt,name=domain,proto3" json:"domain,omitempty"`
	//* The service port *
	Port int32 `protobuf:"varint,4,opt,name=port,proto3" json:"port,omitempty"`
}

func (x *ServerInfo) Reset() {
	*x = ServerInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lb_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ServerInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServerInfo) ProtoMessage() {}

func (x *ServerInfo) ProtoReflect() protoreflect.Message {
	mi := &file_lb_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServerInfo.ProtoReflect.Descriptor instead.
func (*ServerInfo) Descriptor() ([]byte, []int) {
	return file_lb_proto_rawDescGZIP(), []int{0}
}

func (x *ServerInfo) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ServerInfo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ServerInfo) GetDomain() string {
	if x != nil {
		return x.Domain
	}
	return ""
}

func (x *ServerInfo) GetPort() int32 {
	if x != nil {
		return x.Port
	}
	return 0
}

//*
// That message contain information about server load.
type LoadInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	//* The server info *
	ServerInfo *ServerInfo `protobuf:"bytes,1,opt,name=serverInfo,proto3" json:"serverInfo,omitempty"`
	//* The cpu usage
	Load1  float64 `protobuf:"fixed64,2,opt,name=load1,proto3" json:"load1,omitempty"`   // during the last minutes
	Load5  float64 `protobuf:"fixed64,3,opt,name=load5,proto3" json:"load5,omitempty"`   // the last five minutes
	Load15 float64 `protobuf:"fixed64,4,opt,name=load15,proto3" json:"load15,omitempty"` // the last
}

func (x *LoadInfo) Reset() {
	*x = LoadInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lb_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LoadInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LoadInfo) ProtoMessage() {}

func (x *LoadInfo) ProtoReflect() protoreflect.Message {
	mi := &file_lb_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LoadInfo.ProtoReflect.Descriptor instead.
func (*LoadInfo) Descriptor() ([]byte, []int) {
	return file_lb_proto_rawDescGZIP(), []int{1}
}

func (x *LoadInfo) GetServerInfo() *ServerInfo {
	if x != nil {
		return x.ServerInfo
	}
	return nil
}

func (x *LoadInfo) GetLoad1() float64 {
	if x != nil {
		return x.Load1
	}
	return 0
}

func (x *LoadInfo) GetLoad5() float64 {
	if x != nil {
		return x.Load5
	}
	return 0
}

func (x *LoadInfo) GetLoad15() float64 {
	if x != nil {
		return x.Load15
	}
	return 0
}

//* Return the list of servers for a given service. *
type GetCanditatesRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ServiceName string `protobuf:"bytes,1,opt,name=serviceName,proto3" json:"serviceName,omitempty"`
}

func (x *GetCanditatesRequest) Reset() {
	*x = GetCanditatesRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lb_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetCanditatesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetCanditatesRequest) ProtoMessage() {}

func (x *GetCanditatesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_lb_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetCanditatesRequest.ProtoReflect.Descriptor instead.
func (*GetCanditatesRequest) Descriptor() ([]byte, []int) {
	return file_lb_proto_rawDescGZIP(), []int{2}
}

func (x *GetCanditatesRequest) GetServiceName() string {
	if x != nil {
		return x.ServiceName
	}
	return ""
}

type GetCanditatesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Servers []*ServerInfo `protobuf:"bytes,1,rep,name=servers,proto3" json:"servers,omitempty"`
}

func (x *GetCanditatesResponse) Reset() {
	*x = GetCanditatesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lb_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetCanditatesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetCanditatesResponse) ProtoMessage() {}

func (x *GetCanditatesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_lb_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetCanditatesResponse.ProtoReflect.Descriptor instead.
func (*GetCanditatesResponse) Descriptor() ([]byte, []int) {
	return file_lb_proto_rawDescGZIP(), []int{3}
}

func (x *GetCanditatesResponse) GetServers() []*ServerInfo {
	if x != nil {
		return x.Servers
	}
	return nil
}

//*
// Report load to the load balancer.
type ReportLoadInfoRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Info *LoadInfo `protobuf:"bytes,1,opt,name=info,proto3" json:"info,omitempty"`
}

func (x *ReportLoadInfoRequest) Reset() {
	*x = ReportLoadInfoRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lb_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReportLoadInfoRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReportLoadInfoRequest) ProtoMessage() {}

func (x *ReportLoadInfoRequest) ProtoReflect() protoreflect.Message {
	mi := &file_lb_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReportLoadInfoRequest.ProtoReflect.Descriptor instead.
func (*ReportLoadInfoRequest) Descriptor() ([]byte, []int) {
	return file_lb_proto_rawDescGZIP(), []int{4}
}

func (x *ReportLoadInfoRequest) GetInfo() *LoadInfo {
	if x != nil {
		return x.Info
	}
	return nil
}

type ReportLoadInfoResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ReportLoadInfoResponse) Reset() {
	*x = ReportLoadInfoResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lb_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReportLoadInfoResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReportLoadInfoResponse) ProtoMessage() {}

func (x *ReportLoadInfoResponse) ProtoReflect() protoreflect.Message {
	mi := &file_lb_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReportLoadInfoResponse.ProtoReflect.Descriptor instead.
func (*ReportLoadInfoResponse) Descriptor() ([]byte, []int) {
	return file_lb_proto_rawDescGZIP(), []int{5}
}

var File_lb_proto protoreflect.FileDescriptor

var file_lb_proto_rawDesc = []byte{
	0x0a, 0x08, 0x6c, 0x62, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x6c, 0x62, 0x22, 0x5c,
	0x0a, 0x0a, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x0e, 0x0a, 0x02,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x12, 0x16, 0x0a, 0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f, 0x72, 0x74,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x22, 0x7e, 0x0a, 0x08,
	0x4c, 0x6f, 0x61, 0x64, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x2e, 0x0a, 0x0a, 0x73, 0x65, 0x72, 0x76,
	0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x6c,
	0x62, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x0a, 0x73, 0x65,
	0x72, 0x76, 0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x6f, 0x61, 0x64,
	0x31, 0x18, 0x02, 0x20, 0x01, 0x28, 0x01, 0x52, 0x05, 0x6c, 0x6f, 0x61, 0x64, 0x31, 0x12, 0x14,
	0x0a, 0x05, 0x6c, 0x6f, 0x61, 0x64, 0x35, 0x18, 0x03, 0x20, 0x01, 0x28, 0x01, 0x52, 0x05, 0x6c,
	0x6f, 0x61, 0x64, 0x35, 0x12, 0x16, 0x0a, 0x06, 0x6c, 0x6f, 0x61, 0x64, 0x31, 0x35, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x01, 0x52, 0x06, 0x6c, 0x6f, 0x61, 0x64, 0x31, 0x35, 0x22, 0x38, 0x0a, 0x14,
	0x47, 0x65, 0x74, 0x43, 0x61, 0x6e, 0x64, 0x69, 0x74, 0x61, 0x74, 0x65, 0x73, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x20, 0x0a, 0x0b, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x4e,
	0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x22, 0x41, 0x0a, 0x15, 0x47, 0x65, 0x74, 0x43, 0x61, 0x6e,
	0x64, 0x69, 0x74, 0x61, 0x74, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x28, 0x0a, 0x07, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x0e, 0x2e, 0x6c, 0x62, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f,
	0x52, 0x07, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x73, 0x22, 0x39, 0x0a, 0x15, 0x52, 0x65, 0x70,
	0x6f, 0x72, 0x74, 0x4c, 0x6f, 0x61, 0x64, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x20, 0x0a, 0x04, 0x69, 0x6e, 0x66, 0x6f, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x0c, 0x2e, 0x6c, 0x62, 0x2e, 0x4c, 0x6f, 0x61, 0x64, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04,
	0x69, 0x6e, 0x66, 0x6f, 0x22, 0x18, 0x0a, 0x16, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x4c, 0x6f,
	0x61, 0x64, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0xab,
	0x01, 0x0a, 0x14, 0x4c, 0x6f, 0x61, 0x64, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x69, 0x6e, 0x67,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x46, 0x0a, 0x0d, 0x47, 0x65, 0x74, 0x43, 0x61,
	0x6e, 0x64, 0x69, 0x74, 0x61, 0x74, 0x65, 0x73, 0x12, 0x18, 0x2e, 0x6c, 0x62, 0x2e, 0x47, 0x65,
	0x74, 0x43, 0x61, 0x6e, 0x64, 0x69, 0x74, 0x61, 0x74, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x19, 0x2e, 0x6c, 0x62, 0x2e, 0x47, 0x65, 0x74, 0x43, 0x61, 0x6e, 0x64, 0x69,
	0x74, 0x61, 0x74, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12,
	0x4b, 0x0a, 0x0e, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x4c, 0x6f, 0x61, 0x64, 0x49, 0x6e, 0x66,
	0x6f, 0x12, 0x19, 0x2e, 0x6c, 0x62, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x4c, 0x6f, 0x61,
	0x64, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x6c,
	0x62, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x4c, 0x6f, 0x61, 0x64, 0x49, 0x6e, 0x66, 0x6f,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x28, 0x01, 0x42, 0x09, 0x5a, 0x07,
	0x6c, 0x62, 0x2f, 0x6c, 0x62, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_lb_proto_rawDescOnce sync.Once
	file_lb_proto_rawDescData = file_lb_proto_rawDesc
)

func file_lb_proto_rawDescGZIP() []byte {
	file_lb_proto_rawDescOnce.Do(func() {
		file_lb_proto_rawDescData = protoimpl.X.CompressGZIP(file_lb_proto_rawDescData)
	})
	return file_lb_proto_rawDescData
}

var file_lb_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_lb_proto_goTypes = []interface{}{
	(*ServerInfo)(nil),             // 0: lb.ServerInfo
	(*LoadInfo)(nil),               // 1: lb.LoadInfo
	(*GetCanditatesRequest)(nil),   // 2: lb.GetCanditatesRequest
	(*GetCanditatesResponse)(nil),  // 3: lb.GetCanditatesResponse
	(*ReportLoadInfoRequest)(nil),  // 4: lb.ReportLoadInfoRequest
	(*ReportLoadInfoResponse)(nil), // 5: lb.ReportLoadInfoResponse
}
var file_lb_proto_depIdxs = []int32{
	0, // 0: lb.LoadInfo.serverInfo:type_name -> lb.ServerInfo
	0, // 1: lb.GetCanditatesResponse.servers:type_name -> lb.ServerInfo
	1, // 2: lb.ReportLoadInfoRequest.info:type_name -> lb.LoadInfo
	2, // 3: lb.LoadBalancingService.GetCanditates:input_type -> lb.GetCanditatesRequest
	4, // 4: lb.LoadBalancingService.reportLoadInfo:input_type -> lb.ReportLoadInfoRequest
	3, // 5: lb.LoadBalancingService.GetCanditates:output_type -> lb.GetCanditatesResponse
	5, // 6: lb.LoadBalancingService.reportLoadInfo:output_type -> lb.ReportLoadInfoResponse
	5, // [5:7] is the sub-list for method output_type
	3, // [3:5] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_lb_proto_init() }
func file_lb_proto_init() {
	if File_lb_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_lb_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ServerInfo); i {
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
		file_lb_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LoadInfo); i {
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
		file_lb_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetCanditatesRequest); i {
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
		file_lb_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetCanditatesResponse); i {
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
		file_lb_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ReportLoadInfoRequest); i {
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
		file_lb_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ReportLoadInfoResponse); i {
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
			RawDescriptor: file_lb_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_lb_proto_goTypes,
		DependencyIndexes: file_lb_proto_depIdxs,
		MessageInfos:      file_lb_proto_msgTypes,
	}.Build()
	File_lb_proto = out.File
	file_lb_proto_rawDesc = nil
	file_lb_proto_goTypes = nil
	file_lb_proto_depIdxs = nil
}
