package main

import (
	"os"
	"strings"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// F1 — binary identity proof requires a manifest entrypoint.
//
// Governing intent: service.installation_is_not_runtime_truth. Installation
// succeeding and a SPECIFIC EXECUTABLE's identity being verified are two
// different claims, and the second one requires evidence that names an
// executable: the entrypoint checksum (a direct measurement of the binary on
// disk) or the build id (which names the exact build that produced it).
//
// The convergence hash is neither. It is sha256 over the controller-rendered
// convergence inputs (publisher + name + version + build_number + config), so
// agreement proves the node-agent applied what the controller asked for — not
// which bytes landed on disk.

// pkgWith builds an installed-package record.
func pkgWith(version, convergenceHash, buildID, entrypointChecksum string) *node_agentpb.InstalledPackage {
	p := &node_agentpb.InstalledPackage{
		Version:  version,
		Checksum: convergenceHash,
		BuildId:  buildID,
	}
	if entrypointChecksum != "" {
		p.Metadata = map[string]string{"entrypoint_checksum": entrypointChecksum}
	}
	return p
}

const (
	binSHA  = "aaaa1111bbbb2222cccc3333dddd4444eeee5555ffff6666aaaa7777bbbb8888"
	convSHA = "9999888877776666555544443333222211110000ffffeeeeddddccccbbbbaaaa"
	tarSHA  = "1111222233334444555566667777888899990000aaaabbbbccccddddeeeeffff"
)

// 1. Entrypoint plus matching checksum produces strict binary proof.
func TestF1_EntrypointMatch_YieldsInstalledVerified(t *testing.T) {
	v := decideNodeRolloutProof("1.2.0", convSHA, "bid-1", binSHA,
		pkgWith("1.2.0", convSHA, "bid-1", binSHA), true, true)
	if v.ProofStatus != RolloutProofInstalledVerified {
		t.Fatalf("ProofStatus=%q want=%q (reason=%q)", v.ProofStatus, RolloutProofInstalledVerified, v.Reason)
	}
	if v.FindingID != "" {
		t.Errorf("verified proof must carry no finding, got %q", v.FindingID)
	}
}

// 2. Entrypoint plus MISMATCHED checksum fails.
func TestF1_EntrypointMismatch_Fails(t *testing.T) {
	v := decideNodeRolloutProof("1.2.0", convSHA, "bid-1", binSHA,
		pkgWith("1.2.0", convSHA, "bid-1", tarSHA), true, true)
	if v.ProofStatus != RolloutProofMismatch {
		t.Fatalf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofMismatch)
	}
	if v.FindingID != FindingRolloutInstalledHashMismatch {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingRolloutInstalledHashMismatch)
	}
}

// 3./6. No entrypoint on either side cannot claim verified identity, even when
// everything else agrees and the unit is healthy. This is the core F1 case.
func TestF1_NoEntrypoint_CannotBecomeInstalledVerified(t *testing.T) {
	v := decideNodeRolloutProof("1.2.0", convSHA, "", "",
		pkgWith("1.2.0", convSHA, "", ""), true, true)
	if v.ProofStatus == RolloutProofInstalledVerified {
		t.Fatalf("a package with NO manifest entrypoint reached %q purely because its "+
			"install completed and convergence inputs agreed (reason=%q)",
			v.ProofStatus, v.Reason)
	}
	if v.ProofStatus != RolloutProofInventoryClaim {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, RolloutProofInventoryClaim)
	}
}

// 7. …but installation still completes with an honest non-binary proof state,
// and raises no drift finding. Absence of binary proof is not a failure.
func TestF1_NoEntrypoint_CompletesWithHonestStateAndNoFinding(t *testing.T) {
	v := decideNodeRolloutProof("1.2.0", convSHA, "", "",
		pkgWith("1.2.0", convSHA, "", ""), true, true)
	if v.FindingID != "" {
		t.Errorf("missing binary proof is not drift; got finding %q", v.FindingID)
	}
	if !strings.Contains(v.Reason, "unproven") {
		t.Errorf("reason must say the executable identity is unproven, got %q", v.Reason)
	}
}

