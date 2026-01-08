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

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type agentClient struct {
	endpoint string
	conn     *grpc.ClientConn
	client   nodeagentpb.NodeAgentServiceClient
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
		client:   nodeagentpb.NewNodeAgentServiceClient(conn),
	}
	client.touch()
	return client, nil
}

func (a *agentClient) ApplyPlan(ctx context.Context, plan *clustercontrollerpb.NodePlan, operationID string) error {
	if plan == nil {
		return nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req := &nodeagentpb.ApplyPlanRequest{Plan: plan}
	if strings.TrimSpace(operationID) != "" {
		req.OperationId = operationID
	}
	_, err := a.client.ApplyPlan(reqCtx, req)
	a.touch()
	return err
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
