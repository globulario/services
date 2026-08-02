package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// TestRunUninstallPackageSurvivesCallerCancel reproduces the orphaned-package
// resurrection bug: uninstall-package removed the on-disk binary, but the
// caller (e.g. a CLI with a short --timeout) had already given up and
// canceled its context by the time the handler reached the etcd delete. The
// old code passed that canceled ctx straight into DeleteInstalledPackage, so
// the delete failed immediately, was swallowed as "non-fatal", and the RPC
// still reported SUCCEEDED — leaving installed_state stale so cluster-doctor
// re-flagged the package as orphaned/installed on the next scan.
//
// Step 2 must run on a context detached from the caller (the file removal in
// Step 1 already created a completion obligation that must survive caller
// disconnect), and it must retry rather than silently give up.
func TestRunUninstallPackageSurvivesCallerCancel(t *testing.T) {
	origDirs := commandBinaryDirs
	origBinDir := globularBinDir
	tmp := t.TempDir()
	commandBinaryDirs = []string{tmp}
	globularBinDir = tmp
	t.Cleanup(func() {
		commandBinaryDirs = origDirs
		globularBinDir = origBinDir
	})

	origDelete := deleteInstalledPackage
	t.Cleanup(func() { deleteInstalledPackage = origDelete })

	var sawCanceledCtx bool
	calls := 0
	deleteInstalledPackage = func(ctx context.Context, nodeID, kind, name string) error {
		calls++
		if ctx.Err() != nil {
			sawCanceledCtx = true
		}
		return nil // etcd reachable, delete succeeds once given a live context
	}

	// Simulate a caller that has already timed out / disconnected by the
	// time the handler runs — this is the exact condition that triggered the
	// bug (client --timeout shorter than a slow uninstall under load).
	callerCtx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := NewNodeAgentServer("test-node", nil, NodeAgentConfig{})
	resp, err := srv.runUninstallPackage(callerCtx, &node_agentpb.RunWorkflowRequest{
		Inputs: map[string]string{"package_name": "yt-dlp", "kind": "COMMAND"},
	})
	if err != nil {
		t.Fatalf("runUninstallPackage returned error: %v", err)
	}
	if resp.GetStatus() != "SUCCEEDED" {
		t.Fatalf("expected SUCCEEDED once the delete is given a live context, got %s (err=%s)", resp.GetStatus(), resp.GetError())
	}
	if calls == 0 {
		t.Fatalf("deleteInstalledPackage was never called")
	}
	if sawCanceledCtx {
		t.Fatalf("deleteInstalledPackage was called with the caller's canceled context — Step 2 must be detached from the caller")
	}
}

// TestRunUninstallPackageFailsInsteadOfSwallowingDeleteError verifies that a
// persistent etcd-delete failure surfaces as FAILED (per this file's header
// rule: step failures must surface as such) instead of being logged as a
// warning while the RPC still reports SUCCEEDED.
func TestRunUninstallPackageFailsInsteadOfSwallowingDeleteError(t *testing.T) {
	origDirs := commandBinaryDirs
	origBinDir := globularBinDir
	tmp := t.TempDir()
	commandBinaryDirs = []string{tmp}
	globularBinDir = tmp
	t.Cleanup(func() {
		commandBinaryDirs = origDirs
		globularBinDir = origBinDir
	})

	origDelete := deleteInstalledPackage
	t.Cleanup(func() { deleteInstalledPackage = origDelete })

	wantErr := errors.New("etcd: context canceled")
	calls := 0
	deleteInstalledPackage = func(ctx context.Context, nodeID, kind, name string) error {
		calls++
		return wantErr
	}

	srv := NewNodeAgentServer("test-node", nil, NodeAgentConfig{})
	resp, err := srv.runUninstallPackage(context.Background(), &node_agentpb.RunWorkflowRequest{
		Inputs: map[string]string{"package_name": "yt-dlp", "kind": "COMMAND"},
	})
	if err != nil {
		t.Fatalf("runUninstallPackage returned error: %v", err)
	}
	if resp.GetStatus() != "FAILED" {
		t.Fatalf("expected FAILED when installed-state can never be cleared, got %s", resp.GetStatus())
	}
	if calls != 3 {
		t.Fatalf("expected 3 retry attempts, got %d", calls)
	}
	if resp.GetError() == "" {
		t.Fatalf("expected a non-empty error explaining the files-removed-but-not-cleared state")
	}
}

// sanity: package.uninstall for COMMAND kind must not touch any path outside
// the overridden test dirs.
func TestCommandBinaryDirsOverrideIsolatesFilesystem(t *testing.T) {
	tmp := t.TempDir()
	origDirs := commandBinaryDirs
	commandBinaryDirs = []string{tmp}
	t.Cleanup(func() { commandBinaryDirs = origDirs })

	for _, p := range commandBinaryPaths("yt-dlp") {
		if filepath.Dir(p) != tmp {
			t.Fatalf("commandBinaryPaths escaped test dir: %s", p)
		}
	}
}
