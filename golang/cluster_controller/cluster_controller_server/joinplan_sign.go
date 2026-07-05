package main

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/globulario/services/golang/security"
)

// joinPlanCanonical is the subset of JoinPlan fields that are signed.
// It deliberately excludes the Signature field so that the payload is
// stable before signing and can be recomputed during verification.
type joinPlanCanonical struct {
	JoinID               string           `json:"join_id"`
	ClusterID            string           `json:"cluster_id"`
	ClusterUID           string           `json:"cluster_uid,omitempty"` // membership identity — signed
	ControllerGeneration int64            `json:"controller_generation"`
	IssuedAt             int64            `json:"issued_at_unix"`  // Unix seconds for determinism
	ExpiresAt            int64            `json:"expires_at_unix"` // Unix seconds for determinism
	AssignedProfiles     []string         `json:"assigned_profiles"`
	BaseReleaseVersion   string           `json:"base_release_version,omitempty"`
	BaseReleaseBuildID   string           `json:"base_release_build_id,omitempty"`
	EtcdJoinIntent       *EtcdJoinIntent  `json:"etcd_join_intent,omitempty"`
	ExpectedNodeIdentity NodePlanIdentity `json:"expected_node_identity"`
	BootstrapEndpoints   []string         `json:"bootstrap_endpoints,omitempty"`
	CAFingerprint        string           `json:"ca_fingerprint,omitempty"`
	AssignedNodeID       string           `json:"assigned_node_id"`
	NodePrincipal        string           `json:"node_principal,omitempty"`
	SignerKeyID          string           `json:"signer_key_id"`
}

// canonicalJoinPlanBytes returns the deterministic bytes that are signed/verified.
// The returned bytes are the JSON encoding of joinPlanCanonical — all JoinPlan
// fields except Signature.
func canonicalJoinPlanBytes(plan *JoinPlan) ([]byte, error) {
	if plan == nil {
		return nil, errors.New("nil plan")
	}
	c := joinPlanCanonical{
		JoinID:               plan.JoinID,
		ClusterID:            plan.ClusterID,
		ClusterUID:           plan.ClusterUID,
		ControllerGeneration: plan.ControllerGeneration,
		IssuedAt:             plan.IssuedAt.Unix(),
		ExpiresAt:            plan.ExpiresAt.Unix(),
		AssignedProfiles:     plan.AssignedProfiles,
		BaseReleaseVersion:   plan.BaseReleaseVersion,
		BaseReleaseBuildID:   plan.BaseReleaseBuildID,
		EtcdJoinIntent:       plan.EtcdJoinIntent,
		ExpectedNodeIdentity: plan.ExpectedNodeIdentity,
		BootstrapEndpoints:   plan.BootstrapEndpoints,
		CAFingerprint:        plan.CAFingerprint,
		AssignedNodeID:       plan.AssignedNodeID,
		NodePrincipal:        plan.NodePrincipal,
		SignerKeyID:          plan.SignerKeyID,
	}
	return json.Marshal(c)
}

// signJoinPlanWithKey signs plan in-place using the provided Ed25519 private key
// and kid. After this call, plan.SignerKeyID and plan.Signature are set.
// SignerKeyID must be set to kid before calling — it is included in the signed
// payload so the verifier can load the right public key.
func signJoinPlanWithKey(plan *JoinPlan, priv ed25519.PrivateKey, kid string) error {
	if plan == nil {
		return errors.New("nil plan")
	}
	plan.SignerKeyID = kid
	payload, err := canonicalJoinPlanBytes(plan)
	if err != nil {
		return fmt.Errorf("canonical bytes: %w", err)
	}
	plan.Signature = ed25519.Sign(priv, payload)
	return nil
}

// verifyJoinPlanWithKey verifies plan.Signature using the provided Ed25519 public
// key. The canonical bytes are recomputed (excluding the Signature field) and
// verified against the stored signature.
func verifyJoinPlanWithKey(plan *JoinPlan, pub ed25519.PublicKey) error {
	if plan == nil {
		return ErrJoinPlanNoSignature
	}
	if len(plan.Signature) == 0 {
		return ErrJoinPlanNoSignature
	}
	payload, err := canonicalJoinPlanBytes(plan)
	if err != nil {
		return fmt.Errorf("canonical bytes: %w", err)
	}
	if !ed25519.Verify(pub, payload, plan.Signature) {
		return ErrJoinPlanInvalidSignature
	}
	return nil
}

// SignJoinPlan signs plan in-place using the cluster-controller's issuer key
// loaded from the security package keystore. The issuer is "cluster-controller".
// This is the production path; tests use signJoinPlanWithKey directly.
func SignJoinPlan(plan *JoinPlan) error {
	if security.GetIssuerSigningKey == nil {
		return errors.New("SignJoinPlan: GetIssuerSigningKey not configured")
	}
	priv, kid, err := security.GetIssuerSigningKey(joinPlanIssuer)
	if err != nil {
		return fmt.Errorf("load signing key: %w", err)
	}
	return signJoinPlanWithKey(plan, priv, kid)
}

// VerifyJoinPlan verifies plan.Signature by loading the public key identified
// by plan.SignerKeyID from the security package. Installers and the controller
// can call this function to authenticate a JoinPlan received from the gateway.
func VerifyJoinPlan(plan *JoinPlan) error {
	if plan == nil || len(plan.Signature) == 0 {
		return ErrJoinPlanNoSignature
	}
	if security.GetPeerPublicKey == nil {
		return errors.New("VerifyJoinPlan: GetPeerPublicKey not configured")
	}
	pub, err := security.GetPeerPublicKey(joinPlanIssuer, plan.SignerKeyID)
	if err != nil {
		return fmt.Errorf("load public key (kid=%s): %w", plan.SignerKeyID, err)
	}
	return verifyJoinPlanWithKey(plan, pub)
}

// joinPlanIssuer is the issuer name used when loading/storing the signing key.
const joinPlanIssuer = "cluster-controller"
