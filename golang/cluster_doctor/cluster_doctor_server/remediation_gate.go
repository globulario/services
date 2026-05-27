package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const autoRemediationEscalationThreshold = 3

const remediationGateEtcdPrefix = "/globular/cluster_doctor/remediation_gate/"

var autoRemediationGateByTarget sync.Map // key -> remediationGateState

type remediationGateState struct {
	CooldownRejections int   `json:"cooldown_rejections"`
	LastRejectionAt    int64 `json:"last_rejection_at_unix"`
	Escalated          bool  `json:"escalated"`
}

var getRemediationGateEtcdClient = config.GetEtcdClient

var (
	remediationGatePersistFn = remediationGatePersist
	remediationGateLoadFn    = remediationGateLoad
	remediationGateDeleteFn  = remediationGateDelete
)

func remediationGateKey(findingID string, stepIndex uint32, actionType cluster_doctorpb.ActionType) string {
	return fmt.Sprintf("%s|%s|%d", findingID, actionType.String(), stepIndex)
}

func remediationGateEtcdKey(gateKey string) string {
	return remediationGateEtcdPrefix + gateKey
}

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

func remediationGatePersist(ctx context.Context, key string, state remediationGateState) {
	cli, err := getRemediationGateEtcdClient()
	if err != nil {
		return
	}
	body, err := json.Marshal(state)
	if err != nil {
		return
	}
	putCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, _ = cli.Put(putCtx, remediationGateEtcdKey(key), string(body))
}

func remediationGateLoad(ctx context.Context, key string) (remediationGateState, bool) {
	cli, err := getRemediationGateEtcdClient()
	if err != nil {
		return remediationGateState{}, false
	}
	getCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := cli.Get(getCtx, remediationGateEtcdKey(key))
	if err != nil || len(resp.Kvs) == 0 {
		return remediationGateState{}, false
	}
	var state remediationGateState
	if err := json.Unmarshal(resp.Kvs[0].Value, &state); err != nil {
		return remediationGateState{}, false
	}
	return state, true
}

func remediationGateDelete(ctx context.Context, key string) {
	cli, err := getRemediationGateEtcdClient()
	if err != nil {
		return
	}
	delCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, _ = cli.Delete(delCtx, remediationGateEtcdKey(key))
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
