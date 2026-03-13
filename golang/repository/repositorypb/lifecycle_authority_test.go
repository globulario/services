package repositorypb

import "testing"

func TestValidStateTransition_OwnerMayDeprecate(t *testing.T) {
	if !ValidStateTransition(PublishState_PUBLISHED, PublishState_DEPRECATED) {
		t.Error("PUBLISHED -> DEPRECATED should be allowed")
	}
}

func TestValidStateTransition_OwnerMayYank(t *testing.T) {
	if !ValidStateTransition(PublishState_PUBLISHED, PublishState_YANKED) {
		t.Error("PUBLISHED -> YANKED should be allowed")
	}
}

func TestValidStateTransition_TerminalRevoked(t *testing.T) {
	// REVOKED is terminal -- no transitions out (except FAILED, tested separately).
	targets := []PublishState{
		PublishState_PUBLISHED,
		PublishState_DEPRECATED,
		PublishState_YANKED,
		PublishState_QUARANTINED,
		PublishState_REVOKED,
	}
	for _, to := range targets {
		if ValidStateTransition(PublishState_REVOKED, to) {
			t.Errorf("REVOKED -> %s should be disallowed (terminal state)", to)
		}
	}
}

func TestValidStateTransition_RevokedCanFail(t *testing.T) {
	// Even terminal states can transition to FAILED.
	if !ValidStateTransition(PublishState_REVOKED, PublishState_FAILED) {
		t.Error("REVOKED -> FAILED should be allowed (any state can fail)")
	}
}

func TestIsDownloadBlocked_StatesCorrect(t *testing.T) {
	blocked := map[PublishState]bool{
		PublishState_YANKED:       true,
		PublishState_QUARANTINED:  true,
		PublishState_REVOKED:      true,
		PublishState_PUBLISHED:    false,
		PublishState_DEPRECATED:   false,
		PublishState_STAGING:      false,
		PublishState_VERIFIED:     false,
		PublishState_FAILED:       false,
		PublishState_ORPHANED:     false,
	}
	for state, want := range blocked {
		got := IsDownloadBlocked(state)
		if got != want {
			t.Errorf("IsDownloadBlocked(%s) = %v, want %v", state, got, want)
		}
	}
}

func TestIsDiscoveryHidden_IncludesStagingOrphaned(t *testing.T) {
	// All hidden states per the behavior semantics table.
	hidden := []PublishState{
		PublishState_YANKED,
		PublishState_QUARANTINED,
		PublishState_REVOKED,
		PublishState_ORPHANED,
		PublishState_FAILED,
		PublishState_STAGING,
	}
	for _, s := range hidden {
		if !IsDiscoveryHidden(s) {
			t.Errorf("IsDiscoveryHidden(%s) should be true", s)
		}
	}

	// Visible states.
	visible := []PublishState{
		PublishState_PUBLISHED,
		PublishState_DEPRECATED,
		PublishState_VERIFIED,
	}
	for _, s := range visible {
		if IsDiscoveryHidden(s) {
			t.Errorf("IsDiscoveryHidden(%s) should be false", s)
		}
	}
}
