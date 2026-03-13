package main

// workflow_invariants_test.go — invariant tests for PR7-PR11:
// build identity, deterministic resolution, publish state filtering,
// promotion-as-success-condition, and UploadBundle dual-write.

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── PR7: Build number identity consistency ──────────────────────────────────

func TestArtifactKeyWithBuild_5FieldKey(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "gateway",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
	}
	got := artifactKeyWithBuild(ref, 3)
	want := "core@globular.io%gateway%1.0.0%linux_amd64%3"
	if got != want {
		t.Errorf("artifactKeyWithBuild = %q, want %q", got, want)
	}
}

func TestArtifactKeyWithBuild_ZeroBuildNumber(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "gateway",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
	}
	got := artifactKeyWithBuild(ref, 0)
	want := "core@globular.io%gateway%1.0.0%linux_amd64%0"
	if got != want {
		t.Errorf("artifactKeyWithBuild(build=0) = %q, want %q", got, want)
	}
}

func TestArtifactKeyLegacy_4FieldKey(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "gateway",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
	}
	got := artifactKeyLegacy(ref)
	want := "core@globular.io%gateway%1.0.0%linux_amd64"
	if got != want {
		t.Errorf("artifactKeyLegacy = %q, want %q", got, want)
	}
}

func TestArtifactKeyWithBuild_DifferentBuildsDifferentKeys(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "gateway",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
	}
	k1 := artifactKeyWithBuild(ref, 1)
	k2 := artifactKeyWithBuild(ref, 2)
	if k1 == k2 {
		t.Error("different build numbers must produce different keys")
	}
}

// ── PR7/PR8: Publish state marshal/unmarshal round-trip ─────────────────────

func TestMarshalUnmarshalManifestWithState_RoundTrip(t *testing.T) {
	manifest := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "core@globular.io",
			Name:        "gateway",
			Version:     "1.0.0",
			Platform:    "linux_amd64",
		},
		Checksum:  "abc123",
		SizeBytes: 1024,
	}

	for _, state := range []repopb.PublishState{
		repopb.PublishState_STAGING,
		repopb.PublishState_VERIFIED,
		repopb.PublishState_PUBLISHED,
		repopb.PublishState_FAILED,
		repopb.PublishState_ORPHANED,
	} {
		t.Run(state.String(), func(t *testing.T) {
			data, err := marshalManifestWithState(manifest, state)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			got, gotState, err := unmarshalManifestWithState(data)
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if gotState != state {
				t.Errorf("state = %v, want %v", gotState, state)
			}
			if got.GetRef().GetName() != "gateway" {
				t.Errorf("name = %q, want gateway", got.GetRef().GetName())
			}
		})
	}
}

func TestMarshalManifestWithState_UnspecifiedOmitsField(t *testing.T) {
	manifest := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{Name: "test"},
	}
	data, err := marshalManifestWithState(manifest, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED)
	if err != nil {
		t.Fatal(err)
	}
	// Unspecified should not inject publishState.
	_, state, err := unmarshalManifestWithState(data)
	if err != nil {
		t.Fatal(err)
	}
	if state != repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
		t.Errorf("state = %v, want UNSPECIFIED", state)
	}
}

// ── PR8: Publish state filtering in promote transitions ─────────────────────

func TestValidPromoteTransition_PublishedFilter(t *testing.T) {
	// Only VERIFIED can be promoted to PUBLISHED.
	tests := []struct {
		from repopb.PublishState
		to   repopb.PublishState
		ok   bool
	}{
		{repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED, true},
		{repopb.PublishState_STAGING, repopb.PublishState_PUBLISHED, false},
		{repopb.PublishState_ORPHANED, repopb.PublishState_PUBLISHED, false},
		{repopb.PublishState_FAILED, repopb.PublishState_PUBLISHED, false},
		// PUBLISHED → PUBLISHED is idempotent.
		{repopb.PublishState_PUBLISHED, repopb.PublishState_PUBLISHED, true},
		// Any state → FAILED is allowed.
		{repopb.PublishState_VERIFIED, repopb.PublishState_FAILED, true},
		{repopb.PublishState_PUBLISHED, repopb.PublishState_FAILED, true},
		{repopb.PublishState_STAGING, repopb.PublishState_FAILED, true},
	}
	for _, tt := range tests {
		name := tt.from.String() + "->" + tt.to.String()
		t.Run(name, func(t *testing.T) {
			got := repopb.ValidPromoteTransition(tt.from, tt.to)
			if got != tt.ok {
				t.Errorf("ValidPromoteTransition(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.ok)
			}
		})
	}
}

// ── PR11: UploadBundle dual-write key format ────────────────────────────────

func TestUploadBundleDualWrite_UsesArtifactKey(t *testing.T) {
	// The dual-write from UploadBundle should use the same 5-field key format
	// as UploadArtifact, with build_number=0 for legacy bundles.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "gateway",
		Version:     "2.0.0",
		Platform:    "linux_amd64",
	}
	key := artifactKeyWithBuild(ref, 0)
	binKey := binaryStorageKey(key)
	manKey := manifestStorageKey(key)

	if binKey != "artifacts/core@globular.io%gateway%2.0.0%linux_amd64%0.bin" {
		t.Errorf("binary key = %q", binKey)
	}
	if manKey != "artifacts/core@globular.io%gateway%2.0.0%linux_amd64%0.manifest.json" {
		t.Errorf("manifest key = %q", manKey)
	}
}
