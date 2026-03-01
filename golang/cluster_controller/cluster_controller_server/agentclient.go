package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	planpb "github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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
	serverName := serverNameOverride
	if serverName == "" {
		serverName = endpoint
		if host, _, err := net.SplitHostPort(endpoint); err == nil {
			serverName = host
		}
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
	conn, err := grpc.DialContext(dialCtx, endpoint, opts...)
	if err != nil {
		return nil, err
	}
	client := &agentClient{
		endpoint: endpoint,
		conn:     conn,
		client:   node_agentpb.NewNodeAgentServiceClient(conn),
	}
	client.touch()
	return client, nil
}

func (a *agentClient) ApplyPlan(ctx context.Context, plan *cluster_controllerpb.NodePlan, operationID string) error {
	if plan == nil {
		return nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req := &node_agentpb.ApplyPlanRequest{Plan: plan}
	if strings.TrimSpace(operationID) != "" {
		req.OperationId = operationID
	}
	_, err := a.client.ApplyPlan(reqCtx, req)
	a.touch()
	return err
}

// ApplyPlanV1 submits a V1 plan to the node agent.
func (a *agentClient) ApplyPlanV1(ctx context.Context, plan *planpb.NodePlan, operationID string) error {
	if plan == nil {
		return fmt.Errorf("plan is required")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req := &node_agentpb.ApplyPlanV1Request{Plan: plan}
	if strings.TrimSpace(operationID) != "" {
		req.OperationId = operationID
	}
	_, err := a.client.ApplyPlanV1(reqCtx, req)
	a.touch()
	return err
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
