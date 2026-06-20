package rdf

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

func fixtureBundle() *Bundle {
	return &Bundle{
		Signals: []api.Signal{{ID: "s1", Project: "P", Domain: "D", Kind: api.SignalObservedRuntimeFact,
			Status: api.StatusRawSignal, SourceRef: "probe", Provenance: api.Provenance{MemoryID: "m1"},
			Metadata: map[string]string{"source_refs": "ai-memory:m1"}}},
		Claims: []api.Claim{{ID: "c1", Project: "P", Domain: "D", SignalID: "s1", Status: api.StatusExtractedClaim,
			SubjectEntity: "etcd", Predicate: "alarm", ObjectValue: "NOSPACE"}},
		Evidence: []api.Evidence{{ID: "e1", Project: "P", Domain: "D", TargetKind: "claim", TargetID: "c1",
			Lane: api.LaneRuntimeRequired, ObservedFrom: "s1", Satisfies: []api.RequiredEvidenceRef{"req.a"}}},
		Authorities: []api.Authority{{ID: "auth.x", Project: "P", Domain: "D", Title: "etcd auth", GovernsRefs: []string{"behavioral:claim/c1"}}},
		Conditions:  []api.Condition{{ID: "cond.x", Project: "P", Domain: "D", Title: "nospace"}},
		Contradictions: []api.Contradiction{{ID: "con1", Project: "P", Domain: "D", Kind: "claim_vs_claim", Resolution: "open", LeftRef: "c1", RightRef: "c2"}},
		Principles: []api.Principle{{ID: "p1", Project: "P", Domain: "D", Title: "preserve quorum", Status: api.StatusPromotedPrinciple,
			RiskLevel: "high", AppliesWhen: []api.ConditionRef{"cond.x"}, Authorities: []api.AuthorityRef{"auth.x"},
			RequiredEvidence: []api.RequiredEvidenceRef{"req.a"}, ForbiddenMoves: []api.ForbiddenMoveRef{"forbid.y"},
			PromotionDecisionID: "d1", SourceRefs: []string{"ai-memory:m1"}, GeneratedFrom: []string{"opsknowledge:runbook.z"}}},
		PromotionDecisions: []api.PromotionDecisionRecord{{ID: "d1", Project: "P", Domain: "D", PrincipleID: "p1", Decision: api.PromotionAllowed, Verdict: "ok"}},
		RevocationRules:    []api.RevocationRule{{ID: "r1", Project: "P", Domain: "D", PrincipleID: "p1", Action: "REVOKED"}},
		ActionChecks: []api.ActionCheck{{ID: "ac1", Project: "P", Domain: "D", Status: "blocked", CheckedAgainstPrinciples: []string{"p1"},
			ForbiddenMatched: []api.ForbiddenMoveRef{"forbid.y"}, MissingEvidence: []api.RequiredEvidenceRef{"req.a"}}},
		Outcomes: []api.Outcome{{ID: "o1", Project: "P", Domain: "D", ActionCheckID: "ac1", Status: "failure", Theme: "etcd",
			SupportsPrinciples: []string{"p1"}, WeakensPrinciples: []string{"p2"}}},
	}
}

func projectStr(t *testing.T) string {
	t.Helper()
	return string(Project(fixtureBundle()))
}

func hasTriple(out, s, p, o string) bool { return strings.Contains(out, s+" "+p+" "+o+" .") }

func wantTriple(t *testing.T, out, s, p, o string) {
	t.Helper()
	if !hasTriple(out, s, p, o) {
		t.Errorf("missing triple:\n  %s %s %s .", s, p, o)
	}
}

// Every entity type projects to a stable URI subject with the right class.
func TestStableURIForEveryEntity(t *testing.T) {
	out := projectStr(t)
	cases := []struct {
		kind api.EntityKind
		id   string
		cls  string
	}{
		{api.KindSignal, "s1", ClassSignal}, {api.KindClaim, "c1", ClassClaim}, {api.KindEvidence, "e1", ClassEvidence},
		{api.KindAuthority, "auth.x", ClassAuthority}, {api.KindCondition, "cond.x", ClassCondition},
		{api.KindContradiction, "con1", ClassContradiction}, {api.KindPrinciple, "p1", ClassPrinciple},
		{api.KindPromotionDecision, "d1", ClassPromotionDecision}, {api.KindRevocationRule, "r1", ClassRevocationRule},
		{api.KindActionCheck, "ac1", ClassActionCheck}, {api.KindOutcome, "o1", ClassOutcome},
	}
	for _, c := range cases {
		wantTriple(t, out, instanceIRI(c.kind, c.id), iri(rdfType), classIRI(c.cls))
	}
}

func TestSignalClaimEvidenceProjection(t *testing.T) {
	out := projectStr(t)
	sig, claim, ev := instanceIRI(api.KindSignal, "s1"), instanceIRI(api.KindClaim, "c1"), instanceIRI(api.KindEvidence, "e1")
	// signal → claim (producesClaim) + claim → signal (derivedFromSignal)
	wantTriple(t, out, sig, predIRI(PredProducesClaim), claim)
	wantTriple(t, out, claim, predIRI(PredDerivedFromSignal), sig)
	// evidence supports claim (both directions)
	wantTriple(t, out, ev, predIRI(PredSupportsTarget), claim)
	wantTriple(t, out, claim, predIRI(PredSupportedBy), ev)
	// evidence observed from signal + satisfies required evidence
	wantTriple(t, out, ev, predIRI(PredObservedFrom), sig)
	wantTriple(t, out, ev, predIRI(PredSatisfies), instanceIRI(api.KindRequiredEvidence, "req.a"))
	// AWG-compatible type on runtime evidence
	wantTriple(t, out, ev, iri(rdfType), awgIRI(AWGRuntimeEvidence))
}

