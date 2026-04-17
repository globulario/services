package main

// leader_proxy.go — Transparent leader forwarding for write RPCs.
//
// When a non-leader controller receives a write RPC, instead of returning
// "not leader" to the client, requireLeaderOrForward returns a gRPC connection
// to the leader so the handler can forward the request transparently.
//
// This makes the cluster transparent: clients connect to any node via
// globular.internal and writes always reach the leader.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type leaderProxy struct {
	mu   sync.Mutex
	conn *grpc.ClientConn
	addr string
}

var proxy = &leaderProxy{}

func (srv *server) getLeaderConn(ctx context.Context) (*grpc.ClientConn, error) {
	addr, _ := srv.leaderAddr.Load().(string)
	if addr == "" && srv.kv != nil {
		if resp, err := srv.kv.Get(ctx, leaderElectionPrefix+"/addr"); err == nil && resp != nil && len(resp.Kvs) > 0 {
			addr = string(resp.Kvs[0].Value)
			srv.leaderAddr.Store(addr)
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("leader address unknown")
	}

	proxy.mu.Lock()
	defer proxy.mu.Unlock()

	if proxy.conn != nil && proxy.addr == addr {
		return proxy.conn, nil
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
	return conn, nil
}

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

// leaderForward forwards a gRPC call to the leader using the full method name.
// req must be the original request proto, resp must be a zero-value proto of
// the correct response type. Incoming auth metadata is propagated.
func (srv *server) leaderForward(ctx context.Context, method string, req, resp interface{}) error {
	conn, err := srv.getLeaderConn(ctx)
	if err != nil {
		// Fall back to returning the standard "not leader" error.
		addr, _ := srv.leaderAddr.Load().(string)
		return status.Errorf(codes.FailedPrecondition,
			"not leader (leader_addr=%s, epoch=%d)", addr, srv.leaderEpoch.Load())
	}

	// Propagate incoming auth metadata to the leader.
	inMD, _ := metadata.FromIncomingContext(ctx)
	outCtx := metadata.NewOutgoingContext(ctx, inMD)

	if err := conn.Invoke(outCtx, method, req, resp); err != nil {
		return err
	}
	slog.Debug("leader-proxy: forwarded", "method", method[strings.LastIndex(method, "/")+1:])
	return nil
}
