package main

// objectstore_disk_cmds.go — MinIO disk admission and topology planning CLI.
//
//   globular objectstore disk scan             — show candidates reported by all nodes
//   globular objectstore disk list [--node N]  — list candidates for one/all nodes
//   globular objectstore disk approve          — admit a disk path for MinIO
//   globular objectstore disk reject           — reject (un-admit) a disk path
//   globular objectstore topology plan         — compute a topology proposal
//   globular objectstore topology apply        — apply a proposal (triggers workflow)

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/config"
)

// ── top-level disk subcommand ─────────────────────────────────────────────────

var objectstoreDiskCmd = &cobra.Command{
	Use:   "disk",
	Short: "MinIO disk admission (scan, approve, reject)",
}

// ── disk scan ─────────────────────────────────────────────────────────────────

var (
	diskScanJSON    bool
	diskScanNodeID  string
)

var objectstoreDiskScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Show disk candidates reported by node agents (read-only)",
	Long: `Display all disk candidates that node agents have discovered and reported
to etcd. Node agents report facts — they never select or format disks.

This command reads /globular/nodes/{id}/storage/candidates/* and prints
eligibility status, size, and MinIO-readiness for each disk.

Use 'globular objectstore disk approve' to admit a disk for MinIO use.`,
	RunE: runObjectstoreDiskScan,
}

func init() {
	objectstoreDiskScanCmd.Flags().BoolVar(&diskScanJSON, "json", false, "Output as JSON")
	objectstoreDiskScanCmd.Flags().StringVar(&diskScanNodeID, "node", "", "Filter to a specific node ID or IP")
	objectstoreDiskCmd.AddCommand(objectstoreDiskScanCmd)
}

func runObjectstoreDiskScan(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	allCandidates, err := config.LoadAllDiskCandidates(ctx)
	if err != nil {
		return fmt.Errorf("load candidates: %w", err)
	}

	// Filter by node if requested.
	if diskScanNodeID != "" {
		filtered := make(map[string][]*config.DiskCandidate)
		for nodeID, cands := range allCandidates {
			if strings.Contains(nodeID, diskScanNodeID) {
				filtered[nodeID] = cands
			}
		}
		allCandidates = filtered
	}

	if len(allCandidates) == 0 {
		fmt.Println("No disk candidates found. Node agents may not have reported yet.")
		fmt.Println("Check that node-agent is running (globular-node-agent.service) and wait ~5 minutes.")
		return nil
	}

	if diskScanJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(allCandidates)
	}

	// Load admitted disks to annotate.
	admitted, _ := config.LoadAdmittedDisks(ctx)
	admittedSet := make(map[string]bool) // "nodeID:path" → admitted
	for _, ad := range admitted {
		admittedSet[ad.NodeID+":"+ad.Path] = true
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nDisk Candidates")
	fmt.Fprintln(w, strings.Repeat("─", 80))
	fmt.Fprintln(w, "NODE\tDEVICE\tMOUNT\tFS\tSIZE\tAVAIL\tELIGIBLE\tADMITTED\tNOTES")

	// Print nodes in sorted order.
	nodeIDs := sortedKeys(allCandidates)
	for _, nodeID := range nodeIDs {
		cands := allCandidates[nodeID]
		sort.Slice(cands, func(i, j int) bool { return cands[i].MountPath < cands[j].MountPath })
		for _, dc := range cands {
			eligible := "no"
			if dc.Eligible {
				eligible = "yes"
			}
			admitted := " "
			if admittedSet[dc.NodeID+":"+dc.MountPath] {
				admitted = "✓"
			}
			notes := ""
			if dc.HasMinioSys {
				notes += "[.minio.sys] "
			}
			if dc.HasExistingData {
				notes += "[existing-data] "
			}
			if dc.IsRoot {
				notes += "[root] "
			}
			nodeLabel := nodeID
			if len(nodeLabel) > 12 {
				nodeLabel = nodeLabel[:12]
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				nodeLabel,
				basename(dc.Device),
				dc.MountPath,
				dc.FSType,
				humanBytes(uint64(dc.SizeBytes)),
				humanBytes(uint64(dc.AvailableBytes)),
				eligible,
				admitted,
				strings.TrimSpace(notes),
			)
		}
	}
	w.Flush()
	fmt.Println()
	fmt.Println("To admit a disk: globular objectstore disk approve --node <node-id> --path <mount-path>")
	return nil
}

