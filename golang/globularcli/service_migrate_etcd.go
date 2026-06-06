// @awareness namespace=globular.platform
// @awareness component=platform_globularcli.service_migrate_node_loader
// @awareness file_role=cli_helper_loads_node_records_via_cluster_controller_listnodes
// @awareness risk=low
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	mobility "github.com/globulario/services/golang/services_mobility"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// loadClusterNodesEtcd loads the cluster's node-records from the
// cluster_controller via its typed ListNodes RPC. The name retains the
// "Etcd" suffix from the original intent (read identity directly from
// etcd) — but in current Globular the node identity is held in-memory
// at the controller and exposed only via ListNodes. The function does
// still touch etcd to resolve the controller's gRPC endpoint.
//
// The orchestration steps below this function only touch node-agents
// (start/stop/list-packages); cluster_controller does not participate
// in the migration itself. We only need it once, here, to resolve the
// topology.
func loadClusterNodesEtcd(ctx context.Context, etcd *clientv3.Client) (map[string]string, []mobility.NodeRecord, error) {
	endpoint, err := resolveControllerEndpoint(ctx, etcd)
	if err != nil {
		return nil, nil, err
	}
	creds, err := loadServiceCreds()
	if err != nil {
		return nil, nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, endpoint,
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()

	client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	callCtx, callCancel := context.WithTimeout(ctx, 10*time.Second)
	defer callCancel()
	resp, err := client.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return nil, nil, err
	}

	ipToID := map[string]string{}
	var records []mobility.NodeRecord
	for _, n := range resp.GetNodes() {
		nodeID := n.GetNodeId()
		if nodeID == "" {
			continue
		}
		agentEndpoint := n.GetAgentEndpoint()
		records = append(records, mobility.NodeRecord{
			NodeID:        nodeID,
			AgentEndpoint: agentEndpoint,
		})
		if agentEndpoint != "" {
			if host, _, splitErr := net.SplitHostPort(agentEndpoint); splitErr == nil && host != "" {
				ipToID[host] = nodeID
			}
		}
		if id := n.GetIdentity(); id != nil {
			if hostname := id.GetHostname(); hostname != "" {
				ipToID[hostname] = nodeID
			}
		}
	}
	return ipToID, records, nil
}

// resolveControllerEndpoint scans /globular/services/<uuid>/config for
// the cluster controller's registration and returns its host:port.
func resolveControllerEndpoint(ctx context.Context, etcd *clientv3.Client) (string, error) {
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := etcd.Get(rctx, "/globular/services/", clientv3.WithPrefix())
	if err != nil {
		return "", err
	}
	type svcConfig struct {
		Name    string `json:"Name"`
		Address string `json:"Address"`
		Port    int    `json:"Port"`
	}
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.HasSuffix(key, "/config") {
			continue
		}
		var cfg svcConfig
		if err := json.Unmarshal(kv.Value, &cfg); err != nil {
			continue
		}
		name := strings.ToLower(cfg.Name)
		if strings.Contains(name, "clustercontroller") || strings.Contains(name, "cluster_controller") {
			if cfg.Address == "" {
				continue
			}
			// Address stored in etcd is host-only or host:port; if no
			// port suffix, append Port.
			if _, _, splitErr := net.SplitHostPort(cfg.Address); splitErr != nil && cfg.Port > 0 {
				return cfg.Address + ":" + itoa(cfg.Port), nil
			}
			return cfg.Address, nil
		}
	}
	return "", errors.New("cluster_controller service registration not found in etcd")
}

// itoa is a tiny inline helper to avoid pulling strconv in for one use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// loadServiceCreds builds a TLS config using the cluster's standard
// service certificate. The CLI runs with those creds when invoked by
// an operator on a cluster node; for off-node invocation, the operator
// must export GLOBULAR_PKI_* env vars pointing at a valid client cert.
func loadServiceCreds() (credentials.TransportCredentials, error) {
	caPath := envOr("GLOBULAR_PKI_CA", "/var/lib/globular/pki/ca.crt")
	certPath := envOr("GLOBULAR_PKI_CERT", "/var/lib/globular/pki/issued/services/service.crt")
	keyPath := envOr("GLOBULAR_PKI_KEY", "/var/lib/globular/pki/issued/services/service.key")

	caBytes, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, errors.New("parse CA bundle")
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}), nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
