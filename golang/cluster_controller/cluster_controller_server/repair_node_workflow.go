package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/versionutil"
	"github.com/globulario/services/golang/workflow/engine"
)

// ---------------------------------------------------------------------------
// Repair data model
// ---------------------------------------------------------------------------

// RepairMode determines how repair sources artifacts and plans.
type RepairMode string

const (
	RepairFromRepository RepairMode = "from_repository"
	RepairFromReference  RepairMode = "from_reference"
	RepairFullReseed     RepairMode = "full_reseed"
)

// IdentityIntegrityStatus classifies the node's identity material health.
type IdentityIntegrityStatus string

const (
	IdentityClean   IdentityIntegrityStatus = "clean"
	IdentitySuspect IdentityIntegrityStatus = "suspect"
	IdentityCorrupt IdentityIntegrityStatus = "corrupt"
)

// PackageRepairIntent describes what action to take for a single package.
type PackageRepairIntent struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"`              // SERVICE, INFRASTRUCTURE, COMMAND
	Action           string `json:"action"`             // reinstall, restart, verify_only, skip
	ExpectedVersion  string `json:"expected_version"`
	ExpectedBuildNum int64  `json:"expected_build_number"`
	ExpectedBuildID  string `json:"expected_build_id"`
	ExpectedChecksum string `json:"expected_checksum"`   // from repo manifest
	ReferenceChecksum string `json:"reference_checksum"` // from reference node (validation)
	CurrentVersion   string `json:"current_version"`     // what broken node has
	CurrentChecksum  string `json:"current_checksum"`    // what broken node binary is
	DriftReason      string `json:"drift_reason"`        // version_mismatch, checksum_mismatch, partial_apply, missing, not_installed
}

// RepairPlan is the structured output of the classify step.
type RepairPlan struct {
	RepairMode               RepairMode            `json:"repair_mode"`
	NodeID                   string                `json:"node_id"`
	ReferenceNodeID          string                `json:"reference_node_id,omitempty"`
	Packages                 []PackageRepairIntent `json:"packages"`
	ControllerRepairRequired bool                  `json:"controller_repair_required"`
	IdentityAction           string                `json:"identity_action"` // none, rotate_certs, blocked
	PriorityOrder            []string              `json:"priority_order"`  // package names in repair order
}

// forbiddenIdentityFields are fields that must never appear in a repair plan.
// Used by ValidateRepairPlan to reject identity-copy attempts.
var forbiddenIdentityFields = []string{
	"copy_private_key", "copy_ca_key", "copy_node_id",
	"copy_identity", "clone_certs", "clone_keys",
}

