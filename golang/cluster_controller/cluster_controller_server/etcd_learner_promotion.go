package main

// etcd_learner_promotion.go — topology-aware learner promotion (Policy A).
//
// Day-1 etcd joins add the new member as a NON-VOTING learner (installer:
// `etcdctl member add --learner`), so a failed pre-promotion join never changes
// founder quorum (invariant:infra.etcd.full_voter_join_may_break_founder_quorum,
// proven in etcd_learner_harness_test.go). This file owns the other half: when —
// and only when — a learner is promoted to a voter.
//
// Policy A′ (invariant:infra.etcd.two_voter_topology_is_not_ha): a 2-voter etcd
// cluster is NOT highly available (lose either voter and quorum is gone), so the
// controller must never SETTLE at 2 voters as a steady state. But etcd 3.5.14
// hard-caps learners at 1 (proven in etcd_learner_harness_test.go), so the only
// way to grow 1 -> 3 voters is SEQUENTIALLY, passing THROUGH a transient 2-voter
// state: 1v -> +learner -> promote -> 2v(transient) -> +learner -> promote -> 3v.
// The invariant forbids 2 voters as a FINAL state, not as a transitional step
// driven immediately onward to 3. Promotion is therefore gated on an intended
// voter target: promote only while there is more HA to reach (voters < target)
// and the target is itself an HA topology (target >= etcdHAVoterTarget). Catch-up
// is enforced by etcd itself (MemberPromote fails until the learner is in sync),
// so this driver only decides the topology question; it retries sync each cycle.

import (
	"context"
	"log"
	"time"
)

// etcdHAVoterTarget is the minimum number of voting members for a
// highly-available etcd control plane. 1 voter = single-node (not HA); 2 voters
// = transitional (not HA — both required for quorum); 3 voters = first HA state
// (survives one voter loss). Matches founding.quorum.three_nodes_required.
const etcdHAVoterTarget = 3

// topologyAllowsLearnerPromotion implements Policy A′: promote a learner to a
// voter only while the cluster is still climbing toward an HA voter target, and
// only when that target is itself HA (>= etcdHAVoterTarget). Because etcd 3.5.14
// permits only one learner at a time, this deliberately DOES allow the transient
// 2-voter step (voters=1 -> 2) — but only when target >= 3, so the driver never
// stops at 2: it keeps promoting until voters == target.
//
//	voters=1 learners=0 target=3 -> false (nothing to promote)
//	voters=1 learners=1 target=3 -> true  (promote to 2v, a transient step to 3)
//	voters=2 learners=1 target=3 -> true  (finish the 3-voter transition)
//	voters=3 learners=0 target=3 -> false (already at HA target)
//	voters=1 learners=1 target=2 -> false (target is not HA — never settle at 2v)
//	voters=1 learners=1 target=1 -> false (single-node intent; keep the learner)
//	voters=3 learners=1 target=3 -> false (at target; do not overshoot to 4 voters)
//
// target is the number of nodes INTENDED to be etcd voters, supplied by the
// controller from cluster intent (not read from etcd).
func topologyAllowsLearnerPromotion(voters, learners, target int) bool {
	if learners == 0 {
		return false
	}
	if target < etcdHAVoterTarget {
		// Only grow toward a genuinely HA target; a 2-voter target is the trap.
		return false
	}
	return voters < target
}

// reconcileLearnerPromotion promotes at most one caught-up learner per cycle while
// Policy A′ permits growth toward targetVoters. It is idempotent and bounded: if
// no promotion is warranted, or a learner is not yet in sync (etcd rejects
// MemberPromote), it does nothing and is retried on the next reconcile tick.
// Returns true if a member was promoted (state changed).
//
// targetVoters is the intended etcd voter count from cluster intent. The mutation
// here (MemberPromote) is a controller-initiated membership change that only ADDS
// a voter toward the target; it never removes voters and never runs when quorum
// is already lost.
func (m *etcdMemberManager) reconcileLearnerPromotion(ctx context.Context, targetVoters int) (dirty bool) {
	if m == nil || m.client == nil {
		return false
	}

	listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	resp, err := m.client.MemberList(listCtx)
	cancel()
	if err != nil {
		log.Printf("etcd promotion: member list failed: %v", err)
		return false
	}

	voters, learners := 0, 0
	learnerIDs := make([]uint64, 0, len(resp.Members))
	for _, mem := range resp.Members {
		if mem.IsLearner {
			learners++
			learnerIDs = append(learnerIDs, mem.ID)
		} else {
			voters++
		}
	}

	if !topologyAllowsLearnerPromotion(voters, learners, targetVoters) {
		// Policy A′: either nothing to promote, already at the HA target, or the
		// target is not itself HA — in all cases keep learner(s) non-voting rather
		// than settle at a non-HA voter count
		// (invariant:infra.etcd.two_voter_topology_is_not_ha).
		return false
	}

	// Promote one caught-up learner per cycle; etcd rejects a learner that is not
	// yet in sync with the leader, which we treat as "retry next cycle".
	for _, id := range learnerIDs {
		promoteCtx, pcancel := context.WithTimeout(ctx, 5*time.Second)
		_, perr := m.client.MemberPromote(promoteCtx, id)
		pcancel()
		if perr != nil {
			log.Printf("etcd promotion: learner %x not yet promotable: %v (retry next cycle)", id, perr)
			continue
		}
		log.Printf("etcd promotion: promoted learner %x to voter (topology permits HA progress; voters were %d, learners %d)",
			id, voters, learners)
		return true // one promotion per cycle; re-evaluate topology on the next tick
	}
	return false
}
