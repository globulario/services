package domain

import "testing"

func seedCats() Catalogs {
	return Catalogs{
		ForbiddenMoves: []CatalogEntry{
			{ID: "forbidden.wipe", Title: "Wipe", Fields: map[string]string{"action": "scylla.data.wipe"}},
			{ID: "forbidden.other", Title: "Other", Fields: map[string]string{"reason": "because"}},
		},
		Principles: []PrincipleSeed{
			{ID: "seed.units", Title: "Restart drifted unit",
				AppliesWhen: []string{"cond.unit_not_running"},
				Authorities: []string{"auth.owner"}, RiskLevel: "low"},
			{ID: "seed.mismatch.a", Title: "No recovery claim",
				AppliesWhen: []string{"cond.mismatch"}, Authorities: []string{"auth.owner"}},
			{ID: "seed.mismatch.b", Title: "Preserve boundary",
				AppliesWhen: []string{"cond.mismatch"}, Authorities: []string{"auth.owner"}},
			{ID: "seed.wipe", Title: "Never wipe without health",
				AppliesWhen: []string{"cond.healthy"}, ForbiddenMoves: []string{"forbidden.wipe"}},
		},
	}
}

// TestMatchByConditionReturnsThePackSeed — the whole point of catalog matching:
// the draft is the pack's own seed, so its refs resolve by construction rather
// than by a second author remembering to keep a copy in sync.
func TestMatchByConditionReturnsThePackSeed(t *testing.T) {
	got, ok := MatchCandidateTemplate(seedCats(), LearningObservation{Conditions: []string{"cond.unit_not_running"}})
	if !ok {
		t.Fatal("a condition claimed by exactly one seed produced no template")
	}
	if got.ID != "seed.units" {
		t.Errorf("matched %q, want seed.units", got.ID)
	}
	if got.Authorities[0] != "auth.owner" {
		t.Errorf("authority = %q; the draft must carry the SEED's refs, not a restatement",
			got.Authorities[0])
	}
}

// TestAmbiguityYieldsNoTemplate is the safe direction. Two seeds claiming one
// condition means the pack has not said which governs it; choosing either would
// ask a human to approve a scope nobody established.
func TestAmbiguityYieldsNoTemplate(t *testing.T) {
	if _, ok := MatchCandidateTemplate(seedCats(), LearningObservation{Conditions: []string{"cond.mismatch"}}); ok {
		t.Error("a condition claimed by two seeds produced a template; the tie must refuse, not guess")
	}
}

// TestUnknownConditionYieldsNoTemplate — no hint is honest.
func TestUnknownConditionYieldsNoTemplate(t *testing.T) {
	if _, ok := MatchCandidateTemplate(seedCats(), LearningObservation{Conditions: []string{"cond.never.seen"}}); ok {
		t.Error("an unrecognized condition produced a template")
	}
}

// TestForbiddenMoveAliasMatchesAction proves the alias path works when a pack
// declares one. cluster_operator declares none today, which is why this uses a
// synthetic pack rather than pretending the live catalogs exercise it.
func TestForbiddenMoveAliasMatchesAction(t *testing.T) {
	got, ok := MatchCandidateTemplate(seedCats(), LearningObservation{
		ActionType: "scylla.data.wipe", Conditions: []string{"cond.healthy"},
	})
	if !ok {
		t.Fatal("an action naming a seed's forbidden move produced no template")
	}
	if got.ID != "seed.wipe" {
		t.Errorf("matched %q, want seed.wipe", got.ID)
	}
}

// TestMatchNeverMatchesOnNameResemblance — two ids sharing a prefix is a naming
// coincidence, not a governance statement. Matching on resemblance would make a
// rename silently change which rule gets proposed.
func TestMatchNeverMatchesOnNameResemblance(t *testing.T) {
	cats := Catalogs{Principles: []PrincipleSeed{
		{ID: "seed.x", AppliesWhen: []string{"condition.cluster.node.systemd_unit_not_running"}},
	}}
	obs := LearningObservation{Conditions: []string{"condition.cluster.node.systemd_unit_not_running.v2"}}
	if _, ok := MatchCandidateTemplate(cats, obs); ok {
		t.Error("matched a condition that merely shares a prefix with the seed's")
	}
}

// --- LearningSource precedence ---

type fakePack struct {
	cats   Catalogs
	seedID string
	has    bool
}

func (f fakePack) Name() string       { return "fake" }
func (f fakePack) Catalogs() Catalogs { return f.cats }
func (f fakePack) CandidateTemplate(LearningObservation) (string, bool) {
	return f.seedID, f.has
}

// TestLearningSourceBreaksAmbiguity — the reason the interface exists. The pack
// resolves a tie generic matching correctly refuses.
func TestLearningSourceBreaksAmbiguity(t *testing.T) {
	p := fakePack{cats: seedCats(), seedID: "seed.mismatch.a", has: true}
	got, ok := CandidateTemplateFor(p, LearningObservation{Conditions: []string{"cond.mismatch"}})
	if !ok {
		t.Fatal("pack named a seed but no template was returned")
	}
	if got.ID != "seed.mismatch.a" {
		t.Errorf("got %q, want the seed the pack named", got.ID)
	}
}

// TestUnresolvableSeedIDIsRefusedNotFallenBackOn is the load-bearing guard. A
// pack naming a seed it does not have is a defect IN THE PACK. Silently falling
// back to a catalog match would hide it behind a plausible candidate — the
// exact class of invisible failure the old doctor table could not detect.
func TestUnresolvableSeedIDIsRefusedNotFallenBackOn(t *testing.T) {
	p := fakePack{cats: seedCats(), seedID: "seed.does.not.exist", has: true}
	// cond.unit_not_running WOULD match seed.units if we fell back.
	if _, ok := CandidateTemplateFor(p, LearningObservation{Conditions: []string{"cond.unit_not_running"}}); ok {
		t.Error("an unresolvable seed id fell back to a catalog match instead of being refused")
	}
}

// TestPackWithoutTemplateFallsBackToMatching — LearningSource is optional, and
// declining one observation must not disable matching for it.
func TestPackWithoutTemplateFallsBackToMatching(t *testing.T) {
	p := fakePack{cats: seedCats(), has: false}
	got, ok := CandidateTemplateFor(p, LearningObservation{Conditions: []string{"cond.unit_not_running"}})
	if !ok || got.ID != "seed.units" {
		t.Errorf("got (%q,%v), want seed.units via catalog matching", got.ID, ok)
	}
}
