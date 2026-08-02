package api

import "testing"

// allAuthorityLevels is the complete declared enumeration. If a level is added
// to status.go without being ranked, TestAuthorityRank_EnumerationIsExhaustive
// fails — the point being that a NEW level must never silently become trusted
// (or silently become untrusted) by falling through a default branch.
var allAuthorityLevels = []ObservationAuthorityLevel{
	ObservationAuthorityInterpretation,
	ObservationAuthorityEventStream,
	ObservationAuthorityDiagnostic,
	ObservationAuthorityDerived,
	ObservationAuthorityTruthPlane,
}

func TestAuthorityRank_EveryDeclaredLevelHasAnExplicitRank(t *testing.T) {
	for _, l := range allAuthorityLevels {
		if _, ok := AuthorityRank(l); !ok {
			t.Errorf("authority %q has no explicit rank — it would fail closed everywhere, "+
				"including where it should be trusted", l)
		}
	}
}

// Guards against a level being added to the model but not to the rank table.
// A future level left unranked cannot satisfy any floor, so a real producer
// stamping it would be silently ungovernable.
func TestAuthorityRank_EnumerationIsExhaustive(t *testing.T) {
	if len(authorityRank) != len(allAuthorityLevels) {
		t.Fatalf("rank table has %d entries but %d levels are declared — "+
			"a level was added without a rank", len(authorityRank), len(allAuthorityLevels))
	}
}

// Ordering must be strictly monotonic in the declared trust order. A tie or an
// inversion would let a weaker authority satisfy a stronger floor.
func TestAuthorityRank_IsStrictlyMonotonic(t *testing.T) {
	prev := -1
	for _, l := range allAuthorityLevels {
		r, ok := AuthorityRank(l)
		if !ok {
			t.Fatalf("%q unranked", l)
		}
		if r <= prev {
			t.Fatalf("%q rank %d is not greater than the preceding %d — "+
				"ordering must be strictly increasing", l, r, prev)
		}
		prev = r
	}
}

func TestAuthorityRank_UnknownAndUnspecifiedAreNotRanked(t *testing.T) {
	for _, l := range []ObservationAuthorityLevel{
		ObservationAuthorityUnspecified,
		"",
		"SOMETHING_ADDED_LATER",
		"truth_plane", // case matters; near-misses must not rank
		"TRUTH_PLANE ",
	} {
		if _, ok := AuthorityRank(l); ok {
			t.Errorf("%q must not be rankable", l)
		}
	}
}

// The full floor matrix over the real enumeration.
func TestAuthorityAtLeast_Matrix(t *testing.T) {
	for i, have := range allAuthorityLevels {
		for j, floor := range allAuthorityLevels {
			want := i >= j // equal or higher satisfies
			if got := AuthorityAtLeast(have, floor); got != want {
				t.Errorf("AuthorityAtLeast(%q, %q) = %v, want %v", have, floor, got, want)
			}
		}
	}
}

// Fails closed on both sides. An unrankable FLOOR is never satisfiable — a
// requirement whose own floor cannot be interpreted must not be waived by
// default, which is what would happen if an unknown floor meant "no floor".
func TestAuthorityAtLeast_FailsClosed(t *testing.T) {
	unknown := ObservationAuthorityLevel("MYSTERY")

	if AuthorityAtLeast(unknown, ObservationAuthorityInterpretation) {
		t.Error("unrankable evidence authority must not satisfy even the lowest floor")
	}
	if AuthorityAtLeast(ObservationAuthorityTruthPlane, unknown) {
		t.Error("unrankable floor must not be satisfiable, not even by the highest authority")
	}
	if AuthorityAtLeast(ObservationAuthorityTruthPlane, ObservationAuthorityUnspecified) {
		t.Error("an unspecified floor is an unfinished catalog entry, not 'accept anything'")
	}
	if AuthorityAtLeast(ObservationAuthorityUnspecified, ObservationAuthorityUnspecified) {
		t.Error("unspecified must not satisfy unspecified — that would make blank floors self-satisfying")
	}
}

// The reason ranks exist at all: lexical comparison is nearly the reverse of
// the trust order, so a string compare would let INTERPRETATION outrank
// DERIVED_EVIDENCE and satisfy a requirement demanding authoritative proof.
func TestAuthorityOrder_IsNotLexical(t *testing.T) {
	lower, higher := ObservationAuthorityInterpretation, ObservationAuthorityDerived
	if !(string(lower) > string(higher)) {
		t.Skip("lexical relationship changed; this guard needs revisiting")
	}
	if AuthorityAtLeast(lower, higher) {
		t.Fatal("INTERPRETATION must not satisfy a DERIVED_EVIDENCE floor — " +
			"ordering has regressed to a string comparison")
	}
}
