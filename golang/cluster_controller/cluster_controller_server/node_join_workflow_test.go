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

func TestNodeJoinWorkflowMediaPackagesRequireMediaServerProfile(t *testing.T) {
	loader := v1alpha1.NewLoader()
	defPath := filepath.Join("..", "..", "workflow", "definitions", "node.join.yaml")
	def, err := loader.LoadFile(defPath)
	if err != nil {
		t.Fatalf("load node.join workflow: %v", err)
	}

	stepByID := make(map[string]v1alpha1.WorkflowStepSpec, len(def.Spec.Steps))
	for _, step := range def.Spec.Steps {
		stepByID[step.ID] = step
	}

	assertStepExcludesPackages(t, stepByID["install_workloads"], []string{"title", "media", "torrent"})
	assertStepExcludesPackages(t, stepByID["install_commands"], []string{"ffmpeg", "yt-dlp"})

	assertMediaStep(t, stepByID["install_media_workloads"], []string{"title", "media", "torrent"})
	assertMediaStep(t, stepByID["install_media_commands"], []string{"ffmpeg", "yt-dlp"})
}

func assertMediaStep(t *testing.T, step v1alpha1.WorkflowStepSpec, wantPackages []string) {
	t.Helper()
	if step.ID == "" {
		t.Fatal("media package step is missing")
	}
	if step.When == nil || !strings.Contains(step.When.Expr, "media-server") {
		t.Fatalf("%s must be gated by media-server profile, got %#v", step.ID, step.When)
	}
	got := map[string]bool{}
	for _, name := range packageNamesFromStep(step) {
		got[name] = true
	}
	for _, want := range wantPackages {
		if !got[want] {
			t.Fatalf("%s missing %s", step.ID, want)
		}
	}
}

func assertStepExcludesPackages(t *testing.T, step v1alpha1.WorkflowStepSpec, names []string) {
	t.Helper()
	for _, got := range packageNamesFromStep(step) {
		for _, forbidden := range names {
			if got == forbidden {
				t.Fatalf("package %q must not be installed by unconditional %s", got, step.ID)
			}
		}
	}
}

func TestNodeJoinWorkflowReportDependsOnMediaSteps(t *testing.T) {
	loader := v1alpha1.NewLoader()
	defPath := filepath.Join("..", "..", "workflow", "definitions", "node.join.yaml")
	def, err := loader.LoadFile(defPath)
	if err != nil {
		t.Fatalf("load node.join workflow: %v", err)
	}

	stepByID := make(map[string]v1alpha1.WorkflowStepSpec, len(def.Spec.Steps))
	for _, step := range def.Spec.Steps {
		stepByID[step.ID] = step
	}
	report := stepByID["report_installed"]
	for _, want := range []string{"install_media_workloads", "install_media_commands"} {
		if !testContainsString(report.DependsOn, want) {
			t.Fatalf("report_installed dependencies = %v, missing %s", report.DependsOn, want)
		}
	}
}

func testContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func packageNamesFromStep(step v1alpha1.WorkflowStepSpec) []string {
	packages, _ := step.With["packages"].([]any)
	names := make([]string, 0, len(packages))
	for _, raw := range packages {
		pkg, _ := raw.(map[string]any)
		name, _ := pkg["name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
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
