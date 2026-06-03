// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.join_plan_gate
// @awareness file_role=node_local_signature_validation_gate_for_controller_issued_joinplan
// @awareness implements=globular.platform:intent.controller.join_lifecycle_fsm_gates_cluster_decisions
// @awareness implements=globular.platform:intent.join.token.validation
// @awareness risk=critical
package main

// join_plan_gate.go — every cluster-affecting action on a joining
// node MUST be preceded by JoinPlan signature validation here.
// The plan is signed by the controller's join key; any of:
//   - missing plan
//   - missing signature
//   - bad signature
//   - expired plan
// is a HARD STOP. Proceeding past any of those errors would let
// an attacker (or a misconfigured installer) join a node without
// controller approval and corrupt the cluster's admission FSM.
//
// The sentinel errors here are the operator vocabulary —
// surfacing a generic "join failed" instead of
// ErrNodePlanInvalidSig forces operators to guess what went
// wrong; keep them specific.

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/component_catalog"
	"github.com/globulario/services/golang/security"
)

// Sentinel errors for node-agent JoinPlan gate validation.
var (
	ErrNodePlanMissing         = errors.New("join blocked: signed JoinPlan missing")
	ErrNodePlanNoSignature     = errors.New("join blocked: JoinPlan has no signature")
	ErrNodePlanInvalidSig      = errors.New("join blocked: JoinPlan signature is invalid")
	ErrNodePlanExpired         = errors.New("join blocked: JoinPlan expired")
	ErrNodePlanWrongCluster    = errors.New("join blocked: JoinPlan cluster mismatch")
	ErrNodePlanWrongIdentity   = errors.New("join blocked: JoinPlan node identity mismatch")
	ErrNodePlanNoProfiles      = errors.New("join blocked: JoinPlan has no assigned profiles")
	ErrNodePlanUnknownProfiles = errors.New("join blocked: JoinPlan has unknown assigned profiles")
	ErrNodePlanMalformed       = errors.New("join blocked: JoinPlan is malformed")
	ErrNodePlanMalformedIntent = errors.New("join blocked: malformed etcd join intent")
	ErrNodePlanStaleGeneration = errors.New("join blocked: JoinPlan generation stale")
)

// nodeJoinPlan is the node-agent's local representation of a controller-issued
// JoinPlan. It is JSON-compatible with the controller's JoinPlan struct.
//
// Do not import the cluster_controller_server package — it is package main
// and cannot be imported. The JSON tags must exactly match the controller's.
type nodeJoinPlan struct {
	JoinID               string          `json:"join_id"`
	ClusterID            string          `json:"cluster_id"`
	ControllerGeneration int64           `json:"controller_generation"`
	IssuedAt             time.Time       `json:"issued_at"`
	ExpiresAt            time.Time       `json:"expires_at"`
	AssignedProfiles     []string        `json:"assigned_profiles"`
	AssignedNodeID       string          `json:"assigned_node_id"`
	ExpectedNodeIdentity nodeJoinIdent   `json:"expected_node_identity"`
	EtcdJoinIntent       *nodeEtcdIntent `json:"etcd_join_intent,omitempty"`
	SignerKeyID          string          `json:"signer_key_id"`
	Signature            []byte          `json:"signature,omitempty"`
}

type nodeJoinIdent struct {
	Hostname string   `json:"hostname"`
	IPs      []string `json:"ips,omitempty"`
}

type nodeEtcdIntent struct {
	JoinType           string   `json:"join_type"`
	PeerURLs           []string `json:"peer_urls,omitempty"`
	ClusterToken       string   `json:"cluster_token,omitempty"`
	InitialCluster     string   `json:"initial_cluster,omitempty"`
	ExistingMemberURLs []string `json:"existing_member_urls,omitempty"`
}

