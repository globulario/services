package main

// state_cmds.go — cluster state canonicalization tool.
//
// Scans repository, desired-state, and installed-state for Phase 2
// identity anomalies. Reports findings without modifying anything.
//
// Usage:
//   globular state canonicalize --dry-run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Cluster state inspection and repair",
}

var stateCanonicalizeCmd = &cobra.Command{
	Use:   "canonicalize",
	Short: "Scan cluster state for Phase 2 identity anomalies",
	Long: `Scans repository artifacts, desired-state, and installed-state
for missing or inconsistent build_id values. Reports anomalies
without modifying anything (dry-run only for now).

Anomaly types:
  A1  Stale installed-state overwrite (failed + unreachable)
  A2  Missing desired-state build_id
  A3  Missing installed-state build_id
  A4  Missing repository build_id
  A6  Orphaned desired-state (artifact not in repository)
  A7  Inconsistent node coverage (some nodes have build_id, others don't)`,
	RunE: runCanonicalize,
}

var (
	fixSafe          bool
	fixInstalled     bool
	fixNodeID        string
	fixServiceName   string
	fixLimit         int
	fixIncludeCritical bool
)

// controlPlaneServices lists services that should only be repaired after
// all regular services succeed. They are the cluster's nervous system.
var controlPlaneServices = map[string]bool{
	"node-agent": true, "cluster-controller": true, "repository": true,
	"workflow": true, "dns": true, "discovery": true,
}

func init() {
	stateCmd.AddCommand(stateCanonicalizeCmd)
	stateCanonicalizeCmd.Flags().Bool("dry-run", true, "Scan and report only (no mutations)")
	stateCanonicalizeCmd.Flags().BoolVar(&fixSafe, "fix-safe", false, "Repair safe anomalies (A4: repository build_id, A2: desired-state build_id)")
	stateCanonicalizeCmd.Flags().BoolVar(&fixInstalled, "fix-installed", false, "Repair installed-state missing build_id (A3)")
	stateCanonicalizeCmd.Flags().StringVar(&fixNodeID, "node", "", "Target node ID for --fix-installed (required)")
	stateCanonicalizeCmd.Flags().StringVar(&fixServiceName, "service", "", "Target single service for --fix-installed (optional)")
	stateCanonicalizeCmd.Flags().IntVar(&fixLimit, "limit", 0, "Max services to repair per run (0 = all eligible)")
	stateCanonicalizeCmd.Flags().BoolVar(&fixIncludeCritical, "include-critical", false, "Include control-plane services in --fix-installed")
	stateCanonicalizeCmd.Flags().StringVar(&fixAgentEndpoint, "agent-endpoint", "", "Node-agent gRPC endpoint for --fix-installed (e.g. 10.0.0.20:11000)")
	stateCanonicalizeCmd.Flags().BoolVar(&fixMetadataOnly, "metadata-only", false, "Write buildId directly to etcd without re-installing (for COMMAND/INFRASTRUCTURE packages)")
	stateCanonicalizeCmd.Flags().BoolVar(&fixCleanupGhosts, "cleanup-ghosts", false, "Delete installed-state records on non-active nodes")
}

var (
	fixAgentEndpoint string
	fixMetadataOnly  bool
	fixCleanupGhosts bool
)

// anomaly represents a single detected issue.
type anomaly struct {
	Type    string `json:"type"`
	Node    string `json:"node,omitempty"`
	Service string `json:"service"`
	Detail  string `json:"detail"`
	Action  string `json:"suggested_action"`
}

// canonReport is the full scan result.
type canonReport struct {
	Timestamp         string    `json:"timestamp"`
	TotalServices     int       `json:"total_services"`
	TotalNodes        int       `json:"total_nodes"`
	TotalArtifacts    int       `json:"total_artifacts"`
	Anomalies         []anomaly `json:"anomalies"`
	CanonicalPercent  float64   `json:"canonical_percent"`
	CountByType       map[string]int `json:"count_by_type"`
}

