package main

import (
	"testing"
)

// isAwarenessBundlePath is the gate that decides whether the MCP opens
// the graph read-only. Misclassification has direct user-visible
// consequences: classifying a writable runtime path as a bundle disables
// learn_from_fix and friends; classifying a bundle path as writable
// triggers the "attempt to write a readonly database" cascade this
// commit set out to fix.

func TestIsAwarenessBundlePath_CurrentSymlink(t *testing.T) {
	// /var/lib/globular/awareness/current is the active-bundle symlink.
	// Recognised by the lexical-prefix branch even when the link itself
	// is missing (dev workstations).
	if !isAwarenessBundlePath("/var/lib/globular/awareness/current/graph.db") {
		t.Error("'/var/lib/globular/awareness/current/graph.db' must be classified as a bundle path")
	}
}

func TestIsAwarenessBundlePath_InstalledTree(t *testing.T) {
	// The /current symlink resolves into /installed/<version>/<uuid>/.
	// A direct path under /installed/ must also be classified as a bundle.
	if !isAwarenessBundlePath(
		"/var/lib/globular/awareness/installed/1.2.44/abc-uuid/graph.db") {
		t.Error("'.../awareness/installed/...' must be classified as a bundle path")
	}
}

func TestIsAwarenessBundlePath_RuntimeWritableCopy(t *testing.T) {
	// The transitional workaround staged a writable copy at /runtime/.
	// That path is NOT a bundle — it's the writable runtime database —
	// so the helper must classify it as non-bundle (Open, not OpenReadOnly).
	if isAwarenessBundlePath("/var/lib/globular/awareness/runtime/graph.db") {
		t.Error("'/var/lib/globular/awareness/runtime/graph.db' must NOT be classified as a bundle path")
	}
}

func TestIsAwarenessBundlePath_DevCheckout(t *testing.T) {
	// Developer machines use <repoRoot>/.globular/awareness/graph.db.
	// Writable, not a signed bundle.
	if isAwarenessBundlePath("/home/dev/repo/.globular/awareness/graph.db") {
		t.Error("dev-checkout graph.db must NOT be classified as a bundle path")
	}
}

func TestIsAwarenessBundlePath_Empty(t *testing.T) {
	if isAwarenessBundlePath("") {
		t.Error("empty path must not match")
	}
}
