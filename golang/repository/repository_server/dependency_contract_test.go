package main

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/globulario/services/golang/dependency"
)

// TestDependencyDeclarationMatchesLiveEnforcement — drift detector.
// The repository's dependency modes declared in golang/dependency/modes.go
// MUST agree with the runtime enforcement in dep_health.go. If they
// drift, an operator reading the contract sees one answer and the
// running service does another. This test exists so a contributor who
// changes either side breaks CI until they update both.
//
// See docs/intent/service.dependency_degradation_modes.yaml.
func TestDependencyDeclarationMatchesLiveEnforcement(t *testing.T) {
	contract := dependency.Lookup("repository")
	if contract == nil {
		t.Fatal("repository must register a dependency contract in golang/dependency/modes.go")
	}

	// Build an in-process watchdog (no goroutine, no Start) so we can
	// flip the atomic health flags and inspect RequireCapability.
	healthy := &atomic.Bool{}
	initialized := &atomic.Bool{}
	initialized.Store(true)
	w := &depHealthWatchdog{
		healthy:     healthy,
		initialized: initialized,
	}

	// Case 1: ScyllaDB dependency is declared ModeReadOnly.
	// Live behavior: ScyllaDB down → CapRepoWrite + CapRepoQuery blocked,
	// CapRepoRead still served. That's exactly ModeReadOnly semantics:
	// reads allowed, writes refused.
	scylla := contract.For("scylladb")
	if scylla.Mode != dependency.ModeReadOnly {
		t.Fatalf("scylladb declared mode: got %s, want read_only — see dep_health.go", scylla.Mode)
	}
	healthy.Store(false) // ScyllaDB unhealthy
	if err := w.RequireCapability(CapRepoWrite); err == nil {
		t.Fatal("declared scylladb=read_only but RequireCapability(CapRepoWrite) allowed when scylla unhealthy — contract drift")
	}
	if err := w.RequireCapability(CapRepoQuery); err == nil {
		t.Fatal("declared scylladb=read_only but RequireCapability(CapRepoQuery) allowed when scylla unhealthy — contract drift")
	}
	if err := w.RequireCapability(CapRepoRead); err != nil {
		t.Fatalf("declared scylladb=read_only but RequireCapability(CapRepoRead) refused when scylla unhealthy: %v — contract drift (read must remain allowed)", err)
	}

	// Case 2: MinIO is NOT a repository dependency. Packages never live in
	// MinIO — the local POSIX CAS is the sole blob authority. The contract
	// must not declare a minio/mirror dependency.
	if minio := contract.For("minio"); minio.Name != "" {
		t.Fatalf("repository must NOT declare a minio dependency — packages never live in MinIO (got mode %s)", minio.Mode)
	}

	// Case 3: etcd dependency is declared ModeStop.
	// Live behavior: dep_health.go does not probe etcd (the repository
	// can't bootstrap without it, so total absence is observed by the
	// controller's service-registration loop, not by RPC-time gating).
	// The declaration captures the contract — operators reading the
	// registry get an honest answer about etcd's role.
	etcd := contract.For("etcd")
	if etcd.Mode != dependency.ModeStop {
		t.Fatalf("etcd declared mode: got %s, want stop", etcd.Mode)
	}
	if !strings.Contains(etcd.OperatorMessage, "bootstrap") {
		t.Fatalf("etcd operator message must mention bootstrap, got: %q", etcd.OperatorMessage)
	}
}

// TestOperationalStatusMatchesContractCapabilities — second drift guard.
// The live OperationalStatus().Capabilities list must include every
// capability the contract surface implies, in a form operators reading
// the dependency registry would expect.
func TestOperationalStatusMatchesContractCapabilities(t *testing.T) {
	healthy := &atomic.Bool{}
	healthy.Store(true)
	initialized := &atomic.Bool{}
	initialized.Store(true)
	w := &depHealthWatchdog{
		healthy:     healthy,
		initialized: initialized,
	}
	status := w.OperationalStatus()
	want := []string{CapRepoWrite, CapRepoQuery, CapRepoRead}
	got := map[string]bool{}
	gotNames := make([]string, 0, len(status.Capabilities))
	for _, c := range status.Capabilities {
		got[c.Name] = true
		gotNames = append(gotNames, c.Name)
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("OperationalStatus must report capability %s; got %v", w, gotNames)
		}
	}
}
