package main

import "testing"

// TestIsSyntheticReleaseName locks in the contract the patch helpers
// depend on: any release name emitted by cluster.reconcile's drift
// dispatch (reconcile_actions.go:371-372) is treated as synthetic and
// its status patches become no-ops. The prefix "reconcile-" is the
// unique marker — real releases are named after their package.
//
// If this predicate drifts (e.g. a new reconcile path uses a different
// prefix, or a real release accidentally gets a "reconcile-" name),
// this test fails loudly instead of the status patches silently
// succeeding on a release that actually existed.
func TestIsSyntheticReleaseName(t *testing.T) {
	cases := map[string]bool{
		// Synthetic — dispatched by cluster.reconcile drift loop.
		"reconcile-cluster-controller": true,
		"reconcile-etcd":               true,
		"reconcile-scylladb":           true,
		// Real releases — package-named, persisted in etcd.
		"cluster-controller": false,
		"etcd":               false,
		"scylladb":           false,
		"":                   false,
		// Edge: name that merely contains "reconcile" mid-string.
		"pkg-reconcile":         false,
		"reconcilable-service":  false,
	}
	for in, want := range cases {
		if got := isSyntheticReleaseName(in); got != want {
			t.Errorf("isSyntheticReleaseName(%q) = %v, want %v", in, got, want)
		}
	}
}
