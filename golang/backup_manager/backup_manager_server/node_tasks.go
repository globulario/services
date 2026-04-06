package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ProviderScope classifies whether a provider runs once (CLUSTER) or per-node.
type ProviderScope int

const (
	ProviderScopeCluster ProviderScope = iota // run once on the leader
	ProviderScopeNode                         // run on every node
)

const (
	nodeAgentDefaultPort = 11000
	nodeAgentPollInterval = 3 * time.Second
)

// providerScope returns the execution scope for a provider type.
func providerScope(t backup_managerpb.BackupProviderType) ProviderScope {
	switch t {
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
		return ProviderScopeNode
	default:
		// etcd, scylla, minio are cluster-scoped (run once)
		return ProviderScopeCluster
	}
}

// NodeResult captures the result of running a provider on a specific node.
type NodeResult struct {
	NodeID   string
	Hostname string
	Result   *backup_managerpb.BackupProviderResult
}

// NodeCoverage records which nodes succeeded/failed for per-node providers.
type NodeCoverage struct {
	Provider string           `json:"provider"`
	Nodes    []NodeCoverageEntry `json:"nodes"`
}

// NodeCoverageEntry records the result for a single node.
type NodeCoverageEntry struct {
	NodeID   string `json:"node_id"`
	Hostname string `json:"hostname"`
	Ok       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// runProviderOnAllNodes fans out a node-scoped provider to all nodes in the topology.
// Local nodes run the provider directly; remote nodes are executed via node-agent RPC.
// Each node's results are stored in per-node capsule subdirectories.
func (srv *server) runProviderOnAllNodes(
	ctx context.Context,
	spec *backup_managerpb.BackupProviderSpec,
	backupID string,
	nodes []TopologyNode,
) (*backup_managerpb.BackupProviderResult, *NodeCoverage) {

	name := providerName(spec.Type)
	coverage := &NodeCoverage{Provider: name}

	if len(nodes) == 0 {
		slog.Warn("no nodes available for per-node provider", "provider", name)
		return failResult(spec.Type, "no nodes available for fan-out", nil), coverage
	}

	var mu sync.Mutex
	var allResults []NodeResult
	var wg sync.WaitGroup

	for _, node := range nodes {
		wg.Add(1)
		go func(n TopologyNode) {
			defer wg.Done()

			isLocal := srv.isLocalNode(n)

			var result *backup_managerpb.BackupProviderResult
			if isLocal {
				// Run locally with per-node capsule paths
				cc, err := srv.NewNodeCapsuleContext(backupID, name, n.NodeID)
				if err != nil {
					result = failResult(spec.Type, fmt.Sprintf("create node capsule: %v", err), nil)
				} else {
					result = srv.runProvider(ctx, spec, cc)
				}
			} else {
				// Run on remote node via node-agent RPC
				result = srv.runProviderOnRemoteNode(ctx, spec, backupID, name, n)
			}

			// Tag result with node info
			if result.Outputs == nil {
				result.Outputs = make(map[string]string)
			}
			result.Outputs["node_id"] = n.NodeID
			result.Outputs["node_hostname"] = n.Hostname

			mu.Lock()
			allResults = append(allResults, NodeResult{
				NodeID:   n.NodeID,
				Hostname: n.Hostname,
				Result:   result,
			})
			mu.Unlock()
		}(node)
	}

	wg.Wait()

	// Build coverage and aggregate results
	allOk := true
	var aggregatedOutputs = make(map[string]string)
	var snapshotIDs []string
	var totalBytes uint64

	for _, nr := range allResults {
		ok := nr.Result.State == backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
		entry := NodeCoverageEntry{
			NodeID:   nr.NodeID,
			Hostname: nr.Hostname,
			Ok:       ok,
		}
		if !ok {
			entry.Error = nr.Result.ErrorMessage
			allOk = false
		}
		coverage.Nodes = append(coverage.Nodes, entry)
		totalBytes += nr.Result.BytesWritten

		// Collect per-node snapshot IDs
		if snapID, exists := nr.Result.Outputs["snapshot_id"]; exists && snapID != "" {
			snapshotIDs = append(snapshotIDs, nr.NodeID+":"+snapID)
			aggregatedOutputs["snapshot_id_"+nr.NodeID] = snapID
		}
	}

	// Set top-level snapshot_id from the first (or only) node so validation can find it
	if len(snapshotIDs) > 0 {
		// Use the first node's snapshot_id as the canonical one
		parts := strings.SplitN(snapshotIDs[0], ":", 2)
		if len(parts) == 2 {
			aggregatedOutputs["snapshot_id"] = parts[1]
		}
	}

	aggregatedOutputs["node_count"] = fmt.Sprintf("%d", len(allResults))
	aggregatedOutputs["method"] = "per-node fan-out"

	state := backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
	severity := backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO
	summary := fmt.Sprintf("%s completed on %d/%d nodes", name, len(coverage.Nodes), len(nodes))
	if !allOk {
		state = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		severity = backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR
		summary = fmt.Sprintf("%s failed on some nodes", name)
	}

	return &backup_managerpb.BackupProviderResult{
		Type:         spec.Type,
		Enabled:      true,
		State:        state,
		Severity:     severity,
		Summary:      summary,
		Outputs:      aggregatedOutputs,
		BytesWritten: totalBytes,
	}, coverage
}

// runProviderOnRemoteNode executes a backup provider on a remote node via node-agent RPC.
// It calls RunBackupProvider to start the task, then polls GetBackupTaskResult until done.
func (srv *server) runProviderOnRemoteNode(
	ctx context.Context,
	spec *backup_managerpb.BackupProviderSpec,
	backupID, provName string,
	node TopologyNode,
) *backup_managerpb.BackupProviderResult {

	endpoint := srv.nodeAgentEndpoint(node)
	slog.Info("starting remote backup provider",
		"provider", provName, "node_id", node.NodeID, "endpoint", endpoint)

	conn, err := srv.dialNodeAgent(ctx, endpoint)
	if err != nil {
		return failResult(spec.Type,
			fmt.Sprintf("dial node-agent on %s (%s): %v", node.NodeID, endpoint, err), nil)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)

	// Build the node-agent provider spec from the backup-manager spec
	agentSpec := &node_agentpb.BackupProviderSpec{
		Provider:       provName,
		Options:        srv.providerOptionsForNode(spec, provName),
		TimeoutSeconds: uint32(srv.ProviderTimeoutSeconds),
	}

	// Start the backup task on the remote node
	runResp, err := client.RunBackupProvider(ctx, &node_agentpb.RunBackupProviderRequest{
		BackupId: backupID,
		Spec:     agentSpec,
		NodeId:   node.NodeID,
	})
	if err != nil {
		return failResult(spec.Type,
			fmt.Sprintf("RunBackupProvider RPC failed on node %s: %v", node.NodeID, err), nil)
	}

	taskID := runResp.TaskId
	slog.Info("remote backup task started",
		"provider", provName, "node_id", node.NodeID, "task_id", taskID)

	// Poll until the task is done
	result, err := srv.pollBackupTask(ctx, client, taskID, node.NodeID)
	if err != nil {
		return failResult(spec.Type,
			fmt.Sprintf("poll backup task on node %s: %v", node.NodeID, err), nil)
	}

	// Write remote artifacts into the local capsule
	if len(result.Artifacts) > 0 {
		capsuleDir := srv.CapsuleDir(backupID)
		for relPath, data := range result.Artifacts {
			if err := CapsuleWriteFile(capsuleDir, relPath, data); err != nil {
				slog.Warn("failed to write remote artifact",
					"node_id", node.NodeID, "path", relPath, "error", err)
			}
		}
		slog.Info("wrote remote artifacts to capsule",
			"node_id", node.NodeID, "count", len(result.Artifacts))
	}

	// Convert node-agent result to backup-manager result
	return srv.convertAgentResult(spec.Type, result, node)
}

// pollBackupTask polls GetBackupTaskResult until the task completes or the context expires.
func (srv *server) pollBackupTask(
	ctx context.Context,
	client node_agentpb.NodeAgentServiceClient,
	taskID, nodeID string,
) (*node_agentpb.BackupProviderResult, error) {

	ticker := time.NewTicker(nodeAgentPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for task %s on node %s", taskID, nodeID)
		case <-ticker.C:
			resp, err := client.GetBackupTaskResult(ctx, &node_agentpb.GetBackupTaskResultRequest{
				TaskId: taskID,
			})
			if err != nil {
				slog.Warn("poll backup task error (will retry)",
					"task_id", taskID, "node_id", nodeID, "error", err)
				continue
			}
			if resp.Result != nil && resp.Result.Done {
				slog.Info("remote backup task completed",
					"task_id", taskID, "node_id", nodeID,
					"ok", resp.Result.Ok, "provider", resp.Result.Provider)
				return resp.Result, nil
			}
		}
	}
}