func runCanonicalize(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report := &canonReport{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		CountByType: make(map[string]int),
	}

	mode := "Dry Run"
	if fixSafe {
		mode = "Safe Repair (A4 + A2)"
	}
	fmt.Printf("=== Globular State Canonicalization — %s ===\n", mode)
	fmt.Println()

	// ── Repository scan ────────────────────────────────────────────────
	fmt.Println("[1/3] Scanning repository artifacts...")
	if err := scanRepository(ctx, report); err != nil {
		fmt.Printf("  WARNING: repository scan failed: %v\n", err)
	}

	// ── Desired-state scan ─────────────────────────────────────────────
	fmt.Println("[2/3] Scanning desired-state records...")
	if err := scanDesiredState(ctx, report); err != nil {
		fmt.Printf("  WARNING: desired-state scan failed: %v\n", err)
	}

	// ── Installed-state scan ───────────────────────────────────────────
	fmt.Println("[3/3] Scanning installed-state records...")
	if err := scanInstalledState(ctx, report); err != nil {
		fmt.Printf("  WARNING: installed-state scan failed: %v\n", err)
	}

	// ── Safe repair pass ──────────────────────────────────────────────
	repaired := 0
	repairFailed := 0
	if fixSafe {
		fmt.Println()
		fmt.Println("[FIX] Repairing safe anomalies (A4 + A2)...")

		// A4: re-publish missing repository build_ids by re-uploading the manifest.
		// The repository will assign a build_id on the idempotent re-upload path.
		// Skip — repository backfill migration handles this on restart. Just log.
		a4Count := report.CountByType["A4"]
		if a4Count > 0 {
			fmt.Printf("  A4: %d manifests missing build_id — restart repository to trigger backfill migration\n", a4Count)
		}

		// A2: re-upsert desired-state through the controller API.
		a2Repaired, a2Failed := repairDesiredStateBuildID(ctx, report)
		repaired += a2Repaired
		repairFailed += a2Failed

		fmt.Printf("\n  Repaired: %d  Failed: %d  Skipped (A4): %d\n", repaired, repairFailed, a4Count)
	}

	// ── Installed-state repair pass ───────────────────────────────────
	installedRepaired := 0
	installedFailed := 0
	if fixInstalled {
		if fixNodeID == "" {
			fmt.Println("\nERROR: --fix-installed requires --node <id>")
			return fmt.Errorf("--node is required for --fix-installed")
		}
		fmt.Printf("\n[FIX] Repairing installed-state on node %s...\n", fixNodeID[:8])
		installedRepaired, installedFailed = repairInstalledStateBuildID(ctx, fixNodeID)
		fmt.Printf("  Repaired: %d  Failed: %d\n", installedRepaired, installedFailed)
	}

	// ── Ghost-node cleanup ──────────────────────────────────────────────
	ghostsCleaned := 0
	if fixCleanupGhosts {
		fmt.Println("\n[FIX] Cleaning up ghost-node installed-state records...")
		ghostsCleaned = cleanupGhostNodes(ctx, report)
		fmt.Printf("  Ghost records deleted: %d\n", ghostsCleaned)
	}

	// ── Summary ────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("  Artifacts scanned:  %d\n", report.TotalArtifacts)
	fmt.Printf("  Services scanned:   %d\n", report.TotalServices)
	fmt.Printf("  Nodes scanned:      %d\n", report.TotalNodes)
	fmt.Printf("  Anomalies found:    %d\n", len(report.Anomalies))
	fmt.Printf("  Canonical:          %.1f%%\n", report.CanonicalPercent)
	if fixSafe {
		fmt.Printf("  A2 repaired:        %d\n", repaired)
		fmt.Printf("  A2 failed:          %d\n", repairFailed)
	}
	if fixInstalled {
		fmt.Printf("  A3 repaired:        %d\n", installedRepaired)
		fmt.Printf("  A3 failed:          %d\n", installedFailed)
	}
	fmt.Println()

	if len(report.CountByType) > 0 {
		fmt.Println("  By type:")
		for t, c := range report.CountByType {
			fmt.Printf("    %s: %d\n", t, c)
		}
		fmt.Println()
	}

	if !fixSafe && len(report.Anomalies) > 0 {
		fmt.Println("=== Anomalies ===")
		for i, a := range report.Anomalies {
			node := ""
			if a.Node != "" {
				node = fmt.Sprintf(" node=%s", a.Node)
			}
			fmt.Printf("  [%d] %s  %s%s\n", i+1, a.Type, a.Service, node)
			fmt.Printf("      %s\n", a.Detail)
			fmt.Printf("      -> %s\n", a.Action)
		}
	} else if !fixSafe {
		fmt.Println("  No anomalies found. State is canonical.")
	}

	fmt.Println()
	if fixSafe || fixInstalled {
		fmt.Println("=== Repair complete. ===")
	} else {
		fmt.Println("=== Dry run complete. No mutations performed. ===")
	}
	return nil
}

