package main

// release_proof_status.go — Phase 4 (Diagnostic Honesty Refactor).
//
// The release state machine (PENDING/RESOLVED/AVAILABLE/...) tracks where a
// rollout is in its lifecycle. It does NOT track HOW confident the controller
// is that the rollout actually reached reality. Today, AVAILABLE can be
// written from a node-agent's "installed" claim alone — exactly the
// structural lie the diagnostic-honesty refactor exists to remove.
//
// This file adds a second dimension — ProofStatus — that any release
// transition can carry. It mirrors the claim-vs-proof split from Phase 3
// (handlers_health.go::decideVersionVerdict). The strict CONVERGED model
// from the brief (PENDING / APPLYING / INSTALLED_CLAIMED / INSTALLED_VERIFIED /
// RUNTIME_VERIFIED / PARTIAL / CONVERGED) is encoded here as a verification
// level rather than as new Phase values, so the existing pipeline keeps
// running while operators gain visibility into the proof gap.
//
// Phase 9 (verifier) will plug GetServiceRuntimeProof into the level-bumping
// path and let strict mode refuse AVAILABLE when proof is short.

import (
	"fmt"
	"strings"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Rollout proof levels — ordered weakest to strongest.
//
//	"inventory_claim"     — node-agent reported the package installed; no
//	                        independent verification of disk or process.
//	"installed_verified"  — installed binary checksum on disk matches the
//	                        expected artifact hash AND the package's runtime
//	                        unit (when required) is active. This is the
//	                        ceiling the controller can reach today without
//	                        an external verifier.
//	"runtime_verified"    — Phase 9: GetServiceRuntimeProof confirmed the
//	                        /proc/<pid>/exe hash, runtime version, and
//	                        systemd effective config also match.
//	"mismatch"            — one or more proof levels disagreed; the
//	                        finding_id names the specific drift.
//	"unknown"             — proof could not be gathered (heartbeat stale,
//	                        node unreachable, runtime check not applicable).
//
// The constants live in this Go-only file rather than the proto so that
// the proto-level NodeReleaseStatus stays additive (a `proof_status` string
// field) and doesn't require a wire-protocol enum migration.
const (
	RolloutProofUnknown           = ""
	RolloutProofInventoryClaim    = "inventory_claim"
	RolloutProofInstalledVerified = "installed_verified"
	RolloutProofRuntimeVerified   = "runtime_verified"
	RolloutProofMismatch          = "mismatch"
)

// rolloutProofRank returns the ordering rank for a proof level. Higher is
// stronger. "mismatch" and "unknown" both sort below inventory_claim — we
// never want to silently round them up.
func rolloutProofRank(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case RolloutProofRuntimeVerified:
		return 3
	case RolloutProofInstalledVerified:
		return 2
	case RolloutProofInventoryClaim:
		return 1
	case RolloutProofMismatch:
		return -1
	default:
		return 0 // unknown
	}
}

// rolloutProofMin returns the weaker of two proof levels.
func rolloutProofMin(a, b string) string {
	if rolloutProofRank(a) <= rolloutProofRank(b) {
		return a
	}
	return b
}

// Findings emitted alongside the proof status. Keep these in sync with
// docs/awareness/failure_modes.yaml.
const (
	// FindingRolloutPartialNotConverged surfaces when a release reaches
	// AVAILABLE but at least one required node is below installed_verified.
	// Operators see "AVAILABLE per workflow report, but proof says PARTIAL."
	FindingRolloutPartialNotConverged = "rollout.partial_not_converged"
	// FindingRolloutInstalledHashMismatch surfaces when the node-agent
	// reports a hash that disagrees with the resolved artifact digest.
	FindingRolloutInstalledHashMismatch = "rollout.installed_hash_mismatch"
	// FindingRolloutInstalledBuildIdMismatch surfaces when the installed
	// build_id disagrees with the resolved build_id (an apply targeted the
	// right version but a stale artifact slipped in).
	FindingRolloutInstalledBuildIdMismatch = "rollout.installed_build_id_mismatch"
	// FindingRolloutInstalledVersionMismatch surfaces when the installed
	// version string disagrees with the resolved version.
	FindingRolloutInstalledVersionMismatch = "rollout.installed_version_mismatch"
	// FindingRolloutProofMissing surfaces when the controller has no
	// independent measurement at all (e.g. node never reported back).
	FindingRolloutProofMissing = "rollout.proof_missing"
)

