package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// generateTestKeyPair returns an Ed25519 key pair and a KID for use in tests.
func generateTestKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return pub, priv, "test-kid-1"
}

// signedPlan returns a JoinPlan signed with the given key pair.
func signedPlan(t *testing.T, priv ed25519.PrivateKey, kid string) *JoinPlan {
	t.Helper()
	plan := &JoinPlan{
		JoinID:           "test-join-1",
		ClusterID:        "cluster-abc",
		IssuedAt:         time.Now(),
		ExpiresAt:        time.Now().Add(time.Hour),
		AssignedProfiles: []string{"core", "control-plane", "storage"},
		AssignedNodeID:   "node-abc",
		ExpectedNodeIdentity: NodePlanIdentity{
			Hostname: "node-01",
			IPs:      []string{"10.0.0.1"},
		},
	}
	if err := signJoinPlanWithKey(plan, priv, kid); err != nil {
		t.Fatalf("sign plan: %v", err)
	}
	return plan
}

// newJoinAuthServer creates a server pre-loaded with a valid join token and an
// Ed25519 key pair wired via signJoinPlanWithKey/verifyJoinPlanWithKey (test path).
func newJoinAuthServer(t *testing.T) *server {
	t.Helper()
	state := newControllerState()
	state.JoinTokens["tok-v2"] = &joinTokenRecord{
		Token:     "tok-v2",
		ExpiresAt: time.Now().Add(time.Hour),
		MaxUses:   10,
	}
	state.ClusterId = "cluster-abc"                              // namespace (legacy overloaded field)
	state.ClusterUID = "abcdef01-2345-6789-abcd-ef0123456789"    // minted membership UUID (identity)
	state.ClusterNetworkSpec = &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain: "globular.internal",
	}
	return newTestServer(t, state)
}

// TestJoinPlan_CarriesClusterUID (A5): the issued plan carries the minted
// membership UUID as its identity, distinct from the namespace ClusterID.
func TestJoinPlan_CarriesClusterUID(t *testing.T) {
	srv := newJoinAuthServer(t)
	resp, err := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-01", IPs: []string{"10.0.0.1"}},
		Nonce:     "nonce-carry-uid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed || resp.Plan == nil {
		t.Fatalf("expected an allowed plan, denied=%q", resp.DeniedReason)
	}
	if resp.Plan.ClusterUID != srv.state.ClusterUID {
		t.Errorf("plan.ClusterUID = %q, want the minted membership UUID %q", resp.Plan.ClusterUID, srv.state.ClusterUID)
	}
	if resp.Plan.ClusterUID == resp.Plan.ClusterID {
		t.Errorf("plan must carry a distinct identity (ClusterUID) and namespace (ClusterID)")
	}
}

// TestJoinPlan_ClusterUID_IsSigned (A5): the membership UUID is covered by the
// Ed25519 signature — tampering it must invalidate the plan.
func TestJoinPlan_ClusterUID_IsSigned(t *testing.T) {
	pub, priv, kid := generateTestKeyPair(t)
	plan := &JoinPlan{
		JoinID:               "j-uid",
		ClusterID:            "globular.internal", // namespace
		ClusterUID:           "abcdef01-2345-6789-abcd-ef0123456789",
		IssuedAt:             time.Now(),
		ExpiresAt:            time.Now().Add(time.Hour),
		AssignedProfiles:     []string{"core"},
		AssignedNodeID:       "node-x",
		ExpectedNodeIdentity: NodePlanIdentity{Hostname: "node-01", IPs: []string{"10.0.0.1"}},
	}
	if err := signJoinPlanWithKey(plan, priv, kid); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := verifyJoinPlanWithKey(plan, pub); err != nil {
		t.Fatalf("verify (untampered): %v", err)
	}
	plan.ClusterUID = "00000000-0000-0000-0000-000000000000" // tamper
	if err := verifyJoinPlanWithKey(plan, pub); err == nil {
		t.Error("tampering ClusterUID must break the signature — the membership UUID must be signed")
	}
}