// ── disk approve ──────────────────────────────────────────────────────────────

var (
	approveNodeID      string
	approvePath        string
	approveDrives      int
	approveForceRoot   bool
	approveForceData   bool
	approveNodeIP      string
)

var objectstoreDiskApproveCmd = &cobra.Command{
	Use:   "approve",
	Short: "Admit a disk path for MinIO use (operator decision)",
	Long: `Approve a mount path on a node as the MinIO data directory.
This records the operator's explicit decision in etcd.

Approval does NOT start MinIO or modify any files on disk. It only
records the intent. Run 'globular objectstore topology plan' to
compute a topology proposal, then 'globular objectstore topology apply'
to apply it.

Flags:
  --node     Node ID or hostname of the node
  --path     Mount path to approve (e.g. /mnt/data, /mnt/40F43F08F43EFFA8)
  --node-ip  Node's routable IP (required for topology planning)
  --drives   Number of drives on this node (default 1)
  --force-root          Allow the root filesystem (/)
  --force-existing-data Allow paths that contain existing non-MinIO data`,
	RunE: runObjectstoreDiskApprove,
}

func init() {
	objectstoreDiskApproveCmd.Flags().StringVar(&approveNodeID, "node", "", "Node ID (required)")
	objectstoreDiskApproveCmd.Flags().StringVar(&approvePath, "path", "", "Mount path to approve (required)")
	objectstoreDiskApproveCmd.Flags().StringVar(&approveNodeIP, "node-ip", "", "Node routable IP (required)")
	objectstoreDiskApproveCmd.Flags().IntVar(&approveDrives, "drives", 1, "Drives per node")
	objectstoreDiskApproveCmd.Flags().BoolVar(&approveForceRoot, "force-root", false, "Allow root filesystem path")
	objectstoreDiskApproveCmd.Flags().BoolVar(&approveForceData, "force-existing-data", false, "Allow path with existing non-MinIO data")
	objectstoreDiskApproveCmd.MarkFlagRequired("node")    //nolint:errcheck
	objectstoreDiskApproveCmd.MarkFlagRequired("path")    //nolint:errcheck
	objectstoreDiskApproveCmd.MarkFlagRequired("node-ip") //nolint:errcheck
	objectstoreDiskCmd.AddCommand(objectstoreDiskApproveCmd)
}

func runObjectstoreDiskApprove(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Guard: refuse root path unless --force-root.
	if approvePath == "/" && !approveForceRoot {
		return fmt.Errorf("path %q is the root filesystem — use --force-root to override (data loss risk)", approvePath)
	}

	// Look up the disk candidate to check eligibility and get facts.
	candidates, err := config.LoadDiskCandidates(ctx, approveNodeID)
	if err == nil {
		for _, dc := range candidates {
			if dc.MountPath == approvePath {
				if !dc.Eligible {
					fmt.Printf("WARNING: disk %s on node %s is not automatically eligible:\n", approvePath, approveNodeID)
					for _, r := range dc.Reasons {
						fmt.Printf("  - %s\n", r)
					}
					if !approveForceRoot && dc.IsRoot {
						return fmt.Errorf("root filesystem — add --force-root to proceed")
					}
					if !approveForceData && dc.HasExistingData && !dc.HasMinioSys {
						return fmt.Errorf("path contains existing non-MinIO data — add --force-existing-data to proceed")
					}
					fmt.Println("Proceeding with admission despite eligibility warnings.")
				}
				break
			}
		}
	}

	ph := config.PathHash(approvePath)
	ad := &config.AdmittedDisk{
		NodeID:            approveNodeID,
		NodeIP:            approveNodeIP,
		Path:              approvePath,
		PathHash:          ph,
		DrivesPerNode:     approveDrives,
		ForceRoot:         approveForceRoot,
		ForceExistingData: approveForceData,
		ApprovedAt:        time.Now().UTC(),
	}

	if err := config.SaveAdmittedDisk(ctx, ad); err != nil {
		return fmt.Errorf("save admitted disk: %w", err)
	}

	fmt.Printf("✓ Admitted disk path %q on node %s (ip=%s drives=%d)\n",
		approvePath, approveNodeID, approveNodeIP, approveDrives)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Admit paths on all pool nodes")
	fmt.Println("  2. globular objectstore topology plan")
	fmt.Println("  3. globular objectstore topology apply --proposal <id>")
	return nil
}