// ValidateRepairPlan checks that a repair plan does not contain forbidden
// identity-copy actions. Returns an error if any are found.
func ValidateRepairPlan(plan map[string]any) error {
	for _, field := range forbiddenIdentityFields {
		if v, ok := plan[field]; ok && v != nil && v != false {
			return fmt.Errorf("repair plan contains forbidden identity-copy field %q — identity material must never be copied from another node", field)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Default repair priority ordering
// ---------------------------------------------------------------------------

// repairPriorityClass returns the priority class for a package.
// Lower number = repaired first.
//
// Default priority:
//   0: controller (must be safe before anything else)
//   1: node-agent (must be safe before it can apply other packages)
//   2: infrastructure (etcd, envoy, xds, gateway, minio, etc.)
//   3: services (everything else)
//
// The classifier may refine intra-class ordering dynamically from
// dependency/runtime facts collected from the reference node.
func repairPriorityClass(name, kind string) int {
	switch name {
	case "cluster-controller":
		return 0
	case "node-agent":
		return 1
	}
	if kind == "INFRASTRUCTURE" || kind == "COMMAND" {
		return 2
	}
	return 3
}

// ---------------------------------------------------------------------------
// Controller-side repair workflow handlers
// ---------------------------------------------------------------------------

// buildNodeRepairControllerConfig wires the node.repair workflow actions
// to live controller state.
func (srv *server) buildNodeRepairControllerConfig() engine.NodeRepairControllerConfig {
	return engine.NodeRepairControllerConfig{
		MarkStarted:       srv.repairMarkStarted,
		ValidateReference: srv.validateReferenceNode,
		Classify:          srv.repairClassify,
		IsolateNode:       srv.repairIsolateNode,
		RejoinNode:        srv.repairRejoinNode,
		MarkRecovered:     srv.repairMarkRecovered,
		MarkFailed:        srv.repairMarkFailed,
		EmitRecovered:     srv.repairEmitRecovered,
	}
}

// buildNodeRepairAgentConfig returns the NodeRepairAgentConfig that
// proxies node-agent repair actions to the target node via gRPC.
// The controller registers these handlers because the workflow service
// routes ALL actor callbacks through the controller (line 77 of
// workflow_execute.go).
func (srv *server) buildNodeRepairAgentConfig() engine.NodeRepairAgentConfig {
	return engine.NodeRepairAgentConfig{
		CollectRepairFacts: func(ctx context.Context, nodeID string, targetPackages []any) (map[string]any, error) {
			// For now, collect facts from installed-state in etcd (controller-side).
			// This avoids needing a new gRPC RPC on the node-agent.
			return srv.repairCollectFactsFromEtcd(ctx, nodeID)
		},
		RepairPackages: func(ctx context.Context, nodeID string, repairPlan map[string]any) (map[string]any, error) {
			return srv.repairPackagesViaAgent(ctx, nodeID, repairPlan)
		},
		RestartRepairedServices: func(ctx context.Context, nodeID string, repairResult map[string]any) error {
			// Restart is handled by ApplyPackageRelease (it restarts after install).
			log.Printf("repair: restart step — services already restarted by apply path")
			return nil
		},
		VerifyRepairRuntime: func(ctx context.Context, nodeID string, repairPlan map[string]any) error {
			return srv.repairVerifyRuntimeFromController(ctx, nodeID)
		},
		SyncInstalledState: func(ctx context.Context, nodeID string) error {
			// Node-agent sync happens on next heartbeat cycle after repair.
			log.Printf("repair: installed-state sync — will be synced on next heartbeat")
			return nil
		},
	}
}

// repairCollectFactsFromEtcd collects diagnostic facts about a node using
// the controller's view of installed-state in etcd. This is the controller-
// side proxy for the node.collect_repair_facts action.
func (srv *server) repairCollectFactsFromEtcd(ctx context.Context, nodeID string) (map[string]any, error) {
	var packageFacts []map[string]any

	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "COMMAND"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			fact := map[string]any{
				"name":    pkg.GetName(),
				"kind":    kind,
				"version": pkg.GetVersion(),
				"status":  pkg.GetStatus(),
			}
			if md := pkg.GetMetadata(); md != nil {
				fact["checksum"] = md["entrypoint_checksum"]
			}
			packageFacts = append(packageFacts, fact)
		}
	}

	// Check node's controller version for identity/version gate.
	identityStatus := "clean" // default from controller side
	srv.lock("repairCollectFacts")
	if node, ok := srv.state.Nodes[nodeID]; ok {
		if ctrlVer, ok := node.InstalledVersions["cluster-controller"]; ok {
			if !isReconcileSafe(ctrlVer) {
				identityStatus = "suspect" // old controller is suspicious
			}
		}
	}
	srv.unlock()

	return map[string]any{
		"node_id":                   nodeID,
		"packages":                  packageFacts,
		"identity_integrity_status": identityStatus,
		"package_count":             len(packageFacts),
	}, nil
}

// repairPackagesViaAgent dispatches ApplyPackageRelease RPCs to the target
// node's node-agent for each package in the repair plan.
func (srv *server) repairPackagesViaAgent(ctx context.Context, nodeID string, repairPlan map[string]any) (map[string]any, error) {
	packages, _ := repairPlan["packages"].([]PackageRepairIntent)
	if packages == nil {
		// Try to deserialize from map[string]any (workflow engine passes generic types).
		if pkgsAny, ok := repairPlan["packages"].([]any); ok {
			for _, p := range pkgsAny {
				if pm, ok := p.(map[string]any); ok {
					packages = append(packages, PackageRepairIntent{
						Name:            fmt.Sprint(pm["name"]),
						Kind:            fmt.Sprint(pm["kind"]),
						Action:          fmt.Sprint(pm["action"]),
						ExpectedVersion: fmt.Sprint(pm["expected_version"]),
					})
				}
			}
		}
	}

	priorityOrder, _ := repairPlan["priority_order"].([]string)
	if priorityOrder == nil {
		if poAny, ok := repairPlan["priority_order"].([]any); ok {
			for _, p := range poAny {
				priorityOrder = append(priorityOrder, fmt.Sprint(p))
			}
		}
	}

	// Build lookup for packages.
	byName := make(map[string]PackageRepairIntent)
	for _, pkg := range packages {
		byName[pkg.Name] = pkg
	}

	// Apply in priority order.
	repaired := 0
	skipped := 0
	failed := 0
	var failedNames []string

	applyOrder := priorityOrder
	if len(applyOrder) == 0 {
		for _, pkg := range packages {
			if pkg.Action == "reinstall" {
				applyOrder = append(applyOrder, pkg.Name)
			}
		}
	}

	// Resolve the node's agent endpoint for RPC dispatch.
	srv.lock("repairPackagesViaAgent")
	node, ok := srv.state.Nodes[nodeID]
	agentEndpoint := ""
	if ok && node != nil {
		agentEndpoint = node.AgentEndpoint
	}
	srv.unlock()
	if agentEndpoint == "" {
		return nil, fmt.Errorf("cannot resolve agent endpoint for node %s", nodeID)
	}

	// Resolve repository address.
	repo := resolveRepositoryInfo()

	for _, name := range applyOrder {
		pkg, ok := byName[name]
		if !ok || pkg.Action != "reinstall" {
			skipped++
			continue
		}

		log.Printf("repair: applying %s/%s@%s to node %s via %s", pkg.Kind, pkg.Name, pkg.ExpectedVersion, nodeID, agentEndpoint)

		err := srv.remoteApplyPackageRelease(ctx, nodeID, agentEndpoint,
			pkg.Name, pkg.Kind, pkg.ExpectedVersion,
			"", // publisher — resolved by node-agent
			repo.Address,
			pkg.ExpectedBuildNum,
			false, // force
			pkg.ExpectedBuildID)
		if err != nil {
			log.Printf("repair: ApplyPackageRelease %s failed: %v", name, err)
			failed++
			failedNames = append(failedNames, name)
			continue
		}
		log.Printf("repair: applied %s@%s successfully", name, pkg.ExpectedVersion)
		repaired++
	}

	if failed > 0 {
		return nil, fmt.Errorf("repair failed for %d packages: %v", failed, failedNames)
	}

	return map[string]any{
		"repaired": repaired,
		"skipped":  skipped,
		"node_id":  nodeID,
	}, nil
}

// repairVerifyRuntimeFromController verifies repair postconditions using
// the controller's view of the node (etcd installed-state + heartbeat data).
func (srv *server) repairVerifyRuntimeFromController(ctx context.Context, nodeID string) error {
	// Check all packages are in "installed" state.
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "COMMAND"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			status := pkg.GetStatus()
			if status == "partial_apply" || status == "failed" || status == "updating" {
				return fmt.Errorf("package %s/%s is in %s state", kind, pkg.GetName(), status)
			}
		}
	}

	// Check controller version on the repaired node.
	srv.lock("repairVerifyRuntime")
	node, ok := srv.state.Nodes[nodeID]
	if !ok {
		srv.unlock()
		return fmt.Errorf("node %s not found in cluster state", nodeID)
	}
	if ctrlVer, ok := node.InstalledVersions["cluster-controller"]; ok {
		if !isReconcileSafe(ctrlVer) {
			srv.unlock()
			return fmt.Errorf("controller version %s is still below minimum safe version %s", ctrlVer, minSafeReconcileVersion)
		}
	}
	srv.unlock()

	log.Printf("repair: runtime verification passed for %s", nodeID)
	return nil
}

