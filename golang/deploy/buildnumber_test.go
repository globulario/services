package deploy

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestSelectLatestStableVersion(t *testing.T) {
	manifests := []*repopb.ArtifactManifest{
		{Ref: &repopb.ArtifactRef{Version: "1.2.259"}, Channel: repopb.ArtifactChannel_STABLE},
		{Ref: &repopb.ArtifactRef{Version: "1.2.260"}, Channel: repopb.ArtifactChannel_CANDIDATE},
		{Ref: &repopb.ArtifactRef{Version: "1.2.258"}, Channel: repopb.ArtifactChannel_CHANNEL_UNSET},
	}
	got, err := selectLatestStableVersion(manifests)
	if err != nil {
		t.Fatalf("selectLatestStableVersion returned error: %v", err)
	}
	if got != "1.2.259" {
		t.Fatalf("latest stable version = %q, want 1.2.259", got)
	}
}

func TestStableVersionExists(t *testing.T) {
	manifests := []*repopb.ArtifactManifest{
		{Ref: &repopb.ArtifactRef{Version: "1.2.259"}, Channel: repopb.ArtifactChannel_STABLE},
		{Ref: &repopb.ArtifactRef{Version: "1.2.260"}, Channel: repopb.ArtifactChannel_CANDIDATE},
	}
	if !stableVersionExists(manifests, "1.2.259") {
		t.Fatal("expected 1.2.259 to be recognized as a published stable version")
	}
	if stableVersionExists(manifests, "1.2.260") {
		t.Fatal("candidate-only 1.2.260 must not count as a published stable base version")
	}
}
