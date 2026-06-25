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

	"github.com/globulario/services/golang/audittrail"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/types/known/emptypb"
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
	fixSafe            bool
	fixInstalled       bool
	fixNodeID          string
	fixServiceName     string
	fixLimit           int
	fixIncludeCritical bool
)

// controlPlaneServices lists services that should only be repaired after
// all regular services succeed. They are the cluster's nervous system.
var controlPlaneServices = map[string]bool{
	"node-agent": true, "cluster-controller": true, "repository": true,
	"workflow": true, "dns": true,
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
	Timestamp        string          `json:"timestamp"`
	TotalServices    int             `json:"total_services"`
	TotalNodes       int             `json:"total_nodes"`
	TotalArtifacts   int             `json:"total_artifacts"`
	Anomalies        []anomaly       `json:"anomalies"`
	CanonicalPercent float64         `json:"canonical_percent"`
	CountByType      map[string]int  `json:"count_by_type"`
	DesiredServices  map[string]bool `json:"-"` // services with desired-state entries
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
	provenanceBackfilled := 0
	provenanceBackfillFailed := 0
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
		provenanceBackfilled, provenanceBackfillFailed = repairDesiredWriteProvenanceLegacy(ctx)

		fmt.Printf("\n  Repaired: %d  Failed: %d  Skipped (A4): %d\n", repaired, repairFailed, a4Count)
		fmt.Printf("  Provenance backfilled: %d  Failed: %d\n", provenanceBackfilled, provenanceBackfillFailed)
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
		fmt.Printf("  Legacy provenance:  %d\n", provenanceBackfilled)
		fmt.Printf("  Prov backfill fail: %d\n", provenanceBackfillFailed)
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

// scanDesiredState consumes the typed GetDesiredState RPC on
// cluster_controller. The controller's listAllDesiredServices already
// merges SDV + InfrastructureRelease + ApplicationRelease into a flat
// DesiredService list keyed by canonical service_id, so a single RPC
// replaces the prior pair of /globular/resources/{ServiceDesiredVersion,
// InfrastructureRelease}/ etcd scans.
//
// Anchored by:
//
//	invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//	forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc
func scanDesiredState(ctx context.Context, report *canonReport) error {
	cc, err := controllerClient()
	if err != nil {
		return fmt.Errorf("dial cluster_controller: %w", err)
	}
	defer cc.Close()

	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
	resp, err := client.GetDesiredState(ctx, &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("GetDesiredState: %w", err)
	}

	total := 0
	withBuildID := 0
	services := make(map[string]bool)

	for _, svc := range resp.GetServices() {
		total++
		name := svc.GetServiceId()
		if name == "" {
			continue
		}
		// service_id is the canonical short name (controller strips
		// any publisher prefix in canonicalServiceName before
		// populating).
		services[name] = true

		if svc.GetBuildId() != "" {
			withBuildID++
			continue
		}
		report.addAnomaly("A2", "", name,
			fmt.Sprintf("desired-state missing build_id (version=%s build=%d)",
				svc.GetVersion(), svc.GetBuildNumber()),
			"re-upsert via controller API to resolve build_id")
	}

	report.TotalServices = len(services)
	report.DesiredServices = services
	fmt.Printf("  %d desired-state records, %d with build_id, %d missing\n",
		total, withBuildID, total-withBuildID)

	return nil
}

// ── Installed-state scan ─────────────────────────────────────────────────

// scanInstalledState consumes typed RPCs: ListNodes on the
// cluster_controller (to enumerate nodes + their agent endpoints) and
// node_agent.ListInstalledPackages per node (the owner's typed view of
// L3 installed state). Replaces the prior raw /globular/nodes/ etcd
// prefix scan — owned by node_agent, never by globularcli — per
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
//
// Degraded-read semantics: if any node-agent is unreachable or its
// agent endpoint is missing from the controller's NodeRecord, an
// explicit A8 anomaly is recorded ("node unreachable — installed state
// not observed"). The canonical-percent denominator excludes those
// nodes so a partial observation is never mistaken for canonical
// truth. Operators see a clearly degraded report rather than a falsely
// healthy one.
func scanInstalledState(ctx context.Context, report *canonReport) error {
	cc, err := controllerClient()
	if err != nil {
		return fmt.Errorf("dial cluster_controller: %w", err)
	}
	defer cc.Close()
	ctrlClient := cluster_controllerpb.NewClusterControllerServiceClient(cc)

	lnCtx, lnCancel := context.WithTimeout(ctx, 10*time.Second)
	nodesResp, err := ctrlClient.ListNodes(lnCtx, &cluster_controllerpb.ListNodesRequest{})
	lnCancel()
	if err != nil {
		return fmt.Errorf("ListNodes: %w", err)
	}

	nodes := make(map[string]bool)
	total := 0
	withBuildID := 0
	staleCount := 0
	unreachableNodes := 0
	// Track build_id coverage per service.
	svcNodes := make(map[string]map[string]bool) // service → {nodeID: hasBuildID}

	for _, node := range nodesResp.GetNodes() {
		nid := node.GetNodeId()
		if nid == "" {
			continue
		}
		nodes[nid] = true
		endpoint := strings.TrimSpace(node.GetAgentEndpoint())
		if endpoint == "" {
			unreachableNodes++
			report.addAnomaly("A8", nid, "",
				"node has no agent_endpoint in cluster controller — installed state not observed",
				"verify node agent registered correctly; rejoin if necessary")
			continue
		}

		nc, dErr := nodeClientWith(endpoint)
		if dErr != nil {
			unreachableNodes++
			report.addAnomaly("A8", nid, "",
				fmt.Sprintf("node-agent unreachable at %s: %v — installed state not observed", endpoint, dErr),
				"check node connectivity; re-run after agent is reachable")
			continue
		}

		agent := node_agentpb.NewNodeAgentServiceClient(nc)
		listCtx, listCancel := context.WithTimeout(ctx, 10*time.Second)
		pkgResp, lpErr := agent.ListInstalledPackages(listCtx, &node_agentpb.ListInstalledPackagesRequest{})
		listCancel()
		nc.Close()
		if lpErr != nil {
			unreachableNodes++
			report.addAnomaly("A8", nid, "",
				fmt.Sprintf("node-agent ListInstalledPackages failed at %s: %v — installed state not observed", endpoint, lpErr),
				"check node-agent health; re-run after agent recovers")
			continue
		}

		for _, pkg := range pkgResp.GetPackages() {
			name := pkg.GetName()
			if name == "" {
				continue
			}
			total++

			if svcNodes[name] == nil {
				svcNodes[name] = make(map[string]bool)
			}

			status := pkg.GetStatus()
			meta := pkg.GetMetadata()
			version := pkg.GetVersion()
			buildID := pkg.GetBuildId()
			buildNumber := pkg.GetBuildNumber()

			// A1: Stale failed record from unreachable repository.
			if status == "failed" && meta != nil {
				errMsg := meta["error"]
				if strings.Contains(errMsg, "repository unreachable") ||
					strings.Contains(errMsg, "no local package found") {
					staleCount++
					report.addAnomaly("A1", nid, name,
						fmt.Sprintf("stale failed record (version=%s build=%d, error=%q)",
							version, buildNumber, errMsg),
						"delete stale record; convergence loop will re-apply correctly")
					continue
				}
			}

			// A3: Missing build_id on installed record. Metadata-only
			// packages (no desired-state) had no repository artifact at
			// install time — skip the anomaly for those.
			if status == "installed" {
				if buildID != "" {
					withBuildID++
					svcNodes[name][nid] = true
				} else if report.DesiredServices != nil && !report.DesiredServices[name] {
					withBuildID++ // metadata-only: count as non-anomalous
				} else {
					svcNodes[name][nid] = false
					report.addAnomaly("A3", nid, name,
						fmt.Sprintf("installed-state missing buildId (version=%s build=%d)",
							version, buildNumber),
						"re-apply service with build_id to gain exact identity")
				}
			}
		}
	}

	// A7: Inconsistent node coverage.
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

	// Compute canonical percentage. Exclude stale records from the
	// denominator, AND exclude unreachable-node observations: we have
	// no installed-state truth for those nodes and must not let their
	// absence inflate or deflate the canonical percentage. If every
	// node was unreachable, leave CanonicalPercent at zero so the
	// caller sees the degraded signal.
	totalRecords := total - staleCount
	if totalRecords > 0 {
		report.CanonicalPercent = float64(withBuildID) / float64(totalRecords) * 100
	}

	fmt.Printf("  %d installed-state records, %d with buildId, %d stale/failed, %d unreachable nodes\n",
		total, withBuildID, staleCount, unreachableNodes)

	return nil
}

// ── A2 repair: desired-state build_id ─────────────────────────────────────

// repairDesiredStateBuildID re-upserts each desired-state record missing
// build_id through the controller API. The controller resolves build_id
// from the repository manifest and writes it to etcd.
// repairDesiredStateBuildID collects entries whose build_id is empty
// and re-upserts them so the controller's resolver allocates a fresh
// build_id from the repository. The read side now uses the typed
// GetDesiredState RPC instead of scanning
// /globular/resources/ServiceDesiredVersion/ etcd directly — anchored
// by invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
// The write side already used UpsertDesiredService, so this function
// is now end-to-end typed (no etcd primitive).
func repairDesiredStateBuildID(ctx context.Context, report *canonReport) (repaired, failed int) {
	// Collect A2 anomalies.
	type a2Entry struct {
		service     string
		version     string
		buildNumber int64
	}
	var entries []a2Entry

	cc, err := controllerClient()
	if err != nil {
		fmt.Printf("  A2: cannot connect to controller: %v\n", err)
		return 0, 0
	}
	defer cc.Close()
	ctrlClient := cluster_controllerpb.NewClusterControllerServiceClient(cc)

	getCtx, getCancel := context.WithTimeout(ctx, 10*time.Second)
	resp, err := ctrlClient.GetDesiredState(getCtx, &emptypb.Empty{})
	getCancel()
	if err != nil {
		fmt.Printf("  A2: GetDesiredState failed: %v\n", err)
		return 0, 0
	}

	for _, svc := range resp.GetServices() {
		if svc.GetBuildId() == "" {
			entries = append(entries, a2Entry{
				service:     svc.GetServiceId(),
				version:     svc.GetVersion(),
				buildNumber: svc.GetBuildNumber(),
			})
		}
	}

	if len(entries) == 0 {
		fmt.Println("  A2: no records to repair")
		return 0, 0
	}

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
			_ = audittrail.WriteDesiredWriteRecord(ctx, audittrail.DesiredWriteRecord{
				Service:   e.service,
				Actor:     "canonicalize-tool",
				Source:    "state.canonicalize.fix-safe-A2",
				Action:    "backfill_build_id",
				Reason:    "repair desired-state missing build_id",
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			})
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

// repairDesiredWriteProvenanceLegacy emits provenance markers for desired-state
// services whose records predate desired-write provenance emission.
//
// The desired-state side now consumes GetDesiredState (anchored by
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage).
// The provenance audit read against /globular/audit/desired_writes/
// is a separate ownership domain (audittrail) and is left as a tracked
// follow-up.
func repairDesiredWriteProvenanceLegacy(ctx context.Context) (backfilled, failed int) {
	cc, err := controllerClient()
	if err != nil {
		fmt.Printf("  provenance backfill skipped: cannot dial controller: %v\n", err)
		return 0, 1
	}
	defer cc.Close()
	ctrlClient := cluster_controllerpb.NewClusterControllerServiceClient(cc)

	getCtx, getCancel := context.WithTimeout(ctx, 10*time.Second)
	dResp, err := ctrlClient.GetDesiredState(getCtx, &emptypb.Empty{})
	getCancel()
	if err != nil {
		fmt.Printf("  provenance backfill skipped: GetDesiredState failed: %v\n", err)
		return 0, 1
	}
	if len(dResp.GetServices()) == 0 {
		return 0, 0
	}

	// Read existing provenance markers (audittrail-owned prefix —
	// tracked follow-up to migrate to a typed audittrail RPC).
	cli, err := config.GetEtcdClient()
	if err != nil {
		fmt.Printf("  provenance backfill warning: audittrail etcd read unavailable: %v\n", err)
	}
	covered := map[string]bool{}
	if cli != nil {
		pResp, perr := cli.Get(ctx, "/globular/audit/desired_writes/", clientv3.WithPrefix())
		if perr != nil {
			fmt.Printf("  provenance backfill warning: cannot read existing provenance: %v\n", perr)
		} else {
			for _, kv := range pResp.Kvs {
				var rec struct {
					Service string `json:"service"`
				}
				if err := json.Unmarshal(kv.Value, &rec); err != nil {
					continue
				}
				svc := strings.TrimSpace(rec.Service)
				if svc != "" {
					covered[svc] = true
				}
			}
		}
	}

	for _, svc := range dResp.GetServices() {
		name := strings.TrimSpace(svc.GetServiceId())
		if name == "" || covered[name] {
			continue
		}
		if err := audittrail.WriteDesiredWriteRecord(ctx, audittrail.DesiredWriteRecord{
			Service:   name,
			Actor:     "canonicalize-tool",
			Source:    "state.canonicalize.provenance-backfill",
			Action:    "legacy_backfill_marker",
			Reason:    "desired-state record predates provenance emission",
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		}); err != nil {
			failed++
			continue
		}
		covered[name] = true
		backfilled++
	}
	return backfilled, failed
}

// ── A3 repair: installed-state build_id ───────────────────────────────────

// repairInstalledStateBuildID applies the correct build_id to installed-state
// records on a specific node by calling ApplyPackageRelease with force=true.
//
// Desired-state side reads GetDesiredState (anchored by
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage).
// The controller's listAllDesiredServices already merges SDV +
// InfrastructureRelease into a flat list; the prior per-prefix etcd
// scans collapsed into one typed call. Installed-state read against
// /globular/nodes/{node}/packages/ is a separate ratchet — it needs
// node_agent.ListInstalledPackages.
func repairInstalledStateBuildID(ctx context.Context, nodeID string) (repaired, failed int) {
	// Local mirror of the desired-state tuple the rest of this
	// function uses. Identical shape to the prior etcd-parsed type so
	// downstream code is unchanged.
	type dSpec struct {
		ServiceName string
		Version     string
		BuildNumber int64
		BuildID     string
	}

	cc, err := controllerClient()
	if err != nil {
		fmt.Printf("  controller unavailable: %v\n", err)
		return 0, 0
	}
	defer cc.Close()
	ctrlClient := cluster_controllerpb.NewClusterControllerServiceClient(cc)

	getCtx, getCancel := context.WithTimeout(ctx, 10*time.Second)
	dsResp, err := ctrlClient.GetDesiredState(getCtx, &emptypb.Empty{})
	getCancel()
	if err != nil {
		fmt.Printf("  GetDesiredState failed: %v\n", err)
		return 0, 0
	}

	desired := make(map[string]*dSpec, len(dsResp.GetServices()))
	for _, svc := range dsResp.GetServices() {
		name := svc.GetServiceId()
		if name == "" {
			continue
		}
		desired[name] = &dSpec{
			ServiceName: name,
			Version:     svc.GetVersion(),
			BuildNumber: svc.GetBuildNumber(),
			BuildID:     svc.GetBuildId(),
		}
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		fmt.Printf("  etcd unavailable for installed-state read: %v\n", err)
		return 0, 0
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

	// Resolve node-agent endpoint. Both modes write owner-owned installed state
	// THROUGH the node-agent (the owner of /globular/nodes), never raw etcd (RT-2):
	// --metadata-only via SetInstalledPackage (stamp build_id without reinstalling),
	// the normal path via ApplyPackageRelease. Requires the node-agent reachable.
	var agentClient node_agentpb.NodeAgentServiceClient
	{
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
		agentClient = node_agentpb.NewNodeAgentServiceClient(conn)
	}

	// Metadata-only repair: write buildId directly to the existing etcd record
	// without reinstalling. Used for COMMAND/INFRASTRUCTURE packages that are
	// already installed but can't be re-applied through the standard path.
	metadataUpdate := func(svc string) bool {
		d := desired[svc]
		// Stamp build_id on every kind-prefixed installed record missing it, through
		// the node-agent's typed RPC (the owner of /globular/nodes) instead of a raw
		// etcd write — see setInstalledBuildID.
		updated := false
		for _, inst := range installed {
			if inst.Name != svc || inst.BuildID != "" || inst.Status != "installed" {
				continue
			}
			if err := setInstalledBuildID(ctx, agentClient, nodeID, inst.Kind, svc, d.BuildID); err != nil {
				fmt.Printf("    FAIL  %-25s  %s: %v\n", svc, inst.Kind, err)
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
		// ExpectedSha256 intentionally omitted: this is an operator
		// metadata-repair escape hatch (Force=true). The node-agent will
		// write installed_unverified — the honest signal that identity is
		// unproven. The next controller reconcile pass DOES resolve manifest
		// and re-dispatches with ExpectedSha256, transitioning the state to
		// verified installed. Synthesising a hash here is forbidden — see
		// invariant controller.apply_package_release_requires_manifest_checksum
		// and the allow-list entry in dispatch_expected_sha256_test.go.
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

// cleanupGhostNodes removes installed-state records for nodes that
// are no longer cluster members. v1.2.190 migrates this off the
// prior raw-etcd path:
//   - active-node listing → cluster_controller.ListNodes (already
//     typed)
//   - ghost-node enumeration → same prefix-scan via etcd as before,
//     but the SCAN is read-only and short-lived; routing it through
//     a typed RPC would require a new server-side enumerator. The
//     destructive WRITE is now the typed
//     cluster_controller.CleanupGhostNodePackages call (server-side
//     guard refuses to clean an active node).
//
// Anchored by:
//
//	invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//	invariant:destructive_actions.require_explicit_guard
//	forbidden_fix:cross_layer_write_by_non_owner
func cleanupGhostNodes(ctx context.Context, report *canonReport) int {
	cli, err := config.GetEtcdClient()
	if err != nil {
		fmt.Printf("  etcd unavailable: %v\n", err)
		return 0
	}

	cc, err := controllerClient()
	if err != nil {
		fmt.Printf("  controller unavailable: %v — skipping ghost cleanup\n", err)
		return 0
	}
	defer cc.Close()
	ctrlClient := cluster_controllerpb.NewClusterControllerServiceClient(cc)

	// Get active node IDs from the controller.
	activeNodes := make(map[string]bool)
	{
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

	// Enumerate node-ids that have package records via a keys-only
	// scan. This is a discovery step — the destructive delete is
	// dispatched through the controller's typed RPC below.
	scanCtx, scanCancel := context.WithTimeout(ctx, 10*time.Second)
	resp, err := cli.Get(scanCtx, "/globular/nodes/",
		clientv3.WithPrefix(), clientv3.WithKeysOnly(), clientv3.WithLimit(5000))
	scanCancel()
	if err != nil {
		return 0
	}
	ghosts := make(map[string]bool)
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.Contains(key, "/packages/") {
			continue
		}
		parts := strings.Split(key, "/")
		if len(parts) < 4 {
			continue
		}
		nodeID := parts[3]
		if activeNodes[nodeID] {
			continue
		}
		ghosts[nodeID] = true
	}

	cleaned := 0
	for nodeID := range ghosts {
		callCtx, callCancel := context.WithTimeout(ctx, 30*time.Second)
		cleanupResp, cErr := ctrlClient.CleanupGhostNodePackages(callCtx, &cluster_controllerpb.CleanupGhostNodePackagesRequest{
			NodeId: nodeID,
		})
		callCancel()
		if cErr != nil {
			fmt.Printf("    FAIL  CleanupGhostNodePackages(%s): %v\n", short8(nodeID), cErr)
			continue
		}
		if cleanupResp.GetRefusedActiveNode() {
			fmt.Printf("    SKIP  %s (controller refused — node is active)\n", short8(nodeID))
			continue
		}
		n := int(cleanupResp.GetDeleted())
		fmt.Printf("    DEL   %d records (ghost node %s)\n", n, short8(nodeID))
		writeAuditRecord(ctx, auditRecord{
			Action:      "cleanup-ghost",
			Service:     fmt.Sprintf("/globular/nodes/%s/packages/", nodeID),
			Node:        nodeID,
			BeforeState: "installed",
			AfterState:  "deleted",
			Detail:      fmt.Sprintf("deleted %d records via typed RPC", n),
		})
		cleaned += n
	}
	return cleaned
}

// short8 returns the first 8 chars of s for log lines.
func short8(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// resolveAgentEndpoint finds the node-agent gRPC endpoint for a
// node ID via the cluster_controller's typed ListNodes RPC.
//
// History: previously read /globular/nodes/{nodeID}/status directly
// from etcd. That prefix is owned by node_agent and the controller's
// node registry, so a CLI consumer reading raw etcd violated
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
// setInstalledBuildID stamps build_id onto an existing installed-package record
// through the node-agent — the registered owner of /globular/nodes — instead of a
// raw etcd write (RT-2). SetInstalledPackage has replace semantics
// (installed_state.WriteInstalledPackage writes the whole record), so we GET the
// full current package first, set only build_id, and SET it back, preserving every
// other field. The node-agent's typed path passes the owner-write governance the
// CLI could not.
func setInstalledBuildID(ctx context.Context, agent node_agentpb.NodeAgentServiceClient, nodeID, kind, name, buildID string) error {
	getCtx, getCancel := context.WithTimeout(ctx, 5*time.Second)
	getResp, err := agent.GetInstalledPackage(getCtx, &node_agentpb.GetInstalledPackageRequest{
		NodeId: nodeID, Kind: kind, Name: name,
	})
	getCancel()
	if err != nil {
		return fmt.Errorf("get installed %s/%s: %w", kind, name, err)
	}
	pkg := getResp.GetPackage()
	if pkg == nil {
		return fmt.Errorf("installed package %s/%s not found", kind, name)
	}
	pkg.BuildId = buildID
	setCtx, setCancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = agent.SetInstalledPackage(setCtx, &node_agentpb.SetInstalledPackageRequest{Package: pkg})
	setCancel()
	if err != nil {
		return fmt.Errorf("set installed %s/%s: %w", kind, name, err)
	}
	return nil
}

// NodeRecord.AgentEndpoint carries the same value the prior code
// extracted from the JSON status blob.
//
// The cli *clientv3.Client argument is preserved for caller-site
// stability but is no longer consulted.
func resolveAgentEndpoint(ctx context.Context, _ *clientv3.Client, nodeID string) string {
	cc, err := controllerClient()
	if err != nil {
		return ""
	}
	defer cc.Close()
	ctrl := cluster_controllerpb.NewClusterControllerServiceClient(cc)
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := ctrl.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil || resp == nil {
		return ""
	}
	for _, n := range resp.GetNodes() {
		if n.GetNodeId() == nodeID {
			return strings.TrimSpace(n.GetAgentEndpoint())
		}
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
