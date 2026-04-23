package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewNodeAgentServerSeedsDomainAndProtocolFromRuntimeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", tmpDir)

	cfgPath := filepath.Join(tmpDir, "config.json")
	cfgData := []byte(`{"Domain":"globular.internal","Protocol":"https","Name":"node-a","Address":"10.0.0.10"}`)
	if err := os.WriteFile(cfgPath, cfgData, 0o644); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}

	statePath := filepath.Join(tmpDir, "nodeagent", "state.json")
	srv := NewNodeAgentServer(statePath, newNodeAgentState(), NodeAgentConfig{
		Port:         "11000",
		AdvertiseAddr: "10.0.0.10:11000",
		ClusterMode:  true,
	})

	if got := srv.state.ClusterDomain; got != "globular.internal" {
		t.Fatalf("cluster_domain = %q, want globular.internal", got)
	}
	if got := srv.state.Protocol; got != "https" {
		t.Fatalf("protocol = %q, want https", got)
	}

	if err := srv.saveState(); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	loaded, err := loadNodeAgentState(statePath)
	if err != nil {
		t.Fatalf("loadNodeAgentState: %v", err)
	}
	if loaded.ClusterDomain != "globular.internal" {
		t.Fatalf("persisted cluster_domain = %q, want globular.internal", loaded.ClusterDomain)
	}
	if loaded.Protocol != "https" {
		t.Fatalf("persisted protocol = %q, want https", loaded.Protocol)
	}
}

func TestSaveStateRefreshesDomainAndProtocolFromRuntimeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", tmpDir)

	cfgPath := filepath.Join(tmpDir, "config.json")
	cfgData := []byte(`{"Domain":"globular.internal","Protocol":"https","Name":"node-a","Address":"10.0.0.10"}`)
	if err := os.WriteFile(cfgPath, cfgData, 0o644); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}

	statePath := filepath.Join(tmpDir, "nodeagent", "state.json")
	initial := &nodeAgentState{ClusterDomain: "", Protocol: ""}
	srv := NewNodeAgentServer(statePath, initial, NodeAgentConfig{
		Port:          "11000",
		AdvertiseAddr: "10.0.0.10:11000",
		ClusterMode:   true,
	})

	// Simulate stale in-memory values to ensure saveState refreshes from config.json.
	srv.state.ClusterDomain = ""
	srv.state.Protocol = ""
	if err := srv.saveState(); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("unmarshal state file: %v", err)
	}
	if persisted["cluster_domain"] != "globular.internal" {
		t.Fatalf("state.cluster_domain = %v, want globular.internal", persisted["cluster_domain"])
	}
	if persisted["protocol"] != "https" {
		t.Fatalf("state.protocol = %v, want https", persisted["protocol"])
	}
}

func TestNewNodeAgentServerIgnoresLoopbackCachedControllerEndpointInClusterMode(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", tmpDir)

	cfgPath := filepath.Join(tmpDir, "config.json")
	cfgData := []byte(`{"Domain":"globular.internal","Protocol":"https","Name":"node-a","Address":"10.0.0.10"}`)
	if err := os.WriteFile(cfgPath, cfgData, 0o644); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}

	state := &nodeAgentState{
		ControllerEndpoint: "localhost:12000",
	}
	srv := NewNodeAgentServer(filepath.Join(tmpDir, "nodeagent", "state.json"), state, NodeAgentConfig{
		Port:          "11000",
		AdvertiseAddr: "10.0.0.10:11000",
		ClusterMode:   true,
	})

	if got := srv.controllerEndpoint; got == "localhost:12000" {
		t.Fatalf("controllerEndpoint must not keep loopback cache, got %q", got)
	}
	if isNonRoutableEndpoint(srv.controllerEndpoint) {
		t.Fatalf("controllerEndpoint should be routable in cluster mode, got %q", srv.controllerEndpoint)
	}
	if got := srv.state.ControllerEndpoint; got == "localhost:12000" {
		t.Fatalf("state.ControllerEndpoint must not keep loopback cache, got %q", got)
	}
}