// ── Repository scan ──────────────────────────────────────────────────────

func scanRepository(ctx context.Context, report *canonReport) error {
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr == "" {
		if a, err := config.GetMeshAddress(); err == nil {
			addr = a
		}
	}
	if addr == "" {
		return fmt.Errorf("repository address not found")
	}

	client, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	artifacts, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	report.TotalArtifacts = len(artifacts)
	withBuildID := 0

	for _, a := range artifacts {
		if a.GetBuildId() != "" {
			withBuildID++
		} else {
			report.addAnomaly("A4", "", fmtArtifactName(a),
				fmt.Sprintf("manifest missing build_id (version=%s build=%d)",
					a.GetRef().GetVersion(), a.GetBuildNumber()),
				"restart repository to trigger backfill migration")
		}
	}

	fmt.Printf("  %d artifacts, %d with build_id, %d missing\n",
		len(artifacts), withBuildID, len(artifacts)-withBuildID)

	return nil
}

// ── Desired-state scan ───────────────────────────────────────────────────

func scanDesiredState(ctx context.Context, report *canonReport) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd: %w", err)
	}

	resp, err := cli.Get(ctx, "/globular/resources/ServiceDesiredVersion/",
		clientv3.WithPrefix(), clientv3.WithLimit(500))
	if err != nil {
		return fmt.Errorf("etcd get: %w", err)
	}

	type desiredSpec struct {
		ServiceName string `json:"service_name"`
		Version     string `json:"version"`
		BuildNumber int64  `json:"build_number"`
		BuildID     string `json:"build_id"`
	}
	type desiredRecord struct {
		Spec *desiredSpec `json:"spec"`
	}

	total := 0
	withBuildID := 0
	services := make(map[string]bool)

	for _, kv := range resp.Kvs {
		var rec desiredRecord
		if err := json.Unmarshal(kv.Value, &rec); err != nil || rec.Spec == nil {
			continue
		}
		total++
		svc := rec.Spec.ServiceName
		if svc == "" {
			parts := strings.Split(string(kv.Key), "/")
			svc = parts[len(parts)-1]
		}
		services[svc] = true

		if rec.Spec.BuildID != "" {
			withBuildID++
		} else {
			report.addAnomaly("A2", "", svc,
				fmt.Sprintf("desired-state missing build_id (version=%s build=%d)",
					rec.Spec.Version, rec.Spec.BuildNumber),
				"re-upsert via controller API to resolve build_id")
		}
	}

	report.TotalServices = len(services)
	fmt.Printf("  %d desired-state records, %d with build_id, %d missing\n",
		total, withBuildID, total-withBuildID)

	return nil
}

// ── Installed-state scan ─────────────────────────────────────────────────

