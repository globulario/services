package main

import "testing"

// TestIsOrphanedInstall pins the drift-reconciler's orphaned-install guard
// against the real compiled component catalog. An orphan is a package whose
// catalog profiles do not overlap the node's profiles — it can never converge
// there, so the drift-reconciler must suppress its dispatch (and cluster-doctor
// surfaces the operator-facing verdict).
func TestIsOrphanedInstall(t *testing.T) {
	// The live single-node test cluster's profiles.
	node := []string{"control-plane", "core", "storage"}

	cases := []struct {
		desc string
		pkg  string
		node []string
		want bool
	}{
		// torrent requires [media-server]; this node has none → orphan.
		{"torrent on non-media-server node is orphan", "torrent", node, true},
		// torrent is NO LONGER authorized by compute (moved to media-server) → orphan.
		{"torrent on compute node is now orphan", "torrent", []string{"compute"}, true},
		// torrent IS placeable on a media-server node → not an orphan.
		{"torrent on media-server node is not orphan", "torrent", []string{"media-server"}, false},
		// dns is placeable on core/control-plane → not an orphan.
		{"dns on this node is not orphan", "dns", node, false},
		// mcp is a control-plane service → placeable here.
		{"mcp on control-plane node is not orphan", "mcp", node, false},
		// gateway is control-plane/gateway → placeable (node has control-plane).
		{"gateway on control-plane node is not orphan", "gateway", node, false},
		// A package with NO catalog entry is NOT an orphan — "unknown to the
		// catalog" is a distinct condition and must not be swallowed here.
		{"unknown package is not classified as orphan", "does-not-exist-xyz", node, false},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if got := isOrphanedInstall(tc.pkg, tc.node); got != tc.want {
				t.Errorf("isOrphanedInstall(%q, %v) = %v, want %v", tc.pkg, tc.node, got, tc.want)
			}
		})
	}
}
