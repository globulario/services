// Package clusterstate/etcd provides a read-only etcd snapshot collector.
// It reads desired-service state from etcd and emits divergence edges when
// the desired version in etcd differs from the installed version already
// recorded in the awareness graph (from the varlib/receipt collector).
//
// Design:
//   - Accepts a ClientFactory function for dependency injection and testability.
//   - A nil factory (or factory returning error) causes graceful skip.
//   - Does not modify cluster state — read-only.
//   - Key namespaces read: /globular/resources/ServiceDesiredVersion/*
//                          /globular/nodes/*/packages/*/*

package clusterstate

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/awareness/graph"
)

// EtcdClientFactory returns a connected etcd client.
// Return (nil, nil) to skip the collector gracefully.
type EtcdClientFactory func() (*clientv3.Client, error)

// EtcdConnectError is returned by a ClientFactory when the etcd connection
// cannot be established (e.g. TLS certificates not found, no endpoints configured).
// CollectEtcd treats this as a graceful skip, not an error.
type EtcdConnectError struct {
	Reason string
}

func (e *EtcdConnectError) Error() string { return "etcd connect: " + e.Reason }

// desiredVersionRecord is the etcd JSON structure for ServiceDesiredVersion.
type desiredVersionRecord struct {
	Spec *struct {
		ServiceName string `json:"service_name"`
		Version     string `json:"version"`
		BuildID     string `json:"build_id"`
	} `json:"spec"`
}

// installedPackageRecord is the etcd JSON structure for an installed package.
type installedPackageRecord struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// CollectEtcd reads desired-service state from etcd and records divergence
// relative to the installed state already in the graph.
// A nil factory skips gracefully with status="skipped".
func CollectEtcd(ctx context.Context, g *graph.Graph, factory EtcdClientFactory) (CollectorHealth, error) {
	health := CollectorHealth{
		CollectorID: "etcd",
		Status:      "skipped",
	}

	if factory == nil {
		return health, nil
	}

	cli, err := factory()
	if err != nil || cli == nil {
		health.Status = "skipped"
		return health, nil
	}
	defer cli.Close()

	scanCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	collectedAt := time.Now().Unix()

	// ── Desired Services ─────────────────────────────────────────────────────
	desired, err := readDesiredServices(scanCtx, cli)
	if err != nil {
		health.Status = "error"
		health.Error = err.Error()
		return health, nil
	}

	for serviceName, rec := range desired {
		if rec.Spec == nil || rec.Spec.Version == "" {
			continue
		}

		// Emit an etcd desired-state node.
		etcdKey := "/globular/resources/ServiceDesiredVersion/" + serviceName
		meta := map[string]any{
			"desired_version": rec.Spec.Version,
			"source_tier":     "etcd_desired_state",
			"collected_at":    collectedAt,
		}
		if rec.Spec.BuildID != "" {
			meta["desired_build_id"] = rec.Spec.BuildID
		}
		n := graph.Node{
			ID:       "etcd:" + etcdKey,
			Type:     "etcd_desired_state",
			Name:     serviceName,
			Summary:  "desired version " + rec.Spec.Version,
			Metadata: meta,
		}
		if err := g.AddNode(ctx, n); err != nil {
			continue
		}
		health.NodesEmitted++

		// Link the etcd desired-state node to the package node.
		pkgID := "package:" + serviceName
		_ = g.AddEdge(ctx, graph.Edge{
			Src:  "etcd:" + etcdKey,
			Kind: graph.EdgeDefines,
			Dst:  pkgID,
		})

		// Detect version divergence against installed state in the graph.
		detectDesiredInstalledDrift(ctx, g, serviceName, rec.Spec.Version, etcdKey)
	}

	// ── Installed Packages via etcd ──────────────────────────────────────────
	installed, err := readInstalledPackages(scanCtx, cli)
	if err != nil {
		// Non-fatal — log in health but don't fail the collector.
		health.Status = "ok"
		health.Error = "installed packages read partial: " + err.Error()
		return health, nil
	}

	for nodeID, pkgs := range installed {
		for _, pkg := range pkgs {
			if pkg.Name == "" || pkg.Version == "" {
				continue
			}
			// Emit installed-state node scoped to the cluster node.
			nodePrefix := "node:" + nodeID + "/installed/" + pkg.Kind
			n := graph.Node{
				ID:      nodePrefix + ":" + pkg.Name,
				Type:    "installed_package",
				Name:    pkg.Name,
				Summary: "installed " + pkg.Version + " on " + nodeID,
				Metadata: map[string]any{
					"version":      pkg.Version,
					"kind":         pkg.Kind,
					"node_id":      nodeID,
					"source_tier":  "etcd_desired_state",
					"collected_at": collectedAt,
				},
			}
			if err := g.AddNode(ctx, n); err != nil {
				continue
			}
			health.NodesEmitted++

			// Link to package.
			_ = g.AddEdge(ctx, graph.Edge{
				Src:  nodePrefix + ":" + pkg.Name,
				Kind: graph.EdgeCurrentStatusOf,
				Dst:  "package:" + pkg.Name,
			})
		}
	}

	health.Status = "ok"
	return health, nil
}

