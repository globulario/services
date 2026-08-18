package store

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

const (
	gapProject = "globular-services"
	gapDomain  = "cluster_operator"
	gapTheme   = "coverage.scylla.data.wipe"
)

func ungovCheck(id string, governed bool) api.ActionCheck {
	return api.ActionCheck{
		ID: id, Project: gapProject, Domain: api.DomainRef(gapDomain),
		ActionType: "scylla.data.wipe", Target: "globule-ryzen",
		Theme: gapTheme, Governed: governed, Allowed: !governed, Status: "allowed",
		Conditions:  []api.ConditionRef{"condition.scylla.cluster_healthy"},
		Explanation: "ungoverned default-allow",
	}
}

// TestListUngovernedActionChecksByTheme_ReturnsHydratedChecks holds the same
// contract the outcomes-by-theme lookup learned the hard way on 2026-08-01: a
// theme index is an id list, not a projection. A caller deciding whether a
// coverage gap deserves review needs the whole verdict, so the lookup must
// hydrate from the base table rather than return what the index happens to
// carry.
func TestListUngovernedActionChecksByTheme_ReturnsHydratedChecks(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()

	for _, id := range []string{"check-1", "check-2"} {
		c := ungovCheck(id, false)
		if err := m.RecordActionCheck(ctx, &c); err != nil {
			t.Fatalf("record %s: %v", id, err)
		}
	}

	got, err := m.ListUngovernedActionChecksByTheme(ctx, gapProject, gapDomain, gapTheme)
	if err != nil {
		t.Fatalf("list ungoverned checks: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d checks, want 2", len(got))
	}
	for _, c := range got {
		if c.ActionType == "" {
			t.Errorf("check %s came back with no ActionType — a hollow record cannot "+
				"justify review, which is exactly how the outcomes lookup failed", c.ID)
		}
		if len(c.Conditions) == 0 {
			t.Errorf("check %s came back with no Conditions — the shape of the gap is "+
				"unrecoverable, so a reviewer cannot tell what went ungoverned", c.ID)
		}
	}
}

// TestListUngovernedActionChecksByTheme_ExcludesGoverned is the load-bearing
// exclusion. A check a promoted principle already reached is not a coverage
// gap. Counting it would inflate the support for creating a principle that
// already exists — the system would keep proposing rules for ground it already
// governs.
func TestListUngovernedActionChecksByTheme_ExcludesGoverned(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()

	ungoverned := ungovCheck("gap-1", false)
	governed := ungovCheck("governed-1", true)
	for _, c := range []*api.ActionCheck{&ungoverned, &governed} {
		if err := m.RecordActionCheck(ctx, c); err != nil {
			t.Fatalf("record %s: %v", c.ID, err)
		}
	}

	got, err := m.ListUngovernedActionChecksByTheme(ctx, gapProject, gapDomain, gapTheme)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ID != "gap-1" {
		t.Fatalf("got %+v, want only the ungoverned check", got)
	}
}

// TestRecordActionCheck_ReplayDoesNotDoubleCount pins idempotence. The same
// check id re-recorded is one observation, not two — otherwise a retry would
// inflate a theme toward its repeat threshold without anything new happening.
func TestRecordActionCheck_ReplayDoesNotDoubleCount(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()

	c := ungovCheck("check-1", false)
	for i := 0; i < 3; i++ {
		if err := m.RecordActionCheck(ctx, &c); err != nil {
			t.Fatalf("record attempt %d: %v", i, err)
		}
	}

	got, err := m.ListUngovernedActionChecksByTheme(ctx, gapProject, gapDomain, gapTheme)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d checks after 3 replays of one id, want 1 — a replayed write "+
			"is not a second observation", len(got))
	}
}

// TestListUngovernedActionChecksByTheme_UnknownThemeIsEmptyNotError — most
// themes never repeat; asking about one that has not occurred is normal.
func TestListUngovernedActionChecksByTheme_UnknownThemeIsEmptyNotError(t *testing.T) {
	got, err := NewMemoryStore().ListUngovernedActionChecksByTheme(
		context.Background(), gapProject, gapDomain, "coverage.never.happened")
	if err != nil {
		t.Fatalf("unknown theme must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d checks for an unrecorded theme, want 0", len(got))
	}
}

// TestListPrincipleSummaries_ReportsEveryStatus backs the enforcement summary.
// A scope with nothing promoted must be distinguishable from a scope nobody has
// asked about — principles_by_condition cannot do that, because it indexes only
// PROMOTED rules and only under the conditions they declare.
func TestListPrincipleSummaries_ReportsEveryStatus(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()

	for _, p := range []api.Principle{
		{ID: "p.promoted", Project: gapProject, Domain: api.DomainRef(gapDomain),
			Title: "Promoted", Status: api.StatusPromotedPrinciple, RiskLevel: "high"},
		{ID: "p.proposed", Project: gapProject, Domain: api.DomainRef(gapDomain),
			Title: "Proposed", Status: api.StatusProposedPrinciple, RiskLevel: "low"},
		{ID: "p.other_scope", Project: gapProject, Domain: api.DomainRef("other"),
			Title: "Elsewhere", Status: api.StatusPromotedPrinciple},
	} {
		cp := p
		if err := m.CreatePrinciple(ctx, &cp); err != nil {
			t.Fatalf("create %s: %v", p.ID, err)
		}
	}

	got, err := m.ListPrincipleSummaries(ctx, gapProject, gapDomain)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d summaries, want 2 (the other scope must not leak in): %+v", len(got), got)
	}
	var promoted int
	for _, s := range got {
		if s.Status == api.StatusPromotedPrinciple {
			promoted++
		}
		if s.Title == "" {
			t.Errorf("%s came back with no title", s.ID)
		}
	}
	if promoted != 1 {
		t.Errorf("counted %d promoted, want 1", promoted)
	}
}

// TestListPrincipleSummaries_EmptyScopeIsZeroNotError — "this scope has no
// principles" is the answer that makes NO ENFORCEMENT reportable. It must be a
// normal empty result, not a fault that the caller swallows into a zero.
func TestListPrincipleSummaries_EmptyScopeIsZeroNotError(t *testing.T) {
	got, err := NewMemoryStore().ListPrincipleSummaries(context.Background(), gapProject, gapDomain)
	if err != nil {
		t.Fatalf("empty scope must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d summaries for an empty scope, want 0", len(got))
	}
}
