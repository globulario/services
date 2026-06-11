// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.objectstore_physical_overlap
// @awareness file_role=doctor_rules_detecting_minio_pool_physical_storage_overlap_and_write_quorum_loss
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness risk=critical
package rules

// objectstore_physical_overlap.go — DIAGNOSTIC ONLY. Detects two
// failure-class topologies that produce silent MinIO corruption:
//
//   1. Two pool nodes whose drives resolve to the same physical
//      storage (NFS-mounted from one node, local on another).
//      Both write format.json to the same bytes → silent
//      corruption → heal deadlock on next restart.
//   2. Insufficient erasure-coding redundancy (EC:1, single-node
//      pool, fewer drives than write quorum).
//
// MUST NOT delete drives, rewrite format.json, or attempt to
// "fix" overlap. The blast radius of an incorrect auto-repair is
// data loss for everything stored in the pool. The rule emits
// findings; the operator runs `mc mirror` to a safe pool BEFORE
// any drive-count change. See
// docs/operators/minio-topology-validation.md and
// session_minio_topology_apply_inc2026_0010.md.

// Root cause this file guards against: two MinIO pool nodes configured with
// paths that resolve to the same physical storage (e.g., node A mounts
// 10.0.0.20:/mnt/data via NFS and node B is 10.0.0.20 with /mnt/data locally).
// Both nodes write format.json to the same bytes → silent corruption → heal
// deadlock on next startup.
//
// Invariants (in evaluation order):
//
//	objectstore.duplicate_physical_path — CRITICAL: two pool nodes share storage
//	objectstore.network_mount_used      — WARN: pool path is NFS/CIFS/SMB
//	objectstore.zero_write_fault_tolerance — WARN: EC:1 config is marginal
//	objectstore.write_quorum_lost       — CRITICAL: active nodes < write quorum
//	objectstore.format_heal_deadlock    — CRITICAL: all drives healing simultaneously

import (
	"fmt"
	"strings"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/config"
)

// ── topology helpers ──────────────────────────────────────────────────────────

// TopologyClass describes the quality of a MinIO pool topology.
type TopologyClass int

const (
	TopologyClassInvalid          TopologyClass = iota // physical overlap or network mount
	TopologyClassDevOnly                               // standalone / single node
	TopologyClassFragile                               // 3 drives, EC:1, fault_tolerance=1
	TopologyClassAcceptable                            // 4-5 drives, EC:2
	TopologyClassProductionReady                       // ≥6 drives, EC:3+, local block only
)

func (c TopologyClass) String() string {
	switch c {
	case TopologyClassInvalid:
		return "INVALID"
	case TopologyClassDevOnly:
		return "DEV_ONLY"
	case TopologyClassFragile:
		return "FRAGILE"
	case TopologyClassAcceptable:
		return "ACCEPTABLE"
	case TopologyClassProductionReady:
		return "PRODUCTION_READY"
	default:
		return "UNKNOWN"
	}
}

// classifyTopology returns the topology class and parity/quorum figures for
// the given ObjectStoreDesiredState. Returns DevOnly for standalone/nil.
func classifyTopology(desired *config.ObjectStoreDesiredState, hasNetworkMount, hasDuplicatePath bool) (TopologyClass, int, int) {
	if desired == nil || desired.Mode != config.ObjectStoreModeDistributed {
		return TopologyClassDevOnly, 0, 0
	}
	if hasDuplicatePath {
		return TopologyClassInvalid, 0, 0
	}
	if hasNetworkMount {
		return TopologyClassInvalid, 0, 0
	}
	nodeCount := len(desired.Nodes)
	drivesPerNode := desired.DrivesPerNode
	if drivesPerNode < 1 {
		drivesPerNode = 1
	}
	total := nodeCount * drivesPerNode
	parity := total / 2
	if parity < 1 {
		parity = 1
	}
	// MinIO write quorum: parity+1 drives required (not total-parity).
	// For N=3: WQ=2, FT=1. For N=4: WQ=3, FT=1. For N=6: WQ=4, FT=2.
	writeQuorum := parity + 1

	switch {
	case total < 4:
		return TopologyClassFragile, parity, writeQuorum
	case total < 6:
		return TopologyClassAcceptable, parity, writeQuorum
	default:
		return TopologyClassProductionReady, parity, writeQuorum
	}
}

