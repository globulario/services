package main

import (
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
	CooldownRejections int
	LastRejectionAt    time.Time
	Escalated          bool
}

func remediationGateKey(findingID string, stepIndex uint32, actionType cluster_doctorpb.ActionType) string {
	return fmt.Sprintf("%s|%s|%d", findingID, actionType.String(), stepIndex)
}

func remediationGateRecordCooldownRejection(key string, now time.Time) remediationGateState {
	current := remediationGateState{}
	if v, ok := autoRemediationGateByTarget.Load(key); ok {
		current = v.(remediationGateState)
	}
	current.CooldownRejections++
	current.LastRejectionAt = now
	if current.CooldownRejections >= autoRemediationEscalationThreshold {
		current.Escalated = true
	}
	autoRemediationGateByTarget.Store(key, current)
	return current
}

func remediationGateGet(key string) (remediationGateState, bool) {
	if v, ok := autoRemediationGateByTarget.Load(key); ok {
		return v.(remediationGateState), true
	}
	return remediationGateState{}, false
}

func remediationGateClear(key string) {
	autoRemediationGateByTarget.Delete(key)
}

func appendRemediationGateEvidence(findings []rules.Finding) {
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
					"last_rejection_unix":  fmt.Sprintf("%d", state.LastRejectionAt.Unix()),
				},
				Timestamp: timestamppb.Now(),
			})
			if state.Escalated {
				finding.Summary = finding.Summary + " [auto-remediation escalated: operator approval required]"
			}
			break
		}
	}
}
