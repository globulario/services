package main

// leader_proxy.go — Transparent leader forwarding for write RPCs.
//
// When a non-leader controller receives a write RPC (UpsertDesiredService,
// RemoveDesiredService, etc.), instead of returning "not leader" to the client,
// it forwards the request to the current leader and returns the leader's response.
//
// This makes the cluster transparent: clients connect to any node via
// globular.internal and writes always reach the leader without client-side
// retries or hardcoded addresses.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// leaderProxy manages a cached gRPC connection to the current leader.
type leaderProxy struct {
	mu   sync.Mutex
	conn *grpc.ClientConn
	addr string
}

var proxy = &leaderProxy{}

// getLeaderClient returns a gRPC client connected to the current leader.
// Caches the connection and reconnects if the leader address changes.
func (srv *server) getLeaderClient(ctx context.Context) (cluster_controllerpb.ClusterControllerServiceClient, error) {
	addr, _ := srv.leaderAddr.Load().(string)
	if addr == "" {
		if srv.kv != nil {
			if resp, err := srv.kv.Get(ctx, leaderElectionPrefix+"/addr"); err == nil && resp != nil && len(resp.Kvs) > 0 {
				addr = string(resp.Kvs[0].Value)
				srv.leaderAddr.Store(addr)
			}
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("leader address unknown")
	}

	proxy.mu.Lock()
	defer proxy.mu.Unlock()

	if proxy.conn != nil && proxy.addr == addr {
		return cluster_controllerpb.NewClusterControllerServiceClient(proxy.conn), nil
	}

	if proxy.conn != nil {
		proxy.conn.Close()
		proxy.conn = nil
	}

	tlsCfg, err := leaderProxyTLS()
	if err != nil {
		return nil, fmt.Errorf("leader proxy TLS: %w", err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("leader proxy dial %s: %w", addr, err)
	}

	proxy.conn = conn
	proxy.addr = addr
	slog.Info("leader-proxy: connected to leader", "addr", addr)
	return cluster_controllerpb.NewClusterControllerServiceClient(conn), nil
}

// leaderProxyTLS builds a TLS config using the service certificate and CA.
func leaderProxyTLS() (*tls.Config, error) {
	caPath := config.GetCACertificatePath()
	caData, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("parse CA cert")
	}

	svcDir := filepath.Join(config.GetStateRootDir(), "pki", "issued", "services")
	cert, err := tls.LoadX509KeyPair(
		filepath.Join(svcDir, "service.crt"),
		filepath.Join(svcDir, "service.key"),
	)
	if err != nil {
		return nil, fmt.Errorf("load service cert: %w", err)
	}

	return &tls.Config{
		RootCAs:      caPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// forwardUpsertDesiredService proxies UpsertDesiredService to the leader.
func (srv *server) forwardUpsertDesiredService(ctx context.Context, req *cluster_controllerpb.UpsertDesiredServiceRequest) (*cluster_controllerpb.DesiredState, error) {
	client, err := srv.getLeaderClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("leader proxy: %w", err)
	}
	slog.Debug("leader-proxy: forwarding UpsertDesiredService")
	return client.UpsertDesiredService(ctx, req)
}

// forwardRemoveDesiredService proxies RemoveDesiredService to the leader.
func (srv *server) forwardRemoveDesiredService(ctx context.Context, req *cluster_controllerpb.RemoveDesiredServiceRequest) (*cluster_controllerpb.DesiredState, error) {
	client, err := srv.getLeaderClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("leader proxy: %w", err)
	}
	slog.Debug("leader-proxy: forwarding RemoveDesiredService")
	return client.RemoveDesiredService(ctx, req)
}
