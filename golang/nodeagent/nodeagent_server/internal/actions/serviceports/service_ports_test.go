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

func TestEnsureServicePortConfigRewritesReservedPort(t *testing.T) {
	binDir := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_STATE_DIR", stateRoot)
	t.Setenv("GLOBULAR_PORT_RANGE", "10000-10002")

	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac-id\",\"Address\":\"localhost:10000\",\"Port\":10000}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cfgPath := filepath.Join(stateRoot, "services", "rbac-id.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	cfg := map[string]any{"Id": "rbac-id", "Address": "localhost:10000", "Port": 10000}
	if b, err := json.Marshal(cfg); err == nil {
		if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
			t.Fatalf("write cfg: %v", err)
		}
	}

	if err := EnsureServicePortConfig(context.Background(), "rbac", binDir); err != nil {
		t.Fatalf("ensure config: %v", err)
	}

	out, _ := os.ReadFile(cfgPath)
	var final map[string]any
	if err := json.Unmarshal(out, &final); err != nil {
		t.Fatalf("unmarshal final: %v", err)
	}
	port := int(final["Port"].(float64))
	if port == 10000 {
		t.Fatalf("reserved port not rewritten, still %d", port)
	}
	if port < 10000 || port > 10002 {
		t.Fatalf("port out of range after rewrite: %d", port)
	}
}

func TestEnsureServicePortConfigRewritesInUsePort(t *testing.T) {
	binDir := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_STATE_DIR", stateRoot)
	t.Setenv("GLOBULAR_PORT_RANGE", "12000-12002")

	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac-id\",\"Address\":\"localhost:12001\",\"Port\":12001}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cfgPath := filepath.Join(stateRoot, "services", "rbac-id.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	cfg := map[string]any{"Id": "rbac-id", "Address": "localhost:12001", "Port": 12001}
	if b, err := json.Marshal(cfg); err == nil {
		if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
			t.Fatalf("write cfg: %v", err)
		}
	}

	ln, err := net.Listen("tcp", "0.0.0.0:12001")
	if err != nil {
		t.Skipf("cannot bind listener: %v", err)
	}
	defer ln.Close()

	if err := EnsureServicePortConfig(context.Background(), "rbac", binDir); err != nil {
		t.Fatalf("ensure config: %v", err)
	}

	out, _ := os.ReadFile(cfgPath)
	var final map[string]any
	if err := json.Unmarshal(out, &final); err != nil {
		t.Fatalf("unmarshal final: %v", err)
	}
	port := int(final["Port"].(float64))
	if port == 12001 {
		t.Fatalf("in-use port not rewritten, still %d", port)
	}
	if port < 12000 || port > 12002 {
		t.Fatalf("port out of range after rewrite: %d", port)
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

func TestEnsureServicePortReadyHealsConflict_XDS(t *testing.T) {
	binDir := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_STATE_DIR", stateRoot)
	t.Setenv("GLOBULAR_PORT_RANGE", "62001-62004")

	binPath := filepath.Join(binDir, "xds_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"xds.XdsService\",\"Address\":\"localhost:62001\",\"Port\":62001}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write xds bin: %v", err)
	}

	cfgPath := filepath.Join(stateRoot, "services", "xds.XdsService.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	initial := map[string]any{"Id": "xds.XdsService", "Address": "localhost:62001", "Port": 62001}
	if b, err := json.Marshal(initial); err == nil {
		if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
			t.Fatalf("write cfg: %v", err)
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:62001")
	if err != nil {
		t.Skipf("listen not permitted: %v", err)
	}
	defer ln.Close()

	if err := EnsureServicePortReady(context.Background(), "xds", "globular-xds.service"); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	var final map[string]any
	if err := json.Unmarshal(data, &final); err != nil {
		t.Fatalf("unmarshal cfg: %v", err)
	}
	port := int(final["Port"].(float64))
	if port == 62001 || port < 62001 || port > 62004 {
		t.Fatalf("xds port not healed/in range: %d", port)
	}
}

func TestEnsureServicePortReadyHealsConflict_Gateway(t *testing.T) {
	binDir := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_STATE_DIR", stateRoot)
	t.Setenv("GLOBULAR_PORT_RANGE", "63001-63002")

	binPath := filepath.Join(binDir, "gateway_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"gateway.GatewayService\",\"Address\":\"localhost:80\",\"Port\":80}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write gateway bin: %v", err)
	}

	cfgPath := filepath.Join(stateRoot, "services", "gateway.GatewayService.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	initial := map[string]any{"Id": "gateway.GatewayService", "Address": "localhost:80", "Port": 80}
	if b, err := json.Marshal(initial); err == nil {
		if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
			t.Fatalf("write cfg: %v", err)
		}
	}

	if err := EnsureServicePortReady(context.Background(), "globular-gateway", "globular-gateway.service"); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	var final map[string]any
	if err := json.Unmarshal(data, &final); err != nil {
		t.Fatalf("unmarshal cfg: %v", err)
	}
	port := int(final["Port"].(float64))
	if port < 63001 || port > 63002 {
		t.Fatalf("gateway port not normalized into range: %d", port)
	}
}

func TestExecutableForServiceIncludesXDSGateway(t *testing.T) {
	cases := map[string]string{
		"xds":                      "xds_server",
		"globular-xds":             "xds_server",
		"globular-xds.service":     "xds_server",
		"gateway":                  "gateway_server",
		"globular-gateway":         "gateway_server",
		"globular-gateway.service": "gateway_server",
	}
	for input, want := range cases {
		if got := executableForService(input); got != want {
			t.Fatalf("executableForService(%q) = %q, want %q", input, got, want)
		}
	}
}
