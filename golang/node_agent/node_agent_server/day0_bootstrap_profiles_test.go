package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDay0BootstrapProfilesInput_LegacyFallbackDoesNotInjectMediaServer(t *testing.T) {
	inputs := map[string]any{}

	usedFallback := ensureDay0BootstrapProfilesInput(inputs)
	if !usedFallback {
		t.Fatal("expected legacy fallback to be used")
	}

	got, ok := inputs["bootstrap_node_profiles"].([]string)
	if !ok {
		t.Fatalf("bootstrap_node_profiles type = %T, want []string", inputs["bootstrap_node_profiles"])
	}
	want := []string{"core", "control-plane", "storage"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("bootstrap_node_profiles = %v, want %v", got, want)
	}
	for _, profile := range got {
		if profile == "media-server" {
			t.Fatal("legacy fallback must not inject media-server")
		}
	}
}

func TestDefaultBootstrapProfiles_QuorumOnly(t *testing.T) {
	got := defaultBootstrapProfiles()
	want := []string{"core", "control-plane", "storage"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("defaultBootstrapProfiles = %v, want %v", got, want)
	}
	for _, profile := range got {
		if profile == "media-server" {
			t.Fatal("default bootstrap profiles must not include media-server")
		}
	}
}

func TestEnsureDay0BootstrapProfilesInput_PreservesExplicitProfiles(t *testing.T) {
	inputs := map[string]any{
		"bootstrap_node_profiles": []string{"core", "control-plane", "storage", "media-server"},
	}

	usedFallback := ensureDay0BootstrapProfilesInput(inputs)
	if usedFallback {
		t.Fatal("explicit bootstrap_node_profiles must not use fallback")
	}
	got := inputs["bootstrap_node_profiles"].([]string)
	if len(got) != 4 || got[3] != "media-server" {
		t.Fatalf("explicit bootstrap_node_profiles mutated: %v", got)
	}
}

func TestDecodeWorkflowInputValue_ProfileLists(t *testing.T) {
	got, ok := decodeWorkflowInputValue("node_profiles", "core, control-plane, media-server").([]any)
	if !ok {
		t.Fatalf("node_profiles type = %T, want []any", got)
	}
	want := []string{"core", "control-plane", "media-server"}
	if len(got) != len(want) {
		t.Fatalf("node_profiles length = %d, want %d: %v", len(got), len(want), got)
	}
	for i, value := range want {
		if got[i] != value {
			t.Fatalf("node_profiles[%d] = %v, want %s", i, got[i], value)
		}
	}

	if got := decodeWorkflowInputValue("repository_address", "10.0.0.63:443"); got != "10.0.0.63:443" {
		t.Fatalf("repository_address decoded as %#v", got)
	}
}

func TestInstallDay0Script_PassesBootstrapNodeProfiles(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	scriptPath := filepath.Join(root, "scripts", "release", "install-day0.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read %s: %v", scriptPath, err)
	}
	text := string(data)

	if !strings.Contains(text, `BOOTSTRAP_NODE_PROFILES_JSON=$(printf '%s' "$FOUNDING_PROFILES"`) {
		t.Fatal("install-day0.sh must derive BOOTSTRAP_NODE_PROFILES_JSON from FOUNDING_PROFILES")
	}
	if !strings.Contains(text, `bootstrap_node_profiles: $bootstrap_node_profiles`) {
		t.Fatal("install-day0.sh RunWorkflow request must include bootstrap_node_profiles")
	}
	if !strings.Contains(text, `FOUNDING_PROFILES="${FOUNDING_PROFILES:-core}"`) {
		t.Fatal("install-day0.sh default FOUNDING_PROFILES must be core-only")
	}
	if !strings.Contains(text, `has_profile()`) {
		t.Fatal("install-day0.sh must define a has_profile helper for media gating")
	}
	if !strings.Contains(text, `COMMON_WORKLOAD_PKGS=(`) || !strings.Contains(text, `MEDIA_WORKLOAD_PKGS=(`) {
		t.Fatal("install-day0.sh must split common and media workload package arrays")
	}
	if !strings.Contains(text, `COMMON_CMDS_PKGS=(`) || !strings.Contains(text, `MEDIA_CMDS_PKGS=(`) {
		t.Fatal("install-day0.sh must split common and media command package arrays")
	}
	if strings.Contains(text, `
OPTIONAL_WORKLOAD_PKGS=(`) {
		t.Fatal("install-day0.sh must not keep an unconditional OPTIONAL_WORKLOAD_PKGS array")
	}
	if strings.Contains(text, `
CMDS_PKGS=(`) {
		t.Fatal("install-day0.sh must not keep an unconditional CMDS_PKGS array")
	}
	if !strings.Contains(text, "install_list \"${COMMON_WORKLOAD_PKGS[@]}\"\nif has_profile \"media-server\"; then\n  install_list \"${MEDIA_WORKLOAD_PKGS[@]}\"\nfi") {
		t.Fatal("install-day0.sh must guard media workload installation with has_profile \"media-server\"")
	}
	if !strings.Contains(text, "install_list \"${COMMON_CMDS_PKGS[@]}\"\nif has_profile \"media-server\"; then\n  install_list \"${MEDIA_CMDS_PKGS[@]}\"\nfi") {
		t.Fatal("install-day0.sh must guard media command installation with has_profile \"media-server\"")
	}
}
