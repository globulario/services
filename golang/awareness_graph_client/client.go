// Package awareness_graph_client provides a thin gRPC client for the
// awareness-graph service. Address is resolved from the Envoy mesh gateway
// (config.GetMeshAddress) when no override is supplied. TLS uses the
// cluster's internal mTLS credentials by default; pass WithInsecure() for
// local plaintext dev instances.
package awareness_graph_client

import (
	"context"
	"fmt"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the awareness-graph gRPC stub.
type Client struct {
	conn grpc.ClientConnInterface
	stub awarenesspb.AwarenessGraphClient
}

// Option configures a Client.
type Option func(*options)

type options struct {
	insecure bool
}

// WithInsecure disables TLS. For local dev against a plaintext
// awareness-graph instance only.
func WithInsecure() Option {
	return func(o *options) { o.insecure = true }
}

// New dials the awareness-graph service and returns a Client.
// If addr is empty the gateway mesh address is resolved from etcd via
// config.GetMeshAddress; if that also fails the dial is attempted against
// localhost:7877 (the awareness-graph default port) as a last resort.
func New(addr string, opts ...Option) (*Client, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	if addr == "" {
		if mesh, err := config.GetMeshAddress(); err == nil {
			addr = mesh
		} else {
			addr = "localhost:7877"
		}
	}

	var dialOpts []grpc.DialOption
	if o.insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		internal, err := globular.InternalDialOptions()
		if err != nil {
			return nil, fmt.Errorf("awareness-graph client: internal TLS: %w", err)
		}
		dialOpts = append(dialOpts, internal...)
	}

	cc, err := grpc.Dial(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("awareness-graph client: dial %s: %w", addr, err)
	}
	return &Client{conn: cc, stub: awarenesspb.NewAwarenessGraphClient(cc)}, nil
}

// Close releases the underlying gRPC connection.
func (c *Client) Close() error {
	if cc, ok := c.conn.(*grpc.ClientConn); ok {
		return cc.Close()
	}
	return nil
}

// Briefing composes a prose briefing for a file or task.
func (c *Client) Briefing(ctx context.Context, file, task, depth string) (*awarenesspb.BriefingResponse, error) {
	return c.stub.Briefing(ctx, &awarenesspb.BriefingRequest{
		File:  file,
		Task:  task,
		Depth: depth,
	})
}

// Impact returns the structured anchor surface for a repo-relative file path.
func (c *Client) Impact(ctx context.Context, file string) (*awarenesspb.ImpactResponse, error) {
	return c.stub.Impact(ctx, &awarenesspb.ImpactRequest{File: file})
}

// Resolve fetches a single awareness node by class and bare id.
func (c *Client) Resolve(ctx context.Context, class, id string) (*awarenesspb.ResolveResponse, error) {
	return c.stub.Resolve(ctx, &awarenesspb.ResolveRequest{Class: class, Id: id})
}

// Query forwards a structured query to the awareness-graph service.
func (c *Client) Query(ctx context.Context, req *awarenesspb.QueryRequest) (*awarenesspb.QueryResponse, error) {
	return c.stub.Query(ctx, req)
}
