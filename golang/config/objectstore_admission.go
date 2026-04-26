package config

// objectstore_admission.go defines the etcd schema and Go types for MinIO disk
// admission and topology planning.
//
// Ownership invariants:
//   - /globular/nodes/{id}/storage/candidates/*  — node-agent ONLY (read-only observation)
//   - /globular/objectstore/disk/admitted/*       — operator only (CLI approve)
//   - /globular/objectstore/disk/rejected/*       — operator only (CLI reject)
//   - /globular/objectstore/topology/proposals/*  — CLI planner (computed from admitted disks)
//   - /globular/objectstore/topology/apply_request — CLI → controller handoff (transient)
//   - /globular/objectstore/topology/apply_result  — controller → CLI response (transient)
//
// The controller NEVER writes to disk candidate keys.
// Node agents NEVER write to admission or topology keys.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ── etcd key helpers ──────────────────────────────────────────────────────────

// EtcdKeyDiskCandidate returns the etcd key for a single disk candidate report.
func EtcdKeyDiskCandidate(nodeID, diskID string) string {
	return "/globular/nodes/" + nodeID + "/storage/candidates/" + diskID
}

// EtcdKeyDiskCandidatesPrefix returns the prefix for all disk candidates on a node.
func EtcdKeyDiskCandidatesPrefix(nodeID string) string {
	return "/globular/nodes/" + nodeID + "/storage/candidates/"
}

// EtcdKeyDiskAdmitted returns the etcd key for an admitted disk.
func EtcdKeyDiskAdmitted(nodeID, pathHash string) string {
	return "/globular/objectstore/disk/admitted/" + nodeID + "/" + pathHash
}

// EtcdKeyDiskAdmittedNodePrefix returns the prefix for all admitted disks on a node.
func EtcdKeyDiskAdmittedNodePrefix(nodeID string) string {
	return "/globular/objectstore/disk/admitted/" + nodeID + "/"
}

// EtcdKeyAllAdmittedDisksPrefix is the global prefix for all admitted disks.
const EtcdKeyAllAdmittedDisksPrefix = "/globular/objectstore/disk/admitted/"

// EtcdKeyTopologyProposal returns the etcd key for a topology proposal.
func EtcdKeyTopologyProposal(proposalID string) string {
	return "/globular/objectstore/topology/proposals/" + proposalID
}

// EtcdKeyTopologyProposalsPrefix is the prefix for all topology proposals.
const EtcdKeyTopologyProposalsPrefix = "/globular/objectstore/topology/proposals/"

// EtcdKeyTopologyTransition returns the etcd key for a destructive topology
// transition record. Written by the controller when an apply request with
// ForceDestructive=true is accepted. Node agents read this before wiping
// .minio.sys to confirm the wipe was explicitly approved.
func EtcdKeyTopologyTransition(generation int64) string {
	return fmt.Sprintf("/globular/objectstore/topology/transition/%d", generation)
}

// EtcdKeyObjectStoreApplyRequest is written by the CLI to request a topology
// apply. The controller watches this key and processes it.
const EtcdKeyObjectStoreApplyRequest = "/globular/objectstore/topology/apply_request"

// EtcdKeyObjectStoreApplyResult is written by the controller after processing
// an apply request. The CLI polls this key for the result.
const EtcdKeyObjectStoreApplyResult = "/globular/objectstore/topology/apply_result"

// EtcdKeyObjectStoreAppliedStateFingerprint stores the state fingerprint for
// the last successfully applied topology generation. Written after health
// verification in the topology workflow.
const EtcdKeyObjectStoreAppliedStateFingerprint = "/globular/objectstore/topology/applied_state_fingerprint"

// EtcdKeyObjectStoreAppliedVolumesHash stores the volumes_hash for the last
// successfully applied topology generation. Written alongside the fingerprint.
const EtcdKeyObjectStoreAppliedVolumesHash = "/globular/objectstore/topology/applied_volumes_hash"

// ── DiskCandidate ─────────────────────────────────────────────────────────────

