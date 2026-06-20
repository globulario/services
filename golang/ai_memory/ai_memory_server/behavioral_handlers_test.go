package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testProject = "globular-services"
	testDomain  = "cluster_operator"
)

// Both AiMemoryService and BehavioralMemoryService register on one gRPC server.
func TestBothServicesRegisterOnOneServer(t *testing.T) {
	gs := grpc.NewServer()
	ai_memorypb.RegisterAiMemoryServiceServer(gs, &server{})
	registerBehavioralService(gs, store.NewMemoryStore())

	info := gs.GetServiceInfo()
	for _, want := range []string{
		"ai_memory.AiMemoryService",
		"behavioral_memory.BehavioralMemoryService",
	} {
		if _, ok := info[want]; !ok {
			t.Errorf("service %q not registered; registered: %v", want, keys(info))
		}
	}
}

// As of PR-4 the kernel implements all 12 RPCs; none returns Unimplemented for a
// well-formed request. Runtime-RPC behavior is covered by behavioral_runtime_test.go.

// helper: record a signal via the handler, return its id.
func recordTestSignal(t *testing.T, h *behavioralHandler) string {
	t.Helper()
	resp, err := h.RecordSignal(context.Background(), &bpb.RecordSignalRequest{
		Signal: &bpb.Signal{
			Project: testProject, Domain: testDomain,
			Kind: bpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT, SourceKind: "probe", Payload: "etcd alarm NOSPACE",
		},
	})
	if err != nil {
		t.Fatalf("RecordSignal: %v", err)
	}
	return resp.GetSignalId()
}

// helper: extract one claim from a signal, return its id.
func extractTestClaim(t *testing.T, h *behavioralHandler, signalID string) string {
	t.Helper()
	resp, err := h.ExtractClaim(context.Background(), &bpb.ExtractClaimRequest{
		SignalId: signalID, Project: testProject, Domain: testDomain,
		Claims: []*bpb.Claim{{Statement: "etcd reported NOSPACE", SubjectEntity: "etcd", Predicate: "alarm", ObjectValue: "NOSPACE"}},
	})
	if err != nil {
		t.Fatalf("ExtractClaim: %v", err)
	}
	if len(resp.GetClaimIds()) != 1 {
		t.Fatalf("ExtractClaim: got %d claim ids, want 1", len(resp.GetClaimIds()))
	}
	return resp.GetClaimIds()[0]
}

// #1 RecordSignal persists a typed signal and returns its id/scope/status.
func TestRecordSignalPersists(t *testing.T) {
	st := store.NewMemoryStore()
	h := newBehavioralHandler(st)
	resp, err := h.RecordSignal(context.Background(), &bpb.RecordSignalRequest{
		Signal: &bpb.Signal{Project: testProject, Domain: testDomain, Kind: bpb.SignalKind_SIGNAL_HUMAN_CORRECTION, Payload: "operator note"},
	})
	if err != nil {
		t.Fatalf("RecordSignal: %v", err)
	}
	if resp.GetSignalId() == "" {
		t.Fatal("RecordSignal: empty signal id")
	}
	if resp.GetStatus() != bpb.GovernanceStatus_RAW_SIGNAL {
		t.Errorf("status = %v, want RAW_SIGNAL", resp.GetStatus())
	}
	got, err := st.GetSignal(context.Background(), testProject, testDomain, resp.GetSignalId())
	if err != nil {
		t.Fatalf("GetSignal: %v", err)
	}
	if got.Project != testProject || string(got.Domain) != testDomain {
		t.Errorf("scope mismatch: project=%q domain=%q", got.Project, got.Domain)
	}
	if got.Kind != api.SignalHumanCorrection {
		t.Errorf("kind = %q, want HUMAN_CORRECTION (typed, not collapsed)", got.Kind)
	}
	if got.Status != api.StatusRawSignal {
		t.Errorf("persisted status = %q, want RAW_SIGNAL", got.Status)
	}
}

// #2 ExtractClaim creates a claim linked to a signal at EXTRACTED_CLAIM.
func TestExtractClaimLinksToSignal(t *testing.T) {
	st := store.NewMemoryStore()
	h := newBehavioralHandler(st)
	sigID := recordTestSignal(t, h)
	claimID := extractTestClaim(t, h, sigID)

	got, err := st.GetClaim(context.Background(), testProject, testDomain, claimID)
	if err != nil {
		t.Fatalf("GetClaim: %v", err)
	}
	if got.SignalID != sigID {
		t.Errorf("claim.SignalID = %q, want %q (signal→claim link must be first-class)", got.SignalID, sigID)
	}
	if got.Status != api.StatusExtractedClaim {
		t.Errorf("claim status = %q, want EXTRACTED_CLAIM", got.Status)
	}
}

// ExtractClaim fails loud when the referenced signal does not exist.
func TestExtractClaimRequiresSignal(t *testing.T) {
	h := newBehavioralHandler(store.NewMemoryStore())
	_, err := h.ExtractClaim(context.Background(), &bpb.ExtractClaimRequest{
		SignalId: "missing", Project: testProject, Domain: testDomain,
		Claims: []*bpb.Claim{{Statement: "x"}},
	})
	if status.Code(err) == codes.OK {
		t.Fatal("ExtractClaim with missing signal should fail")
	}
}

