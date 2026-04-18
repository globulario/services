package repositorypb

// ── Lifecycle State Behavior Semantics ──────────────────────────────────────
//
// Each publish state has explicit behavioral rules:
//
// | State         | Search | Latest Resolve | Download    | Pin Install | Rollback | Owner-Only | Warning/Error | Who May Transition To       |
// |---------------|--------|----------------|-------------|-------------|----------|------------|---------------|-----------------------------|
// | PUBLISHED     | yes    | yes            | yes         | yes         | yes      | no         | none          | publisher / owner / admin   |
// | DEPRECATED    | yes    | no (skip)      | yes+warn    | yes         | yes      | no         | warning       | publisher / owner / admin   |
// | YANKED        | no     | no             | no          | no          | no       | yes        | hard error    | publisher / owner / admin   |
// | QUARANTINED   | no     | no             | no          | no          | no       | yes        | hard error    | admin only (moderation)     |
// | REVOKED       | no     | no             | no          | no          | no       | yes        | hard error    | admin or owner (self-revoke)|
// | ARCHIVED      | no     | no             | owner/admin | no          | no       | yes        | none          | GC or admin                 |
// | ORPHANED      | no     | no             | no          | no          | no       | no         | none          | system                      |
// | FAILED        | no     | no             | no          | no          | no       | no         | none          | system                      |
// | CORRUPTED     | no     | no             | no          | no          | no       | yes        | hard error    | system (integrity check)    |
// | STAGING       | no     | no             | no          | no          | no       | no         | none          | system                      |
// | VERIFIED      | no     | no             | yes         | no          | no       | no         | none          | system (auto-promote)       |
//
// ARCHIVED semantics:
//   - Hidden from all discovery (search, list) except for owners and admins.
//   - Download is NOT blocked — owners/admins may still retrieve the binary.
//   - Cannot be promoted or installed. Binary is retained in MinIO.
//   - Transition source: PUBLISHED, DEPRECATED, VERIFIED, FAILED, ORPHANED (via GC).
//   - Transition target: REVOKED (admin can permanently revoke an archived artifact).
//   - Purpose: soft-delete that preserves binary while freeing catalog space.
//     A future purge step may hard-delete ARCHIVED artifacts.
//
// "Owner-Only" means visible/accessible only to namespace owners and admins.
//
// Authority rules for transitions:
//   - QUARANTINE (to or from): admin/superuser only — moderation action.
//   - REVOKE: admin OR namespace owner (self-revoke).
//   - ARCHIVE: GC reconciler or admin.
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
		return to == PublishState_PUBLISHED || to == PublishState_ORPHANED || to == PublishState_ARCHIVED

	case PublishState_PUBLISHED:
		switch to {
		case PublishState_PUBLISHED: // idempotent
			return true
		case PublishState_DEPRECATED, PublishState_YANKED, PublishState_QUARANTINED:
			return true
		case PublishState_ARCHIVED: // GC soft-delete
			return true
		}
		return false

	case PublishState_DEPRECATED:
		switch to {
		case PublishState_PUBLISHED: // un-deprecate
			return true
		case PublishState_YANKED, PublishState_REVOKED:
			return true
		case PublishState_ARCHIVED: // GC soft-delete
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

	case PublishState_CORRUPTED:
		switch to {
		case PublishState_PUBLISHED: // re-verified after fix
			return true
		case PublishState_REVOKED:
			return true
		}
		return false

	case PublishState_FAILED, PublishState_ORPHANED:
		// Intermediate states that can be soft-deleted by GC if abandoned.
		if to == PublishState_ARCHIVED {
			return true
		}
		return false

	case PublishState_ARCHIVED:
		// One-way soft-delete. Only admin REVOKE is allowed out.
		if to == PublishState_REVOKED {
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
// by non-owners/non-admins.
// ARCHIVED is NOT download-blocked — owners/admins may retrieve the binary.
func IsDownloadBlocked(s PublishState) bool {
	return s == PublishState_YANKED || s == PublishState_QUARANTINED || s == PublishState_REVOKED || s == PublishState_CORRUPTED
}

// IsDiscoveryHidden returns true if artifacts in this state should be hidden from
// search/list results for non-owners/non-admins.
// ARCHIVED is hidden — it is a soft-deleted artifact not intended for general use.
func IsDiscoveryHidden(s PublishState) bool {
	switch s {
	case PublishState_YANKED, PublishState_QUARANTINED, PublishState_REVOKED, PublishState_CORRUPTED:
		return true
	case PublishState_ARCHIVED:
		return true
	case PublishState_ORPHANED, PublishState_FAILED, PublishState_STAGING, PublishState_VERIFIED:
		return true
	default:
		return false
	}
}

// IsEligibleForLatestResolve returns true if an artifact in this state can be picked
// by the "latest published" resolver. Only PUBLISHED qualifies.
// DEPRECATED is explicitly excluded from latest resolution (must be pinned explicitly).
// PUBLISH_STATE_UNSPECIFIED is no longer accepted — the migration
// (migration.go) promotes legacy manifests to PUBLISHED on first startup.
func IsEligibleForLatestResolve(s PublishState) bool {
	return s == PublishState_PUBLISHED
}

// IsInstallableByPin returns true if an artifact in this state can be installed
// when explicitly pinned by version. DEPRECATED artifacts are installable by pin
// (with warning), but YANKED/QUARANTINED/REVOKED are not.
func IsInstallableByPin(s PublishState) bool {
	switch s {
	case PublishState_PUBLISHED:
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
