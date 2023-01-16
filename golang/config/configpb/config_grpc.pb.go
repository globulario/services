// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.12.4
// source: config.proto

package configpb

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

// ConfigServiceClient is the client API for ConfigService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ConfigServiceClient interface {
	// Set a service configuration.
	SetServiceConfiguration(ctx context.Context, in *SetServiceConfigurationRequest, opts ...grpc.CallOption) (*SetServiceConfigurationResponse, error)
	// Get the configuration at a given path on the server.
	GetServiceConfiguration(ctx context.Context, in *GetServiceConfigurationRequest, opts ...grpc.CallOption) (*GetServiceConfigurationResponse, error)
	// Get service configuration with a given id.
	GetServiceConfigurationById(ctx context.Context, in *GetServiceConfigurationByIdRequest, opts ...grpc.CallOption) (*GetServiceConfigurationByIdResponse, error)
	// Get list of service configuration with a given name
	GetServicesConfigurationsByName(ctx context.Context, in *GetServicesConfigurationsByNameRequest, opts ...grpc.CallOption) (*GetServicesConfigurationsByNameResponse, error)
	// Get the list of all services configurations
	GetServicesConfigurations(ctx context.Context, in *GetServicesConfigurationsRequest, opts ...grpc.CallOption) (*GetServicesConfigurationsResponse, error)
}

type configServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewConfigServiceClient(cc grpc.ClientConnInterface) ConfigServiceClient {
	return &configServiceClient{cc}
}