// readDesiredServices fetches all ServiceDesiredVersion keys from etcd.
func readDesiredServices(ctx context.Context, cli *clientv3.Client) (map[string]desiredVersionRecord, error) {
	prefix := "/globular/resources/ServiceDesiredVersion/"
	resp, err := cli.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithLimit(500))
	if err != nil {
		return nil, err
	}
	out := make(map[string]desiredVersionRecord, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		name := strings.TrimPrefix(key, prefix)
		if name == "" {
			continue
		}
		var rec desiredVersionRecord
		if err := json.Unmarshal(kv.Value, &rec); err != nil {
			continue
		}
		out[name] = rec
	}
	return out, nil
}

// readInstalledPackages fetches installed package entries from etcd node records.
func readInstalledPackages(ctx context.Context, cli *clientv3.Client) (map[string][]installedPackageRecord, error) {
	prefix := "/globular/nodes/"
	resp, err := cli.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithLimit(2000))
	if err != nil {
		return nil, err
	}
	out := make(map[string][]installedPackageRecord)
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		// Expect: /globular/nodes/<nodeID>/packages/<kind>/<name>
		parts := strings.SplitN(strings.TrimPrefix(key, prefix), "/", 4)
		if len(parts) != 4 || parts[1] != "packages" {
			continue
		}
		nodeID := parts[0]
		kind := parts[2]
		name := parts[3]
		if name == "" {
			continue
		}
		var rec installedPackageRecord
		_ = json.Unmarshal(kv.Value, &rec)
		if rec.Name == "" {
			rec.Name = name
		}
		if rec.Kind == "" {
			rec.Kind = kind
		}
		out[nodeID] = append(out[nodeID], rec)
	}
	return out, nil
}

// detectDesiredInstalledDrift looks up the installed version of serviceName
// in the graph (as emitted by the receipt/varlib collector) and emits a
// HasStateDelta edge when the versions differ.
func detectDesiredInstalledDrift(ctx context.Context, g *graph.Graph, serviceName, desiredVersion, etcdKey string) {
	receiptID := "receipt:" + serviceName
	receiptNode, err := g.FindNode(ctx, receiptID)
	if err != nil || receiptNode == nil {
		return
	}
	installedVersion, _ := receiptNode.Metadata["version"].(string)
	if installedVersion == "" || installedVersion == desiredVersion {
		return
	}
	// Emit a state-delta edge marking the divergence.
	_ = g.AddEdge(ctx, graph.Edge{
		Src:  "etcd:" + etcdKey,
		Kind: graph.EdgeHasStateDelta,
		Dst:  receiptID,
		Metadata: map[string]any{
			"desired_version":   desiredVersion,
			"installed_version": installedVersion,
			"drift":             true,
		},
	})
}
