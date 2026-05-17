package main

// awareness_live_snapshot_cmd.go — live mirror refresh command.
//
// Runs only live collectors (systemd, varlib, etcd, PKI, RBAC, workflow) and
// updates the awareness graph with fresh runtime evidence.
//
// Does NOT re-extract static sources (Go AST, YAML knowledge files, protos).
// This is distinct from 'awareness build', which rebuilds the full static graph.
//
// Designed for scheduled execution via systemd timer:
//   awareness-live-snapshot.service + awareness-live-snapshot.timer
//
// Output format:
//   { "live_overlay": { "status": "fresh|partial|failed|absent", "collected_at": "...",
//                       "expires_at": "...", "collectors": [...] } }

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/globulario/services/golang/awareness/extractors/clusterstate"
	"github.com/globulario/services/golang/awareness/extractors/pki"
	"github.com/globulario/services/golang/awareness/extractors/rbac"
	"github.com/globulario/services/golang/awareness/extractors/workflowstate"
	"github.com/globulario/services/golang/awareness/graph"
)

// liveSnapshotTTLSeconds is how long a live snapshot is considered fresh.
const liveSnapshotTTLSeconds = 300 // 5 minutes

var liveSnapshotCfg = struct {
	output             string
	collectSystemd     bool
	collectVarLib      bool
	collectEtcd        bool
	collectConvergence bool
	collectPKI         bool
	collectRBAC        bool
	collectWorkflow    bool
	workflowAddr       string
}{
	collectSystemd:     true,
	collectVarLib:      true,
	collectEtcd:        false,
	collectConvergence: false,
	collectPKI:         true,
	collectRBAC:        true,
	collectWorkflow:    false,
}

// LiveOverlayReport is the JSON output of a live-snapshot run.
type LiveOverlayReport struct {
	LiveOverlay LiveOverlayStatus `json:"live_overlay"`
}

// LiveOverlayStatus summarises a single live mirror refresh.
type LiveOverlayStatus struct {
	Status      string                 `json:"status"` // fresh | partial | failed | absent
	CollectedAt string                 `json:"collected_at"`
	ExpiresAt   string                 `json:"expires_at"`
	Collectors  []LiveCollectorSummary `json:"collectors"`
}

// LiveCollectorSummary is a per-collector result within a live snapshot.
type LiveCollectorSummary struct {
	Collector string `json:"collector"`
	Status    string `json:"status"`
	ItemsSeen int    `json:"items_seen,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

var awarenessLiveSnapshotCmd = &cobra.Command{
	Use:   "live-snapshot",
	Short: "Refresh live overlay data without rebuilding the static graph",
	Long: `Runs live collectors (systemd, varlib, etcd, PKI, RBAC, workflow execution) and
updates the awareness graph with fresh runtime evidence. Does NOT re-extract
static sources (Go AST, YAML knowledge files, proto definitions, docs).

Use this command when you need up-to-date runtime evidence without the cost of
a full 'awareness build'. Designed for scheduled execution via systemd timer.

Output: JSON live_overlay status with per-collector health.

Scheduling:
  Install docs/awareness/systemd/awareness-live-snapshot.service and
  docs/awareness/systemd/awareness-live-snapshot.timer for automatic refresh.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Open existing graph R/W. Do NOT clean — we only update live overlay nodes.
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return fmt.Errorf("open graph: %w", err)
		}
		defer g.Close()

		now := time.Now()
		var items []graph.CollectorHealthItem
		var collectors []LiveCollectorSummary

		record := func(id, tier, status string, nodes int, errStr string) {
			items = append(items, graph.CollectorHealthItem{
				CollectorID:  id,
				SourceTier:   tier,
				Status:       status,
				NodesEmitted: nodes,
				Error:        errStr,
				Priority:     "P0",
			})
			c := LiveCollectorSummary{Collector: id, Status: status, ItemsSeen: nodes}
			if errStr != "" {
				c.LastError = errStr
			}
			collectors = append(collectors, c)
		}

		if liveSnapshotCfg.collectSystemd {
			h, err := clusterstate.CollectSystemd(ctx, g)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			record("systemd", "systemd_runtime", h.Status, h.NodesEmitted, errStr)
		}

		if liveSnapshotCfg.collectVarLib {
			h, err := clusterstate.CollectVarLib(ctx, g)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			record("varlib", "installed_metadata", h.Status, h.NodesEmitted, errStr)
		}

		if liveSnapshotCfg.collectEtcd {
			var etcdFactory clusterstate.EtcdClientFactory
			if client, err := getEtcdClientFactory(); err == nil {
				etcdFactory = client
			}
			h, err := clusterstate.CollectEtcd(ctx, g, etcdFactory)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			record("etcd", "etcd_desired_state", h.Status, h.NodesEmitted, errStr)
		}

		if liveSnapshotCfg.collectConvergence {
			var etcdFactory clusterstate.EtcdClientFactory
			if client, err := getEtcdClientFactory(); err == nil {
				etcdFactory = client
			}
			h, err := clusterstate.CollectConvergence(ctx, g, etcdFactory)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			record("convergence", "cluster_authority", h.Status, h.NodesEmitted, errStr)
		}

		if liveSnapshotCfg.collectPKI {
			h, err := pki.Extract(ctx, g, pki.DefaultPKIPaths[0])
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			record("pki", "cluster_security", h.Status, h.NodesEmitted, errStr)
		}

		if liveSnapshotCfg.collectRBAC {
			h, err := rbac.Extract(ctx, g, rbac.DefaultPolicyDir)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			record("rbac", "cluster_security", h.Status, h.NodesEmitted, errStr)
		}

		if liveSnapshotCfg.collectWorkflow {
			var wfFactory workflowstate.GRPCConnFactory
			if addr := liveSnapshotCfg.workflowAddr; addr != "" {
				wfFactory = func() (*grpc.ClientConn, error) {
					return dialGRPC(addr)
				}
			}
			repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
			docsDir := filepath.Join(repoRoot, "docs", "awareness")
			h, err := workflowstate.Collect(ctx, g, wfFactory, docsDir)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			record("workflow_execution", "live_runtime", h.Status, h.NodesEmitted, errStr)
		}

		// Persist the live snapshot record in the graph (always overwrites the fixed ID).
		repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
		if err := g.UpsertBuildRecord(ctx, graph.LiveSnapshotBuildID, repoRoot, "", "", graph.BuildStats{}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: store live snapshot record: %v\n", err)
		}
		if len(items) > 0 {
			if err := g.SetBuildCollectorHealth(ctx, graph.LiveSnapshotBuildID, items); err != nil {
				fmt.Fprintf(os.Stderr, "warning: store live snapshot health: %v\n", err)
			}
		}

		overallStatus := liveOverlayStatusFromCollectors(collectors)
		expiresAt := now.Add(liveSnapshotTTLSeconds * time.Second)

		report := LiveOverlayReport{
			LiveOverlay: LiveOverlayStatus{
				Status:      overallStatus,
				CollectedAt: now.UTC().Format(time.RFC3339),
				ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
				Collectors:  collectors,
			},
		}

		out, _ := json.MarshalIndent(report, "", "  ")

		if liveSnapshotCfg.output != "" && liveSnapshotCfg.output != "-" {
			if err := os.WriteFile(liveSnapshotCfg.output, out, 0644); err != nil {
				return fmt.Errorf("write snapshot: %w", err)
			}
			fmt.Fprintf(os.Stderr, "live snapshot written to %s (status=%s)\n", liveSnapshotCfg.output, overallStatus)
		} else {
			fmt.Fprintln(os.Stdout, string(out))
		}
		return nil
	},
}