// ── disk reject ───────────────────────────────────────────────────────────────

var (
	rejectNodeID string
	rejectPath   string
)

var objectstoreDiskRejectCmd = &cobra.Command{
	Use:   "reject",
	Short: "Remove a previously admitted disk path",
	RunE:  runObjectstoreDiskReject,
}

func init() {
	objectstoreDiskRejectCmd.Flags().StringVar(&rejectNodeID, "node", "", "Node ID (required)")
	objectstoreDiskRejectCmd.Flags().StringVar(&rejectPath, "path", "", "Mount path to reject (required)")
	objectstoreDiskRejectCmd.MarkFlagRequired("node") //nolint:errcheck
	objectstoreDiskRejectCmd.MarkFlagRequired("path") //nolint:errcheck
	objectstoreDiskCmd.AddCommand(objectstoreDiskRejectCmd)
}

func runObjectstoreDiskReject(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ph := config.PathHash(rejectPath)
	if err := config.DeleteAdmittedDisk(ctx, rejectNodeID, ph); err != nil {
		return fmt.Errorf("delete admission: %w", err)
	}

	fmt.Printf("✓ Removed admission for %q on node %s\n", rejectPath, rejectNodeID)
	fmt.Println("Run 'globular objectstore topology plan' to recompute the proposal.")
	return nil
}

// ── disk list ─────────────────────────────────────────────────────────────────

var (
	diskListNodeID string
	diskListJSON   bool
)

var objectstoreDiskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List admitted disk paths",
	RunE:  runObjectstoreDiskList,
}

func init() {
	objectstoreDiskListCmd.Flags().StringVar(&diskListNodeID, "node", "", "Filter to a specific node ID")
	objectstoreDiskListCmd.Flags().BoolVar(&diskListJSON, "json", false, "Output as JSON")
	objectstoreDiskCmd.AddCommand(objectstoreDiskListCmd)
}

func runObjectstoreDiskList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	admitted, err := config.LoadAdmittedDisks(ctx)
	if err != nil {
		return fmt.Errorf("load admitted disks: %w", err)
	}

	if diskListNodeID != "" {
		var filtered []*config.AdmittedDisk
		for _, ad := range admitted {
			if strings.Contains(ad.NodeID, diskListNodeID) {
				filtered = append(filtered, ad)
			}
		}
		admitted = filtered
	}

	if diskListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(admitted)
	}

	if len(admitted) == 0 {
		fmt.Println("No admitted disks. Run 'globular objectstore disk scan' then 'disk approve'.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tIP\tPATH\tDRIVES\tAPPROVED_AT")
	for _, ad := range admitted {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			ad.NodeID, ad.NodeIP, ad.Path, ad.DrivesPerNode, ad.ApprovedAt.Format(time.RFC3339))
	}
	w.Flush()
	return nil
}

// ── topology plan ─────────────────────────────────────────────────────────────

var (
	planJSON bool
)

var objectstoreTopologyPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Compute a MinIO topology proposal from admitted disks",
	Long: `Reads admitted disk paths from etcd and computes a MinIO topology proposal.

The proposal records:
  - node_paths: per-node MinIO data paths
  - drives_per_node: how many drives each node contributes
  - nodes: ordered pool IP list
  - is_destructive: whether applying would wipe .minio.sys
  - validation_errors: any fatal problems

The proposal is stored in etcd and its ID is printed for use with
'globular objectstore topology apply --proposal <id>'.`,
	RunE: runObjectstoreTopologyPlan,
}

func init() {
	objectstoreTopologyPlanCmd.Flags().BoolVar(&planJSON, "json", false, "Output proposal as JSON")
	objectstoreTopologyCmd.AddCommand(objectstoreTopologyPlanCmd)
}

func runObjectstoreTopologyPlan(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Load admitted disks.
	admitted, err := config.LoadAdmittedDisks(ctx)
	if err != nil {
		return fmt.Errorf("load admitted disks: %w", err)
	}
	if len(admitted) == 0 {
		return fmt.Errorf("no admitted disks — run 'globular objectstore disk approve' first")
	}

	// Load current desired state (for destructiveness analysis).
	current, _ := config.LoadObjectStoreDesiredState(ctx)

	// Build proposal from admitted disks.
	proposal := buildTopologyProposal(admitted, current)

	// Save proposal to etcd.
	if err := config.SaveTopologyProposal(ctx, proposal); err != nil {
		return fmt.Errorf("save proposal: %w", err)
	}

	if planJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(proposal)
	}

	printTopologyProposal(proposal)
	return nil
}

