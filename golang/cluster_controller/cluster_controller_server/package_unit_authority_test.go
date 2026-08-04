package main

import (
	"os"
	"regexp"
	"testing"
)

// packageToUnit (identity.UnitForService) is the unit-name AUTHORITY.
//
// Four node-agent callbacks in workflow_release.go — restart, maybe-restart,
// stop and disable — each built the unit name as "globular-"+name+".service".
// That is only correct when the unit happens to be named after the package.
// scylladb ships as scylla-server.service, so every one of those actions
// targeted a unit that does not exist on the node: the ControlService call
// failed against a phantom unit while the real service was never touched.
func TestPackageToUnit_CanonicalMappings(t *testing.T) {
	for pkg, want := range map[string]string{
		"etcd":     "globular-etcd.service",
		"minio":    "globular-minio.service",
		"scylladb": "scylla-server.service", // NOT globular-scylladb.service
	} {
		if got := packageToUnit(pkg); got != want {
			t.Errorf("packageToUnit(%q) = %q, want %q", pkg, got, want)
		}
	}
}

// A package with no registry entry keeps the conventional fallback.
func TestPackageToUnit_FallbackForUnregistered(t *testing.T) {
	if got := packageToUnit("echo"); got != "globular-echo.service" {
		t.Errorf("packageToUnit(echo) = %q, want globular-echo.service", got)
	}
}

// Structural ratchet: no node-agent callback may rebuild the unit name by hand.
func TestNodeAgentCallbacks_UseUnitAuthority(t *testing.T) {
	b, err := os.ReadFile("workflow_release.go")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Executable (non-comment) hand-built unit names.
	re := regexp.MustCompile(`(?m)^[^/\n]*unit := "globular-" \+ name \+ "\.service"`)
	if m := re.FindAllString(string(b), -1); len(m) > 0 {
		t.Errorf("%d callback(s) still construct the unit name by hand; use packageToUnit(name) "+
			"so scylladb resolves to scylla-server.service: %v", len(m), m)
	}
}
