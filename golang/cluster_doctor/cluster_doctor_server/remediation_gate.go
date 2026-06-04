// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.remediation_gate
// @awareness file_role=multi_layer_circuit_breaker_for_autonomous_remediation
// @awareness implements=globular.platform:intent.autonomy.remediation_is_bounded_and_escalates
// @awareness implements=globular.platform:intent.circuit_breakers_protect_convergence
// @awareness risk=critical
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const autoRemediationEscalationThreshold = 3

var autoRemediationGateByTarget sync.Map // key -> remediationGateState

type remediationGateState struct {
	CooldownRejections int   `json:"cooldown_rejections"`
	LastRejectionAt    int64 `json:"last_rejection_at_unix"`
	Escalated          bool  `json:"escalated"`
}

// The gate state was previously persisted to /globular/cluster_doctor/
// remediation_gate/<key> in etcd so escalations survived a doctor restart.
// invariant:cluster_doctor.observer_only_never_writes_etcd forbids that
// write. As of v1.2.166 the gate state is in-memory only via the
// autoRemediationGateByTarget sync.Map.
//
// Trade-off acknowledged: failure_mode:doctor.escalation_state_lost_on_restart
// is now exposed at the doctor-restart boundary. Recovering it without an
// etcd write requires persistence through ai-memory (the doctor's typed
// history store) — tracked as the follow-up to this commit. Until that
// lands the gate counters reset on doctor restart and operator approval
// is required again for any action that was previously escalated.
//
// The hook functions remain in place so tests can inject a fake persistence
// surface and so the eventual ai-memory wiring slots in here without
// touching call sites.
var (
	remediationGatePersistFn = remediationGatePersist
	remediationGateLoadFn    = remediationGateLoad
	remediationGateDeleteFn  = remediationGateDelete
)

func remediationGateKey(findingID string, stepIndex uint32, actionType cluster_doctorpb.ActionType) string {
	return fmt.Sprintf("%s|%s|%d", findingID, actionType.String(), stepIndex)
}

// remediationGateRecordCooldownRejection increments the escalation counter for
// a (findingID, stepIndex, actionType) gate key. After
// autoRemediationEscalationThreshold rejections the gate is escalated — all
// further auto-executions for that key require operator approval.
//
func remediationGateRecordCooldownRejection(key string, now time.Time) remediationGateState {
	current, _ := remediationGateGet(key)
	current.CooldownRejections++
	current.LastRejectionAt = now.Unix()
	if current.CooldownRejections >= autoRemediationEscalationThreshold {
		current.Escalated = true
	}
	autoRemediationGateByTarget.Store(key, current)
	remediationGatePersistFn(context.Background(), key, current)
	return current
}

func remediationGateGet(key string) (remediationGateState, bool) {
	if v, ok := autoRemediationGateByTarget.Load(key); ok {
		return v.(remediationGateState), true
	}
	state, ok := remediationGateLoadFn(context.Background(), key)
	if !ok {
		return remediationGateState{}, false
	}
	autoRemediationGateByTarget.Store(key, state)
	return state, true
}

func remediationGateClear(key string) {
	autoRemediationGateByTarget.Delete(key)
	remediationGateDeleteFn(context.Background(), key)
}

// remediationGatePersist is intentionally a no-op as of v1.2.166.
//
// invariant:cluster_doctor.observer_only_never_writes_etcd forbids the
// previous /globular/cluster_doctor/remediation_gate/<key> write. The
// gate state is held in autoRemediationGateByTarget (sync.Map) for the
// doctor's process lifetime.
//
// See:
//   invariant:cluster_doctor.observer_only_never_writes_etcd
//   forbidden_fix:cluster_doctor_direct_write_to_etcd
//   failure_mode:doctor.escalation_state_lost_on_restart (now exposed
//                until ai-memory wiring lands)
func remediationGatePersist(_ context.Context, _ string, _ remediationGateState) {
	// Observer-only: no etcd write. In-memory state in
	// autoRemediationGateByTarget is the only persistence until ai-memory
	// wiring replaces this.
}

// remediationGateLoad is intentionally a no-op as of v1.2.166. See
// remediationGatePersist. Returns zero-state so callers fall through to
// "no prior state" semantics.
func remediationGateLoad(_ context.Context, _ string) (remediationGateState, bool) {
	return remediationGateState{}, false
}

// remediationGateDelete is intentionally a no-op as of v1.2.166. The
// in-memory state is removed by the autoRemediationGateByTarget.Delete
// call in remediationGateClear, which is the only thing callers need.
func remediationGateDelete(_ context.Context, _ string) {
	// Observer-only: no etcd delete.
}

// Keep the time import referenced (still used by LastRejectionAt).
var _ = time.Now

type remediationGateSummary struct {
	Escalated int
	Cooldown  int
}

func appendRemediationGateEvidence(findings []rules.Finding) remediationGateSummary {
	summary := remediationGateSummary{}
	for i := range findings {
		if len(findings[i].Remediation) == 0 {
			continue
		}
		for stepIndex, step := range findings[i].Remediation {
			action := step.GetAction()
			if action == nil {
				continue
			}
			key := remediationGateKey(findings[i].FindingID, uint32(stepIndex), action.GetActionType())
			state, ok := remediationGateGet(key)
			if !ok || state.CooldownRejections == 0 {
				continue
			}
			if state.Escalated {
				summary.Escalated++
			} else {
				summary.Cooldown++
			}
			finding := &findings[i]
			finding.Evidence = append(finding.Evidence, &cluster_doctorpb.Evidence{
				SourceService: "cluster_doctor",
				SourceRpc:     "ExecuteRemediation",
				KeyValues: map[string]string{
					"remediation_gate":     map[bool]string{true: "escalated", false: "cooldown"}[state.Escalated],
					"step_index":           fmt.Sprintf("%d", stepIndex),
					"action_type":          action.GetActionType().String(),
					"cooldown_rejections":  fmt.Sprintf("%d", state.CooldownRejections),
					"escalation_threshold": fmt.Sprintf("%d", autoRemediationEscalationThreshold),
					"last_rejection_unix":  fmt.Sprintf("%d", state.LastRejectionAt),
				},
				Timestamp: timestamppb.Now(),
			})
			if state.Escalated {
				finding.Summary = finding.Summary + " [auto-remediation escalated: operator approval required]"
			}
			break
		}
	}
	return summary
}
