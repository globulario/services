package main

import "testing"

// TestClassifyGhostCleanup is the regression guard for the destructive
// ghost-package cleanup path: an empty-but-successful node registry read is
// UNKNOWN authority, not permission to erase a node's Layer-3 records.
func TestClassifyGhostCleanup(t *testing.T) {
	cases := []struct {
		name       string
		registered []string
		target     string
		want       ghostCleanupDecision
	}{
		{
			name:       "empty registry (empty success) — REFUSE, cannot classify ghost",
			registered: nil,
			target:     "node-1",
			want:       ghostCleanupRefuseEmptyRegistry,
		},
		{
			name:       "empty registry (zero-length slice) — REFUSE",
			registered: []string{},
			target:     "node-1",
			want:       ghostCleanupRefuseEmptyRegistry,
		},
		{
			name:       "target IS a listed member — REFUSE active node",
			registered: []string{"node-1", "node-2", "node-3"},
			target:     "node-1",
			want:       ghostCleanupRefuseActiveMember,
		},
		{
			name:       "target NOT in a non-empty registry — ALLOW cleanup",
			registered: []string{"node-2", "node-3"},
			target:     "node-1",
			want:       ghostCleanupAllow,
		},
		{
			name:       "target matches after whitespace trim — REFUSE active node",
			registered: []string{" node-1 ", "node-2"},
			target:     "node-1",
			want:       ghostCleanupRefuseActiveMember,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyGhostCleanup(tc.registered, tc.target); got != tc.want {
				t.Fatalf("classifyGhostCleanup(%v, %q) = %d, want %d", tc.registered, tc.target, got, tc.want)
			}
		})
	}
}
