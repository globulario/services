package main

import (
	"testing"
	"time"
)

// Adapter projection: every MCPNodeEntry field used by bundle discovery must
// land in the bundlesync.NodeRegistryEntry equivalent. A field added to
// either side without updating the projection should fail this test rather
// than silently work-but-degrade.
func TestMcpEntryToBundlesyncProjection(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	in := MCPNodeEntry{
		NodeID:                 "node-a",
		Hostname:               "node-a.globular.internal",
		IP:                     "10.0.0.8",
		MCPURL:                 "https://10.0.0.8:10260",
		MCPPort:                10260,
		ClusterID:              "cluster-x",
		ReleaseVersion:         "v1.2.30",
		BuildID:                "abc123",
		AwarenessBundleVersion: "v1.2.30",
		LastSeen:               now,
		Status:                 "RUNNING",
	}

	out := mcpEntryToBundlesync(in)

	if out.NodeID != in.NodeID {
		t.Errorf("NodeID: got %q, want %q", out.NodeID, in.NodeID)
	}
	if out.PeerURL != in.MCPURL {
		t.Errorf("PeerURL: got %q, want %q (MCPURL)", out.PeerURL, in.MCPURL)
	}
	if out.ClusterID != in.ClusterID {
		t.Errorf("ClusterID: got %q, want %q", out.ClusterID, in.ClusterID)
	}
	if out.ReleaseVersion != in.ReleaseVersion {
		t.Errorf("ReleaseVersion: got %q, want %q", out.ReleaseVersion, in.ReleaseVersion)
	}
	if out.BuildID != in.BuildID {
		t.Errorf("BuildID: got %q, want %q", out.BuildID, in.BuildID)
	}
	if out.AwarenessBundleVersion != in.AwarenessBundleVersion {
		t.Errorf("AwarenessBundleVersion: got %q, want %q", out.AwarenessBundleVersion, in.AwarenessBundleVersion)
	}
	if !out.LastSeen.Equal(in.LastSeen) {
		t.Errorf("LastSeen: got %v, want %v", out.LastSeen, in.LastSeen)
	}
	if out.Status != in.Status {
		t.Errorf("Status: got %q, want %q", out.Status, in.Status)
	}
}
