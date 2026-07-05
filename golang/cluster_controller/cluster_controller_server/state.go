package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/netutil"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// minioCredentialsFile is the installer-managed file that holds the MinIO
// root credentials in "access:secret" format. When this file exists we
// reuse those credentials so the controller and the running MinIO instance
// share the same root account without any reconciliation step.
const minioCredentialsFile = "/var/lib/globular/minio/credentials"

// generateMinioCredentials creates random MinIO root credentials.
// Access key is kept to ≤20 chars (MinIO enforces this limit).
func generateMinioCredentials() *minioCredentials {
	user := make([]byte, 5) // "gl-" (3) + 10 hex chars = 13 chars total (well under 20)
	pass := make([]byte, 16)
	rand.Read(user)
	rand.Read(pass)
	return &minioCredentials{
		RootUser:     "gl-" + hex.EncodeToString(user),
		RootPassword: hex.EncodeToString(pass),
	}
}

// readOrGenerateMinioCredentials reads the MinIO root credentials from the
// well-known installer credential file when it exists and is valid. Falls back
// to generating fresh random credentials. This prevents a mismatch between the
// credentials the controller stores in etcd and the credentials MinIO was
// actually initialized with by the installer package.
func readOrGenerateMinioCredentials() *minioCredentials {
	data, err := os.ReadFile(minioCredentialsFile)
	if err == nil {
		line := strings.TrimSpace(string(data))
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" &&
			len(parts[0]) <= 20 { // MinIO access key max is 20 chars
			log.Printf("minio-creds: read from %s (user=%s)", minioCredentialsFile, parts[0])
			return &minioCredentials{
				RootUser:     parts[0],
				RootPassword: parts[1],
			}
		}
	}
	creds := generateMinioCredentials()
	log.Printf("minio-creds: generated fresh credentials (user=%s)", creds.RootUser)
	return creds
}

// defaultClusterStatePath is the canonical state-file location. The
// hyphenated directory is the documented runtime-dir convention; the previous
// "clustercontroller" (no separator) form was a typo that escaped review.
// MigrateLegacyStatePathOnce handles the rename so existing nodes keep their
// state across the upgrade.
const defaultClusterStatePath = "/var/lib/globular/cluster-controller/state.json"

// legacyClusterStatePath is the pre-Project-O location. Kept here for the
// migration helper only — do not write to it.
const legacyClusterStatePath = "/var/lib/globular/clustercontroller/state.json"

// controllerState is the in-memory cluster state that cluster-controller
// persists to etcd (authoritative) and to a local JSON backup. It holds
// join tokens, per-node identity/health, MinIO pool membership, and the
// cluster network spec.
//
// +globular:schema:key="/globular/clustercontroller/state"
// +globular:schema:writer="globular-cluster-controller"
// +globular:schema:readers="globular-cluster-controller"
// +globular:schema:description="Authoritative cluster state: nodes, join tokens, MinIO pool, network spec."
// +globular:schema:invariants="Single-writer; local disk copy is a backup only; persistStateLocked is the ONLY write path."
type controllerState struct {
	JoinTokens           map[string]*joinTokenRecord             `json:"join_tokens"`
	JoinRequests         map[string]*joinRequestRecord           `json:"join_requests"`
	Nodes                map[string]*nodeState                   `json:"nodes"`
	// ClusterId is the cluster DNS/storage NAMESPACE (the domain). It is NOT a
	// membership identity — see ClusterUID. (Historically overloaded; the identity
	// program splits the two. Kept for the namespace/scoping role.)
	ClusterId string `json:"cluster_id"`
	// ClusterUID is the opaque cluster MEMBERSHIP identity — the minted UUID from
	// /globular/system/cluster/id (config.ClusterMembershipIDKey). Populated by the
	// Day-0 seed. This is what identity readers (join gate, membership records,
	// GetClusterInfo) use; the domain is never a membership credential.
	ClusterUID           string                                  `json:"cluster_uid,omitempty"`
	CreatedAt            time.Time                               `json:"created_at"`
	ClusterNetworkSpec   *cluster_controllerpb.ClusterNetworkSpec `json:"cluster_network_spec,omitempty"`
	NetworkingGeneration uint64                                  `json:"networking_generation"`
	// MinIO pool membership — ordered, append-only list of node IPs.
	// New nodes are appended; existing entries never change order.
	// This preserves erasure set boundaries across pool expansion.
	MinioPoolNodes       []string          `json:"minio_pool_nodes,omitempty"`
	MinioCredentials     *minioCredentials `json:"minio_credentials,omitempty"`
	MinioNodePaths       map[string]string `json:"minio_node_paths,omitempty"`      // IP → base data path (default: /var/lib/globular/minio)
	MinioDrivesPerNode   int               `json:"minio_drives_per_node,omitempty"` // drives per node (0/1 = single, 2+ = multi-drive)
	// ObjectStoreGeneration is incremented each time the MinIO pool topology changes.
	// Monotonically increasing. Node agents compare their observed generation to detect drift.
	ObjectStoreGeneration int64 `json:"objectstore_generation,omitempty"`

	// DesiredObjectStoreMembers is the Phase E-lite explicit desired membership list.
	// When non-nil, reconcileMinioJoinPhases uses this list as the authoritative gate
	// instead of the profile-derived profilesForMinio check.
	//
	// nil → legacy mode: profilesForMinio governs (backward compat for existing clusters).
	// non-nil → v2 mode: only nodes listed here may join the MinIO erasure pool.
	//
	// Populated by approveJoinRecordLocked (Day-0 bootstrap) or apply-topology calls.
	// On first upgrade, objectStoreDesiredMembersFromIntents migrates from intent flags.
	DesiredObjectStoreMembers []ObjectStoreMember `json:"desired_objectstore_members,omitempty"`

	// PendingObjectStoreTransition is the in-flight topology transition record.
	// Phase E.1: membership changes must go through a generation-gated transition.
	// nil means no transition is currently pending or applying.
	// Once applied (status==applied), callers may clear this field or archive it.
	PendingObjectStoreTransition *ObjectStoreTopologyTransition `json:"pending_objectstore_transition,omitempty"`

	// CAGeneration is incremented each time the cluster CA is rotated.
	// Monotonically increasing. Published in CAMetadata so doctor rules can track
	// per-node CA drift (node still on generation N-1 after controller moved to N).
	CAGeneration int64 `json:"ca_generation,omitempty"`

	// lastPersistedHash is the SHA-256 of the JSON-serialized state at the
	// last SUCCESSFUL saveToEtcd write. saveToEtcd uses this to skip the
	// etcd Put when the serialized state is byte-identical to the previous
	// one — Phase 36 mitigation for /globular/clustercontroller/state
	// dominating etcd MVCC bloat (96% of cumulative write load on a
	// single-node cluster). Zero value = "never persisted" → first write
	// always proceeds. Tagged json:"-" so it never round-trips to etcd or
	// disk; it's a runtime-only deduplication cache.
	lastPersistedHash [sha256.Size]byte `json:"-"`
}

