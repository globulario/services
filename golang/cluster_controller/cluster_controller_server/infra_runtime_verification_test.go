package main

import "testing"

// F6 verification boundary.
//
// The restart workflow verifies with node.verify_package_runtime, whose
// controller callback returns early when skipRuntimeCheck(name) is true. If
// that skipped stateful infrastructure, the workflow's verification block would
// pass WITHOUT ever probing the unit — a restart would self-certify and every
// downstream ordering guarantee would rest on nothing.
//
// skipRuntimeCheck skips only KindCommand packages, so etcd/minio/scylladb are
// genuinely verified. This ratchet pins that, because a catalog change that
// reclassified any of them would silently turn verification into a no-op.
func TestSkipRuntimeCheck_DoesNotSkipStatefulInfrastructure(t *testing.T) {
	for _, comp := range []string{"etcd", "minio", "scylladb"} {
		if skipRuntimeCheck(comp) {
			t.Errorf("skipRuntimeCheck(%q) = true — the restart workflow's verification "+
				"would pass without probing the unit, letting a restart self-certify", comp)
		}
	}
}

// The skip must still apply to command packages, which own no unit.
func TestSkipRuntimeCheck_SkipsCommandPackages(t *testing.T) {
	var cmd string
	for _, c := range []string{"etcdctl", "sha256sum", "yt-dlp", "codex", "claude"} {
		if comp := CatalogByName(c); comp != nil && comp.Kind == KindCommand {
			cmd = c
			break
		}
	}
	if cmd == "" {
		t.Skip("no COMMAND package in the catalog to exercise the skip path")
	}
	if !skipRuntimeCheck(cmd) {
		t.Errorf("skipRuntimeCheck(%q) = false; command packages have no unit to verify", cmd)
	}
}

// The three restart components must resolve to their canonical units, so the
// verification probes the unit the restart actually touched.
func TestRestartComponents_ResolveToCanonicalUnits(t *testing.T) {
	for comp, want := range map[string]string{
		"etcd":     "globular-etcd.service",
		"minio":    "globular-minio.service",
		"scylladb": "scylla-server.service",
	} {
		if got := packageToUnit(comp); got != want {
			t.Errorf("packageToUnit(%q) = %q, want %q — restart and verification must "+
				"target the same canonical unit", comp, got, want)
		}
	}
}