// 8. Desired hash is never substituted for the entrypoint checksum: a package
// whose convergence hash agrees but whose desired entrypoint is absent must not
// borrow the convergence value as binary evidence.
func TestF1_DesiredHashNeverSubstitutesForEntrypoint(t *testing.T) {
	// The installed record even carries an entrypoint measurement, but the
	// manifest declared none — so there is nothing authoritative to compare to.
	v := decideNodeRolloutProof("1.2.0", convSHA, "", "",
		pkgWith("1.2.0", convSHA, "", binSHA), true, true)
	if v.ProofStatus == RolloutProofInstalledVerified {
		t.Fatalf("an observed runtime binary was retroactively treated as manifest "+
			"authority (reason=%q)", v.Reason)
	}
	// Substituting the convergence hash into the entrypoint slot does NOT
	// surface as a false verification — it surfaces as a FALSE MISMATCH, because
	// the convergence value is then compared against the installed binary
	// measurement across incompatible schemas. Assert that too, or the aliasing
	// defect passes this test unnoticed.
	if v.FindingID == FindingRolloutInstalledHashMismatch {
		t.Fatalf("desired convergence hash was aliased into the entrypoint slot and "+
			"compared against the installed binary measurement — incompatible "+
			"schemas produce a permanent false mismatch (reason=%q)", v.Reason)
	}
	if v.ProofStatus == RolloutProofMismatch {
		t.Fatalf("no manifest entrypoint must not yield a mismatch verdict: %q", v.Reason)
	}
}

// 9. Archive/tarball digest is never substituted for the entrypoint checksum.
// Comparing the tarball digest to the installed binary measurement is the
// v1.2.56/57/58 false-positive class.
func TestF1_ArchiveDigestNeverComparedToBinary(t *testing.T) {
	v := decideNodeRolloutProof("1.2.0", convSHA, "", tarSHA,
		pkgWith("1.2.0", convSHA, "", binSHA), true, true)
	if v.ProofStatus == RolloutProofMismatch && v.FindingID == FindingRolloutInstalledHashMismatch {
		// A mismatch here is legitimate ONLY because both sides are declared
		// entrypoint values. Guard the schema confusion explicitly instead.
		if strings.Contains(v.Reason, tarSHA) && strings.Contains(v.Reason, binSHA) {
			return // entrypoint-vs-entrypoint comparison, correct schema
		}
		t.Fatalf("mismatch compared incompatible schemas: %q", v.Reason)
	}
}

// 10. Build id is executable-identifying, but it is not a binary checksum: it
// must never populate the binary-identity field.
func TestF1_BuildIDIsNotABinaryChecksum(t *testing.T) {
	v := decideNodeRolloutProof("1.2.0", "", "bid-1", "",
		pkgWith("1.2.0", "", "bid-1", ""), true, true)
	if v.ProofStatus != RolloutProofInstalledVerified {
		t.Fatalf("build_id agreement is accepted evidence for legacy records; got %q", v.ProofStatus)
	}
	if strings.Contains(v.Reason, "entrypoint_checksum verified") {
		t.Errorf("build_id agreement must not be reported as binary verification: %q", v.Reason)
	}
}

// 11. A no-canonical-entrypoint package stays distinguishable from a verified
// executable: same version, same convergence, different proof status.
func TestF1_NoEntrypointRemainsDistinguishableFromVerified(t *testing.T) {
	verified := decideNodeRolloutProof("1.2.0", convSHA, "bid-1", binSHA,
		pkgWith("1.2.0", convSHA, "bid-1", binSHA), true, true)
	noEntry := decideNodeRolloutProof("1.2.0", convSHA, "", "",
		pkgWith("1.2.0", convSHA, "", ""), true, true)
	if verified.ProofStatus == noEntry.ProofStatus {
		t.Fatalf("verified executable and no-entrypoint package are indistinguishable: both %q",
			verified.ProofStatus)
	}
}