// convertAgentResult maps a node-agent BackupProviderResult to a backup-manager BackupProviderResult.
func (srv *server) convertAgentResult(
	provType backup_managerpb.BackupProviderType,
	agent *node_agentpb.BackupProviderResult,
	node TopologyNode,
) *backup_managerpb.BackupProviderResult {

	state := backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
	severity := backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO
	if !agent.Ok {
		state = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		severity = backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR
	}

	outputs := make(map[string]string)
	for k, v := range agent.Outputs {
		outputs[k] = v
	}
	outputs["node_id"] = node.NodeID
	outputs["node_hostname"] = node.Hostname
	outputs["remote"] = "true"

	return &backup_managerpb.BackupProviderResult{
		Type:         provType,
		Enabled:      true,
		State:        state,
		Severity:     severity,
		Summary:      agent.Summary,
		ErrorMessage: agent.ErrorMessage,
		Outputs:      outputs,
		OutputFiles:  agent.OutputFiles,
	}
}

// dialNodeAgent connects to a node-agent gRPC endpoint.
// Uses TLS if the backup-manager has TLS configured, otherwise plaintext.
// dialNodeAgent creates a gRPC connection to a node-agent. Uses
// config.ResolveDialTarget for canonical endpoint resolution — loopback
// IPs are rewritten to "localhost" so the service cert SAN matches.
func (srv *server) dialNodeAgent(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dt := config.ResolveDialTarget(endpoint)

	var opts []grpc.DialOption

	if srv.TLS {
		tlsCfg, err := srv.hookTLSConfig(dt.Address)
		if err != nil {
			slog.Warn("node-agent TLS config failed, falling back to insecure",
				"endpoint", dt.Address, "error", err)
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			tlsCfg.ServerName = dt.ServerName
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	opts = append(opts, grpc.WithBlock())

	return grpc.DialContext(dialCtx, dt.Address, opts...)
}

// nodeAgentEndpoint returns the gRPC endpoint for a node's agent.
// Prefers AgentEndpoint from topology; falls back to Address:11000.
func (srv *server) nodeAgentEndpoint(n TopologyNode) string {
	if n.AgentEndpoint != "" {
		return n.AgentEndpoint
	}
	return fmt.Sprintf("%s:%d", n.Address, nodeAgentDefaultPort)
}

// providerOptionsForNode builds the options map for a node-agent provider spec
// from the backup-manager's configuration.
func (srv *server) providerOptionsForNode(spec *backup_managerpb.BackupProviderSpec, provName string) map[string]string {
	opts := make(map[string]string)

	switch provName {
	case "restic":
		if srv.ResticRepo != "" {
			opts["repo"] = srv.ResticRepo
		}
		if srv.ResticPassword != "" {
			opts["password"] = srv.ResticPassword
		}
		if srv.ResticPaths != "" {
			opts["paths"] = srv.ResticPaths
		}
	}

	// Merge any spec-level options (they take precedence)
	for k, v := range spec.Options {
		opts[k] = v
	}

	return opts
}

// isLocalNode determines if a topology node is the node where backup-manager is running.
func (srv *server) isLocalNode(n TopologyNode) bool {
	if n.NodeID == srv.Id {
		return true
	}
	if n.Address == srv.Address || n.Address == "127.0.0.1" || n.Address == "localhost" {
		return true
	}
	// Compare by hostname
	if hostname, err := os.Hostname(); err == nil && n.Hostname == hostname {
		return true
	}
	// Compare by local IP addresses
	if n.Address != "" {
		addrs, err := net.InterfaceAddrs()
		if err == nil {
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					if ipnet.IP.String() == n.Address {
						return true
					}
				}
			}
		}
	}
	return false
}

