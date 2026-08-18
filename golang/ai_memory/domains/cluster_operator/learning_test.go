package cluster_operator

import (
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
)

// TestLearningTemplatesResolve is the guard the doctor's draftForInvariant
// table could not have. That table named authority/condition/evidence refs
// directly, and nothing checked they existed — an unresolvable ref produced no
// error, just a review queue that silently stopped filling, which reads exactly
// like a healthy cluster.
//
// Naming a SEED id instead makes the claim checkable: this test fails the
// moment a named seed is renamed or removed, before the drift can become an
// invisible learning outage.
func TestLearningTemplatesResolve(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	seeds := map[string]bool{}
	for _, s := range p.Catalogs().Principles {
		seeds[s.ID] = true
	}
	for _, id := range LearningTemplateSeedIDs() {
		if !seeds[id] {
			t.Errorf("learning template names seed %q, which does not exist in this pack's "+
				"catalogs — the template would silently produce no candidate", id)
		}
	}
}

// TestLearningTemplateOnlyDisambiguates keeps this file from growing back into
// the table it replaced. A condition claimed by exactly one seed needs no entry;
// listing it would recreate the duplication — a second place stating what the
// catalogs already say, free to drift from them.
func TestLearningTemplateOnlyDisambiguates(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	cats := p.Catalogs()
	for condition := range learningTemplateByCondition {
		n := 0
		for _, s := range cats.Principles {
			for _, c := range s.AppliesWhen {
				if c == condition {
					n++
					break
				}
			}
		}
		if n < 2 {
			t.Errorf("condition %q is claimed by %d seed(s); generic matching already "+
				"resolves it, so this entry is redundant duplication rather than "+
				"disambiguation", condition, n)
		}
	}
}

// TestDoctorInvariantConditionsYieldTemplates pins the behaviour the doctor
// depends on: both invariants it maps must resolve to a domain-authored draft.
// If either stops resolving, the doctor silently queues nothing for it.
func TestDoctorInvariantConditionsYieldTemplates(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	for _, condition := range []string{
		"condition.cluster.node.systemd_unit_not_running",
		"condition.cluster.service.desired_observed_mismatch",
	} {
		seed, ok := domain.CandidateTemplateFor(p, domain.LearningObservation{
			Conditions: []string{condition},
		})
		if !ok {
			t.Errorf("condition %q yields no template; the doctor would stop queueing "+
				"candidates for the invariant that maps to it", condition)
			continue
		}
		// A draft with no authority cannot be reviewed against anything.
		if len(seed.Authorities) == 0 {
			t.Errorf("template %q for %q declares no authority", seed.ID, condition)
		}
		if seed.Title == "" {
			t.Errorf("template %q for %q has no title", seed.ID, condition)
		}
	}
}

// TestTemplateAuthorityComesFromThePack records the drift this change removed.
// The doctor's table claimed authority.cluster.node_agent.runtime_state for the
// restart rule; the pack's seed claims authority.cluster.owner_service.runtime_state.
// Two hand-maintained copies of one governance claim, disagreeing. Now there is
// one copy, and it is the pack's.
func TestTemplateAuthorityComesFromThePack(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	seed, ok := domain.CandidateTemplateFor(p, domain.LearningObservation{
		Conditions: []string{"condition.cluster.node.systemd_unit_not_running"},
	})
	if !ok {
		t.Fatal("no template for the systemd-unit condition")
	}
	var found bool
	for _, a := range seed.Authorities {
		if a == "authority.cluster.owner_service.runtime_state" {
			found = true
		}
	}
	if !found {
		t.Errorf("template authorities = %v; expected the pack's own "+
			"authority.cluster.owner_service.runtime_state, not a doctor-side restatement",
			seed.Authorities)
	}
}

// TestScyllaWipeActionResolvesItsSeed proves the forbidden-move alias path
// works against the REAL pack, not a synthetic one.
//
// This test exists because the first version of the matcher read a field name
// no pack uses ("action"/"action_type"), and the accompanying comment asserted
// that no pack declared aliases at all. Both were wrong: gap 4 already shipped
// forbidden.cluster.raw_scylla_data_wipe carrying
// `action_aliases: scylla.data.wipe, ...`. The matcher silently scored zero and
// looked deliberate.
//
// A capability claimed in a comment and contradicted by the data is worse than
// an absent one — nothing fails, so nobody looks.
func TestScyllaWipeActionResolvesItsSeed(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	seed, ok := domain.CandidateTemplateFor(p, domain.LearningObservation{
		ActionType: "scylla.data.wipe",
	})
	if !ok {
		t.Fatal("the declared action alias scylla.data.wipe resolved no template; " +
			"the forbidden-move matching path is not reaching action_aliases")
	}
	if seed.ID != "principle.cluster.no_raw_scylla_data_wipe" {
		t.Errorf("matched %q, want principle.cluster.no_raw_scylla_data_wipe", seed.ID)
	}
	if seed.RiskLevel != "irreversible" && seed.RiskLevel != "high" {
		t.Errorf("risk_level = %q; a raw data wipe must carry irreversible/high risk",
			seed.RiskLevel)
	}
}

// TestDeclaredAliasesAreReachable is the general form: every action alias any
// forbidden move declares must resolve to some seed template. An alias that
// resolves to nothing is a governance statement with no reachable rule behind
// it — the pack says "this action is forbidden" and the learner cannot find the
// principle that says so.
func TestDeclaredAliasesAreReachable(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	cats := p.Catalogs()
	// A forbidden move is only reachable through a seed that references it.
	referenced := map[string]bool{}
	for _, s := range cats.Principles {
		for _, fm := range s.ForbiddenMoves {
			referenced[fm] = true
		}
	}
	for _, fm := range cats.ForbiddenMoves {
		aliases := domain.ForbiddenMoveAliases(fm)
		if len(aliases) == 0 || !referenced[fm.ID] {
			continue
		}
		for _, alias := range aliases {
			if _, ok := domain.CandidateTemplateFor(p, domain.LearningObservation{ActionType: alias}); !ok {
				t.Errorf("forbidden move %q declares alias %q, but no seed template resolves "+
					"for it — the action is declared forbidden with no reachable rule",
					fm.ID, alias)
			}
		}
	}
}
