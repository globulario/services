package repositorypb

import "testing"

func TestValidStateTransition_FullMatrix(t *testing.T) {
	type tc struct {
		from, to PublishState
		want     bool
	}

	tests := []tc{
		// ── Original pipeline transitions ──
		{PublishState_PUBLISH_STATE_UNSPECIFIED, PublishState_VERIFIED, true},
		{PublishState_STAGING, PublishState_VERIFIED, true},
		{PublishState_VERIFIED, PublishState_PUBLISHED, true},
		{PublishState_VERIFIED, PublishState_ORPHANED, true},
		{PublishState_PUBLISHED, PublishState_PUBLISHED, true}, // idempotent

		// ── Any → FAILED ──
		{PublishState_PUBLISH_STATE_UNSPECIFIED, PublishState_FAILED, true},
		{PublishState_STAGING, PublishState_FAILED, true},
		{PublishState_VERIFIED, PublishState_FAILED, true},
		{PublishState_PUBLISHED, PublishState_FAILED, true},
		{PublishState_DEPRECATED, PublishState_FAILED, true},
		{PublishState_YANKED, PublishState_FAILED, true},
		{PublishState_QUARANTINED, PublishState_FAILED, true},
		{PublishState_REVOKED, PublishState_FAILED, true},

		// ── PUBLISHED → lifecycle states ──
		{PublishState_PUBLISHED, PublishState_DEPRECATED, true},
		{PublishState_PUBLISHED, PublishState_YANKED, true},
		{PublishState_PUBLISHED, PublishState_QUARANTINED, true},
		{PublishState_PUBLISHED, PublishState_REVOKED, false}, // must go through intermediate

		// ── DEPRECATED transitions ──
		{PublishState_DEPRECATED, PublishState_PUBLISHED, true},  // un-deprecate
		{PublishState_DEPRECATED, PublishState_YANKED, true},
		{PublishState_DEPRECATED, PublishState_REVOKED, true},
		{PublishState_DEPRECATED, PublishState_QUARANTINED, false},
		{PublishState_DEPRECATED, PublishState_DEPRECATED, false},

		// ── YANKED transitions ──
		{PublishState_YANKED, PublishState_PUBLISHED, true}, // un-yank
		{PublishState_YANKED, PublishState_REVOKED, true},
		{PublishState_YANKED, PublishState_DEPRECATED, false},
		{PublishState_YANKED, PublishState_QUARANTINED, false},
		{PublishState_YANKED, PublishState_YANKED, false},

		// ── QUARANTINED transitions ──
		{PublishState_QUARANTINED, PublishState_PUBLISHED, true}, // un-quarantine
		{PublishState_QUARANTINED, PublishState_REVOKED, true},
		{PublishState_QUARANTINED, PublishState_DEPRECATED, false},
		{PublishState_QUARANTINED, PublishState_YANKED, false},
		{PublishState_QUARANTINED, PublishState_QUARANTINED, false},

		// ── REVOKED is terminal ──
		{PublishState_REVOKED, PublishState_PUBLISHED, false},
		{PublishState_REVOKED, PublishState_DEPRECATED, false},
		{PublishState_REVOKED, PublishState_YANKED, false},
		{PublishState_REVOKED, PublishState_QUARANTINED, false},
		{PublishState_REVOKED, PublishState_REVOKED, false},

		// ── Invalid forward jumps ──
		{PublishState_STAGING, PublishState_PUBLISHED, false},
		{PublishState_STAGING, PublishState_DEPRECATED, false},
		{PublishState_PUBLISH_STATE_UNSPECIFIED, PublishState_PUBLISHED, false},
		{PublishState_ORPHANED, PublishState_PUBLISHED, false},
		{PublishState_FAILED, PublishState_PUBLISHED, false},
	}

	for _, tt := range tests {
		got := ValidStateTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("ValidStateTransition(%s → %s) = %v, want %v",
				tt.from, tt.to, got, tt.want)
		}
	}
}

