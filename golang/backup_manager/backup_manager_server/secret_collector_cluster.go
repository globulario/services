package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// defaultCollectBackupSecretsTimeout bounds the per-node RPC. Don't use
// no-timeout contexts; a hung node_agent must not stall the whole backup
// preparation phase.
const defaultCollectBackupSecretsTimeout = 30 * time.Second

// defaultSecretCollectionConcurrency caps the parallel fan-out. For a 5-node
// cluster all-at-once is fine; this constant keeps the implementation sane
// at 50 nodes too.
const defaultSecretCollectionConcurrency = 8

// bootstrapSAPasswordPath is the singleton cluster secret. Phase 1's collector
// reports it under this `original_path`. The conflict detector compares its
// SHA-256 across nodes; mismatched fingerprints mean two nodes claim the
// secret with different content — corruption signal, fail backup.
const bootstrapSAPasswordPath = "/var/lib/globular/.bootstrap-sa-password"

// BackupSecretTargetNode is the input to a per-node CollectBackupSecrets
// call. Built from existing TopologyNode discovery — no new authority.
type BackupSecretTargetNode struct {
	NodeID        string
	Hostname      string
	Address       string
	AgentEndpoint string
	Required      bool // Phase 2: every topology node is required by default
}

// BackupSecretNodeResult is the per-node aggregation of a CollectBackupSecrets
// response. Mirrors node_agentpb.CollectBackupSecretsResponse but with the
// extra Status / Error fields used by the cluster-level manifest.
type BackupSecretNodeResult struct {
	NodeID           string                          `json:"node_id"`
	Hostname         string                          `json:"hostname,omitempty"`
	PrimaryIP        string                          `json:"primary_ip,omitempty"`
	Status           string                          `json:"status"` // "ok" | "error"
	Error            string                          `json:"error,omitempty"`
	CapsulePath      string                          `json:"capsule_path,omitempty"`   // dir relative to capsule root
	ManifestPath     string                          `json:"manifest_path,omitempty"`  // capsule-relative
	Entries          []*node_agentpb.SecretFileEntry `json:"-"`
	MissingRequired  []string                        `json:"missing_required,omitempty"`
	MissingOptional  []string                        `json:"missing_optional,omitempty"`
	Warnings         []string                        `json:"warnings,omitempty"`
}

// BackupSecretCollector abstracts the call into node_agent so tests can
// inject a mock without standing up a real gRPC server.
type BackupSecretCollector interface {
	CollectBackupSecrets(ctx context.Context, node BackupSecretTargetNode, capsuleDir, backupID string) (*BackupSecretNodeResult, error)
}

// makeBackupSecretCollector returns the production collector. Overridable
// in tests; production callers leave it alone.
var makeBackupSecretCollector = func(srv *server) BackupSecretCollector {
	return &nodeAgentSecretCollector{srv: srv}
}

// nodeAgentSecretCollector is the production implementation: dials each
// node_agent over the existing TLS/gRPC plumbing and invokes the RPC. The
// only filesystem reads happen inside node_agent — backup_manager never
// reads remote-node secret files directly. That property is enforced
// structurally by this interface (the only entry point is the RPC) and
// asserted by TestSecretCollector_NoDirectFilesystemScraping.
type nodeAgentSecretCollector struct {
	srv *server
}

func (c *nodeAgentSecretCollector) CollectBackupSecrets(ctx context.Context, node BackupSecretTargetNode, capsuleDir, backupID string) (*BackupSecretNodeResult, error) {
	endpoint := node.AgentEndpoint
	if endpoint == "" {
		// Fall back to the helper that backs RunBackupProvider — same
		// default-port logic, no new discovery.
		endpoint = c.srv.nodeAgentEndpoint(TopologyNode{
			NodeID:        node.NodeID,
			Hostname:      node.Hostname,
			Address:       node.Address,
			AgentEndpoint: node.AgentEndpoint,
		})
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, defaultCollectBackupSecretsTimeout)
	defer dialCancel()

	conn, err := c.srv.dialNodeAgent(dialCtx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("dial node-agent at %s: %w", endpoint, err)
	}
	defer conn.Close()

	rpcCtx, rpcCancel := context.WithTimeout(ctx, defaultCollectBackupSecretsTimeout)
	defer rpcCancel()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	resp, err := client.CollectBackupSecrets(rpcCtx, &node_agentpb.CollectBackupSecretsRequest{
		CapsuleDir: capsuleDir,
		BackupId:   backupID,
	})
	if err != nil {
		return nil, fmt.Errorf("CollectBackupSecrets RPC on %s: %w", node.NodeID, err)
	}
	return &BackupSecretNodeResult{
		NodeID:          resp.NodeId,
		Hostname:        resp.Hostname,
		PrimaryIP:       resp.PrimaryIp,
		Status:          "ok",
		CapsulePath:     filepath.Join("payload", "secrets", resp.NodeId),
		ManifestPath:    resp.PerNodeManifest,
		Entries:         resp.Entries,
		MissingRequired: resp.MissingRequired,
		MissingOptional: resp.MissingOptional,
	}, nil
}