func (c *configServiceClient) SetServiceConfiguration(ctx context.Context, in *SetServiceConfigurationRequest, opts ...grpc.CallOption) (*SetServiceConfigurationResponse, error) {
	out := new(SetServiceConfigurationResponse)
	err := c.cc.Invoke(ctx, "/config.ConfigService/SetServiceConfiguration", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *configServiceClient) GetServiceConfiguration(ctx context.Context, in *GetServiceConfigurationRequest, opts ...grpc.CallOption) (*GetServiceConfigurationResponse, error) {
	out := new(GetServiceConfigurationResponse)
	err := c.cc.Invoke(ctx, "/config.ConfigService/GetServiceConfiguration", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *configServiceClient) GetServiceConfigurationById(ctx context.Context, in *GetServiceConfigurationByIdRequest, opts ...grpc.CallOption) (*GetServiceConfigurationByIdResponse, error) {
	out := new(GetServiceConfigurationByIdResponse)
	err := c.cc.Invoke(ctx, "/config.ConfigService/GetServiceConfigurationById", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *configServiceClient) GetServicesConfigurationsByName(ctx context.Context, in *GetServicesConfigurationsByNameRequest, opts ...grpc.CallOption) (*GetServicesConfigurationsByNameResponse, error) {
	out := new(GetServicesConfigurationsByNameResponse)
	err := c.cc.Invoke(ctx, "/config.ConfigService/GetServicesConfigurationsByName", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *configServiceClient) GetServicesConfigurations(ctx context.Context, in *GetServicesConfigurationsRequest, opts ...grpc.CallOption) (*GetServicesConfigurationsResponse, error) {
	out := new(GetServicesConfigurationsResponse)
	err := c.cc.Invoke(ctx, "/config.ConfigService/GetServicesConfigurations", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConfigServiceServer is the server API for ConfigService service.
// All implementations should embed UnimplementedConfigServiceServer
// for forward compatibility
type ConfigServiceServer interface {
	// Set a service configuration.
	SetServiceConfiguration(context.Context, *SetServiceConfigurationRequest) (*SetServiceConfigurationResponse, error)
	// Get the configuration at a given path on the server.
	GetServiceConfiguration(context.Context, *GetServiceConfigurationRequest) (*GetServiceConfigurationResponse, error)
	// Get service configuration with a given id.
	GetServiceConfigurationById(context.Context, *GetServiceConfigurationByIdRequest) (*GetServiceConfigurationByIdResponse, error)
	// Get list of service configuration with a given name
	GetServicesConfigurationsByName(context.Context, *GetServicesConfigurationsByNameRequest) (*GetServicesConfigurationsByNameResponse, error)
	// Get the list of all services configurations
	GetServicesConfigurations(context.Context, *GetServicesConfigurationsRequest) (*GetServicesConfigurationsResponse, error)
}

// UnimplementedConfigServiceServer should be embedded to have forward compatible implementations.
type UnimplementedConfigServiceServer struct {
}

func (UnimplementedConfigServiceServer) SetServiceConfiguration(context.Context, *SetServiceConfigurationRequest) (*SetServiceConfigurationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetServiceConfiguration not implemented")
}
func (UnimplementedConfigServiceServer) GetServiceConfiguration(context.Context, *GetServiceConfigurationRequest) (*GetServiceConfigurationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServiceConfiguration not implemented")
}
func (UnimplementedConfigServiceServer) GetServiceConfigurationById(context.Context, *GetServiceConfigurationByIdRequest) (*GetServiceConfigurationByIdResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServiceConfigurationById not implemented")
}
func (UnimplementedConfigServiceServer) GetServicesConfigurationsByName(context.Context, *GetServicesConfigurationsByNameRequest) (*GetServicesConfigurationsByNameResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServicesConfigurationsByName not implemented")
}
func (UnimplementedConfigServiceServer) GetServicesConfigurations(context.Context, *GetServicesConfigurationsRequest) (*GetServicesConfigurationsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServicesConfigurations not implemented")
}

// UnsafeConfigServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ConfigServiceServer will
// result in compilation errors.
type UnsafeConfigServiceServer interface {
	mustEmbedUnimplementedConfigServiceServer()
}

func RegisterConfigServiceServer(s grpc.ServiceRegistrar, srv ConfigServiceServer) {
	s.RegisterService(&ConfigService_ServiceDesc, srv)
}

func _ConfigService_SetServiceConfiguration_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetServiceConfigurationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfigServiceServer).SetServiceConfiguration(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/config.ConfigService/SetServiceConfiguration",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfigServiceServer).SetServiceConfiguration(ctx, req.(*SetServiceConfigurationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConfigService_GetServiceConfiguration_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetServiceConfigurationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfigServiceServer).GetServiceConfiguration(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/config.ConfigService/GetServiceConfiguration",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfigServiceServer).GetServiceConfiguration(ctx, req.(*GetServiceConfigurationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConfigService_GetServiceConfigurationById_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetServiceConfigurationByIdRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfigServiceServer).GetServiceConfigurationById(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/config.ConfigService/GetServiceConfigurationById",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfigServiceServer).GetServiceConfigurationById(ctx, req.(*GetServiceConfigurationByIdRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConfigService_GetServicesConfigurationsByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetServicesConfigurationsByNameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfigServiceServer).GetServicesConfigurationsByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/config.ConfigService/GetServicesConfigurationsByName",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfigServiceServer).GetServicesConfigurationsByName(ctx, req.(*GetServicesConfigurationsByNameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConfigService_GetServicesConfigurations_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetServicesConfigurationsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfigServiceServer).GetServicesConfigurations(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/config.ConfigService/GetServicesConfigurations",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfigServiceServer).GetServicesConfigurations(ctx, req.(*GetServicesConfigurationsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ConfigService_ServiceDesc is the grpc.ServiceDesc for ConfigService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ConfigService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "config.ConfigService",
	HandlerType: (*ConfigServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetServiceConfiguration",
			Handler:    _ConfigService_SetServiceConfiguration_Handler,
		},
		{
			MethodName: "GetServiceConfiguration",
			Handler:    _ConfigService_GetServiceConfiguration_Handler,
		},
		{
			MethodName: "GetServiceConfigurationById",
			Handler:    _ConfigService_GetServiceConfigurationById_Handler,
		},
		{
			MethodName: "GetServicesConfigurationsByName",
			Handler:    _ConfigService_GetServicesConfigurationsByName_Handler,
		},
		{
			MethodName: "GetServicesConfigurations",
			Handler:    _ConfigService_GetServicesConfigurations_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "config.proto",
}
