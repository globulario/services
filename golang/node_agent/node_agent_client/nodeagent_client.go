package nodeagent_client

import (
	"context"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	globular "github.com/globulario/services/golang/globular_client"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	planpb "github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// NodeAgentClient wraps the gRPC stub and keeps client metadata in sync with Globular expectations.
type NodeAgentClient struct {
	cc *grpc.ClientConn
	c  node_agentpb.NodeAgentServiceClient

	id      string
	name    string
	domain  string
	address string
	mac     string
	state   string
	port    int

	hasTLS   bool
	keyFile  string
	certFile string
	caFile   string

	ctx context.Context
}

// NewNodeAgentClient builds and connects a Globular-friendly node agent client.
func NewNodeAgentClient(address string, id string) (*NodeAgentClient, error) {
	client := &NodeAgentClient{}
	if err := globular.InitClient(client, address, id); err != nil {
		return nil, err
	}

	if err := client.Reconnect(); err != nil {
		return nil, err
	}

	return client, nil
}

func (client *NodeAgentClient) Reconnect() error {
	var err error
	tries := 10
	for i := 0; i < tries; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = node_agentpb.NewNodeAgentServiceClient(client.cc)
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func (client *NodeAgentClient) SetAddress(address string) {
	client.address = address
}

func (client *NodeAgentClient) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *NodeAgentClient) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{
			"token":   string(token),
			"domain":  client.domain,
			"mac":     client.mac,
			"address": client.address,
		})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

func (client *NodeAgentClient) GetDomain() string {
	return client.domain
}

func (client *NodeAgentClient) GetAddress() string {
	return client.address
}

func (client *NodeAgentClient) GetId() string {
	return client.id
}

func (client *NodeAgentClient) GetState() string {
	return client.state
}

func (client *NodeAgentClient) GetName() string {
	return client.name
}

func (client *NodeAgentClient) GetMac() string {
	return client.mac
}

func (client *NodeAgentClient) Close() {
	if client.cc != nil {
		client.cc.Close()
	}
}

func (client *NodeAgentClient) SetPort(port int) {
	client.port = port
}

func (client *NodeAgentClient) GetPort() int {
	return client.port
}

func (client *NodeAgentClient) SetId(id string) {
	client.id = id
}

func (client *NodeAgentClient) SetName(name string) {
	client.name = name
}

func (client *NodeAgentClient) SetMac(mac string) {
	client.mac = mac
}

func (client *NodeAgentClient) SetState(state string) {
	client.state = state
}

func (client *NodeAgentClient) SetDomain(domain string) {
	client.domain = domain
}

func (client *NodeAgentClient) HasTLS() bool {
	return client.hasTLS
}

func (client *NodeAgentClient) GetCertFile() string {
	return client.certFile
}

func (client *NodeAgentClient) GetKeyFile() string {
	return client.keyFile
}

func (client *NodeAgentClient) GetCaFile() string {
	return client.caFile
}

func (client *NodeAgentClient) SetTLS(value bool) {
	client.hasTLS = value
}

func (client *NodeAgentClient) SetCertFile(path string) {
	client.certFile = path
}

func (client *NodeAgentClient) SetKeyFile(path string) {
	client.keyFile = path
}

func (client *NodeAgentClient) SetCaFile(path string) {
	client.caFile = path
}

func (client *NodeAgentClient) JoinCluster(ctx context.Context, controllerEndpoint, joinToken string) (*node_agentpb.JoinClusterResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	req := &node_agentpb.JoinClusterRequest{
		ControllerEndpoint: controllerEndpoint,
		JoinToken:          joinToken,
	}
	return client.c.JoinCluster(ctx, req)
}

func (client *NodeAgentClient) GetInventory(ctx context.Context) (*node_agentpb.GetInventoryResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.GetInventory(ctx, &node_agentpb.GetInventoryRequest{})
}

func (client *NodeAgentClient) ApplyPlan(ctx context.Context, plan *cluster_controllerpb.NodePlan) (*node_agentpb.ApplyPlanResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.ApplyPlan(ctx, &node_agentpb.ApplyPlanRequest{Plan: plan})
}

func (client *NodeAgentClient) WatchOperation(ctx context.Context, operationID string) (node_agentpb.NodeAgentService_WatchOperationClient, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	req := &node_agentpb.WatchOperationRequest{OperationId: operationID}
	return client.c.WatchOperation(ctx, req)
}

func (client *NodeAgentClient) ApplyPlanV1(ctx context.Context, plan *planpb.NodePlan) (*node_agentpb.ApplyPlanV1Response, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.ApplyPlanV1(ctx, &node_agentpb.ApplyPlanV1Request{Plan: plan})
}

func (client *NodeAgentClient) GetPlanStatusV1(ctx context.Context, operationID string) (*node_agentpb.GetPlanStatusV1Response, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.GetPlanStatusV1(ctx, &node_agentpb.GetPlanStatusV1Request{
		OperationId: operationID,
	})
}

func (client *NodeAgentClient) BootstrapFirstNode(ctx context.Context, clusterDomain, controllerBind string, profiles []string) (*node_agentpb.BootstrapFirstNodeResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	req := &node_agentpb.BootstrapFirstNodeRequest{
		ClusterDomain:  clusterDomain,
		ControllerBind: controllerBind,
		Profiles:       profiles,
	}
	return client.c.BootstrapFirstNode(ctx, req)
}

// ListInstalledPackages returns all installed packages on a node, optionally filtered by kind.
func (client *NodeAgentClient) ListInstalledPackages(ctx context.Context, nodeID, kind string) ([]*node_agentpb.InstalledPackage, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	resp, err := client.c.ListInstalledPackages(ctx, &node_agentpb.ListInstalledPackagesRequest{
		NodeId: nodeID,
		Kind:   kind,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetPackages(), nil
}

// GetInstalledPackage returns a single installed package record from a node.
func (client *NodeAgentClient) GetInstalledPackage(ctx context.Context, nodeID, kind, name string) (*node_agentpb.InstalledPackage, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	resp, err := client.c.GetInstalledPackage(ctx, &node_agentpb.GetInstalledPackageRequest{
		NodeId: nodeID,
		Kind:   kind,
		Name:   name,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetPackage(), nil
}
