package core

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// CoverageThemePrefix marks a theme that was derived from an ungoverned action
// check rather than authored by a domain. It keeps a derived grouping key
// visibly distinct from a domain-authored outcome theme: a candidate built on
// coverage gaps must not be mistaken for one built on observed results.
const CoverageThemePrefix = "coverage."

// coverageTheme derives the stable grouping key for an action check.
//
// The key is a function of the action type and the DECLARED conditions only —
// never the target, the agent, or the timestamp. Two checks of the same shape
// against different targets are the same coverage gap; including the target
// would scatter one gap across as many themes as there are hosts and no gap
// would ever reach a repeat threshold.
//
// Conditions are sorted before hashing so an unordered condition set yields one
// theme: identity computation must not depend on the order a caller happened to
// supply. The hash is truncated for readability, not for security — this is a
// grouping key, not an authority ref, and it is never resolved through the
// domain registry.
func coverageTheme(actionType string, conditions []api.ConditionRef) string {
	action := strings.TrimSpace(actionType)
	if action == "" {
		return ""
	}
	refs := make([]string, 0, len(conditions))
	for _, c := range conditions {
		if trimmed := strings.TrimSpace(string(c)); trimmed != "" {
			refs = append(refs, trimmed)
		}
	}
	sort.Strings(refs)
	refs = dedupeSorted(refs)

	if len(refs) == 0 {
		return CoverageThemePrefix + action
	}
	sum := sha256.Sum256([]byte(action + "\x00" + strings.Join(refs, "\x00")))
	return CoverageThemePrefix + action + "." + hex.EncodeToString(sum[:])[:12]
}

// dedupeSorted removes adjacent duplicates from an already-sorted slice.
func dedupeSorted(in []string) []string {
	if len(in) < 2 {
		return in
	}
	out := in[:1]
	for _, v := range in[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}
