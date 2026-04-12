// remote_runner.go provides gRPC client helpers for calling the ComputeRunnerService
// on remote nodes. Used by workflow action handlers to dispatch staging and execution
// to the node chosen by computeChooseNode.
//
// Address resolution uses etcd (config.ResolveServiceAddrs) — no hardcoded ports.
// TLS uses the cluster service certificate + CA trust chain.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/globulario/services/golang/compute/compute_runnerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// computeServiceName is used for etcd-based service discovery.
	// All compute instances register under this name; the runner RPC
	// is served on the same port.
	computeServiceName = "compute.ComputeService"
)

// dialComputeRunner creates a gRPC connection to the ComputeRunnerService
// at the given endpoint (host:port from etcd). Uses mTLS with the cluster
// service certificate and an optional bearer token.
func dialComputeRunner(endpoint string) (*grpc.ClientConn, error) {
	dt := config.ResolveDialTarget(endpoint)
	if dt.Address == "" {
		return nil, fmt.Errorf("empty endpoint after resolution")
	}

	certFile := "/var/lib/globular/pki/issued/services/service.crt"
	keyFile := "/var/lib/globular/pki/issued/services/service.key"
	caFile := "/var/lib/globular/pki/ca.crt"

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   dt.ServerName,
	})

	token, _ := security.GetLocalToken("")

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}
	if token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(runnerTokenAuth{token: token}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return grpc.DialContext(ctx, dt.Address, opts...)
}

// runnerClient dials the compute runner at endpoint and returns a typed client.
// The caller must close the returned connection when done.
func runnerClient(endpoint string) (compute_runnerpb.ComputeRunnerServiceClient, *grpc.ClientConn, error) {
	conn, err := dialComputeRunner(endpoint)
	if err != nil {
		return nil, nil, err
	}
	return compute_runnerpb.NewComputeRunnerServiceClient(conn), conn, nil
}

// resolveComputeEndpoints returns all registered compute service endpoints
// from etcd. Each endpoint serves both ComputeService and ComputeRunnerService.
func resolveComputeEndpoints() []string {
	addrs := config.ResolveServiceAddrs(computeServiceName)
	if len(addrs) == 0 {
		slog.Warn("compute: no compute service instances found via service discovery")
	}
	return addrs
}

// runnerTokenAuth implements grpc.PerRPCCredentials for bearer token auth.
type runnerTokenAuth struct {
	token string
}

func (t runnerTokenAuth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"token": t.token}, nil
}

func (t runnerTokenAuth) RequireTransportSecurity() bool { return false }