// liveOverlayStatusFromCollectors derives overall status from per-collector results.
// "absent" is used by callers that read the graph and find no snapshot record.
func liveOverlayStatusFromCollectors(collectors []LiveCollectorSummary) string {
	if len(collectors) == 0 {
		return "absent"
	}
	ok, failed := 0, 0
	for _, c := range collectors {
		switch c.Status {
		case "ok", "checked_with_matches", "checked_clean", "skipped":
			ok++
		default:
			failed++
		}
	}
	if failed == 0 {
		return "fresh"
	}
	if ok > 0 {
		return "partial"
	}
	return "failed"
}

func init() {
	awarenessLiveSnapshotCmd.Flags().StringVar(&liveSnapshotCfg.output, "output", "", "Write JSON output to file instead of stdout (use '-' for stdout)")
	awarenessLiveSnapshotCmd.Flags().BoolVar(&liveSnapshotCfg.collectSystemd, "collect-systemd", true, "Collect systemd unit state")
	awarenessLiveSnapshotCmd.Flags().BoolVar(&liveSnapshotCfg.collectVarLib, "collect-var-lib", true, "Scan /var/lib/globular for PKI certs and receipts")
	awarenessLiveSnapshotCmd.Flags().BoolVar(&liveSnapshotCfg.collectEtcd, "collect-etcd", false, "Collect desired/installed state from etcd")
	awarenessLiveSnapshotCmd.Flags().BoolVar(&liveSnapshotCfg.collectConvergence, "collect-convergence", false, "Collect convergence deltas from etcd")
	awarenessLiveSnapshotCmd.Flags().BoolVar(&liveSnapshotCfg.collectPKI, "collect-pki", true, "Extract public certificate metadata")
	awarenessLiveSnapshotCmd.Flags().BoolVar(&liveSnapshotCfg.collectRBAC, "collect-rbac", true, "Extract RBAC roles and permissions")
	awarenessLiveSnapshotCmd.Flags().BoolVar(&liveSnapshotCfg.collectWorkflow, "collect-workflow", false, "Collect live workflow execution state (requires --workflow-addr)")
	awarenessLiveSnapshotCmd.Flags().StringVar(&liveSnapshotCfg.workflowAddr, "workflow-addr", "", "Workflow service gRPC address")
	awarenessLiveSnapshotCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json")
	awarenessLiveSnapshotCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
}