// poolEndpoints returns a slice of (nodeID, nodeIP, configuredPath) for every
// node in the MinIO pool, joined from desired.NodePaths and snap.Nodes.
type poolEndpoint struct {
	nodeID string
	nodeIP string
	path   string
}

func poolEndpoints(snap *collector.Snapshot, desired *config.ObjectStoreDesiredState) []poolEndpoint {
	if desired == nil {
		return nil
	}
	// Use the IP-list-aware lookup so VIP-holders (whose AdvertiseIp may be
	// empty or the floating VIP) still resolve to their nodeID via any IP in
	// their Identity.Ips list. See node_ip_matching.go for the rationale.
	nodeIDByIP := nodeIDByPoolIP(snap.Nodes, desired.Nodes)
	var out []poolEndpoint
	for _, ip := range desired.Nodes {
		path := desired.NodePaths[ip]
		if path == "" {
			path = "/var/lib/globular/minio"
		}
		out = append(out, poolEndpoint{
			nodeID: nodeIDByIP[ip],
			nodeIP: ip,
			path:   path,
		})
	}
	return out
}

// findCandidateForPath returns the DiskCandidate whose MountPath is the
// longest prefix of configuredPath. Returns nil if none matches.
func findCandidateForPath(candidates []*config.DiskCandidate, configuredPath string) *config.DiskCandidate {
	var best *config.DiskCandidate
	for _, dc := range candidates {
		mp := dc.MountPath
		if mp == configuredPath || strings.HasPrefix(configuredPath, mp+"/") {
			if best == nil || len(mp) > len(best.MountPath) {
				best = dc
			}
		}
	}
	return best
}

// ── objectstore.duplicate_physical_path ──────────────────────────────────────
//
// CRITICAL when two or more MinIO pool endpoints resolve to the same physical
// storage:
//   - Same NFS/CIFS mount source (e.g., both point to "10.0.0.20:/mnt/data")
//   - Same block device stable ID on different nodes (same UUID on two nodes
//     indicates replication or shared SAN without volume isolation)
//
// This is the root cause of the ryzen NFS overlap incident: ryzen mounted
// dell's disk via NFS and both nodes competed to own the same format.json,
// eventually causing a heal deadlock after a power outage.

type objectstoreDuplicatePhysicalPath struct{}

func (objectstoreDuplicatePhysicalPath) ID() string {
	return "objectstore.duplicate_physical_path"
}
func (objectstoreDuplicatePhysicalPath) Category() string { return "objectstore" }
func (objectstoreDuplicatePhysicalPath) Scope() string    { return "cluster" }

