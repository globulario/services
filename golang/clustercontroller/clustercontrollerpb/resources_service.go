package clustercontrollerpb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ApplyClusterNetworkRequest struct {
	Object *ClusterNetwork
}

type ApplyServiceDesiredVersionRequest struct {
	Object *ServiceDesiredVersion
}

type DeleteServiceDesiredVersionRequest struct {
	Name string
}

type GetClusterNetworkRequest struct{}

type ListServiceDesiredVersionsRequest struct{}

type ListServiceDesiredVersionsResponse struct {
	Items []*ServiceDesiredVersion
}

type WatchRequest struct {
	Type                string
	Prefix              string
	FromResourceVersion string
	IncludeExisting     bool
}

func (x *ApplyClusterNetworkRequest) GetObject() *ClusterNetwork {
	if x != nil {
		return x.Object
	}
	return nil
}

func (x *ApplyServiceDesiredVersionRequest) GetObject() *ServiceDesiredVersion {
	if x != nil {
		return x.Object
	}
	return nil
}

func (x *DeleteServiceDesiredVersionRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *WatchRequest) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *WatchRequest) GetPrefix() string {
	if x != nil {
		return x.Prefix
	}
	return ""
}

func (x *WatchRequest) GetFromResourceVersion() string {
	if x != nil {
		return x.FromResourceVersion
	}
	return ""
}

func (x *WatchRequest) GetIncludeExisting() bool {
	if x != nil {
		return x.IncludeExisting
	}
	return false
}

type ResourcesServiceServer interface {
	ApplyClusterNetwork(context.Context, *ApplyClusterNetworkRequest) (*ClusterNetwork, error)
	GetClusterNetwork(context.Context, *GetClusterNetworkRequest) (*ClusterNetwork, error)
	ApplyServiceDesiredVersion(context.Context, *ApplyServiceDesiredVersionRequest) (*ServiceDesiredVersion, error)
	DeleteServiceDesiredVersion(context.Context, *DeleteServiceDesiredVersionRequest) (*emptypb.Empty, error)
	ListServiceDesiredVersions(context.Context, *ListServiceDesiredVersionsRequest) (*ListServiceDesiredVersionsResponse, error)
	Watch(*WatchRequest, ResourcesService_WatchServer) error
}

type UnimplementedResourcesServiceServer struct{}

func (UnimplementedResourcesServiceServer) ApplyClusterNetwork(context.Context, *ApplyClusterNetworkRequest) (*ClusterNetwork, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ApplyClusterNetwork not implemented")
}
func (UnimplementedResourcesServiceServer) GetClusterNetwork(context.Context, *GetClusterNetworkRequest) (*ClusterNetwork, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetClusterNetwork not implemented")
}
func (UnimplementedResourcesServiceServer) ApplyServiceDesiredVersion(context.Context, *ApplyServiceDesiredVersionRequest) (*ServiceDesiredVersion, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ApplyServiceDesiredVersion not implemented")
}
func (UnimplementedResourcesServiceServer) DeleteServiceDesiredVersion(context.Context, *DeleteServiceDesiredVersionRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteServiceDesiredVersion not implemented")
}
func (UnimplementedResourcesServiceServer) ListServiceDesiredVersions(context.Context, *ListServiceDesiredVersionsRequest) (*ListServiceDesiredVersionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListServiceDesiredVersions not implemented")
}
func (UnimplementedResourcesServiceServer) Watch(*WatchRequest, ResourcesService_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}

type ResourcesService_WatchServer interface {
	Send(*WatchEvent) error
	grpc.ServerStream
}

func RegisterResourcesServiceServer(s *grpc.Server, srv ResourcesServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "clustercontroller.ResourcesService",
		HandlerType: (*ResourcesServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "ApplyClusterNetwork",
				Handler:    _ResourcesService_ApplyClusterNetwork_Handler,
			},
			{
				MethodName: "GetClusterNetwork",
				Handler:    _ResourcesService_GetClusterNetwork_Handler,
			},
			{
				MethodName: "ApplyServiceDesiredVersion",
				Handler:    _ResourcesService_ApplyServiceDesiredVersion_Handler,
			},
			{
				MethodName: "DeleteServiceDesiredVersion",
				Handler:    _ResourcesService_DeleteServiceDesiredVersion_Handler,
			},
			{
				MethodName: "ListServiceDesiredVersions",
				Handler:    _ResourcesService_ListServiceDesiredVersions_Handler,
			},
		},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "Watch",
				Handler:       _ResourcesService_Watch_Handler,
				ServerStreams: true,
			},
		},
	}, srv)
}

