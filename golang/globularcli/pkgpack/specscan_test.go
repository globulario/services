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
	execPath := filepath.Join(assets, "bin", "node_agent")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	specDir := filepath.Join(dir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specDir, "node_agent_service.yaml")
	content := "steps:\n  - cmd: '/internal/assets/bin/node_agent --flag'\n"
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	roots := AssetRoots{
		BinRoot:    filepath.Join(assets, "bin"),
		ConfigRoot: filepath.Join(assets, "config"),
	}
	info, err := ScanSpec(specPath, roots, ScanOptions{SkipMissingConfig: true, SkipMissingSystemd: true})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if info.ExecName != "node_agent" {
		t.Fatalf("expected exec node_agent, got %s", info.ExecName)
	}
}