// buildTopologyProposal computes a TopologyProposal from admitted disks.
func buildTopologyProposal(admitted []*config.AdmittedDisk, current *config.ObjectStoreDesiredState) *config.TopologyProposal {
	// Build IP-indexed admission map; last admission per IP wins.
	admittedByIP := make(map[string]*config.AdmittedDisk, len(admitted))
	for _, ad := range admitted {
		if ad.NodeIP != "" {
			admittedByIP[ad.NodeIP] = ad
		}
	}

	// Build ordered node list (sort IPs for determinism).
	nodes := make([]string, 0, len(admittedByIP))
	for ip := range admittedByIP {
		nodes = append(nodes, ip)
	}
	sort.Strings(nodes)

	// Preserve existing pool order for nodes already in the pool.
	if current != nil && len(current.Nodes) > 0 {
		nodes = mergePreservingOrder(current.Nodes, nodes)
	}

	// Build node_paths map.
	nodePaths := make(map[string]string, len(nodes))
	drivesPerNode := 1
	for ip, ad := range admittedByIP {
		nodePaths[ip] = ad.Path
		if ad.DrivesPerNode > drivesPerNode {
			drivesPerNode = ad.DrivesPerNode
		}
	}

	proposal := &config.TopologyProposal{
		ProposalID:    generateProposalID(nodes, nodePaths, drivesPerNode),
		GeneratedAt:   time.Now().UTC(),
		NodePaths:     nodePaths,
		DrivesPerNode: drivesPerNode,
		Nodes:         nodes,
		Status:        "proposed",
	}

	// Validate the proposal.
	proposal.ValidationErrors = validateProposalLocally(proposal, admittedByIP)

	// Compute destructiveness.
	if proposal.Valid() {
		isDestructive, reasons := computeTopologyDestructiveness(proposal, current)
		proposal.IsDestructive = isDestructive
		proposal.DestructiveReasons = reasons
	}

	// Warnings.
	if len(nodes) < 2 {
		proposal.Warnings = append(proposal.Warnings,
			"single-node topology: MinIO will run standalone (no erasure coding)")
	}
	if len(nodes) == 2 {
		proposal.Warnings = append(proposal.Warnings,
			"2-node topology: MinIO requires ≥4 drives for full erasure coding; consider adding a 3rd node")
	}

	return proposal
}

// validateProposalLocally validates a proposal without hitting the controller.
func validateProposalLocally(p *config.TopologyProposal, admittedByIP map[string]*config.AdmittedDisk) []string {
	var errs []string

	if len(p.Nodes) == 0 {
		return []string{"no pool nodes"}
	}

	seenPaths := make(map[string]string)
	for _, ip := range p.Nodes {
		path, ok := p.NodePaths[ip]
		if !ok || path == "" {
			errs = append(errs, fmt.Sprintf("node %s has no admitted path", ip))
			continue
		}

		// Root filesystem guard.
		ad := admittedByIP[ip]
		if path == "/" && (ad == nil || !ad.ForceRoot) {
			errs = append(errs, fmt.Sprintf("node %s: path / is root filesystem (admit with --force-root)", ip))
		}

		// Existing data guard.
		if ad != nil && ad.ForceExistingData == false {
			// We can't check HasExistingData without reading the candidate; skip here.
		}

		// Duplicate path.
		if prev, dup := seenPaths[path]; dup {
			errs = append(errs, fmt.Sprintf("duplicate path %q on nodes %s and %s", path, prev, ip))
		}
		seenPaths[path] = ip
	}

	return errs
}

// computeTopologyDestructiveness mirrors the server-side function for CLI use.
func computeTopologyDestructiveness(proposal *config.TopologyProposal, current *config.ObjectStoreDesiredState) (bool, []string) {
	if current == nil {
		if len(proposal.Nodes) >= 2 {
			return true, []string{"first distributed topology: .minio.sys will be wiped on all pool nodes"}
		}
		return false, nil
	}

	var reasons []string

	if current.Mode == config.ObjectStoreModeStandalone && len(proposal.Nodes) >= 2 {
		reasons = append(reasons, fmt.Sprintf(
			"standalone → distributed: .minio.sys will be wiped on %d nodes", len(proposal.Nodes)))
	}

	for ip, newPath := range proposal.NodePaths {
		if oldPath, ok := current.NodePaths[ip]; ok && oldPath != newPath {
			reasons = append(reasons, fmt.Sprintf("node %s path change %q → %q", ip, oldPath, newPath))
		}
	}

	return len(reasons) > 0, reasons
}

