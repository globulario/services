package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestApplyPackageRelease_SuppressesDuplicateInstallKey(t *testing.T) {
	actions.ActionStateDir = t.TempDir()
	t.Cleanup(func() { actions.ActionStateDir = "/var/lib/globular" })

	if _, err := actions.AcquireInstallOwnership(actions.AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-running",
	}); err != nil {
		t.Fatalf("seed ownership: %v", err)
	}

	srv := &NodeAgentServer{nodeID: "node-1"}
	resp, err := srv.ApplyPackageRelease(context.Background(), &node_agentpb.ApplyPackageReleaseRequest{
		PackageName: "envoy",
		PackageKind: "INFRASTRUCTURE",
		Version:     "1.2.3",
		BuildId:     "build-a",
		OperationId: "wf-1",
	})
	if err != nil {
		t.Fatalf("ApplyPackageRelease returned error: %v", err)
	}
	if !resp.GetOk() {
		t.Fatalf("Ok = false, want true for duplicate suppression")
	}
	if resp.GetStatus() != "suppressed_duplicate" {
		t.Fatalf("status = %q, want suppressed_duplicate", resp.GetStatus())
	}
}

func TestApplyPackageRelease_BlocksNormalInstallDuringPartialRecovery(t *testing.T) {
	actions.ActionStateDir = t.TempDir()
	t.Cleanup(func() { actions.ActionStateDir = "/var/lib/globular" })

	if _, err := actions.AcquireInstallOwnership(actions.AcquireInstallOwnershipRequest{
		NodeID:        "node-1",
		PackageID:     "envoy",
		TargetBuildID: "build-a",
		TransactionID: "txn-old",
		RecoveryMode:  true,
	}); err != nil {
		t.Fatalf("seed ownership acquire: %v", err)
	}
	if err := actions.CloseInstallOwnership("node-1", "envoy", "build-a", "txn-old", actions.InstallOwnershipStatePartialInstallRecovery, "rollback incomplete", 0); err != nil {
		t.Fatalf("seed ownership: %v", err)
	}

	srv := &NodeAgentServer{nodeID: "node-1"}
	resp, err := srv.ApplyPackageRelease(context.Background(), &node_agentpb.ApplyPackageReleaseRequest{
		PackageName: "envoy",
		PackageKind: "INFRASTRUCTURE",
		Version:     "1.2.3",
		BuildId:     "build-a",
		OperationId: "wf-1",
	})
	if err != nil {
		t.Fatalf("ApplyPackageRelease returned error: %v", err)
	}
	if resp.GetOk() {
		t.Fatalf("Ok = true, want false when partial recovery blocks normal install")
	}
	if resp.GetStatus() != "partial_install_recovery" {
		t.Fatalf("status = %q, want partial_install_recovery", resp.GetStatus())
	}
}
