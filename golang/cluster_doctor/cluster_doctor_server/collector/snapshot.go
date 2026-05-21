package collector

import (
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/verifier"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// DesiredServiceTarget is the desired-state record the verifier reconciles
// against per-node runtime proofs. Populated from ServiceRelease and
// InfrastructureRelease in etcd by the collector's
// fetchDesiredServiceTargets step. Phase 9 wire-up.
type DesiredServiceTarget struct {
	Service        string
	PublisherID    string
	DesiredVersion string
	DesiredBuildID string
	DesiredHash    string
	RuntimeNeeded  bool
	RequiredNodes  []string
	// ApplyTime is the LastTransitionUnixMs of the release status when it
	// reached AVAILABLE. Used by verifier to detect old_pid_after_upgrade.
	// Zero disables that sub-check.
	ApplyTime time.Time
}

// DataError records a failed upstream RPC call.
type DataError struct {
	Service string
	RPC     string
	Err     error
}

// Snapshot holds a point-in-time view of cluster state gathered from upstream services.
type Snapshot struct {
	SnapshotID     string
	GeneratedAt    time.Time
	DataSources    []string
	DataIncomplete bool
	DataErrors     []DataError

	Nodes       []*cluster_controllerpb.NodeRecord
	NodeHealths map[string]*cluster_controllerpb.NodeHealth // keyed by NodeId
	Inventories map[string]*node_agentpb.Inventory          // keyed by NodeId

	// Per-node subsystem health, populated from the
	// GetSubsystemHealth RPC. Consumed by the "subsystem.stuck"
	// invariant family in rules/ to surface stuck/failed goroutines.
	// Keyed by NodeId.
	SubsystemHealth map[string]*node_agentpb.GetSubsystemHealthResponse

	// Per-node certificate status, populated from the
	// GetCertificateStatus RPC. Consumed by the "security.certs.*"
	// invariant family in rules/ to surface expiring certs and
	// SAN coverage gaps as doctor findings. Keyed by NodeId.
	CertificateStatus map[string]*node_agentpb.GetCertificateStatusResponse

	// Per-node artifact integrity reports, populated from the
	// VerifyPackageIntegrity RPC. Consumed by the
	// "artifact.*" invariant family in rules/ to surface
	// cache / installed-digest mismatches as doctor findings.
	// Keyed by NodeId. Missing entries mean the collector could
	// not obtain a report (dial failure, unimplemented on that
	// node's running binary, etc).
	IntegrityReports map[string]*IntegrityReport

	// OpsKnowledgeMemoryEntries maps each operational-knowledge seed
	// entry id (as ai-memory currently has it) to the seed_sha256
	// stamped in the row's metadata. Populated by the collector when
	// aiMemoryClient is wired; nil otherwise. Consumed by the
	// "ops_knowledge.seed_integrity" rule to compare ai-memory's
	// stored hashes against what the active awareness bundle declares.
	//
	// nil = collector did not query ai-memory (no client configured,
	// or the call failed). Empty map = ai-memory was queried but
	// returned zero seed entries (Day-1 fresh, autoseed has not run).
	OpsKnowledgeMemoryEntries map[string]string

	// Repository-level findings, populated from the repository service's
	// ListRepositoryFindings RPC. Cluster-scoped (not per-node). Consumed
	// by the "repository.*" invariant family in rules/ to surface broken
	// blob, checksum mismatch, missing required signature, REVOKED /
	// QUARANTINED-but-installable artifacts, and rollback failures.
	//
	// Nil / empty when the repository service is unreachable OR when the
	// rpc returns no findings (the healthy-cluster steady state).
	RepositoryFindings []*RepositoryFindingSnapshot

	// RepositoryOperationalStatus is the live mode of the repository service,
	// populated from GetRepositoryStatus. Nil when the collector has no
	// repository client configured; ReachError non-nil when the RPC failed.
	// Consumed by the "repository.operational_mode" invariant family.
	RepositoryOperationalStatus *RepositoryOperationalStatus

	// RepositoryEndpointMissing is true when the collector could not find
	// a "repository.PackageRepository" registration in etcd and the cluster
	// has at least one node registered (i.e. we are past bootstrap). During
	// pre-bootstrap the flag is never set. Consumed by the
	// "repository.endpoint_missing" invariant in repository_status.go.
	RepositoryEndpointMissing bool

	// RepositoryBuildIDIndex is the set of build_ids the repository can
	// resolve as installable artifacts, populated by the collector from a
	// ListArtifacts RPC against the live repository service. This is the
	// AUTHORITATIVE source for "does the repository have build_id X?" —
	// no other field is a valid substitute (in particular, scanning
	// NodeHealth.InstalledBuildIds is wrong: during Day-1 bootstrap the
	// repository has the build_ids but no node has installed them yet,
	// which would make every desired pin look orphaned).
	//
	// Nil means the collector had no repository client OR the ListArtifacts
	// call failed. Rules that consume this MUST treat nil as "no signal —
	// do not infer anything" and emit no findings. An empty (non-nil) map
	// is the legitimate "repository has zero installable artifacts" state.
	//
	// Consumed by the "repository.desired_build_ids_resolve" rule.
	RepositoryBuildIDIndex map[string]bool

	// RepositoryVersionIndex maps package name → set of installable version
	// strings available in the repository. Built from the same ListArtifacts
	// call that populates RepositoryBuildIDIndex; only PUBLISHED/DEPRECATED
	// artifacts enter the index. Nil when the collector had no repository
	// client or the call failed (rules must treat nil as "no signal").
	//
	// Consumed by the "repository.package_version_authority" rule.
	RepositoryVersionIndex map[string]map[string]bool

	// Workflow convergence telemetry — see WI17/WI18.
	StepOutcomes      []*workflowpb.WorkflowStepOutcome
	WorkflowSummaries []*workflowpb.WorkflowRunSummary
	DriftUnresolved   []*workflowpb.DriftUnresolved
	BlockedRuns       []*workflowpb.WorkflowRun // MC-4: runs paused for operator approval

	// WF-DEFER B3: persistent across-runs defer counters that hit
	// max_defers and were marked abandoned. Each entry needs operator
	// action — automatic re-dispatch is suspended until the row is
	// cleared via WorkflowService.ClearCorrelationDeferState.
	AbandonedDeferCorrelations []*workflowpb.CorrelationDeferStateRecord

	// Prometheus-derived control-plane signals (optional)
	PromMetrics map[string]float64 // small, fixed key set
	PromTS      time.Time          // scrape timestamp

	// ObjectStoreDesired is the authoritative objectstore topology read from
	// etcd (/globular/objectstore/config) during snapshot collection.
	// Nil when the key has not yet been published (pre-pool formation) OR
	// when ObjectStoreDesiredLoadError is non-nil (transient etcd error).
	// Consumed by the "objectstore.*" invariant family.
	ObjectStoreDesired *config.ObjectStoreDesiredState

	// ObjectStoreDesiredLoadError is non-nil when the etcd read for
	// ObjectStoreDesired failed. Rules must distinguish this from a
	// confirmed key-absent case (nil desired + nil error = key not found).
	ObjectStoreDesiredLoadError error

	// ObjectStoreAppliedGeneration is the last topology generation that was
	// successfully applied by the objectstore.minio.apply_topology_generation
	// workflow. Zero means the workflow has never run (standalone only).
	// Compared against ObjectStoreDesired.Generation to detect unapplied topology changes.
	ObjectStoreAppliedGeneration int64

	// CAMetadata is the CA fingerprint descriptor published by the cluster
	// controller to etcd (/globular/pki/ca). Used by "pki.*" invariants to
	// detect CA rotation and per-node cert drift.
	// Nil when the controller has not yet published (pre-bootstrap).
	CAMetadata *config.CAMetadata

	// IngressSpecPresent indicates whether /globular/ingress/v1/spec exists.
	// IngressSpecLoadError is non-nil only when etcd access failed.
	IngressSpecPresent   bool
	IngressSpecLoadError error
	IngressSpecRaw       string

	// IngressNodeStatus stores per-node ingress status payloads from
	// /globular/ingress/v1/status/<node_id>. Values are raw JSON maps so rules
	// can evaluate phase/state without coupling to node-agent internals.
	IngressNodeStatus map[string]map[string]interface{}

	// ScyllaSchemaGuardStatus stores per-keyspace schema-guard status read from
	// /globular/scylla/schema_guard/<keyspace>.
	ScyllaSchemaGuardStatus map[string]map[string]interface{}

	// DNSZoneReloadStatus stores the status payload from
	// /globular/dns/v1/status published by the DNS service.
	DNSZoneReloadStatus map[string]interface{}

	// ReconcileLaneStatus stores controller reconcile lane statuses from
	// /globular/controller/reconcile/lanes/*.
	ReconcileLaneStatus map[string]map[string]interface{}

	// CriticalKeyPresent stores presence checks for critical etcd keys used by
	// control-plane guardians. Value is true when the key exists.
	CriticalKeyPresent map[string]bool

	// CriticalKeyQueryError records a failed etcd Get for a key or prefix in
	// the critical-key registry. A non-nil error means the check could not run —
	// the key's absence from CriticalKeyPresent is NOT a confirmed absence.
	// Rules must check this field before emitting FAIL findings; emit CHECK_ERROR
	// instead so the operator sees "query failed" rather than "key missing".
	CriticalKeyQueryError map[string]error

	// CriticalKeyPolicyGaps lists critical keys (from config.CriticalEtcdKeys
	// and config.CriticalEtcdPrefixes) that have no entry in
	// config.CriticalKeyPolicies. Populated statically at snapshot creation —
	// no etcd query required. A non-empty slice means ownership governance is
	// incomplete for those keys, and any future key addition will be caught by
	// the doctor without a live cluster.
	//
	// Invariant: critical_state.registry_ownership_required
	CriticalKeyPolicyGaps []string

	// NodeDriftAge records how long each node has had a services-hash mismatch
	// (desired ≠ applied). Populated by the collector's driftSince tracker.
	// Missing entries mean the node is currently converged. Used by the
	// cluster.services.drift rule to escalate severity over time.
	NodeDriftAge map[string]time.Duration

	// NodeRenderedGenerations maps node ID → the objectstore generation that
	// the node last successfully rendered to disk.
	// Collected from /globular/nodes/{id}/objectstore/rendered_generation.
	// Missing entries (zero) mean the node has not rendered any generation yet.
	NodeRenderedGenerations map[string]int64

	// NodeRenderedFingerprints maps node ID → the state fingerprint for the
	// last rendered generation. The doctor compares these against the
	// RenderStateFingerprint(desired) to detect nodes that rendered a
	// different topology (wrong mode, wrong pool membership, etc.).
	// Collected from /globular/nodes/{id}/objectstore/rendered_state_fingerprint.
	NodeRenderedFingerprints map[string]string

	// AdmittedDisks is the list of operator-approved disk records read from
	// /globular/objectstore/disk/admitted/**. Consumed by the
	// "objectstore.minio.unapproved_path" and "objectstore.minio.existing_data_guard"
	// invariants. Empty when no disks have been admitted yet.
	AdmittedDisks []*config.AdmittedDisk

	// DiskCandidates is the per-node disk inventory reported by each node agent,
	// keyed by node ID. Consumed by "objectstore.minio.existing_data_guard".
	// Missing entries mean the node agent has not reported candidates yet.
	DiskCandidates map[string][]*config.DiskCandidate

	// AppliedStateFingerprint is the topology fingerprint recorded in etcd at
	// /globular/objectstore/topology/applied_state_fingerprint after the last
	// successful apply_topology_generation workflow run. Empty when no topology
	// has been applied yet.
	AppliedStateFingerprint string

	// AppliedVolumesHash is the volumes hash recorded alongside the applied
	// fingerprint. Used by the "objectstore.minio.splitbrain" invariant to
	// detect drive/path changes that have not been reconciled.
	AppliedVolumesHash string

	// DesiredTopologyTransition is the pending destructive transition record for
	// the current desired generation, read from
	// /globular/objectstore/topology/transition/{generation}.
	// Nil when the current generation is not destructive or no transition record
	// has been written. Consumed by "objectstore.minio.destructive_guard".
	DesiredTopologyTransition *config.TopologyTransition

	// KindMismatches is the set of per-node, per-package kind mismatch records
	// read from /globular/controller/kind_mismatches/**. Each entry represents
	// a package whose desired kind (in the controller's desired state) does not
	// match the artifact kind published in the repository. The drift reconciler
	// blocks dispatch for these packages and writes a fresh record on each pass.
	// Records older than kindMismatchStaleness are considered resolved and
	// ignored by the "package.kind_mismatch" rule.
	KindMismatches []KindMismatchRecord

	// LeaderPendingUpdate is the optional record written to
	// /globular/controller/leader_pending_update when the controller leader
	// cannot resign because no follower has reached the target build. Nil when
	// no record exists (condition is resolved or has never occurred).
	LeaderPendingUpdate *LeaderPendingUpdateRecord

	// NodePackageKinds is the authoritative per-node package-kind map read from
	// etcd (/globular/nodes/{nodeID}/packages/{KIND}/{name}).
	// Outer key: nodeID. Inner key: canonical package name. Value: uppercase
	// kind string ("SERVICE", "INFRASTRUCTURE", "COMMAND", "APPLICATION").
	// Rules use this to avoid hardcoding package-classification lists — any
	// package with kind=="COMMAND" has no systemd unit and must be skipped by
	// runtime-convergence checks. Missing inner entry means kind is unknown;
	// callers fall back to static inference.
	NodePackageKinds map[string]map[string]string

	// ActiveLocalOverrides is the set of active local override records read from
	// /globular/releases/local_overrides/<name> in etcd. Keyed by package name.
	// Nil when the etcd read failed; empty map when no overrides are active.
	// Consumed by the "package.local_override_stale" invariant family.
	ActiveLocalOverrides map[string]*cluster_controllerpb.LocalOverride

	// ── Phase 9 (Diagnostic Honesty Refactor) — verifier evidence ─────────
	//
	// RuntimeProofs are independent runtime evidence reports collected from
	// each node via GetServiceRuntimeProof (Phase 2 RPC). One slice per
	// node, one entry per installed SERVICE/INFRASTRUCTURE/APPLICATION
	// package. Nil = collector had no client or every node returned an
	// error; empty map slice = node was reachable but had no services.
	// Consumed by the "diagnostic.runtime_verification" rule.
	RuntimeProofs map[string][]*node_agentpb.ServiceRuntimeProof

	// DesiredServiceTargets is the desired-state set the verifier reconciles
	// against. Keyed by canonical service name. Populated from
	// ServiceRelease + InfrastructureRelease records in etcd.
	DesiredServiceTargets map[string]*DesiredServiceTarget

	// VerifierResult is the cluster-wide roll-up produced by running
	// verifier.VerifyTarget for every (service, node) target found in
	// DesiredServiceTargets and AggregateResult across the verdicts.
	// Nil = verifier did not run (e.g. no desired state available);
	// non-nil = sweep completed. Per-(node, service) results are also
	// persisted to etcd at /globular/verification/runtime/<node>/<service>
	// for cross-process consumption.
	VerifierResult *verifier.Result

	mu sync.Mutex
}

// KindMismatchRecord mirrors the JSON written by the controller to
// /globular/controller/kind_mismatches/{nodeID}/{pkgName}.
type KindMismatchRecord struct {
	NodeID         string `json:"node_id"`
	PkgName        string `json:"pkg_name"`
	DesiredKind    string `json:"desired_kind"`
	RepoKind       string `json:"repo_kind"`
	DetectedAtUnix int64  `json:"detected_at_unix"`
}

// LeaderPendingUpdateRecord mirrors the JSON written to
// /globular/controller/leader_pending_update when the controller leader is
// waiting for a safe successor before it can resign and self-update.
type LeaderPendingUpdateRecord struct {
	LeaderNodeID   string            `json:"leader_node_id"`
	CurrentVersion string            `json:"current_version"`
	TargetVersion  string            `json:"target_version"`
	FollowersTotal int               `json:"followers_total"`
	BlockedReasons map[string]string `json:"blocked_reasons"`
	StuckSinceUnix int64             `json:"stuck_since_unix"`
	DetectedAtUnix int64             `json:"detected_at_unix"`
}

// IntegrityReport is the internal representation of the JSON report returned
// by node_agent's VerifyPackageIntegrity RPC. It mirrors the schema produced
// by the `package.verify_integrity` action in node_agent/internal/actions.
//
// Unmarshalled from report_json verbatim — keep field tags in sync with the
// action's integrityReport type.
type IntegrityReport struct {
	NodeID     string             `json:"node_id"`
	Checked    int                `json:"checked"`
	Findings   []IntegrityFinding `json:"findings"`
	Errors     []string           `json:"errors,omitempty"`
	Invariants map[string]int     `json:"invariants"`
}

// IntegrityFinding is a single artifact-integrity violation.
type IntegrityFinding struct {
	Invariant string            `json:"invariant"`
	Severity  string            `json:"severity"`
	Package   string            `json:"package"`
	Kind      string            `json:"kind"`
	Summary   string            `json:"summary"`
	Evidence  map[string]string `json:"evidence,omitempty"`
}

func newSnapshot(id string) *Snapshot {
	return &Snapshot{
		SnapshotID:               id,
		GeneratedAt:              time.Now(),
		NodeHealths:              make(map[string]*cluster_controllerpb.NodeHealth),
		Inventories:              make(map[string]*node_agentpb.Inventory),
		SubsystemHealth:          make(map[string]*node_agentpb.GetSubsystemHealthResponse),
		CertificateStatus:        make(map[string]*node_agentpb.GetCertificateStatusResponse),
		IntegrityReports:         make(map[string]*IntegrityReport),
		NodeRenderedGenerations:  make(map[string]int64),
		NodeRenderedFingerprints: make(map[string]string),
		DiskCandidates:           make(map[string][]*config.DiskCandidate),
		IngressNodeStatus:        make(map[string]map[string]interface{}),
		ScyllaSchemaGuardStatus:  make(map[string]map[string]interface{}),
		DNSZoneReloadStatus:      make(map[string]interface{}),
		ReconcileLaneStatus:      make(map[string]map[string]interface{}),
		CriticalKeyPresent:       make(map[string]bool),
		CriticalKeyQueryError:    make(map[string]error),
		NodePackageKinds:         make(map[string]map[string]string),
		ActiveLocalOverrides:     make(map[string]*cluster_controllerpb.LocalOverride),
		RuntimeProofs:            make(map[string][]*node_agentpb.ServiceRuntimeProof),
		DesiredServiceTargets:    make(map[string]*DesiredServiceTarget),
	}
}

func (s *Snapshot) addSource(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DataSources = append(s.DataSources, name)
}

func (s *Snapshot) addError(service, rpc string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DataErrors = append(s.DataErrors, DataError{Service: service, RPC: rpc, Err: err})
	s.DataIncomplete = true
}

// ─── SnapshotCache ────────────────────────────────────────────────────────────

// SnapshotCache caches the most recent Snapshot for a configurable TTL.
// Concurrent callers during a fetch share a single in-flight request (singleflight).
type SnapshotCache struct {
	mu        sync.Mutex
	snapshot  *Snapshot
	fetchedAt time.Time
	ttl       time.Duration

	// singleflight fields
	inflight bool
	waiters  []chan *Snapshot
}

func NewSnapshotCache(ttl time.Duration) *SnapshotCache {
	return &SnapshotCache{ttl: ttl}
}

// get returns the cached snapshot if still fresh, along with a done channel that the
// caller must close after a fresh fetch (nil chan means cache hit — no fetch needed).
func (c *SnapshotCache) get() (*Snapshot, chan *Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.snapshot != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.snapshot, nil
	}

	if c.inflight {
		ch := make(chan *Snapshot, 1)
		c.waiters = append(c.waiters, ch)
		return nil, ch
	}

	c.inflight = true
	return nil, nil
}

// invalidate drops the cached snapshot so the next get() forces a
// fresh fetch. Used to implement FreshnessMode.FRESHNESS_FRESH — the
// caller asks for authoritative state, so we throw the cache away
// before the collector runs its fetch cycle.
func (c *SnapshotCache) invalidate() {
	c.mu.Lock()
	c.snapshot = nil
	c.fetchedAt = time.Time{}
	c.mu.Unlock()
}

// ttlFor returns the TTL the cache was configured with, so render
// layers can expose it to callers without reaching into unexported
// fields.
func (c *SnapshotCache) ttlFor() time.Duration { return c.ttl }

// set stores a freshly fetched snapshot and notifies waiters.
func (c *SnapshotCache) set(snap *Snapshot) {
	c.mu.Lock()
	waiters := c.waiters
	c.snapshot = snap
	c.fetchedAt = time.Now()
	c.inflight = false
	c.waiters = nil
	c.mu.Unlock()

	for _, ch := range waiters {
		ch <- snap
	}
}
