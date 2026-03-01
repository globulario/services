package clusterdoctor_client

import (
	"context"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
)

// ClusterDoctorClient is the Go client for ClusterDoctorService.
// It implements the globular.Client interface so InitClient / GetClientConnection work unchanged.
type ClusterDoctorClient struct {
	cc *grpc.ClientConn
	c  cluster_doctorpb.ClusterDoctorServiceClient

	id       string
	name     string
	domain   string
	mac      string
	address  string
	port     int
	hasTLS   bool
	state    string
	certFile string
	keyFile  string
	caFile   string

	ctx context.Context
}

func NewClusterDoctorClient(address, id string) (*ClusterDoctorClient, error) {
	client := &ClusterDoctorClient{}
	if err := globular.InitClient(client, address, id); err != nil {
		return nil, err
	}
	if err := client.Reconnect(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *ClusterDoctorClient) Reconnect() error {
	var err error
	for i := 0; i < 10; i++ {
		c.cc, err = globular.GetClientConnection(c)
		if err == nil {
			c.c = cluster_doctorpb.NewClusterDoctorServiceClient(c.cc)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func (c *ClusterDoctorClient) GetCtx() context.Context {
	if c.ctx == nil {
		c.ctx = globular.GetClientContext(c)
	}
	return c.ctx
}

// ─── globular.Client interface ────────────────────────────────────────────────

func (c *ClusterDoctorClient) GetAddress() string  { return c.address }
func (c *ClusterDoctorClient) GetDomain() string   { return c.domain }
func (c *ClusterDoctorClient) GetId() string       { return c.id }
func (c *ClusterDoctorClient) GetMac() string      { return c.mac }
func (c *ClusterDoctorClient) GetName() string     { return c.name }
func (c *ClusterDoctorClient) GetState() string    { return c.state }
func (c *ClusterDoctorClient) GetPort() int        { return c.port }
func (c *ClusterDoctorClient) HasTLS() bool        { return c.hasTLS }

func (c *ClusterDoctorClient) SetAddress(v string) { c.address = v }
func (c *ClusterDoctorClient) SetDomain(v string)  { c.domain = v }
func (c *ClusterDoctorClient) SetId(v string)      { c.id = v }
func (c *ClusterDoctorClient) SetMac(v string)     { c.mac = v }
func (c *ClusterDoctorClient) SetName(v string)    { c.name = v }
func (c *ClusterDoctorClient) SetState(v string)   { c.state = v }
func (c *ClusterDoctorClient) SetPort(v int)       { c.port = v }
func (c *ClusterDoctorClient) SetTLS(v bool)       { c.hasTLS = v }
func (c *ClusterDoctorClient) GetCertFile() string { return c.certFile }
func (c *ClusterDoctorClient) GetKeyFile() string  { return c.keyFile }
func (c *ClusterDoctorClient) GetCaFile() string   { return c.caFile }
func (c *ClusterDoctorClient) SetCertFile(v string) { c.certFile = v }
func (c *ClusterDoctorClient) SetKeyFile(v string)  { c.keyFile = v }
func (c *ClusterDoctorClient) SetCaFile(v string)   { c.caFile = v }

func (c *ClusterDoctorClient) Close() {
	if c.cc != nil {
		c.cc.Close()
	}
}

func (c *ClusterDoctorClient) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	return globular.GetClientContext(c), nil
}

// ─── RPC wrappers ─────────────────────────────────────────────────────────────

func (c *ClusterDoctorClient) GetClusterReport(ctx context.Context) (*cluster_doctorpb.ClusterReport, error) {
	return c.c.GetClusterReport(ctx, &cluster_doctorpb.ClusterReportRequest{})
}

func (c *ClusterDoctorClient) GetNodeReport(ctx context.Context, nodeID string) (*cluster_doctorpb.NodeReport, error) {
	return c.c.GetNodeReport(ctx, &cluster_doctorpb.NodeReportRequest{NodeId: nodeID})
}

func (c *ClusterDoctorClient) GetDriftReport(ctx context.Context, nodeID string) (*cluster_doctorpb.DriftReport, error) {
	return c.c.GetDriftReport(ctx, &cluster_doctorpb.DriftReportRequest{NodeId: nodeID})
}

func (c *ClusterDoctorClient) ExplainFinding(ctx context.Context, findingID string) (*cluster_doctorpb.FindingExplanation, error) {
	return c.c.ExplainFinding(ctx, &cluster_doctorpb.ExplainFindingRequest{FindingId: findingID})
}
