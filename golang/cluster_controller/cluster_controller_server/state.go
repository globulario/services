package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/netutil"
)

// generateMinioCredentials creates random MinIO root credentials.
func generateMinioCredentials() *minioCredentials {
	user := make([]byte, 10)
	pass := make([]byte, 16)
	rand.Read(user)
	rand.Read(pass)
	return &minioCredentials{
		RootUser:     "globular-" + hex.EncodeToString(user),
		RootPassword: hex.EncodeToString(pass),
	}
}

const defaultClusterStatePath = "/var/lib/globular/clustercontroller/state.json"

type controllerState struct {
	JoinTokens           map[string]*joinTokenRecord             `json:"join_tokens"`
	JoinRequests         map[string]*joinRequestRecord           `json:"join_requests"`
	Nodes                map[string]*nodeState                   `json:"nodes"`
	ClusterId            string                                  `json:"cluster_id"`
	CreatedAt            time.Time                               `json:"created_at"`
	ClusterNetworkSpec   *cluster_controllerpb.ClusterNetworkSpec `json:"cluster_network_spec,omitempty"`
	NetworkingGeneration uint64                                  `json:"networking_generation"`
	// MinIO pool membership — ordered, append-only list of node IPs.
	// New nodes are appended; existing entries never change order.
	// This preserves erasure set boundaries across pool expansion.
	MinioPoolNodes   []string          `json:"minio_pool_nodes,omitempty"`
	MinioCredentials *minioCredentials `json:"minio_credentials,omitempty"`
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
	Status            string              `json:"status"`
	Reason            string              `json:"reason,omitempty"`
	Profiles          []string            `json:"profiles,omitempty"`
	AssignedNodeID    string              `json:"assigned_node_id,omitempty"`
	NodeToken         string              `json:"node_token,omitempty"`
	NodePrincipal     string              `json:"node_principal,omitempty"`
	Capabilities      *storedCapabilities `json:"capabilities,omitempty"`
	SuggestedProfiles []string            `json:"suggested_profiles,omitempty"`
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
	BootstrapWorkloadReady  BootstrapPhase = "workload_ready"  // Phase 6: normal service reconcile
	BootstrapStorageJoining BootstrapPhase = "storage_joining" // Phase 7: optional storage join
	BootstrapFailed         BootstrapPhase = "bootstrap_failed"
)

// bootstrapPhaseReady returns true if the node is ready for normal
// workload service reconciliation.
func bootstrapPhaseReady(phase BootstrapPhase) bool {
	return phase == BootstrapNone || phase == BootstrapWorkloadReady || phase == BootstrapStorageJoining
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
	MinioJoinNone        MinioJoinPhase = ""             // not a minio node
	MinioJoinPrepared    MinioJoinPhase = "prepared"     // unit exists, ready to join pool
	MinioJoinPoolUpdated MinioJoinPhase = "pool_updated" // IP appended to MinioPoolNodes, config re-rendered
	MinioJoinStarted     MinioJoinPhase = "started"      // globular-minio.service active
	MinioJoinVerified    MinioJoinPhase = "verified"     // healthy (TCP:9000 reachable)
	MinioJoinFailed      MinioJoinPhase = "failed"       // join failed
)

// EtcdJoinPhase tracks where a node is in the etcd cluster join sequence.
type EtcdJoinPhase string

const (
	EtcdJoinNone       EtcdJoinPhase = ""            // not joining / not an etcd node
	EtcdJoinPrepared   EtcdJoinPhase = "prepared"    // package installed, unit exists, ready for MemberAdd
	EtcdJoinMemberAdded EtcdJoinPhase = "member_added" // MemberAdd called, config rendered, awaiting service start
	EtcdJoinStarted    EtcdJoinPhase = "started"     // etcd service started, awaiting health verification
	EtcdJoinVerified   EtcdJoinPhase = "verified"    // etcd member healthy and participating
	EtcdJoinFailed     EtcdJoinPhase = "failed"      // join failed, rollback performed or needed
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
	LastPlanSentAt        time.Time          `json:"last_plan_sent_at,omitempty"`
	LastPlanError         string             `json:"last_plan_error,omitempty"`
	LastPlanHash          string             `json:"last_plan_hash,omitempty"`
	LastAppliedGeneration uint64             `json:"last_applied_generation,omitempty"`
	AppliedServicesHash   string             `json:"applied_services_hash,omitempty"`
	InstalledVersions     map[string]string  `json:"installed_versions,omitempty"`
	// Health tracking fields
	FailedHealthChecks   int       `json:"failed_health_checks,omitempty"`
	LastRecoveryAttempt  time.Time `json:"last_recovery_attempt,omitempty"`
	RecoveryAttempts     int       `json:"recovery_attempts,omitempty"`
	MarkedUnhealthySince time.Time `json:"marked_unhealthy_since,omitempty"`
	// etcd join state machine (Phase-based expansion)
	EtcdJoinPhase     EtcdJoinPhase `json:"etcd_join_phase,omitempty"`
	EtcdJoinStartedAt time.Time     `json:"etcd_join_started_at,omitempty"`
	EtcdJoinError     string        `json:"etcd_join_error,omitempty"`
	EtcdMemberID      uint64        `json:"etcd_member_id,omitempty"` // for rollback via MemberRemove
	// MinIO pool join state machine (erasure-coded expansion)
	MinioJoinPhase     MinioJoinPhase `json:"minio_join_phase,omitempty"`
	MinioJoinStartedAt time.Time      `json:"minio_join_started_at,omitempty"`
	MinioJoinError     string         `json:"minio_join_error,omitempty"`
	// ScyllaDB join state machine (gossip-based cluster expansion)
	ScyllaJoinPhase     ScyllaJoinPhase `json:"scylla_join_phase,omitempty"`
	ScyllaJoinStartedAt time.Time       `json:"scylla_join_started_at,omitempty"`
	ScyllaJoinError     string          `json:"scylla_join_error,omitempty"`
	// Bootstrap phase state machine (phased node initialization)
	BootstrapPhase     BootstrapPhase `json:"bootstrap_phase,omitempty"`
	BootstrapStartedAt time.Time      `json:"bootstrap_started_at,omitempty"`
	BootstrapError     string         `json:"bootstrap_error,omitempty"`
	BootstrapRunID     string         `json:"bootstrap_run_id,omitempty"`
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
}

// restartAttempt tracks lightweight restart attempts for a single service.
// Lives in-memory only — resets on controller restart (acceptable for v1).
type restartAttempt struct {
	Count        int       `json:"-"` // not persisted
	LastAt       time.Time `json:"-"`
	LastError    string    `json:"-"`
	BackoffUntil time.Time `json:"-"`
}

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
func (n *nodeState) PrimaryIP() string {
	for _, ip := range n.Identity.Ips {
		if ip != "" && ip != "127.0.0.1" && ip != "::1" {
			return ip
		}
	}
	return ""
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
		MinioCredentials:     generateMinioCredentials(),
	}
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
	// Cluster ID must match what the node agent uses (the domain).
	// Migrate any legacy UUID-based cluster IDs to the domain.
	domain := netutil.DefaultClusterDomain()
	if state.ClusterId == "" || !isDomainLike(state.ClusterId) {
		state.ClusterId = domain
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
	return state, nil
}

// isDomainLike returns true if s looks like a domain (contains a dot),
// as opposed to a bare UUID.
func isDomainLike(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
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