// discoverBackupSecretTargets converts the existing TopologyNode list (from
// captureTopology) into the BackupSecretTargetNode set. Every topology node
// is treated as Required: silently skipping a node would produce a backup
// missing node-local secret material — exactly the failure mode the spec
// forbids ("a fake parachute").
func discoverBackupSecretTargets(topo []TopologyNode) []BackupSecretTargetNode {
	out := make([]BackupSecretTargetNode, 0, len(topo))
	for _, n := range topo {
		out = append(out, BackupSecretTargetNode{
			NodeID:        n.NodeID,
			Hostname:      n.Hostname,
			Address:       n.Address,
			AgentEndpoint: n.AgentEndpoint,
			Required:      true,
		})
	}
	return out
}

// ClusterSecretManifest is the top-level summary written into the capsule
// at payload/secrets/manifest.json. Schema is intentionally simple and
// jq-friendly; the goal is operational clarity, not exhaustive metadata.
type ClusterSecretManifest struct {
	SchemaVersion int                       `json:"schema_version"`
	CreatedAt     string                    `json:"created_at"`
	Source        string                    `json:"source"`
	Nodes         []*BackupSecretNodeResult `json:"nodes"`
	Conflicts     []SecretConflict          `json:"conflicts,omitempty"`
	Summary       ClusterSecretSummary      `json:"summary"`
}

// ClusterSecretSummary is the cluster-view tally.
type ClusterSecretSummary struct {
	NodesTargeted        int `json:"nodes_targeted"`
	NodesSucceeded       int `json:"nodes_succeeded"`
	NodesFailed          int `json:"nodes_failed"`
	RequiredMissingCount int `json:"required_missing_count"`
	OptionalMissingCount int `json:"optional_missing_count"`
	WarningsCount        int `json:"warnings_count"`
}

// SecretConflict reports two-or-more nodes claiming the same singleton
// cluster secret with different fingerprints. Bytes never leave node_agent;
// only the SHA-256 metadata is compared here.
type SecretConflict struct {
	SecretName   string                 `json:"secret_name"`
	Fingerprints []SecretFingerprintRef `json:"fingerprints"`
}

// SecretFingerprintRef records which node claimed a singleton with a given
// SHA-256. Used only inside conflict reporting.
type SecretFingerprintRef struct {
	NodeID string `json:"node_id"`
	SHA256 string `json:"sha256"`
}

// MissingRequiredByNode groups missing-required paths by node for the
// failure message. Order is stable for deterministic logs.
type MissingRequiredByNode struct {
	NodeID string   `json:"node_id"`
	Paths  []string `json:"paths"`
}

