// @awareness namespace=globular.platform
// @awareness component=platform_controller.join_lifecycle
// @awareness file_role=join_plan_type_definitions
// @awareness risk=medium
package main

import "time"

// JoinPlan is the signed authorization contract issued by the cluster
// controller that permits a specific node to execute cluster-affecting join
// steps. The installer must validate this plan before proceeding.
//
// Design rule: the plan is controller-owned. The gateway is a courier. The
// installer is a validator. A node may not join without a valid, unexpired,
// identity-matching plan signed by the controller.
type JoinPlan struct {
	// JoinID is a unique identifier for this authorization.
	JoinID string `json:"join_id"`
	// ClusterID is the cluster this plan is valid for.
	ClusterID string `json:"cluster_id"`
	// ControllerGeneration is the controller state generation at issuance.
	ControllerGeneration int64 `json:"controller_generation"`
	// IssuedAt is when the plan was signed.
	IssuedAt time.Time `json:"issued_at"`
	// ExpiresAt is the deadline after which the plan is invalid.
	ExpiresAt time.Time `json:"expires_at"`
	// AssignedProfiles are the profiles assigned by the controller.
	// The installer MUST use these and MUST NOT override them.
	AssignedProfiles []string `json:"assigned_profiles"`
	// BaseReleaseVersion is the platform release the node should install.
	BaseReleaseVersion string `json:"base_release_version,omitempty"`
	// BaseReleaseBuildID is the canonical artifact identity.
	BaseReleaseBuildID string `json:"base_release_build_id,omitempty"`
	// EtcdJoinIntent describes the etcd join the controller has authorized.
	// Nil means the controller has not yet authorized an etcd join for this node.
	EtcdJoinIntent *EtcdJoinIntent `json:"etcd_join_intent,omitempty"`
	// ExpectedNodeIdentity is the node identity this plan was issued for.
	// Installer must verify its own identity matches before proceeding.
	ExpectedNodeIdentity NodePlanIdentity `json:"expected_node_identity"`
	// BootstrapEndpoints are the controller/gateway endpoints the node uses.
	BootstrapEndpoints []string `json:"bootstrap_endpoints,omitempty"`
	// CAFingerprint is the SHA-256 fingerprint of the cluster CA.
	CAFingerprint string `json:"ca_fingerprint,omitempty"`
	// AssignedNodeID is the node_id pre-assigned by the controller.
	AssignedNodeID string `json:"assigned_node_id"`
	// NodePrincipal is the JWT principal for the node-agent auth token.
	NodePrincipal string `json:"node_principal,omitempty"`
	// SignerKeyID is the KID of the Ed25519 key that signed this plan.
	// Required: used to select the public key for verification.
	SignerKeyID string `json:"signer_key_id"`
	// Signature is the Ed25519 signature over the canonical plan bytes.
	// Excludes the Signature field itself (signed content = all other fields).
	Signature []byte `json:"signature,omitempty"`
}

// EtcdJoinIntent describes the controller-authorized etcd membership step.
type EtcdJoinIntent struct {
	// JoinType is "new" (first-cluster bootstrap) or "existing" (joining a
	// running cluster). Empty string is invalid.
	JoinType string `json:"join_type"`
	// PeerURLs are the authorized peer advertisement URLs for the joining node.
	PeerURLs []string `json:"peer_urls,omitempty"`
	// ClusterToken is the etcd cluster token (only set for "new" bootstraps).
	ClusterToken string `json:"cluster_token,omitempty"`
	// InitialCluster is the full initial-cluster string (only for "new").
	InitialCluster string `json:"initial_cluster,omitempty"`
	// ExistingMemberURLs are the peer URLs of already-joined members (for "existing").
	ExistingMemberURLs []string `json:"existing_member_urls,omitempty"`
}

// NodePlanIdentity is the minimal stable identity embedded in a JoinPlan.
// The installer matches its own identity against this before accepting the plan.
// Deliberately excludes domain and MAC:
//   - domain is cluster/routing scope, not per-node identity
//   - MAC is used for local node-id/token mechanics, not join membership proof
type NodePlanIdentity struct {
	// Hostname is the stable DNS hostname of the node (required).
	Hostname string `json:"hostname"`
	// IPs are the routable IP addresses at join time.
	IPs []string `json:"ips,omitempty"`
}

// JoinAuthorizationRequest is submitted by the installer/script to the
// controller to obtain a signed JoinPlan. The controller is the sole authority
// for profiles, etcd intent, and assigned node identity.
//
// Callers MUST NOT specify profiles — profile assignment is entirely
// controller-owned. Capabilities may be provided for profile deduction hints.
type JoinAuthorizationRequest struct {
	// JoinToken is the one-time join authorization credential.
	JoinToken string `json:"join_token"`
	// Identity is the self-reported stable identity of the joining node.
	Identity NodePlanIdentity `json:"identity"`
	// Labels are arbitrary metadata the installer can attach.
	Labels map[string]string `json:"labels,omitempty"`
	// CPUCount is the number of logical CPU cores (for profile deduction).
	CPUCount uint32 `json:"cpu_count,omitempty"`
	// RAMBytes is total RAM in bytes (for profile deduction).
	RAMBytes uint64 `json:"ram_bytes,omitempty"`
	// DiskBytes is total disk capacity in bytes (for profile deduction).
	DiskBytes uint64 `json:"disk_bytes,omitempty"`
	// InstallerVersion is the version/build of the installer making this request.
	InstallerVersion string `json:"installer_version,omitempty"`
	// ClusterID is the cluster ID the installer believes it is joining.
	// The controller rejects the request if this does not match the cluster.
	ClusterID string `json:"cluster_id,omitempty"`
	// Nonce is a caller-generated unique request ID for idempotency tracking.
	Nonce string `json:"nonce"`
}

// JoinAuthorizationResponse carries the controller's authorization decision.
// On success, Plan contains the signed JoinPlan.
// The installer must validate Plan.Signature before executing cluster-affecting steps.
type JoinAuthorizationResponse struct {
	// Allowed is true when the controller issues a valid JoinPlan.
	Allowed bool `json:"allowed"`
	// DeniedReason is set when Allowed=false.
	DeniedReason string `json:"denied_reason,omitempty"`
	// JoinID is the unique identifier for this authorization.
	JoinID string `json:"join_id,omitempty"`
	// Plan is the signed authorization plan. Nil when Allowed=false.
	Plan *JoinPlan `json:"plan,omitempty"`
	// ExpiresAt is the plan expiry (mirrors Plan.ExpiresAt for quick polling).
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// ControllerGeneration is the controller state generation at issuance.
	ControllerGeneration int64 `json:"controller_generation,omitempty"`
}

// JoinPlanValidationParams carries the caller's context for ValidateJoinPlan.
type JoinPlanValidationParams struct {
	// ClusterID is the cluster ID the installer expects to join.
	// Empty string skips the cluster check.
	ClusterID string
	// NodeIdentity is the installer's self-reported identity.
	NodeIdentity NodePlanIdentity
	// ControllerGeneration, when non-zero, is the minimum acceptable generation.
	ControllerGeneration int64
	// Now overrides the time used for expiry checks. Zero → time.Now().
	Now time.Time
	// PublicKey, when non-nil (ed25519.PublicKey), is used directly for signature
	// verification instead of loading via security.GetPeerPublicKey.
	// Only set this in tests.
	PublicKey interface{}
}
