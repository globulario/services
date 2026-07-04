package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// etcdPhaseNode builds an etcd-capable node (core profile) in the given
// etcd_join_phase. "promoting" marks a non-voting learner awaiting promotion.
func etcdPhaseNode(id, hostname, ip, phase string) *cluster_controllerpb.NodeRecord {
	return &cluster_controllerpb.NodeRecord{
		NodeId:   id,
		Identity: &cluster_controllerpb.NodeIdentity{Hostname: hostname, Ips: []string{ip}},
		Profiles: []string{"core", "control-plane", "storage"},
		Metadata: map[string]string{"etcd_join_phase": phase},
	}
}

// TestEtcdQuorumHealth_LearnerNotCountedAsVerifiedVoter proves the etcd.quorum
// rule counts a "promoting" learner as joining, never as a verified voter
// (meta.limited_members_are_not_capacity). With 1 real voter + 1 failed + 1
// learner, quorum is at risk (1 verified < 2 needed) and the CRITICAL finding
// MUST fire. If the learner were miscounted as verified, verified would be 2 and
// the risk would be silently masked (a false PASS on a path to cluster loss).
func TestEtcdQuorumHealth_LearnerNotCountedAsVerifiedVoter(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdPhaseNode("n1", "globule-1", "10.0.0.1", "verified"),
		etcdPhaseNode("n2", "globule-2", "10.0.0.2", "failed"),
		etcdPhaseNode("n3", "globule-3", "10.0.0.3", "promoting"), // learner
	}}

	findings := (etcdQuorumHealth{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("expected a quorum_risk finding — a learner must NOT be counted as a verified voter (would mask the risk)")
	}
	f := findings[0]
	verified := ""
	for _, ev := range f.Evidence {
		if v, ok := ev.GetKeyValues()["verified"]; ok {
			verified = v
		}
	}
	if verified != "1" {
		t.Fatalf("expected verified=1 (learner excluded from voter count), got verified=%q", verified)
	}
}

// TestEtcdStaleMember_LearnerExcludedFromVoterQuorum proves the etcd.stale_member
// rule excludes a "promoting" learner from the voter-quorum analysis: 1 voter + 1
// learner is NOT a 2-voter cluster, so the "2 members / zero fault tolerance"
// warning MUST NOT fire. If the learner were counted, it would falsely warn about
// a 2/2 quorum that does not exist.
func TestEtcdStaleMember_LearnerExcludedFromVoterQuorum(t *testing.T) {
	snap := &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{
		etcdPhaseNode("n1", "globule-1", "10.0.0.1", "verified"),  // the only voter
		etcdPhaseNode("n2", "globule-2", "10.0.0.2", "promoting"), // learner
	}}

	findings := (etcdStaleMember{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Fatalf("1 voter + 1 learner is not a 2-voter cluster — expected no findings, got %d: %+v",
			len(findings), findings)
	}
}
