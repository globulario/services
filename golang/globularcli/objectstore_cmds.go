package main

// objectstore_cmds.go — read-only MinIO topology diagnostics.
//
//   globular objectstore topology status
//
// Reads from etcd and probes the MinIO health endpoint. No mutations.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"os"

	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
)

var objectstoreCmd = &cobra.Command{
	Use:   "objectstore",
	Short: "Objectstore diagnostics",
}

var objectstoreTopologyCmd = &cobra.Command{
	Use:   "topology",
	Short: "MinIO topology diagnostics",
}

var (
	topoStatusJSON bool
)

var objectstoreTopologyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print MinIO topology convergence status (read-only)",
	Long: `Prints the full MinIO topology convergence status without making any changes.

Reads from etcd:
  - Desired objectstore state (generation, mode, pool nodes, volumes hash)
  - Applied generation and last restart result
  - Per-node rendered generation and state fingerprint

Probes:
  - MinIO health endpoint (HTTP GET /minio/health/live)

Exit codes:
  0  fully converged
  1  not converged or check failed
`,
	RunE: runObjectstoreTopologyStatus,
}

func init() {
	objectstoreTopologyStatusCmd.Flags().BoolVar(&topoStatusJSON, "json", false, "Output as JSON")
	objectstoreTopologyCmd.AddCommand(objectstoreTopologyStatusCmd)
	objectstoreCmd.AddCommand(objectstoreTopologyCmd)
}

// ── status command ────────────────────────────────────────────────────────────

type topologyStatusReport struct {
	Desired *topologyDesiredFields `json:"desired,omitempty"`
	Applied topologyAppliedFields  `json:"applied"`
	Lock    topologyLockFields     `json:"lock"`
	Nodes   []topologyNodeStatus   `json:"nodes"`
	Health  topologyHealthStatus   `json:"health"`
	Summary string                 `json:"summary"`
	Converged bool                 `json:"converged"`
}

type topologyDesiredFields struct {
	Generation       int64    `json:"generation"`
	Mode             string   `json:"mode"`
	PoolNodes        []string `json:"pool_nodes"`
	DrivesPerNode    int      `json:"drives_per_node"`
	Endpoint         string   `json:"endpoint"`
	VolumesHash      string   `json:"volumes_hash"`
	ExpectedFingerprint string `json:"expected_fingerprint"`
	WrittenAt        string   `json:"written_at"`
}

type topologyAppliedFields struct {
	Generation     int64  `json:"generation"`
	Pending        bool   `json:"pending"`
	RestartInProgress bool `json:"restart_in_progress"`
	LastResult     string `json:"last_result"`
}

type topologyLockFields struct {
	Held     bool   `json:"held"`
	HeldSince string `json:"held_since,omitempty"`
}

type topologyNodeStatus struct {
	NodeID              string `json:"node_id"`
	PoolIP              string `json:"pool_ip"`
	RenderedGeneration  int64  `json:"rendered_generation"`
	RenderedFingerprint string `json:"rendered_fingerprint"`
	FingerprintMatch    string `json:"fingerprint_match"` // "match" | "mismatch" | "missing"
	ServiceState        string `json:"service_state"`
}

type topologyHealthStatus struct {
	Endpoint  string `json:"endpoint"`
	StatusCode int   `json:"status_code,omitempty"`
	Healthy   bool   `json:"healthy"`
	Error     string `json:"error,omitempty"`
}

func runObjectstoreTopologyStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report := &topologyStatusReport{}

	// ── etcd client ───────────────────────────────────────────────────────────
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	// ── desired state ─────────────────────────────────────────────────────────
	desired, err := config.LoadObjectStoreDesiredState(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: cannot read desired state: %v\n", err)
	}
	if desired != nil {
		fp := config.RenderStateFingerprint(desired)
		report.Desired = &topologyDesiredFields{
			Generation:          desired.Generation,
			Mode:                string(desired.Mode),
			PoolNodes:           desired.Nodes,
			DrivesPerNode:       desired.DrivesPerNode,
			Endpoint:            desired.Endpoint,
			VolumesHash:         desired.VolumesHash,
			ExpectedFingerprint: fp,
			WrittenAt:           desired.WrittenAt.Format(time.RFC3339),
		}
	}

	// ── applied generation ────────────────────────────────────────────────────
	if resp, err := cli.Get(ctx, config.EtcdKeyObjectStoreAppliedGeneration); err == nil && len(resp.Kvs) > 0 {
		if gen, err := strconv.ParseInt(string(resp.Kvs[0].Value), 10, 64); err == nil {
			report.Applied.Generation = gen
		}
	}
	if desired != nil {
		report.Applied.Pending = report.Applied.Generation < desired.Generation
	}
	if resp, err := cli.Get(ctx, config.EtcdKeyObjectStoreRestartInProgress); err == nil && len(resp.Kvs) > 0 {
		report.Applied.RestartInProgress = true
	}
	if resp, err := cli.Get(ctx, config.EtcdKeyObjectStoreLastRestartResult); err == nil && len(resp.Kvs) > 0 {
		report.Applied.LastResult = string(resp.Kvs[0].Value)
	}

	// ── topology lock ─────────────────────────────────────────────────────────
	if resp, err := cli.Get(ctx, config.EtcdKeyObjectStoreTopologyLock); err == nil && len(resp.Kvs) > 0 {
		report.Lock.Held = true
		val := string(resp.Kvs[0].Value)
		// Lock value format: "2006-01-02T15:04:05Z07:00|lease=..."
		if idx := strings.Index(val, "|"); idx > 0 {
			report.Lock.HeldSince = val[:idx]
		} else if len(val) <= 30 {
			report.Lock.HeldSince = val
		}
	}

	// ── per-node status ───────────────────────────────────────────────────────
	if desired != nil && len(desired.Nodes) > 0 {
		expectedFP := config.RenderStateFingerprint(desired)
		ipToNodeID, ipToServiceState := buildIPMaps(ctx)

		for _, poolIP := range desired.Nodes {
			ns := topologyNodeStatus{
				PoolIP:              poolIP,
				FingerprintMatch:    "missing",
				ServiceState:        "unknown",
			}

			nodeID := ipToNodeID[poolIP]
			ns.NodeID = nodeID

			if nodeID != "" {
				ns.ServiceState = ipToServiceState[poolIP]

				genKey := config.EtcdKeyNodeRenderedGeneration(nodeID)
				if r, err := cli.Get(ctx, genKey); err == nil && len(r.Kvs) > 0 {
					if gen, err := strconv.ParseInt(string(r.Kvs[0].Value), 10, 64); err == nil {
						ns.RenderedGeneration = gen
					}
				}

				fpKey := config.EtcdKeyNodeRenderedStateFingerprint(nodeID)
				if r, err := cli.Get(ctx, fpKey); err == nil && len(r.Kvs) > 0 {
					ns.RenderedFingerprint = string(r.Kvs[0].Value)
					if ns.RenderedFingerprint == expectedFP {
						ns.FingerprintMatch = "match"
					} else {
						ns.FingerprintMatch = "mismatch"
					}
				}
			}

			report.Nodes = append(report.Nodes, ns)
		}
	}

	// ── MinIO health probe ────────────────────────────────────────────────────
	if desired != nil && desired.Endpoint != "" {
		host := desired.Endpoint
		if !strings.Contains(host, ":") {
			host = host + ":9000"
		}
		healthURL := "http://" + host + "/minio/health/live"
		report.Health.Endpoint = healthURL
		hCtx, hCancel := context.WithTimeout(ctx, 10*time.Second)
		req, _ := http.NewRequestWithContext(hCtx, http.MethodGet, healthURL, nil)
		httpClient := &http.Client{Timeout: 10 * time.Second}
		if resp, err := httpClient.Do(req); err != nil {
			report.Health.Error = err.Error()
		} else {
			resp.Body.Close()
			report.Health.StatusCode = resp.StatusCode
			report.Health.Healthy = resp.StatusCode == http.StatusOK
			if !report.Health.Healthy {
				report.Health.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
		}
		hCancel()
	}

	// ── converged judgment ────────────────────────────────────────────────────
	report.Converged = isConverged(report)
	if report.Converged {
		report.Summary = "CONVERGED"
	} else {
		report.Summary = summarizeProblems(report)
	}

	// ── output ────────────────────────────────────────────────────────────────
	if topoStatusJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	printTopologyReport(report)
	if !report.Converged {
		return fmt.Errorf("%s", report.Summary)
	}
	return nil
}