func (srv *server) repairMarkStarted(ctx context.Context, nodeID, reason string) error {
	log.Printf("node-repair: started for %s — %s", nodeID, reason)
	srv.lock("repairMarkStarted")
	defer srv.unlock()
	if node, ok := srv.state.Nodes[nodeID]; ok {
		node.BootstrapPhase = "repairing"
	}
	return nil
}

func (srv *server) repairClassify(ctx context.Context, nodeID string, diagnosis map[string]any) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ValidateRepairPlan(diagnosis); err != nil {
		return nil, err
	}

	// Check identity integrity.
	identityStatus, _ := diagnosis["identity_integrity_status"].(string)
	if identityStatus == string(IdentityCorrupt) {
		return nil, fmt.Errorf("node %s identity is CORRUPT — repair blocked, requires operator approval for identity regeneration", nodeID)
	}

	identityAction := "none"
	if identityStatus == string(IdentitySuspect) {
		identityAction = "rotate_certs"
	}

	// Determine repair mode from diagnosis.
	repairMode := RepairFromRepository
	if modeStr, ok := diagnosis["repair_mode"].(string); ok && modeStr != "" {
		repairMode = RepairMode(modeStr)
	}

	referenceNodeID, _ := diagnosis["reference_node_id"].(string)

	// Collect desired state as the authority for target versions.
	desired := srv.collectDesiredVersions(ctx)

	// Collect installed state for the broken node.
	var packages []PackageRepairIntent
	controllerRepairRequired := false

	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "COMMAND"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			name := strings.TrimSpace(pkg.GetName())
			if name == "" {
				continue
			}
			desiredKey := kind + "/" + name
			dv := desired[desiredKey]

			// Also check SERVICE key for infra/command that might be desired as SERVICE.
			if dv.version == "" && kind != "SERVICE" {
				dv = desired["SERVICE/"+name]
			}

			if dv.version == "" {
				continue // not in desired state — skip (unmanaged)
			}

			intent := PackageRepairIntent{
				Name:             name,
				Kind:             kind,
				ExpectedVersion:  dv.version,
				ExpectedBuildNum: dv.buildNumber,
				ExpectedBuildID:  dv.buildID,
				CurrentVersion:   pkg.GetVersion(),
			}

			// Check for checksum metadata.
			if md := pkg.GetMetadata(); md != nil {
				intent.CurrentChecksum = md["entrypoint_checksum"]
			}

			// Classify drift.
			switch {
			case pkg.GetStatus() == "partial_apply":
				intent.Action = "reinstall"
				intent.DriftReason = "partial_apply"
			case !versionutil.EqualFull(dv.version, dv.buildNumber, pkg.GetVersion(), pkg.GetBuildNumber()):
				intent.Action = "reinstall"
				intent.DriftReason = "version_mismatch"
			default:
				intent.Action = "verify_only"
				intent.DriftReason = ""
			}

			if intent.Action == "reinstall" && name == "cluster-controller" {
				controllerRepairRequired = true
			}

			packages = append(packages, intent)
		}
	}

	// Check for desired packages not installed at all.
	installedNames := make(map[string]bool)
	for _, pkg := range packages {
		installedNames[pkg.Kind+"/"+pkg.Name] = true
	}
	for key, dv := range desired {
		if installedNames[key] {
			continue
		}
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		kind, name := parts[0], parts[1]
		packages = append(packages, PackageRepairIntent{
			Name:            name,
			Kind:            kind,
			Action:          "reinstall",
			ExpectedVersion: dv.version,
			ExpectedBuildNum: dv.buildNumber,
			ExpectedBuildID: dv.buildID,
			DriftReason:     "not_installed",
		})
		if name == "cluster-controller" {
			controllerRepairRequired = true
		}
	}

	// Check controller version gate.
	srv.lock("repairClassify:controllerVersion")
	if node, ok := srv.state.Nodes[nodeID]; ok {
		if ctrlVer, ok := node.InstalledVersions["cluster-controller"]; ok {
			if !isReconcileSafe(ctrlVer) {
				controllerRepairRequired = true
			}
		}
	}
	srv.unlock()

	// Build priority order.
	// Default: controller → node-agent → infrastructure → services
	// Classifier may refine intra-class ordering dynamically.
	type prioritized struct {
		name  string
		class int
	}
	var prios []prioritized
	for _, pkg := range packages {
		if pkg.Action == "verify_only" || pkg.Action == "skip" {
			continue
		}
		prios = append(prios, prioritized{name: pkg.Name, class: repairPriorityClass(pkg.Name, pkg.Kind)})
	}
	// Stable sort by priority class.
	for i := 0; i < len(prios); i++ {
		for j := i + 1; j < len(prios); j++ {
			if prios[j].class < prios[i].class {
				prios[i], prios[j] = prios[j], prios[i]
			}
		}
	}
	var priorityOrder []string
	for _, p := range prios {
		priorityOrder = append(priorityOrder, p.name)
	}

	plan := &RepairPlan{
		RepairMode:               repairMode,
		NodeID:                   nodeID,
		ReferenceNodeID:          referenceNodeID,
		Packages:                 packages,
		ControllerRepairRequired: controllerRepairRequired,
		IdentityAction:           identityAction,
		PriorityOrder:            priorityOrder,
	}

	// Convert to map for workflow output.
	result := map[string]any{
		"repair_mode":                string(plan.RepairMode),
		"node_id":                   plan.NodeID,
		"reference_node_id":         plan.ReferenceNodeID,
		"packages":                  plan.Packages,
		"controller_repair_required": plan.ControllerRepairRequired,
		"identity_action":           plan.IdentityAction,
		"priority_order":            plan.PriorityOrder,
	}

	log.Printf("node-repair: classified %s — %d packages to repair, controller_repair=%v, identity=%s",
		nodeID, len(priorityOrder), controllerRepairRequired, identityAction)

	return result, nil
}

