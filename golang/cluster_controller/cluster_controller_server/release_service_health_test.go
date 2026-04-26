package main

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ── serviceHealthyForRelease: command-tool false-positive guard ───────────────
//
// Root cause: etcdctl, sha256sum, yt-dlp are published as ServiceRelease with
// kind=SERVICE but systemd=none.  serviceHealthyForRelease searched node.Units
// for the generated unit name ("globular-etcdctl.service"), never found it, and
// returned false — marking the node DEGRADED every drift cycle.
//
// Fix: skipRuntimeCheck now includes these names, and serviceHealthyForRelease
// returns true immediately for any skipRuntimeCheck package.

// makeServiceRelease returns a minimal ServiceRelease for the given service name.
func makeServiceRelease(name string) *cluster_controllerpb.ServiceRelease {
	return &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "svc/" + name},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: name},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:           cluster_controllerpb.ReleasePhaseAvailable,
			ResolvedVersion: "1.0.0",
		},
	}
}

// TestServiceHealthyForRelease_CommandTool_NoUnit_IsHealthy verifies that
// etcdctl (no systemd unit, kind=SERVICE, systemd=none) is treated as healthy.
// Before the fix, this returned false → permanent DEGRADED drift every cycle.
func TestServiceHealthyForRelease_CommandTool_NoUnit_IsHealthy(t *testing.T) {
	node := &nodeState{
		NodeID:   "n1",
		Status:   "ready",
		LastSeen: time.Now(),
		Units:    []unitStatusRecord{}, // etcdctl has no systemd unit
	}
	srv := newTestServer(t, &controllerState{Nodes: map[string]*nodeState{"n1": node}})
	rel := makeServiceRelease("etcdctl")

	if !srv.serviceHealthyForRelease(node, rel) {
		t.Fatal("serviceHealthyForRelease must return true for etcdctl (command tool, no systemd unit)")
	}
}

// TestServiceHealthyForRelease_Sha256sum_NoUnit_IsHealthy verifies sha256sum.
func TestServiceHealthyForRelease_Sha256sum_NoUnit_IsHealthy(t *testing.T) {
	node := &nodeState{
		NodeID:   "n1",
		Status:   "ready",
		LastSeen: time.Now(),
		Units:    []unitStatusRecord{},
	}
	srv := newTestServer(t, &controllerState{Nodes: map[string]*nodeState{"n1": node}})

	if !srv.serviceHealthyForRelease(node, makeServiceRelease("sha256sum")) {
		t.Fatal("serviceHealthyForRelease must return true for sha256sum (command tool)")
	}
}

// TestServiceHealthyForRelease_YtDlp_NoUnit_IsHealthy verifies yt-dlp.
func TestServiceHealthyForRelease_YtDlp_NoUnit_IsHealthy(t *testing.T) {
	node := &nodeState{
		NodeID:   "n1",
		Status:   "ready",
		LastSeen: time.Now(),
		Units:    []unitStatusRecord{},
	}
	srv := newTestServer(t, &controllerState{Nodes: map[string]*nodeState{"n1": node}})

	if !srv.serviceHealthyForRelease(node, makeServiceRelease("yt-dlp")) {
		t.Fatal("serviceHealthyForRelease must return true for yt-dlp (command tool)")
	}
}

// TestServiceHealthyForRelease_Daemon_ActiveUnit_IsHealthy verifies that a
// real daemon service with an active unit returns true (positive case).
func TestServiceHealthyForRelease_Daemon_ActiveUnit_IsHealthy(t *testing.T) {
	node := &nodeState{
		NodeID:   "n1",
		Status:   "ready",
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-cluster-controller.service", State: "active"},
		},
	}
	srv := newTestServer(t, &controllerState{Nodes: map[string]*nodeState{"n1": node}})

	if !srv.serviceHealthyForRelease(node, makeServiceRelease("cluster-controller")) {
		t.Fatal("serviceHealthyForRelease must return true for daemon with active unit")
	}
}

// TestServiceHealthyForRelease_Daemon_InactiveUnit_IsUnhealthy verifies that
// a daemon with an inactive unit returns false (correct negative case).
func TestServiceHealthyForRelease_Daemon_InactiveUnit_IsUnhealthy(t *testing.T) {
	node := &nodeState{
		NodeID:   "n1",
		Status:   "ready",
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-cluster-controller.service", State: "inactive"},
		},
	}
	srv := newTestServer(t, &controllerState{Nodes: map[string]*nodeState{"n1": node}})

	if srv.serviceHealthyForRelease(node, makeServiceRelease("cluster-controller")) {
		t.Fatal("serviceHealthyForRelease must return false for daemon with inactive unit")
	}
}

// TestServiceHealthyForRelease_Daemon_MissingUnit_IsUnhealthy verifies that
// a daemon service absent from node.Units returns false (not silently healthy).
func TestServiceHealthyForRelease_Daemon_MissingUnit_IsUnhealthy(t *testing.T) {
	node := &nodeState{
		NodeID:   "n1",
		Status:   "ready",
		LastSeen: time.Now(),
		Units:    []unitStatusRecord{}, // unit not reported
	}
	srv := newTestServer(t, &controllerState{Nodes: map[string]*nodeState{"n1": node}})

	if srv.serviceHealthyForRelease(node, makeServiceRelease("cluster-controller")) {
		t.Fatal("serviceHealthyForRelease must return false when daemon unit is missing from node.Units")
	}
}

// TestSkipRuntimeCheck_CommandTools verifies the skipRuntimeCheck allowlist
// covers all known command-style packages (both old and newly added).
func TestSkipRuntimeCheck_CommandTools(t *testing.T) {
	tools := []string{
		"restic", "rclone", "ffmpeg", "sctool", "mc",
		"etcdctl", "sha256sum", "yt-dlp",
	}
	for _, name := range tools {
		if !skipRuntimeCheck(name) {
			t.Errorf("skipRuntimeCheck(%q) = false; want true (command tool has no systemd unit)", name)
		}
	}
}

// TestSkipRuntimeCheck_DaemonServices verifies that real daemon services are
// NOT skipped — their runtime health must be verified.
func TestSkipRuntimeCheck_DaemonServices(t *testing.T) {
	daemons := []string{
		"cluster-controller", "cluster-doctor", "workflow",
		"xds", "envoy", "minio", "scylladb", "prometheus",
	}
	for _, name := range daemons {
		if skipRuntimeCheck(name) {
			t.Errorf("skipRuntimeCheck(%q) = true; want false (daemon services must be health-checked)", name)
		}
	}
}
