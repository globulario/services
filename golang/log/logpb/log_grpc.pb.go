// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.4.0
// - protoc             v3.21.10
// source: log.proto

package logpb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.62.0 or later.
const _ = grpc.SupportPackageIsVersion8

const (
	LogService_Log_FullMethodName         = "/log.LogService/Log"
	LogService_GetLog_FullMethodName      = "/log.LogService/GetLog"
	LogService_DeleteLog_FullMethodName   = "/log.LogService/DeleteLog"
	LogService_ClearAllLog_FullMethodName = "/log.LogService/ClearAllLog"
)

// LogServiceClient is the client API for LogService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// LogService provides RPC methods for logging operations.
type LogServiceClient interface {
	// Logs a new message.
	Log(ctx context.Context, in *LogRqst, opts ...grpc.CallOption) (*LogRsp, error)
	// Retrieves log entries based on a query.
	// This is a server streaming RPC where the response is a stream of messages.
	GetLog(ctx context.Context, in *GetLogRqst, opts ...grpc.CallOption) (LogService_GetLogClient, error)
	// Deletes a specific log entry.
	DeleteLog(ctx context.Context, in *DeleteLogRqst, opts ...grpc.CallOption) (*DeleteLogRsp, error)
	// Clears all logs or logs matching a specific query pattern.
	ClearAllLog(ctx context.Context, in *ClearAllLogRqst, opts ...grpc.CallOption) (*ClearAllLogRsp, error)
}

type logServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewLogServiceClient(cc grpc.ClientConnInterface) LogServiceClient {
	return &logServiceClient{cc}
}

func (c *logServiceClient) Log(ctx context.Context, in *LogRqst, opts ...grpc.CallOption) (*LogRsp, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LogRsp)
	err := c.cc.Invoke(ctx, LogService_Log_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *logServiceClient) GetLog(ctx context.Context, in *GetLogRqst, opts ...grpc.CallOption) (LogService_GetLogClient, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &LogService_ServiceDesc.Streams[0], LogService_GetLog_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &logServiceGetLogClient{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type LogService_GetLogClient interface {
	Recv() (*GetLogRsp, error)
	grpc.ClientStream
}

type logServiceGetLogClient struct {
	grpc.ClientStream
}

func (x *logServiceGetLogClient) Recv() (*GetLogRsp, error) {
	m := new(GetLogRsp)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *logServiceClient) DeleteLog(ctx context.Context, in *DeleteLogRqst, opts ...grpc.CallOption) (*DeleteLogRsp, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteLogRsp)
	err := c.cc.Invoke(ctx, LogService_DeleteLog_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *logServiceClient) ClearAllLog(ctx context.Context, in *ClearAllLogRqst, opts ...grpc.CallOption) (*ClearAllLogRsp, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ClearAllLogRsp)
	err := c.cc.Invoke(ctx, LogService_ClearAllLog_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LogServiceServer is the server API for LogService service.
// All implementations should embed UnimplementedLogServiceServer
// for forward compatibility
//
// LogService provides RPC methods for logging operations.
type LogServiceServer interface {
	// Logs a new message.
	Log(context.Context, *LogRqst) (*LogRsp, error)
	// Retrieves log entries based on a query.
	// This is a server streaming RPC where the response is a stream of messages.
	GetLog(*GetLogRqst, LogService_GetLogServer) error
	// Deletes a specific log entry.
	DeleteLog(context.Context, *DeleteLogRqst) (*DeleteLogRsp, error)
	// Clears all logs or logs matching a specific query pattern.
	ClearAllLog(context.Context, *ClearAllLogRqst) (*ClearAllLogRsp, error)
}

// UnimplementedLogServiceServer should be embedded to have forward compatible implementations.
type UnimplementedLogServiceServer struct {
}

func (UnimplementedLogServiceServer) Log(context.Context, *LogRqst) (*LogRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Log not implemented")
}
func (UnimplementedLogServiceServer) GetLog(*GetLogRqst, LogService_GetLogServer) error {
	return status.Errorf(codes.Unimplemented, "method GetLog not implemented")
}
func (UnimplementedLogServiceServer) DeleteLog(context.Context, *DeleteLogRqst) (*DeleteLogRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteLog not implemented")
}
func (UnimplementedLogServiceServer) ClearAllLog(context.Context, *ClearAllLogRqst) (*ClearAllLogRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ClearAllLog not implemented")
}

// UnsafeLogServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to LogServiceServer will
// result in compilation errors.
type UnsafeLogServiceServer interface {
	mustEmbedUnimplementedLogServiceServer()
}

func RegisterLogServiceServer(s grpc.ServiceRegistrar, srv LogServiceServer) {
	s.RegisterService(&LogService_ServiceDesc, srv)
}

func _LogService_Log_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogRqst)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LogServiceServer).Log(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LogService_Log_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LogServiceServer).Log(ctx, req.(*LogRqst))
	}
	return interceptor(ctx, in, info, handler)
}

func _LogService_GetLog_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetLogRqst)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(LogServiceServer).GetLog(m, &logServiceGetLogServer{ServerStream: stream})
}

type LogService_GetLogServer interface {
	Send(*GetLogRsp) error
	grpc.ServerStream
}

type logServiceGetLogServer struct {
	grpc.ServerStream
}

func (x *logServiceGetLogServer) Send(m *GetLogRsp) error {
	return x.ServerStream.SendMsg(m)
}

func _LogService_DeleteLog_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteLogRqst)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LogServiceServer).DeleteLog(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LogService_DeleteLog_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LogServiceServer).DeleteLog(ctx, req.(*DeleteLogRqst))
	}
	return interceptor(ctx, in, info, handler)
}

func _LogService_ClearAllLog_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClearAllLogRqst)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LogServiceServer).ClearAllLog(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LogService_ClearAllLog_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LogServiceServer).ClearAllLog(ctx, req.(*ClearAllLogRqst))
	}
	return interceptor(ctx, in, info, handler)
}

// LogService_ServiceDesc is the grpc.ServiceDesc for LogService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var LogService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "log.LogService",
	HandlerType: (*LogServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Log",
			Handler:    _LogService_Log_Handler,
		},
		{
			MethodName: "DeleteLog",
			Handler:    _LogService_DeleteLog_Handler,
		},
		{
			MethodName: "ClearAllLog",
			Handler:    _LogService_ClearAllLog_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetLog",
			Handler:       _LogService_GetLog_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "log.proto",
}