// TestJoinAuthorization_ClusterUIDValidatedWhenPresent (A6, transitional): when
// the installer presents a cluster_uid it must equal the minted UUID; the domain
// is rejected as identity. (Empty is still allowed until A6 requires it.)
func TestJoinAuthorization_ClusterUIDValidatedWhenPresent(t *testing.T) {
	srv := newJoinAuthServer(t)
	for _, badUID := range []string{"wrong-uuid", "globular.internal", "cluster-abc"} {
		resp, err := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
			JoinToken:  "tok-v2",
			Identity:   NodePlanIdentity{Hostname: "node-01", IPs: []string{"10.0.0.1"}},
			Nonce:      "nonce-" + badUID,
			ClusterUID: badUID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Allowed || !strings.Contains(resp.DeniedReason, "cluster_uid mismatch") {
			t.Errorf("cluster_uid %q must be denied as a mismatch (domain/wrong is not identity); got allowed=%v reason=%q",
				badUID, resp.Allowed, resp.DeniedReason)
		}
	}
	// The matching minted UUID passes the identity gate.
	resp, err := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
		JoinToken:  "tok-v2",
		Identity:   NodePlanIdentity{Hostname: "node-02", IPs: []string{"10.0.0.2"}},
		Nonce:      "nonce-good-uid",
		ClusterUID: srv.state.ClusterUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(resp.DeniedReason, "cluster_uid mismatch") {
		t.Errorf("matching cluster_uid must pass the identity gate, got: %s", resp.DeniedReason)
	}
}

// TestJoinAuthorization_FoundingNodeReceivesPlan verifies the happy path:
// a valid token + routable identity → allowed=true, signed plan with profiles.
func TestJoinAuthorization_FoundingNodeReceivesPlan(t *testing.T) {
	srv := newJoinAuthServer(t)
	req := &JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-01", IPs: []string{"10.0.0.1"}},
		Nonce:     "nonce-1",
		ClusterID: "cluster-abc",
	}
	resp, err := srv.requestJoinAuthorizationCore(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Fatalf("expected allowed=true, denied_reason=%q", resp.DeniedReason)
	}
	if resp.Plan == nil {
		t.Fatal("expected plan to be present")
	}
	if len(resp.Plan.AssignedProfiles) == 0 {
		t.Fatal("controller must assign profiles")
	}
	if resp.Plan.AssignedNodeID == "" {
		t.Fatal("controller must assign a node_id")
	}
	if resp.Plan.ClusterID != "cluster-abc" {
		t.Errorf("plan cluster_id = %q, want cluster-abc", resp.Plan.ClusterID)
	}
}

// TestJoinAuthorization_FirstThreeNodesGetFoundingProfiles verifies that
// founding quorum enforcement runs: the first node in a fresh cluster must
// receive core+control-plane+storage regardless of what it requests.
func TestJoinAuthorization_FirstThreeNodesGetFoundingProfiles(t *testing.T) {
	srv := newJoinAuthServer(t)
	req := &JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-founding", IPs: []string{"10.0.0.5"}},
		Nonce:     "nonce-founding",
	}
	resp, err := srv.requestJoinAuthorizationCore(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Fatalf("expected allowed, denied: %q", resp.DeniedReason)
	}
	profileSet := map[string]bool{}
	for _, p := range resp.Plan.AssignedProfiles {
		profileSet[p] = true
	}
	for _, required := range []string{"core", "control-plane", "storage"} {
		if !profileSet[required] {
			t.Errorf("founding node missing profile %q; got %v", required, resp.Plan.AssignedProfiles)
		}
	}
}