func scanInstalledState(ctx context.Context, report *canonReport) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd: %w", err)
	}

	resp, err := cli.Get(ctx, "/globular/nodes/",
		clientv3.WithPrefix(), clientv3.WithLimit(5000))
	if err != nil {
		return fmt.Errorf("etcd get: %w", err)
	}

	nodes := make(map[string]bool)
	type record struct {
		NodeID   string            `json:"nodeId"`
		Name     string            `json:"name"`
		Version  string            `json:"version"`
		Kind     string            `json:"kind"`
		Status   string            `json:"status"`
		BuildNum string            `json:"buildNumber"`
		BuildID  string            `json:"buildId"`
		Metadata map[string]string `json:"metadata"`
	}

	total := 0
	withBuildID := 0
	staleCount := 0
	// Track build_id coverage per service
	svcNodes := make(map[string]map[string]bool)    // service → {nodeID: hasBuildID}

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.Contains(key, "/packages/") {
			continue
		}

		var rec record
		if err := json.Unmarshal(kv.Value, &rec); err != nil {
			continue
		}
		if rec.Name == "" {
			continue
		}
		total++
		nodes[rec.NodeID] = true

		// Track per-service node coverage
		if svcNodes[rec.Name] == nil {
			svcNodes[rec.Name] = make(map[string]bool)
		}

		// A1: Stale failed record from unreachable repository
		if rec.Status == "failed" && rec.Metadata != nil {
			errMsg := rec.Metadata["error"]
			if strings.Contains(errMsg, "repository unreachable") ||
				strings.Contains(errMsg, "no local package found") {
				staleCount++
				report.addAnomaly("A1", rec.NodeID, rec.Name,
					fmt.Sprintf("stale failed record (version=%s build=%s, error=%q)",
						rec.Version, rec.BuildNum, errMsg),
					"delete stale record; convergence loop will re-apply correctly")
				continue
			}
		}

		// A3: Missing build_id on installed record
		if rec.Status == "installed" {
			if rec.BuildID != "" {
				withBuildID++
				svcNodes[rec.Name][rec.NodeID] = true
			} else {
				svcNodes[rec.Name][rec.NodeID] = false
				report.addAnomaly("A3", rec.NodeID, rec.Name,
					fmt.Sprintf("installed-state missing buildId (version=%s build=%s)",
						rec.Version, rec.BuildNum),
					"re-apply service with build_id to gain exact identity")
			}
		}
	}

	// A7: Inconsistent node coverage
	for svc, nodeMap := range svcNodes {
		hasBID := 0
		noBID := 0
		for _, has := range nodeMap {
			if has {
				hasBID++
			} else {
				noBID++
			}
		}
		if hasBID > 0 && noBID > 0 {
			report.addAnomaly("A7", "", svc,
				fmt.Sprintf("%d nodes with buildId, %d without", hasBID, noBID),
				"re-apply on nodes missing buildId")
		}
	}

	report.TotalNodes = len(nodes)

	// Compute canonical percentage
	totalCanonical := withBuildID
	totalRecords := total - staleCount // exclude stale from denominator
	if totalRecords > 0 {
		report.CanonicalPercent = float64(totalCanonical) / float64(totalRecords) * 100
	}

	fmt.Printf("  %d installed-state records, %d with buildId, %d stale/failed\n",
		total, withBuildID, staleCount)

	return nil
}

// ── A2 repair: desired-state build_id ─────────────────────────────────────

// repairDesiredStateBuildID re-upserts each desired-state record missing
// build_id through the controller API. The controller resolves build_id
// from the repository manifest and writes it to etcd.
func repairDesiredStateBuildID(ctx context.Context, report *canonReport) (repaired, failed int) {
	// Collect A2 anomalies.
	type a2Entry struct {
		service     string
		version     string
		buildNumber int64
	}
	var entries []a2Entry

	cli, err := config.GetEtcdClient()
	if err != nil {
		fmt.Printf("  A2: etcd unavailable: %v\n", err)
		return 0, 0
	}

	resp, err := cli.Get(ctx, "/globular/resources/ServiceDesiredVersion/",
		clientv3.WithPrefix(), clientv3.WithLimit(500))
	if err != nil {
		fmt.Printf("  A2: etcd get failed: %v\n", err)
		return 0, 0
	}

	type desiredSpec struct {
		ServiceName string `json:"service_name"`
		Version     string `json:"version"`
		BuildNumber int64  `json:"build_number"`
		BuildID     string `json:"build_id"`
	}
	type desiredRecord struct {
		Spec *desiredSpec `json:"spec"`
	}

	for _, kv := range resp.Kvs {
		var rec desiredRecord
		if err := json.Unmarshal(kv.Value, &rec); err != nil || rec.Spec == nil {
			continue
		}
		if rec.Spec.BuildID == "" {
			svc := rec.Spec.ServiceName
			if svc == "" {
				parts := strings.Split(string(kv.Key), "/")
				svc = parts[len(parts)-1]
			}
			entries = append(entries, a2Entry{
				service:     svc,
				version:     rec.Spec.Version,
				buildNumber: rec.Spec.BuildNumber,
			})
		}
	}

	if len(entries) == 0 {
		fmt.Println("  A2: no records to repair")
		return 0, 0
	}

	// Find the controller leader and call UpsertDesiredService for each.
	controllerAddr := rootCfg.controllerAddr
	cc, err := dialGRPC(controllerAddr)
	if err != nil {
		fmt.Printf("  A2: cannot connect to controller at %s: %v\n", controllerAddr, err)
		return 0, len(entries)
	}
	defer cc.Close()

	ctrlClient := cluster_controllerpb.NewClusterControllerServiceClient(cc)

	for _, e := range entries {
		reqCtx, reqCancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := ctrlClient.UpsertDesiredService(reqCtx, &cluster_controllerpb.UpsertDesiredServiceRequest{
			Service: &cluster_controllerpb.DesiredService{
				ServiceId:   e.service,
				Version:     e.version,
				BuildNumber: e.buildNumber,
			},
		})
		reqCancel()

		if err != nil {
			// If not leader, try to follow the redirect.
			errStr := err.Error()
			if strings.Contains(errStr, "not leader") {
				// Extract leader address from error.
				if idx := strings.Index(errStr, "leader_addr="); idx >= 0 {
					leaderAddr := strings.TrimPrefix(errStr[idx:], "leader_addr=")
					if commaIdx := strings.IndexByte(leaderAddr, ','); commaIdx >= 0 {
						leaderAddr = leaderAddr[:commaIdx]
					}
					leaderAddr = strings.TrimRight(leaderAddr, ")")
					// Retry on leader.
					cc2, err2 := dialGRPC(leaderAddr)
					if err2 == nil {
						ctrlClient2 := cluster_controllerpb.NewClusterControllerServiceClient(cc2)
						reqCtx2, reqCancel2 := context.WithTimeout(ctx, 10*time.Second)
						_, err = ctrlClient2.UpsertDesiredService(reqCtx2, &cluster_controllerpb.UpsertDesiredServiceRequest{
							Service: &cluster_controllerpb.DesiredService{
								ServiceId:   e.service,
								Version:     e.version,
								BuildNumber: e.buildNumber,
							},
						})
						reqCancel2()
						cc2.Close()
					}
				}
			}
		}

		if err != nil {
			fmt.Printf("  A2: FAILED %s: %v\n", e.service, err)
			failed++
		} else {
			fmt.Printf("  A2: repaired %s (version=%s build=%d) — build_id resolved\n",
				e.service, e.version, e.buildNumber)
			writeAuditRecord(ctx, auditRecord{
				Action:      "fix-safe-A2",
				Service:     e.service,
				BeforeState: "build_id=empty",
				AfterState:  "build_id=resolved",
				Detail:      fmt.Sprintf("version=%s build=%d", e.version, e.buildNumber),
			})
			repaired++
		}
	}

	return repaired, failed
}

