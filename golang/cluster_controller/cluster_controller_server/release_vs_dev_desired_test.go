package main

import (
	"testing"

	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

// b1 of package.release_vs_dev_channel_boundary: the desired-state write authority
// (validateArtifactInRepo, the only caller-path into ServiceDesiredVersion) must
// reject a DEV-channel artifact so no caller — operator CLI, agent/MCP, or deploy —
// can make a dev build a convergence target. Release tiers stay eligible.

func TestChannelEligibleForDesiredState_DevRejected(t *testing.T) {
	if channelEligibleForDesiredState(repositorypb.ArtifactChannel_DEV) {
		t.Fatal("DEV-channel artifact must NOT be eligible for cluster desired-state")
	}
}

func TestChannelEligibleForDesiredState_ReleaseTiersAccepted(t *testing.T) {
	for _, ch := range []repositorypb.ArtifactChannel{
		repositorypb.ArtifactChannel_CHANNEL_UNSET, // treated as STABLE
		repositorypb.ArtifactChannel_STABLE,
		repositorypb.ArtifactChannel_CANDIDATE,
		repositorypb.ArtifactChannel_CANARY,
		repositorypb.ArtifactChannel_BOOTSTRAP,
	} {
		if !channelEligibleForDesiredState(ch) {
			t.Fatalf("release-tier channel %v must be eligible for desired-state (must not overblock)", ch)
		}
	}
}