// minioCredentials holds the MinIO root credentials for the cluster.
// Generated at bootstrap, shared across all MinIO nodes.
type minioCredentials struct {
	RootUser     string `json:"root_user"`
	RootPassword string `json:"root_password"`
}

type joinTokenRecord struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	MaxUses   int       `json:"max_uses"`
	Uses      int       `json:"uses"`
}

type joinRequestRecord struct {
	RequestID         string              `json:"request_id"`
	Token             string              `json:"token"`
	Identity          storedIdentity      `json:"identity"`
	Labels            map[string]string   `json:"labels"`
	RequestedAt       time.Time           `json:"requested_at"`
	// Status is the legacy string status. New code must prefer LifecyclePhase.
	// Kept for backward compatibility with persisted state and v1 API consumers.
	Status            string              `json:"status"`
	Reason            string              `json:"reason,omitempty"`
	Profiles          []string            `json:"profiles,omitempty"`
	AssignedNodeID    string              `json:"assigned_node_id,omitempty"`
	NodeToken         string              `json:"node_token,omitempty"`
	NodePrincipal     string              `json:"node_principal,omitempty"`
	Capabilities      *storedCapabilities `json:"capabilities,omitempty"`
	SuggestedProfiles []string            `json:"suggested_profiles,omitempty"`
	JoinPlanJSON      []byte              `json:"join_plan_json,omitempty"`
	// LifecyclePhase is the typed v2 lifecycle state. When set, it takes precedence
	// over the legacy Status field for admission and eligibility decisions.
	// Migration: empty → derive from Status via normalizeJoinLifecyclePhase.
	LifecyclePhase    JoinLifecyclePhase  `json:"lifecycle_phase,omitempty"`
}

func (jr *joinRequestRecord) statusMessage() string {
	// Prefer typed LifecyclePhase for operator-facing messages.
	phase := effectiveLifecyclePhase(jr)
	switch phase {
	case JoinPhaseRequested:
		return "join request received; awaiting authorization"
	case JoinPhaseAuthorized:
		return "signed JoinPlan issued; node is not yet admitted to the cluster"
	case JoinPhaseBootstrapping:
		return "node is bootstrapping; waiting for node-agent registration"
	case JoinPhaseNodeAgentRegistered:
		return "node-agent registered; admission pending"
	case JoinPhaseAdmissionPending:
		return "node registered; controller is evaluating admission"
	case JoinPhaseAdmitted:
		return "node admitted; desired state is being written"
	case JoinPhaseConverging:
		return "node admitted; converging toward desired state"
	case JoinPhaseActive:
		return "node is active; runtime proof verified"
	case JoinPhaseBlocked:
		if jr.Reason != "" {
			return "blocked: " + jr.Reason
		}
		return "blocked"
	case JoinPhaseRejected:
		if jr.Reason != "" {
			return "rejected: " + jr.Reason
		}
		return "rejected"
	case JoinPhaseQuarantined:
		return "quarantined; requires operator intervention"
	case JoinPhaseRemoving:
		return "node is being removed"
	case JoinPhaseRemoved:
		return "node has been removed"
	case JoinPhaseStaleGhost:
		return "stale record; node-agent never connected"
	}
	// Legacy fallback for records with no lifecycle phase and unrecognized Status.
	return "pending approval"
}

