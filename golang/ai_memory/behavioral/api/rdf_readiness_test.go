package api

import (
	"reflect"
	"testing"
)

// These tests are the RDF-readiness guard required by the design constraint:
// even though RDF/Ontology projection is deferred (plan §18, PR-7), the model
// must be projectable WITHOUT redesign from PR-1 onward. They fail loudly if a
// future change removes a stable id, drops project/domain scope, or demotes a
// governance relation into metadata-only.

// entityTypes is the closed set of first-class behavioral entities, paired with
// the URI kind each projects to. Keep in sync with AllEntityKinds.
var entityTypes = []struct {
	kind EntityKind
	typ  reflect.Type
}{
	{KindSignal, reflect.TypeOf(Signal{})},
	{KindClaim, reflect.TypeOf(Claim{})},
	{KindEvidence, reflect.TypeOf(Evidence{})},
	{KindAuthority, reflect.TypeOf(Authority{})},
	{KindCondition, reflect.TypeOf(Condition{})},
	{KindContradiction, reflect.TypeOf(Contradiction{})},
	{KindPrinciple, reflect.TypeOf(Principle{})},
	{KindForbiddenMove, reflect.TypeOf(ForbiddenMove{})},
	{KindRequiredEvidence, reflect.TypeOf(RequiredEvidence{})},
	{KindOutcome, reflect.TypeOf(Outcome{})},
	{KindPromotionDecision, reflect.TypeOf(PromotionDecisionRecord{})},
	{KindRevocationRule, reflect.TypeOf(RevocationRule{})},
	{KindActionCheck, reflect.TypeOf(ActionCheck{})},
}

func hasStringField(t reflect.Type, name string) bool {
	f, ok := t.FieldByName(name)
	return ok && f.Type.Kind() == reflect.String
}

// Every first-class entity must carry a stable canonical id and project/domain
// scope — the prerequisites for becoming an RDF URI with a scope.
func TestRDFReadiness_StableIDAndScope(t *testing.T) {
	if len(entityTypes) != len(AllEntityKinds) {
		t.Fatalf("entityTypes (%d) and AllEntityKinds (%d) are out of sync", len(entityTypes), len(AllEntityKinds))
	}
	for _, e := range entityTypes {
		for _, field := range []string{"ID", "Project", "Domain"} {
			if !hasStringField(e.typ, field) {
				t.Errorf("%s: missing string-kind field %q (required for RDF URI + scope)", e.typ.Name(), field)
			}
		}
	}
}

// Governance relations must be first-class fields, never hidden in metadata.
// This is the structural enforcement of design rule #2.
func TestRDFReadiness_RelationsAreFirstClass(t *testing.T) {
	required := map[reflect.Type][]string{
		reflect.TypeOf(Claim{}):         {"SignalID"},
		reflect.TypeOf(Evidence{}):      {"TargetID", "ObservedFrom", "Satisfies"},
		reflect.TypeOf(Authority{}):     {"GovernsRefs"},
		reflect.TypeOf(Principle{}):     {"AppliesWhen", "Authorities", "RequiredEvidence", "ForbiddenMoves", "PromotionDecisionID", "RevocationRuleID", "SupersededBy", "NarrowedBy", "SourceRefs", "GeneratedFrom"},
		reflect.TypeOf(Contradiction{}): {"LeftRef", "RightRef"},
		reflect.TypeOf(Outcome{}):       {"ActionCheckID", "SupportsPrinciples", "WeakensPrinciples"},
		reflect.TypeOf(ActionCheck{}):   {"CheckedAgainstPrinciples", "ForbiddenMatched", "MissingEvidence"},
	}
	for typ, fields := range required {
		for _, name := range fields {
			if _, ok := typ.FieldByName(name); !ok {
				t.Errorf("%s: governance relation %q must be a first-class field, not metadata-only", typ.Name(), name)
			}
		}
	}
}

// Every entity type must expose a Metadata map — the extension hatch — so callers
// have somewhere for non-semantic extras without polluting the typed relations.
func TestRDFReadiness_MetadataHatchPresent(t *testing.T) {
	for _, e := range entityTypes {
		f, ok := e.typ.FieldByName("Metadata")
		if !ok || f.Type.Kind() != reflect.Map {
			t.Errorf("%s: missing Metadata map extension hatch", e.typ.Name())
		}
	}
}

// The Scylla id is the semantic identity: the canonical URI derives directly from
// it, and no synthetic identity is invented for an empty id.
func TestRDFReadiness_CanonicalURI(t *testing.T) {
	if got, want := CanonicalURI(KindPrinciple, "abc"), "behavioral:principle/abc"; got != want {
		t.Errorf("CanonicalURI = %q, want %q", got, want)
	}
	if got := CanonicalURI(KindSignal, ""); got != "" {
		t.Errorf("CanonicalURI with empty id = %q, want empty (no synthetic identity)", got)
	}
}
