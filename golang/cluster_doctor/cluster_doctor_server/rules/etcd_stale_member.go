// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.etcd_stale_member
// @awareness file_role=doctor_rule_detecting_quorum_loss_risk_from_unreachable_or_even_count_etcd_members
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness risk=high
package rules

// etcd_stale_member.go — DIAGNOSTIC ONLY. Read-only doctor rule.
// Detects two etcd topologies that caused real Day-1 quorum loss:
//
//   1. Even-numbered cluster (2 members → no fault tolerance).
//   2. Members joined to etcd whose heartbeat is stale (node wiped
//      without `etcdctl member remove` first).
//
// MUST NOT mutate any etcd state. The rule emits findings; the
// operator decides whether to evict, and runs the typed eviction
// RPC explicitly. An auto-eviction shortcut here would re-create
// the keepalived-VIP-eviction cascade (StableIP vs PrimaryIP
// confusion) that took down ryzen for hours.

import (
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ── etcd stale member / unsafe cluster size ─────────────────────────────────
//
// Detects two conditions that caused quorum loss during Day 1 testing:
//
// 1. Even-numbered etcd cluster (2 members = no fault tolerance, both need
//    to be up for quorum). This is always wrong — use 1 or 3.
//
// 2. Nodes that were added to etcd (etcd_join_phase=verified) but are now
//    unreachable (stale last_seen). If the node was wiped without removing
//    its etcd membership, the cluster will lose quorum.

type etcdStaleMember struct{}

func (etcdStaleMember) ID() string       { return "etcd.stale_member" }
func (etcdStaleMember) Category() string { return "etcd" }
func (etcdStaleMember) Scope() string    { return "cluster" }

func (etcdStaleMember) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	// Guard: refuse to emit findings when the data source errored — "no data" must not become "no problems." See meta.absence_scope_must_be_explicit.
	if snap.HadError("cluster_controller", "ListNodes") {
		return nil
	}

	var findings []Finding

	// Gather etcd-capable nodes and their status.
	type etcdNode struct {
		nodeID    string
		hostname  string
		joinPhase string
		lastSeen  time.Time
		stale     bool
		isLearner bool
	}

	var etcdNodes []etcdNode
	staleThreshold := cfg.HeartbeatStale
	if staleThreshold == 0 {
		staleThreshold = 2 * time.Minute
	}

	for _, node := range snap.Nodes {
		meta := node.GetMetadata()
		if meta == nil {
			continue
		}
		joinPhase := meta["etcd_join_phase"]
		if joinPhase == "" {
			continue
		}
		// A learner replicates but does NOT vote, so it never counts toward
		// quorum. Fault-tolerance arithmetic must use VOTERS, not members.
		isLearner := meta["etcd_is_learner"] == "true"

		lastSeen := node.GetLastSeen().AsTime()
		stale := time.Since(lastSeen) > staleThreshold

		etcdNodes = append(etcdNodes, etcdNode{
			isLearner: isLearner,
			nodeID:    node.GetNodeId(),
			hostname:  node.GetIdentity().GetHostname(),
			joinPhase: joinPhase,
			lastSeen:  lastSeen,
			stale:     stale,
		})
	}

	if len(etcdNodes) < 2 {
		return nil
	}

	// Check for stale etcd members (joined but unreachable).
	var staleMembers []string
	var verifiedCount int
	for _, n := range etcdNodes {
		if n.joinPhase == "verified" {
			verifiedCount++
		}
		if n.stale && n.joinPhase == "verified" {
			staleMembers = append(staleMembers, fmt.Sprintf("%s(%s)", n.hostname, n.nodeID[:8]))
		}
	}

	if len(staleMembers) > 0 {
		quorumNeeded := len(etcdNodes)/2 + 1
		healthyCount := verifiedCount - len(staleMembers)
		sev := cluster_doctorpb.Severity_SEVERITY_WARN
		if healthyCount < quorumNeeded {
			sev = cluster_doctorpb.Severity_SEVERITY_CRITICAL
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("etcd.stale_member", "cluster", strings.Join(staleMembers, ",")),
			InvariantID: "etcd.stale_member",
			Severity:    sev,
			Category:    "etcd",
			EntityRef:   "cluster",
			Summary: fmt.Sprintf("etcd has %d stale member(s): %s — node(s) joined etcd but stopped heartbeating. "+
				"If wiped without removing etcd membership, quorum will be lost (%d healthy / %d needed).",
				len(staleMembers), strings.Join(staleMembers, ", "), healthyCount, quorumNeeded),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "etcd.stale_member", map[string]string{
					"stale_members": strings.Join(staleMembers, ","),
					"total_etcd":    fmt.Sprintf("%d", len(etcdNodes)),
					"verified":      fmt.Sprintf("%d", verifiedCount),
					"healthy":       fmt.Sprintf("%d", healthyCount),
					"quorum_needed": fmt.Sprintf("%d", quorumNeeded),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Remove the stale node from the cluster (this also removes its etcd membership)",
					"globular --timeout 30s cluster nodes remove <stale-node-id> --force --drain=false"),
				step(2, "If quorum is already lost (etcd unresponsive), force single-node recovery",
					"sudo systemctl stop globular-etcd && sudo rm -rf /var/lib/globular/etcd && sudo systemctl start globular-etcd"),
				step(3, "After etcd recovery, restart ALL services to re-register",
					"sudo systemctl restart 'globular-*.service'"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// Warn about an EVEN voter count (no added fault tolerance).
	//
	// Count VOTERS, not members. Learners replicate the full keyspace but do not
	// vote, so a 1-voter + 1-learner cluster has quorum 1 and survives losing the
	// learner entirely — the opposite of the danger this rule warns about. The
	// controller deliberately parks the second node as a learner and promotes
	// only onto an odd voter count, so counting members here reported "zero
	// fault tolerance" for the exact arrangement that provides it, on every
	// healthy 2-node cluster.
	voters := 0
	learners := 0
	for _, n := range etcdNodes {
		if n.isLearner {
			learners++
			continue
		}
		voters++
	}
	if voters == 2 {
		findings = append(findings, Finding{
			FindingID:   FindingID("etcd.stale_member", "cluster", "even_size_2"),
			InvariantID: "etcd.stale_member",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "etcd",
			EntityRef:   "cluster",
			Summary: fmt.Sprintf("etcd has 2 voting members — both must be up for quorum (2/2). "+
				"This has zero fault tolerance, the same as a single voter. "+
				"Use 1 voter (dev) or 3 (production).%s",
				learnerSuffix(learners)),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "etcd.stale_member", map[string]string{
					"etcd_voters":   "2",
					"etcd_learners": fmt.Sprintf("%d", learners),
					"quorum_needed": "2",
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Add a third node for fault tolerance, or stay single-node for dev", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}

// learnerSuffix renders the non-voting member count so an operator can see why
// the voter total differs from the member total.
func learnerSuffix(learners int) string {
	if learners == 0 {
		return ""
	}
	if learners == 1 {
		return " (1 learner is present and does not vote)"
	}
	return fmt.Sprintf(" (%d learners are present and do not vote)", learners)
}