// BootstrapPhase tracks where a node is in the phased bootstrap sequence.
// A new node must join the cluster as a machine (trust), then as an
// infrastructure participant (etcd, xDS, Envoy), and only then as a
// host for workload services.
type BootstrapPhase string

const (
	BootstrapNone           BootstrapPhase = ""                // legacy / bootstrap node (treated as workload_ready)
	BootstrapAdmitted       BootstrapPhase = "admitted"        // Phase 0: trust established, node-agent running
	BootstrapInfraPreparing BootstrapPhase = "infra_preparing" // Phase 1: infra packages installing
	BootstrapEtcdJoining    BootstrapPhase = "etcd_joining"    // Phase 2: etcd join state machine active
	BootstrapEtcdReady      BootstrapPhase = "etcd_ready"      // Phase 3: etcd verified, discovery live
	BootstrapXdsReady       BootstrapPhase = "xds_ready"       // Phase 4: xDS connected to etcd
	BootstrapEnvoyReady     BootstrapPhase = "envoy_ready"     // Phase 5: Envoy healthy
	BootstrapAwarenessReady BootstrapPhase = "awareness_ready"  // Phase 6: awareness bundle installed and verified
	BootstrapWorkloadReady  BootstrapPhase = "workload_ready"   // Phase 7: normal service reconcile
	BootstrapStorageJoining BootstrapPhase = "storage_joining"  // Phase 8: optional storage join
	BootstrapFailed         BootstrapPhase = "bootstrap_failed"
)

// bootstrapPhaseReady returns true if the node is ready for normal
// workload service reconciliation.
func bootstrapPhaseReady(phase BootstrapPhase) bool {
	return phase == BootstrapNone || phase == BootstrapWorkloadReady || phase == BootstrapStorageJoining
}

// bootstrapAwarenessKey is the key used in InstalledBuildIDs to report the
// installed awareness bundle build_id.
const bootstrapAwarenessKey = "awareness_bundle"

// bootstrapInfraReady returns true if the node's infra mesh is up (Envoy is
// active) and control-plane-critical workloads may be dispatched. This is a
// wider gate than bootstrapPhaseReady — it allows deployment of services like
// cluster-controller and workflow before workload_ready so they can unblock
// nodes stuck at envoy_ready (e.g. due to the MinIO topology contract).
func bootstrapInfraReady(phase BootstrapPhase) bool {
	return phase == BootstrapNone ||
		phase == BootstrapWorkloadReady ||
		phase == BootstrapStorageJoining ||
		phase == BootstrapAwarenessReady ||
		phase == BootstrapEnvoyReady
}

// ---------------------------------------------------------------------------
// Day 1 lifecycle phases — full lifecycle tracking on top of BootstrapPhase.
// ---------------------------------------------------------------------------

// Day1Phase tracks the full Day 1 lifecycle of a node from join to ready.
// It provides a single observable field that summarizes where the node is in
// its initialization, combining bootstrap progress, intent resolution,
// infra convergence, and workload convergence into one phase.
type Day1Phase string

const (
	Day1Joined              Day1Phase = "joined"               // Node registered, trust established
	Day1IdentityReady       Day1Phase = "identity_ready"       // Certs, hostname, domain configured
	Day1ClusterConfigSynced Day1Phase = "cluster_config_synced" // etcd joined, cluster config available
	Day1ProfileResolved     Day1Phase = "profile_resolved"     // Profiles resolved to capabilities/components
	Day1InfraPlanned        Day1Phase = "infra_planned"        // Infra install plan generated
	Day1InfraInstalled      Day1Phase = "infra_installed"      // All required infra packages installed
	Day1InfraHealthy        Day1Phase = "infra_healthy"        // All required infra verified healthy
	Day1WorkloadsPlanned    Day1Phase = "workloads_planned"    // Workload install plans generated
	Day1WorkloadsInstalled  Day1Phase = "workloads_installed"  // All desired workloads installed
	Day1Ready               Day1Phase = "ready"                // Node fully converged

	// Degraded/blocking states
	Day1InfraBlocked          Day1Phase = "infra_blocked"           // Infra install blocked (package missing, failed)
	Day1WorkloadBlocked       Day1Phase = "workload_blocked"        // Workload blocked on unhealthy deps
	Day1DependencyMissing     Day1Phase = "dependency_missing"      // Required dep not in catalog/specs
	Day1PackageMetadataInvalid Day1Phase = "package_metadata_invalid" // Spec metadata incomplete/invalid
)

// day1PhaseReady returns true if the node has completed Day 1 initialization.
func day1PhaseReady(phase Day1Phase) bool {
	return phase == Day1Ready
}

// ScyllaJoinPhase tracks where a node is in the ScyllaDB cluster join sequence.
// ScyllaDB uses gossip-based peer discovery — no explicit MemberAdd needed.
// The controller renders scylla.yaml with correct seeds, starts the service,
// and verifies the node joined the gossip ring.
type ScyllaJoinPhase string