// buildIPMaps queries the cluster controller for node records and builds
// IP → nodeID and IP → minio service state maps. Returns empty maps on error.
func buildIPMaps(ctx context.Context) (ipToNodeID map[string]string, ipToServiceState map[string]string) {
	ipToNodeID = make(map[string]string)
	ipToServiceState = make(map[string]string)

	cc, err := controllerClient()
	if err != nil {
		return
	}
	defer cc.Close()

	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.ListNodes(listCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return
	}

	healthResp, _ := client.GetClusterHealthV1(listCtx, &cluster_controllerpb.GetClusterHealthV1Request{})
	unitsByNode := make(map[string]map[string]string)
	_ = healthResp
	_ = unitsByNode

	for _, node := range resp.GetNodes() {
		nodeID := node.GetNodeId()
		for _, ip := range node.GetIdentity().GetIps() {
			ipToNodeID[ip] = nodeID
		}
		// Service state approximation: check node status field.
		state := node.GetStatus()
		for _, ip := range node.GetIdentity().GetIps() {
			ipToServiceState[ip] = state
		}
	}
	return
}

func isConverged(r *topologyStatusReport) bool {
	if r.Desired == nil {
		return true // no pool yet
	}
	if r.Applied.Pending {
		return false
	}
	if r.Applied.RestartInProgress {
		return false
	}
	if r.Lock.Held {
		return false
	}
	if !r.Health.Healthy && r.Health.Endpoint != "" && r.Health.Error != "" {
		return false
	}
	for _, n := range r.Nodes {
		if n.FingerprintMatch != "match" {
			return false
		}
	}
	return true
}

