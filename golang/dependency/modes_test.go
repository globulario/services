package dependency

import (
	"strings"
	"testing"
)

// TestCriticalDependencyDeclaresDegradationMode — contract test. Every
// service the registry knows about declares at least one dependency, and
// every declared dependency has a known mode. A service may not register a
// half-formed contract that ignores the dependency-mode discipline.
func TestCriticalDependencyDeclaresDegradationMode(t *testing.T) {
	services := RegisteredServices()
	if len(services) == 0 {
		t.Fatal("registry must contain at least the built-in critical services")
	}
	mustHave := []string{"repository", "workflow", "cluster_doctor"}
	for _, want := range mustHave {
		c := Lookup(want)
		if c == nil {
			t.Fatalf("critical service %q has no dependency contract", want)
		}
		if len(c.Dependencies) == 0 {
			t.Fatalf("service %q registered empty contract", want)
		}
		for _, d := range c.Dependencies {
			if strings.TrimSpace(d.Name) == "" {
				t.Fatalf("service %q has dependency with empty Name", want)
			}
			if !d.Mode.IsKnown() {
				t.Fatalf("service %q dependency %q declared unknown mode %q",
					want, d.Name, d.Mode)
			}
		}
	}
}

// TestRepositoryMinioIsDegradedNotReadOnly — contract test. The
// repository's MinIO dependency is a SECONDARY (mirror) authority, not a
// primary one — the local POSIX CAS is the installability authority.
// When MinIO is down, the repository degrades (mirror writes skipped)
// but continues to accept writes against the local POSIX CAS. This is
// distinct from the ScyllaDB dependency, where unavailability does
// drop the service to read_only. See
// docs/operators/remediation-contracts.md §6 and dep_health.go.
func TestRepositoryMinioIsDegradedNotReadOnly(t *testing.T) {
	c := Lookup("repository")
	if c == nil {
		t.Fatal("repository contract missing")
	}
	dep := c.For("minio")
	if dep.Name == "" {
		t.Fatal("repository must declare minio dependency")
	}
	if dep.Mode != ModeDegraded {
		t.Fatalf("repository minio mode: got %s, want degraded — MinIO is a mirror, not primary authority", dep.Mode)
	}

	// Under degraded, reads/writes/dispatch all proceed by default —
	// only operations explicitly listed in BlockedOperations refuse.
	// The minio dependency's BlockedOperations is empty (mirror-class
	// gating happens inside the repository's CapRepoMirror check, not at
	// the broad operation level).
	for _, op := range []Operation{OperationReadOnly, OperationWrite, OperationDispatch} {
		if ok, reason := AllowOperation(dep, op); !ok {
			t.Fatalf("op %s under MinIO-degraded must be allowed (mirror gating is separate): %s", op, reason)
		}
	}

	// And: the ScyllaDB dependency IS the one that drops the repository
	// to read_only — primary metadata authority for the package index.
	scylla := c.For("scylladb")
	if scylla.Mode != ModeReadOnly {
		t.Fatalf("scylladb mode: got %s, want read_only", scylla.Mode)
	}
	if ok, _ := AllowOperation(scylla, OperationWrite); ok {
		t.Fatal("write while ScyllaDB down must be refused")
	}
	if ok, _ := AllowOperation(scylla, OperationDispatch); ok {
		t.Fatal("dispatch while ScyllaDB down must be refused")
	}
}

// TestWorkflowDispatchBlocksWhenRequiredDependencyModeIsStop — contract
// test. When a workflow dependency is declared ModeStop, dispatch must be
// refused. The block must apply to read AND dispatch since stop is total.
func TestWorkflowDispatchBlocksWhenRequiredDependencyModeIsStop(t *testing.T) {
	c := Lookup("workflow")
	if c == nil {
		t.Fatal("workflow contract missing")
	}
	dep := c.For("etcd")
	if dep.Name == "" {
		t.Fatal("workflow must declare etcd dependency")
	}
	if dep.Mode != ModeStop {
		t.Fatalf("workflow etcd mode: got %s, want stop", dep.Mode)
	}
	for _, op := range []Operation{OperationReadOnly, OperationWrite, OperationDispatch} {
		ok, reason := AllowOperation(dep, op)
		if ok {
			t.Fatalf("op %s must be refused under stop mode, got allowed", op)
		}
		if !strings.Contains(reason, "stop") {
			t.Fatalf("op %s refusal must mention stop mode, got: %q", op, reason)
		}
	}

	// ModeHold blocks dispatch + write but allows reads — a strictly weaker
	// bound than stop. Verify the same workflow contract gets this right
	// for its ScyllaDB dependency.
	holdDep := c.For("scylladb")
	if holdDep.Name == "" {
		t.Fatal("workflow must declare scylladb dependency")
	}
	if holdDep.Mode != ModeHold {
		t.Fatalf("workflow scylladb mode: got %s, want hold", holdDep.Mode)
	}
	if ok, _ := AllowOperation(holdDep, OperationReadOnly); !ok {
		t.Fatal("read must be allowed under hold mode")
	}
	if ok, _ := AllowOperation(holdDep, OperationDispatch); ok {
		t.Fatal("dispatch must be refused under hold mode")
	}
}

// TestUnknownModeIsRejected — defense-in-depth. A future contributor who
// adds a service contract with a typo in the mode string must trip the
// validator, not silently allow operations.
func TestUnknownModeIsRejected(t *testing.T) {
	dep := DependencyContract{Name: "made_up", Mode: Mode("not_a_real_mode")}
	if ok, reason := AllowOperation(dep, OperationReadOnly); ok {
		t.Fatalf("unknown mode must refuse operations, got allowed")
	} else if !strings.Contains(reason, "unknown mode") {
		t.Fatalf("refusal reason must mention unknown mode, got: %q", reason)
	}
}
