package rules

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func etcdNodeRecord(id string, learner bool) *cluster_controllerpb.NodeRecord {
	return &cluster_controllerpb.NodeRecord{
		NodeId:   id,
		Identity: &cluster_controllerpb.NodeIdentity{Hostname: id},
		LastSeen: timestamppb.New(time.Now()),
		Metadata: map[string]string{
			"etcd_join_phase": "verified",
			"etcd_is_learner": map[bool]string{true: "true", false: "false"}[learner],
		},
	}
}

func findEvenVoterFinding(fs []Finding) *Finding {
	for i := range fs {
		if fs[i].FindingID == FindingID("etcd.stale_member", "cluster", "even_size_2") {
			return &fs[i]
		}
	}
	return nil
}

// SCAR (2026-07-30): the rule counted MEMBERS, so a healthy 1-voter + 1-learner
// cluster was reported as "2 members ... zero fault tolerance".
//
// That is exactly backwards. A learner replicates the full keyspace but does not
// vote, so quorum stays 1 and the cluster survives losing the learner outright.
// The controller now deliberately parks the second node as a learner and
// promotes only onto an odd voter count, so counting members made the doctor
// warn about the very arrangement that provides the safety.
func TestEtcdStaleMember_LearnerIsNotAVoter(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdNodeRecord("ryzen", false), // voter
		etcdNodeRecord("nuc", true),    // learner — does not vote
	}}
	if f := findEvenVoterFinding(etcdStaleMember{}.Evaluate(snap, Config{})); f != nil {
		t.Errorf("1 voter + 1 learner must NOT warn about an even voter count; got: %s", f.Summary)
	}
}

// Two real voters is the genuinely bad shape: quorum 2-of-2, zero tolerance.
func TestEtcdStaleMember_TwoVotersStillWarns(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdNodeRecord("ryzen", false),
		etcdNodeRecord("nuc", false),
	}}
	f := findEvenVoterFinding(etcdStaleMember{}.Evaluate(snap, Config{}))
	if f == nil {
		t.Fatal("2 voting members must warn — quorum is 2/2 with zero fault tolerance")
	}
}

// Three voters is the healthy production shape.
func TestEtcdStaleMember_ThreeVotersDoesNotWarn(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdNodeRecord("a", false), etcdNodeRecord("b", false), etcdNodeRecord("c", false),
	}}
	if f := findEvenVoterFinding(etcdStaleMember{}.Evaluate(snap, Config{})); f != nil {
		t.Errorf("3 voters tolerate 1 failure and must not warn; got: %s", f.Summary)
	}
}

// Missing metadata (older controller) must not be read as "learner" — that
// would silently suppress the warning on a genuine 2-voter cluster.
func TestEtcdStaleMember_AbsentLearnerFlagCountsAsVoter(t *testing.T) {
	mk := func(id string) *cluster_controllerpb.NodeRecord {
		n := etcdNodeRecord(id, false)
		delete(n.Metadata, "etcd_is_learner")
		return n
	}
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{mk("a"), mk("b")}}
	if f := findEvenVoterFinding(etcdStaleMember{}.Evaluate(snap, Config{})); f == nil {
		t.Fatal("absent etcd_is_learner must count as a VOTER (fail safe), so 2 nodes still warn")
	}
}
