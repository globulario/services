package main

import (
	"context"
	"testing"

	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

// TestVoterClientEndpoints_ExcludesLearners verifies the authoritative voter
// filter: voterClientEndpoints returns ONLY the client URLs of non-learner
// (voting) members, excluding non-voting learners (which refuse client RPCs) and
// unnamed ghost members. This is the list handed to a joining learner node so it
// never depends on its own learner for desired-state authority.
func TestVoterClientEndpoints_ExcludesLearners(t *testing.T) {
	fake := &fakeEtcdAPI{members: []*etcdserverpb.Member{
		{ID: 1, Name: "founder", IsLearner: false, ClientURLs: []string{"https://10.0.0.63:2379"}},
		{ID: 2, Name: "joiner", IsLearner: true, ClientURLs: []string{"https://10.0.0.8:2379"}}, // learner — excluded
		{ID: 3, Name: "", IsLearner: false, ClientURLs: []string{"https://10.0.0.99:2379"}},     // ghost — excluded
	}}
	m := &etcdMemberManager{client: fake}

	eps, err := m.voterClientEndpoints(context.Background())
	if err != nil {
		t.Fatalf("voterClientEndpoints error: %v", err)
	}
	if len(eps) != 1 || eps[0] != "https://10.0.0.63:2379" {
		t.Fatalf("expected only the voter's client URL, got %v", eps)
	}
}

// TestVoterClientEndpoints_MultipleVotersSorted verifies all voters are returned,
// deduplicated and sorted, once the cluster is at HA.
func TestVoterClientEndpoints_MultipleVotersSorted(t *testing.T) {
	fake := &fakeEtcdAPI{members: []*etcdserverpb.Member{
		{ID: 1, Name: "c", IsLearner: false, ClientURLs: []string{"https://10.0.0.9:2379"}},
		{ID: 2, Name: "a", IsLearner: false, ClientURLs: []string{"https://10.0.0.8:2379"}},
		{ID: 3, Name: "b", IsLearner: false, ClientURLs: []string{"https://10.0.0.63:2379"}},
	}}
	m := &etcdMemberManager{client: fake}

	eps, err := m.voterClientEndpoints(context.Background())
	if err != nil {
		t.Fatalf("voterClientEndpoints error: %v", err)
	}
	want := []string{"https://10.0.0.63:2379", "https://10.0.0.8:2379", "https://10.0.0.9:2379"}
	if len(eps) != len(want) {
		t.Fatalf("expected %d voter endpoints, got %v", len(want), eps)
	}
	for i := range want {
		if eps[i] != want[i] {
			t.Fatalf("voter endpoints not sorted as expected: got %v want %v", eps, want)
		}
	}
}
