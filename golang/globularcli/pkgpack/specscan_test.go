package pkgpack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServiceNameFromFilename(t *testing.T) {
	name := deriveServiceName(filepath.Join(os.TempDir(), "node_agent_service.yaml"), map[string]any{})
	if name != "node-agent" {
		t.Fatalf("expected node-agent, got %s", name)
	}
}

func TestExecDerivationFromBinPath(t *testing.T) {
	dir := t.TempDir()
	assets := filepath.Join(dir, "internal", "assets")
	if err := os.MkdirAll(filepath.Join(assets, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	execPath := filepath.Join(assets, "bin", "nodeagent_server")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	specDir := filepath.Join(dir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specDir, "node_agent_service.yaml")
	content := "steps:\n  - cmd: '/internal/assets/bin/nodeagent_server --flag'\n"
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := ScanSpec(specPath, assets, ScanOptions{SkipMissingConfig: true, SkipMissingSystemd: true})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if info.ExecName != "nodeagent_server" {
		t.Fatalf("expected exec nodeagent_server, got %s", info.ExecName)
	}
}
