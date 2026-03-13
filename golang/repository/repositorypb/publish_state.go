package repositorypb

// ── Lifecycle State Behavior Semantics ──────────────────────────────────────
//
// Each publish state has explicit behavioral rules:
//
// | State         | Search | Latest Resolve | Download | Pin Install | Rollback | Owner-Only | Warning/Error | Who May Transition To      |
// |---------------|--------|----------------|----------|-------------|----------|------------|---------------|----------------------------|
// | PUBLISHED     | yes    | yes            | yes      | yes         | yes      | no         | none          | publisher / owner / admin   |
// | DEPRECATED    | yes    | no (skip)      | yes+warn | yes         | yes      | no         | warning       | publisher / owner / admin   |
// | YANKED        | no     | no             | no       | no          | no       | yes        | hard error    | publisher / owner / admin   |
// | QUARANTINED   | no     | no             | no       | no          | no       | yes        | hard error    | admin only (moderation)     |
// | REVOKED       | no     | no             | no       | no          | no       | yes        | hard error    | admin or owner (self-revoke)|
// | ORPHANED      | no     | no             | no       | no          | no       | no         | none          | system                      |
// | FAILED        | no     | no             | no       | no          | no       | no         | none          | system                      |
// | STAGING       | no     | no             | no       | no          | no       | no         | none          | system                      |
// | VERIFIED      | no     | no             | yes      | no          | no       | no         | none          | system                      |
//
// "Owner-Only" means visible/accessible only to namespace owners and admins.
//
// Authority rules for transitions:
//   - QUARANTINE (to or from): admin/superuser only — moderation action.
//   - REVOKE: admin OR namespace owner (self-revoke).
//   - All other transitions: any authorized publisher/owner.

// ValidPromoteTransition returns true if transitioning from `from` to `to` is allowed
// within the original publish pipeline (STAGING → VERIFIED → PUBLISHED).
// Allowed transitions:
//   - UNSPECIFIED/STAGING → VERIFIED (upload complete)
//   - VERIFIED → PUBLISHED (descriptor registered)
//   - VERIFIED → ORPHANED (descriptor registration failed)
//   - VERIFIED → FAILED (publish pipeline failed)
//   - PUBLISHED → PUBLISHED (idempotent)
//   - * → FAILED (any state can transition to failed)
func ValidPromoteTransition(from, to PublishState) bool {
	if to == PublishState_FAILED {
		return true // anything can fail
	}
	switch from {
	case PublishState_PUBLISH_STATE_UNSPECIFIED, PublishState_STAGING:
		return to == PublishState_VERIFIED
	case PublishState_VERIFIED:
		return to == PublishState_PUBLISHED || to == PublishState_ORPHANED
	case PublishState_PUBLISHED:
		return to == PublishState_PUBLISHED // idempotent
	default:
		return false
	}
}

// ValidStateTransition returns true if a lifecycle state transition from → to is allowed.
// This extends ValidPromoteTransition with the full lifecycle management states:
//
//   - PUBLISHED → DEPRECATED, YANKED, QUARANTINED
//   - DEPRECATED → PUBLISHED (un-deprecate), YANKED, REVOKED
//   - YANKED → PUBLISHED (un-yank), REVOKED
//   - QUARANTINED → PUBLISHED (un-quarantine), REVOKED
//   - REVOKED → (terminal, no transitions out)
//   - Any → FAILED (existing rule)
//
// The original pipeline transitions (STAGING→VERIFIED→PUBLISHED) are also valid.
func ValidStateTransition(from, to PublishState) bool {
	// Anything can fail.
	if to == PublishState_FAILED {
		return true
	}

	// Original pipeline transitions.
	switch from {
	case PublishState_PUBLISH_STATE_UNSPECIFIED, PublishState_STAGING:
		return to == PublishState_VERIFIED

	case PublishState_VERIFIED:
		return to == PublishState_PUBLISHED || to == PublishState_ORPHANED

	case PublishState_PUBLISHED:
		switch to {
		case PublishState_PUBLISHED: // idempotent
			return true
		case PublishState_DEPRECATED, PublishState_YANKED, PublishState_QUARANTINED:
			return true
		}
		return false

	case PublishState_DEPRECATED:
		switch to {
		case PublishState_PUBLISHED: // un-deprecate
			return true
		case PublishState_YANKED, PublishState_REVOKED:
			return true
		}
		return false

	case PublishState_YANKED:
		switch to {
		case PublishState_PUBLISHED: // un-yank
			return true
		case PublishState_REVOKED:
			return true
		}
		return false

	case PublishState_QUARANTINED:
		switch to {
		case PublishState_PUBLISHED: // un-quarantine
			return true
		case PublishState_REVOKED:
			return true
		}
		return false

	case PublishState_REVOKED:
		// Terminal — no transitions out.
		return false

	default:
		return false
	}
}

// IsTerminalState returns true if the state is terminal (no further transitions allowed
// except to FAILED).
func IsTerminalState(s PublishState) bool {
	return s == PublishState_REVOKED
}

// IsDownloadBlocked returns true if artifacts in this state should not be downloadable
// by non-owners/non-admins. Per behavior semantics: YANKED, QUARANTINED, REVOKED.
func IsDownloadBlocked(s PublishState) bool {
	return s == PublishState_YANKED || s == PublishState_QUARANTINED || s == PublishState_REVOKED
}

// IsDiscoveryHidden returns true if artifacts in this state should be hidden from
// search/list results for non-owners/non-admins.
// Per behavior semantics: YANKED, QUARANTINED, REVOKED, ORPHANED, FAILED, STAGING.
func IsDiscoveryHidden(s PublishState) bool {
	switch s {
	case PublishState_YANKED, PublishState_QUARANTINED, PublishState_REVOKED:
		return true
	case PublishState_ORPHANED, PublishState_FAILED, PublishState_STAGING:
		return true
	default:
		return false
	}
}

// IsEligibleForLatestResolve returns true if an artifact in this state can be picked
// by the "latest published" resolver. Only PUBLISHED qualifies.
// DEPRECATED is explicitly excluded from latest resolution (must be pinned explicitly).
func IsEligibleForLatestResolve(s PublishState) bool {
	return s == PublishState_PUBLISHED || s == PublishState_PUBLISH_STATE_UNSPECIFIED
}

// IsInstallableByPin returns true if an artifact in this state can be installed
// when explicitly pinned by version. DEPRECATED artifacts are installable by pin
// (with warning), but YANKED/QUARANTINED/REVOKED are not.
func IsInstallableByPin(s PublishState) bool {
	switch s {
	case PublishState_PUBLISHED, PublishState_PUBLISH_STATE_UNSPECIFIED:
		return true
	case PublishState_DEPRECATED:
		return true // installable but should emit warning
	default:
		return false
	}
}

// RequiresWarning returns true if operations on artifacts in this state should
// emit a warning (but not block).
func RequiresWarning(s PublishState) bool {
	return s == PublishState_DEPRECATED
}
