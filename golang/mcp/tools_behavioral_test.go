package main

import (
	"context"
	"net"
	"strings"
	"testing"

	behavioralpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// fakeBehavioralServer is an in-process BehavioralMemoryService used to prove the
// MCP tools wire request→RPC→response correctly. It is a canned/stateful stand-in
// (the real governance logic is tested in ai_memory_server); it implements all 12
// RPCs as required by the generated server interface.
type fakeBehavioralServer struct {
	lastSignal  *behavioralpb.Signal
	lastOutcome *behavioralpb.Outcome
}

func (f *fakeBehavioralServer) RecordSignal(_ context.Context, r *behavioralpb.RecordSignalRequest) (*behavioralpb.RecordSignalResponse, error) {
	f.lastSignal = r.GetSignal()
	return &behavioralpb.RecordSignalResponse{SignalId: "sig-1", Status: behavioralpb.GovernanceStatus_RAW_SIGNAL}, nil
}
func (f *fakeBehavioralServer) ExtractClaim(context.Context, *behavioralpb.ExtractClaimRequest) (*behavioralpb.ExtractClaimResponse, error) {
	return &behavioralpb.ExtractClaimResponse{}, nil
}
func (f *fakeBehavioralServer) RecordEvidence(context.Context, *behavioralpb.RecordEvidenceRequest) (*behavioralpb.RecordEvidenceResponse, error) {
	return &behavioralpb.RecordEvidenceResponse{}, nil
}
func (f *fakeBehavioralServer) MapAuthority(context.Context, *behavioralpb.MapAuthorityRequest) (*behavioralpb.MapAuthorityResponse, error) {
	return &behavioralpb.MapAuthorityResponse{}, nil
}
func (f *fakeBehavioralServer) RecordContradiction(context.Context, *behavioralpb.RecordContradictionRequest) (*behavioralpb.RecordContradictionResponse, error) {
	return &behavioralpb.RecordContradictionResponse{}, nil
}
func (f *fakeBehavioralServer) ProposePrinciple(_ context.Context, r *behavioralpb.ProposePrincipleRequest) (*behavioralpb.ProposePrincipleResponse, error) {
	return &behavioralpb.ProposePrincipleResponse{PrincipleId: "princ-1", Status: behavioralpb.GovernanceStatus_PROPOSED_PRINCIPLE}, nil
}
func (f *fakeBehavioralServer) PromotePrinciple(context.Context, *behavioralpb.PromotePrincipleRequest) (*behavioralpb.PromotePrincipleResponse, error) {
	// Mirror the gate: a freshly-proposed principle is BLOCKED, never hidden.
	return &behavioralpb.PromotePrincipleResponse{
		Decision: behavioralpb.PromotionDecision_PROMOTION_BLOCKED,
		Status:   behavioralpb.GovernanceStatus_PROPOSED_PRINCIPLE,
		Record:   &behavioralpb.PromotionDecisionRecord{Id: "dec-1", Decision: behavioralpb.PromotionDecision_PROMOTION_BLOCKED, Verdict: "no evidence"},
	}, nil
}
func (f *fakeBehavioralServer) RevokePrinciple(context.Context, *behavioralpb.RevokePrincipleRequest) (*behavioralpb.RevokePrincipleResponse, error) {
	return &behavioralpb.RevokePrincipleResponse{Status: behavioralpb.GovernanceStatus_REVOKED}, nil
}
func (f *fakeBehavioralServer) ExplainPrinciple(_ context.Context, r *behavioralpb.ExplainPrincipleRequest) (*behavioralpb.ExplainPrincipleResponse, error) {
	return &behavioralpb.ExplainPrincipleResponse{
		Principle: &behavioralpb.Principle{
			Id: r.GetPrincipleId(), Status: behavioralpb.GovernanceStatus_PROMOTED_PRINCIPLE, RiskLevel: "high",
			AppliesWhen: []string{"condition.cluster.etcd.nospace_alarm"}, Authorities: []string{"authority.cluster.etcd.member_health"},
			RequiredEvidence: []string{"evidence.cluster.etcd.alarm_status"}, ForbiddenMoves: []string{"forbidden.cluster.restart_before_quorum_check"},
			RecommendedAction: "establish member health first", SourceRefs: []string{"seed:x"}, GeneratedFrom: []string{"opsknowledge:y"},
		},
		Explanation: "why this principle exists",
	}, nil
}
func (f *fakeBehavioralServer) ResolveGovernedContext(context.Context, *behavioralpb.ResolveGovernedContextRequest) (*behavioralpb.ResolveGovernedContextResponse, error) {
	return &behavioralpb.ResolveGovernedContextResponse{Context: &behavioralpb.GovernedContext{
		ApplicablePrinciples: []*behavioralpb.Principle{{Id: "p1", Title: "preserve quorum", RiskLevel: "high",
			RecommendedAction: "check member health", ForbiddenMoves: []string{"forbidden.cluster.restart_before_quorum_check"},
			RequiredEvidence: []string{"evidence.cluster.etcd.alarm_status"}, Authorities: []string{"authority.cluster.etcd.member_health"}}},
		RecommendedBehavior: "establish quorum safety before restart", Confidence: "high",
	}}, nil
}
func (f *fakeBehavioralServer) CheckAction(_ context.Context, r *behavioralpb.CheckActionRequest) (*behavioralpb.CheckActionResponse, error) {
	status, allowed := "allowed", true
	switch {
	case strings.HasPrefix(r.GetActionType(), "forbidden."):
		status, allowed = "blocked", false
	case r.GetActionType() == "needs-evidence":
		status, allowed = "needs_evidence", false
	}
	return &behavioralpb.CheckActionResponse{Result: &behavioralpb.ActionCheck{
		Id: "ac-1", Status: status, Allowed: allowed, ActionType: r.GetActionType(),
	}}, nil
}
func (f *fakeBehavioralServer) RecordOutcome(_ context.Context, r *behavioralpb.RecordOutcomeRequest) (*behavioralpb.RecordOutcomeResponse, error) {
	f.lastOutcome = r.GetOutcome()
	return &behavioralpb.RecordOutcomeResponse{OutcomeId: "out-1"}, nil
}

// startFakeBehavioral starts the fake on a local TCP listener and returns a server
// wired to it through a pre-inserted insecure conn (bypassing the TLS dial path).
func startFakeBehavioral(t *testing.T) (*server, *fakeBehavioralServer) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeBehavioralServer{}
	gs := grpc.NewServer()
	behavioralpb.RegisterBehavioralMemoryServiceServer(gs, fake)
	go gs.Serve(lis)
	t.Cleanup(gs.Stop)

	addr := lis.Addr().String()
	old := behavioralEndpoint
	behavioralEndpoint = func() string { return addr }
	t.Cleanup(func() { behavioralEndpoint = old })

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	s := &server{
		tools:   make(map[string]*registeredTool),
		cfg:     &MCPConfig{},
		clients: &clientPool{conns: map[string]*grpc.ClientConn{addr: conn}},
	}
	registerBehavioralTools(s)
	return s, fake
}