func TestPrincipleProjection(t *testing.T) {
	out := projectStr(t)
	p := instanceIRI(api.KindPrinciple, "p1")
	wantTriple(t, out, p, predIRI(PredAppliesWhen), instanceIRI(api.KindCondition, "cond.x"))
	wantTriple(t, out, p, predIRI(PredGovernedBy), instanceIRI(api.KindAuthority, "auth.x"))
	wantTriple(t, out, p, predIRI(PredRequiresEvidence), instanceIRI(api.KindRequiredEvidence, "req.a"))
	wantTriple(t, out, p, predIRI(PredForbidsMove), instanceIRI(api.KindForbiddenMove, "forbid.y"))
	wantTriple(t, out, p, predIRI(PredPromotedBy), instanceIRI(api.KindPromotionDecision, "d1"))
	wantTriple(t, out, p, predIRI(PredGovernanceStatus), literal("PROMOTED_PRINCIPLE"))
	// AWG-compatible forbidden-move type
	wantTriple(t, out, instanceIRI(api.KindForbiddenMove, "forbid.y"), iri(rdfType), awgIRI(AWGForbiddenRepairMove))
}

func TestPrincipleLineageProjection(t *testing.T) {
	out := projectStr(t)
	p := instanceIRI(api.KindPrinciple, "p1")
	// generated lineage → GeneratedPrinciple + OperationalKnowledgeSource
	wantTriple(t, out, p, iri(rdfType), classIRI(ClassGeneratedPrinciple))
	src := instanceIRI(kindOpsSource, "runbook.z")
	wantTriple(t, out, p, predIRI(PredGeneratedFrom), src)
	wantTriple(t, out, src, iri(rdfType), classIRI(ClassOperationalKnowledge))
	// backfilled lineage → backfilledFromMemory + sourceRef literal + BackfilledMemory
	mem := instanceIRI(kindBackfilledMemory, "m1")
	wantTriple(t, out, p, predIRI(PredBackfilledFromMemory), mem)
	wantTriple(t, out, mem, iri(rdfType), classIRI(ClassBackfilledMemory))
	wantTriple(t, out, p, predIRI(PredSourceRef), literal("ai-memory:m1"))
}

func TestSignalBackfillLineage(t *testing.T) {
	out := projectStr(t)
	sig := instanceIRI(api.KindSignal, "s1")
	wantTriple(t, out, sig, predIRI(PredBackfilledFromMemory), instanceIRI(kindBackfilledMemory, "m1"))
	wantTriple(t, out, sig, predIRI(PredSourceRef), literal("ai-memory:m1"))
}

func TestActionCheckAndOutcomeProjection(t *testing.T) {
	out := projectStr(t)
	ac, oc, p1 := instanceIRI(api.KindActionCheck, "ac1"), instanceIRI(api.KindOutcome, "o1"), instanceIRI(api.KindPrinciple, "p1")
	wantTriple(t, out, ac, predIRI(PredCheckedAgainst), p1)
	wantTriple(t, out, ac, predIRI(PredBlockedBy), instanceIRI(api.KindForbiddenMove, "forbid.y"))
	wantTriple(t, out, ac, predIRI(PredMissingEvidence), instanceIRI(api.KindRequiredEvidence, "req.a"))
	wantTriple(t, out, oc, predIRI(PredResultedFrom), ac)
	wantTriple(t, out, oc, predIRI(PredSupportsPrinciple), p1)
	wantTriple(t, out, oc, predIRI(PredWeakensPrinciple), instanceIRI(api.KindPrinciple, "p2"))
	wantTriple(t, out, oc, predIRI(PredGroupedByTheme), literal("etcd"))
	wantTriple(t, out, oc, iri(rdfType), awgIRI(AWGOutcomeFeedback))
}

func TestDeterministicAndNoDuplicates(t *testing.T) {
	a := Project(fixtureBundle())
	b := Project(fixtureBundle())
	if !bytes.Equal(a, b) {
		t.Error("projection is not deterministic")
	}
	lines := strings.Split(strings.TrimSpace(string(a)), "\n")
	seen := map[string]bool{}
	for _, l := range lines {
		if seen[l] {
			t.Errorf("duplicate triple: %s", l)
		}
		seen[l] = true
	}
	// sorted ascending
	for i := 1; i < len(lines); i++ {
		if lines[i] < lines[i-1] {
			t.Errorf("triples not sorted at %d", i)
		}
	}
}

// The projector cannot call the runtime decision RPCs: it imports neither
// behavioral/core (where CheckAction/ResolveGovernedContext live) nor any DB
// driver, and it never makes a CheckAction/ResolveGovernedContext CALL. We check
// imports + call expressions via the AST (so doc-comment mentions don't false-positive).
func TestProjectorDoesNotCallRuntime(t *testing.T) {
	fset := token.NewFileSet()
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, imp := range f.Imports {
			if strings.Contains(imp.Path.Value, "behavioral/core") || strings.Contains(imp.Path.Value, "gocql") {
				t.Errorf("%s imports %s — projector must stay pure (no runtime/driver)", e.Name(), imp.Path.Value)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && (sel.Sel.Name == "CheckAction" || sel.Sel.Name == "ResolveGovernedContext") {
				t.Errorf("%s calls %s — projection must not invoke a runtime decision RPC", e.Name(), sel.Sel.Name)
			}
			return true
		})
	}
	_ = filepath.Separator
}