// NodeRolloutProofVerdict is the per-node decision returned by
// decideNodeRolloutProof. It carries the proof level + a finding id when
// applicable, so consumers can render the claim-vs-proof gap without
// having to re-derive it from raw fields.
type NodeRolloutProofVerdict struct {
	// ProofStatus is one of the RolloutProof* constants.
	ProofStatus string
	// FindingID names the specific drift when ProofStatus is "mismatch" or
	// when proof is shorter than the consumer requires. Empty when the
	// verdict is benign (matched or simply weaker than required, no drift).
	FindingID string
	// Reason is a human-readable line for logs and operator UIs. It is
	// intentionally NOT the same string as PackageConvergence.Reason so we
	// can surface the proof dimension without losing the existing message.
	Reason string
}

// decideNodeRolloutProof maps a (desired vs installed) snapshot into a
// rollout proof verdict. The runtime-needed and runtime-ok flags come from
// classifyPackageConvergence — we re-use its measurements rather than
// re-deriving them here.
//
// HASH SCHEMA — non-negotiable (see docs/awareness/failure_modes.yaml entry
// verifier.hash_schema_confusion and the v1.2.59 brief at
// /home/dave/Downloads/claude_fix_rollout_hash_schema_bootstrap_skew.md).
//
//   desiredEntrypoint    sha256 of the on-disk service binary, sourced from
//                        the artifact manifest (ServiceRelease.Status
//                        .ResolvedEntrypointChecksum). The node-agent records
//                        its measurement in installed.Metadata["entrypoint_checksum"];
//                        comparing the two is the v1.2.59 binary-integrity
//                        proof signal.
//   desiredConvergence   sha256 of the controller-rendered convergence inputs
//                        (publisher + name + version + build_number + config).
//                        Stamped by the node-agent into installed.Checksum
//                        post-install so the controller's convergence
//                        comparison terminates — apples-to-apples. NOT a
//                        binary integrity signal.
//
// What we MUST NOT do (the v1.2.56/57/58 false-positive class of bug):
// compare installed.Checksum (convergence) against ResolvedArtifactDigest
// (tarball). Different hash schemas — guaranteed mismatch.
//
// Decision order:
//
//   - installed == nil → ProofStatus=unknown, finding=rollout.proof_missing.
//   - desiredEntrypoint set, installed entrypoint_checksum present and
//     disagrees → ProofStatus=mismatch, finding=rollout.installed_hash_mismatch.
//   - desiredBuildID set but installed.BuildId disagrees → mismatch +
//     rollout.installed_build_id_mismatch.
//   - desiredVersion set but installed.Version disagrees → mismatch +
//     rollout.installed_version_mismatch.
//   - desiredConvergence and installed.Checksum both present and disagree →
//     mismatch + rollout.installed_hash_mismatch (convergence-level drift:
//     the node-agent applied something, but its rendered convergence hash
//     doesn't match the controller's. Either input drift or stale stamp).
//   - Identities agree AND (runtime not needed OR runtime active) →
//     ProofStatus=installed_verified.
//   - Identities agree but runtime required and unit not active →
//     mismatch + rollout.partial_not_converged.
//   - Insufficient evidence (no entrypoint checksum AND no convergence hash
//     AND no build_id) → ProofStatus=inventory_claim.
func decideNodeRolloutProof(
	desiredVersion, desiredConvergence, desiredBuildID, desiredEntrypoint string,
	installed *node_agentpb.InstalledPackage,
	runtimeNeeded, runtimeOK bool,
) NodeRolloutProofVerdict {
	if installed == nil {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofUnknown,
			FindingID:   FindingRolloutProofMissing,
			Reason:      "no installed-package record from node-agent",
		}
	}

	gotVersion := strings.TrimSpace(installed.GetVersion())
	wantVersion := strings.TrimSpace(desiredVersion)
	gotConvergence := normalizeDesiredHash(installed.GetChecksum())
	wantConvergence := normalizeDesiredHash(desiredConvergence)
	gotBuild := strings.TrimSpace(installed.GetBuildId())
	wantBuild := strings.TrimSpace(desiredBuildID)
	gotEntrypoint := ""
	if md := installed.GetMetadata(); md != nil {
		gotEntrypoint = normalizeDesiredHash(md["entrypoint_checksum"])
	}
	wantEntrypoint := normalizeDesiredHash(desiredEntrypoint)

	// 1. Binary integrity (preferred drift signal). entrypoint_checksum is
	//    the sha256 of the binary on disk; the node-agent stamps it at apply
	//    time and the artifact manifest carries the expected value.
	if wantEntrypoint != "" && gotEntrypoint != "" && gotEntrypoint != wantEntrypoint {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofMismatch,
			FindingID:   FindingRolloutInstalledHashMismatch,
			Reason: fmt.Sprintf("installed entrypoint_checksum %s != desired %s",
				gotEntrypoint, wantEntrypoint),
		}
	}

	if wantBuild != "" && gotBuild != "" && gotBuild != wantBuild {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofMismatch,
			FindingID:   FindingRolloutInstalledBuildIdMismatch,
			Reason:      fmt.Sprintf("installed build_id %s != desired %s", gotBuild, wantBuild),
		}
	}
	if wantVersion != "" && gotVersion != "" && gotVersion != wantVersion {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofMismatch,
			FindingID:   FindingRolloutInstalledVersionMismatch,
			Reason:      fmt.Sprintf("installed version %s != desired %s", gotVersion, wantVersion),
		}
	}

	// 2. Convergence drift: controller-stamped hash should match installed.
	//    This is apples-to-apples (convergence vs convergence). Disagreement
	//    means the node-agent didn't get the expected rendering — distinct
	//    from tampering, but still drift.
	//
	// GUARD: only run this leg when Checksum can actually BE a convergence
	// hash. Per the proto it is "SHA256 of installed archive", and the
	// node-agent — the single writing actor for the Installed layer — now
	// writes the binary SHA there deliberately, saying so at the write site.
	// When it does, Checksum equals the record's own entrypoint_checksum, and
	// comparing it to desired_hash compares two different hash schemas, which
	// can only ever yield drift.
	//
	// The binary leg above does NOT short-circuit on success — it returns only
	// on mismatch — so a package whose binary identity matched perfectly fell
	// straight through to here and was reported as drift. Measured 2026-08-18
	// on the 5-node simulation: resource@1.2.312 with installed 70dee39b vs
	// desired 7b14db46, where 70dee39b was byte-identical to its own
	// entrypoint_checksum. The node went DEGRADED, the release went DEFERRED,
	// and DEFERRED re-defers on every 2-minute retry — permanently stuck.
	//
	// Skipping the leg in exactly that case keeps the legacy detector alive
	// for the shape it was built for (a writer that really did put the
	// release-identity hash in Checksum — pinned by the pre-fix fixture in
	// TestDecideNodeRolloutProof_ClusterControllerStaleHashRegression, where
	// entrypoint_checksum is absent) while removing the false positive.
	checksumIsBinaryHash := gotEntrypoint != "" && gotConvergence == gotEntrypoint
	if wantConvergence != "" && gotConvergence != "" && gotConvergence != wantConvergence &&
		!checksumIsBinaryHash {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofMismatch,
			FindingID:   FindingRolloutInstalledHashMismatch,
			Reason: fmt.Sprintf("installed convergence_hash %s != desired %s",
				gotConvergence, wantConvergence),
		}
	}

	// 3. Strength of agreement. installed_verified requires the binary
	//    identity to have been proven; convergence-hash agreement alone is
	//    a claim, not proof. build_id agreement is acceptable as a fallback
	//    when binary proof isn't carried (legacy / pre-entrypoint records).
	entrypointMatched := wantEntrypoint != "" && gotEntrypoint != "" && gotEntrypoint == wantEntrypoint
	buildMatched := wantBuild != "" && gotBuild != "" && gotBuild == wantBuild
	// A Checksum that is really the binary hash is not convergence evidence.
	convergenceMatched := wantConvergence != "" && gotConvergence != "" &&
		gotConvergence == wantConvergence && !checksumIsBinaryHash

	// Runtime posture is evaluated BEFORE proof strength: a unit that is down
	// is a real, reportable condition regardless of how strongly the installed
	// artifact's identity is evidenced. Ordering this after the evidence gate
	// would silently drop rollout.partial_not_converged for every package whose
	// binary identity is unproven — exactly the packages already carrying the
	// least visibility.
	if runtimeNeeded && !runtimeOK {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofMismatch,
			FindingID:   FindingRolloutPartialNotConverged,
			Reason:      "installed artifact on disk, but runtime unit not active",
		}
	}

	// installed_verified is a claim about a SPECIFIC EXECUTABLE's identity, and
	// the ONLY evidence that measures the executable on disk is the entrypoint
	// checksum. Everything else is metadata ABOUT the install:
	//
	//   build_id     names the build that was supposed to produce the binary,
	//                but never measures the bytes that actually landed;
	//   convergence  proves the node-agent applied the inputs the controller
	//                rendered — installation succeeded, nothing more.
	//
	// Both are useful inventory/convergence evidence and neither is binary
	// proof, so neither can carry a verdict documented as proof of the
	// installed binary checksum. Collapsing them is how
	//
	//   installation succeeded  ≠  specific executable identity verified
	//
	// gets lost (intent: service.installation_is_not_runtime_truth).
	//
	// This is also what puts this writer at parity with the sibling
	// MarkNodeSucceeded writers, which already demote to inventory_claim
	// whenever ResolvedEntrypointChecksum is absent — without regard to
	// build_id. Previously the two disagreed about the same release: one said
	// installed_verified on build_id agreement, the other said inventory_claim.
	if !entrypointMatched {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofInventoryClaim,
			FindingID:   "",
			Reason:      weakEvidenceReason(buildMatched, convergenceMatched),
		}
	}

	return NodeRolloutProofVerdict{
		ProofStatus: RolloutProofInstalledVerified,
		FindingID:   "",
		Reason:      "installed identity verified; runtime unit active",
	}
}

