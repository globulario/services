package serviceports

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureServicePortReadyHealsConflict(t *testing.T) {
	binDir := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_STATE_DIR", stateRoot)
	t.Setenv("GLOBULAR_PORT_RANGE", "61001-61006")

	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac-id\",\"Address\":\"localhost:61001\"}'; else exit 0; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cfgPath := filepath.Join(stateRoot, "services", "rbac-id.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	initial := map[string]any{"Id": "rbac-id", "Address": "localhost:61001", "Port": 61001}
	b, _ := json.Marshal(initial)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:61001")
	if err != nil {
		t.Skipf("listen not permitted: %v", err)
	}
	defer ln.Close()

	if err := EnsureServicePortReady(context.Background(), "rbac", "globular-rbac.service"); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	out, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	var final map[string]any
	if err := json.Unmarshal(out, &final); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	port := int(final["Port"].(float64))
	if port == 61001 || port < 61001 || port > 61006 {
		t.Fatalf("port not healed/in range: %d", port)
	}
}

func TestEnsureServicePortReadyAvoidsOtherConfigPorts(t *testing.T) {
	binDir := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_STATE_DIR", stateRoot)
	t.Setenv("GLOBULAR_PORT_RANGE", "61001-61003")

	// Existing stopped service with port 61002
	otherCfg := filepath.Join(stateRoot, "services", "other.json")
	if err := os.MkdirAll(filepath.Dir(otherCfg), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	other := map[string]any{"Id": "other", "Address": "localhost:61002", "Port": 61002}
	b, _ := json.Marshal(other)
	if err := os.WriteFile(otherCfg, b, 0o644); err != nil {
		t.Fatalf("write other cfg: %v", err)
	}

	// Target service wants 61001 (will be in-use)
	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac-id\",\"Address\":\"localhost:61001\"}'; else exit 0; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cfgPath := filepath.Join(stateRoot, "services", "rbac-id.json")
	initial := map[string]any{"Id": "rbac-id", "Address": "localhost:61001", "Port": 61001}
	b, _ = json.Marshal(initial)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:61001")
	if err != nil {
		t.Skipf("listen not permitted: %v", err)
	}
	defer ln.Close()

	if err := EnsureServicePortReady(context.Background(), "rbac", "globular-rbac.service"); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	out, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	var final map[string]any
	if err := json.Unmarshal(out, &final); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	port := int(final["Port"].(float64))
	if port != 61003 { // only remaining free port
		t.Fatalf("expected heal to 61003, got %d", port)
	}
}

func TestEnsureServicePortReadyLogsHealOldToNew(t *testing.T) {
	binDir := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_STATE_DIR", stateRoot)
	t.Setenv("GLOBULAR_PORT_RANGE", "71001-71002")

	// binary
	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac-id\",\"Address\":\"localhost:71001\"}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cfgPath := filepath.Join(stateRoot, "services", "rbac-id.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := map[string]any{"Id": "rbac-id", "Address": "localhost:71001", "Port": 71001}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:71001")
	if err != nil {
		t.Skipf("listen not permitted: %v", err)
	}
	defer ln.Close()

	// capture stdout
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	if err := EnsureServicePortReady(context.Background(), "rbac", "globular-rbac.service"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	w.Close()
	outBytes, _ := io.ReadAll(r)
	logStr := string(outBytes)
	if !strings.Contains(logStr, "71001->71002") {
		t.Fatalf("log missing heal message: %s", logStr)
	}

	finalBytes, _ := os.ReadFile(cfgPath)
	var final map[string]any
	if err := json.Unmarshal(finalBytes, &final); err != nil {
		t.Fatalf("unmarshal final: %v", err)
	}
	if int(final["Port"].(float64)) != 71002 {
		t.Fatalf("expected port 71002, got %v", final["Port"])
	}
}

func TestPortFromAddressFallbackFormats(t *testing.T) {
	cases := map[string]int{
		"localhost:61001": 61001,
		":61002":          61002,
		"61003":           61003,
		"  61004  ":       61004,
		"localhost":       0,
		"":                0,
	}

	for addr, want := range cases {
		if got := portFromAddress(addr); got != want {
			t.Fatalf("portFromAddress(%q) = %d, want %d", addr, got, want)
		}
	}
}

func TestEnsureServicePortReadyStrictMode(t *testing.T) {
	t.Setenv("GLOBULAR_PORT_PREFLIGHT_STRICT", "1")
	// Missing binary triggers describe error
	if err := EnsureServicePortReady(context.Background(), "rbac", "globular-rbac.service"); err == nil {
		t.Fatalf("expected error in strict mode for missing binary")
	}
}
