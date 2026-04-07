package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type agentClient struct {
	endpoint string
	conn     *grpc.ClientConn
	client   node_agentpb.NodeAgentServiceClient
	mu       sync.Mutex
	lastUsed time.Time
}

func newAgentClient(ctx context.Context, endpoint string, insecureEnabled bool, caPath, serverNameOverride string) (*agentClient, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}
	// One canonical resolution step: loopback rewrite + SNI extraction.
	target := config.ResolveDialTarget(endpoint)
	serverName := serverNameOverride
	if serverName == "" {
		serverName = target.ServerName
	}
	if insecureEnabled {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		if caPath == "" {
			return nil, fmt.Errorf("agent CA path must be provided when using secure agent gRPC")
		}
		pool, err := loadCertPool(caPath)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(pool, serverName)))
	}
	// Inject node token so calls are authenticated on the remote node-agent.
	if token := loadControllerToken(); token != "" {
		opts = append(opts, grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{},
			cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				md = metadata.New(nil)
			} else {
				md = md.Copy()
			}
			md.Set("token", token)
			return invoker(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, callOpts...)
		}))
	}
	conn, err := grpc.DialContext(dialCtx, target.Address, opts...)
	if err != nil {
		return nil, err
	}
	client := &agentClient{
		endpoint: target.Address,
		conn:     conn,
		client:   node_agentpb.NewNodeAgentServiceClient(conn),
	}
	client.touch()
	return client, nil
}

// GetInventory retrieves the node's inventory, used to verify connectivity.
func (a *agentClient) GetInventory(ctx context.Context) (*node_agentpb.GetInventoryResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := a.client.GetInventory(reqCtx, &node_agentpb.GetInventoryRequest{})
	if err != nil {
		return nil, err
	}
	a.touch()
	return resp, nil
}

// ControlService sends a restart/stop/start/status command to the node agent.
func (a *agentClient) ControlService(ctx context.Context, unit, action string) (*node_agentpb.ControlServiceResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := a.client.ControlService(reqCtx, &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: action,
	})
	if err != nil {
		return nil, err
	}
	a.touch()
	return resp, nil
}

func (a *agentClient) touch() {
	a.mu.Lock()
	a.lastUsed = time.Now()
	a.mu.Unlock()
}

func (a *agentClient) idleDuration() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Since(a.lastUsed)
}

func (a *agentClient) Close() error {
	if a.conn == nil {
		return nil
	}
	return a.conn.Close()
}

func loadCertPool(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA %s: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("failed to parse CA %s", path)
	}
	return pool, nil
}
