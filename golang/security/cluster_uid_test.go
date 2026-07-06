package security

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestClaims_ClusterUID_CoexistsWithClusterID: the additive membership-UUID claim
// round-trips and sits alongside the legacy domain cluster_id — dual-claim.
func TestClaims_ClusterUID_CoexistsWithClusterID(t *testing.T) {
	in := &Claims{ClusterID: "globular.internal", ClusterUID: "9f1c2d3e-4a5b-6c7d-8e9f-0a1b2c3d4e5f"}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"cluster_id":"globular.internal"`) {
		t.Errorf("legacy cluster_id claim missing: %s", b)
	}
	if !strings.Contains(string(b), `"cluster_uid":"9f1c2d3e-4a5b-6c7d-8e9f-0a1b2c3d4e5f"`) {
		t.Errorf("additive cluster_uid claim missing: %s", b)
	}
	var out Claims
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.ClusterID != in.ClusterID || out.ClusterUID != in.ClusterUID {
		t.Errorf("round-trip mismatch: got id=%q uid=%q", out.ClusterID, out.ClusterUID)
	}
}

// TestClaims_ClusterUID_OmittedWhenEmpty: pre-mint clusters issue tokens with no
// cluster_uid (omitempty) — the additive claim never breaks issuance.
func TestClaims_ClusterUID_OmittedWhenEmpty(t *testing.T) {
	b, err := json.Marshal(&Claims{ClusterID: "globular.internal"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "cluster_uid") {
		t.Errorf("empty cluster_uid must be omitted (omitempty), got: %s", b)
	}
}

// TestGetLocalClusterUID_CacheHitIsNotDomain: a populated cache returns the
// minted UUID directly (no etcd) and is never the domain — the reader does not
// derive identity from a mutable attribute.
func TestGetLocalClusterUID_CacheHitIsNotDomain(t *testing.T) {
	invalidateClusterUIDForTest()
	t.Cleanup(invalidateClusterUIDForTest)

	clusterUIDMu.Lock()
	clusterUIDVal = "11111111-2222-3333-4444-555555555555"
	clusterUIDMu.Unlock()

	got, err := GetLocalClusterUID()
	if err != nil {
		t.Fatalf("cache hit should not error: %v", err)
	}
	if got != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("got %q, want the cached minted UUID", got)
	}
	if got == "globular.internal" {
		t.Errorf("membership UUID must never be the domain")
	}
}

// TestValidateClusterMembership_UUIDOnlyFailClosed: membership is granted ONLY by
// a matching minted UUID. Empty, mismatched, and — critically — the domain are
// all denied. The domain is never a membership credential.
func TestValidateClusterMembership_UUIDOnlyFailClosed(t *testing.T) {
	invalidateClusterUIDForTest()
	t.Cleanup(invalidateClusterUIDForTest)

	clusterUIDMu.Lock()
	clusterUIDVal = "aaaaaaaa-1111-2222-3333-444444444444"
	clusterUIDMu.Unlock()

	if err := ValidateClusterMembership(""); err == nil {
		t.Error("empty cluster_uid must be denied (fail-closed)")
	}
	if err := ValidateClusterMembership("aaaaaaaa-1111-2222-3333-444444444444"); err != nil {
		t.Errorf("matching cluster_uid must be accepted: %v", err)
	}
	if err := ValidateClusterMembership("bbbbbbbb-0000-0000-0000-000000000000"); err == nil {
		t.Error("mismatched cluster_uid must be denied")
	}
	if err := ValidateClusterMembership("globular.internal"); err == nil {
		t.Error("the domain must NEVER satisfy cluster membership — it is not an identity")
	}
}
