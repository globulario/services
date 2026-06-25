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

// Escalation-state persistence (EX-3). The gate state is held in-memory in
// autoRemediationGateByTarget for the process lifetime AND persisted across
// restart/failover through ai-memory (NEVER etcd —
// invariant:cluster_doctor.observer_only_never_writes_etcd). The persist/load/
// delete bodies live in remediation_gate_persist.go; they are the sanctioned
// replacement for the etcd persistence removed in v1.2.166, closing
// failure_mode:doctor.escalation_state_lost_on_restart without reintroducing a
// doctor etcd write. An escalation is observer memory — a safety refusal — not
// desired state; it is cleared only by remediationGateClear (success) or an
// operator, never on a timer
// (forbidden_fix:auto_clear_escalation_without_operator_approval).
//
// The Fn seams let tests inject a fake persistence surface.
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