// weakEvidenceReason describes what agreed when no manifest entrypoint checksum
// was available to measure the executable. It names the evidence as metadata
// rather than checksum proof, so an operator reading a verdict can tell why the
// identity is unproven instead of assuming evidence was simply missing.
func weakEvidenceReason(buildMatched, convergenceMatched bool) string {
	const tail = "; no manifest entrypoint checksum was available, so the installed " +
		"executable's identity is unproven"
	switch {
	case buildMatched && convergenceMatched:
		return "build_id and convergence inputs agree (build metadata, not a binary measurement)" + tail
	case buildMatched:
		return "build_id agrees (build metadata, not a binary measurement)" + tail
	case convergenceMatched:
		return "convergence inputs agree (installation applied, not a binary measurement)" + tail
	default:
		return "version reported by node-agent; no entrypoint/build_id/convergence evidence" + tail
	}
}

// AggregateRolloutProof is the release-level roll-up across nodes. The
// floor across required nodes wins — a single mismatch or unknown drags
// the release down.
type AggregateRolloutProof struct {
	// ProofStatus is the floor across the per-node verdicts. Empty (unknown)
	// when no required nodes were inspected.
	ProofStatus string
	// Findings deduplicates the union of per-node FindingIDs that matter at
	// release scope. rollout.partial_not_converged is always emitted when
	// the floor is below installed_verified at AVAILABLE.
	Findings []string
}

// aggregateRolloutProof rolls up per-node verdicts into a release-level
// status. requiredNodes filters to the set of nodes that must converge for
// the release to count as AVAILABLE (so failed-but-not-required nodes
// don't drag the proof level down).
func aggregateRolloutProof(verdicts []NodeRolloutProofVerdict, atAvailable bool) AggregateRolloutProof {
	out := AggregateRolloutProof{}
	if len(verdicts) == 0 {
		return out
	}

	seen := make(map[string]bool, 4)
	addFinding := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out.Findings = append(out.Findings, id)
	}

	// Compute the floor.
	floor := RolloutProofRuntimeVerified
	for _, v := range verdicts {
		floor = rolloutProofMin(floor, v.ProofStatus)
		addFinding(v.FindingID)
	}
	out.ProofStatus = floor

	// A release that has transitioned to AVAILABLE but whose proof floor
	// is below installed_verified is, per the brief, partial-not-converged.
	// Surface that finding even if no per-node verdict produced it, so
	// operators see a single release-scope reason.
	if atAvailable && rolloutProofRank(floor) < rolloutProofRank(RolloutProofInstalledVerified) {
		addFinding(FindingRolloutPartialNotConverged)
	}

	return out
}
