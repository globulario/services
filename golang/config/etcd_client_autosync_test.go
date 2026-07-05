package config

import "testing"

// TestAutoSyncIntervalFor covers the Wall-2 client rule: when the local etcd
// member is ABSENT from the endpoint list (the deliberate "point at remote voters
// while local is a learner" case), AutoSync must be DISABLED so a MemberList
// against a voter cannot drag the local learner endpoint back in. When the local
// member IS present (founder / promoted voter), AutoSync stays enabled.
func TestAutoSyncIntervalFor(t *testing.T) {
	// 127.0.0.1 / localhost are always treated as local, so these are deterministic
	// regardless of the host's routable IP.
	if got := autoSyncIntervalFor([]string{"203.0.113.10:2379"}); got != 0 {
		t.Fatalf("voter-only endpoints (no local member) must disable AutoSync, got %v", got)
	}
	if got := autoSyncIntervalFor([]string{"203.0.113.10:2379", "198.51.100.7:2379"}); got != 0 {
		t.Fatalf("multiple remote voters (no local member) must disable AutoSync, got %v", got)
	}
	if got := autoSyncIntervalFor([]string{"127.0.0.1:2379"}); got == 0 {
		t.Fatal("a list containing the local member must keep AutoSync enabled")
	}
	if got := autoSyncIntervalFor([]string{"203.0.113.10:2379", "127.0.0.1:2379"}); got == 0 {
		t.Fatal("a list containing the local member (alongside a remote) must keep AutoSync enabled")
	}
}

// TestLocalEtcdIsAmongEndpoints spot-checks the predicate the rule is built on.
func TestLocalEtcdIsAmongEndpoints(t *testing.T) {
	if localEtcdIsAmongEndpoints([]string{"203.0.113.10:2379"}) {
		t.Fatal("a purely remote endpoint list must not be seen as containing the local member")
	}
	if !localEtcdIsAmongEndpoints([]string{"localhost:2379"}) {
		t.Fatal("localhost must be recognized as the local member")
	}
}
