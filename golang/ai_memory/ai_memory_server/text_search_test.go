package main

import "testing"

// TestMemoryMatchesAllTerms is the regression for the 2026-06-21 ai-memory bug:
// memory_query text_search matched the ENTIRE query as one contiguous substring,
// so any multi-word query returned zero results even when each term was present
// — silently masking the seeded ops.* corpus from semantic recall. The filter
// now requires every whitespace-separated term to appear in title+content (AND).
func TestMemoryMatchesAllTerms(t *testing.T) {
	const title = "RBAC permissive-fallback storm: callerIsAdmin ignored built-in sa superadmin"
	const content = "Service-to-service authz lookups failed; role binding read denied for caller sa."

	terms := func(words ...string) []string { return words } // already-lowercased

	cases := []struct {
		name  string
		terms []string
		want  bool
	}{
		// The exact failing case from the incident — multi-word, all terms present.
		{"multi-word all present", terms("rbac", "sa", "superadmin"), true},
		{"multi-word across title+content", terms("rbac", "authz", "binding"), true},
		{"single token (old behavior preserved)", terms("rbac"), true},
		{"empty terms match everything", nil, true},
		{"one term absent fails (AND)", terms("rbac", "minio"), false},
		{"contiguous-phrase no longer required", terms("superadmin", "callerisadmin"), true},
		{"absent token", terms("kubernetes"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := memoryMatchesAllTerms(title, content, c.terms); got != c.want {
				t.Errorf("memoryMatchesAllTerms(%v) = %v, want %v", c.terms, got, c.want)
			}
		})
	}
}