// collectClusterSecrets is the orchestration entry point. Spec sequence:
//  1. Resolve backup target node set.
//  2. (capsule dir already created by EnsureCapsuleDir before this is called)
//  3. Fan out to node agents with CollectBackupSecrets.
//  4. Write aggregated cluster secret manifest.
//
// Returns nil on success. On failure, the cluster manifest is still written
// (for diagnostics) and the error message names the failure class. Caller
// (the cluster-backup orchestration in handlers.go) translates the error
// into a job failure verdict.
func (srv *server) collectClusterSecrets(ctx context.Context, backupID string, topo []TopologyNode) error {
	targets := discoverBackupSecretTargets(topo)
	if len(targets) == 0 {
		slog.Warn("backup-secrets: no target nodes discovered; skipping collection",
			"backup_id", backupID)
		// Still write an empty manifest so the absence is explicit.
		return srv.writeClusterSecretManifest(backupID, &ClusterSecretManifest{
			SchemaVersion: 1,
			CreatedAt:     time.Now().UTC().Format(time.RFC3339),
			Source:        "backup_manager",
			Summary:       ClusterSecretSummary{},
		})
	}
	capsuleDir := srv.CapsuleDir(backupID)
	collector := makeBackupSecretCollector(srv)
	results := fanOutCollectSecrets(ctx, collector, targets, capsuleDir, backupID)

	// Deterministic ordering by node_id so tests and operator-facing manifests
	// don't shuffle on every run.
	sort.Slice(results, func(i, j int) bool { return results[i].NodeID < results[j].NodeID })

	conflicts := detectBootstrapSecretConflicts(results)
	manifest := buildClusterManifest(results, conflicts)

	// Always write the manifest, even on failure paths — operator diagnostics.
	if werr := srv.writeClusterSecretManifest(backupID, manifest); werr != nil {
		slog.Warn("backup-secrets: write cluster manifest failed",
			"backup_id", backupID, "error", werr.Error())
	}

	// Failure conditions, in priority order:
	//  1. Any required node-agent unreachable (RPC error) — fake parachute.
	//  2. Any node reported missing_required entries — incomplete capsule.
	//  3. Bootstrap-secret fingerprint conflict — corruption signal.
	if unreachable := collectUnreachable(results); len(unreachable) > 0 {
		return fmt.Errorf("backup-secrets: required node-agent(s) unreachable: %s",
			formatUnreachable(unreachable))
	}
	if missByNode := collectMissingRequired(results); len(missByNode) > 0 {
		return fmt.Errorf("backup-secrets: required paths unreadable on %d node(s); see manifest payload/secrets/manifest.json: %s",
			len(missByNode), formatMissingRequired(missByNode))
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("backup-secrets: %d singleton secret(s) have mismatched fingerprints across nodes; refusing backup",
			len(conflicts))
	}

	slog.Info("backup-secrets: cluster collection complete",
		"backup_id", backupID,
		"nodes_targeted", len(targets),
		"nodes_ok", manifest.Summary.NodesSucceeded,
		"warnings", manifest.Summary.WarningsCount,
	)
	return nil
}

// fanOutCollectSecrets invokes the collector for every target concurrently
// (bounded by defaultSecretCollectionConcurrency). Returns one result per
// target — error results have Status="error" and a populated Error string.
func fanOutCollectSecrets(ctx context.Context, c BackupSecretCollector, targets []BackupSecretTargetNode, capsuleDir, backupID string) []*BackupSecretNodeResult {
	results := make([]*BackupSecretNodeResult, len(targets))
	sem := make(chan struct{}, defaultSecretCollectionConcurrency)
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, t BackupSecretTargetNode) {
			defer wg.Done()
			defer func() { <-sem }()
			r, err := c.CollectBackupSecrets(ctx, t, capsuleDir, backupID)
			if err != nil {
				slog.Warn("backup-secrets: per-node collection failed",
					"node_id", t.NodeID, "error", err.Error())
				results[i] = &BackupSecretNodeResult{
					NodeID: t.NodeID,
					Status: "error",
					Error:  err.Error(),
				}
				return
			}
			// Defensive: make sure NodeID is populated even if the response
			// somehow returned empty (mocked tests, edge cases).
			if r.NodeID == "" {
				r.NodeID = t.NodeID
			}
			if r.Status == "" {
				r.Status = "ok"
			}
			slog.Info("backup-secrets: per-node collection succeeded",
				"node_id", r.NodeID,
				"required_missing", len(r.MissingRequired),
				"optional_missing", len(r.MissingOptional),
			)
			results[i] = r
		}(i, t)
	}
	wg.Wait()
	return results
}

