// Package awareness_graph_client is the gRPC client for the awareness-graph
// service. It follows the standard Globular service client pattern
// (echo_client / rbac_client / etc.): address resolution + TLS credentials
// come from globular.InitClient, the gRPC stub is reused across RPCs, and
// the client value implements the globular_client.Client interface so it
// can be obtained via globular_client.GetClient(...).
package awareness_graph_client

import (
	"context"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// awareness-graph Client Service
////////////////////////////////////////////////////////////////////////////////

type AwarenessGraph_Client struct {
	cc *grpc.ClientConn
	c  awarenesspb.AwarenessGraphClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The client domain
	domain string

	// keep the last connection state of the client.
	state string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	// The port
	port int

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string
}

// NewAwarenessGraphService_Client constructs a client using the standard
// Globular service-client init (address discovery + TLS credentials).
func NewAwarenessGraphService_Client(address string, id string) (*AwarenessGraph_Client, error) {
	client := new(AwarenessGraph_Client)
	if err := globular.InitClient(client, address, id); err != nil {
		return nil, err
	}
	if err := client.Reconnect(); err != nil {
		return nil, err
	}
	return client, nil
}

// Reconnect re-establishes the underlying gRPC connection. Called by the
// constructor and on transient failures.
func (client *AwarenessGraph_Client) Reconnect() error {
	var err error
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return err
	}
	client.c = awarenesspb.NewAwarenessGraphClient(client.cc)
	return nil
}

// Invoke routes a method through the globular client invoker.
func (client *AwarenessGraph_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

// GetCtx returns the standard outgoing context with the cluster auth token.
func (client *AwarenessGraph_Client) GetCtx() context.Context {
	return globular.GetClientContext(client)
}

// Close releases the underlying gRPC connection.
func (client *AwarenessGraph_Client) Close() {
	if client.cc != nil {
		_ = client.cc.Close()
	}
}

////////////////// Globular service-client interface accessors //////////////////

func (client *AwarenessGraph_Client) GetId() string             { return client.id }
func (client *AwarenessGraph_Client) GetName() string           { return client.name }
func (client *AwarenessGraph_Client) GetDomain() string         { return client.domain }
func (client *AwarenessGraph_Client) GetAddress() string        { return client.address }
func (client *AwarenessGraph_Client) GetPort() int              { return client.port }
func (client *AwarenessGraph_Client) GetMac() string            { return client.mac }
func (client *AwarenessGraph_Client) GetState() string          { return client.state }

func (client *AwarenessGraph_Client) SetId(id string)             { client.id = id }
func (client *AwarenessGraph_Client) SetName(name string)         { client.name = name }
func (client *AwarenessGraph_Client) SetDomain(domain string)     { client.domain = domain }
func (client *AwarenessGraph_Client) SetAddress(address string)   { client.address = address }
func (client *AwarenessGraph_Client) SetPort(port int)            { client.port = port }
func (client *AwarenessGraph_Client) SetMac(mac string)           { client.mac = mac }
func (client *AwarenessGraph_Client) SetState(state string)       { client.state = state }

////////////////// TLS ///////////////////

func (client *AwarenessGraph_Client) HasTLS() bool                { return client.hasTLS }
func (client *AwarenessGraph_Client) GetCertFile() string         { return client.certFile }
func (client *AwarenessGraph_Client) GetKeyFile() string          { return client.keyFile }
func (client *AwarenessGraph_Client) GetCaFile() string           { return client.caFile }
func (client *AwarenessGraph_Client) SetTLS(hasTLS bool)          { client.hasTLS = hasTLS }
func (client *AwarenessGraph_Client) SetCertFile(certFile string) { client.certFile = certFile }
func (client *AwarenessGraph_Client) SetKeyFile(keyFile string)   { client.keyFile = keyFile }
func (client *AwarenessGraph_Client) SetCaFile(caFile string)     { client.caFile = caFile }

// //////////////// Api //////////////////////

// Briefing composes a prose briefing for a file or task.
func (client *AwarenessGraph_Client) Briefing(ctx context.Context, file, task, depth string) (*awarenesspb.BriefingResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.Briefing(ctx, &awarenesspb.BriefingRequest{
		File:  file,
		Task:  task,
		Depth: depth,
	})
}

// Impact returns the structured anchor surface for a repo-relative file path.
func (client *AwarenessGraph_Client) Impact(ctx context.Context, file string) (*awarenesspb.ImpactResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.Impact(ctx, &awarenesspb.ImpactRequest{File: file})
}

// Resolve fetches a single awareness node by class and bare id.
func (client *AwarenessGraph_Client) Resolve(ctx context.Context, class, id string) (*awarenesspb.ResolveResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.Resolve(ctx, &awarenesspb.ResolveRequest{Class: class, Id: id})
}

// Query forwards a structured query to the awareness-graph service.
func (client *AwarenessGraph_Client) Query(ctx context.Context, req *awarenesspb.QueryRequest) (*awarenesspb.QueryResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.Query(ctx, req)
}

// Metadata returns graph-level coverage and freshness signals. Cheap to call;
// agents typically invoke once per session to interpret EMPTY briefings.
func (client *AwarenessGraph_Client) Metadata(ctx context.Context) (*awarenesspb.MetadataResponse, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return client.c.Metadata(ctx, &awarenesspb.MetadataRequest{})
}
