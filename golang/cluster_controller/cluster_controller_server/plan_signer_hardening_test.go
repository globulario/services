package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"testing"

	"github.com/globulario/services/golang/plan/planpb"
)

// --- Gap 5: Signing key validation tests ---

func TestValidateSigningKey_Valid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	if err := validateSigningKey(priv, pub, "plan-v1-abc123"); err != nil {
		t.Errorf("valid keypair should pass: %v", err)
	}
}

func TestValidateSigningKey_BadPrivateKeyLength(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	badPriv := ed25519.PrivateKey(make([]byte, 32)) // wrong size
	if err := validateSigningKey(badPriv, pub, "kid"); err == nil {
		t.Error("should reject bad private key length")
	}
}

func TestValidateSigningKey_BadPublicKeyLength(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	badPub := ed25519.PublicKey(make([]byte, 16)) // wrong size
	if err := validateSigningKey(priv, badPub, "kid"); err == nil {
		t.Error("should reject bad public key length")
	}
}

func TestValidateSigningKey_MismatchedKeypair(t *testing.T) {
	pub1, _, _ := ed25519.GenerateKey(rand.Reader)
	_, priv2, _ := ed25519.GenerateKey(rand.Reader)
	if err := validateSigningKey(priv2, pub1, "kid"); err == nil {
		t.Error("should reject mismatched keypair")
	}
}

func TestValidateSigningKey_EmptyKID(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	if err := validateSigningKey(priv, pub, ""); err == nil {
		t.Error("should reject empty KID")
	}
	if err := validateSigningKey(priv, pub, "  "); err == nil {
		t.Error("should reject whitespace-only KID")
	}
}

// --- Gap 1: Unsigned dispatch enforcement tests ---

func TestAllowUnsignedDispatch_Default(t *testing.T) {
	os.Unsetenv("ALLOW_UNSIGNED_PLAN_DISPATCH")
	if allowUnsignedDispatch() {
		t.Error("should default to false (hardened mode)")
	}
}

func TestAllowUnsignedDispatch_False(t *testing.T) {
	os.Setenv("ALLOW_UNSIGNED_PLAN_DISPATCH", "false")
	defer os.Unsetenv("ALLOW_UNSIGNED_PLAN_DISPATCH")
	if allowUnsignedDispatch() {
		t.Error("should be false when env=false")
	}
}

func TestAllowUnsignedDispatch_True(t *testing.T) {
	os.Setenv("ALLOW_UNSIGNED_PLAN_DISPATCH", "true")
	defer os.Unsetenv("ALLOW_UNSIGNED_PLAN_DISPATCH")
	if !allowUnsignedDispatch() {
		t.Error("should be true when env=true")
	}
}

func TestSignOrAbort_HardenedMode_NoSigner(t *testing.T) {
	os.Setenv("ALLOW_UNSIGNED_PLAN_DISPATCH", "false")
	defer os.Unsetenv("ALLOW_UNSIGNED_PLAN_DISPATCH")

	srv := &server{} // no planSignerState
	plan := &planpb.NodePlan{PlanId: "test-plan"}
	err := srv.signOrAbort(plan)
	if err == nil {
		t.Error("should return error in hardened mode when signer not initialized")
	}
}

func TestSignOrAbort_CompatMode_NoSigner(t *testing.T) {
	os.Setenv("ALLOW_UNSIGNED_PLAN_DISPATCH", "true")
	defer os.Unsetenv("ALLOW_UNSIGNED_PLAN_DISPATCH")

	srv := &server{} // no planSignerState
	plan := &planpb.NodePlan{PlanId: "test-plan"}
	err := srv.signOrAbort(plan)
	if err != nil {
		t.Errorf("should return nil in compatibility mode: %v", err)
	}
}

// --- Deterministic signing idempotency ---

func TestSignPlan_DeterministicIdempotency(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "det-kid"},
	}

	makePlan := func() *planpb.NodePlan {
		return &planpb.NodePlan{
			PlanId:        "plan-det",
			NodeId:        "node-1",
			Generation:    7,
			Reason:        "upgrade service-x",
			ExpiresUnixMs: 9999999999999,
		}
	}

	plan1 := makePlan()
	if err := srv.signPlan(plan1); err != nil {
		t.Fatalf("signPlan #1: %v", err)
	}
	plan2 := makePlan()
	if err := srv.signPlan(plan2); err != nil {
		t.Fatalf("signPlan #2: %v", err)
	}

	// Same plan content → same signature
	if string(plan1.Signature.GetSig()) != string(plan2.Signature.GetSig()) {
		t.Error("identical plans must produce identical signatures (deterministic signing)")
	}
	if plan1.Signature.GetKeyId() != plan2.Signature.GetKeyId() {
		t.Error("key_id should be identical")
	}
}

func TestSignPlan_DifferentPlansProduceDifferentSignatures(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "diff-kid"},
	}

	plan1 := &planpb.NodePlan{
		PlanId:        "plan-A",
		NodeId:        "node-1",
		Generation:    1,
		ExpiresUnixMs: 9999999999999,
	}
	plan2 := &planpb.NodePlan{
		PlanId:        "plan-B", // different
		NodeId:        "node-1",
		Generation:    1,
		ExpiresUnixMs: 9999999999999,
	}

	if err := srv.signPlan(plan1); err != nil {
		t.Fatalf("signPlan plan-A: %v", err)
	}
	if err := srv.signPlan(plan2); err != nil {
		t.Fatalf("signPlan plan-B: %v", err)
	}

	if string(plan1.Signature.GetSig()) == string(plan2.Signature.GetSig()) {
		t.Error("different plans must produce different signatures")
	}
}

func TestSignOrAbort_HardenedMode_WithSigner(t *testing.T) {
	os.Setenv("ALLOW_UNSIGNED_PLAN_DISPATCH", "false")
	defer os.Unsetenv("ALLOW_UNSIGNED_PLAN_DISPATCH")

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "test-kid"},
	}
	plan := &planpb.NodePlan{PlanId: "test-plan", ExpiresUnixMs: 9999999999999}
	err := srv.signOrAbort(plan)
	if err != nil {
		t.Errorf("should succeed with valid signer: %v", err)
	}
	if plan.Signature == nil {
		t.Error("plan should be signed")
	}
}