// TestJoinAuthorization_SignatureVerifiesWithTestKey verifies that
// signJoinPlanWithKey + verifyJoinPlanWithKey round-trip correctly.
func TestJoinAuthorization_SignatureVerifiesWithTestKey(t *testing.T) {
	pub, priv, kid := generateTestKeyPair(t)
	plan := signedPlan(t, priv, kid)

	if err := verifyJoinPlanWithKey(plan, pub); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}
}

// TestJoinAuthorization_TamperedPlanFailsValidation checks that altering any
// field after signing causes ValidateJoinPlan to reject the plan.
func TestJoinAuthorization_TamperedPlanFailsValidation(t *testing.T) {
	pub, priv, kid := generateTestKeyPair(t)
	plan := signedPlan(t, priv, kid)

	// Tamper: change the cluster_id after signing.
	plan.ClusterID = "evil-cluster"

	err := ValidateJoinPlan(plan, JoinPlanValidationParams{
		PublicKey: pub,
	})
	if err == nil {
		t.Fatal("expected validation to fail for tampered plan")
	}
	if !strings.Contains(err.Error(), "invalid") && err != ErrJoinPlanInvalidSignature {
		t.Errorf("expected invalid-signature error, got: %v", err)
	}
}

// TestJoinAuthorization_ExpiredPlanFailsValidation ensures plans past their
// ExpiresAt are rejected.
func TestJoinAuthorization_ExpiredPlanFailsValidation(t *testing.T) {
	pub, priv, kid := generateTestKeyPair(t)

	plan := &JoinPlan{
		JoinID:           "test-join-expired",
		ClusterID:        "cluster-abc",
		IssuedAt:         time.Now().Add(-3 * time.Hour),
		ExpiresAt:        time.Now().Add(-1 * time.Hour),
		AssignedProfiles: []string{"core"},
		AssignedNodeID:   "node-1",
		ExpectedNodeIdentity: NodePlanIdentity{
			Hostname: "node-01",
		},
	}
	if err := signJoinPlanWithKey(plan, priv, kid); err != nil {
		t.Fatalf("sign plan: %v", err)
	}

	err := ValidateJoinPlan(plan, JoinPlanValidationParams{
		PublicKey: pub,
		Now:       time.Now(),
	})
	if err != ErrJoinPlanExpired && !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got: %v", err)
	}
}

// TestJoinAuthorization_WrongNodeIdentityFailsValidation verifies that an
// installer with a different hostname than the plan expects is rejected.
func TestJoinAuthorization_WrongNodeIdentityFailsValidation(t *testing.T) {
	pub, priv, kid := generateTestKeyPair(t)
	plan := signedPlan(t, priv, kid) // plan issued for "node-01"

	err := ValidateJoinPlan(plan, JoinPlanValidationParams{
		PublicKey:    pub,
		NodeIdentity: NodePlanIdentity{Hostname: "node-99"},
	})
	if err != ErrJoinPlanWrongIdentity && !strings.Contains(err.Error(), "identity") {
		t.Errorf("expected wrong-identity error, got: %v", err)
	}
}

// TestJoinAuthorization_NoPlanWhenGatewayAssignsNoProfiles ensures that a plan
// with an empty profile list is rejected during ValidateJoinPlan. The gateway
// must never invent profiles — if profiles are missing, the installer refuses.
func TestJoinAuthorization_NoPlanWhenGatewayAssignsNoProfiles(t *testing.T) {
	pub, priv, kid := generateTestKeyPair(t)

	plan := &JoinPlan{
		JoinID:           "test-join-noprof",
		ClusterID:        "cluster-abc",
		IssuedAt:         time.Now(),
		ExpiresAt:        time.Now().Add(time.Hour),
		AssignedProfiles: []string{}, // deliberately empty
		AssignedNodeID:   "node-1",
		ExpectedNodeIdentity: NodePlanIdentity{
			Hostname: "node-01",
		},
	}
	if err := signJoinPlanWithKey(plan, priv, kid); err != nil {
		t.Fatalf("sign plan: %v", err)
	}

	err := ValidateJoinPlan(plan, JoinPlanValidationParams{
		PublicKey: pub,
	})
	if err != ErrJoinPlanNoProfiles {
		t.Errorf("expected ErrJoinPlanNoProfiles, got: %v", err)
	}
}

