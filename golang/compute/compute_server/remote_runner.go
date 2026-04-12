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
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/globulario/services/golang/compute/compute_runnerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	clientv3 "go.etcd.io/etcd/client/v3"
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

// resolveComputeEndpoints returns the DIRECT (non-mesh) endpoints for all
// compute service instances. The ComputeRunnerService is not routed through
// the Envoy mesh, so we need the actual service port, not :443.
func resolveComputeEndpoints() []string {
	svcs, err := config.GetServicesConfigurationsByName(computeServiceName)
	if err != nil || len(svcs) == 0 {
		slog.Warn("compute: no compute service instances found via etcd")
		return nil
	}
	var addrs []string
	for _, s := range svcs {
		// Address is the unique per-node routable endpoint (e.g. "10.0.0.20:10300").
		// Domain is the shared cluster DNS name — not useful for distinct placement.
		if addr, ok := s["Address"].(string); ok && addr != "" {
			addrs = append(addrs, addr)
			continue
		}
		// Fallback: construct from Domain + Port (all nodes share Domain, so
		// this collapses — only used when Address is missing).
		host, _ := s["Domain"].(string)
		port := 0
		switch v := s["Port"].(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		}
		if host != "" && port > 0 {
			addrs = append(addrs, fmt.Sprintf("%s:%d", host, port))
		}
	}
	if len(addrs) == 0 {
		slog.Warn("compute: no compute service endpoints resolved from etcd")
	}
	return addrs
}

// computeNodeInfo holds a resolved compute node's identity and endpoint.
type computeNodeInfo struct {
	Address  string   // e.g. "10.0.0.20:10300"
	NodeID   string   // service instance ID
	Mac      string   // node MAC for identification
	Hostname string   // node hostname
	Profiles []string // node profiles (core, compute, storage, etc.)
}

// resolveComputeNodes returns detailed node info for all compute instances,
// enriched with node profiles from the cluster controller for placement filtering.
func resolveComputeNodes() []computeNodeInfo {
	svcs, err := config.GetServicesConfigurationsByName(computeServiceName)
	if err != nil || len(svcs) == 0 {
		return nil
	}

	// Build node info from service configs.
	var nodes []computeNodeInfo
	for _, s := range svcs {
		addr, _ := s["Address"].(string)
		if addr == "" {
			continue
		}
		nodeID, _ := s["Id"].(string)
		mac, _ := s["Mac"].(string)
		nodes = append(nodes, computeNodeInfo{
			Address: addr,
			NodeID:  nodeID,
			Mac:     mac,
		})
	}

	// Enrich with node profiles from etcd cluster state (best-effort).
	enrichNodeProfiles(nodes)
	return nodes
}

// enrichNodeProfiles reads node profiles from etcd and matches them
// to compute instances by IP address.
func enrichNodeProfiles(nodes []computeNodeInfo) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, "/globular/nodes/", clientv3.WithPrefix())
	if err != nil {
		return
	}

	// Build IP → profiles map from node records.
	type nodeRecord struct {
		Hostname string   `json:"hostname"`
		Profiles []string `json:"profiles"`
		IPs      []string `json:"ips"`
	}
	ipProfiles := map[string]*nodeRecord{}
	for _, kv := range resp.Kvs {
		var rec nodeRecord
		if json.Unmarshal(kv.Value, &rec) != nil {
			continue
		}
		for _, ip := range rec.IPs {
			ipProfiles[ip] = &rec
		}
	}

	// Match compute nodes by extracting IP from their address.
	for i := range nodes {
		host, _, _ := net.SplitHostPort(nodes[i].Address)
		if rec, ok := ipProfiles[host]; ok {
			nodes[i].Profiles = rec.Profiles
			nodes[i].Hostname = rec.Hostname
		}
	}
}

// filterByProfiles returns only nodes that have at least one of the required profiles.
func filterByProfiles(nodes []computeNodeInfo, required []string) []computeNodeInfo {
	if len(required) == 0 {
		return nodes
	}
	var out []computeNodeInfo
	for _, n := range nodes {
		for _, req := range required {
			for _, p := range n.Profiles {
				if p == req {
					out = append(out, n)
					goto next
				}
			}
		}
	next:
	}
	return out
}

// runnerTokenAuth implements grpc.PerRPCCredentials for bearer token auth.
type runnerTokenAuth struct {
	token string
}

func (t runnerTokenAuth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"token": t.token}, nil
}

func (t runnerTokenAuth) RequireTransportSecurity() bool { return false }