// generateProposalID produces a stable 12-hex-char ID from proposal inputs.
func generateProposalID(nodes []string, paths map[string]string, drives int) string {
	// Include a random component so two plans at the same time don't collide.
	var rnd [4]byte
	rand.Read(rnd[:]) //nolint:errcheck
	key := fmt.Sprintf("%v|%v|%d|%x", nodes, paths, drives, rnd)
	h := config.PathHash(key) // reuse sha256 helper
	return h
}

// mergePreservingOrder merges new IPs into an existing ordered list, appending
// new IPs that don't yet appear.
func mergePreservingOrder(existing, incoming []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, ip := range existing {
		seen[ip] = true
	}
	out := make([]string, len(existing))
	copy(out, existing)
	for _, ip := range incoming {
		if !seen[ip] {
			out = append(out, ip)
		}
	}
	return out
}

func printTopologyProposal(p *config.TopologyProposal) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nTopology Proposal")
	fmt.Fprintln(w, strings.Repeat("─", 60))
	fmt.Fprintf(w, "Proposal ID:\t%s\n", p.ProposalID)
	fmt.Fprintf(w, "Generated at:\t%s\n", p.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Pool nodes:\t%s\n", strings.Join(p.Nodes, ", "))
	fmt.Fprintf(w, "Drives/node:\t%d\n", p.DrivesPerNode)
	fmt.Fprintln(w, strings.Repeat("─", 60))
	fmt.Fprintln(w, "NODE IP\tDATA PATH")
	for ip, path := range p.NodePaths {
		fmt.Fprintf(w, "%s\t%s\n", ip, path)
	}
	if len(p.ValidationErrors) > 0 {
		fmt.Fprintln(w, strings.Repeat("─", 60))
		fmt.Fprintln(w, "VALIDATION ERRORS (cannot apply):")
		for _, e := range p.ValidationErrors {
			fmt.Fprintf(w, "  ✗ %s\n", e)
		}
	}
	if p.IsDestructive {
		fmt.Fprintln(w, strings.Repeat("─", 60))
		fmt.Fprintln(w, "DESTRUCTIVE OPERATION:")
		for _, r := range p.DestructiveReasons {
			fmt.Fprintf(w, "  ⚠ %s\n", r)
		}
	}
	if len(p.Warnings) > 0 {
		fmt.Fprintln(w, strings.Repeat("─", 60))
		fmt.Fprintln(w, "Warnings:")
		for _, warn := range p.Warnings {
			fmt.Fprintf(w, "  ! %s\n", warn)
		}
	}
	w.Flush()
	fmt.Println()

	if !p.Valid() {
		fmt.Println("✗ Proposal has validation errors and cannot be applied.")
		return
	}

	applyCmd := fmt.Sprintf("globular objectstore topology apply --proposal %s", p.ProposalID)
	if p.IsDestructive {
		applyCmd += " --i-understand-data-reset"
		fmt.Printf("⚠ DESTRUCTIVE: to apply:\n  %s\n", applyCmd)
	} else {
		fmt.Printf("✓ Valid. To apply:\n  %s\n", applyCmd)
	}
}

// ── topology apply ────────────────────────────────────────────────────────────

var (
	applyProposalID         string
	applyForceDestructive   bool
	applyWaitTimeout        time.Duration
)

var objectstoreTopologyApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a topology proposal (triggers MinIO restart workflow)",
	Long: `Apply a previously computed topology proposal to the cluster.

The controller validates the proposal, updates ObjectStoreDesiredState,
and triggers the objectstore.minio.apply_topology_generation workflow to
coordinate a rolling MinIO restart across all pool nodes.

Destructive proposals (standalone→distributed transitions, path changes)
require --i-understand-data-reset. Without this flag, the apply is rejected.

CAUTION: Destructive applies wipe .minio.sys on affected nodes. All objects
stored in standalone mode are LOST. Re-publish packages with 'globular pkg publish'.`,
	RunE: runObjectstoreTopologyApply,
}

