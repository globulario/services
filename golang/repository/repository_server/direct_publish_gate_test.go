package main

// direct_publish_gate_test.go — required tests for the release-authority gate on
// the direct publish path (UploadArtifact / `globular pkg publish` / MCP), P4.
//
// These prove "agent builds = DEV by construction": a caller with write access
// but no release.allocate cannot land STABLE — non-official publishers are
// downgraded to DEV, and the sealed official namespace is rejected (it cannot be
// DEV). Only STABLE is gated; an authorized caller is untouched.

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestDirectPublishGate_UnauthorizedNonOfficialStableForcedToDev(t *testing.T) {
	final, reject := directPublishChannelGate(repopb.ArtifactChannel_STABLE, "local@ryzen", false)
	if reject {
		t.Fatal("non-official publisher must not be rejected — it is downgraded")
	}
	if final != repopb.ArtifactChannel_DEV {
		t.Fatalf("unauthorized non-official STABLE must be forced to DEV; got %v", final)
	}
}

func TestDirectPublishGate_UnauthorizedOfficialStableRejected(t *testing.T) {
	final, reject := directPublishChannelGate(repopb.ArtifactChannel_STABLE, officialPublisher, false)
	if !reject {
		t.Fatalf("unauthorized official STABLE must be rejected (cannot be DEV per lane Rule 2); got final=%v", final)
	}
}

func TestDirectPublishGate_AuthorizedStableUntouched(t *testing.T) {
	for _, pub := range []string{officialPublisher, "acme", "org@example.io"} {
		final, reject := directPublishChannelGate(repopb.ArtifactChannel_STABLE, pub, true)
		if reject || final != repopb.ArtifactChannel_STABLE {
			t.Fatalf("authorized STABLE for %q must pass unchanged; got (final=%v, reject=%v)", pub, final, reject)
		}
	}
}

func TestDirectPublishGate_NonStableChannelsPassThrough(t *testing.T) {
	// Non-STABLE claims are never gated, regardless of authority or publisher.
	for _, ch := range []repopb.ArtifactChannel{
		repopb.ArtifactChannel_DEV,
		repopb.ArtifactChannel_CANDIDATE,
		repopb.ArtifactChannel_CANARY,
		repopb.ArtifactChannel_BOOTSTRAP,
	} {
		final, reject := directPublishChannelGate(ch, "local@ryzen", false)
		if reject || final != ch {
			t.Fatalf("channel %v must pass through unchanged; got (final=%v, reject=%v)", ch, final, reject)
		}
	}
}

// The downgraded result must be a legal identity lane: non-official + DEV passes
// validateLocalIdentityRules (Rule 2 only forbids official + DEV). This ties the
// gate's output to the lane invariant it must not violate.
func TestDirectPublishGate_DowngradedResultIsLaneLegal(t *testing.T) {
	final, reject := directPublishChannelGate(repopb.ArtifactChannel_STABLE, "local@ryzen", false)
	if reject {
		t.Fatal("non-official must downgrade, not reject")
	}
	if err := validateLocalIdentityRules("local@ryzen", final, "1.2.43"); err != nil {
		t.Fatalf("downgraded (non-official, DEV) must satisfy identity-lane rules; got %v", err)
	}
}