// NewNodeCapsuleContext creates a per-node capsule context:
// payload/nodes/<node_id>/<provider>/ and provider/nodes/<node_id>/<provider>/
func (srv *server) NewNodeCapsuleContext(backupID, providerName, nodeID string) (*CapsuleContext, error) {
	capsuleDir := srv.CapsuleDir(backupID)
	providerDir := fmt.Sprintf("%s/provider/%s/%s", capsuleDir, providerName, nodeID)
	payloadDir := fmt.Sprintf("%s/payload/nodes/%s/%s", capsuleDir, nodeID, providerName)

	for _, d := range []string{providerDir, payloadDir} {
		if err := ensureDir(d); err != nil {
			return nil, err
		}
	}

	return &CapsuleContext{
		BackupID:    backupID,
		CapsuleDir:  capsuleDir,
		ProviderDir: providerDir,
		PayloadDir:  payloadDir,
	}, nil
}

// writeCoverage writes node coverage data into the capsule.
func (srv *server) writeCoverage(backupID string, coverages []*NodeCoverage) error {
	if len(coverages) == 0 {
		return nil
	}
	data, err := json.MarshalIndent(coverages, "", "  ")
	if err != nil {
		return err
	}
	return CapsuleWriteFile(srv.CapsuleDir(backupID), "meta/coverage.json", data)
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