// NodeJoinPlanParams carries the caller's context for validateNodeJoinPlan.
type NodeJoinPlanParams struct {
	// ClusterID is the cluster this node expects to join. Empty skips the check.
	ClusterID string
	// NodeHostname is the node's stable hostname. Empty skips the identity check.
	NodeHostname string
	// MinControllerGeneration, when non-zero, is the minimum acceptable generation.
	MinControllerGeneration int64
	// Now overrides the time used for expiry checks. Zero → time.Now().
	Now time.Time
	// PublicKey, when non-nil (ed25519.PublicKey), is used for signature
	// verification directly instead of loading from keystore. Tests only.
	PublicKey interface{}
	// SkipSignatureVerification allows the gate to pass without verifying the
	// Ed25519 signature (e.g., during tests that don't have key material).
	// Never set to true in production.
	SkipSignatureVerification bool
}

// validateNodeJoinPlan verifies a raw JSON JoinPlan against the caller's context.
// All 7 Phase A validation rules are enforced:
//  1. Signature present and valid.
//  2. Plan not expired.
//  3. ClusterID matches (when params.ClusterID non-empty).
//  4. Node identity matches (when params.NodeHostname non-empty).
//  5. AssignedProfiles non-empty.
//  6. EtcdJoinIntent structurally valid when present.
//  7. ControllerGeneration acceptable (when params.MinControllerGeneration > 0).
func validateNodeJoinPlan(planJSON []byte, params NodeJoinPlanParams) (*nodeJoinPlan, error) {
	if len(planJSON) == 0 {
		return nil, ErrNodePlanMissing
	}

	var plan nodeJoinPlan
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNodePlanMalformed, err)
	}

	// 1. Signature.
	if len(plan.Signature) == 0 {
		return nil, ErrNodePlanNoSignature
	}
	if !params.SkipSignatureVerification {
		if err := verifyNodeJoinPlanSig(&plan, params.PublicKey); err != nil {
			return nil, err
		}
	}

	// 2. Expiry.
	now := params.Now
	if now.IsZero() {
		now = time.Now()
	}
	if now.After(plan.ExpiresAt) {
		return nil, fmt.Errorf("%w: expired at %v", ErrNodePlanExpired, plan.ExpiresAt.UTC())
	}

	// 3. Cluster identity.
	if params.ClusterID != "" && plan.ClusterID != params.ClusterID {
		return nil, fmt.Errorf("%w: plan=%q local=%q",
			ErrNodePlanWrongCluster, plan.ClusterID, params.ClusterID)
	}

	// 4. Node identity: hostname is the minimum stable identifier.
	if strings.TrimSpace(plan.ExpectedNodeIdentity.Hostname) == "" {
		return nil, fmt.Errorf("%w: plan has no expected hostname", ErrNodePlanWrongIdentity)
	}
	if params.NodeHostname != "" &&
		!strings.EqualFold(plan.ExpectedNodeIdentity.Hostname, params.NodeHostname) {
		return nil, fmt.Errorf("%w: plan issued for %q, node is %q",
			ErrNodePlanWrongIdentity,
			plan.ExpectedNodeIdentity.Hostname,
			params.NodeHostname)
	}

	// 5. Profiles must come from the controller — not the gateway.
	if len(plan.AssignedProfiles) == 0 {
		return nil, ErrNodePlanNoProfiles
	}
	if unknown := component_catalog.UnknownProfiles(plan.AssignedProfiles); len(unknown) > 0 {
		return nil, fmt.Errorf("%w: %v (known: %v)",
			ErrNodePlanUnknownProfiles, unknown, component_catalog.ProfileNames())
	}

	// 6. EtcdJoinIntent structural validity.
	if plan.EtcdJoinIntent != nil {
		if err := validateNodeEtcdIntent(plan.EtcdJoinIntent); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNodePlanMalformedIntent, err)
		}
	}

	// 7. Controller generation: stale plans are rejected.
	if params.MinControllerGeneration > 0 &&
		plan.ControllerGeneration != 0 &&
		plan.ControllerGeneration < params.MinControllerGeneration {
		return nil, fmt.Errorf("%w: plan generation %d < expected %d",
			ErrNodePlanStaleGeneration,
			plan.ControllerGeneration,
			params.MinControllerGeneration)
	}

	return &plan, nil
}

