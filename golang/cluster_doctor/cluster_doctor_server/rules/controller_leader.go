package rules

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// pendingUpdateStaleness is the maximum age of a leader_pending_update record
// before the doctor treats the condition as resolved. The controller refreshes
// every ~30 s, so 5 minutes without a refresh means the leader resigned or
// the mismatch was otherwise corrected.
const pendingUpdateStaleness = 5 * time.Minute

// pendingUpdateEscalateAfter is the StuckSinceUnix age beyond which the
// severity escalates from WARNING to ERROR. A leader stuck for this long
// indicates all followers are broken or unreachable — manual action is needed.
const pendingUpdateEscalateAfter = 20 * time.Minute

// --- controller.leader_pending_update ----------------------------------------
//
// Fires when the controller leader cannot self-update because no follower
// controller has reached the target build. While stuck, no follower is safe
// to inherit leadership, so the cluster is running an outdated control plane.
//
// The record is written by the controller's reconcileControllerSelfUpdate loop
// and is refreshed every ~30 s while the condition persists. The doctor treats
// the record as stale (resolved) if it has not been refreshed within
// pendingUpdateStaleness.

type controllerLeaderPendingUpdate struct{}

func (controllerLeaderPendingUpdate) ID() string       { return "controller.leader_pending_update" }
func (controllerLeaderPendingUpdate) Category() string { return "control_plane" }
func (controllerLeaderPendingUpdate) Scope() string    { return "cluster" }

func (c controllerLeaderPendingUpdate) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	rec := snap.LeaderPendingUpdate
	if rec == nil {
		return nil
	}
	if rec.DetectedAtUnix == 0 {
		return nil
	}

	cutoff := time.Now().Add(-pendingUpdateStaleness)
	if time.Unix(rec.DetectedAtUnix, 0).Before(cutoff) {
		return nil
	}

	stuckSince := time.Unix(rec.StuckSinceUnix, 0)
	stuckDuration := time.Since(stuckSince)

	severity := cluster_doctorpb.Severity_SEVERITY_WARN
	if rec.StuckSinceUnix > 0 && stuckDuration > pendingUpdateEscalateAfter {
		severity = cluster_doctorpb.Severity_SEVERITY_ERROR
	}

	entityRef := "cluster/controller-leader"
	if rec.LeaderNodeID != "" {
		entityRef = rec.LeaderNodeID + "/cluster-controller"
	}

	summary := fmt.Sprintf(
		"Controller leader %s is stuck at %s — cannot resign to update to %s: no follower is at target build (%d total, all blocked)",
		rec.LeaderNodeID, rec.CurrentVersion, rec.TargetVersion, rec.FollowersTotal)

	evidenceFields := map[string]string{
		"leader_node_id":  rec.LeaderNodeID,
		"current_version": rec.CurrentVersion,
		"target_version":  rec.TargetVersion,
		"followers_total": fmt.Sprintf("%d", rec.FollowersTotal),
	}
	if rec.StuckSinceUnix > 0 {
		evidenceFields["stuck_since"] = stuckSince.UTC().Format(time.RFC3339)
		evidenceFields["stuck_duration"] = stuckDuration.Truncate(time.Second).String()
	}

	return []Finding{{
		FindingID:   FindingID(c.ID(), entityRef, rec.TargetVersion),
		InvariantID: c.ID(),
		Severity:    severity,
		Category:    c.Category(),
		EntityRef:   entityRef,
		Summary:     summary,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "/globular/controller/leader_pending_update", evidenceFields),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Check the status of each follower controller — they must reach the target build before the leader can resign",
				"globular cluster get-node-plan --service cluster-controller"),
			step(2, "If a follower is stuck installing the update, check its node-agent logs",
				fmt.Sprintf("globular node logs --node %s --unit cluster-controller.service", rec.LeaderNodeID)),
			step(3, "If followers cannot update automatically, deploy the target version manually to each follower first, then the leader",
				fmt.Sprintf("globular deploy cluster-controller --version %s", rec.TargetVersion)),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}