func (srv *server) repairIsolateNode(ctx context.Context, nodeID string, repairPlan map[string]any) error {
	log.Printf("node-repair: isolating node %s from convergence", nodeID)
	srv.lock("repairIsolateNode")
	defer srv.unlock()
	if node, ok := srv.state.Nodes[nodeID]; ok {
		node.BootstrapPhase = "repairing"
	}
	return nil
}

func (srv *server) repairRejoinNode(ctx context.Context, nodeID string) error {
	log.Printf("node-repair: rejoining node %s", nodeID)
	srv.lock("repairRejoinNode")
	defer srv.unlock()
	if node, ok := srv.state.Nodes[nodeID]; ok {
		node.BootstrapPhase = "ready"
	}
	return nil
}

func (srv *server) repairMarkRecovered(ctx context.Context, nodeID string) error {
	log.Printf("node-repair: node %s RECOVERED", nodeID)
	return nil
}

func (srv *server) repairMarkFailed(ctx context.Context, nodeID string) error {
	log.Printf("node-repair: node %s repair FAILED — node remains isolated", nodeID)
	return nil
}

func (srv *server) repairEmitRecovered(ctx context.Context, nodeID string) error {
	log.Printf("node-repair: emitting recovery event for %s", nodeID)
	return nil
}

// ---------------------------------------------------------------------------
// Reference node validation
// ---------------------------------------------------------------------------

