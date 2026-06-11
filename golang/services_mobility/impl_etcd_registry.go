// @awareness namespace=globular.platform
// @awareness component=platform_services_mobility.etcd_registry
// @awareness file_role=production_service_registry_reads_globular_services_prefix
// @awareness risk=medium
package mobility

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdServiceRegistry implements ServiceRegistry against the live etcd
// store. It reads /globular/services/<uuid>/config records (the canonical
// service-registration location) and matches by the Name field.
//
// The Name accepted by InstancesOf can be either the fully-qualified
// protobuf service name ("ai_memory.AiMemoryService") OR a friendly
// short name ("ai-memory") which the registry normalizes. The friendly
// form is what operators type in the CLI; the canonical form is what
// etcd stores. Both must resolve to the same set of instances.
type EtcdServiceRegistry struct {
	Etcd       *clientv3.Client
	NodeIPToID map[string]string // optional: maps host or IP to a node-ID for cross-reference
}

// NewEtcdServiceRegistry constructs a registry backed by an existing
// authenticated etcd client.
func NewEtcdServiceRegistry(etcd *clientv3.Client) *EtcdServiceRegistry {
	return &EtcdServiceRegistry{Etcd: etcd, NodeIPToID: map[string]string{}}
}

// serviceConfig is the subset of the etcd-stored service config we care about.
type serviceConfig struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
	Port    int    `json:"Port"`
}

// InstancesOf returns the node IDs currently serving serviceName. The
// principle here is: the registry is the authority on "where is this
// service running" — we read what etcd says, we do NOT infer from
// other sources. If a service is registered but its host isn't in
// NodeIPToID, the entry is dropped from the returned set (we cannot
// orchestrate against a node we cannot identify).
func (r *EtcdServiceRegistry) InstancesOf(ctx context.Context, serviceName string) ([]string, error) {
	canonical := canonicalServiceName(serviceName)

	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := r.Etcd.Get(rctx, "/globular/services/", clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd list services: %w", err)
	}

	var nodeIDs []string
	seen := map[string]bool{}
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.HasSuffix(key, "/config") {
			continue
		}
		var cfg serviceConfig
		if err := json.Unmarshal(kv.Value, &cfg); err != nil {
			slog.Warn("services_mobility: InstancesOf corrupt service config record", "key", string(kv.Key), "error", err)
			continue
		}
		if !nameMatches(cfg.Name, serviceName, canonical) {
			continue
		}
		nodeID := r.resolveNodeID(cfg.Address)
		if nodeID == "" {
			continue
		}
		if !seen[nodeID] {
			seen[nodeID] = true
			nodeIDs = append(nodeIDs, nodeID)
		}
	}
	return nodeIDs, nil
}

// IsHealthy reports whether the named service is registered on the
// given node. This is the registration-presence proxy; a full health
// probe (calling the service's Health RPC) is follow-up work.
func (r *EtcdServiceRegistry) IsHealthy(ctx context.Context, nodeID, serviceName string) (bool, error) {
	instances, err := r.InstancesOf(ctx, serviceName)
	if err != nil {
		return false, err
	}
	for _, id := range instances {
		if id == nodeID {
			return true, nil
		}
	}
	return false, nil
}

// resolveNodeID maps a service Address (host:port or IP:port form) to
// the node ID that hosts it, using the configured NodeIPToID map. If
// the address is unrecognized the result is empty and the entry is
// dropped from registry lookups.
func (r *EtcdServiceRegistry) resolveNodeID(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	if id, ok := r.NodeIPToID[host]; ok {
		return id
	}
	return ""
}

// canonicalServiceName normalizes a friendly service name to its
// proto-style canonical form, so "ai-memory" and "ai_memory" and
// "ai_memory.AiMemoryService" all match the same etcd records.
func canonicalServiceName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "-", "_")
	if !strings.Contains(name, ".") {
		// Best-effort canonicalization for known shorthand. Operators
		// who use the fully-qualified proto name bypass this branch.
		switch name {
		case "ai_memory":
			return "ai_memory.aimemoryservice"
		}
	}
	return name
}

// nameMatches returns true if the stored Name in the etcd config
// matches any of the user-supplied forms.
func nameMatches(stored, raw, canonical string) bool {
	s := strings.ToLower(strings.TrimSpace(stored))
	if s == strings.ToLower(strings.TrimSpace(raw)) {
		return true
	}
	if s == canonical {
		return true
	}
	// Match the "Name." prefix in case stored is the qualified form
	// and the user passed just the package.
	if strings.HasPrefix(s, canonical+".") {
		return true
	}
	return false
}
