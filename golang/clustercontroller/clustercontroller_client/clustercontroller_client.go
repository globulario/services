package clustercontroller_client

import (
	"context"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ClusterControllerClient struct {
	cc *grpc.ClientConn
	c  clustercontrollerpb.ClusterControllerServiceClient

	id      string
	name    string
	domain  string
	mac     string
	address string
	port    int
	hasTLS  bool
	state   string

	ctx context.Context
}

func NewClusterControllerClient(address, id string) (*ClusterControllerClient, error) {
	client := &ClusterControllerClient{}
	if err := globular.InitClient(client, address, id); err != nil {
		return nil, err
	}
	if err := client.Reconnect(); err != nil {
		return nil, err
	}
	return client, nil
}

func (client *ClusterControllerClient) Reconnect() error {
	var err error
	for i := 0; i < 10; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = clustercontrollerpb.NewClusterControllerServiceClient(client.cc)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func (client *ClusterControllerClient) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	return client.ctx
}

func (client *ClusterControllerClient) GetAddress() string {
	return client.address
}

func (client *ClusterControllerClient) GetDomain() string {
	return client.domain
}

func (client *ClusterControllerClient) GetId() string {
	return client.id
}

func (client *ClusterControllerClient) GetMac() string {
	return client.mac
}

func (client *ClusterControllerClient) GetName() string {
	return client.name
}

func (client *ClusterControllerClient) GetState() string {
	return client.state
}

func (client *ClusterControllerClient) Close() {
	if client.cc != nil {
		client.cc.Close()
	}
}

func (client *ClusterControllerClient) SetAddress(address string) {
	client.address = address
}

func (client *ClusterControllerClient) SetDomain(domain string) {
	client.domain = domain
}

func (client *ClusterControllerClient) SetId(id string) {
	client.id = id
}

func (client *ClusterControllerClient) SetMac(mac string) {
	client.mac = mac
}

func (client *ClusterControllerClient) SetName(name string) {
	client.name = name
}

func (client *ClusterControllerClient) SetState(state string) {
	client.state = state
}

func (client *ClusterControllerClient) SetPort(port int) {
	client.port = port
}

func (client *ClusterControllerClient) GetPort() int {
	return client.port
}

func (client *ClusterControllerClient) HasTLS() bool {
	return client.hasTLS
}

func (client *ClusterControllerClient) SetTLS(value bool) {
	client.hasTLS = value
}

func (client *ClusterControllerClient) GetCertFile() string {
	return ""
}

func (client *ClusterControllerClient) GetKeyFile() string {
	return ""
}

func (client *ClusterControllerClient) GetCaFile() string {
	return ""
}

func (client *ClusterControllerClient) SetCertFile(path string) {}

func (client *ClusterControllerClient) SetKeyFile(path string) {}

func (client *ClusterControllerClient) SetCaFile(path string) {}

func (client *ClusterControllerClient) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *ClusterControllerClient) GetClusterInfo(ctx context.Context, req *timestamppb.Timestamp) (*clustercontrollerpb.ClusterInfo, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.GetClusterInfo(ctx, req)
}

func (client *ClusterControllerClient) CreateJoinToken(ctx context.Context, req *clustercontrollerpb.CreateJoinTokenRequest) (*clustercontrollerpb.CreateJoinTokenResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.CreateJoinToken(ctx, req)
}

func (client *ClusterControllerClient) RequestJoin(ctx context.Context, req *clustercontrollerpb.RequestJoinRequest) (*clustercontrollerpb.RequestJoinResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.RequestJoin(ctx, req)
}

func (client *ClusterControllerClient) ListJoinRequests(ctx context.Context, req *clustercontrollerpb.ListJoinRequestsRequest) (*clustercontrollerpb.ListJoinRequestsResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.ListJoinRequests(ctx, req)
}

func (client *ClusterControllerClient) ApproveJoin(ctx context.Context, req *clustercontrollerpb.ApproveJoinRequest) (*clustercontrollerpb.ApproveJoinResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.ApproveJoin(ctx, req)
}

func (client *ClusterControllerClient) RejectJoin(ctx context.Context, req *clustercontrollerpb.RejectJoinRequest) (*clustercontrollerpb.RejectJoinResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.RejectJoin(ctx, req)
}

func (client *ClusterControllerClient) ListNodes(ctx context.Context, req *clustercontrollerpb.ListNodesRequest) (*clustercontrollerpb.ListNodesResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.ListNodes(ctx, req)
}

func (client *ClusterControllerClient) SetNodeProfiles(ctx context.Context, req *clustercontrollerpb.SetNodeProfilesRequest) (*clustercontrollerpb.SetNodeProfilesResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.SetNodeProfiles(ctx, req)
}

func (client *ClusterControllerClient) GetNodePlan(ctx context.Context, req *clustercontrollerpb.GetNodePlanRequest) (*clustercontrollerpb.GetNodePlanResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.GetNodePlan(ctx, req)
}

func (client *ClusterControllerClient) WatchOperations(ctx context.Context, req *clustercontrollerpb.WatchOperationsRequest, opts ...grpc.CallOption) (clustercontrollerpb.ClusterControllerService_WatchOperationsClient, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.WatchOperations(ctx, req, opts...)
}
