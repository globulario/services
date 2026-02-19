package clustercontrollerpb

// NOTE: This file provides minimal resource types to enable the new resource
// store and RPC scaffolding. It intentionally avoids full generated code while
// keeping field shapes compatible with future proto generation.

type ObjectMeta struct {
	Name            string            `json:"name,omitempty"`
	ResourceVersion string            `json:"resource_version,omitempty"`
	Generation      int64             `json:"generation,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

type ObjectStatus struct {
	ObservedGeneration int64 `json:"observed_generation,omitempty"`
}

type ClusterNetwork struct {
	Meta   *ObjectMeta         `json:"meta,omitempty"`
	Spec   *ClusterNetworkSpec `json:"spec,omitempty"`
	Status *ObjectStatus       `json:"status,omitempty"`
}

type ServiceDesiredVersionSpec struct {
	ServiceName string `json:"service_name,omitempty"`
	Version     string `json:"version,omitempty"`
}

type ServiceDesiredVersion struct {
	Meta   *ObjectMeta                `json:"meta,omitempty"`
	Spec   *ServiceDesiredVersionSpec `json:"spec,omitempty"`
	Status *ObjectStatus              `json:"status,omitempty"`
}

type NodeSpec struct {
	Labels map[string]string `json:"labels,omitempty"`
	Roles  []string          `json:"roles,omitempty"`
}

type Node struct {
	Meta   *ObjectMeta   `json:"meta,omitempty"`
	Spec   *NodeSpec     `json:"spec,omitempty"`
	Status *ObjectStatus `json:"status,omitempty"`
}

// ── Service Lifecycle v1 ──────────────────────────────────────────────────────

// RolloutStrategy constants for ServiceReleaseSpec.RolloutStrategy.
const (
	RolloutRolling   = "ROLLING"     // One batch at a time; gates on MinReadySeconds
	RolloutAllAtOnce = "ALL_AT_ONCE" // All nodes concurrently up to MaxParallelNodes
)

// ReleasePhase constants for ServiceReleaseStatus.Phase and NodeReleaseStatus.Phase.
const (
	ReleasePhasePending    = "PENDING"     // Created/updated, awaiting resolution
	ReleasePhaseResolved   = "RESOLVED"    // Exact version + artifact digest known
	ReleasePhasePlanned    = "PLANNED"     // NodePlans written to plan store
	ReleasePhaseApplying   = "APPLYING"    // At least one node plan in progress
	ReleasePhaseAvailable  = "AVAILABLE"   // All target nodes at desired version
	ReleasePhaseDegraded   = "DEGRADED"    // Some nodes failed; min replicas still met
	ReleasePhaseFailed     = "FAILED"      // Cannot reach desired state; retries exhausted
	ReleasePhaseRolledBack = "ROLLED_BACK" // Rollback plans succeeded on all target nodes
)

// NodeAssignment is an optional per-node version override within a ServiceRelease.
type NodeAssignment struct {
	NodeID  string            `json:"node_id,omitempty"`
	Version string            `json:"version,omitempty"` // Empty = use release default
	Pins    map[string]string `json:"pins,omitempty"`
}

// ServiceReleaseSpec declares the desired state of a service across the cluster.
// Both PublisherID and ServiceName are required; they form the ArtifactRef identity.
type ServiceReleaseSpec struct {
	PublisherID      string            `json:"publisher_id,omitempty"`
	ServiceName      string            `json:"service_name,omitempty"`
	Version          string            `json:"version,omitempty"` // Exact; empty = resolve via Channel
	Channel          string            `json:"channel,omitempty"` // "stable" | "nightly" | "beta"
	RepositoryID     string            `json:"repository_id,omitempty"`
	Platform         string            `json:"platform,omitempty"`         // e.g. "linux_amd64"
	RolloutStrategy  string            `json:"rollout_strategy,omitempty"` // RolloutRolling | RolloutAllAtOnce
	MaxParallelNodes uint32            `json:"max_parallel_nodes,omitempty"`
	MinReadySeconds  uint32            `json:"min_ready_seconds,omitempty"`
	MaxUnavailable   uint32            `json:"max_unavailable,omitempty"`
	NodeAssignments  []*NodeAssignment `json:"node_assignments,omitempty"`
	Config           map[string]string `json:"config,omitempty"`
	Paused           bool              `json:"paused,omitempty"`
	Replicas         *ReplicaSpec      `json:"replicas,omitempty"`
}

// ReplicaSpec declares min/max replicas for a release.
type ReplicaSpec struct {
	Min int32 `json:"min,omitempty"`
	Max int32 `json:"max,omitempty"`
}

// NodeReleaseStatus tracks per-node progress within a ServiceRelease rollout.
type NodeReleaseStatus struct {
	NodeID           string `json:"node_id,omitempty"`
	PlanID           string `json:"plan_id,omitempty"`
	Phase            string `json:"phase,omitempty"` // ReleasePhase* constants
	InstalledVersion string `json:"installed_version,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
	UpdatedUnixMs    int64  `json:"updated_unix_ms,omitempty"`
}

// ServiceReleaseStatus is the controller-managed status of a ServiceRelease.
type ServiceReleaseStatus struct {
	Phase                  string               `json:"phase,omitempty"`
	ResolvedVersion        string               `json:"resolved_version,omitempty"`
	ResolvedArtifactDigest string               `json:"resolved_artifact_digest,omitempty"` // SHA256 hex
	DesiredHash            string               `json:"desired_hash,omitempty"`             // SHA256 of (publisher+name+version+config)
	Nodes                  []*NodeReleaseStatus `json:"nodes,omitempty"`
	Message                string               `json:"message,omitempty"`
	LastTransitionUnixMs   int64                `json:"last_transition_unix_ms,omitempty"`
	ObservedGeneration     int64                `json:"observed_generation,omitempty"`
}

// ServiceRelease is the top-level desired-state object for service lifecycle.
// The cluster-controller watches ServiceRelease objects and drives reconciliation.
type ServiceRelease struct {
	Meta   *ObjectMeta           `json:"meta,omitempty"`
	Spec   *ServiceReleaseSpec   `json:"spec,omitempty"`
	Status *ServiceReleaseStatus `json:"status,omitempty"`
}

// WatchEvent is a resource change notification sent over the Watch stream.
type WatchEvent struct {
	EventType             string                 `json:"event_type,omitempty"`
	ResourceVersion       string                 `json:"resource_version,omitempty"`
	ClusterNetwork        *ClusterNetwork        `json:"cluster_network,omitempty"`
	ServiceDesiredVersion *ServiceDesiredVersion `json:"service_desired_version,omitempty"`
	ServiceRelease        *ServiceRelease        `json:"service_release,omitempty"`
}