const (
	ScyllaJoinNone       ScyllaJoinPhase = ""            // not a scylla node
	ScyllaJoinPrepared   ScyllaJoinPhase = "prepared"    // package installed, unit exists
	ScyllaJoinConfigured ScyllaJoinPhase = "configured"  // scylla.yaml rendered with seeds
	ScyllaJoinStarted    ScyllaJoinPhase = "started"     // scylla-server running
	ScyllaJoinVerified   ScyllaJoinPhase = "verified"    // node in gossip ring
	ScyllaJoinFailed     ScyllaJoinPhase = "failed"      // join failed
)

// MinioJoinPhase tracks where a node is in the MinIO pool join sequence.
// MinIO uses erasure coding sets that are fixed at creation — expansion
// appends new nodes to the ordered pool list and restarts all nodes.
type MinioJoinPhase string

const (
	MinioJoinNone        MinioJoinPhase = ""             // not yet classified
	MinioJoinNonMember   MinioJoinPhase = "non_member"   // confirmed non-pool-member; MinIO correctly held
	MinioJoinPrepared    MinioJoinPhase = "prepared"     // unit exists, ready to join pool
	MinioJoinPoolUpdated MinioJoinPhase = "pool_updated" // IP appended to MinioPoolNodes, config re-rendered
	MinioJoinStarted     MinioJoinPhase = "started"      // globular-minio.service active
	MinioJoinVerified    MinioJoinPhase = "verified"     // healthy (TCP:9000 reachable)
	MinioJoinFailed      MinioJoinPhase = "failed"       // join failed
)

// EtcdJoinPhase tracks where a node is in the etcd cluster join sequence.
type EtcdJoinPhase string

const (
	EtcdJoinNone        EtcdJoinPhase = ""               // not joining / not an etcd node
	EtcdJoinPrepared    EtcdJoinPhase = "prepared"       // package installed, unit exists, ready for MemberAdd
	EtcdJoinMemberAdded EtcdJoinPhase = "member_added"   // MemberAdd called, config rendered, awaiting service start
	EtcdJoinStarted     EtcdJoinPhase = "started"        // etcd service started, awaiting health verification
	EtcdJoinVerified    EtcdJoinPhase = "verified"       // etcd member healthy and participating
	EtcdJoinFailed      EtcdJoinPhase = "failed"         // join failed, rollback performed or needed

	// Rejoin states — set when a node is permanently stuck in etcd_joining
	// (e.g. WAL records its own removal, or ghost member left from prior attempt).
	// No automatic destructive action is taken; operator must run:
	//   globular node repair-etcd --node <hostname> --wipe-local-etcd
	EtcdJoinRejoinRequired   EtcdJoinPhase = "rejoin_required"    // stuck join detected; operator action required
	EtcdJoinRejoinInProgress EtcdJoinPhase = "rejoin_in_progress" // node.etcd.rejoin workflow is running
	EtcdJoinRejoinFailed     EtcdJoinPhase = "rejoin_failed"      // node.etcd.rejoin workflow failed
)