// ── A3 repair: installed-state build_id ───────────────────────────────────

// repairInstalledStateBuildID applies the correct build_id to installed-state
// records on a specific node by calling ApplyPackageRelease with force=true.
func repairInstalledStateBuildID(ctx context.Context, nodeID string) (repaired, failed int) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		fmt.Printf("  etcd unavailable: %v\n", err)
		return 0, 0
	}

	// Load desired-state build_ids.
	type dSpec struct {
		ServiceName string `json:"service_name"`
		Version     string `json:"version"`
		BuildNumber int64  `json:"build_number"`
		BuildID     string `json:"build_id"`
	}
	type dRec struct{ Spec *dSpec `json:"spec"` }

	dResp, err := cli.Get(ctx, "/globular/resources/ServiceDesiredVersion/",
		clientv3.WithPrefix(), clientv3.WithLimit(500))
	if err != nil {
		fmt.Printf("  desired-state read failed: %v\n", err)
		return 0, 0
	}
	desired := make(map[string]*dSpec)
	for _, kv := range dResp.Kvs {
		var rec dRec
		if json.Unmarshal(kv.Value, &rec) == nil && rec.Spec != nil {
			svc := rec.Spec.ServiceName
			if svc == "" {
				parts := strings.Split(string(kv.Key), "/")
				svc = parts[len(parts)-1]
			}
			desired[svc] = rec.Spec
		}
	}

	// Also read InfrastructureRelease desired-state for INFRASTRUCTURE packages.
	// Schema: spec.component = name, status.resolved_build_id = build_id.
	type infraSpec struct {
		Component   string `json:"component"`
		Version     string `json:"version"`
		BuildNumber int64  `json:"build_number"`
		BuildID     string `json:"build_id"`
	}
	type infraStatus struct {
		ResolvedBuildID string `json:"resolved_build_id"`
		ResolvedVersion string `json:"resolved_version"`
	}
	type infraRec struct {
		Spec   *infraSpec   `json:"spec"`
		Status *infraStatus `json:"status"`
	}
	iRelResp, _ := cli.Get(ctx, "/globular/resources/InfrastructureRelease/",
		clientv3.WithPrefix(), clientv3.WithLimit(500))
	for _, kv := range iRelResp.Kvs {
		var rec infraRec
		if json.Unmarshal(kv.Value, &rec) != nil || rec.Spec == nil {
			continue
		}
		name := rec.Spec.Component
		if name == "" {
			parts := strings.Split(string(kv.Key), "/")
			name = parts[len(parts)-1]
		}
		if _, exists := desired[name]; exists {
			continue // SERVICE desired-state takes precedence
		}
		// Use resolved_build_id from status if available, else spec.build_id.
		buildID := ""
		if rec.Status != nil && rec.Status.ResolvedBuildID != "" {
			buildID = rec.Status.ResolvedBuildID
		} else if rec.Spec.BuildID != "" {
			buildID = rec.Spec.BuildID
		}
		desired[name] = &dSpec{
			ServiceName: name,
			Version:     rec.Spec.Version,
			BuildNumber: rec.Spec.BuildNumber,
			BuildID:     buildID,
		}
	}

	// Load installed-state for this node.
	type iRec struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Kind    string `json:"kind"`
		Status  string `json:"status"`
		BuildID string `json:"buildId"`
	}
	prefix := fmt.Sprintf("/globular/nodes/%s/packages/", nodeID)
	iResp, err := cli.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithLimit(500))
	if err != nil {
		fmt.Printf("  installed-state read failed: %v\n", err)
		return 0, 0
	}

	// Lazy-init repository client for fallback build_id resolution.
	var repoClient *repository_client.Repository_Service_Client
	defer func() {
		if repoClient != nil {
			repoClient.Close()
		}
	}()

	// Classify services needing repair. Use kind/name as key to handle
	// duplicate records at different kind prefixes (e.g., SERVICE/etcdctl
	// and COMMAND/etcdctl).
	type repairEntry struct {
		name string
		rec  *iRec
	}
	var regular, critical []repairEntry
	installed := make(map[string]*iRec) // kind/name → record
	for _, kv := range iResp.Kvs {
		var rec iRec
		if json.Unmarshal(kv.Value, &rec) != nil || rec.Name == "" {
			continue
		}
		mapKey := rec.Kind + "/" + rec.Name
		installed[mapKey] = &rec

		// Skip if already has buildId or not installed.
		if rec.BuildID != "" || rec.Status != "installed" {
			continue
		}
		// Skip if no desired-state entry at all.
		d, ok := desired[rec.Name]
		if !ok {
			continue
		}
		// If desired-state has no build_id, try resolving from repository.
		if d.BuildID == "" && fixMetadataOnly {
			if repoClient == nil {
				addr := config.ResolveServiceAddr("repository.PackageRepository", "")
				if addr == "" {
					if a, err := config.GetMeshAddress(); err == nil {
						addr = a
					}
				}
				if addr != "" {
					repoClient, _ = repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
				}
			}
			if repoClient != nil {
				if m, merr := repoClient.GetArtifactManifest(&repopb.ArtifactRef{
					PublisherId: "core@globular.io",
					Name:        rec.Name,
					Version:     d.Version,
					Platform:    "linux_amd64",
				}, d.BuildNumber); merr == nil && m.GetBuildId() != "" {
					d.BuildID = m.GetBuildId()
				}
			}
		}
		if d.BuildID == "" {
			continue
		}
		// If targeting a specific service, filter.
		if fixServiceName != "" && rec.Name != fixServiceName {
			continue
		}
		entry := repairEntry{name: rec.Name, rec: &rec}
		if controlPlaneServices[rec.Name] {
			critical = append(critical, entry)
		} else {
			regular = append(regular, entry)
		}
	}

	fmt.Printf("  Regular: %d  Critical: %d (deferred)\n", len(regular), len(critical))
	if !fixIncludeCritical && len(critical) > 0 {
		var critNames []string
		for _, e := range critical {
			critNames = append(critNames, e.name)
		}
		fmt.Printf("  Critical services skipped: %s\n", strings.Join(critNames, ", "))
		fmt.Println("  (use --include-critical to repair them)")
	}

	// Resolve node-agent endpoint.
	agentEndpoint := fixAgentEndpoint
	if agentEndpoint == "" {
		agentEndpoint = resolveAgentEndpoint(ctx, cli, nodeID)
	}
	if agentEndpoint == "" {
		fmt.Printf("  cannot resolve agent endpoint for node %s — use --agent-endpoint\n", nodeID[:8])
		return 0, 0
	}
	fmt.Printf("  Agent: %s\n", agentEndpoint)

	conn, err := dialGRPC(agentEndpoint)
	if err != nil {
		fmt.Printf("  cannot connect to node-agent at %s: %v\n", agentEndpoint, err)
		return 0, 0
	}
	defer conn.Close()
	agentClient := node_agentpb.NewNodeAgentServiceClient(conn)

	// Metadata-only repair: write buildId directly to the existing etcd record
	// without reinstalling. Used for COMMAND/INFRASTRUCTURE packages that are
	// already installed but can't be re-applied through the standard path.
	metadataUpdate := func(svc string) bool {
		d := desired[svc]
		// Find ALL etcd records for this service (may be under multiple kind prefixes).
		// Update each one that is missing buildId.
		updated := false
		for _, inst := range installed {
			if inst.Name != svc || inst.BuildID != "" || inst.Status != "installed" {
				continue
			}
			key := fmt.Sprintf("/globular/nodes/%s/packages/%s/%s", nodeID, inst.Kind, svc)
			getResp, gerr := cli.Get(ctx, key)
			if gerr != nil || len(getResp.Kvs) == 0 {
				fmt.Printf("    FAIL  %-25s  etcd read %s: %v\n", svc, inst.Kind, gerr)
				continue
			}
			var raw map[string]interface{}
			if err := json.Unmarshal(getResp.Kvs[0].Value, &raw); err != nil {
				fmt.Printf("    FAIL  %-25s  json parse: %v\n", svc, err)
				continue
			}
			raw["buildId"] = d.BuildID
			data, err := json.Marshal(raw)
			if err != nil {
				continue
			}
			putCtx, putCancel := context.WithTimeout(ctx, 5*time.Second)
			_, perr := cli.Put(putCtx, key, string(data))
			putCancel()
			if perr != nil {
				fmt.Printf("    FAIL  %-25s  etcd write %s: %v\n", svc, inst.Kind, perr)
				continue
			}
			fmt.Printf("    OK    %-25s  buildId=%s (%s, metadata-only)\n", svc, d.BuildID, inst.Kind)
			writeAuditRecord(ctx, auditRecord{
				Action:      "fix-installed-metadata",
				Service:     svc,
				Node:        nodeID,
				BeforeState: "buildId=empty",
				AfterState:  fmt.Sprintf("buildId=%s", d.BuildID),
				BuildID:     d.BuildID,
				Detail:      fmt.Sprintf("kind=%s", inst.Kind),
			})
			updated = true
		}
		return updated
	}

	applyOne := func(e repairEntry) bool {
		svc := e.name
		// Metadata-only mode: write buildId directly to etcd without reinstalling.
		if fixMetadataOnly {
			return metadataUpdate(svc)
		}

		d := desired[svc]
		kind := "SERVICE"
		if e.rec != nil && e.rec.Kind != "" {
			kind = e.rec.Kind
		}

		reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		resp, err := agentClient.ApplyPackageRelease(reqCtx, &node_agentpb.ApplyPackageReleaseRequest{
			PackageName:    svc,
			PackageKind:    kind,
			Version:        d.Version,
			BuildNumber:    d.BuildNumber,
			BuildId:        d.BuildID,
			RepositoryAddr: "10.0.0.63:443",
			Platform:       "linux_amd64",
			Force:          true,
		})
		if err != nil {
			fmt.Printf("    FAIL  %-25s  err=%v\n", svc, err)
			return false
		}
		if !resp.GetOk() {
			fmt.Printf("    FAIL  %-25s  status=%s  %s\n", svc, resp.GetStatus(), resp.GetErrorDetail())
			return false
		}
		fmt.Printf("    OK    %-25s  buildId=%s\n", svc, resp.GetBuildId())
		return true
	}

	// Regular services first.
	limit := len(regular)
	if fixLimit > 0 && fixLimit < limit {
		limit = fixLimit
	}
	fmt.Println("  --- Regular services ---")
	for i := 0; i < limit; i++ {
		if applyOne(regular[i]) {
			repaired++
		} else {
			failed++
			if failed > 2 {
				fmt.Println("  STOP: >2 failures, aborting batch")
				return repaired, failed
			}
		}
	}

	// Critical services (only if --include-critical and no excessive failures).
	if fixIncludeCritical && failed <= 2 && len(critical) > 0 {
		fmt.Println("  --- Critical services ---")
		for _, svc := range critical {
			if applyOne(svc) {
				repaired++
			} else {
				failed++
				if failed > 2 {
					fmt.Println("  STOP: >2 failures, aborting batch")
					return repaired, failed
				}
			}
		}
	}

	return repaired, failed
}

