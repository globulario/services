package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

func TestNodeJoinWorkflowIncludesRepoInstallableProfilePackages(t *testing.T) {
	loader := v1alpha1.NewLoader()
	defPath := filepath.Join("..", "..", "workflow", "definitions", "node.join.yaml")
	def, err := loader.LoadFile(defPath)
	if err != nil {
		t.Fatalf("load node.join workflow: %v", err)
	}

	got := make(map[string]bool)
	for _, step := range def.Spec.Steps {
		if step.Actor != v1alpha1.ActorNodeAgent || step.Action != "node.install_packages" {
			continue
		}
		packages, _ := step.With["packages"].([]any)
		for _, raw := range packages {
			pkg, _ := raw.(map[string]any)
			name, _ := pkg["name"].(string)
			if name != "" {
				got[name] = true
			}
		}
	}

	requiredProfiles := map[string]bool{
		"core":          true,
		"control-plane": true,
		"storage":       true,
		"database":      true,
	}
	for _, comp := range catalog {
		if comp.InstallMode == InstallModeDay0Join {
			continue // bootstrapped by join script, not installed by the workflow
		}
		if comp.InstallMode == InstallModeTopologyWorkflow {
			continue // requires quorum precondition; installed by topology workflow, not node.join
		}
		include := false
		for _, profile := range comp.Profiles {
			if requiredProfiles[profile] {
				include = true
				break
			}
		}
		if !include {
			continue
		}
		if !got[comp.Name] {
			t.Errorf("node.join workflow missing package %q", comp.Name)
		}
	}
}

func TestJoinWorkflowInputsCarriesAssignedProfiles(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID:   "node-1",
		Profiles: []string{"control-plane", "gateway"},
	}
	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	inputs, profiles, err := srv.joinWorkflowInputs("node-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(inputs["node_profiles"]) == "" {
		t.Fatal("node_profiles input must be sent to node-agent")
	}
	for _, want := range []string{"control-plane", "core", "gateway"} {
		if !contains(profiles, want) {
			t.Fatalf("profiles = %v, missing %q", profiles, want)
		}
		if !strings.Contains(inputs["node_profiles"], want) {
			t.Fatalf("node_profiles input %q missing %q", inputs["node_profiles"], want)
		}
	}

	state.Nodes["node-empty"] = &nodeState{NodeID: "node-empty"}
	if _, _, err := srv.joinWorkflowInputs("node-empty"); err == nil {
		t.Fatal("join workflow must not start without assigned profiles")
	}
}

func TestRemoveStaleNodesLockedRemovesDuplicateEndpoint(t *testing.T) {
	state := newControllerState()
	state.Nodes["new"] = &nodeState{
		NodeID:        "new",
		Identity:      storedIdentity{Hostname: "globule-nuc", Ips: []string{"10.0.0.8"}},
		AgentEndpoint: "10.0.0.8:11000",
		LastSeen:      time.Now(),
		Status:        "ready",
	}
	state.Nodes["old"] = &nodeState{
		NodeID:        "old",
		Identity:      storedIdentity{Hostname: "globule-nuc", Ips: []string{"10.0.0.8"}},
		AgentEndpoint: "10.0.0.8:11000",
		LastSeen:      time.Now().Add(-10 * time.Second),
		Status:        "ready",
	}
	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	srv.lock("test")
	srv.removeStaleNodesLocked("new", state.Nodes["new"].Identity, state.Nodes["new"].AgentEndpoint)
	srv.unlock()

	if _, ok := srv.state.Nodes["old"]; ok {
		t.Fatal("expected duplicate node with same endpoint to be removed")
	}
	if _, ok := srv.state.Nodes["new"]; !ok {
		t.Fatal("expected authoritative node to remain present")
	}
}