// All 8 behavioral tools register under the behavioral group; default is on.
func TestBehavioralToolsRegister(t *testing.T) {
	s := &server{tools: make(map[string]*registeredTool), cfg: &MCPConfig{}}
	registerBehavioralTools(s)
	for _, name := range []string{
		"behavioral_resolve_context", "behavioral_check_action", "behavioral_record_signal",
		"behavioral_record_outcome", "behavioral_explain_principle", "behavioral_propose_principle",
		"behavioral_promote_principle", "behavioral_revoke_principle",
	} {
		if !s.hasTool(name) {
			t.Errorf("tool %q not registered", name)
		}
	}
	if !defaultConfig().ToolGroups.Behavioral {
		t.Error("behavioral tool group should default to true")
	}
}

// Schemas expose governance relations as first-class inputs (not hidden in metadata).
func TestBehavioralProposeSchemaIsFirstClass(t *testing.T) {
	s := &server{tools: make(map[string]*registeredTool), cfg: &MCPConfig{}}
	registerBehavioralTools(s)
	props := s.tools["behavioral_propose_principle"].def.InputSchema.Properties
	for _, k := range []string{"applies_when", "authorities", "required_evidence", "forbidden_moves", "recommended_behavior", "risk_level", "revocation_rule"} {
		if _, ok := props[k]; !ok {
			t.Errorf("propose schema missing first-class field %q", k)
		}
	}
	if _, ok := props["metadata"]; ok {
		t.Error("propose schema must not route governance through a metadata field")
	}
	req := s.tools["behavioral_propose_principle"].def.InputSchema.Required
	for _, k := range []string{"actor", "promotion_reason", "revocation_rule", "risk_level"} {
		if !containsStrT(req, k) {
			t.Errorf("propose schema should require %q", k)
		}
	}
}