// DiskCandidate is a read-only fact about a mounted filesystem on a node.
// Written exclusively by the node-agent. The node-agent NEVER writes
// objectstore desired state — it only reports what it observes.
//
// +globular:schema:key="/globular/nodes/{node_id}/storage/candidates/{disk_id}"
// +globular:schema:writer="globular-node-agent"
// +globular:schema:readers="globular-cluster-controller,globular-cluster-doctor,globular-cli"
type DiskCandidate struct {
	NodeID          string    `json:"node_id"`
	DiskID          string    `json:"disk_id"`
	Device          string    `json:"device"`
	MountPath       string    `json:"mount_path"`
	FSType          string    `json:"fs_type"`
	SizeBytes       int64     `json:"size_bytes"`
	AvailableBytes  int64     `json:"available_bytes"`
	StableID        string    `json:"stable_id,omitempty"`
	IsRoot          bool      `json:"is_root"`
	IsRemovable     bool      `json:"is_removable"`
	HasMinioSys     bool      `json:"has_minio_sys"`
	HasExistingData bool      `json:"has_existing_data"`
	Eligible        bool      `json:"eligible"`
	Reasons         []string  `json:"reasons,omitempty"`
	ReportedAt      time.Time `json:"reported_at"`
}

// DiskIDFromPath returns a stable 12-hex-char ID from device+mountPath.
func DiskIDFromPath(device, mountPath string) string {
	h := sha256.Sum256([]byte(device + "|" + mountPath))
	return fmt.Sprintf("%x", h[:6])
}

// PathHash returns a 12-hex-char hash of a path, used as the etcd key suffix.
func PathHash(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:6])
}

// ── AdmittedDisk ──────────────────────────────────────────────────────────────

// AdmittedDisk records an operator's explicit approval of a disk path on a node.
// Written by `globular objectstore disk approve`.
//
// Physical identity fields (StableID, Device, FSType, SizeBytes, AdmittedCandidateDiskID)
// are copied from the DiskCandidate at admission time. The controller compares them
// against the live candidate on each apply to detect silent disk replacements.
//
// +globular:schema:key="/globular/objectstore/disk/admitted/{node_id}/{path_hash}"
// +globular:schema:writer="globular-cli (operator)"
type AdmittedDisk struct {
	NodeID            string    `json:"node_id"`
	NodeIP            string    `json:"node_ip"`
	Path              string    `json:"path"`
	PathHash          string    `json:"path_hash"`
	DrivesPerNode     int       `json:"drives_per_node,omitempty"`
	ForceRoot         bool      `json:"force_root,omitempty"`
	ForceExistingData bool      `json:"force_existing_data,omitempty"`
	ApprovedAt        time.Time `json:"approved_at"`
	ApprovedBy        string    `json:"approved_by,omitempty"`

	// Physical identity captured from DiskCandidate at approval time.
	// Used to detect disk replacement behind the same mount path.
	StableID                string `json:"stable_id,omitempty"`                  // blkid PARTUUID / UUID
	Device                  string `json:"device,omitempty"`                      // e.g. /dev/sdb1
	FSType                  string `json:"fs_type,omitempty"`                     // ext4, xfs, btrfs …
	SizeBytesAtAdmission    int64  `json:"size_bytes_at_admission,omitempty"`     // capacity when admitted
	AdmittedCandidateDiskID string `json:"admitted_candidate_disk_id,omitempty"` // DiskCandidate.DiskID
}

// ── TopologyProposal ──────────────────────────────────────────────────────────

// TopologyProposal is a computed MinIO topology from admitted disks.
// Written by `globular objectstore topology plan`.
// Applied by `globular objectstore topology apply --proposal <id>`.
//
// A proposal is NEVER applied automatically. Destructive proposals require
// --i-understand-data-reset.
//
// +globular:schema:key="/globular/objectstore/topology/proposals/{proposal_id}"
// +globular:schema:writer="globular-cli (operator)"
type TopologyProposal struct {
	ProposalID         string            `json:"proposal_id"`
	GeneratedAt        time.Time         `json:"generated_at"`
	NodePaths          map[string]string `json:"node_paths"`        // nodeIP → basePath
	DrivesPerNode      int               `json:"drives_per_node"`
	Nodes              []string          `json:"nodes"`             // ordered pool IPs
	IsDestructive      bool              `json:"is_destructive"`
	DestructiveReasons []string          `json:"destructive_reasons,omitempty"`
	ValidationErrors   []string          `json:"validation_errors,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`
	Status             string            `json:"status"` // "proposed" | "applied" | "superseded"
}

// Valid returns true when the proposal has no ValidationErrors.
func (p *TopologyProposal) Valid() bool { return len(p.ValidationErrors) == 0 }

// ── ObjectStoreApplyRequest ───────────────────────────────────────────────────

// ObjectStoreApplyRequest is written by the CLI to request a topology apply.
// The controller watches /globular/objectstore/topology/apply_request and
// processes it on the next reconcile cycle. This key is transient.
type ObjectStoreApplyRequest struct {
	ProposalID       string            `json:"proposal_id"`
	Proposal         *TopologyProposal `json:"proposal"`
	ForceDestructive bool              `json:"force_destructive"`
	RequestedAt      time.Time         `json:"requested_at"`
	RequestID        string            `json:"request_id"` // random nonce for CLI to match result
}

