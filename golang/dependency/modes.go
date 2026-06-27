// Package dependency defines the per-service contract for what happens
// when a required dependency is unhealthy. Every critical service must
// declare its dependency modes here; consumers (workflow dispatch, service
// startup, doctor preflight) read the contract to decide whether an
// operation may proceed. See
// docs/intent/service.dependency_degradation_modes.yaml.
package dependency

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Mode describes how a service behaves when a single required dependency
// is unhealthy. Modes are graded from least- to most-restrictive: Ignore
// (continue normally) → Degraded (some features off) → ReadOnly (no
// writes) → Hold (queue / refuse new work) → Stop (do not accept anything).
type Mode string

const (
	// ModeIgnore — the dependency is informational; the service continues
	// without any change in behavior.
	ModeIgnore Mode = "ignore"

	// ModeDegraded — specific features that need this dependency are
	// disabled. Other features continue.
	ModeDegraded Mode = "degraded"

	// ModeReadOnly — the service answers read requests but refuses writes.
	// Used for state-mutation surfaces whose authority lives in the
	// unavailable dependency.
	ModeReadOnly Mode = "read_only"

	// ModeHold — the service refuses new work (queueing or 503) but
	// continues to serve in-flight requests. Recovery happens when the
	// dependency returns.
	ModeHold Mode = "hold"

	// ModeStop — the service must not start, must not accept any work,
	// and must surface the dependency failure to operators. Used for
	// strict prerequisites whose absence makes the service unsafe.
	ModeStop Mode = "stop"
)

// IsKnown returns true when m is one of the declared modes. Unknown modes
// must be rejected at validation time — silence is not "ignore".
func (m Mode) IsKnown() bool {
	switch m {
	case ModeIgnore, ModeDegraded, ModeReadOnly, ModeHold, ModeStop:
		return true
	}
	return false
}

// blocksDispatch reports whether the mode prevents the workflow engine
// from sending new dispatches that need this dependency. Hold and Stop
// block; ReadOnly only blocks write-class operations (caller decides).
func (m Mode) blocksDispatch() bool {
	switch m {
	case ModeHold, ModeStop:
		return true
	}
	return false
}

// DependencyContract is one row of a service's dependency declaration.
type DependencyContract struct {
	// Name of the dependency (e.g. "etcd", "minio", "scylladb", "workflow").
	Name string

	// Mode that applies when this dependency is unhealthy.
	Mode Mode

	// AuthorityRole names what role the dependency plays for this service —
	// e.g. "config_truth", "object_store", "schema_db". Operator-facing.
	AuthorityRole string

	// AllowedOperations lists operation names the service may still perform
	// while the dependency is unhealthy. Empty means "everything except
	// what is explicitly blocked by Mode."
	AllowedOperations []string

	// BlockedOperations lists operation names that MUST be refused when
	// this dependency is unhealthy. The list is exhaustive — anything not
	// here remains permitted by Mode's default.
	BlockedOperations []string

	// OperatorMessage is the human-readable explanation surfaced to
	// operators when the breaker triggers.
	OperatorMessage string
}

// ServiceContract is the full dependency declaration for one service.
type ServiceContract struct {
	ServiceID    string
	Dependencies []DependencyContract
}

// For returns the DependencyContract for depName, or zero (Name: "") when
// the service did not declare that dependency. Callers should treat a
// missing dependency as "no contract" (panic-conservative).
func (c *ServiceContract) For(depName string) DependencyContract {
	if c == nil {
		return DependencyContract{}
	}
	for _, d := range c.Dependencies {
		if strings.EqualFold(strings.TrimSpace(d.Name), strings.TrimSpace(depName)) {
			return d
		}
	}
	return DependencyContract{}
}

