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

// TestRepositoryHasNoMinioDependency — contract test. Packages never live in
// MinIO; the local POSIX CAS is the sole blob authority (operator decision
// 2026-06-12). The repository must therefore declare NO minio/mirror
// dependency. The ScyllaDB dependency (package index) is the one that drops the
// repository to read_only. See dep_health.go.
func TestRepositoryHasNoMinioDependency(t *testing.T) {
	c := Lookup("repository")
	if c == nil {
		t.Fatal("repository contract missing")
	}
	if dep := c.For("minio"); dep.Name != "" {
		t.Fatalf("repository must NOT declare a minio dependency — packages never live in MinIO (got mode %s)", dep.Mode)
	}

	// The ScyllaDB dependency IS the one that drops the repository to
	// read_only — primary metadata authority for the package index.
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
