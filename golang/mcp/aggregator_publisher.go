package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ── MCP node registry writer ──────────────────────────────────────────────────
//
// Each MCP server publishes its own endpoint metadata to etcd at
// /globular/mcp/nodes/<node-id>. The aggregator's loadMCPNodeOverrides reads
// this prefix when listing nodes so the cluster can find each node's true
// MCP HTTP port even when it differs from the canonical aggregatorMCPPort.
//
// Lifecycle:
//   - Called once after the HTTP server begins listening.
//   - The writer attaches a lease (TTL 60s) and refreshes the entry every 30s.
//   - On shutdown the lease expires automatically (no explicit delete needed).
//
// Failure mode:
//   - If etcd is unreachable, we log and continue. The aggregator will still
//     find this node via the controller-derived URL using aggregatorMCPPort
//     (the canonical default).

const (
	mcpRegistryLeaseTTL  = 60 // seconds
	mcpRegistryRefresh   = 30 * time.Second
	mcpRegistryDeadline  = 5 * time.Second
	mcpAwarenessBundleSymlink = "/var/lib/globular/awareness/current"
)

// publishMCPNodeRegistry begins publishing this node's MCP endpoint metadata
// to etcd. It runs in its own goroutine and exits when ctx is cancelled.
//
// scheme is "http" or "https"; advertiseHost is the IP or hostname operators
// should use to reach this MCP server; port is the actual listen port.
func publishMCPNodeRegistry(ctx context.Context, scheme, advertiseHost string, port int) {
	go func() {
		ticker := time.NewTicker(mcpRegistryRefresh)
		defer ticker.Stop()

		// Publish immediately on startup, then on each tick.
		if err := writeMCPRegistryEntry(ctx, scheme, advertiseHost, port); err != nil {
			log.Printf("mcp registry: initial publish failed: %v", err)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := writeMCPRegistryEntry(ctx, scheme, advertiseHost, port); err != nil {
					log.Printf("mcp registry: refresh failed: %v", err)
				}
			}
		}
	}()
}

// writeMCPRegistryEntry produces a single MCPNodeEntry and writes it to etcd
// under a lease so it self-expires if the server goes away.
func writeMCPRegistryEntry(parent context.Context, scheme, advertiseHost string, port int) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, mcpRegistryDeadline)
	defer cancel()

	nodeID, _ := config.GetName()
	hostname, _ := config.GetHostname()
	ip := advertiseHost
	if ip == "" {
		ip = config.GetRoutableIPv4()
	}

	mcpURL := buildMCPURL(ip, port)
	if scheme == "http" {
		// Override scheme; buildMCPURL always emits https.
		mcpURL = fmt.Sprintf("http://%s:%d", ip, port)
	}

	entry := MCPNodeEntry{
		NodeID:                 nodeID,
		Hostname:               hostname,
		IP:                     ip,
		MCPURL:                 mcpURL,
		MCPPort:                port,
		ClusterID:              readClusterID(),
		ReleaseVersion:         readReleaseVersion(),
		AwarenessBundleVersion: readAwarenessBundleVersion(),
		LastSeen:               time.Now().UTC(),
		Status:                 "RUNNING",
	}

	value, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Lease auto-expires the entry if this MCP server crashes without unregistering.
	lease, err := cli.Grant(ctx, mcpRegistryLeaseTTL)
	if err != nil {
		return err
	}

	key := etcdMCPNodePrefix + nodeID
	if _, err := cli.Put(ctx, key, string(value), clientv3.WithLease(lease.ID)); err != nil {
		return err
	}
	return nil
}

// readClusterID returns the cluster identifier from local config or empty.
func readClusterID() string {
	lc, err := config.GetLocalConfig(true)
	if err != nil {
		return ""
	}
	if v, ok := lc["ClusterId"].(string); ok {
		return v
	}
	if v, ok := lc["Cluster"].(string); ok {
		return v
	}
	return ""
}

// readReleaseVersion reads the platform release version from /var/lib/globular/release-index.json
// when present. Returns "" if not available — we never block on this.
func readReleaseVersion() string {
	const releaseIndexPath = "/var/lib/globular/release-index.json"
	data, err := os.ReadFile(releaseIndexPath)
	if err != nil {
		return ""
	}
	var idx struct {
		Version string `json:"version"`
		Release string `json:"release"`
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return ""
	}
	if idx.Version != "" {
		return idx.Version
	}
	return idx.Release
}

// readAwarenessBundleVersion resolves the awareness bundle version by reading
// /var/lib/globular/awareness/current/manifest.json (when the bundle symlink exists).
// Falls back to the resolved symlink directory name when no manifest is present.
func readAwarenessBundleVersion() string {
	manifestPath := filepath.Join(mcpAwarenessBundleSymlink, "manifest.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var m struct {
			Version string `json:"version"`
			Release string `json:"release"`
		}
		if err := json.Unmarshal(data, &m); err == nil {
			if m.Version != "" {
				return m.Version
			}
			if m.Release != "" {
				return m.Release
			}
		}
	}
	if target, err := os.Readlink(mcpAwarenessBundleSymlink); err == nil {
		return strings.TrimSpace(filepath.Base(target))
	}
	return ""
}

