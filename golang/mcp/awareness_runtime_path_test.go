package main

import (
	"testing"
)

// awarenessRuntimeDBPath must always resolve to the same writable file at
// the awareness root, regardless of which installed/<version>/<uuid>/
// bundle the /current symlink currently points to. This stability is the
// whole point of putting it at the root: bundle reinstalls swap the
// symlink, the runtime database stays put.

func TestAwarenessRuntimeDBPath_StableUnderCurrentSymlink(t *testing.T) {
	got := awarenessRuntimeDBPath("/var/lib/globular/awareness/current/graph.db")
	want := "/var/lib/globular/awareness/runtime/runtime.db"
	if got != want {
		t.Errorf("awarenessRuntimeDBPath(current) = %q, want %q", got, want)
	}
}

func TestAwarenessRuntimeDBPath_StableUnderInstalledRevision(t *testing.T) {
	// Different installed revisions must all resolve to the same runtime
	// database — bundle versioning must not silo runtime data per version.
	v1 := awarenessRuntimeDBPath("/var/lib/globular/awareness/installed/1.2.44/abc-uuid/graph.db")
	v2 := awarenessRuntimeDBPath("/var/lib/globular/awareness/installed/1.2.99/xyz-uuid/graph.db")
	if v1 != v2 {
		t.Errorf("runtime path drifts between bundle versions: %q vs %q", v1, v2)
	}
	if v1 != "/var/lib/globular/awareness/runtime/runtime.db" {
		t.Errorf("runtime path = %q, want %q", v1, "/var/lib/globular/awareness/runtime/runtime.db")
	}
}

func TestAwarenessRuntimeDBPath_NotInsideBundle(t *testing.T) {
	// The runtime database MUST live outside any path that
	// isAwarenessBundlePath would classify as a bundle. Otherwise
	// OpenComposite would try to ATTACH the writable runtime as a
	// read-only bundle on the next restart.
	rt := awarenessRuntimeDBPath("/var/lib/globular/awareness/current/graph.db")
	if isAwarenessBundlePath(rt) {
		t.Errorf("runtime path %q must not be classified as bundle", rt)
	}
}