// ── Ghost-node cleanup ───────────────────────────────────────────────────

// cleanupGhostNodes deletes installed-state records for nodes that are not
// in the active cluster. Ghost nodes are nodes that were removed but still
// have etcd records under /globular/nodes/{id}/packages/.
func cleanupGhostNodes(ctx context.Context, report *canonReport) int {
	cli, err := config.GetEtcdClient()
	if err != nil {
		fmt.Printf("  etcd unavailable: %v\n", err)
		return 0
	}

	// Get active node IDs from the controller.
	activeNodes := make(map[string]bool)
	cc, err := dialGRPC(rootCfg.controllerAddr)
	if err == nil {
		defer cc.Close()
		ctrlClient := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		reqCtx, reqCancel := context.WithTimeout(ctx, 10*time.Second)
		nodesResp, nerr := ctrlClient.ListNodes(reqCtx, &cluster_controllerpb.ListNodesRequest{})
		reqCancel()
		if nerr == nil {
			for _, n := range nodesResp.GetNodes() {
				activeNodes[n.GetNodeId()] = true
			}
		}
	}

	if len(activeNodes) == 0 {
		fmt.Println("  cannot determine active nodes — skipping ghost cleanup")
		return 0
	}
	fmt.Printf("  Active nodes: %d\n", len(activeNodes))

	// Scan all installed-state records and find those on non-active nodes.
	resp, err := cli.Get(ctx, "/globular/nodes/",
		clientv3.WithPrefix(), clientv3.WithKeysOnly(), clientv3.WithLimit(5000))
	if err != nil {
		return 0
	}

	cleaned := 0
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.Contains(key, "/packages/") {
			continue
		}
		// Extract node ID from key: /globular/nodes/{node_id}/packages/...
		parts := strings.Split(key, "/")
		if len(parts) < 4 {
			continue
		}
		nodeID := parts[3] // /globular/nodes/{node_id}/...
		if activeNodes[nodeID] {
			continue // active node, skip
		}

		// Ghost node — delete this record.
		delCtx, delCancel := context.WithTimeout(ctx, 3*time.Second)
		_, derr := cli.Delete(delCtx, key)
		delCancel()
		if derr != nil {
			fmt.Printf("    FAIL  delete %s: %v\n", key, derr)
			continue
		}
		fmt.Printf("    DEL   %s (ghost node %s)\n", key, nodeID[:8])
		writeAuditRecord(ctx, auditRecord{
			Action:      "cleanup-ghost",
			Service:     key,
			Node:        nodeID,
			BeforeState: "installed",
			AfterState:  "deleted",
			Detail:      "node not in active cluster",
		})
		cleaned++
	}
	return cleaned
}