type nodeState struct {
	NodeID                string             `json:"node_id"`
	Identity              storedIdentity     `json:"identity"`
	Profiles              []string           `json:"profiles"`
	LastSeen              time.Time          `json:"last_seen"`
	Status                string             `json:"status"`
	Metadata              map[string]string  `json:"metadata,omitempty"`
	AgentEndpoint         string             `json:"agent_endpoint,omitempty"`
	Units                 []unitStatusRecord `json:"units,omitempty"`
	LastError             string             `json:"last_error,omitempty"`
	ReportedAt            time.Time          `json:"reported_at,omitempty"`
	LastAppliedGeneration uint64             `json:"last_applied_generation,omitempty"`
	AppliedServicesHash   string             `json:"applied_services_hash,omitempty"`
	InstalledVersions     map[string]string  `json:"installed_versions,omitempty"`
	InstalledBuildIDs     map[string]string  `json:"installed_build_ids,omitempty"`
	// Health tracking fields
	FailedHealthChecks   int       `json:"failed_health_checks,omitempty"`
	LastRecoveryAttempt  time.Time `json:"last_recovery_attempt,omitempty"`
	RecoveryAttempts     int       `json:"recovery_attempts,omitempty"`
	MarkedUnhealthySince time.Time `json:"marked_unhealthy_since,omitempty"`
	// etcd join state machine (Phase-based expansion)
	EtcdJoinPhase      EtcdJoinPhase `json:"etcd_join_phase,omitempty"`
	EtcdJoinStartedAt  time.Time     `json:"etcd_join_started_at,omitempty"`
	EtcdJoinError      string        `json:"etcd_join_error,omitempty"`
	EtcdMemberID       uint64        `json:"etcd_member_id,omitempty"`        // for rollback via MemberRemove
	EtcdMissingCycles  int           `json:"etcd_missing_cycles,omitempty"`   // consecutive cycles where member missing + etcd not running
	// MinIO pool join state machine (erasure-coded expansion)
	MinioJoinPhase     MinioJoinPhase `json:"minio_join_phase,omitempty"`
	MinioJoinStartedAt time.Time      `json:"minio_join_started_at,omitempty"`
	MinioJoinError     string         `json:"minio_join_error,omitempty"`
	// ScyllaHostID is the UUID reported by this node's ScyllaDB instance via
	// heartbeat (installed_build_ids["scylla:host_id"]). Used by RemoveNode to
	// call nodetool removenode on a healthy peer when this node is removed.
	// Empty until the node's ScyllaDB has fully started and reported at least one heartbeat.
	ScyllaHostID string `json:"scylla_host_id,omitempty"`
	// ScyllaDB join state machine (gossip-based cluster expansion)
	ScyllaJoinPhase        ScyllaJoinPhase `json:"scylla_join_phase,omitempty"`
	ScyllaJoinStartedAt    time.Time       `json:"scylla_join_started_at,omitempty"`
	ScyllaJoinError        string          `json:"scylla_join_error,omitempty"`
	ScyllaJoinRestarts     int             `json:"scylla_join_restarts,omitempty"`
	// ScyllaWasEverVerified is set once a node reaches ScyllaJoinVerified. It gates
	// the wipe escalation in the restart/wipe pipeline: we must never wipe an
	// existing cluster member that is only experiencing a temporary probe regression
	// (e.g. because a peer was removed). The wipe is only safe for nodes that have
	// never successfully joined the ring.
	ScyllaWasEverVerified  bool            `json:"scylla_was_ever_verified,omitempty"`
	// ScyllaReplaceAddress is set when a node rejoins after being removed without
	// decommissioning (its IP is still DN in the gossip ring). The rendered
	// scylla.yaml will include replace_address_first_boot so ScyllaDB can claim
	// ownership of the dead node's tokens instead of refusing to bootstrap.
	ScyllaReplaceAddress string `json:"scylla_replace_address,omitempty"`
	// JoinLifecyclePhase is the v2 admission lifecycle state for this node.
	// Empty on legacy nodes (treat as eligible — backward compat). When set,
	// IsNodeVerifiedStorageEligible uses it to gate RF and topology participation.
	// The transition sequence: bootstrapping → node_agent_registered →
	// admission_pending → admitted → converging → active.
	JoinLifecyclePhase JoinLifecyclePhase `json:"join_lifecycle_phase,omitempty"`
	// Bootstrap phase state machine (phased node initialization)
	BootstrapPhase     BootstrapPhase `json:"bootstrap_phase,omitempty"`
	BootstrapStartedAt time.Time      `json:"bootstrap_started_at,omitempty"`
	BootstrapError     string         `json:"bootstrap_error,omitempty"`
	BootstrapRunID             string `json:"bootstrap_run_id,omitempty"`
	BootstrapWorkflowActive    bool   `json:"-"` // in-memory only: true while bootstrap workflow engine is driving this node
	// DNS-first naming field (PR2)
	AdvertiseFqdn string `json:"advertise_fqdn,omitempty"`
	// Structured blocked reason (Phase 7)
	BlockedReason  string `json:"blocked_reason,omitempty"`  // e.g. "unknown_profile" | "missing_units" | "apply_failed"
	BlockedDetails string `json:"blocked_details,omitempty"` // human-readable details
	// Day 1 lifecycle tracking
	Day1Phase       Day1Phase  `json:"day1_phase,omitempty"`        // Current Day 1 lifecycle phase
	Day1PhaseReason string     `json:"day1_phase_reason,omitempty"` // Human-readable reason for current phase
	// Day 1 resolved intent (populated during reconcile from profile + catalog resolution).
	ResolvedIntent *NodeIntent `json:"resolved_intent,omitempty"`

	// Infrastructure intents — controller-authorized membership records.
	//
	// Design rule (v2-join Phase F-lite):
	//   profiles  = capability labels ("this node CAN run storage")
	//   intents   = controller authorization ("this node IS authorized to join")
	//   runtime   = observed truth ("this node HAS joined and is healthy")
	//
	// When present, eligibility predicates prefer intents over profile inference.
	// Legacy nodes with nil intents keep existing profile-based behavior.
	EtcdMemberIntent  *EtcdMemberIntent  `json:"etcd_member_intent,omitempty"`
	ScyllaIntent      *ScyllaIntent      `json:"scylla_intent,omitempty"`
	ObjectStoreIntent *ObjectStoreIntent `json:"object_store_intent,omitempty"`

	// Per-file content hashes of the last successfully applied rendered service configs (Phase 4b).
	// Map key is the output file path; value is sha256 hex of the file content.
	// Committed only after the node agent reports apply success (not just on dispatch).
	RenderedConfigHashes map[string]string `json:"rendered_config_hashes,omitempty"`
	// PendingRenderedConfigHashes holds hashes from a dispatched plan that has not yet
	// been confirmed as applied. On success these are promoted to RenderedConfigHashes;
	// on failure they are cleared so the next cycle re-detects the change.
	PendingRenderedConfigHashes map[string]string `json:"pending_rendered_config_hashes,omitempty"`
	// InventoryComplete is true when the node agent last reported a full unit-file inventory.
	// When false, capability gating uses soft mode (warn but don't block).
	InventoryComplete bool `json:"inventory_complete,omitempty"`
	// Capabilities holds the hardware stats last reported by this node's agent.
	Capabilities *storedCapabilities `json:"capabilities,omitempty"`
	// RestartAttempts tracks lightweight restart attempts per service (in-memory only).
	// Keyed by canonical service name. Resets on controller restart.
	RestartAttempts map[string]*restartAttempt `json:"-"`
	// LastAdmissionProof is the most recently stored admission proof evaluation
	// result. Updated on every heartbeat where EvaluateNodeAdmissionProof is
	// called. nil for legacy nodes (empty JoinLifecyclePhase) or nodes that
	// have not been evaluated yet. Persisted so operators can query it.
	LastAdmissionProof *AdmissionProofStatus `json:"last_admission_proof,omitempty"`
	// lastAdmissionReason is the reason string from the last *logged* proof
	// evaluation. In-memory only (json:"-") — used to suppress repeated
	// identical log lines. Resets on controller restart (acceptable).
	lastAdmissionReason string `json:"-"`
}

