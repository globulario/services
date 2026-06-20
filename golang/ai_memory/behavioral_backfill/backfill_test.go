package behavioral_backfill

import (
	"context"
	"errors"
	"strings"
	"testing"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

const (
	tProject = "globular-services"
	tDomain  = "cluster_operator"
)

// fakeSource returns a fixed memory set.
type fakeSource struct{ mems []*ai_memorypb.Memory }

func (f *fakeSource) Query(context.Context, MemoryFilter) ([]*ai_memorypb.Memory, error) {
	return f.mems, nil
}

func corpus() *fakeSource {
	return &fakeSource{mems: []*ai_memorypb.Memory{
		{Id: "m-debug", Type: ai_memorypb.MemoryType_DEBUG, Title: "DNS crash", Content: "badger corruption",
			Tags: []string{"dns"}, AgentId: "claude", ConversationId: "c1", CreatedAt: 100,
			Metadata: map[string]string{"root_cause": "unclean shutdown"}},
		{Id: "m-fb", Type: ai_memorypb.MemoryType_FEEDBACK, Title: "deploy worked", CreatedAt: 200,
			Tags: []string{"deploy"}, Metadata: map[string]string{"outcome": "success", "theme": "deploy"}},
		{Id: "m-fbv", Type: ai_memorypb.MemoryType_FEEDBACK, Title: "vague note", CreatedAt: 200, Metadata: map[string]string{}},
		{Id: "m-scratch", Type: ai_memorypb.MemoryType_SCRATCH, Title: "temp", CreatedAt: 50},
		{Id: "m-dec", Type: ai_memorypb.MemoryType_DECISION, Title: "preserve quorum", CreatedAt: 300,
			Metadata: map[string]string{
				"condition": "condition.cluster.etcd.nospace_alarm", "authority": "authority.cluster.etcd.member_health",
				"required_evidence": "evidence.cluster.etcd.alarm_status", "forbidden_move": "forbidden.cluster.restart_before_quorum_check",
				"recommended_behavior": "establish member health first", "promotion_reason": "incidents recurred",
				"revocation_rule": "narrow if etcd semantics change", "risk_level": "high"}},
		{Id: "m-decp", Type: ai_memorypb.MemoryType_DECISION, Title: "partial rule", CreatedAt: 300,
			Metadata: map[string]string{"condition": "cond.x", "risk_level": "low"}}, // missing authority/evidence/etc.
	}}
}

func run(t *testing.T, st store.Store, dryRun bool) *Report {
	t.Helper()
	rep, err := Run(context.Background(), corpus(), st, Options{Project: tProject, Domain: tDomain, DryRun: dryRun})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return rep
}

// Dry-run writes nothing and reports would-create counts + skip reasons.
func TestDryRunWritesNothing(t *testing.T) {
	st := store.NewMemoryStore()
	rep := run(t, st, true)
	if _, err := st.GetSignal(context.Background(), tProject, tDomain, signalID(corpus().mems[0])); !errors.Is(err, store.ErrNotFound) {
		t.Error("dry-run must not write a signal")
	}
	if rep.WouldCreate["signal"] == 0 {
		t.Error("dry-run report should count would-create signals")
	}
	if len(rep.Created) != 0 {
		t.Errorf("dry-run created rows: %v", rep.Created)
	}
	out := rep.String()
	if !strings.Contains(out, "memories scanned") || !strings.Contains(out, "DRY-RUN") {
		t.Errorf("report missing header/counts:\n%s", out)
	}
}

// Memory → signal deterministic mapping (+ provenance).
func TestMemoryToSignal(t *testing.T) {
	st := store.NewMemoryStore()
	run(t, st, false)
	s, err := st.GetSignal(context.Background(), tProject, tDomain, "signal.ai_memory.m-debug")
	if err != nil {
		t.Fatalf("signal not created: %v", err)
	}
	if s.Kind != api.SignalHistoricalMemory || s.SourceKind != "ai_memory" || s.SourceRef != "m-debug" {
		t.Errorf("signal fields = %+v", s)
	}
	if s.Status != api.StatusRawSignal {
		t.Errorf("signal status = %q, want RAW_SIGNAL", s.Status)
	}
	if s.Provenance.MemoryID != "m-debug" {
		t.Error("memory_id provenance not preserved")
	}
	if !strings.Contains(s.Metadata["source_refs"], "ai-memory:m-debug") || !strings.Contains(s.Metadata["source_refs"], "ai-memory:conversation:c1") {
		t.Errorf("source_refs not preserved: %q", s.Metadata["source_refs"])
	}
	if s.Metadata["generated_from"] != "ai-memory:m-debug" {
		t.Errorf("generated_from not preserved: %q", s.Metadata["generated_from"])
	}
}

// metadata root_cause → claim candidate.
func TestMetadataRootCauseToClaim(t *testing.T) {
	st := store.NewMemoryStore()
	run(t, st, false)
	c, err := st.GetClaim(context.Background(), tProject, tDomain, "claim.ai_memory.m-debug.root_cause")
	if err != nil {
		t.Fatalf("claim not created: %v", err)
	}
	if c.Predicate != "root_cause" || c.Statement != "unclean shutdown" || c.Status != api.StatusCandidateFact {
		t.Errorf("claim fields = %+v", c)
	}
	if c.SignalID != "signal.ai_memory.m-debug" {
		t.Errorf("claim not linked to signal: %q", c.SignalID)
	}
}

// Feedback → outcome only when explicit; vague feedback skipped with reason.
func TestFeedbackToOutcomeOnlyWhenExplicit(t *testing.T) {
	st := store.NewMemoryStore()
	rep := run(t, st, false)
	o, err := st.GetOutcome(context.Background(), tProject, tDomain, "outcome.ai_memory.m-fb")
	if err != nil {
		t.Fatalf("explicit-success outcome not created: %v", err)
	}
	if o.Status != "success" || o.Theme != "deploy" {
		t.Errorf("outcome fields = %+v", o)
	}
	// vague feedback → no outcome + skip reason
	if _, err := st.GetOutcome(context.Background(), tProject, tDomain, "outcome.ai_memory.m-fbv"); !errors.Is(err, store.ErrNotFound) {
		t.Error("vague feedback must not produce an outcome")
	}
	if rep.SkipReasons["feedback without explicit outcome status"] == 0 {
		t.Error("vague feedback should be skipped with a reason")
	}
}

// Ambiguous / non-operational memory is skipped with a reason.
func TestNonOperationalSkipped(t *testing.T) {
	st := store.NewMemoryStore()
	rep := run(t, st, false)
	if rep.SkipReasons["non-operational memory type"] == 0 {
		t.Error("SCRATCH memory should be skipped with a reason")
	}
	if _, err := st.GetSignal(context.Background(), tProject, tDomain, "signal.ai_memory.m-scratch"); !errors.Is(err, store.ErrNotFound) {
		t.Error("non-operational memory must not produce a signal")
	}
}

// Principle is created ONLY when every governance field exists, as PROPOSED;
// partial candidate is reported, never created.
func TestPrincipleOnlyWhenComplete(t *testing.T) {
	st := store.NewMemoryStore()
	rep := run(t, st, false)
	p, err := st.GetPrinciple(context.Background(), tProject, tDomain, "principle.ai_memory.m-dec")
	if err != nil {
		t.Fatalf("complete decision did not produce a principle: %v", err)
	}
	if p.Status != api.StatusProposedPrinciple {
		t.Fatalf("backfilled principle status = %q, want PROPOSED_PRINCIPLE", p.Status)
	}
	if len(p.AppliesWhen) == 0 || len(p.Authorities) == 0 || len(p.RequiredEvidence) == 0 || p.RiskLevel != "high" {
		t.Errorf("principle governance fields not mapped: %+v", p)
	}
	if len(p.SourceRefs) == 0 || len(p.GeneratedFrom) == 0 || p.ProposedBy == "" {
		t.Errorf("principle lineage/provenance missing: %+v", p)
	}
	// partial decision → no principle, reported as a gap
	if _, err := st.GetPrinciple(context.Background(), tProject, tDomain, "principle.ai_memory.m-decp"); !errors.Is(err, store.ErrNotFound) {
		t.Error("partial decision must not become a principle")
	}
	foundGap := false
	for _, g := range rep.MissingFields {
		if g.MemoryID == "m-decp" && len(g.Missing) > 0 {
			foundGap = true
		}
	}
	if !foundGap {
		t.Error("partial decision should be reported as a principle gap with missing fields")
	}
}

// Backfill NEVER creates a PROMOTED/REVOKED principle.
func TestNeverCreatesPromoted(t *testing.T) {
	st := store.NewMemoryStore()
	run(t, st, false)
	for _, m := range corpus().mems {
		p, err := st.GetPrinciple(context.Background(), tProject, tDomain, principleID(m))
		if err != nil {
			continue
		}
		if p.Status != api.StatusProposedPrinciple {
			t.Errorf("backfill created non-proposed principle %q status=%q", p.ID, p.Status)
		}
	}
}

// Backfill is idempotent: a second run writes nothing new.
func TestIdempotent(t *testing.T) {
	st := store.NewMemoryStore()
	run(t, st, false) // first
	rep2 := run(t, st, false)
	if len(rep2.Created) != 0 {
		t.Errorf("second run created rows: %v", rep2.Created)
	}
	if rep2.SkipReasons["signal exists (idempotent)"] == 0 {
		t.Error("second run should skip existing signals idempotently")
	}
}

// A backfilled PROPOSED principle that was later promoted must NOT be demoted by a re-run.
func TestReRunDoesNotDemotePromoted(t *testing.T) {
	st := store.NewMemoryStore()
	run(t, st, false)
	ctx := context.Background()
	// simulate promotion of the backfilled principle
	if err := st.UpdatePrincipleStatus(ctx, tProject, tDomain, "principle.ai_memory.m-dec", api.StatusPromotedPrinciple, 1); err != nil {
		t.Fatal(err)
	}
	rep := run(t, st, false)
	p, _ := st.GetPrinciple(ctx, tProject, tDomain, "principle.ai_memory.m-dec")
	if p.Status != api.StatusPromotedPrinciple {
		t.Errorf("re-run demoted a promoted principle to %q", p.Status)
	}
	if rep.SkipReasons["principle already governed — not overwritten"] == 0 {
		t.Error("re-run should report skipping a governed principle")
	}
}

// Scoping is mandatory.
func TestRequiresProjectAndDomain(t *testing.T) {
	if _, err := Run(context.Background(), corpus(), store.NewMemoryStore(), Options{Project: "", Domain: tDomain}); err == nil {
		t.Error("backfill without project must error")
	}
}
