package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestRefineNodeProfilesFromCapabilities_ExpandsJoiningCoreNode(t *testing.T) {
	t.Parallel()

	node := &nodeState{
		Profiles:           []string{"core"},
		JoinLifecyclePhase: JoinPhaseBootstrapping,
		Metadata: map[string]string{
			nodeProfileSourceMetadataKey: nodeProfileSourceDefault,
		},
	}
	caps := &cluster_controllerpb.NodeCapabilities{
		CpuCount:      8,
		RamBytes:      32 * _GB,
		DiskBytes:     512 * _GB,
		DiskFreeBytes: 480 * _GB,
	}

	if !refineNodeProfilesFromCapabilities(node, caps, false) {
		t.Fatal("expected hardware capabilities to expand core-only joining profile")
	}
	for _, want := range []string{"core", "control-plane", "storage"} {
		if !hasProfile(node.Profiles, want) {
			t.Fatalf("profile %q missing after refinement: got %v", want, node.Profiles)
		}
	}
	if node.Metadata[nodeProfileSourceMetadataKey] != nodeProfileSourceDeduced {
		t.Fatalf("profile source = %q, want %q",
			node.Metadata[nodeProfileSourceMetadataKey], nodeProfileSourceDeduced)
	}
	if node.EtcdMemberIntent == nil || !node.EtcdMemberIntent.Member {
		t.Fatal("refined control-plane/core profile must refresh etcd membership intent")
	}
	if node.ScyllaIntent == nil || !node.ScyllaIntent.Member {
		t.Fatal("refined storage profile must refresh Scylla membership intent")
	}
	if node.ObjectStoreIntent == nil || !node.ObjectStoreIntent.Member {
		t.Fatal("refined storage profile must refresh objectstore membership intent")
	}
}

func TestRefineNodeProfilesFromCapabilities_DoesNotOverrideRequestedCore(t *testing.T) {
	t.Parallel()

	node := &nodeState{
		Profiles:           []string{"core"},
		JoinLifecyclePhase: JoinPhaseBootstrapping,
		Metadata: map[string]string{
			nodeProfileSourceMetadataKey: nodeProfileSourceRequested,
		},
	}
	caps := &cluster_controllerpb.NodeCapabilities{
		CpuCount:      8,
		RamBytes:      32 * _GB,
		DiskFreeBytes: 480 * _GB,
	}

	if refineNodeProfilesFromCapabilities(node, caps, false) {
		t.Fatal("operator-requested core-only profiles must not be rewritten from heartbeat hardware")
	}
	if !sameStrings(node.Profiles, []string{"core"}) {
		t.Fatalf("profiles changed: got %v", node.Profiles)
	}
}

func TestRefineNodeProfilesFromCapabilities_DoesNotRewriteAdmittedNode(t *testing.T) {
	t.Parallel()

	node := &nodeState{
		Profiles:           []string{"core"},
		JoinLifecyclePhase: JoinPhaseAdmitted,
	}
	caps := &cluster_controllerpb.NodeCapabilities{
		CpuCount:      8,
		RamBytes:      32 * _GB,
		DiskFreeBytes: 480 * _GB,
	}

	if refineNodeProfilesFromCapabilities(node, caps, false) {
		t.Fatal("admitted nodes must not be rewritten by heartbeat hardware refinement")
	}
	if !sameStrings(node.Profiles, []string{"core"}) {
		t.Fatalf("profiles changed: got %v", node.Profiles)
	}
}

func TestRefineNodeProfilesFromCapabilities_LowHardwareStaysCore(t *testing.T) {
	t.Parallel()

	node := &nodeState{
		Profiles:           []string{"core"},
		JoinLifecyclePhase: JoinPhaseBootstrapping,
	}
	caps := &cluster_controllerpb.NodeCapabilities{
		CpuCount:      1,
		RamBytes:      512 * 1024 * 1024,
		DiskFreeBytes: 10 * _GB,
	}

	if refineNodeProfilesFromCapabilities(node, caps, false) {
		t.Fatal("low hardware should not expand beyond core")
	}
	if !sameStrings(node.Profiles, []string{"core"}) {
		t.Fatalf("profiles changed: got %v", node.Profiles)
	}
}

func TestRefineNodeProfilesFromCapabilities_DoesNotRewriteSignedPlan(t *testing.T) {
	t.Parallel()

	node := &nodeState{
		Profiles:           []string{"core"},
		JoinLifecyclePhase: JoinPhaseBootstrapping,
		Metadata: map[string]string{
			nodeProfileSourceMetadataKey: nodeProfileSourceDeduced,
		},
	}
	caps := &cluster_controllerpb.NodeCapabilities{
		CpuCount:      8,
		RamBytes:      32 * _GB,
		DiskFreeBytes: 480 * _GB,
	}

	if refineNodeProfilesFromCapabilities(node, caps, true) {
		t.Fatal("signed JoinPlan assignments must not be rewritten from heartbeat hardware")
	}
	if !sameStrings(node.Profiles, []string{"core"}) {
		t.Fatalf("profiles changed: got %v", node.Profiles)
	}
}

func hasProfile(profiles []string, profile string) bool {
	for _, p := range profiles {
		if p == profile {
			return true
		}
	}
	return false
}