// 12./13. No mismatch is emitted merely because BOTH sides lack a comparable
// entrypoint identity — absence of evidence is not evidence of drift.
func TestF1_NoBinaryMismatchWhenNeitherSideHasEntrypoint(t *testing.T) {
	for _, tc := range []struct {
		name             string
		wantEP, gotEP    string
		wantConv, gotCnv string
	}{
		{"neither side has entrypoint", "", "", convSHA, convSHA},
		{"desired only, installed silent", binSHA, "", convSHA, convSHA},
		{"installed only, desired silent", "", binSHA, convSHA, convSHA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := decideNodeRolloutProof("1.2.0", tc.wantConv, "", tc.wantEP,
				pkgWith("1.2.0", tc.gotCnv, "", tc.gotEP), true, true)
			if v.FindingID == FindingRolloutInstalledHashMismatch {
				t.Errorf("false binary mismatch with no comparable entrypoint pair: %q", v.Reason)
			}
		})
	}
}

// 15. Existing valid manifest-entrypoint packages retain their behavior — the
// repair must not demote a genuinely verified install.
func TestF1_VerifiedEntrypointPackagesUnaffected(t *testing.T) {
	for _, runtimeOK := range []bool{true} {
		v := decideNodeRolloutProof("1.2.0", convSHA, "bid-1", binSHA,
			pkgWith("1.2.0", convSHA, "bid-1", binSHA), true, runtimeOK)
		if v.ProofStatus != RolloutProofInstalledVerified {
			t.Fatalf("regression: verified entrypoint install demoted to %q", v.ProofStatus)
		}
	}
	// Runtime down still reports partial-not-converged, unchanged.
	v := decideNodeRolloutProof("1.2.0", convSHA, "bid-1", binSHA,
		pkgWith("1.2.0", convSHA, "bid-1", binSHA), true, false)
	if v.FindingID != FindingRolloutPartialNotConverged {
		t.Errorf("runtime-down finding changed: %q", v.FindingID)
	}
}

// 17. The verdict records which proof schema was actually used, so an operator
// can tell binary proof from a weaker signal without reading the code.
func TestF1_ReasonRecordsWhichProofSchemaWasUsed(t *testing.T) {
	cases := []struct {
		name         string
		v            NodeRolloutProofVerdict
		wantInReason string
	}{
		{"binary", decideNodeRolloutProof("1.2.0", convSHA, "bid-1", binSHA,
			pkgWith("1.2.0", convSHA, "bid-1", binSHA), true, true), "verified"},
		{"build_id only", decideNodeRolloutProof("1.2.0", "", "bid-1", "",
			pkgWith("1.2.0", "", "bid-1", ""), true, true), "build_id"},
		{"convergence only", decideNodeRolloutProof("1.2.0", convSHA, "", "",
			pkgWith("1.2.0", convSHA, "", ""), true, true), "unproven"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(tc.v.Reason, tc.wantInReason) {
				t.Errorf("reason %q does not identify the proof schema (want %q)",
					tc.v.Reason, tc.wantInReason)
			}
		})
	}
}

// 5./16. The projection must not publish a non-binary hash as binary identity.
// InstalledHash is declared "artifact sha256 verified at apply time"; the
// convergence checksum belongs to a different schema entirely.
func TestF1_ProjectionDoesNotPublishConvergenceHashAsBinaryIdentity(t *testing.T) {
	src := releasePipelineSource(t)
	i := strings.Index(src, "nCopy.InstalledHash =")
	if i < 0 {
		t.Fatal("no InstalledHash assignment in the rollout projection")
	}
	stmt := src[i:]
	if j := strings.Index(stmt, "\n"); j > 0 {
		stmt = stmt[:j]
	}
	if strings.Contains(stmt, "GetChecksum()") {
		t.Errorf("projection writes the CONVERGENCE checksum into the binary-identity "+
			"field InstalledHash: %s", strings.TrimSpace(stmt))
	}
	if !strings.Contains(stmt, "entrypoint_checksum") {
		t.Errorf("InstalledHash should be sourced from the node-agent's entrypoint "+
			"measurement, got: %s", strings.TrimSpace(stmt))
	}
}

// releasePipelineSource reads release_pipeline.go with // comments stripped, so
// the structural assertions above cannot be satisfied by prose.
func releasePipelineSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("release_pipeline.go")
	if err != nil {
		t.Fatalf("read release_pipeline.go: %v", err)
	}
	var b strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