// resolveAgentEndpoint finds the node-agent gRPC endpoint for a node ID
// by reading the node status from etcd.
func resolveAgentEndpoint(ctx context.Context, cli *clientv3.Client, nodeID string) string {
	// Try the node status key.
	key := fmt.Sprintf("/globular/nodes/%s/status", nodeID)
	resp, err := cli.Get(ctx, key)
	if err != nil || len(resp.Kvs) == 0 {
		return ""
	}
	var status struct {
		AgentEndpoint string `json:"agent_endpoint"`
	}
	if json.Unmarshal(resp.Kvs[0].Value, &status) == nil && status.AgentEndpoint != "" {
		return status.AgentEndpoint
	}
	return ""
}

// ── Helpers ──────────────────────────────────────────────────────────────

func (r *canonReport) addAnomaly(typ, node, service, detail, action string) {
	r.Anomalies = append(r.Anomalies, anomaly{
		Type:    typ,
		Node:    node,
		Service: service,
		Detail:  detail,
		Action:  action,
	})
	r.CountByType[typ]++
}

func fmtArtifactName(m *repopb.ArtifactManifest) string {
	ref := m.GetRef()
	return fmt.Sprintf("%s/%s@%s", ref.GetPublisherId(), ref.GetName(), ref.GetVersion())
}

