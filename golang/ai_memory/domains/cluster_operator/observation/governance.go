package observation

// governance.go is the SYNCHRONOUS half of the behavioral path.
//
// The Recorder (recorder.go) is deliberately fire-and-forget: learning must
// never delay or endanger reporting. A governance check is the opposite kind of
// call — its answer decides whether an action happens, so it must block, and it
// must fail closed. Sharing one client for both would force one policy on two
// opposite requirements.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	behavioralpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
)

// GateDecision is the governor's answer about one proposed action.
type GateDecision struct {
	ActionCheckID string
	Governed      bool
	Allowed       bool
	Status        string
	Reason        string
	PrincipleIDs  []string
}

// ActionContext is the real subject and action a check is about.
type ActionContext struct {
	Project       string
	Domain        string
	ActionKind    string
	Target        string
	Conditions    []string
	ClusterID     string
	InvariantID   string
	EntityRef     string
	FindingID     string
	WorkflowRunID string
	HumanApproval string
	EvaluatedAt   time.Time
}

// OutcomeRecord is a governed result linked back to the decision that allowed
// (or refused) it.
type OutcomeRecord struct {
	Project            string
	Domain             string
	ActionCheckID      string
	PrincipleIDs       []string
	EvidenceIDs        []string
	Status             string
	Theme              string
	Note               string
	SupportsPrinciples []string
	WeakensPrinciples  []string
	Metadata           map[string]string
}

// Governor is a lazily-connected client for the blocking governance calls.
type Governor struct {
	addr    string
	timeout time.Duration

	mu sync.Mutex
	cc *grpc.ClientConn
}

// NewGovernor returns a client. addr empty resolves from config at first use.
func NewGovernor(addr string, timeout time.Duration) *Governor {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Governor{addr: addr, timeout: timeout}
}

func (g *Governor) conn() (*grpc.ClientConn, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cc != nil {
		return g.cc, nil
	}
	addr := g.addr
	if addr == "" {
		addr = config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	}
	if addr == "" {
		return nil, errors.New("behavioral-memory endpoint not resolvable")
	}
	opts, err := globular.InternalDialOptions()
	if err != nil {
		return nil, fmt.Errorf("behavioral-memory dial options: %w", err)
	}
	cc, err := grpc.Dial(addr, opts...) //nolint:staticcheck // matches Recorder: NewClient changes target resolution
	if err != nil {
		return nil, fmt.Errorf("behavioral-memory dial: %w", err)
	}
	g.cc = cc
	return cc, nil
}

// Close releases the connection.
func (g *Governor) Close() error {
	g.mu.Lock()
	cc := g.cc
	g.cc = nil
	g.mu.Unlock()
	if cc != nil {
		return cc.Close()
	}
	return nil
}

// CheckAction asks whether the action may proceed.
//
// EVERY failure path returns an error rather than a decision. A caller must
// never be able to mistake "the governor was unreachable" for "the governor
// approved" — an unreachable gate that returned a permissive default would make
// governance strongest exactly when it is working and absent exactly when it is
// not.
func (g *Governor) CheckAction(ctx context.Context, a ActionContext) (GateDecision, error) {
	cc, err := g.conn()
	if err != nil {
		// The cached connection may be the problem; drop it so the next attempt
		// redials rather than reusing a corpse.
		_ = g.Close()
		return GateDecision{}, err
	}
	client := behavioralpb.NewBehavioralMemoryServiceClient(cc)
	cctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	var evaluatedAt int64
	if !a.EvaluatedAt.IsZero() {
		evaluatedAt = a.EvaluatedAt.Unix()
	}
	resp, err := client.CheckAction(cctx, &behavioralpb.CheckActionRequest{
		Project:           a.Project,
		Domain:            a.Domain,
		ActionType:        a.ActionKind,
		Target:            a.Target,
		CurrentConditions: a.Conditions,
		HumanApproval:     a.HumanApproval,
		EvaluatedAt:       evaluatedAt,
		// The subject. Note there are deliberately NO provided_evidence_refs:
		// the caller does not get to assert its own evidence into a decision
		// about its own action — qualification is the catalog's job.
		ActionScope: &behavioralpb.ActionScope{
			ClusterId:    a.ClusterID,
			EntityRef:    a.EntityRef,
			ConditionRef: a.InvariantID,
			SourceRef:    a.FindingID,
			ActionRef:    a.WorkflowRunID,
		},
	})
	if err != nil {
		_ = g.Close()
		return GateDecision{}, fmt.Errorf("check action: %w", err)
	}
	r := resp.GetResult()
	if r == nil {
		return GateDecision{}, errors.New("check action: empty result")
	}
	return GateDecision{
		ActionCheckID: r.GetId(),
		Governed:      r.GetGoverned(),
		Allowed:       r.GetAllowed(),
		Status:        r.GetStatus(),
		Reason:        gateReason(r),
		PrincipleIDs:  r.GetCheckedAgainstPrinciples(),
	}, nil
}

// gateReason picks the most actionable explanation the governor offered.
func gateReason(r *behavioralpb.ActionCheck) string {
	if steps := r.GetRecommendedSteps(); len(steps) > 0 {
		return steps[0]
	}
	if e := r.GetExplanation(); e != "" {
		return e
	}
	return r.GetStatus()
}

// RecordOutcome persists what happened after a governed action.
func (g *Governor) RecordOutcome(ctx context.Context, o OutcomeRecord) (string, error) {
	cc, err := g.conn()
	if err != nil {
		_ = g.Close()
		return "", err
	}
	client := behavioralpb.NewBehavioralMemoryServiceClient(cc)
	cctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	resp, err := client.RecordOutcome(cctx, &behavioralpb.RecordOutcomeRequest{
		Outcome: &behavioralpb.Outcome{
			Project:            o.Project,
			Domain:             o.Domain,
			ActionCheckId:      o.ActionCheckID,
			PrincipleIds:       o.PrincipleIDs,
			EvidenceIds:        o.EvidenceIDs,
			Status:             o.Status,
			Theme:              o.Theme,
			Note:               o.Note,
			SupportsPrinciples: o.SupportsPrinciples,
			WeakensPrinciples:  o.WeakensPrinciples,
			Metadata:           o.Metadata,
		},
	})
	if err != nil {
		_ = g.Close()
		return "", fmt.Errorf("record outcome: %w", err)
	}
	return resp.GetOutcomeId(), nil
}
