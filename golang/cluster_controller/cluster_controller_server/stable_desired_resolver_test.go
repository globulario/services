package main

// stable_desired_resolver_test.go — Slice 3: the controller resolves desired
// state from STABLE-channel artifacts only; DEV artifacts are never valid
// convergence targets (docs/design/package-lifecycle.md §3.4; repository.proto
// Invariant E; invariant package.release_vs_dev_channel_boundary).

import (
	"testing"

	"github.com/globulario/services/golang/repository/repositorypb"
)

func mf(name, version string, build int64, id string, ch repositorypb.ArtifactChannel) *repositorypb.ArtifactManifest {
	return &repositorypb.ArtifactManifest{
		Ref:         &repositorypb.ArtifactRef{Name: name, Version: version},
		BuildNumber: build,
		BuildId:     id,
		Channel:     ch,
	}
}

func TestDevChannelArtifactNotResolvableAsDesired(t *testing.T) {
	canon := canonicalServiceName("echo")
	const ver = "1.2.235"

	// 1. Channel-eligibility predicate.
	if !isConvergeableChannel(repositorypb.ArtifactChannel_STABLE) {
		t.Error("STABLE must be convergence-eligible")
	}
	if !isConvergeableChannel(repositorypb.ArtifactChannel_CHANNEL_UNSET) {
		t.Error("CHANNEL_UNSET (legacy) must be convergence-eligible")
	}
	if isConvergeableChannel(repositorypb.ArtifactChannel_DEV) {
		t.Error("DEV must NOT be convergence-eligible")
	}

	// 2. A DEV build with a HIGHER build_number must never advance desired-state:
	//    the STABLE build is selected, the higher DEV build is ignored.
	arts := []*repositorypb.ArtifactManifest{
		mf("echo", ver, 5, "bid-stable", repositorypb.ArtifactChannel_STABLE),
		mf("echo", ver, 9, "bid-dev", repositorypb.ArtifactChannel_DEV),
	}
	if b, id := pickBestConvergeableBuild(arts, canon, ver); b != 5 || id != "bid-stable" {
		t.Fatalf("expected STABLE build 5/bid-stable, got %d/%s (DEV build must be ignored)", b, id)
	}

	// 3. With ONLY a DEV artifact, nothing converges.
	if b, _ := pickBestConvergeableBuild([]*repositorypb.ArtifactManifest{
		mf("echo", ver, 9, "bid-dev", repositorypb.ArtifactChannel_DEV),
	}, canon, ver); b != 0 {
		t.Fatalf("a DEV-only artifact must not be selectable as desired; got build %d", b)
	}

	// 4. Legacy UNSET artifacts remain selectable (backward compat).
	if b, id := pickBestConvergeableBuild([]*repositorypb.ArtifactManifest{
		mf("echo", ver, 3, "bid-legacy", repositorypb.ArtifactChannel_CHANNEL_UNSET),
	}, canon, ver); b != 3 || id != "bid-legacy" {
		t.Fatalf("legacy UNSET artifact must remain selectable; got %d/%s", b, id)
	}
}
