// Package fallback is the Phase 6 primitive of the Diagnostic Honesty
// Refactor. It registers an in-process directory of currently-active
// fallback paths so that:
//
//   - Doctor / verifier code can call Snapshot() to enumerate them.
//   - Operators see a structured finding (service.silent_fallback_active)
//     instead of having to grep slog warnings.
//   - Future Phase 9 work can wire a "ScrapeFallbacks" RPC against any
//     service that calls Enter().
//
// The brief is explicit: fallback may keep the user-facing path alive,
// but it MUST NOT keep the system green. Every fallback path is required
// to call Enter() when it starts using the substitute mode, and Exit()
// when the primary dependency is back. Silent fallback is the failure;
// noisy-but-degraded fallback is the contract.
//
// The package is intentionally tiny: no global I/O, no event bus
// coupling, no goroutines. A service that wants to publish each
// transition to its own cluster-event channel sets a Notifier via
// SetNotifier; otherwise the registry is enough for Snapshot scraping.
package fallback

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// FindingID is the canonical doctor / cluster_doctor failure-mode id
// emitted whenever a fallback path is active. Mirror the value in
// docs/awareness/failure_modes.yaml.
const FindingID = "service.silent_fallback_active"

// Active describes one fallback that is currently in effect. The fields
// match the schema in the Phase 6 brief so a verifier can map directly
// from an Active into a doctor finding without re-shaping.
type Active struct {
	// Service is the canonical service name (e.g. "repository", "dns",
	// "file"). Required.
	Service string
	// Dependency is the primary the service is falling back AWAY from
	// (e.g. "scylladb", "etcd", "minio"). Required.
	Dependency string
	// Mode names the substitute the service is now using (e.g.
	// "minio_read", "in_memory_cache", "local_disk", "read_only").
	// Required — operators key on this to decide remediation.
	Mode string
	// PrimaryError carries the error string that triggered fallback.
	// Optional but strongly recommended; without it, doctor output
	// loses the root-cause line.
	PrimaryError string
	// AffectedPaths is the list of resources or request paths that are
	// being served from the substitute. Optional. e.g. for file service:
	// the bucket(s) being read from local disk.
	AffectedPaths []string
	// NodeID is the cluster node id this fallback is happening on.
	// Optional in tests; production callers should set it.
	NodeID string
	// Since is when the fallback started. If zero, Enter() stamps now().
	Since time.Time
}

// Notifier is an optional hook services use to mirror Enter/Exit
// transitions into their own structured event channel (cluster events,
// audit log, etc.). Empty by default — the in-process registry is the
// minimum useful surface.
type Notifier interface {
	// OnEnter is called the first time a key transitions from absent to
	// present in the registry. Re-entering the same key is idempotent
	// and does NOT re-fire OnEnter.
	OnEnter(a Active)
	// OnExit is called when a key transitions back to absent.
	OnExit(a Active)
}

// registry is the package-level directory of active fallbacks. Keyed by
// service+dependency+mode so two unrelated services don't collide.
var (
	registry   sync.Map // map[string]Active
	notifierMu sync.RWMutex
	notifier   Notifier
)

// SetNotifier installs an optional Notifier. Pass nil to clear.
func SetNotifier(n Notifier) {
	notifierMu.Lock()
	defer notifierMu.Unlock()
	notifier = n
}

func currentNotifier() Notifier {
	notifierMu.RLock()
	defer notifierMu.RUnlock()
	return notifier
}

// keyFor builds the registry key. Trimmed + lower-cased so accidental
// case drift between callsites doesn't fragment the directory.
func keyFor(service, dependency, mode string) string {
	return strings.ToLower(strings.TrimSpace(service)) +
		"|" + strings.ToLower(strings.TrimSpace(dependency)) +
		"|" + strings.ToLower(strings.TrimSpace(mode))
}

// Enter marks a fallback as active. Idempotent: if the same
// service+dependency+mode key is already present the existing record is
// preserved (its Since timestamp does not get reset) so that downstream
// "how long have we been degraded" calculations stay honest. The
// PrimaryError and AffectedPaths fields ARE refreshed on re-entry
// because operators want the latest evidence.
//
// Returns the Active record actually stored (with Since populated).
// Callers should use this return value when logging — its Since is
// guaranteed non-zero.
func Enter(a Active) Active {
	if a.Service == "" || a.Dependency == "" || a.Mode == "" {
		return a
	}
	k := keyFor(a.Service, a.Dependency, a.Mode)
	if a.Since.IsZero() {
		a.Since = time.Now()
	}
	// Preserve original Since on re-entry; refresh PrimaryError + Paths.
	stored := a
	if prev, ok := registry.Load(k); ok {
		p := prev.(Active)
		stored = p
		stored.PrimaryError = a.PrimaryError
		stored.AffectedPaths = a.AffectedPaths
		if a.NodeID != "" {
			stored.NodeID = a.NodeID
		}
		registry.Store(k, stored)
		return stored
	}
	registry.Store(k, stored)
	if n := currentNotifier(); n != nil {
		n.OnEnter(stored)
	}
	return stored
}

// Exit clears a fallback by service+dependency+mode. No-op if no
// matching record is present. Returns true when something was actually
// cleared (callers use this to gate "back to healthy" log lines).
func Exit(service, dependency, mode string) bool {
	k := keyFor(service, dependency, mode)
	prev, loaded := registry.LoadAndDelete(k)
	if !loaded {
		return false
	}
	if n := currentNotifier(); n != nil {
		n.OnExit(prev.(Active))
	}
	return true
}

// ExitMatching clears every active fallback that matches the predicate.
// Returns the cleared records so callers can log them. Used by recovery
// paths where the exact mode key isn't known but the dependency is
// (e.g. "Scylla quorum restored — clear everything that was falling
// back away from Scylla").
func ExitMatching(predicate func(Active) bool) []Active {
	var cleared []Active
	registry.Range(func(k, v any) bool {
		a := v.(Active)
		if predicate(a) {
			registry.Delete(k)
			cleared = append(cleared, a)
		}
		return true
	})
	if n := currentNotifier(); n != nil {
		for _, a := range cleared {
			n.OnExit(a)
		}
	}
	return cleared
}

// Snapshot returns every currently-active fallback. Order is stable
// (service, dependency, mode) so doctor output and test assertions are
// deterministic.
func Snapshot() []Active {
	var out []Active
	registry.Range(func(_, v any) bool {
		out = append(out, v.(Active))
		return true
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		if out[i].Dependency != out[j].Dependency {
			return out[i].Dependency < out[j].Dependency
		}
		return out[i].Mode < out[j].Mode
	})
	return out
}

// resetForTest clears the registry and notifier. Used only by package
// tests; not exported to other Globular code.
func resetForTest() {
	registry.Range(func(k, _ any) bool {
		registry.Delete(k)
		return true
	})
	SetNotifier(nil)
}