func init() {
	objectstoreTopologyApplyCmd.Flags().StringVar(&applyProposalID, "proposal", "", "Proposal ID from 'topology plan' (required)")
	objectstoreTopologyApplyCmd.Flags().BoolVar(&applyForceDestructive, "i-understand-data-reset", false,
		"Confirm destructive apply: wipes .minio.sys where topology changes")
	objectstoreTopologyApplyCmd.Flags().DurationVar(&applyWaitTimeout, "wait", 5*time.Minute,
		"Maximum time to wait for the controller to accept the request")
	objectstoreTopologyApplyCmd.MarkFlagRequired("proposal") //nolint:errcheck
	objectstoreTopologyCmd.AddCommand(objectstoreTopologyApplyCmd)
}

func runObjectstoreTopologyApply(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), applyWaitTimeout+15*time.Second)
	defer cancel()

	// Load the proposal from etcd.
	proposal, err := config.LoadTopologyProposal(ctx, applyProposalID)
	if err != nil {
		return fmt.Errorf("load proposal: %w", err)
	}

	// Pre-flight: reject invalid proposals locally.
	if !proposal.Valid() {
		fmt.Fprintln(os.Stderr, "Proposal has validation errors:")
		for _, e := range proposal.ValidationErrors {
			fmt.Fprintf(os.Stderr, "  ✗ %s\n", e)
		}
		return fmt.Errorf("proposal %s is not valid", applyProposalID)
	}

	// Pre-flight: require --i-understand-data-reset for destructive proposals.
	if proposal.IsDestructive && !applyForceDestructive {
		fmt.Fprintln(os.Stderr, "This proposal is destructive:")
		for _, r := range proposal.DestructiveReasons {
			fmt.Fprintf(os.Stderr, "  ⚠ %s\n", r)
		}
		return fmt.Errorf("destructive apply requires --i-understand-data-reset")
	}

	// Generate a nonce to match request → result.
	var rndBytes [8]byte
	rand.Read(rndBytes[:]) //nolint:errcheck
	requestID := hex.EncodeToString(rndBytes[:])

	// Write the apply request to etcd.
	req := &config.ObjectStoreApplyRequest{
		ProposalID:       applyProposalID,
		Proposal:         proposal,
		ForceDestructive: applyForceDestructive,
		RequestedAt:      time.Now().UTC(),
		RequestID:        requestID,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	// Clear any stale result before writing the request.
	cli.Delete(ctx, config.EtcdKeyObjectStoreApplyResult) //nolint:errcheck

	writeCtx, writeCancel := context.WithTimeout(ctx, 10*time.Second)
	if _, err := cli.Put(writeCtx, config.EtcdKeyObjectStoreApplyRequest, string(reqData)); err != nil {
		writeCancel()
		return fmt.Errorf("write apply request: %w", err)
	}
	writeCancel()

	fmt.Printf("Apply request %s submitted — waiting for controller...\n", requestID)

	// Poll the result key.
	deadline := time.Now().Add(applyWaitTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)

		pollCtx, pollCancel := context.WithTimeout(ctx, 5*time.Second)
		resp, err := cli.Get(pollCtx, config.EtcdKeyObjectStoreApplyResult)
		pollCancel()
		if err != nil || len(resp.Kvs) == 0 {
			continue
		}

		var result config.ObjectStoreApplyResult
		if err := json.Unmarshal(resp.Kvs[0].Value, &result); err != nil {
			continue
		}
		// Match our request.
		if result.RequestID != requestID {
			continue
		}

		if result.Status == "accepted" {
			fmt.Printf("✓ Topology applied: generation=%d\n", result.Generation)
			fmt.Println("The topology workflow is running. Check progress with:")
			fmt.Println("  globular objectstore topology status")
			return nil
		}
		return fmt.Errorf("apply failed: %s", result.Error)
	}

	return fmt.Errorf("timeout waiting for controller to process apply request (try 'globular objectstore topology status')")
}

// ── wire commands ─────────────────────────────────────────────────────────────

func init() {
	objectstoreCmd.AddCommand(objectstoreDiskCmd)
	// topology plan and apply are added to objectstoreTopologyCmd (defined in
	// objectstore_cmds.go), so they appear as subcommands of `objectstore topology`.
	// objectstoreTopologyCmd.AddCommand(objectstoreTopologyPlanCmd) is in the
	// RunE block above.
}

// ── formatting helpers ────────────────────────────────────────────────────────

func basename(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func sortedKeys[K string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// clientv3 import kept alive.
var _ = (*clientv3.Client)(nil)