// ── ObjectStoreApplyResult ────────────────────────────────────────────────────

// ObjectStoreApplyResult is written by the controller after processing an apply
// request. The CLI polls for this key.
type ObjectStoreApplyResult struct {
	RequestID   string    `json:"request_id"`
	ProposalID  string    `json:"proposal_id"`
	Status      string    `json:"status"`              // "accepted" | "failed"
	Generation  int64     `json:"generation,omitempty"` // new generation (on accepted)
	Error       string    `json:"error,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
}

// ── etcd I/O helpers ──────────────────────────────────────────────────────────

// LoadDiskCandidates returns all disk candidates reported by a node.
func LoadDiskCandidates(ctx context.Context, nodeID string) ([]*DiskCandidate, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("disk candidates: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, EtcdKeyDiskCandidatesPrefix(nodeID), clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("disk candidates: etcd get: %w", err)
	}
	out := make([]*DiskCandidate, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var dc DiskCandidate
		if err := json.Unmarshal(kv.Value, &dc); err != nil {
			continue
		}
		out = append(out, &dc)
	}
	return out, nil
}

// LoadAllDiskCandidates returns disk candidates keyed by nodeID across all nodes.
// It only returns entries whose etcd key contains "storage/candidates".
func LoadAllDiskCandidates(ctx context.Context) (map[string][]*DiskCandidate, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("disk candidates: etcd unavailable: %w", err)
	}
	// Enumerate all node IDs first by listing /globular/nodes/ prefix,
	// then filter for storage/candidates entries.
	resp, err := cli.Get(ctx, "/globular/nodes/", clientv3.WithPrefix(),
		clientv3.WithKeysOnly())
	if err != nil {
		return nil, fmt.Errorf("disk candidates: etcd list nodes: %w", err)
	}
	// Collect unique node IDs that have candidate keys.
	seen := make(map[string]bool)
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.Contains(key, "/storage/candidates/") {
			continue
		}
		// /globular/nodes/{nodeID}/storage/candidates/{diskID}
		parts := strings.SplitN(strings.TrimPrefix(key, "/globular/nodes/"), "/", 2)
		if len(parts) < 1 || parts[0] == "" {
			continue
		}
		seen[parts[0]] = true
	}

	out := make(map[string][]*DiskCandidate)
	for nodeID := range seen {
		candidates, err := LoadDiskCandidates(ctx, nodeID)
		if err != nil {
			continue
		}
		out[nodeID] = candidates
	}
	return out, nil
}

// SaveDiskCandidate writes a single disk candidate to etcd.
func SaveDiskCandidate(ctx context.Context, dc *DiskCandidate) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("disk candidate: etcd unavailable: %w", err)
	}
	data, err := json.Marshal(dc)
	if err != nil {
		return fmt.Errorf("disk candidate: marshal: %w", err)
	}
	key := EtcdKeyDiskCandidate(dc.NodeID, dc.DiskID)
	if _, err := cli.Put(ctx, key, string(data)); err != nil {
		return fmt.Errorf("disk candidate: etcd put %s: %w", key, err)
	}
	return nil
}

// DeleteStaleNodeCandidates removes candidate keys for disk IDs no longer reported.
func DeleteStaleNodeCandidates(ctx context.Context, nodeID string, activeDiskIDs map[string]bool) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("disk candidates: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, EtcdKeyDiskCandidatesPrefix(nodeID),
		clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return fmt.Errorf("disk candidates: etcd get keys: %w", err)
	}
	prefix := EtcdKeyDiskCandidatesPrefix(nodeID)
	for _, kv := range resp.Kvs {
		diskID := strings.TrimPrefix(string(kv.Key), prefix)
		if !activeDiskIDs[diskID] {
			if _, err := cli.Delete(ctx, string(kv.Key)); err != nil {
				return fmt.Errorf("disk candidates: delete stale %s: %w", kv.Key, err)
			}
		}
	}
	return nil
}

// LoadAdmittedDisks returns all admitted disks across all nodes.
func LoadAdmittedDisks(ctx context.Context) ([]*AdmittedDisk, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("admitted disks: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, EtcdKeyAllAdmittedDisksPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("admitted disks: etcd get: %w", err)
	}
	out := make([]*AdmittedDisk, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var ad AdmittedDisk
		if err := json.Unmarshal(kv.Value, &ad); err != nil {
			continue
		}
		out = append(out, &ad)
	}
	return out, nil
}

// SaveAdmittedDisk writes an admitted disk record to etcd.
func SaveAdmittedDisk(ctx context.Context, ad *AdmittedDisk) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("admitted disk: etcd unavailable: %w", err)
	}
	data, err := json.Marshal(ad)
	if err != nil {
		return fmt.Errorf("admitted disk: marshal: %w", err)
	}
	key := EtcdKeyDiskAdmitted(ad.NodeID, ad.PathHash)
	if _, err := cli.Put(ctx, key, string(data)); err != nil {
		return fmt.Errorf("admitted disk: etcd put %s: %w", key, err)
	}
	return nil
}

// DeleteAdmittedDisk removes an admitted disk record.
func DeleteAdmittedDisk(ctx context.Context, nodeID, pathHash string) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("admitted disk: etcd unavailable: %w", err)
	}
	key := EtcdKeyDiskAdmitted(nodeID, pathHash)
	if _, err := cli.Delete(ctx, key); err != nil {
		return fmt.Errorf("admitted disk: etcd delete %s: %w", key, err)
	}
	return nil
}

// SaveTopologyProposal writes a topology proposal to etcd.
func SaveTopologyProposal(ctx context.Context, p *TopologyProposal) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("topology proposal: etcd unavailable: %w", err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("topology proposal: marshal: %w", err)
	}
	key := EtcdKeyTopologyProposal(p.ProposalID)
	if _, err := cli.Put(ctx, key, string(data)); err != nil {
		return fmt.Errorf("topology proposal: etcd put %s: %w", key, err)
	}
	return nil
}

// ── TopologyTransition ────────────────────────────────────────────────────────

// TopologyTransition records an approved destructive topology change.
// Written by the controller when a topology apply request with ForceDestructive=true
// is accepted. Node agents MUST check for this record before wiping .minio.sys —
// they must not wipe based on local env-file inference alone.
//
// +globular:schema:key="/globular/objectstore/topology/transition/{generation}"
// +globular:schema:writer="globular-cluster-controller"
// +globular:schema:readers="globular-node-agent"
type TopologyTransition struct {
	Generation    int64             `json:"generation"`
	IsDestructive bool              `json:"is_destructive"`
	AffectedNodes []string          `json:"affected_nodes"`
	AffectedPaths map[string]string `json:"affected_paths"` // nodeIP → base path
	Reasons       []string          `json:"reasons"`
	Approved      bool              `json:"approved"` // ForceDestructive was set by operator
	CreatedAt     time.Time         `json:"created_at"`
}

// SaveTopologyTransition writes a transition record to etcd.
func SaveTopologyTransition(ctx context.Context, t *TopologyTransition) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("topology transition: etcd unavailable: %w", err)
	}
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("topology transition: marshal: %w", err)
	}
	key := EtcdKeyTopologyTransition(t.Generation)
	if _, err := cli.Put(ctx, key, string(data)); err != nil {
		return fmt.Errorf("topology transition: etcd put %s: %w", key, err)
	}
	return nil
}

// LoadTopologyTransition reads the transition record for a given generation.
// Returns nil, nil when no record exists for that generation.
func LoadTopologyTransition(ctx context.Context, generation int64) (*TopologyTransition, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("topology transition: etcd unavailable: %w", err)
	}
	key := EtcdKeyTopologyTransition(generation)
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("topology transition: etcd get %s: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var t TopologyTransition
	if err := json.Unmarshal(resp.Kvs[0].Value, &t); err != nil {
		return nil, fmt.Errorf("topology transition: parse: %w", err)
	}
	return &t, nil
}

// DeleteTopologyTransition removes the transition record for a given generation.
// Called by the controller to clean up a pre-written transition when a
// subsequent step (persist state, concurrent-apply guard) fails.
func DeleteTopologyTransition(ctx context.Context, generation int64) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("topology transition: etcd unavailable: %w", err)
	}
	key := EtcdKeyTopologyTransition(generation)
	if _, err := cli.Delete(ctx, key); err != nil {
		return fmt.Errorf("topology transition: etcd delete %s: %w", key, err)
	}
	return nil
}

// LoadTopologyProposal reads a topology proposal from etcd.
func LoadTopologyProposal(ctx context.Context, proposalID string) (*TopologyProposal, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("topology proposal: etcd unavailable: %w", err)
	}
	key := EtcdKeyTopologyProposal(proposalID)
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("topology proposal: etcd get %s: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("topology proposal %q not found", proposalID)
	}
	var p TopologyProposal
	if err := json.Unmarshal(resp.Kvs[0].Value, &p); err != nil {
		return nil, fmt.Errorf("topology proposal: parse: %w", err)
	}
	return &p, nil
}
