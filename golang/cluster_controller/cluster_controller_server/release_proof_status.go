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
// Cases:
//
//   - installed == nil → ProofStatus=unknown, finding=rollout.proof_missing.
//   - desiredHash set but installed.Checksum disagrees →
//     ProofStatus=mismatch, finding=rollout.installed_hash_mismatch.
//   - desiredBuildID set but installed.BuildId disagrees →
//     ProofStatus=mismatch, finding=rollout.installed_build_id_mismatch.
//   - desiredVersion set but installed.Version disagrees →
//     ProofStatus=mismatch, finding=rollout.installed_version_mismatch.
//   - All hashes/build/version agree AND (runtime not needed OR runtime active) →
//     ProofStatus=installed_verified.
//   - All hashes/build/version agree but runtime is required and not active →
//     ProofStatus=mismatch, finding=rollout.partial_not_converged (the
//     binary is on disk, but the process isn't running it).
//   - Any of desiredHash/BuildID/Version is empty and installed values are
//     non-empty → ProofStatus=inventory_claim (we have a claim but lack
//     the desired identity to verify it against).
func decideNodeRolloutProof(
	desiredVersion, desiredHash, desiredBuildID string,
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
	gotHash := normalizeDesiredHash(installed.GetChecksum())
	wantHash := normalizeDesiredHash(desiredHash)
	gotBuild := strings.TrimSpace(installed.GetBuildId())
	wantBuild := strings.TrimSpace(desiredBuildID)

	// Hash mismatch is the strongest drift signal — Phase 1 promises the
	// post-install hash matched the artifact, so any later disagreement
	// implies tampering, partial extraction, or a stale read.
	if wantHash != "" && gotHash != "" && gotHash != wantHash {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofMismatch,
			FindingID:   FindingRolloutInstalledHashMismatch,
			Reason:      fmt.Sprintf("installed checksum %s != desired %s", gotHash, wantHash),
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

	// All present identities agree. To claim installed_verified we need at
	// minimum the artifact hash to have been matched at apply time
	// (recorded as installed.Checksum). Without that, we only have an
	// inventory claim.
	hashMatched := wantHash != "" && gotHash != "" && gotHash == wantHash
	buildMatched := wantBuild != "" && gotBuild != "" && gotBuild == wantBuild

	if !hashMatched && !buildMatched {
		// No checksum or build_id evidence — node-agent just told us a
		// version is installed. That's a claim, not proof.
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofInventoryClaim,
			FindingID:   "",
			Reason:      "version reported by node-agent; no hash/build_id evidence",
		}
	}

	if runtimeNeeded && !runtimeOK {
		return NodeRolloutProofVerdict{
			ProofStatus: RolloutProofMismatch,
			FindingID:   FindingRolloutPartialNotConverged,
			Reason:      "installed artifact on disk, but runtime unit not active",
		}
	}

	return NodeRolloutProofVerdict{
		ProofStatus: RolloutProofInstalledVerified,
		FindingID:   "",
		Reason:      "installed hash and build_id verified; runtime unit active",
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