// The governed operator loop works end-to-end through the tools.
func TestBehavioralOperatorLoop(t *testing.T) {
	s, fake := startFakeBehavioral(t)
	ctx := context.Background()
	base := map[string]interface{}{"project": "globular-services", "domain": "cluster_operator"}

	// 1. record signal
	r1, err := s.callTool(ctx, "behavioral_record_signal", mergeArgs(base, map[string]interface{}{"signal_kind": "OBSERVED_RUNTIME_FACT", "payload": "etcd NOSPACE"}))
	if err != nil {
		t.Fatalf("record_signal: %v", err)
	}
	if m := r1.(map[string]interface{}); m["signal_id"] != "sig-1" || m["canonical_uri"] != "behavioral:signal/sig-1" {
		t.Errorf("record_signal result = %+v", m)
	}
	if fake.lastSignal.GetKind() != behavioralpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT {
		t.Error("signal kind not passed through to RPC")
	}

	// 2. resolve context
	r2, err := s.callTool(ctx, "behavioral_resolve_context", mergeArgs(base, map[string]interface{}{"conditions": "condition.cluster.etcd.nospace_alarm"}))
	if err != nil {
		t.Fatalf("resolve_context: %v", err)
	}
	if m := r2.(map[string]interface{}); m["recommended_behavior"] == "" || m["confidence"] != "high" {
		t.Errorf("resolve_context result = %+v", m)
	}

	// 3. check action (blocked + allowed)
	blocked, _ := s.callTool(ctx, "behavioral_check_action", mergeArgs(base, map[string]interface{}{"action_type": "forbidden.cluster.restart_before_quorum_check"}))
	if m := blocked.(map[string]interface{}); m["status"] != "blocked" || m["allowed"] != false || m["action_check_id"] != "ac-1" {
		t.Errorf("check_action(forbidden) = %+v, want blocked", m)
	}
	allowed, _ := s.callTool(ctx, "behavioral_check_action", mergeArgs(base, map[string]interface{}{"action_type": "inspect"}))
	if m := allowed.(map[string]interface{}); m["status"] != "allowed" || m["allowed"] != true {
		t.Errorf("check_action(inspect) = %+v, want allowed", m)
	}

	// 4. record outcome
	r4, err := s.callTool(ctx, "behavioral_record_outcome", mergeArgs(base, map[string]interface{}{"status": "success", "theme": "etcd.nospace", "severe": false}))
	if err != nil {
		t.Fatalf("record_outcome: %v", err)
	}
	if m := r4.(map[string]interface{}); m["outcome_id"] != "out-1" || m["theme"] != "etcd.nospace" {
		t.Errorf("record_outcome result = %+v", m)
	}
	if fake.lastOutcome.GetStatus() != "success" {
		t.Error("outcome status not passed through to RPC")
	}
}

// propose returns PROPOSED_PRINCIPLE (never promoted).
func TestBehavioralProposeReturnsProposed(t *testing.T) {
	s, _ := startFakeBehavioral(t)
	res, err := s.callTool(context.Background(), "behavioral_propose_principle", map[string]interface{}{
		"project": "globular-services", "domain": "cluster_operator", "title": "t",
		"recommended_behavior": "do x", "risk_level": "low", "promotion_reason": "r", "revocation_rule": "rr", "actor": "dave",
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if m := res.(map[string]interface{}); m["status"] != "PROPOSED_PRINCIPLE" {
		t.Errorf("propose status = %v, want PROPOSED_PRINCIPLE", m["status"])
	}
}

// promote requires actor+reason and surfaces a BLOCKED decision (gate not bypassed/hidden).
func TestBehavioralPromoteRequiresActorAndSurfacesBlocked(t *testing.T) {
	s, _ := startFakeBehavioral(t)
	ctx := context.Background()
	if _, err := s.callTool(ctx, "behavioral_promote_principle", map[string]interface{}{
		"project": "globular-services", "domain": "cluster_operator", "principle_id": "p1",
	}); err == nil {
		t.Fatal("promote without actor/reason should error")
	}
	res, err := s.callTool(ctx, "behavioral_promote_principle", map[string]interface{}{
		"project": "globular-services", "domain": "cluster_operator", "principle_id": "p1", "actor": "dave", "reason": "repeated incidents",
	})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if m := res.(map[string]interface{}); m["decision"] != "PROMOTION_BLOCKED" {
		t.Errorf("promote decision = %v, want PROMOTION_BLOCKED (never hidden)", m["decision"])
	}
}

// Existing ai-memory MCP tools still register unchanged alongside behavioral tools.
func TestMemoryToolsUnchangedAlongsideBehavioral(t *testing.T) {
	s := &server{tools: make(map[string]*registeredTool), cfg: &MCPConfig{}}
	registerMemoryTools(s)
	registerBehavioralTools(s)
	for _, name := range []string{"memory_store", "memory_query", "memory_get", "session_save"} {
		if !s.hasTool(name) {
			t.Errorf("existing memory tool %q missing", name)
		}
	}
	if !s.hasTool("behavioral_check_action") {
		t.Error("behavioral tools should coexist with memory tools")
	}
}

func mergeArgs(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func containsStrT(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