// restartAttempt tracks lightweight restart attempts for a single service.
// Lives in-memory only — resets on controller restart (acceptable for v1).
type restartAttempt struct {
	Count                  int       `json:"-"`
	LastAt                 time.Time `json:"-"`
	LastError              string    `json:"-"`
	BackoffUntil           time.Time `json:"-"`
	FailureClass           string    `json:"-"` // "process_crash", "startup_timeout", "precondition_failed", "dependency_blocked"
	ConsecutivePrecondFail int       `json:"-"` // consecutive precondition failures
	BlockedReason          string    `json:"-"` // non-empty = service is blocked, skip restarts
	BlockedSince           time.Time `json:"-"`
}

const (
	FailClassProcessCrash      = "process_crash"
	FailClassStartupTimeout    = "startup_timeout"
	FailClassPreconditionFail  = "precondition_failed"
	FailClassDependencyBlocked = "dependency_blocked"

	maxConsecutivePrecondFail = 3
)

// storedCapabilities is the JSON-serializable form of NodeCapabilities.
type storedCapabilities struct {
	CPUCount            uint32 `json:"cpu_count"`
	RAMBytes            uint64 `json:"ram_bytes"`
	DiskBytes           uint64 `json:"disk_bytes"`
	DiskFreeBytes       uint64 `json:"disk_free_bytes"`
	CanApplyPrivileged  bool   `json:"can_apply_privileged,omitempty"`
	PrivilegeReason     string `json:"privilege_reason,omitempty"`
}

type unitStatusRecord struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Details string `json:"details"`
}

type storedIdentity struct {
	Hostname     string   `json:"hostname"`
	Domain       string   `json:"domain"`
	Ips          []string `json:"ips"`
	Os           string   `json:"os"`
	Arch         string   `json:"arch"`
	AgentVersion string   `json:"agent_version"`
}