// validateNodeEtcdIntent checks structural validity of an EtcdJoinIntent.
func validateNodeEtcdIntent(intent *nodeEtcdIntent) error {
	if intent == nil {
		return errors.New("nil etcd join intent")
	}
	switch intent.JoinType {
	case "new":
		if intent.ClusterToken == "" {
			return errors.New("join_type=new requires cluster_token")
		}
		if intent.InitialCluster == "" {
			return errors.New("join_type=new requires initial_cluster")
		}
	case "existing":
		if len(intent.ExistingMemberURLs) == 0 {
			return errors.New("join_type=existing requires existing_member_urls")
		}
	default:
		return fmt.Errorf("join_type must be 'new' or 'existing', got %q", intent.JoinType)
	}
	return nil
}

// nodeJoinPlanCanonical is the subset of nodeJoinPlan that is signed.
// Field names and types must exactly match the controller's joinPlanCanonical.
type nodeJoinPlanCanonical struct {
	JoinID               string          `json:"join_id"`
	ClusterID            string          `json:"cluster_id"`
	ControllerGeneration int64           `json:"controller_generation"`
	IssuedAt             int64           `json:"issued_at_unix"`
	ExpiresAt            int64           `json:"expires_at_unix"`
	AssignedProfiles     []string        `json:"assigned_profiles"`
	BaseReleaseVersion   string          `json:"base_release_version,omitempty"`
	BaseReleaseBuildID   string          `json:"base_release_build_id,omitempty"`
	EtcdJoinIntent       *nodeEtcdIntent `json:"etcd_join_intent,omitempty"`
	ExpectedNodeIdentity nodeJoinIdent   `json:"expected_node_identity"`
	BootstrapEndpoints   []string        `json:"bootstrap_endpoints,omitempty"`
	CAFingerprint        string          `json:"ca_fingerprint,omitempty"`
	AssignedNodeID       string          `json:"assigned_node_id"`
	NodePrincipal        string          `json:"node_principal,omitempty"`
	SignerKeyID          string          `json:"signer_key_id"`
}

func canonicalNodeJoinPlanBytes(plan *nodeJoinPlan) ([]byte, error) {
	if plan == nil {
		return nil, errors.New("nil plan")
	}
	c := nodeJoinPlanCanonical{
		JoinID:               plan.JoinID,
		ClusterID:            plan.ClusterID,
		ControllerGeneration: plan.ControllerGeneration,
		IssuedAt:             plan.IssuedAt.Unix(),
		ExpiresAt:            plan.ExpiresAt.Unix(),
		AssignedProfiles:     plan.AssignedProfiles,
		EtcdJoinIntent:       plan.EtcdJoinIntent,
		ExpectedNodeIdentity: plan.ExpectedNodeIdentity,
		AssignedNodeID:       plan.AssignedNodeID,
		SignerKeyID:          plan.SignerKeyID,
	}
	return json.Marshal(c)
}

func verifyNodeJoinPlanWithKey(plan *nodeJoinPlan, pub ed25519.PublicKey) error {
	if len(plan.Signature) == 0 {
		return ErrNodePlanNoSignature
	}
	payload, err := canonicalNodeJoinPlanBytes(plan)
	if err != nil {
		return fmt.Errorf("canonical bytes: %w", err)
	}
	if !ed25519.Verify(pub, payload, plan.Signature) {
		return ErrNodePlanInvalidSig
	}
	return nil
}

func verifyNodeJoinPlanSig(plan *nodeJoinPlan, pubKeyIface interface{}) error {
	if pubKeyIface != nil {
		pub, ok := pubKeyIface.(ed25519.PublicKey)
		if !ok {
			return errors.New("params.PublicKey must be ed25519.PublicKey")
		}
		return verifyNodeJoinPlanWithKey(plan, pub)
	}
	// Production path: load public key via security keystore.
	if security.GetPeerPublicKey == nil {
		return errors.New("join plan verification: key lookup not configured")
	}
	pub, err := security.GetPeerPublicKey("cluster-controller", plan.SignerKeyID)
	if err != nil {
		return fmt.Errorf("load verification key (kid=%s): %w", plan.SignerKeyID, err)
	}
	return verifyNodeJoinPlanWithKey(plan, pub)
}
