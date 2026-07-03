package main

import (
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/repository/repositorypb"
)

func TestRuntimeChecksumFromManifest_PrefersEntrypoint(t *testing.T) {
	m := &repositorypb.ArtifactManifest{
		Checksum:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		EntrypointChecksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	got := runtimeChecksumFromManifest(m)
	want := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if got != want {
		t.Fatalf("runtimeChecksumFromManifest() = %q, want %q", got, want)
	}
}

func TestRuntimeChecksumFromManifest_FallsBackToArchive(t *testing.T) {
	m := &repositorypb.ArtifactManifest{
		Checksum: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	got := runtimeChecksumFromManifest(m)
	want := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if got != want {
		t.Fatalf("runtimeChecksumFromManifest() = %q, want %q", got, want)
	}
}

func TestHeartbeatChecksumForInstalledState_InfrastructurePreservesConvergenceHash(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Kind:     "INFRASTRUCTURE",
		Checksum: "infra:core@globular.io/envoy=1.35.3+b:0;",
	}
	m := &repositorypb.ArtifactManifest{
		Checksum:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		EntrypointChecksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	got := heartbeatChecksumForInstalledState("INFRASTRUCTURE", existing, m)
	want := "infra:core@globular.io/envoy=1.35.3+b:0;"
	if got != want {
		t.Fatalf("heartbeatChecksumForInstalledState() = %q, want preserved convergence hash %q", got, want)
	}
}

func TestHeartbeatChecksumForInstalledState_ServiceUsesRuntimeIdentity(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Kind:     "SERVICE",
		Checksum: "service-convergence-hash-that-must-not-win",
	}
	m := &repositorypb.ArtifactManifest{
		Checksum:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		EntrypointChecksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	got := heartbeatChecksumForInstalledState("SERVICE", existing, m)
	want := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if got != want {
		t.Fatalf("heartbeatChecksumForInstalledState() = %q, want runtime identity %q", got, want)
	}
}

// TestAssignEntrypointChecksumMetadata_ManifestIsCanonical pins
// identity.has_single_canonical_source_and_is_immutable and
// meta.identity_computation_must_be_invariant. When both the manifest claim
// and a disk observation are present, `entrypoint_checksum` MUST hold the
// manifest's value (canonical identity); the disk hash is recorded only as
// evidence in a distinct field so drift detection can compare the two
// without poisoning the identity.
func TestAssignEntrypointChecksumMetadata_ManifestIsCanonical(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{}
	manifest := "mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm"
	disk := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

	assignEntrypointChecksumMetadata(pkg, manifest, disk)

	if got := pkg.Metadata["entrypoint_checksum"]; got != manifest {
		t.Errorf("entrypoint_checksum = %q; want manifest value %q", got, manifest)
	}
	if got := pkg.Metadata["entrypoint_checksum_disk_observed"]; got != disk {
		t.Errorf("entrypoint_checksum_disk_observed = %q; want disk value %q", got, disk)
	}
	if _, present := pkg.Metadata["entrypoint_checksum_legacy_disk_only"]; present {
		t.Errorf("entrypoint_checksum_legacy_disk_only must NOT be set when manifest carries the identity")
	}
}

// TestAssignEntrypointChecksumMetadata_LegacyManifestUsesDistinctKey pins
// the legacy-artifact path: when the manifest has no entrypoint_checksum,
// the disk hash MUST land under entrypoint_checksum_legacy_disk_only, NEVER
// under entrypoint_checksum. Identity is owned by the manifest; a value the
// manifest does not carry must not silently masquerade as one.
func TestAssignEntrypointChecksumMetadata_LegacyManifestUsesDistinctKey(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{}
	disk := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

	assignEntrypointChecksumMetadata(pkg, "", disk)

	if _, present := pkg.Metadata["entrypoint_checksum"]; present {
		t.Errorf("entrypoint_checksum must NOT be set when manifest lacks the field; got %q",
			pkg.Metadata["entrypoint_checksum"])
	}
	if got := pkg.Metadata["entrypoint_checksum_legacy_disk_only"]; got != disk {
		t.Errorf("entrypoint_checksum_legacy_disk_only = %q; want %q", got, disk)
	}
	if got := pkg.Metadata["entrypoint_checksum_disk_observed"]; got != disk {
		t.Errorf("entrypoint_checksum_disk_observed = %q; want %q", got, disk)
	}
}

// TestAssignEntrypointChecksumMetadata_NoOpOnEmpty confirms the helper is
// a true no-op when neither identity nor evidence is available — never
// writes empty values, never allocates an empty map.
func TestAssignEntrypointChecksumMetadata_NoOpOnEmpty(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{}
	assignEntrypointChecksumMetadata(pkg, "", "")
	if len(pkg.Metadata) > 0 {
		t.Errorf("metadata mutated on empty inputs: %v", pkg.Metadata)
	}
}