func summarizeProblems(r *topologyStatusReport) string {
	var problems []string
	if r.Desired != nil && r.Applied.Pending {
		problems = append(problems, fmt.Sprintf("applied_generation=%d < desired=%d",
			r.Applied.Generation, r.Desired.Generation))
	}
	if r.Applied.RestartInProgress {
		problems = append(problems, "restart_in_progress flag set")
	}
	if r.Lock.Held {
		problems = append(problems, "topology lock held since "+r.Lock.HeldSince)
	}
	if r.Health.Endpoint != "" && !r.Health.Healthy {
		problems = append(problems, "MinIO health endpoint: "+r.Health.Error)
	}
	for _, n := range r.Nodes {
		switch n.FingerprintMatch {
		case "mismatch":
			problems = append(problems, fmt.Sprintf("node %s(%s): fingerprint mismatch (rendered=%s)",
				n.NodeID, n.PoolIP, safePrefix(n.RenderedFingerprint, 8)))
		case "missing":
			problems = append(problems, fmt.Sprintf("node %s(%s): fingerprint not written",
				n.NodeID, n.PoolIP))
		}
	}
	if len(problems) == 0 {
		return "NOT CONVERGED (reason unknown)"
	}
	return "NOT CONVERGED: " + strings.Join(problems, "; ")
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func printTopologyReport(r *topologyStatusReport) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "\nMinIO Topology Status")
	fmt.Fprintln(w, strings.Repeat("─", 60))

	if r.Desired == nil {
		fmt.Fprintln(w, "Desired state:\tnot published (pre-pool-formation)")
	} else {
		d := r.Desired
		fmt.Fprintf(w, "Desired generation:\t%d\n", d.Generation)
		fmt.Fprintf(w, "Mode:\t%s\n", d.Mode)
		fmt.Fprintf(w, "Pool nodes:\t%s\n", strings.Join(d.PoolNodes, ", "))
		fmt.Fprintf(w, "Drives/node:\t%d\n", d.DrivesPerNode)
		fmt.Fprintf(w, "Endpoint:\t%s\n", d.Endpoint)
		fmt.Fprintf(w, "Volumes hash:\t%s\n", d.VolumesHash)
		fmt.Fprintf(w, "Expected fingerprint:\t%s\n", d.ExpectedFingerprint)
		fmt.Fprintf(w, "Written at:\t%s\n", d.WrittenAt)
	}

	fmt.Fprintln(w, strings.Repeat("─", 60))
	pendingStr := "no"
	if r.Applied.Pending {
		pendingStr = "YES"
	}
	ripStr := "no"
	if r.Applied.RestartInProgress {
		ripStr = "YES (flag set)"
	}
	lockStr := "not held"
	if r.Lock.Held {
		lockStr = "HELD since " + r.Lock.HeldSince
	}
	fmt.Fprintf(w, "Applied generation:\t%d\n", r.Applied.Generation)
	fmt.Fprintf(w, "Pending:\t%s\n", pendingStr)
	fmt.Fprintf(w, "Restart in progress:\t%s\n", ripStr)
	fmt.Fprintf(w, "Topology lock:\t%s\n", lockStr)
	if r.Applied.LastResult != "" {
		// Pretty-print: extract status field if JSON
		status := r.Applied.LastResult
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(r.Applied.LastResult), &m); err == nil {
			if s, ok := m["status"].(string); ok {
				at, _ := m["applied_at"].(string)
				if at == "" {
					at, _ = m["failed_at"].(string)
				}
				status = fmt.Sprintf("%s at %s", s, at)
			}
		}
		fmt.Fprintf(w, "Last result:\t%s\n", status)
	}

	if len(r.Nodes) > 0 {
		fmt.Fprintln(w, strings.Repeat("─", 60))
		fmt.Fprintln(w, "NODE\tIP\tRENDERED_GEN\tFINGERPRINT_MATCH\tSERVICE")
		sorted := make([]topologyNodeStatus, len(r.Nodes))
		copy(sorted, r.Nodes)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].PoolIP < sorted[j].PoolIP })
		for _, n := range sorted {
			nodeLabel := n.NodeID
			if nodeLabel == "" {
				nodeLabel = "(unmapped)"
			}
			fpMatch := n.FingerprintMatch
			if fpMatch == "match" {
				fpMatch = "✓"
			} else {
				fpMatch = "✗ " + fpMatch
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				nodeLabel, n.PoolIP, n.RenderedGeneration, fpMatch, n.ServiceState)
		}
	}

	fmt.Fprintln(w, strings.Repeat("─", 60))
	healthStr := "unknown (no endpoint)"
	if r.Health.Endpoint != "" {
		if r.Health.Healthy {
			healthStr = fmt.Sprintf("HEALTHY (HTTP %d) at %s", r.Health.StatusCode, r.Health.Endpoint)
		} else if r.Health.Error != "" {
			healthStr = fmt.Sprintf("UNHEALTHY: %s", r.Health.Error)
		}
	}
	fmt.Fprintf(w, "MinIO health:\t%s\n", healthStr)
	fmt.Fprintln(w, strings.Repeat("─", 60))

	statusLabel := "✓ CONVERGED"
	if !r.Converged {
		statusLabel = "✗ NOT CONVERGED"
	}
	fmt.Fprintf(w, "Overall:\t%s\n", statusLabel)
	if !r.Converged {
		fmt.Fprintf(w, "Reason:\t%s\n", r.Summary)
	}
	w.Flush()
	fmt.Println()
}

// clientv3 type alias used only to keep the etcd import alive.
var _ = (*clientv3.Client)(nil)
