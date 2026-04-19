package serviceports

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Two services both describe the same port; second gets rewritten.
func TestInstallPayloadDuplicatePortGetsRewritten(t *testing.T) {
	binDir := t.TempDir()
	stateDir := t.TempDir()
	BinDir = binDir
	t.Cleanup(func() { BinDir = "/usr/lib/globular/bin" })
	StateDir = stateDir
	t.Cleanup(func() { StateDir = "/var/lib/globular" })
	PortRange = "64001-64003"
	t.Cleanup(func() { PortRange = "" })

	// service A
	binA := filepath.Join(binDir, "rbac_server")
	scriptA := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac-id\",\"Address\":\"localhost:64001\"}'; fi\n"
	if err := os.WriteFile(binA, []byte(scriptA), 0o755); err != nil {
		t.Fatalf("binA: %v", err)
	}
	if err := EnsureServicePortConfig(context.Background(), "rbac", binDir); err != nil {
		t.Fatalf("ensure A: %v", err)
	}

	// service B wants same port
	binB := filepath.Join(binDir, "resource_server")
	scriptB := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"resource-id\",\"Address\":\"localhost:64001\"}'; fi\n"
	if err := os.WriteFile(binB, []byte(scriptB), 0o755); err != nil {
		t.Fatalf("binB: %v", err)
	}
	if err := EnsureServicePortConfig(context.Background(), "resource", binDir); err != nil {
		t.Fatalf("ensure B: %v", err)
	}

	// Use the ID that the identity registry returns for "resource".
	cfgBPath := filepath.Join(stateDir, "services", "resource.ResourceService.json")
	b, err := os.ReadFile(cfgBPath)
	if err != nil {
		t.Fatalf("read cfgB: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("unmarshal cfgB: %v", err)
	}
	port := int(cfg["Port"].(float64))
	if port == 64001 {
		t.Fatalf("port not rewritten away from duplicate: %d", port)
	}
	if port < 64001 || port > 64003 {
		t.Fatalf("port out of range: %d", port)
	}
}