// validateReferenceNode checks that the reference node is healthy, converged,
// and running a safe controller version. Returns an error if the reference
// is not suitable for repair planning.
func (srv *server) validateReferenceNode(ctx context.Context, referenceNodeID string) error {
	srv.lock("validateReferenceNode")
	defer srv.unlock()

	node, ok := srv.state.Nodes[referenceNodeID]
	if !ok {
		return fmt.Errorf("reference node %s not found in cluster state", referenceNodeID)
	}

	// Check heartbeat freshness.
	if node.BootstrapPhase != "" && node.BootstrapPhase != "ready" {
		return fmt.Errorf("reference node %s is in phase %q, not ready", referenceNodeID, node.BootstrapPhase)
	}

	// Check hash convergence.
	if node.AppliedServicesHash == "" {
		return fmt.Errorf("reference node %s has no applied_hash — not yet converged", referenceNodeID)
	}
	srv.unlock()

	// Compute desired hash for this node's service set.
	desiredCanon, _, _ := srv.loadDesiredServices(context.Background())
	// Also include InfrastructureRelease components.
	if srv.resources != nil {
		if items, _, err := srv.resources.List(context.Background(), "InfrastructureRelease", ""); err == nil {
			for _, obj := range items {
				if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil {
					canon := canonicalServiceName(rel.Spec.Component)
					if _, exists := desiredCanon[canon]; !exists {
						desiredCanon[canon] = rel.Spec.Version
					}
				}
			}
		}
	}

	srv.lock("validateReferenceNode:hash")
	node = srv.state.Nodes[referenceNodeID] // re-read under lock
	if node == nil {
		srv.unlock()
		return fmt.Errorf("reference node %s disappeared during validation", referenceNodeID)
	}
	filtered := filterVersionsForNode(desiredCanon, node)
	desiredHash := stableServiceDesiredHash(filtered)
	if node.AppliedServicesHash != desiredHash {
		return fmt.Errorf("reference node %s is not converged: applied_hash=%s desired_hash=%s",
			referenceNodeID, node.AppliedServicesHash[:16], desiredHash[:16])
	}

	// Check controller version.
	if ctrlVer, ok := node.InstalledVersions["cluster-controller"]; ok {
		if !isReconcileSafe(ctrlVer) {
			return fmt.Errorf("reference node %s controller version %s is below minimum safe version %s",
				referenceNodeID, ctrlVer, minSafeReconcileVersion)
		}
	}

	return nil
}
