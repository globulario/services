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
	stateDir := t.TempDir()
	BinDir = binDir
	t.Cleanup(func() { BinDir = "/usr/lib/globular/bin" })
	StateDir = stateDir
	t.Cleanup(func() { StateDir = "/var/lib/globular" })
	PortRange = "61001-61006"
	t.Cleanup(func() { PortRange = "" })

	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac.RbacService\",\"Address\":\"localhost:61001\"}'; else exit 0; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	// Use the ID that the identity registry returns for "rbac" (rbac.RbacService).
	cfgPath := filepath.Join(stateDir, "services", "rbac.RbacService.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	initial := map[string]any{"Id": "rbac.RbacService", "Address": "localhost:61001", "Port": 61001}
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
	stateDir := t.TempDir()
	BinDir = binDir
	t.Cleanup(func() { BinDir = "/usr/lib/globular/bin" })
	StateDir = stateDir
	t.Cleanup(func() { StateDir = "/var/lib/globular" })
	PortRange = "61001-61003"
	t.Cleanup(func() { PortRange = "" })

	// Existing stopped service with port 61002
	otherCfg := filepath.Join(stateDir, "services", "other.json")
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
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac.RbacService\",\"Address\":\"localhost:61001\"}'; else exit 0; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	// Use the ID that the identity registry returns for "rbac" (rbac.RbacService).
	cfgPath := filepath.Join(stateDir, "services", "rbac.RbacService.json")
	initial := map[string]any{"Id": "rbac.RbacService", "Address": "localhost:61001", "Port": 61001}
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
	stateDir := t.TempDir()
	BinDir = binDir
	t.Cleanup(func() { BinDir = "/usr/lib/globular/bin" })
	StateDir = stateDir
	t.Cleanup(func() { StateDir = "/var/lib/globular" })
	PortRange = "71001-71002"
	t.Cleanup(func() { PortRange = "" })

	// binary
	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac-id\",\"Address\":\"localhost:71001\"}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cfgPath := filepath.Join(stateDir, "services", "rbac-id.json")
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
	stateDir := t.TempDir()
	BinDir = binDir
	t.Cleanup(func() { BinDir = "/usr/lib/globular/bin" })
	StateDir = stateDir
	t.Cleanup(func() { StateDir = "/var/lib/globular" })
	// Range starts at 10000 (infra-reserved scylla-admin) to 49005 (free high ports).
	PortRange = "10000-49005"
	t.Cleanup(func() { PortRange = "" })

	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac.RbacService\",\"Address\":\"localhost:10000\",\"Port\":10000}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	// Use the ID that the identity registry returns for "rbac" (rbac.RbacService).
	cfgPath := filepath.Join(stateDir, "services", "rbac.RbacService.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	cfg := map[string]any{"Id": "rbac.RbacService", "Address": "localhost:10000", "Port": 10000}
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
	if port < 10000 || port > 49005 {
		t.Fatalf("port out of range after rewrite: %d", port)
	}
}

func TestEnsureServicePortConfigRewritesInUsePort(t *testing.T) {
	binDir := t.TempDir()
	stateDir := t.TempDir()
	BinDir = binDir
	t.Cleanup(func() { BinDir = "/usr/lib/globular/bin" })
	StateDir = stateDir
	t.Cleanup(func() { StateDir = "/var/lib/globular" })
	// Use high ports unlikely to be occupied on any machine.
	PortRange = "49010-49012"
	t.Cleanup(func() { PortRange = "" })

	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac.RbacService\",\"Address\":\"localhost:49011\",\"Port\":49011}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	// Use the ID that the identity registry returns for "rbac" (rbac.RbacService).
	cfgPath := filepath.Join(stateDir, "services", "rbac.RbacService.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	cfg := map[string]any{"Id": "rbac.RbacService", "Address": "localhost:49011", "Port": 49011}
	if b, err := json.Marshal(cfg); err == nil {
		if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
			t.Fatalf("write cfg: %v", err)
		}
	}

	ln, err := net.Listen("tcp", "0.0.0.0:49011")
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
	if port == 49011 {
		t.Fatalf("in-use port not rewritten, still %d", port)
	}
	if port < 49010 || port > 49012 {
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
	PreflightStrict = true
	t.Cleanup(func() { PreflightStrict = false })
	// Missing binary triggers describe error
	if err := EnsureServicePortReady(context.Background(), "rbac", "globular-rbac.service"); err == nil {
		t.Fatalf("expected error in strict mode for missing binary")
	}
}

func TestEnsureServicePortReadyHealsConflict_XDS(t *testing.T) {
	binDir := t.TempDir()
	stateDir := t.TempDir()
	BinDir = binDir
	t.Cleanup(func() { BinDir = "/usr/lib/globular/bin" })
	StateDir = stateDir
	t.Cleanup(func() { StateDir = "/var/lib/globular" })
	PortRange = "62001-62004"
	t.Cleanup(func() { PortRange = "" })

	binPath := filepath.Join(binDir, "xds")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"globular-xds\",\"Address\":\"localhost:62001\",\"Port\":62001}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write xds bin: %v", err)
	}

	// Use the ID that the identity registry returns for "xds" (globular-xds).
	cfgPath := filepath.Join(stateDir, "services", "globular-xds.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	initial := map[string]any{"Id": "globular-xds", "Address": "localhost:62001", "Port": 62001}
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
	stateDir := t.TempDir()
	BinDir = binDir
	t.Cleanup(func() { BinDir = "/usr/lib/globular/bin" })
	StateDir = stateDir
	t.Cleanup(func() { StateDir = "/var/lib/globular" })
	PortRange = "63001-63002"
	t.Cleanup(func() { PortRange = "" })

	binPath := filepath.Join(binDir, "gateway")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"globular-gateway\",\"Address\":\"localhost:80\",\"Port\":80}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write gateway bin: %v", err)
	}

	// Use the ID that the identity registry returns for "globular-gateway".
	cfgPath := filepath.Join(stateDir, "services", "globular-gateway.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	initial := map[string]any{"Id": "globular-gateway", "Address": "localhost:80", "Port": 80}
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
		"xds":                      "xds",
		"globular-xds":             "xds",
		"globular-xds.service":     "xds",
		"gateway":                  "gateway",
		"globular-gateway":         "gateway",
		"globular-gateway.service": "gateway",
		"event":                    "event_server",
		"file":                     "file_server",
		"dns":                      "dns_server",
		"authentication":           "authentication_server",
		"ai-memory":                "ai_memory_server",
		"globular-ai-memory.service": "ai_memory_server",
		"rbac":                     "rbac_server",
		"resource":                 "resource_server",
		"repository":               "repository_server",
	}
	for input, want := range cases {
		if got := executableForService(input); got != want {
			t.Fatalf("executableForService(%q) = %q, want %q", input, got, want)
		}
	}
}