func _ResourcesService_ApplyClusterNetwork_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ApplyClusterNetworkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ResourcesServiceServer).ApplyClusterNetwork(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/clustercontroller.ResourcesService/ApplyClusterNetwork",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ResourcesServiceServer).ApplyClusterNetwork(ctx, req.(*ApplyClusterNetworkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ResourcesService_GetClusterNetwork_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetClusterNetworkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ResourcesServiceServer).GetClusterNetwork(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/clustercontroller.ResourcesService/GetClusterNetwork",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ResourcesServiceServer).GetClusterNetwork(ctx, req.(*GetClusterNetworkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ResourcesService_ApplyServiceDesiredVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ApplyServiceDesiredVersionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ResourcesServiceServer).ApplyServiceDesiredVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/clustercontroller.ResourcesService/ApplyServiceDesiredVersion",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ResourcesServiceServer).ApplyServiceDesiredVersion(ctx, req.(*ApplyServiceDesiredVersionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ResourcesService_DeleteServiceDesiredVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteServiceDesiredVersionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ResourcesServiceServer).DeleteServiceDesiredVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/clustercontroller.ResourcesService/DeleteServiceDesiredVersion",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ResourcesServiceServer).DeleteServiceDesiredVersion(ctx, req.(*DeleteServiceDesiredVersionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ResourcesService_ListServiceDesiredVersions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListServiceDesiredVersionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ResourcesServiceServer).ListServiceDesiredVersions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/clustercontroller.ResourcesService/ListServiceDesiredVersions",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ResourcesServiceServer).ListServiceDesiredVersions(ctx, req.(*ListServiceDesiredVersionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ResourcesService_Watch_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(WatchRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ResourcesServiceServer).Watch(m, &resourcesServiceWatchServer{stream})
}

type resourcesServiceWatchServer struct {
	grpc.ServerStream
}

func (x *resourcesServiceWatchServer) Send(m *WatchEvent) error {
	return x.ServerStream.SendMsg(m)
}

// ResourcesServiceClient is the client API for ResourcesService.
type ResourcesServiceClient interface {
	ApplyClusterNetwork(ctx context.Context, in *ApplyClusterNetworkRequest, opts ...grpc.CallOption) (*ClusterNetwork, error)
	GetClusterNetwork(ctx context.Context, in *GetClusterNetworkRequest, opts ...grpc.CallOption) (*ClusterNetwork, error)
	ApplyServiceDesiredVersion(ctx context.Context, in *ApplyServiceDesiredVersionRequest, opts ...grpc.CallOption) (*ServiceDesiredVersion, error)
	DeleteServiceDesiredVersion(ctx context.Context, in *DeleteServiceDesiredVersionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	ListServiceDesiredVersions(ctx context.Context, in *ListServiceDesiredVersionsRequest, opts ...grpc.CallOption) (*ListServiceDesiredVersionsResponse, error)
	Watch(ctx context.Context, in *WatchRequest, opts ...grpc.CallOption) (ResourcesService_WatchClient, error)
}

type resourcesServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewResourcesServiceClient(cc grpc.ClientConnInterface) ResourcesServiceClient {
	return &resourcesServiceClient{cc}
}

func (c *resourcesServiceClient) ApplyClusterNetwork(ctx context.Context, in *ApplyClusterNetworkRequest, opts ...grpc.CallOption) (*ClusterNetwork, error) {
	out := new(ClusterNetwork)
	err := c.cc.Invoke(ctx, "/clustercontroller.ResourcesService/ApplyClusterNetwork", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *resourcesServiceClient) GetClusterNetwork(ctx context.Context, in *GetClusterNetworkRequest, opts ...grpc.CallOption) (*ClusterNetwork, error) {
	out := new(ClusterNetwork)
	err := c.cc.Invoke(ctx, "/clustercontroller.ResourcesService/GetClusterNetwork", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *resourcesServiceClient) ApplyServiceDesiredVersion(ctx context.Context, in *ApplyServiceDesiredVersionRequest, opts ...grpc.CallOption) (*ServiceDesiredVersion, error) {
	out := new(ServiceDesiredVersion)
	err := c.cc.Invoke(ctx, "/clustercontroller.ResourcesService/ApplyServiceDesiredVersion", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *resourcesServiceClient) DeleteServiceDesiredVersion(ctx context.Context, in *DeleteServiceDesiredVersionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/clustercontroller.ResourcesService/DeleteServiceDesiredVersion", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *resourcesServiceClient) ListServiceDesiredVersions(ctx context.Context, in *ListServiceDesiredVersionsRequest, opts ...grpc.CallOption) (*ListServiceDesiredVersionsResponse, error) {
	out := new(ListServiceDesiredVersionsResponse)
	err := c.cc.Invoke(ctx, "/clustercontroller.ResourcesService/ListServiceDesiredVersions", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *resourcesServiceClient) Watch(ctx context.Context, in *WatchRequest, opts ...grpc.CallOption) (ResourcesService_WatchClient, error) {
	stream, err := c.cc.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "Watch",
		ServerStreams: true,
	}, "/clustercontroller.ResourcesService/Watch", opts...)
	if err != nil {
		return nil, err
	}
	x := &resourcesServiceWatchClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type ResourcesService_WatchClient interface {
	Recv() (*WatchEvent, error)
	grpc.ClientStream
}

type resourcesServiceWatchClient struct {
	grpc.ClientStream
}

func (x *resourcesServiceWatchClient) Recv() (*WatchEvent, error) {
	m := new(WatchEvent)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}