// HasMode returns true when the service declared at least one dependency
// with mode m. Used by tests to assert every critical service declares its
// hard-stop dependencies.
func (c *ServiceContract) HasMode(m Mode) bool {
	for _, d := range c.Dependencies {
		if d.Mode == m {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────
// Registry: every critical service registers its contract once at init.
// Tests assert the registry is non-empty and covers expected services.
// ─────────────────────────────────────────────────────────────────────

var (
	registryMu sync.RWMutex
	registry   = map[string]*ServiceContract{}
)

// Register adds or replaces a service's dependency contract.
func Register(c *ServiceContract) {
	if c == nil || strings.TrimSpace(c.ServiceID) == "" {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[c.ServiceID] = c
}

// Lookup returns the contract for serviceID, or nil when unregistered.
func Lookup(serviceID string) *ServiceContract {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[serviceID]
}

// RegisteredServices returns the sorted list of services that have a
// contract. Useful for "every critical service must declare" assertions.
func RegisteredServices() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// ─────────────────────────────────────────────────────────────────────
// Operation policy decisions
// ─────────────────────────────────────────────────────────────────────

// Operation tags a request the service might perform. The dependency
// contract uses these to allow or block specific operations even when
// the broad Mode would otherwise permit them.
type Operation string

const (
	OperationReadOnly Operation = "read"
	OperationWrite    Operation = "write"
	OperationDispatch Operation = "dispatch"
)

// AllowOperation reports whether op may proceed for a service whose
// dependency dep is in the given mode. The second return value names the
// policy that fired (or "" when allowed).
//
// Decision matrix:
//
//	mode      | read      | write     | dispatch
//	--------- | --------- | --------- | ---------
//	ignore    | allow     | allow     | allow
//	degraded  | allow     | allow*    | allow*       (* unless BlockedOperations names it)
//	read_only | allow     | block     | block
//	hold      | allow     | block     | block
//	stop      | block     | block     | block
//
// Explicit BlockedOperations on the contract always wins.
func AllowOperation(dep DependencyContract, op Operation) (bool, string) {
	if dep.Name == "" {
		// No declared contract — caller must decide. Default deny is too
		// strict; default allow is what every service does today.
		return true, ""
	}
	if !dep.Mode.IsKnown() {
		return false, fmt.Sprintf("dependency %s declared unknown mode %q", dep.Name, dep.Mode)
	}
	// Explicit block list wins regardless of mode.
	for _, blocked := range dep.BlockedOperations {
		if strings.EqualFold(blocked, string(op)) {
			return false, fmt.Sprintf("dependency %s mode=%s blocks operation %s: %s",
				dep.Name, dep.Mode, op, firstNonEmpty(dep.OperatorMessage, "operation explicitly blocked"))
		}
	}
	switch dep.Mode {
	case ModeIgnore, ModeDegraded:
		return true, ""
	case ModeReadOnly:
		if op == OperationReadOnly {
			return true, ""
		}
		return false, fmt.Sprintf("dependency %s is in read_only mode: %s",
			dep.Name, firstNonEmpty(dep.OperatorMessage, "writes are not safe while dependency is unhealthy"))
	case ModeHold:
		if op == OperationReadOnly {
			return true, ""
		}
		return false, fmt.Sprintf("dependency %s is in hold mode: %s",
			dep.Name, firstNonEmpty(dep.OperatorMessage, "new work is queued until dependency recovers"))
	case ModeStop:
		return false, fmt.Sprintf("dependency %s is in stop mode: %s",
			dep.Name, firstNonEmpty(dep.OperatorMessage, "service must not act while dependency is unhealthy"))
	}
	return false, fmt.Sprintf("dependency %s: unhandled mode %q", dep.Name, dep.Mode)
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// ─────────────────────────────────────────────────────────────────────
// Built-in declarations for the critical services. Adding a service here
// without a corresponding production wiring is fine — the registry only
// surfaces contracts; consumers opt in.
// ─────────────────────────────────────────────────────────────────────

func init() {
	// repository — declarations mirror the live enforcement in
	// golang/repository/repository_server/dep_health.go. Keep these in
	// sync; the package-level test in
	// golang/repository/repository_server/dependency_contract_test.go
	// asserts they agree at CI time.
	//
	// ScyllaDB: indexed read+write authority; both CapRepoWrite and
	// CapRepoQuery are blocked when it's unhealthy, but CapRepoRead
	// (local POSIX CAS) still answers — hence read_only.
	//
	// MinIO: NOT a repository dependency. Packages never live in MinIO —
	// the local POSIX CAS is the sole blob authority (operator decision
	// 2026-06-12). There is no mirror tier to degrade.
	//
	// etcd: hard startup prerequisite — the service can't bootstrap its
	// config or register itself without etcd. Not enforced at the RPC
	// boundary (dep_health.go doesn't probe etcd) because the service
	// can't run at all without it; controller-side observation handles
	// total absence.
	Register(&ServiceContract{
		ServiceID: "repository",
		Dependencies: []DependencyContract{
			{
				Name:            "etcd",
				Mode:            ModeStop,
				AuthorityRole:   "config_truth",
				OperatorMessage: "repository cannot bootstrap or register without etcd config authority",
			},
			{
				Name:            "scylladb",
				Mode:            ModeReadOnly,
				AuthorityRole:   "package_index",
				OperatorMessage: "ScyllaDB unavailable: write/query capabilities blocked; verified local reads available",
			},
		},
	})

	// workflow — dispatch needs etcd (work queue) and ScyllaDB (run history).
	Register(&ServiceContract{
		ServiceID: "workflow",
		Dependencies: []DependencyContract{
			{
				Name:            "etcd",
				Mode:            ModeStop,
				AuthorityRole:   "work_queue",
				OperatorMessage: "workflow engine cannot dispatch without etcd",
			},
			{
				Name:            "scylladb",
				Mode:            ModeHold,
				AuthorityRole:   "run_history",
				OperatorMessage: "workflow run history requires ScyllaDB; new runs queued",
			},
		},
	})

	// cluster_doctor — needs etcd for audit and snapshots; cluster_controller
	// for cluster reports.
	Register(&ServiceContract{
		ServiceID: "cluster_doctor",
		Dependencies: []DependencyContract{
			{
				Name:            "etcd",
				Mode:            ModeReadOnly,
				AuthorityRole:   "audit_store",
				OperatorMessage: "audit writes require etcd; doctor read-only without it",
			},
			{
				Name:            "cluster_controller",
				Mode:            ModeDegraded,
				AuthorityRole:   "cluster_report_source",
				OperatorMessage: "doctor reports degrade to per-node info when controller unhealthy",
			},
		},
	})
}

// OperationRead is an alias spelling for OperationReadOnly so dependency
// declarations can use the more natural plain "read" token in
// BlockedOperations lists.
const OperationRead = OperationReadOnly
