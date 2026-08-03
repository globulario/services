package main

import (
	"encoding/json"
	"strings"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// F1 proof bound to the REAL libnss-resolve artifact.
//
// libnss-resolve is the first shipped package with no executable of its own:
// it is a passive NSS plugin library, so it declares `entrypoint: none` and
// carries no entrypoint checksum. It is therefore the concrete end-to-end case
// for the F1 model — a package that installs successfully and can never have
// its executable identity proven, because there is no executable.
//
// The fixture below is the VERBATIM package.json emitted by the canonical
// packages build (globulario/packages fix/register-libnss-resolve) using the
// services CLI built from this branch:
//
//	archive libnss-resolve_255.4_linux_amd64.tgz
//	sha256  19fa0c3a5ef4540c4579c4ba0601ec85df694a178b5627a1cb6f46106aa06951
//
// Binding to the artifact rather than to helper functions is deliberate: a test
// that only exercised decideNodeRolloutProof with hand-written arguments would
// still pass if the package started emitting a fabricated entrypoint checksum.
const libnssResolveBuiltManifest = `{
  "type": "command",
  "name": "libnss-resolve",
  "version": "255.4",
  "platform": "linux_amd64",
  "publisher": "core@globular.io",
  "entrypoint": "none",
  "defaults": {
    "configDir": "",
    "spec": "specs/libnss_resolve_cmd.yaml",
    "scriptsDir": "scripts"
  },
  "description": "systemd-resolved NSS bridge for reliable *.globular.internal hostname resolution.",
  "keywords": [
    "dns",
    "nss",
    "systemd-resolved",
    "resolver"
  ],
  "license": "LGPL-2.1"
}`

// The built manifest declares `none` and carries NO entrypoint_checksum, so the
// release-index projection has nothing to source EntrypointChecksum from.
func TestLibnssResolve_BuiltManifestCarriesNoEntrypointChecksum(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal([]byte(libnssResolveBuiltManifest), &m); err != nil {
		t.Fatalf("parse built manifest: %v", err)
	}
	if got := m["entrypoint"]; got != "none" {
		t.Fatalf("entrypoint = %v, want the sentinel %q", got, "none")
	}
	if _, present := m["entrypoint_checksum"]; present {
		t.Fatalf("built manifest carries an entrypoint_checksum for a package with no "+
			"executable: %v", m["entrypoint_checksum"])
	}
	// `none` is a sentinel, not a path — nothing may treat it as a bin/ file.
	if strings.Contains(m["entrypoint"].(string), "/") {
		t.Errorf("entrypoint %q looks like a path", m["entrypoint"])
	}
}

// releaseIndexEntrypointChecksum models the projection: the index field is
// sourced from the manifest's entrypoint_checksum, which is absent here.
func releaseIndexEntrypointChecksum(t *testing.T, manifest string) string {
	t.Helper()
	var m struct {
		EntrypointChecksum string `json:"entrypoint_checksum"`
	}
	if err := json.Unmarshal([]byte(manifest), &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m.EntrypointChecksum
}

func TestLibnssResolve_ReleaseIndexEntrypointChecksumIsEmpty(t *testing.T) {
	if got := releaseIndexEntrypointChecksum(t, libnssResolveBuiltManifest); got != "" {
		t.Fatalf("release-index EntrypointChecksum = %q, want empty", got)
	}
}

// The full services proof path for this exact package: an empty manifest
// entrypoint checksum must flow through as an empty ExpectedSha256, leave
// InstalledHash empty, and settle at inventory_claim with no drift finding —
// while the installation itself is still recorded.
func TestLibnssResolve_ServicesProofPath(t *testing.T) {
	resolvedEntrypoint := releaseIndexEntrypointChecksum(t, libnssResolveBuiltManifest)

	// ExpectedSha256 is sourced from the manifest entrypoint checksum; with none
	// declared, the node-agent receives an empty value and skips ONLY the
	// binary-specific comparison (apply_package_release.go gates on != "").
	expectedSha256 := resolvedEntrypoint
	if expectedSha256 != "" {
		t.Fatalf("ApplyPackageRelease.ExpectedSha256 = %q, want empty — a synthesized "+
			"value here would verify bytes the manifest never declared", expectedSha256)
	}

	// What the node-agent reports back after a successful install: the package
	// is present at the right version, convergence inputs agree, and there is no
	// entrypoint measurement because there is no executable.
	installed := &node_agentpb.InstalledPackage{
		Version:  "255.4",
		Checksum: "9999888877776666555544443333222211110000ffffeeeeddddccccbbbbaaaa",
		BuildId:  "bid-libnss-1",
	}

	v := decideNodeRolloutProof(
		"255.4",
		installed.GetChecksum(), // convergence agrees
		installed.GetBuildId(),  // build id agrees
		resolvedEntrypoint,      // "" — no manifest entrypoint
		installed,
		false, // command package: skip_runtime_check, no managed unit
		false,
	)

	if v.ProofStatus != RolloutProofInventoryClaim {
		t.Fatalf("ProofStatus = %q, want %q (reason=%q)",
			v.ProofStatus, RolloutProofInventoryClaim, v.Reason)
	}
	if v.ProofStatus == RolloutProofInstalledVerified {
		t.Fatal("a package with no executable claimed verified executable identity")
	}
	if v.FindingID != "" {
		t.Errorf("no drift finding expected for an unprovable-by-construction package, got %q", v.FindingID)
	}
	if v.FindingID == FindingRolloutInstalledHashMismatch {
		t.Error("binary mismatch finding raised with no entrypoint on either side")
	}
	if !strings.Contains(v.Reason, "unproven") {
		t.Errorf("reason should state the identity is unproven: %q", v.Reason)
	}

	// The installation receipt itself is preserved: version and build id are
	// still reported, so "installed" remains observable even though "verified"
	// is not claimable.
	if installed.GetVersion() != "255.4" || installed.GetBuildId() == "" {
		t.Error("installation receipt was not preserved")
	}
}

// InstalledHash is a binary-identity field. With no entrypoint measurement from
// the node-agent it must stay empty — never backfilled from the convergence
// checksum or the build id.
func TestLibnssResolve_InstalledHashStaysEmpty(t *testing.T) {
	installed := &node_agentpb.InstalledPackage{
		Version:  "255.4",
		Checksum: "9999888877776666555544443333222211110000ffffeeeeddddccccbbbbaaaa",
		BuildId:  "bid-libnss-1",
	}
	// This mirrors the projection in release_pipeline.go: sourced solely from
	// the node-agent's entrypoint_checksum metadata, which libnss-resolve never
	// produces.
	var installedHash string
	if md := installed.GetMetadata(); md != nil {
		installedHash = strings.TrimSpace(md["entrypoint_checksum"])
	}
	if installedHash != "" {
		t.Fatalf("InstalledHash = %q for a package with no executable", installedHash)
	}
	if installedHash == installed.GetChecksum() {
		t.Error("convergence checksum leaked into the binary-identity field")
	}
	if installedHash == installed.GetBuildId() {
		t.Error("build id leaked into the binary-identity field")
	}
}

// Negative control: had the package kept its fabricated bin/ marker, the
// manifest would carry an entrypoint checksum and the same path would report
// installed_verified — proof against a 279-byte echo script that can pass while
// the real NSS library is absent or damaged.
func TestLibnssResolve_FabricatedMarkerWouldHaveClaimedVerified(t *testing.T) {
	const markerSha = "aaaa1111bbbb2222cccc3333dddd4444eeee5555ffff6666aaaa7777bbbb8888"
	installed := &node_agentpb.InstalledPackage{
		Version:  "255.4",
		Checksum: "9999888877776666555544443333222211110000ffffeeeeddddccccbbbbaaaa",
		BuildId:  "bid-libnss-1",
		Metadata: map[string]string{"entrypoint_checksum": markerSha},
	}
	v := decideNodeRolloutProof("255.4", installed.GetChecksum(), installed.GetBuildId(),
		markerSha, installed, false, false)
	if v.ProofStatus != RolloutProofInstalledVerified {
		t.Fatalf("control did not reproduce the defect: got %q", v.ProofStatus)
	}
	t.Log("confirmed: the removed marker would have yielded installed_verified " +
		"against a shim, with the real libnss_resolve.so.2 entirely unchecked")
}