// #3 + #4 RecordEvidence links evidence to a claim, advances it to
// EVIDENCE_LINKED, and maintains evidence_by_target.
func TestRecordEvidenceLinksTargetAndAdvancesClaim(t *testing.T) {
	st := store.NewMemoryStore()
	h := newBehavioralHandler(st)
	ctx := context.Background()
	sigID := recordTestSignal(t, h)
	claimID := extractTestClaim(t, h, sigID)

	resp, err := h.RecordEvidence(ctx, &bpb.RecordEvidenceRequest{
		Evidence: &bpb.Evidence{
			Project: testProject, Domain: testDomain, TargetKind: "claim", TargetId: claimID,
			EvidenceKind: "probe", Lane: bpb.EvidenceLaneMode_EVIDENCE_LANE_RUNTIME_REQUIRED, Result: "pass",
			ObservedFrom: sigID, Satisfies: []string{"required.etcd.alarm_status"},
		},
	})
	if err != nil {
		t.Fatalf("RecordEvidence: %v", err)
	}
	if resp.GetEvidenceId() == "" {
		t.Fatal("empty evidence id")
	}
	// claim advanced
	claim, _ := st.GetClaim(ctx, testProject, testDomain, claimID)
	if claim.Status != api.StatusEvidenceLinked {
		t.Errorf("claim status = %q, want EVIDENCE_LINKED", claim.Status)
	}
	// evidence_by_target maintained
	list, err := st.ListEvidenceForTarget(ctx, testProject, testDomain, claimID)
	if err != nil {
		t.Fatalf("ListEvidenceForTarget: %v", err)
	}
	if len(list) != 1 || list[0].ID != resp.GetEvidenceId() {
		t.Errorf("evidence_by_target = %+v, want one entry id=%q", list, resp.GetEvidenceId())
	}
}

// #5 MapAuthority records an authority→target mapping (no cluster-typed fields)
// and advances the claim to AUTHORITY_MAPPED.
func TestMapAuthorityRecordsMapping(t *testing.T) {
	st := store.NewMemoryStore()
	h := newBehavioralHandler(st)
	ctx := context.Background()
	sigID := recordTestSignal(t, h)
	claimID := extractTestClaim(t, h, sigID)

	authID := "authority.cluster.etcd.member_health"
	resp, err := h.MapAuthority(ctx, &bpb.MapAuthorityRequest{
		TargetKind: "claim", TargetId: claimID, Project: testProject, Domain: testDomain,
		AuthorityIds: []string{authID},
	})
	if err != nil {
		t.Fatalf("MapAuthority: %v", err)
	}
	if resp.GetStatus() != bpb.GovernanceStatus_AUTHORITY_MAPPED {
		t.Errorf("status = %v, want AUTHORITY_MAPPED", resp.GetStatus())
	}
	claim, _ := st.GetClaim(ctx, testProject, testDomain, claimID)
	if claim.Status != api.StatusAuthorityMapped {
		t.Errorf("claim status = %q, want AUTHORITY_MAPPED", claim.Status)
	}
	auth, err := st.GetAuthority(ctx, testProject, testDomain, authID)
	if err != nil {
		t.Fatalf("GetAuthority: %v", err)
	}
	want := api.CanonicalURI(api.KindClaim, claimID)
	if len(auth.GovernsRefs) != 1 || auth.GovernsRefs[0] != want {
		t.Errorf("authority.GovernsRefs = %v, want [%q]", auth.GovernsRefs, want)
	}
}

// #6 RecordContradiction records a contradiction between two claims and advances
// both to CONTRADICTION_TESTED.
func TestRecordContradictionBetweenClaims(t *testing.T) {
	st := store.NewMemoryStore()
	h := newBehavioralHandler(st)
	ctx := context.Background()
	sigID := recordTestSignal(t, h)
	left := extractTestClaim(t, h, sigID)
	right := extractTestClaim(t, h, sigID)

	resp, err := h.RecordContradiction(ctx, &bpb.RecordContradictionRequest{
		Contradiction: &bpb.Contradiction{
			Project: testProject, Domain: testDomain, Kind: "claim_vs_claim", LeftRef: left, RightRef: right,
		},
	})
	if err != nil {
		t.Fatalf("RecordContradiction: %v", err)
	}
	got, err := st.GetContradiction(ctx, testProject, testDomain, resp.GetContradictionId())
	if err != nil {
		t.Fatalf("GetContradiction: %v", err)
	}
	if got.LeftRef != left || got.RightRef != right {
		t.Errorf("contradiction refs = (%q,%q), want (%q,%q)", got.LeftRef, got.RightRef, left, right)
	}
	for _, id := range []string{left, right} {
		c, _ := st.GetClaim(ctx, testProject, testDomain, id)
		if c.Status != api.StatusContradictionTested {
			t.Errorf("claim %q status = %q, want CONTRADICTION_TESTED", id, c.Status)
		}
	}
}

// Ladder guard: PR-2 must reject promotion-tier statuses on ingest.
func TestRecordSignalRejectsForbiddenStatus(t *testing.T) {
	h := newBehavioralHandler(store.NewMemoryStore())
	_, err := h.RecordSignal(context.Background(), &bpb.RecordSignalRequest{
		Signal: &bpb.Signal{Project: testProject, Domain: testDomain, Status: bpb.GovernanceStatus_PROMOTED_PRINCIPLE},
	})
	if status.Code(err) == codes.OK {
		t.Fatal("RecordSignal with PROMOTED_PRINCIPLE status should be rejected in PR-2")
	}
}

// #11 No persistence/hot-path CQL uses ALLOW FILTERING. We inspect only string
// literals (where CQL lives) via the Go parser, so prose in comments mentioning
// the phrase does not produce a false positive.
func TestNoAllowFilteringInBehavioralStore(t *testing.T) {
	files := []string{
		"behavioral_schema.go",
		"../behavioral/store/scylla_store.go",
	}
	fset := token.NewFileSet()
	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if strings.Contains(strings.ToUpper(lit.Value), "ALLOW FILTERING") {
					t.Errorf("%s: CQL string literal contains ALLOW FILTERING — forbidden", f)
				}
			}
			return true
		})
	}
}

func keys(m map[string]grpc.ServiceInfo) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
