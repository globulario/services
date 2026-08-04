package rules

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
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

// ── stale-member quorum arithmetic (F8) ──────────────────────────────────────
//
// The even-voter finding above already counted voters, but the STALE-MEMBER
// finding did not: it used len(etcdNodes) for quorumNeeded and subtracted every
// stale member — learners included — from healthy. One healthy voter beside one
// stale learner therefore computed 1 healthy < 2 needed and raised CRITICAL
// quorum loss on a cluster that never lost quorum. isLearner was parsed, and the
// comment beside it said quorum must use voters, but the arithmetic ignored it.

func staleEtcdNode(id string, learner bool, age time.Duration) *cluster_controllerpb.NodeRecord {
	n := etcdNodeRecord(id, learner)
	n.LastSeen = timestamppb.New(time.Now().Add(-age))
	return n
}

func staleMemberFinding(fs []Finding) *Finding {
	for i := range fs {
		if fs[i].InvariantID == "etcd.stale_member" && fs[i].EntityRef == "cluster" &&
			fs[i].FindingID != FindingID("etcd.stale_member", "cluster", "even_size_2") {
			return &fs[i]
		}
	}
	return nil
}

func evidenceVal(f *Finding, key string) string {
	for _, e := range f.Evidence {
		if v, ok := e.GetKeyValues()[key]; ok {
			return v
		}
	}
	return ""
}

// One healthy voter + one stale learner: quorum is 1-of-1 and remains healthy.
// The learner must still be reported, but must not drive severity to CRITICAL.
func TestStaleMember_OneVoterOneStaleLearner_NotCritical(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdNodeRecord("ryzen", false),             // healthy voter
		staleEtcdNode("nuc", true, 30*time.Minute), // stale LEARNER
	}}
	f := staleMemberFinding(etcdStaleMember{}.Evaluate(snap, Config{}))
	if f == nil {
		t.Fatal("stale learner must still be reported as a stale member")
	}
	if f.Severity == cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("a stale LEARNER must not produce CRITICAL quorum loss; summary=%s", f.Summary)
	}
	if got := evidenceVal(f, "quorum_needed"); got != "1" {
		t.Errorf("quorum_needed = %q, want 1 (learners never raise the requirement)", got)
	}
	if got := evidenceVal(f, "healthy_voters"); got != "1" {
		t.Errorf("healthy_voters = %q, want 1 (the learner is not subtracted from voters)", got)
	}
}

// Three voters, one stale: quorum 2, healthy 2 — available, so WARN not CRITICAL.
func TestStaleMember_ThreeVotersOneStale_QuorumTwo(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdNodeRecord("a", false), etcdNodeRecord("b", false),
		staleEtcdNode("c", false, 30*time.Minute),
	}}
	f := staleMemberFinding(etcdStaleMember{}.Evaluate(snap, Config{}))
	if f == nil {
		t.Fatal("expected a stale-member finding")
	}
	if got := evidenceVal(f, "quorum_needed"); got != "2" {
		t.Errorf("quorum_needed = %q, want 2", got)
	}
	if f.Severity == cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("2 healthy of 3 voters still meets quorum 2; want non-CRITICAL")
	}
}

// Two healthy voters + one stale voter: 3 voters, quorum 2, healthy 2 — available.
func TestStaleMember_TwoHealthyVotersOneStaleVoter_Available(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdNodeRecord("a", false), etcdNodeRecord("b", false),
		staleEtcdNode("c", false, time.Hour),
	}}
	f := staleMemberFinding(etcdStaleMember{}.Evaluate(snap, Config{}))
	if f == nil {
		t.Fatal("expected a stale-member finding")
	}
	if got := evidenceVal(f, "healthy_voters"); got != "2" {
		t.Errorf("healthy_voters = %q, want 2", got)
	}
	if f.Severity == cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("quorum remains available; want non-CRITICAL")
	}
}

// A stale learner must be visible and labelled, separately from stale voters.
func TestStaleMember_StaleLearnerReportedSeparately(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdNodeRecord("a", false), etcdNodeRecord("b", false), etcdNodeRecord("c", false),
		staleEtcdNode("learner1", true, time.Hour),
	}}
	f := staleMemberFinding(etcdStaleMember{}.Evaluate(snap, Config{}))
	if f == nil {
		t.Fatal("stale learner must be reported")
	}
	if got := evidenceVal(f, "stale_learners"); got == "" {
		t.Error("stale_learners evidence must name the learner")
	}
	if got := evidenceVal(f, "stale_voters"); got != "" {
		t.Errorf("stale_voters = %q, want empty (only a learner is stale)", got)
	}
	if got := evidenceVal(f, "quorum_needed"); got != "2" {
		t.Errorf("quorum_needed = %q, want 2 from the 3 voters alone", got)
	}
}