func TestJoinAuthorization_UnknownAssignedProfileRejected(t *testing.T) {
	pub, priv, kid := generateTestKeyPair(t)

	plan := &JoinPlan{
		JoinID:           "test-join-unknown-profile",
		ClusterID:        "cluster-abc",
		IssuedAt:         time.Now(),
		ExpiresAt:        time.Now().Add(time.Hour),
		AssignedProfiles: []string{"core", "unknown-profile"},
		AssignedNodeID:   "node-1",
		ExpectedNodeIdentity: NodePlanIdentity{
			Hostname: "node-01",
		},
	}
	if err := signJoinPlanWithKey(plan, priv, kid); err != nil {
		t.Fatalf("sign plan: %v", err)
	}

	err := ValidateJoinPlan(plan, JoinPlanValidationParams{PublicKey: pub})
	if err == nil || !strings.Contains(err.Error(), "unknown assigned profiles") {
		t.Fatalf("expected unknown-profile validation error, got: %v", err)
	}
}

// TestJoinAuthorization_InstallerRefusesUnsignedPlan verifies that a plan
// with an empty signature is rejected with ErrJoinPlanNoSignature.
func TestJoinAuthorization_InstallerRefusesUnsignedPlan(t *testing.T) {
	plan := &JoinPlan{
		JoinID:           "test-join-unsigned",
		ClusterID:        "cluster-abc",
		IssuedAt:         time.Now(),
		ExpiresAt:        time.Now().Add(time.Hour),
		AssignedProfiles: []string{"core"},
		AssignedNodeID:   "node-1",
		ExpectedNodeIdentity: NodePlanIdentity{
			Hostname: "node-01",
		},
		// Signature deliberately not set
	}

	err := ValidateJoinPlan(plan, JoinPlanValidationParams{})
	if err != ErrJoinPlanNoSignature {
		t.Errorf("expected ErrJoinPlanNoSignature, got: %v", err)
	}
}

// TestJoinAuthorization_MalformedEtcdIntentFailsValidation checks that an
// EtcdJoinIntent with an invalid JoinType is caught during validation.
func TestJoinAuthorization_MalformedEtcdIntentFailsValidation(t *testing.T) {
	pub, priv, kid := generateTestKeyPair(t)

	plan := &JoinPlan{
		JoinID:           "test-join-badintent",
		ClusterID:        "cluster-abc",
		IssuedAt:         time.Now(),
		ExpiresAt:        time.Now().Add(time.Hour),
		AssignedProfiles: []string{"core"},
		AssignedNodeID:   "node-1",
		ExpectedNodeIdentity: NodePlanIdentity{
			Hostname: "node-01",
		},
		EtcdJoinIntent: &EtcdJoinIntent{
			JoinType: "bogus", // invalid
		},
	}
	if err := signJoinPlanWithKey(plan, priv, kid); err != nil {
		t.Fatalf("sign plan: %v", err)
	}

	err := ValidateJoinPlan(plan, JoinPlanValidationParams{
		PublicKey: pub,
	})
	if err != ErrJoinPlanMalformedIntent && !strings.Contains(err.Error(), "malformed") {
		t.Errorf("expected ErrJoinPlanMalformedIntent, got: %v", err)
	}
}

