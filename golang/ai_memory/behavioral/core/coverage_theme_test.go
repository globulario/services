package core

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// TestCoverageThemeIsOrderInvariant — a theme is an identity computation, and
// identity must not depend on the order a caller happened to supply conditions.
// If it did, the same gap would scatter across as many themes as there are
// permutations and none would ever reach a repeat threshold.
func TestCoverageThemeIsOrderInvariant(t *testing.T) {
	a := coverageTheme("scylla.data.wipe", []api.ConditionRef{"cond.b", "cond.a", "cond.c"})
	b := coverageTheme("scylla.data.wipe", []api.ConditionRef{"cond.c", "cond.a", "cond.b"})
	if a != b {
		t.Errorf("theme depends on condition order: %q vs %q", a, b)
	}
}

// TestCoverageThemeIgnoresTarget is the reason the derivation excludes Target.
// The same ungoverned action against fifty hosts is ONE coverage gap. Including
// the target would split it fifty ways and the gap would never become
// reviewable — the system would stay silent precisely because the problem was
// widespread.
func TestCoverageThemeIgnoresTarget(t *testing.T) {
	conds := []api.ConditionRef{"condition.scylla.cluster_healthy"}
	if got, want := coverageTheme("scylla.data.wipe", conds), coverageTheme("scylla.data.wipe", conds); got != want {
		t.Fatalf("derivation is not deterministic: %q vs %q", got, want)
	}
	// Target is not an input at all — proven by the signature, pinned here so a
	// future refactor that adds it fails loudly.
	if strings.Contains(coverageTheme("scylla.data.wipe", conds), "globule-") {
		t.Error("theme leaked a host identity; one gap must not split per target")
	}
}

// TestCoverageThemeSeparatesDistinctShapes — different action types, and the
// same action under different conditions, are different gaps. Collapsing them
// would let evidence for one shape justify a rule about another.
func TestCoverageThemeSeparatesDistinctShapes(t *testing.T) {
	base := coverageTheme("scylla.data.wipe", []api.ConditionRef{"cond.a"})
	otherAction := coverageTheme("scylla.node.decommission", []api.ConditionRef{"cond.a"})
	otherConds := coverageTheme("scylla.data.wipe", []api.ConditionRef{"cond.b"})

	if base == otherAction {
		t.Error("different action types share a theme")
	}
	if base == otherConds {
		t.Error("different condition sets share a theme")
	}
}

// TestCoverageThemeIsMarkedAsDerived keeps a derived grouping key visibly
// distinct from a domain-authored outcome theme. A candidate built on coverage
// gaps must not be mistakable for one built on observed results.
func TestCoverageThemeIsMarkedAsDerived(t *testing.T) {
	got := coverageTheme("scylla.data.wipe", nil)
	if !strings.HasPrefix(got, CoverageThemePrefix) {
		t.Errorf("theme %q is not marked as derived (want prefix %q)", got, CoverageThemePrefix)
	}
}

// TestCoverageThemeEmptyActionHasNoTheme — an action type is the minimum
// identity. Without one there is nothing to group, and inventing a key would
// pool unrelated gaps under a single meaningless theme.
func TestCoverageThemeEmptyActionHasNoTheme(t *testing.T) {
	if got := coverageTheme("  ", []api.ConditionRef{"cond.a"}); got != "" {
		t.Errorf("got theme %q for an empty action type, want none", got)
	}
}

// TestCoverageThemeIgnoresBlankAndDuplicateConditions — normalization must not
// let cosmetic differences fork a theme.
func TestCoverageThemeIgnoresBlankAndDuplicateConditions(t *testing.T) {
	clean := coverageTheme("act", []api.ConditionRef{"cond.a", "cond.b"})
	noisy := coverageTheme("act", []api.ConditionRef{"cond.b", "", "cond.a", " ", "cond.a"})
	if clean != noisy {
		t.Errorf("blank/duplicate conditions forked the theme: %q vs %q", clean, noisy)
	}
}
