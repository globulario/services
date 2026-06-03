// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.joinplan_validate
// @awareness file_role=controller_side_joinplan_signature_and_freshness_validation_for_node_authorization
// @awareness implements=globular.platform:intent.controller.join_lifecycle_fsm_gates_cluster_decisions
// @awareness implements=globular.platform:intent.join.token.validation
// @awareness enforces=globular.platform:invariant.node.admission.proof_must_match_issued_join_id
// @awareness risk=critical
package main

// joinplan_validate.go — controller-side counterpart to
// node_agent/.../join_plan_gate.go (Phase 12). Validates the
// JoinPlan signature against the controller's join key,
// checks the expiry window, and ensures the join_id matches
// the issued token. Any failure is a HARD STOP.
//
// MUST NOT loosen validation. The four sentinel error
// classes (missing plan, missing signature, bad signature,
// expired plan) are the operator-readable vocabulary;
// collapsing them into a generic "join failed" forces
// operators to guess what went wrong and may mask a security
// event as a transient bug.

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/component_catalog"
)

// Sentinel errors for JoinPlan validation.
var (
	ErrJoinPlanNoSignature      = errors.New("join plan has no signature")
	ErrJoinPlanInvalidSignature = errors.New("join plan signature is invalid")
	ErrJoinPlanExpired          = errors.New("join plan has expired")
	ErrJoinPlanWrongCluster     = errors.New("join plan is for a different cluster")
	ErrJoinPlanWrongIdentity    = errors.New("join plan is for a different node identity")
	ErrJoinPlanNoProfiles       = errors.New("join plan has no assigned profiles")
	ErrJoinPlanUnknownProfiles  = errors.New("join plan has unknown assigned profiles")
	ErrJoinPlanMalformedIntent  = errors.New("join plan has malformed etcd join intent")
	ErrJoinPlanStaleGeneration  = errors.New("join plan controller generation is stale")
)

// ValidateJoinPlan verifies that plan is authentic and applicable to the given
// caller context. It enforces every installer validation rule from Phase A:
//
//  1. Signature is present and valid.
//  2. Plan is not expired.
//  3. ClusterID matches (when params.ClusterID is non-empty).
//  4. Node identity matches (hostname at minimum).
//  5. AssignedProfiles is non-empty (controller must supply profiles).
//  6. EtcdJoinIntent is well-formed when present.
//  7. ControllerGeneration is acceptable (when params.ControllerGeneration > 0).
//
// params.PublicKey, when non-nil (ed25519.PublicKey), is used directly for
// signature verification — this is for tests. In production, pass nil and the
// function loads the key via VerifyJoinPlan.
func ValidateJoinPlan(plan *JoinPlan, params JoinPlanValidationParams) error {
	if plan == nil {
		return errors.New("nil join plan")
	}

	// 1. Signature: must be present and valid.
	if len(plan.Signature) == 0 {
		return ErrJoinPlanNoSignature
	}
	if err := verifySignature(plan, params.PublicKey); err != nil {
		return err
	}

	// 2. Expiry.
	now := params.Now
	if now.IsZero() {
		now = time.Now()
	}
	if now.After(plan.ExpiresAt) {
		return fmt.Errorf("%w: expired at %v", ErrJoinPlanExpired, plan.ExpiresAt.UTC())
	}

	// 3. Cluster identity.
	if params.ClusterID != "" && plan.ClusterID != params.ClusterID {
		return fmt.Errorf("%w: plan cluster=%q caller expects=%q",
			ErrJoinPlanWrongCluster, plan.ClusterID, params.ClusterID)
	}

	// 4. Node identity: hostname is the minimum stable identifier.
	if plan.ExpectedNodeIdentity.Hostname == "" {
		return fmt.Errorf("%w: plan has no expected hostname", ErrJoinPlanWrongIdentity)
	}
	if params.NodeIdentity.Hostname != "" &&
		!strings.EqualFold(plan.ExpectedNodeIdentity.Hostname, params.NodeIdentity.Hostname) {
		return fmt.Errorf("%w: plan issued for %q, node is %q",
			ErrJoinPlanWrongIdentity,
			plan.ExpectedNodeIdentity.Hostname,
			params.NodeIdentity.Hostname)
	}

	// 5. Assigned profiles must come from the controller — not the installer.
	// An empty profile list indicates the gateway assigned nothing (forbidden).
	if len(plan.AssignedProfiles) == 0 {
		return ErrJoinPlanNoProfiles
	}
	if unknown := component_catalog.UnknownProfiles(plan.AssignedProfiles); len(unknown) > 0 {
		return fmt.Errorf("%w: %v (known: %v)",
			ErrJoinPlanUnknownProfiles, unknown, component_catalog.ProfileNames())
	}

	// 6. EtcdJoinIntent structural validity.
	if plan.EtcdJoinIntent != nil {
		if err := validateEtcdJoinIntent(plan.EtcdJoinIntent); err != nil {
			return fmt.Errorf("%w: %v", ErrJoinPlanMalformedIntent, err)
		}
	}

	// 7. Controller generation: stale plans from older controller epochs are
	// rejected when the caller knows the current generation.
	if params.ControllerGeneration > 0 &&
		plan.ControllerGeneration != 0 &&
		plan.ControllerGeneration < params.ControllerGeneration {
		return fmt.Errorf("%w: plan generation %d < expected %d",
			ErrJoinPlanStaleGeneration, plan.ControllerGeneration, params.ControllerGeneration)
	}

	return nil
}

// validateEtcdJoinIntent checks structural validity of an EtcdJoinIntent.
func validateEtcdJoinIntent(intent *EtcdJoinIntent) error {
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

// verifySignature routes to verifyJoinPlanWithKey (test path) or VerifyJoinPlan
// (production path) based on whether params.PublicKey is set.
func verifySignature(plan *JoinPlan, pubKeyIface interface{}) error {
	if pubKeyIface != nil {
		if pub, ok := pubKeyIface.(ed25519.PublicKey); ok {
			return verifyJoinPlanWithKey(plan, pub)
		}
		return errors.New("params.PublicKey must be an ed25519.PublicKey")
	}
	return VerifyJoinPlan(plan)
}