// TestJoinAuthorization_CanonicalBytesExcludeSignature ensures that the
// canonical signing payload is stable — re-encoding the same plan produces
// identical bytes, which is required for deterministic signature verification.
func TestJoinAuthorization_CanonicalBytesExcludeSignature(t *testing.T) {
	plan := &JoinPlan{
		JoinID:           "test-canon",
		ClusterID:        "cluster-abc",
		IssuedAt:         time.Unix(1700000000, 0),
		ExpiresAt:        time.Unix(1700007200, 0),
		AssignedProfiles: []string{"core"},
		AssignedNodeID:   "node-1",
		ExpectedNodeIdentity: NodePlanIdentity{
			Hostname: "node-01",
		},
		SignerKeyID: "kid-1",
		Signature:   []byte("should-not-appear"),
	}

	b1, err := canonicalJoinPlanBytes(plan)
	if err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}

	// The Signature field must not appear in canonical bytes.
	if strings.Contains(string(b1), "should-not-appear") {
		t.Error("canonical bytes include signature — signing is not stable")
	}

	// Re-encoding produces the same bytes.
	b2, _ := canonicalJoinPlanBytes(plan)
	if string(b1) != string(b2) {
		t.Error("canonical bytes are not deterministic")
	}
}

// TestJoinAuthorization_PlanJSONStoredOnRecord verifies that after a
// successful RequestJoinAuthorization call, the join request record holds the
// serialized JoinPlan so GetJoinRequestStatus can return it.
func TestJoinAuthorization_PlanJSONStoredOnRecord(t *testing.T) {
	srv := newJoinAuthServer(t)
	req := &JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-01", IPs: []string{"10.0.0.1"}},
		Nonce:     "nonce-stored",
	}
	resp, err := srv.requestJoinAuthorizationCore(req)
	if err != nil || !resp.Allowed {
		t.Fatalf("expected allowed response, err=%v denied=%q", err, resp.DeniedReason)
	}

	jr := srv.state.JoinRequests[resp.JoinID]
	if jr == nil {
		t.Fatal("join request record not found")
	}
	if len(jr.JoinPlanJSON) == 0 {
		t.Fatal("JoinPlanJSON not stored on record")
	}
	var stored JoinPlan
	if err := json.Unmarshal(jr.JoinPlanJSON, &stored); err != nil {
		t.Fatalf("stored JoinPlanJSON is not valid JSON: %v", err)
	}
	if stored.JoinID != resp.Plan.JoinID {
		t.Errorf("stored plan join_id = %q, want %q", stored.JoinID, resp.Plan.JoinID)
	}
}

// TestJoinAuthorization_ExpiredTokenDenied verifies that a call with an
// expired join token is rejected with a gRPC PermissionDenied error.
func TestJoinAuthorization_ExpiredTokenDenied(t *testing.T) {
	state := newControllerState()
	state.JoinTokens["expired-tok"] = &joinTokenRecord{
		Token:     "expired-tok",
		ExpiresAt: time.Now().Add(-time.Minute), // already expired
		MaxUses:   5,
	}
	srv := newTestServer(t, state)

	req := &JoinAuthorizationRequest{
		JoinToken: "expired-tok",
		Identity:  NodePlanIdentity{Hostname: "node-late", IPs: []string{"10.0.0.2"}},
		Nonce:     "nonce-late",
	}
	_, err := srv.requestJoinAuthorizationCore(req)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got: %v", err)
	}
}

// TestJoinAuthorization_ClusterIDMismatchDenied verifies that a join request
// that claims the wrong cluster_id is denied with Allowed=false (not an error).
func TestJoinAuthorization_ClusterIDMismatchDenied(t *testing.T) {
	srv := newJoinAuthServer(t) // ClusterId = "cluster-abc"

	req := &JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-01", IPs: []string{"10.0.0.1"}},
		Nonce:     "nonce-mismatch",
		ClusterID: "wrong-cluster",
	}
	resp, err := srv.requestJoinAuthorizationCore(req)
	if err != nil {
		t.Fatalf("unexpected RPC error: %v", err)
	}
	if resp.Allowed {
		t.Fatal("expected allowed=false for wrong cluster_id")
	}
	if !strings.Contains(resp.DeniedReason, "mismatch") {
		t.Errorf("expected mismatch reason, got: %q", resp.DeniedReason)
	}
}