func TestValidPromoteTransition_BackwardCompat(t *testing.T) {
	tests := []struct {
		from, to PublishState
		want     bool
	}{
		{PublishState_PUBLISH_STATE_UNSPECIFIED, PublishState_VERIFIED, true},
		{PublishState_STAGING, PublishState_VERIFIED, true},
		{PublishState_VERIFIED, PublishState_PUBLISHED, true},
		{PublishState_VERIFIED, PublishState_ORPHANED, true},
		{PublishState_PUBLISHED, PublishState_PUBLISHED, true},
		{PublishState_PUBLISHED, PublishState_DEPRECATED, false}, // old function doesn't know about DEPRECATED
		{PublishState_STAGING, PublishState_PUBLISHED, false},
	}

	for _, tt := range tests {
		got := ValidPromoteTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("ValidPromoteTransition(%s → %s) = %v, want %v",
				tt.from, tt.to, got, tt.want)
		}
	}
}

func TestIsTerminalState(t *testing.T) {
	if !IsTerminalState(PublishState_REVOKED) {
		t.Error("REVOKED should be terminal")
	}
	for _, s := range []PublishState{
		PublishState_PUBLISHED, PublishState_DEPRECATED,
		PublishState_YANKED, PublishState_QUARANTINED,
	} {
		if IsTerminalState(s) {
			t.Errorf("%s should not be terminal", s)
		}
	}
}

func TestIsDownloadBlocked(t *testing.T) {
	blocked := []PublishState{PublishState_YANKED, PublishState_QUARANTINED, PublishState_REVOKED}
	for _, s := range blocked {
		if !IsDownloadBlocked(s) {
			t.Errorf("%s should block downloads", s)
		}
	}
	notBlocked := []PublishState{PublishState_PUBLISHED, PublishState_DEPRECATED, PublishState_STAGING}
	for _, s := range notBlocked {
		if IsDownloadBlocked(s) {
			t.Errorf("%s should not block downloads", s)
		}
	}
}

func TestIsDiscoveryHidden(t *testing.T) {
	hidden := []PublishState{PublishState_YANKED, PublishState_QUARANTINED, PublishState_REVOKED,
		PublishState_ORPHANED, PublishState_FAILED, PublishState_STAGING}
	for _, s := range hidden {
		if !IsDiscoveryHidden(s) {
			t.Errorf("%s should be hidden from discovery", s)
		}
	}
	visible := []PublishState{PublishState_PUBLISHED, PublishState_DEPRECATED}
	for _, s := range visible {
		if IsDiscoveryHidden(s) {
			t.Errorf("%s should be visible in discovery", s)
		}
	}
}

func TestIsEligibleForLatestResolve(t *testing.T) {
	eligible := []PublishState{PublishState_PUBLISHED, PublishState_PUBLISH_STATE_UNSPECIFIED}
	for _, s := range eligible {
		if !IsEligibleForLatestResolve(s) {
			t.Errorf("%s should be eligible for latest resolve", s)
		}
	}
	notEligible := []PublishState{PublishState_DEPRECATED, PublishState_YANKED,
		PublishState_QUARANTINED, PublishState_REVOKED, PublishState_STAGING}
	for _, s := range notEligible {
		if IsEligibleForLatestResolve(s) {
			t.Errorf("%s should NOT be eligible for latest resolve", s)
		}
	}
}

func TestIsInstallableByPin(t *testing.T) {
	installable := []PublishState{PublishState_PUBLISHED, PublishState_DEPRECATED, PublishState_PUBLISH_STATE_UNSPECIFIED}
	for _, s := range installable {
		if !IsInstallableByPin(s) {
			t.Errorf("%s should be installable by pin", s)
		}
	}
	notInstallable := []PublishState{PublishState_YANKED, PublishState_QUARANTINED, PublishState_REVOKED}
	for _, s := range notInstallable {
		if IsInstallableByPin(s) {
			t.Errorf("%s should NOT be installable by pin", s)
		}
	}
}

func TestRequiresWarning(t *testing.T) {
	if !RequiresWarning(PublishState_DEPRECATED) {
		t.Error("DEPRECATED should require warning")
	}
	for _, s := range []PublishState{PublishState_PUBLISHED, PublishState_YANKED, PublishState_REVOKED} {
		if RequiresWarning(s) {
			t.Errorf("%s should not require warning", s)
		}
	}
}
