package store

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// TestListOutcomesByTheme_ReturnsHydratedOutcomes locks the contract that broke
// promotion on 2026-08-01.
//
// outcomes_by_theme is an id index, not a projection. ScyllaStore used to scan
// it alone and return outcomes with empty EvidenceIDs. Nothing errored — the
// records just came back hollow — and GeneratePromotionCandidate, which derives
// a candidate's supporting evidence from its outcomes, then refused every
// candidate with "explicit supporting evidence is required". The promotion path
// could not work in any real cluster.
//
// It passed every unit test because MemoryStore hydrates. The test double was
// more capable than the store it stood in for, which is the specific way this
// class of bug hides: full coverage, green suite, structurally dead in prod.
//
// So the contract under test is not "Scylla works" (unreachable from a unit
// test) but "a theme lookup returns outcomes carrying the fields a caller needs
// to act on them". Any store claiming to implement it must satisfy this.
func TestListOutcomesByTheme_ReturnsHydratedOutcomes(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()

	const (
		project = "globular-services"
		domain  = "cluster_operator"
		theme   = "remediation.node.systemd.units_running"
	)

	want := []api.Outcome{
		{
			ID: "outcome-1", Project: project, Domain: api.DomainRef(domain),
			Theme: theme, Status: "success",
			ActionCheckID: "check-1",
			EvidenceIDs:   []string{"ev-1"},
			PrincipleIDs:  []string{"principle.cluster.restart_drifted_unit_with_observed_finding"},
			Note:          "restarted drifted unit after observed finding",
		},
		{
			ID: "outcome-2", Project: project, Domain: api.DomainRef(domain),
			Theme: theme, Status: "success",
			ActionCheckID: "check-2",
			EvidenceIDs:   []string{"ev-2"},
			Note:          "second repeat",
		},
	}
	for i := range want {
		if err := m.RecordOutcome(ctx, &want[i]); err != nil {
			t.Fatalf("record outcome %s: %v", want[i].ID, err)
		}
	}

	got, err := m.ListOutcomesByTheme(ctx, project, domain, theme)
	if err != nil {
		t.Fatalf("list outcomes by theme: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d outcomes, want %d", len(got), len(want))
	}

	byID := map[string]api.Outcome{}
	for _, o := range got {
		byID[o.ID] = o
	}
	for _, w := range want {
		g, ok := byID[w.ID]
		if !ok {
			t.Errorf("outcome %s missing from theme lookup", w.ID)
			continue
		}
		// The load-bearing field: without it a candidate has no supporting
		// evidence and is refused.
		if len(g.EvidenceIDs) == 0 {
			t.Errorf("outcome %s came back with no EvidenceIDs.\n"+
				"A theme lookup that drops evidence makes GeneratePromotionCandidate "+
				"structurally impossible — it derives supporting evidence from these outcomes.", w.ID)
		}
		if g.ActionCheckID == "" {
			t.Errorf("outcome %s came back with no ActionCheckID — the decision it "+
				"followed is unrecoverable, so the outcome cannot be attributed", w.ID)
		}
		if g.Status != w.Status {
			t.Errorf("outcome %s status = %q, want %q", w.ID, g.Status, w.Status)
		}
	}
}

// TestListOutcomesByTheme_UnknownThemeIsEmptyNotError verifies the common case.
// Most themes never repeat; asking about one that has not occurred is a normal
// question with a normal answer, not a fault.
func TestListOutcomesByTheme_UnknownThemeIsEmptyNotError(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()

	got, err := m.ListOutcomesByTheme(ctx, "globular-services", "cluster_operator", "remediation.never.happened")
	if err != nil {
		t.Fatalf("unknown theme must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d outcomes for an unrecorded theme, want 0", len(got))
	}
}