func (objectstoreDuplicatePhysicalPath) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || desired.Mode != config.ObjectStoreModeDistributed {
		return nil
	}
	endpoints := poolEndpoints(snap, desired)
	if len(endpoints) < 2 {
		return nil
	}

	// Group endpoints by canonical physical key.
	//
	// The canonical key normalises both local mounts and NFS mounts to the
	// same form so cross-protocol overlap is detected:
	//
	//   NFS  10.0.0.20:/mnt/data/data (source 10.0.0.20:/mnt/data)  → "phys:10.0.0.20:/mnt/data"
	//   Local on 10.0.0.20 at /mnt/data                              → "phys:10.0.0.20:/mnt/data"
	//
	// Both produce the same key, flagging that ryzen and dell share bytes.
	//
	// For stable-ID keyed local disks (StableID != ""), use "uuid:{stableID}"
	// as a second key to catch shared SAN volumes that appear as local mounts
	// on multiple nodes.
	byPhysKey := make(map[string][]string) // key → list of "nodeIP:path"

	for _, ep := range endpoints {
		candidates := snap.DiskCandidates[ep.nodeID]
		if len(candidates) == 0 {
			continue
		}
		dc := findCandidateForPath(candidates, ep.path)
		if dc == nil {
			continue
		}

		if dc.IsNetworkMount && dc.MountSource != "" {
			// Canonical key for NFS: "phys:{server_ip}:{exported_root}"
			// MountSource = "host:export" → split on first ":"
			key := "phys:" + dc.MountSource
			byPhysKey[key] = append(byPhysKey[key], fmt.Sprintf("%s:%s", ep.nodeIP, ep.path))

			// Also generate the canonical key that the NFS server's local
			// mount would produce, so we detect cross-protocol overlap even
			// when the server node's DiskCandidate uses the local form.
			// "phys:10.0.0.20:/mnt/data" matches both the NFS consumer and
			// the local provider.
			// (The NFS mount source already IS "10.0.0.20:/mnt/data", so
			//  the above key is already the canonical form.)
		} else {
			// Canonical key for local block: "phys:{nodeIP}:{mountPath}".
			// ep.nodeIP is the desired-pool IP (always populated); using it
			// here both avoids the empty-AdvertiseIp pitfall and keeps the
			// key readable (IP, not UUID).
			key := "phys:" + ep.nodeIP + ":" + dc.MountPath
			byPhysKey[key] = append(byPhysKey[key], fmt.Sprintf("%s:%s", ep.nodeIP, ep.path))

			// Stable-ID key as secondary overlap detector (shared SAN, etc.)
			if dc.StableID != "" {
				uuidKey := "uuid:" + dc.StableID
				byPhysKey[uuidKey] = append(byPhysKey[uuidKey], fmt.Sprintf("%s:%s", ep.nodeIP, ep.path))
			}
		}
	}

	var overlaps []string
	for key, nodes := range byPhysKey {
		if len(nodes) > 1 {
			overlaps = append(overlaps, fmt.Sprintf("[%s → %s]", key, strings.Join(nodes, ", ")))
		}
	}
	if len(overlaps) == 0 {
		return nil
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.duplicate_physical_path", "cluster", strings.Join(overlaps, "|")),
		InvariantID: "objectstore.duplicate_physical_path",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO pool has %d physical storage overlap(s): %v. "+
				"Two pool nodes are sharing the same underlying bytes. "+
				"Both nodes write format.json to the same location, causing "+
				"silent corruption and a heal deadlock on next restart. "+
				"Reconfigure the affected node(s) to use a locally-attached drive.",
			len(overlaps), overlaps),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd+inventory", "objectstore_desired+disk_candidates", map[string]string{
				"overlapping_endpoints": strings.Join(overlaps, "; "),
				"topology_class":        "INVALID",
				"pool_nodes":            fmt.Sprintf("%d", len(endpoints)),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Identify which node uses an NFS/CIFS path for its MinIO drive",
				"globular objectstore disk scan"),
			step(2, "Update the node's MinIO path to a local block device",
				"globular objectstore topology plan  # generates a new proposal with corrected paths"),
			step(3, "Apply the corrected topology (destructive — data on the overlapping path will be re-initialized)",
				"globular objectstore topology apply --proposal <id> --i-understand-data-reset"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ── objectstore.network_mount_used ───────────────────────────────────────────
//
// WARN when any MinIO pool node's configured data path is served by a network
// filesystem (NFS, CIFS, SMB, etc.).
//
// Network mounts are inherently unreliable for MinIO:
//   - NFS retries mask transient failures but corrupt format.json on split-brain
//   - NFS performance is inadequate for EC write amplification
//   - A single NFS server failure takes down multiple MinIO nodes simultaneously
//   - Two pool nodes pointing at the same NFS export trigger duplicate_physical_path
//
// Even when no physical overlap is detected, network mounts should be replaced
// by locally-attached block storage.

type objectstoreNetworkMountUsed struct{}

func (objectstoreNetworkMountUsed) ID() string    { return "objectstore.network_mount_used" }
func (objectstoreNetworkMountUsed) Category() string { return "objectstore" }
func (objectstoreNetworkMountUsed) Scope() string    { return "cluster" }

func (objectstoreNetworkMountUsed) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || desired.Mode != config.ObjectStoreModeDistributed {
		return nil
	}
	endpoints := poolEndpoints(snap, desired)

	var netMounts []string
	for _, ep := range endpoints {
		candidates := snap.DiskCandidates[ep.nodeID]
		if len(candidates) == 0 {
			continue
		}
		dc := findCandidateForPath(candidates, ep.path)
		if dc == nil || !dc.IsNetworkMount {
			continue
		}
		label := fmt.Sprintf("%s:%s (fs=%s src=%s)", ep.nodeIP, ep.path, dc.FSType, dc.MountSource)
		netMounts = append(netMounts, label)
	}
	if len(netMounts) == 0 {
		return nil
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.network_mount_used", "cluster", strings.Join(netMounts, "|")),
		InvariantID: "objectstore.network_mount_used",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO pool has %d network-mounted data path(s): %v. "+
				"Network filesystems (NFS, CIFS, SMB) are unreliable for MinIO: "+
				"they mask transient errors, corrupt format.json on split-brain, "+
				"and a single NFS server failure takes down multiple pool nodes. "+
				"Replace with locally-attached block storage.",
			len(netMounts), netMounts),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd+inventory", "objectstore_desired+disk_candidates", map[string]string{
				"network_mount_paths": strings.Join(netMounts, "; "),
				"topology_class":      "INVALID",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Replace the NFS/CIFS path with a locally-attached drive",
				"globular objectstore disk scan  # verify local block device appears"),
			step(2, "Re-admit the local path",
				"globular objectstore disk approve --node <id> --path <local_path> --node-ip <ip>"),
			step(3, "Re-plan and apply topology with the corrected path",
				"globular objectstore topology plan && globular objectstore topology apply --proposal <id> --i-understand-data-reset"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ── objectstore.zero_write_fault_tolerance ───────────────────────────────────
//
// WARN when the pool is configured with ≤3 total drives (EC:1), yielding only
// 1 drive of write fault tolerance. Any single drive failure + one transient
// error = write unavailability.
//
// MinIO default EC parity = total_drives / 2 (floor, minimum 1).
// For 3 drives: parity=1, write_quorum=2. Fault tolerance = 1 drive.
//
// Recommendation: ≥4 drives (EC:2) provides 2 drives of fault tolerance.
// 6 drives (EC:3) is production-ready.

type objectstoreZeroWriteFaultTolerance struct{}

func (objectstoreZeroWriteFaultTolerance) ID() string {
	return "objectstore.zero_write_fault_tolerance"
}
func (objectstoreZeroWriteFaultTolerance) Category() string { return "objectstore" }
func (objectstoreZeroWriteFaultTolerance) Scope() string    { return "cluster" }

func (objectstoreZeroWriteFaultTolerance) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || desired.Mode != config.ObjectStoreModeDistributed {
		return nil
	}
	nodeCount := len(desired.Nodes)
	drivesPerNode := desired.DrivesPerNode
	if drivesPerNode < 1 {
		drivesPerNode = 1
	}
	total := nodeCount * drivesPerNode
	if total >= 4 {
		return nil // quorum_shape already warns for <4; this invariant is 3-drive specific
	}
	if total < 2 {
		return nil // quorum_shape already fires CRITICAL for <2 nodes
	}

	parity := total / 2
	if parity < 1 {
		parity = 1
	}
	writeQuorum := parity + 1
	faultTolerance := total - writeQuorum // drives that can fail before writes stop

	class, _, _ := classifyTopology(desired, false, false)

	return []Finding{{
		FindingID: FindingID("objectstore.zero_write_fault_tolerance", "cluster",
			fmt.Sprintf("total-%d", total)),
		InvariantID: "objectstore.zero_write_fault_tolerance",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO pool has %d total drives (%d nodes × %d drives): EC:%d, write_quorum=%d, "+
				"fault_tolerance=%d drive(s). Topology class: %s. "+
				"Losing %d drive(s) will block all writes. "+
				"Add drives or nodes to reach ≥4 total drives (EC:2, fault_tolerance=2).",
			total, nodeCount, drivesPerNode, parity, writeQuorum, faultTolerance, class,
			faultTolerance),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "objectstore_desired", map[string]string{
				"nodes":           fmt.Sprintf("%d", nodeCount),
				"drives_per_node": fmt.Sprintf("%d", drivesPerNode),
				"total_drives":    fmt.Sprintf("%d", total),
				"ec_parity":       fmt.Sprintf("%d", parity),
				"write_quorum":    fmt.Sprintf("%d", writeQuorum),
				"fault_tolerance": fmt.Sprintf("%d", faultTolerance),
				"topology_class":  class.String(),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Add a 4th storage node to reach EC:2 (2-drive fault tolerance)",
				"globular cluster join --profiles core,storage"),
			step(2, "Or add a 2nd drive per node (6 total drives → EC:3)",
				"globular objectstore disk approve --node <id> --path <path2> --drives 2"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ── objectstore.write_quorum_lost ─────────────────────────────────────────────
//
// CRITICAL when the number of MinIO pool nodes with an active service falls
// below the write quorum threshold (total_drives - EC_parity).
//
// This invariant detects live quorum loss — the cluster can read existing
// objects but cannot complete new writes until drives are restored.

type objectstoreWriteQuorumLost struct{}

func (objectstoreWriteQuorumLost) ID() string    { return "objectstore.write_quorum_lost" }
func (objectstoreWriteQuorumLost) Category() string { return "objectstore" }
func (objectstoreWriteQuorumLost) Scope() string    { return "cluster" }

func (objectstoreWriteQuorumLost) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || desired.Mode != config.ObjectStoreModeDistributed {
		return nil
	}
	// Only evaluate when the topology has been applied — pre-apply MinIO may not be running.
	if snap.ObjectStoreAppliedGeneration < desired.Generation {
		return nil
	}
	nodeCount := len(desired.Nodes)
	drivesPerNode := desired.DrivesPerNode
	if drivesPerNode < 1 {
		drivesPerNode = 1
	}
	total := nodeCount * drivesPerNode
	parity := total / 2
	if parity < 1 {
		parity = 1
	}
	writeQuorum := parity + 1

	// ── Per-IP classification ─────────────────────────────────────────────────
	//
	// Four buckets:
	//   active       — inventory present & fresh, globular-minio.service active
	//   knownDown    — inventory present & fresh, MinIO confirmed not active
	//   noInventory  — inventory absent, stale, or pool IP has no NodeRecord
	//   staleInv     — subset of noInventory: inventory exists but is older
	//                  than cfg.InventoryStalenessThreshold
	//
	// noInventory is ambiguous: node may be healthy but unreachable, or the
	// snapshot may be partial. When snap.DataIncomplete=true (collector logged
	// errors), we suppress CRITICAL if ALL down nodes are in noInventory.
	nodeIDByIP := nodeIDByPoolIP(snap.Nodes, desired.Nodes)

	// Parallel diagnostic maps — populated for every pool IP regardless of
	// which bucket it lands in. Written into evidence so operators can see the
	// exact per-node state that produced the verdict.
	nodeIDEvid := make([]string, 0, len(desired.Nodes))      // "ip=nodeID"
	invPresentEvid := make([]string, 0, len(desired.Nodes))  // "ip=yes|no|stale|no_node_record"
	minioStateEvid := make([]string, 0, len(desired.Nodes))  // "ip=state"
	unitNamesEvid := make([]string, 0)                        // "ip=[u1|u2|...]" only when state=missing

	var activeNodes, knownDownNodes, noInventoryNodes []string
	for _, ip := range desired.Nodes {
		nodeID := nodeIDByIP[ip]
		nodeIDEvid = append(nodeIDEvid, ip+"="+nodeID)

		if nodeID == "" {
			// No NodeRecord for this pool IP — snapshot may be partial.
			invPresentEvid = append(invPresentEvid, ip+"=no_node_record")
			minioStateEvid = append(minioStateEvid, ip+"=unknown(no_node_record)")
			noInventoryNodes = append(noInventoryNodes, ip)
			continue
		}

		inv := snap.Inventories[nodeID]
		if inv == nil {
			invPresentEvid = append(invPresentEvid, ip+"=no")
			minioStateEvid = append(minioStateEvid, ip+"=no_inventory")
			noInventoryNodes = append(noInventoryNodes, ip)
			continue
		}

		// Staleness check: if the inventory's own timestamp is older than the
		// configured threshold, treat the node as uncertain rather than
		// definitively down. This prevents a cached-snapshot race (MinIO
		// transitioning during the snapshot window) from producing a CRITICAL.
		if cfg.InventoryStalenessThreshold > 0 && inv.GetUnixTime() != nil {
			age := time.Since(inv.GetUnixTime().AsTime())
			if age > cfg.InventoryStalenessThreshold {
				invPresentEvid = append(invPresentEvid, fmt.Sprintf("%s=stale(age=%s)", ip, age.Round(time.Second)))
				minioStateEvid = append(minioStateEvid, ip+"=uncertain(stale_inventory)")
				noInventoryNodes = append(noInventoryNodes, ip)
				continue
			}
		}

		invPresentEvid = append(invPresentEvid, ip+"=yes")
		state := minioServiceState(snap, nodeID)
		minioStateEvid = append(minioStateEvid, ip+"="+state)

		switch state {
		case "active", "hash_drift", UnitStateUnitFileDrift:
			// Service-is-running-but-drifted states. systemd still reports
			// active; the release pipeline's re-install path handles the
			// drift — NOT a quorum-loss signal here. "hash_drift" is the
			// legacy name (pre-refactor); UnitStateUnitFileDrift is the new
			// name (see unit_receipt_drift.go). Accept both across the
			// upgrade window so stale inventories do not go dark.
			activeNodes = append(activeNodes, ip)
		case "no_inventory":
			noInventoryNodes = append(noInventoryNodes, ip)
		case "missing":
			// Unit not found in inventory at all. List the first 10 unit names
			// so operators can see what IS present (helps catch name mismatches).
			units := inv.GetUnits()
			names := make([]string, 0, len(units))
			for i, u := range units {
				if i >= 10 {
					names = append(names, "…")
					break
				}
				names = append(names, u.GetName())
			}
			unitNamesEvid = append(unitNamesEvid, ip+"=["+strings.Join(names, "|")+"]")
			knownDownNodes = append(knownDownNodes, ip)
		default:
			knownDownNodes = append(knownDownNodes, ip)
		}
	}

	activeDrives := len(activeNodes) * drivesPerNode
	if activeDrives >= writeQuorum {
		return nil
	}

	// If the collector failed to fetch node lists or inventories AND every
	// "down" node is only no_inventory (no confirmed-bad state), the finding
	// would be a false positive caused by the collector gap, not a real
	// quorum loss. Guard on the SPECIFIC data sources this rule depends on,
	// not the broad DataIncomplete flag — an unrelated DNS or Prometheus
	// error must not suppress a write-quorum CRITICAL.
	// See meta.fallback_must_degrade_semantics.
	inventoryGap := snap.HadError("cluster_controller", "ListNodes") ||
		snap.HadError("cluster_controller", "GetInventory") ||
		snap.HadError("node_agent", "GetInventory")
	if inventoryGap && len(knownDownNodes) == 0 {
		return nil
	}

	// Even with all noInventory nodes counted as active, quorum is still lost —
	// or we have at least one confirmed-down node. Fire CRITICAL.
	allDownNodes := append(knownDownNodes, noInventoryNodes...)

	return []Finding{{
		FindingID: FindingID("objectstore.write_quorum_lost", "cluster",
			fmt.Sprintf("active-%d-quorum-%d", activeDrives, writeQuorum)),
		InvariantID: "objectstore.write_quorum_lost",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO write quorum lost: active_drives=%d < write_quorum=%d "+
				"(pool=%d drives, EC:%d parity). "+
				"All new object writes will fail. Down nodes: %v.",
			activeDrives, writeQuorum, total, parity, allDownNodes),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd+inventory", "objectstore_desired+unit_state", map[string]string{
				"active_drives":              fmt.Sprintf("%d", activeDrives),
				"write_quorum":               fmt.Sprintf("%d", writeQuorum),
				"total_drives":               fmt.Sprintf("%d", total),
				"ec_parity":                  fmt.Sprintf("%d", parity),
				"active_nodes":               strings.Join(activeNodes, ","),
				"known_down_nodes":           strings.Join(knownDownNodes, ","),
				"no_inventory_nodes":         strings.Join(noInventoryNodes, ","),
				"snapshot_node_count":        fmt.Sprintf("%d", len(snap.Nodes)),
				"desired_pool_node_count":    fmt.Sprintf("%d", len(desired.Nodes)),
				"inventory_node_count":       fmt.Sprintf("%d", len(snap.Inventories)),
				"data_incomplete":            fmt.Sprintf("%v", snap.DataIncomplete),
				"node_id_by_pool_ip":         strings.Join(nodeIDEvid, ","),
				"inventory_present_by_pool_ip": strings.Join(invPresentEvid, ","),
				"minio_state_by_pool_ip":     strings.Join(minioStateEvid, ","),
				"unit_names_by_pool_ip":      strings.Join(unitNamesEvid, " | "),
				"evaluator_node":             cfg.LocalNodeID,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Restore the MinIO service on down nodes",
				"systemctl start globular-minio  # on each down node"),
			step(2, "If MinIO fails to start (heal deadlock), check format_heal_deadlock finding",
				"globular doctor explain objectstore.format_heal_deadlock"),
			step(3, "Inspect MinIO logs for heal errors",
				"journalctl -u globular-minio -n 100 --no-pager"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ── objectstore.format_heal_deadlock ─────────────────────────────────────────
//
// CRITICAL when all MinIO pool nodes are simultaneously down after having been
// previously initialized (.minio.sys present on all pool drives). This is the
// signature of a format heal deadlock:
//
//	MinIO startup → enters heal mode → waits for N/2+1 non-healing drives
//	→ all drives are in healing state → no reference drive → deadlock
//
// This exact scenario occurred in the Globular cluster after a power outage
// when ryzen's NFS overlap had already corrupted format.json on two drives
// (dell and ryzen shared the same physical bytes).
//
// Without a clean reference drive, the only recovery is to wipe .minio.sys on
// all nodes and re-initialize — which requires a ForceDestructive topology apply.

type objectstoreFormatHealDeadlock struct{}

func (objectstoreFormatHealDeadlock) ID() string    { return "objectstore.format_heal_deadlock" }
func (objectstoreFormatHealDeadlock) Category() string { return "objectstore" }
func (objectstoreFormatHealDeadlock) Scope() string    { return "cluster" }

func (objectstoreFormatHealDeadlock) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || desired.Mode != config.ObjectStoreModeDistributed {
		return nil
	}
	if snap.ObjectStoreAppliedGeneration < desired.Generation {
		return nil
	}

	// All pool nodes must be down (write_quorum_lost already fires for partial loss).
	// Here we fire specifically when EVERY node is down AND all have .minio.sys
	// (i.e., they were initialized before but are now stuck in heal).
	var (
		allDown       = true
		allHaveMinSys = true
		downWithSys   []string
		downNoSys     []string
	)

	formatHealNodeIDByIP := nodeIDByPoolIP(snap.Nodes, desired.Nodes)
	for _, ip := range desired.Nodes {
		nodeID := formatHealNodeIDByIP[ip]
		state := ""
		if nodeID != "" {
			state = minioServiceState(snap, nodeID)
		}
		if state == "active" {
			allDown = false
			break
		}
		// Check whether the pool path on this node has .minio.sys.
		path := desired.NodePaths[ip]
		if path == "" {
			path = "/var/lib/globular/minio"
		}
		hasSys := false
		if candidates := snap.DiskCandidates[nodeID]; len(candidates) > 0 {
			if dc := findCandidateForPath(candidates, path); dc != nil {
				hasSys = dc.HasMinioSys
			}
		}
		if hasSys {
			downWithSys = append(downWithSys, ip)
		} else {
			downNoSys = append(downNoSys, ip)
			allHaveMinSys = false
		}
	}

	if !allDown || !allHaveMinSys || len(downWithSys) < 2 {
		return nil
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.format_heal_deadlock", "cluster", strings.Join(downWithSys, ",")),
		InvariantID: "objectstore.format_heal_deadlock",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO format heal deadlock detected: all %d pool nodes are down and all "+
				"have .minio.sys (prior initialization). "+
				"MinIO requires at least 1 non-healing reference drive to complete startup. "+
				"With all drives simultaneously in healing mode, no reference drive exists "+
				"and startup is permanently blocked. "+
				"Recovery requires wiping .minio.sys on all nodes and re-initializing (data loss). "+
				"Affected nodes: %v.",
			len(downWithSys), downWithSys),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd+inventory", "objectstore_applied+disk_candidates+unit_state", map[string]string{
				"down_nodes_with_minio_sys": strings.Join(downWithSys, ","),
				"down_nodes_without_sys":   strings.Join(downNoSys, ","),
				"applied_generation":       fmt.Sprintf("%d", snap.ObjectStoreAppliedGeneration),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Verify the heal deadlock by inspecting MinIO startup logs on all nodes",
				"journalctl -u globular-minio -n 50 --no-pager  # look for 'healing' or 'waitForQuorum'"),
			step(2, "Check for objectstore.duplicate_physical_path — fix any NFS overlap first",
				"globular doctor  # read objectstore.duplicate_physical_path finding"),
			step(3, "Re-initialize MinIO by applying a destructive topology reset (DATA LOSS)",
				"globular objectstore topology apply --proposal <id> --i-understand-data-reset"),
			step(4, "Re-sync all packages from GitHub into the repository after MinIO is restored",
				"globular package sync --all"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}
