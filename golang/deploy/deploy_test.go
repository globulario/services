package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	// Find the catalog file relative to this test.
	catalogPath := findTestCatalog(t)

	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	if len(cat.Services) == 0 {
		t.Fatal("catalog has no services")
	}

	// Verify a known service exists.
	echo, err := cat.Get("echo")
	if err != nil {
		t.Fatalf("Get(echo): %v", err)
	}
	if echo.ExecName() != "echo_server" {
		t.Errorf("echo.ExecName() = %q, want echo_server", echo.ExecName())
	}
	if echo.SystemdUnit() != "globular-echo.service" {
		t.Errorf("echo.SystemdUnit() = %q, want globular-echo.service", echo.SystemdUnit())
	}
}

func TestLoadCatalog_Defaults(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// Verify event (tier 0) has no event dependency.
	event, _ := cat.Get("event")
	for _, d := range event.Dependencies {
		if d == "event" {
			t.Error("event service should not depend on itself")
		}
	}

	// Verify echo (default tier) has implicit event dependency.
	echo, _ := cat.Get("echo")
	hasEvent := false
	for _, d := range echo.Dependencies {
		if d == "event" {
			hasEvent = true
		}
	}
	if !hasEvent {
		t.Errorf("echo should have implicit event dependency, got %v", echo.Dependencies)
	}
}

func TestLoadCatalog_ServiceProperties(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	dns, _ := cat.Get("dns")
	if !dns.NeedsScylla {
		t.Error("dns should need scylla")
	}
	if dns.Priority != 2 {
		t.Errorf("dns priority = %d, want 2", dns.Priority)
	}

	nodeAgent, _ := cat.Get("node_agent")
	if !nodeAgent.RunAsRoot {
		t.Error("node_agent should run as root")
	}
	if nodeAgent.User() != "root" {
		t.Errorf("node_agent.User() = %q, want root", nodeAgent.User())
	}
}

func TestGenerateSpec_Echo(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	echo, _ := cat.Get("echo")
	spec, err := GenerateSpec(echo)
	if err != nil {
		t.Fatalf("GenerateSpec(echo): %v", err)
	}

	// Check key elements are present.
	checks := []string{
		"name: echo",
		"exec: echo_server",
		"globular-echo.service",
		"{{.StateDir}}/echo",
		"{{.Prefix}}/bin/echo_server",
		"profiles: [core, compute]",
	}
	for _, check := range checks {
		if !strings.Contains(spec, check) {
			t.Errorf("spec missing %q", check)
		}
	}

	// Echo should not have scylla wait.
	if strings.Contains(spec, "9042") {
		t.Error("echo spec should not contain scylla wait")
	}
}

func TestGenerateSpec_DNS(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	dns, _ := cat.Get("dns")
	spec, err := GenerateSpec(dns)
	if err != nil {
		t.Fatalf("GenerateSpec(dns): %v", err)
	}

	checks := []string{
		"name: dns",
		"priority: 2",
		"profiles: [core, compute, control-plane]",
		"9042",                        // scylla wait
		"CAP_NET_BIND_SERVICE",        // capability
		"scylla-server.service",       // systemd dep
	}
	for _, check := range checks {
		if !strings.Contains(spec, check) {
			t.Errorf("dns spec missing %q", check)
		}
	}
}

func TestServiceNames(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	names := cat.ServiceNames()
	if len(names) < 20 {
		t.Errorf("expected at least 20 services, got %d", len(names))
	}

	// Verify sorted.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %s before %s", names[i-1], names[i])
		}
	}
}

func TestCatalogGet_DashUnderscore(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// Should find cluster_controller with dashes.
	entry, err := cat.Get("cluster-controller")
	if err != nil {
		t.Fatalf("Get(cluster-controller): %v", err)
	}
	if entry.Name != "cluster_controller" {
		t.Errorf("name = %q, want cluster_controller", entry.Name)
	}
}

// TestGenerateSpec_MatchesExisting verifies that the Go spec generator produces
// output identical to the existing specgen.sh-generated specs on disk.
func TestGenerateSpec_MatchesExisting(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// Find generated/specs directory.
	specsDir := filepath.Join(filepath.Dir(catalogPath), "..", "generated", "specs")
	if _, err := os.Stat(specsDir); err != nil {
		t.Skipf("generated/specs not found at %s: %v", specsDir, err)
	}

	// Compare a representative set of services.
	services := []string{"echo", "dns", "rbac", "node_agent", "backup_manager", "workflow"}

	for _, svc := range services {
		t.Run(svc, func(t *testing.T) {
			entry, err := cat.Get(svc)
			if err != nil {
				t.Fatalf("Get(%s): %v", svc, err)
			}

			generated, err := GenerateSpec(entry)
			if err != nil {
				t.Fatalf("GenerateSpec(%s): %v", svc, err)
			}

			existingPath := filepath.Join(specsDir, svc+"_service.yaml")
			existing, err := os.ReadFile(existingPath)
			if err != nil {
				t.Skipf("existing spec not found: %s", existingPath)
			}

			if generated != string(existing) {
				// Show a useful diff.
				genLines := strings.Split(generated, "\n")
				existLines := strings.Split(string(existing), "\n")
				maxLines := len(genLines)
				if len(existLines) > maxLines {
					maxLines = len(existLines)
				}
				for i := 0; i < maxLines; i++ {
					var gl, el string
					if i < len(genLines) {
						gl = genLines[i]
					}
					if i < len(existLines) {
						el = existLines[i]
					}
					if gl != el {
						t.Errorf("line %d differs:\n  generated: %q\n  existing:  %q", i+1, gl, el)
						if i > 3 {
							t.Errorf("  (showing first 5 diffs only)")
							break
						}
					}
				}
			}
		})
	}
}

func TestVerifyBinary_Good(t *testing.T) {
	// /bin/true should pass verification.
	err := verifyBinary(context.Background(), "/bin/true")
	if err != nil {
		t.Errorf("verifyBinary(/bin/true) = %v, want nil", err)
	}
}

func TestVerifyBinary_Broken(t *testing.T) {
	// Create a broken binary (not executable).
	tmp := filepath.Join(t.TempDir(), "broken")
	if err := os.WriteFile(tmp, []byte("not a binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := verifyBinary(context.Background(), tmp)
	if err == nil {
		t.Error("verifyBinary(broken) = nil, want error")
	}
}

func TestVerifyBinary_Missing(t *testing.T) {
	err := verifyBinary(context.Background(), "/nonexistent/binary")
	if err == nil {
		t.Error("verifyBinary(missing) = nil, want error")
	}
}

func findTestCatalog(t *testing.T) string {
	t.Helper()
	// Walk up from test file to find service_catalog.yaml.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		path := filepath.Join(dir, "service_catalog.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		// Also check golang/ subdir.
		path = filepath.Join(dir, "golang", "service_catalog.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("service_catalog.yaml not found")
		}
		dir = parent
	}
}
