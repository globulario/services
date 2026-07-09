package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
)

// TestServicePolicyDirPresent guards the build_id-skip policy precondition that
// fixes the v1.2.267 empty-resolver incident: a SERVICE installed out-of-band by
// the Day-0 globular-installer has no ActionPolicyDir/{name}/ directory (because
// install_payload — the sole policy deployer — never ran), so the idempotency
// skip must NOT short-circuit. servicePolicyDirPresent is the marker check that
// drives that decision. If this regresses, cluster-doctor's repository RPCs
// (GetRepositoryStatus / ListRepositoryFindings) go PermissionDenied again.
func TestServicePolicyDirPresent(t *testing.T) {
	tmp := t.TempDir()
	prev := actions.ActionPolicyDir
	actions.ActionPolicyDir = tmp
	t.Cleanup(func() { actions.ActionPolicyDir = prev })

	// Absent marker → not present → skip must fall through to reinstall.
	if servicePolicyDirPresent("repository") {
		t.Fatalf("expected policy dir absent for a service that never ran install_payload")
	}

	// install_payload creates the marker dir unconditionally (even for a
	// policy-less package). Once present, the skip is allowed to short-circuit.
	if err := os.MkdirAll(filepath.Join(tmp, "repository"), 0o755); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}
	if !servicePolicyDirPresent("repository") {
		t.Fatalf("expected policy dir present after install_payload marker was created")
	}

	// A regular file at the path is not a valid marker (must be a directory).
	if err := os.WriteFile(filepath.Join(tmp, "notadir"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if servicePolicyDirPresent("notadir") {
		t.Fatalf("a non-directory must not count as a policy marker")
	}
}
