// Package operational defines the common capability/degraded-mode model for
// Globular services.
//
// Every service declares:
//   - required dependencies (blocking when unavailable)
//   - optional dependencies (degrade, never block required capabilities)
//   - capabilities and which dependencies each needs
//   - the composite operating mode
//
// Services express their live state as a ServiceOperationalStatus, which is
// consumed by the cluster_doctor and exposed via a service-specific RPC
// (e.g. GetRepositoryStatus). The grpc health protocol (SERVING / NOT_SERVING)
// is orthogonal: a service may be SERVING with mode DEGRADED or READ_ONLY.
package operational

import "time"

// ── Service-level mode ────────────────────────────────────────────────────────

// ServiceHealthMode is the composite operating mode of a service.
//
//	FULL        — all capabilities available, all dependencies healthy
//	DEGRADED    — optional dependency down; core capabilities available
//	READ_ONLY   — required dependency partially down; writes blocked, reads may work
//	LOCAL_ONLY  — no distributed deps available; only locally-verified data served
//	UNAVAILABLE — service cannot serve any capability
type ServiceHealthMode string

const (
	ModeFull        ServiceHealthMode = "FULL"
	ModeDegraded    ServiceHealthMode = "DEGRADED"
	ModeReadOnly    ServiceHealthMode = "READ_ONLY"
	ModeLocalOnly   ServiceHealthMode = "LOCAL_ONLY"
	ModeUnavailable ServiceHealthMode = "UNAVAILABLE"
)

// ── Per-capability status ─────────────────────────────────────────────────────

// CapabilityStatus is the live status of a named service capability.
//
//	AVAILABLE — operating normally
//	DEGRADED  — available but with reduced guarantees (e.g. mirror skipped)
//	DISABLED  — explicitly turned off by policy or operator
//	BLOCKED   — required dependency unavailable; capability cannot function
//	UNKNOWN   — status not yet determined
type CapabilityStatus string

const (
	CapAvailable CapabilityStatus = "AVAILABLE"
	CapDegraded  CapabilityStatus = "DEGRADED"
	CapDisabled  CapabilityStatus = "DISABLED"
	CapBlocked   CapabilityStatus = "BLOCKED"
	CapUnknown   CapabilityStatus = "UNKNOWN"
)

// ── Dependency classification ─────────────────────────────────────────────────

// DependencyKind classifies a dependency's architectural role.
//
//	REQUIRED  — service must block affected capabilities when this dep is down
//	OPTIONAL  — failure degrades but never blocks a REQUIRED capability
//	AUTHORITY — canonical source of truth for data correctness
//	CACHE     — speeds up reads; loss causes slower but correct results
//	MIRROR    — replication target; loss causes write-behind loss only
//	INDEX     — search/query layer; loss makes discovery unavailable
//	TELEMETRY — metrics/tracing; loss never affects data-plane capabilities
//	EXTERNAL  — external third-party; same rules as OPTIONAL
type DependencyKind string

const (
	DepRequired  DependencyKind = "REQUIRED"
	DepOptional  DependencyKind = "OPTIONAL"
	DepAuthority DependencyKind = "AUTHORITY"
	DepCache     DependencyKind = "CACHE"
	DepMirror    DependencyKind = "MIRROR"
	DepIndex     DependencyKind = "INDEX"
	DepTelemetry DependencyKind = "TELEMETRY"
	DepExternal  DependencyKind = "EXTERNAL"
)

// DependencyStatus is the live health of a single dependency.
//
//	HEALTHY       — reachable and functioning correctly
//	DEGRADED      — reachable but with elevated errors or reduced throughput
//	UNAVAILABLE   — unreachable or refusing connections
//	MISCONFIGURED — reachable but configuration is invalid
//	UNSAFE        — operating in a way that violates a data-safety invariant
type DependencyStatus string

const (
	DepHealthy       DependencyStatus = "HEALTHY"
	DepDegraded      DependencyStatus = "DEGRADED"
	DepUnavailable   DependencyStatus = "UNAVAILABLE"
	DepMisconfigured DependencyStatus = "MISCONFIGURED"
	DepUnsafe        DependencyStatus = "UNSAFE"
)

// ── Structs ───────────────────────────────────────────────────────────────────

// DependencyHealth describes one dependency's live state and which capabilities
// are affected when it is unhealthy.
type DependencyHealth struct {
	Name                string
	Kind                DependencyKind
	Status              DependencyStatus
	Reason              string
	AffectsCapabilities []string // capability names that degrade/block when this dep is down
}

// CapabilityHealth describes the live status of one named capability.
type CapabilityHealth struct {
	Name   string
	Status CapabilityStatus
	Mode   ServiceHealthMode // the sub-mode this capability operates in
	Reason string
}

// ServiceOperationalStatus is the full live operational picture for a service.
// It is returned by service-specific status RPCs (e.g. GetRepositoryStatus).
// It is NOT the grpc health protocol — a service may be SERVING (grpc) with
// mode DEGRADED or READ_ONLY (operational).
type ServiceOperationalStatus struct {
	Service        string
	Mode           ServiceHealthMode
	Reason         string
	Dependencies   []DependencyHealth
	Capabilities   []CapabilityHealth
	ObservedAt     time.Time
	ObservedAtUnix int64
}