// PrimaryIP returns the first routable (non-loopback) IP from the node's identity.
// Returns "" if no routable IP is found.
// WARNING: On the VIP holder, this may return the floating VIP (e.g. 10.0.0.100)
// which is not in service cert SANs and MinIO doesn't bind to. Prefer StableIP()
// for operations that need the node's real (non-VIP) address.
func (n *nodeState) PrimaryIP() string {
	for _, raw := range n.Identity.Ips {
		ip := strings.TrimSpace(raw)
		if ip == "" {
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil {
			continue
		}
		if parsed.IsLoopback() {
			continue
		}
		return ip
	}
	return ""
}

// StableIP returns the first routable IP that is NOT the cluster VIP.
// This is the address services actually bind to (MinIO, etcd, ScyllaDB, etc.)
// and the one present in TLS cert SANs. Falls back to PrimaryIP() if no
// VIP is configured or all IPs are the VIP.
func (n *nodeState) StableIP(vip string) string {
	vip = strings.TrimSpace(vip)
	for _, raw := range n.Identity.Ips {
		ip := strings.TrimSpace(raw)
		if ip == "" {
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.IsLoopback() || parsed.IsLinkLocalUnicast() {
			continue
		}
		if vip != "" && ip == vip {
			continue
		}
		return ip
	}
	return n.PrimaryIP() // fallback
}

func newControllerState() *controllerState {
	return &controllerState{
		JoinTokens:   make(map[string]*joinTokenRecord),
		JoinRequests: make(map[string]*joinRequestRecord),
		Nodes:        make(map[string]*nodeState),
		ClusterId:    netutil.DefaultClusterDomain(),
		CreatedAt:    time.Now(),
		// Day-0 Security: Initialize with default internal domain
		ClusterNetworkSpec: &cluster_controllerpb.ClusterNetworkSpec{
			ClusterDomain: netutil.DefaultClusterDomain(),
			Protocol:      "https",
		},
		NetworkingGeneration: 1,
		MinioCredentials:     readOrGenerateMinioCredentials(),
	}
}

// MigrateLegacyStatePathOnce relocates an existing state.json from the
// pre-Project-O `clustercontroller/` (no separator) directory to the
// canonical `cluster-controller/` (hyphenated) directory. Idempotent.
//
// Rules (mirrors Project O.3 spec):
//   - If canonical exists  → keep canonical, leave legacy untouched, log warn
//     when both are present so the operator can resolve manually.
//   - If only legacy exists → create canonical's parent dir with safe perms,
//     rename legacy → canonical. Old empty parent dir is left in place; a
//     later verified cleanup may remove it.
//   - Neither exists       → no-op, caller falls through to the normal "no
//     state yet" path in loadControllerState.
//
// The migration runs at startup before loadControllerState. Failure to
// migrate is logged but NOT fatal — the load step decides whether the
// canonical path is loadable from whatever ended up there.
func MigrateLegacyStatePathOnce(canonical, legacy string) {
	if canonical == "" || legacy == "" || canonical == legacy {
		return
	}
	legacyExists := pathExists(legacy)
	canonicalExists := pathExists(canonical)
	if !legacyExists {
		return // nothing to do
	}
	if canonicalExists {
		log.Printf("state-migration: both %s and %s exist — canonical wins, legacy left in place for operator review", canonical, legacy)
		return
	}
	// Move legacy → canonical. Ensure parent dir exists with safe permissions.
	parent := filepath.Dir(canonical)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		log.Printf("state-migration: WARN create parent %s: %v", parent, err)
		return
	}
	if err := os.Rename(legacy, canonical); err != nil {
		log.Printf("state-migration: WARN rename %s → %s: %v", legacy, canonical, err)
		return
	}
	log.Printf("state-migration: moved %s → %s", legacy, canonical)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func loadControllerState(path string) (*controllerState, error) {
	state := newControllerState()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(b, state); err != nil {
		log.Printf("WARNING: state file %s is corrupted (%v), starting with fresh state", path, err)
		return newControllerState(), nil
	}
	if state.CreatedAt.IsZero() {
		state.CreatedAt = time.Now()
	}
	// state.ClusterId defaults to the domain ONLY when unset. It is NOT coerced to
	// a domain shape. The previous `!isDomainLike(...)` guard force-rewrote any
	// opaque (UUID) cluster id back to the domain on every load — the identity
	// program's "dragon's nostril": with it in place, a minted membership UUID
	// could never survive a restart. See docs/design/cluster-id-minted-uuid-
	// migration.md. Empty-default only; never rewrite an already-set value.
	if state.ClusterId == "" {
		state.ClusterId = netutil.DefaultClusterDomain()
	}
	// Day-0 Security: Ensure internal domain is always set
	if state.ClusterNetworkSpec == nil {
		state.ClusterNetworkSpec = &cluster_controllerpb.ClusterNetworkSpec{
			ClusterDomain: netutil.DefaultClusterDomain(),
			Protocol:      "https",
		}
		state.NetworkingGeneration++
	} else if state.ClusterNetworkSpec.ClusterDomain == "" {
		state.ClusterNetworkSpec.ClusterDomain = netutil.DefaultClusterDomain()
		state.NetworkingGeneration++
	}
	// Migrate existing nodes: empty BootstrapPhase means the node was
	// created before phased bootstrap existed — treat as fully ready.
	for _, node := range state.Nodes {
		if node != nil && node.BootstrapPhase == BootstrapNone {
			node.BootstrapPhase = BootstrapWorkloadReady
		}
	}
	// Migrate: generate MinIO credentials if missing (pre-credential-management clusters).
	if state.MinioCredentials == nil {
		state.MinioCredentials = generateMinioCredentials()
	}
	// Migrate: replace hostnames in MinioPoolNodes with routable IPs.
	// Older versions wrote FQDNs (e.g. "globule-ryzen.globular.internal") instead
	// of bare IPs, which causes resolveMinioEndpointLocked to reject them.
	migratePoolNodeHostnames(state)
	// Phase E-lite migration: if DesiredObjectStoreMembers is nil but nodes carry
	// ObjectStoreIntent.Member=true (set by Phase F-lite), this is a first boot
	// after upgrade. Populate DesiredObjectStoreMembers from intent flags so that
	// existing pool members are not locked out by the v2 gate.
	// Nodes without a routable IP (not yet registered) stay in nil desired — they
	// will be added on their next heartbeat via the reconcile path.
	if state.DesiredObjectStoreMembers == nil {
		migrated := objectStoreDesiredMembersFromIntents(state.Nodes, uint64(state.ObjectStoreGeneration))
		if len(migrated) > 0 {
			state.DesiredObjectStoreMembers = migrated
			log.Printf("objectstore_migration: populated DesiredObjectStoreMembers from %d node intents", len(migrated))
		}
	}
	return state, nil
}


func (s *controllerState) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// etcdStateKey is the etcd key where controller state is persisted.
// This is the authoritative copy — local disk is a backup.
const etcdStateKey = "/globular/clustercontroller/state"

// saveToEtcdPutFunc is the indirection used by saveToEtcd to actually
// perform the etcd Put. Production code does a real cli.Put; tests swap
// this for a counter/capture so they can assert exactly which writes
// reached etcd without standing up a real etcd. The signature returns
// only error so the production path remains simple.
var saveToEtcdPutFunc = func(ctx context.Context, cli *clientv3.Client, key, value string) error {
	_, err := cli.Put(ctx, key, value)
	return err
}

// saveToEtcd persists the controller state to etcd, skipping the Put when
// the serialized state is byte-identical to the last successfully persisted
// version (Phase 36 — content-hash dedup).
//
// Pre-Phase-36 behaviour was: marshal + Put on every call, even when
// nothing changed. Result: /globular/clustercontroller/state at 334 KB
// rewritten ~2.5×/min produced ~95% of all etcd MVCC bloat between
// compaction cycles. See docs/awareness/reports/etcd_bloat_investigation_2026-06-03.md.
//
// Post-Phase-36 behaviour:
//   - First call after process startup always writes (lastPersistedHash is
//     zero, the sentinel for "never persisted").
//   - Subsequent calls serialize + hash; if the hash matches
//     lastPersistedHash AND it's non-zero, the Put is skipped and the
//     in-memory hash stays the same.
//   - On a successful Put, lastPersistedHash is updated to the new value.
//   - On a Put error, lastPersistedHash is NOT updated — so a later
//     retry will see the previous hash and either skip (if state is
//     still the same and was already-persisted previously) or write
//     (if state changed since the last successful persist).
//
// Caller contract: callers must hold the server lock that protects the
// state struct (the only production caller is persistStateLocked under
// srv.lock); concurrent saveToEtcd on the same *controllerState would
// race on the lastPersistedHash field.
func (s *controllerState) saveToEtcd(cli *clientv3.Client) error {
	if cli == nil {
		return nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)

	// Skip when content is byte-identical to the last successful persist.
	// The zero-value check ensures the FIRST call after startup always
	// writes — even if some prior process wrote the same bytes, the
	// current process has no record of that and must take ownership.
	var zero [sha256.Size]byte
	if s.lastPersistedHash != zero && s.lastPersistedHash == hash {
		slog.Debug("state.persist_skipped_unchanged",
			"key", etcdStateKey,
			"size_bytes", len(data),
			"hash", hex.EncodeToString(hash[:8]),
		)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := saveToEtcdPutFunc(ctx, cli, etcdStateKey, string(data)); err != nil {
		// Do NOT update lastPersistedHash on failure — the next attempt
		// must still try, regardless of whether state changed.
		return err
	}
	s.lastPersistedHash = hash
	slog.Debug("state.persist_written",
		"key", etcdStateKey,
		"size_bytes", len(data),
		"hash", hex.EncodeToString(hash[:8]),
	)
	return nil
}

// loadFromEtcd loads the controller state from etcd.
// Returns nil, nil if the key does not exist (fresh cluster).
func loadFromEtcd(cli *clientv3.Client) (*controllerState, error) {
	if cli == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx, etcdStateKey)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	state := newControllerState()
	if err := json.Unmarshal(resp.Kvs[0].Value, state); err != nil {
		return nil, err
	}
	migratePoolNodeHostnames(state)
	return state, nil
}

// migratePoolNodeHostnames replaces any non-IP entries in MinioPoolNodes with
// the routable IP of the matching node. Older code wrote FQDNs instead of IPs,
// which causes resolveMinioEndpointLocked to reject them with an INVARIANT
// VIOLATION and leave the objectstore endpoint permanently unresolved.
func migratePoolNodeHostnames(state *controllerState) {
	if state == nil || len(state.MinioPoolNodes) == 0 {
		return
	}
	// Build a hostname/FQDN → routable IP map from the loaded nodes.
	hostToIP := make(map[string]string, len(state.Nodes))
	for _, n := range state.Nodes {
		if n == nil {
			continue
		}
		ip := nodeRoutableIP(n)
		if ip == "" {
			continue
		}
		if n.Identity.Hostname != "" {
			hostToIP[n.Identity.Hostname] = ip
		}
		if n.AdvertiseFqdn != "" {
			hostToIP[n.AdvertiseFqdn] = ip
		}
	}
	for i, entry := range state.MinioPoolNodes {
		if net.ParseIP(entry) != nil {
			continue // already a valid IP
		}
		// Try exact match (FQDN or short hostname).
		if ip, ok := hostToIP[entry]; ok {
			log.Printf("migratePoolNodeHostnames: minio_pool_nodes[%d]: replacing %q → %q", i, entry, ip)
			state.MinioPoolNodes[i] = ip
			continue
		}
		// Try stripping domain suffix: "globule-ryzen.globular.internal" → "globule-ryzen".
		if dot := strings.Index(entry, "."); dot >= 0 {
			short := entry[:dot]
			if ip, ok := hostToIP[short]; ok {
				log.Printf("migratePoolNodeHostnames: minio_pool_nodes[%d]: replacing FQDN %q (hostname %q) → %q", i, entry, short, ip)
				state.MinioPoolNodes[i] = ip
				continue
			}
		}
		log.Printf("migratePoolNodeHostnames: WARNING: minio_pool_nodes[%d]=%q is not an IP and no matching node found — leaving as-is", i, entry)
	}
}