// detectBootstrapSecretConflicts scans the per-node results for the
// singleton bootstrap secret and reports a conflict when ≥ 2 nodes claim
// the file with DIFFERENT sha256 fingerprints. Comparison is fingerprint-
// only; content bytes never leave node_agent.
func detectBootstrapSecretConflicts(results []*BackupSecretNodeResult) []SecretConflict {
	byNode := map[string]string{}
	uniqueShas := map[string]struct{}{}
	for _, r := range results {
		if r == nil || r.Status != "ok" {
			continue
		}
		for _, e := range r.Entries {
			if e == nil || e.OriginalPath != bootstrapSAPasswordPath {
				continue
			}
			if !e.Found || e.Sha256 == "" {
				continue
			}
			byNode[r.NodeID] = e.Sha256
			uniqueShas[e.Sha256] = struct{}{}
		}
	}
	if len(byNode) < 2 || len(uniqueShas) < 2 {
		return nil
	}
	// Stable order of fingerprints for deterministic output.
	nodeIDs := make([]string, 0, len(byNode))
	for k := range byNode {
		nodeIDs = append(nodeIDs, k)
	}
	sort.Strings(nodeIDs)
	// Use a clean logical name (no dotfile prefix) in the manifest — matches
	// the spec example and is friendlier to operator inspection / grep.
	conflict := SecretConflict{SecretName: "bootstrap-sa-password"}
	for _, id := range nodeIDs {
		conflict.Fingerprints = append(conflict.Fingerprints, SecretFingerprintRef{
			NodeID: id,
			SHA256: byNode[id],
		})
	}
	return []SecretConflict{conflict}
}

// collectUnreachable returns the per-node entries that came back as RPC
// errors. Reachability is gated on Status=="error" (set by fanOut on any
// RPC error including dial / timeout / Unauthenticated).
func collectUnreachable(results []*BackupSecretNodeResult) []*BackupSecretNodeResult {
	var out []*BackupSecretNodeResult
	for _, r := range results {
		if r != nil && r.Status == "error" {
			out = append(out, r)
		}
	}
	return out
}

// collectMissingRequired returns the per-node grouped missing_required
// paths. Empty when every node reported a clean response.
func collectMissingRequired(results []*BackupSecretNodeResult) []MissingRequiredByNode {
	var out []MissingRequiredByNode
	for _, r := range results {
		if r == nil || r.Status != "ok" || len(r.MissingRequired) == 0 {
			continue
		}
		out = append(out, MissingRequiredByNode{
			NodeID: r.NodeID,
			Paths:  append([]string(nil), r.MissingRequired...),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out
}

// formatUnreachable summarizes the unreachable nodes for an error message.
// Never includes secret contents — only node_id and the RPC error text.
func formatUnreachable(rs []*BackupSecretNodeResult) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, fmt.Sprintf("%s (%s)", r.NodeID, r.Error))
	}
	sort.Strings(parts)
	return joinComma(parts)
}

// formatMissingRequired summarizes the missing-required grouping. Logs the
// node_id and the logical path metadata; never the bytes.
func formatMissingRequired(groups []MissingRequiredByNode) string {
	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		parts = append(parts, fmt.Sprintf("%s=[%s]", g.NodeID, joinComma(g.Paths)))
	}
	return joinComma(parts)
}

func joinComma(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := items[0]
	for i := 1; i < len(items); i++ {
		out += ", " + items[i]
	}
	return out
}

// buildClusterManifest assembles the aggregated top-level manifest from
// per-node results and conflict reports.
func buildClusterManifest(results []*BackupSecretNodeResult, conflicts []SecretConflict) *ClusterSecretManifest {
	m := &ClusterSecretManifest{
		SchemaVersion: 1,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Source:        "backup_manager",
		Nodes:         results,
		Conflicts:     conflicts,
	}
	m.Summary.NodesTargeted = len(results)
	for _, r := range results {
		if r == nil {
			continue
		}
		if r.Status == "ok" {
			m.Summary.NodesSucceeded++
		} else {
			m.Summary.NodesFailed++
		}
		m.Summary.RequiredMissingCount += len(r.MissingRequired)
		m.Summary.OptionalMissingCount += len(r.MissingOptional)
		m.Summary.WarningsCount += len(r.Warnings)
	}
	return m
}

// writeClusterSecretManifest writes payload/secrets/manifest.json into the
// capsule via tmp+rename, mode 0640. Tests override this by overriding
// makeBackupSecretCollector with a mock that doesn't drive the orchestrator.
func (srv *server) writeClusterSecretManifest(backupID string, m *ClusterSecretManifest) error {
	dir := filepath.Join(srv.CapsuleDir(backupID), "payload", "secrets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("mkdir cluster secrets dir: %w", err)
	}
	buf, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cluster manifest: %w", err)
	}
	path := filepath.Join(dir, "manifest.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o640); err != nil {
		return fmt.Errorf("write tmp manifest: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename manifest: %w", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		return fmt.Errorf("chmod manifest: %w", err)
	}
	return nil
}
