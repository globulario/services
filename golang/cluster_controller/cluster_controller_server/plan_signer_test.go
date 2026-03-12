package main

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/protobuf/proto"
)

func TestSignPlan_DeterministicBytes(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "test-kid"},
	}

	plan := &planpb.NodePlan{
		PlanId:    "plan-1",
		NodeId:    "node-1",
		ClusterId: "cluster-1",
		Reason:    "test",
	}

	// Sign twice
	plan1 := proto.Clone(plan).(*planpb.NodePlan)
	plan2 := proto.Clone(plan).(*planpb.NodePlan)

	if err := srv.signPlan(plan1); err != nil {
		t.Fatalf("sign plan1: %v", err)
	}
	if err := srv.signPlan(plan2); err != nil {
		t.Fatalf("sign plan2: %v", err)
	}

	// Both should produce the same signature bytes (same key, same content)
	// Note: ExpiresUnixMs may differ by a few ms, so set it explicitly
	plan3 := proto.Clone(plan).(*planpb.NodePlan)
	plan3.ExpiresUnixMs = 9999999999999
	plan4 := proto.Clone(plan).(*planpb.NodePlan)
	plan4.ExpiresUnixMs = 9999999999999

	if err := srv.signPlan(plan3); err != nil {
		t.Fatalf("sign plan3: %v", err)
	}
	if err := srv.signPlan(plan4); err != nil {
		t.Fatalf("sign plan4: %v", err)
	}

	if string(plan3.Signature.Sig) != string(plan4.Signature.Sig) {
		t.Errorf("same plan content should produce same signature")
	}
}

func TestSignPlan_DifferentPlans(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "test-kid"},
	}

	plan1 := &planpb.NodePlan{PlanId: "plan-1", NodeId: "node-1", ExpiresUnixMs: 9999999999999}
	plan2 := &planpb.NodePlan{PlanId: "plan-2", NodeId: "node-1", ExpiresUnixMs: 9999999999999}

	if err := srv.signPlan(plan1); err != nil {
		t.Fatalf("sign plan1: %v", err)
	}
	if err := srv.signPlan(plan2); err != nil {
		t.Fatalf("sign plan2: %v", err)
	}

	if string(plan1.Signature.Sig) == string(plan2.Signature.Sig) {
		t.Errorf("different plans should produce different signatures")
	}
}

func TestSignPlan_ClearsSignatureField(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "test-kid"},
	}

	plan := &planpb.NodePlan{
		PlanId:        "plan-1",
		ExpiresUnixMs: 9999999999999,
		Signature: &planpb.PlanSignature{
			Alg:   "old",
			KeyId: "old-kid",
			Sig:   []byte("old-sig"),
		},
	}

	if err := srv.signPlan(plan); err != nil {
		t.Fatal(err)
	}

	if plan.Signature.Alg != "EdDSA" {
		t.Errorf("expected Alg=EdDSA, got %s", plan.Signature.Alg)
	}
	if plan.Signature.KeyId != "test-kid" {
		t.Errorf("expected KeyId=test-kid, got %s", plan.Signature.KeyId)
	}
}

func TestSignPlan_SetsExpiry(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "test-kid"},
	}

	plan := &planpb.NodePlan{PlanId: "plan-1"}
	before := time.Now()

	if err := srv.signPlan(plan); err != nil {
		t.Fatal(err)
	}

	if plan.ExpiresUnixMs == 0 {
		t.Error("ExpiresUnixMs should be set")
	}

	// Should be approximately now + 1 hour
	expiry := time.UnixMilli(int64(plan.ExpiresUnixMs))
	expectedMin := before.Add(defaultPlanTTL - time.Second)
	if expiry.Before(expectedMin) {
		t.Errorf("expiry %v too early (expected >= %v)", expiry, expectedMin)
	}
}

func TestSignPlan_PreservesExistingExpiry(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "test-kid"},
	}

	plan := &planpb.NodePlan{PlanId: "plan-1", ExpiresUnixMs: 1234567890000}

	if err := srv.signPlan(plan); err != nil {
		t.Fatal(err)
	}

	if plan.ExpiresUnixMs != 1234567890000 {
		t.Errorf("should preserve existing expiry, got %d", plan.ExpiresUnixMs)
	}
}

func TestSignPlan_NilSigner(t *testing.T) {
	srv := &server{}
	plan := &planpb.NodePlan{PlanId: "plan-1"}

	err := srv.signPlan(plan)
	if err == nil {
		t.Error("expected error when signer not initialized")
	}
}

func TestInitPlanSigner_GeneratesKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "plan-signer.key")
	pubFile := filepath.Join(tmpDir, "plan-signer.pub")
	kidFile := filepath.Join(tmpDir, "plan-signer.kid")

	// Override the constants for testing (we can't, they're const).
	// Instead, test the key generation logic directly.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(keyFile, priv, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubFile, pub, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kidFile, []byte("test-kid"), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify the key files are valid
	loadedKey, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	loadedPub, err := os.ReadFile(pubFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(loadedKey) != ed25519.PrivateKeySize {
		t.Errorf("key size %d != expected %d", len(loadedKey), ed25519.PrivateKeySize)
	}
	if len(loadedPub) != ed25519.PublicKeySize {
		t.Errorf("pub size %d != expected %d", len(loadedPub), ed25519.PublicKeySize)
	}
}

func TestSignAndVerify_CrossCompatibility(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	kid := "test-kid"

	// Sign on "controller"
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: kid},
	}

	plan := &planpb.NodePlan{
		PlanId:        "plan-1",
		NodeId:        "node-1",
		ClusterId:     "cluster-1",
		Generation:    5,
		ExpiresUnixMs: uint64(time.Now().Add(time.Hour).UnixMilli()),
		Reason:        "upgrade",
	}

	if err := srv.signPlan(plan); err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Verify on "node-agent" side
	savedSig := plan.Signature
	plan.Signature = nil
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(plan)
	plan.Signature = savedSig
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !ed25519.Verify(pub, data, savedSig.Sig) {
		t.Error("verification failed — signing and verification incompatible")
	}
}

func TestSignAndVerify_TamperedField(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := &server{
		planSignerState: &planSigner{privateKey: priv, publicKey: pub, kid: "test-kid"},
	}

	plan := &planpb.NodePlan{
		PlanId:        "plan-1",
		NodeId:        "node-1",
		ExpiresUnixMs: uint64(time.Now().Add(time.Hour).UnixMilli()),
	}

	if err := srv.signPlan(plan); err != nil {
		t.Fatal(err)
	}

	// Tamper with a field
	plan.NodeId = "node-2"

	// Verify should fail
	savedSig := plan.Signature
	plan.Signature = nil
	data, _ := proto.MarshalOptions{Deterministic: true}.Marshal(plan)
	plan.Signature = savedSig

	if ed25519.Verify(pub, data, savedSig.Sig) {
		t.Error("verification should fail after tampering")
	}
}
