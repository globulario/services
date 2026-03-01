package rules

import "strings"

// UnitStateEnum is a canonical representation of a systemd unit state,
// normalized from the raw strings reported by NodeAgent.
type UnitStateEnum int

const (
	UnitStateUnknown      UnitStateEnum = iota
	UnitStateActive                     // "active", "running"
	UnitStateInactive                   // "inactive", "dead"
	UnitStateFailed                     // "failed"
	UnitStateDisabled                   // "disabled"
	UnitStateNotFound                   // "not-found", "missing"
	UnitStateActivating                 // "activating"
	UnitStateDeactivating               // "deactivating"
)

func (s UnitStateEnum) String() string {
	switch s {
	case UnitStateActive:
		return "active"
	case UnitStateInactive:
		return "inactive"
	case UnitStateFailed:
		return "failed"
	case UnitStateDisabled:
		return "disabled"
	case UnitStateNotFound:
		return "not-found"
	case UnitStateActivating:
		return "activating"
	case UnitStateDeactivating:
		return "deactivating"
	default:
		return "unknown"
	}
}

// NormalizeUnitState maps agent-reported strings to a canonical UnitStateEnum.
// Add new strings here as the agent evolves — this is the single source of truth.
func NormalizeUnitState(s string) UnitStateEnum {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "active", "running":
		return UnitStateActive
	case "inactive", "dead":
		return UnitStateInactive
	case "failed":
		return UnitStateFailed
	case "disabled":
		return UnitStateDisabled
	case "not-found", "not_found", "notfound", "missing":
		return UnitStateNotFound
	case "activating":
		return UnitStateActivating
	case "deactivating":
		return UnitStateDeactivating
	default:
		return UnitStateUnknown
	}
}
