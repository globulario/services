// Package verifier is the Phase 9 integration of the Diagnostic Honesty
// Refactor. It is the single decision surface that pulls every earlier
// phase's evidence together into one verdict per (service, node) target:
//
//   - Phase 1 post-install binary hash gate     → installed_path + installed_sha256
//   - Phase 2 GetServiceRuntimeProof RPC        → running_exe_*, systemd_*, process_*
//   - Phase 3 health version verdict            → claim-vs-proof for version checks
//   - Phase 4 rollout proof status              → release-level floor (consumed elsewhere)
//   - Phase 5b systemd effective drift detector → unit_drift verdict
//   - Phase 6 silent fallback registry          → live fallbacks per node
//   - Phase 7 cross-node drift detector         → cluster-wide file consistency
//
// The brief is explicit that the verifier need not be a large new service:
// "doctor collector or node_agent RPC aggregator." This package is the
// orchestrator; callers (cluster_doctor today, a dedicated daemon later)
// supply the deps that read desired state from etcd, fetch artifact
// manifests from the repository, and dispatch RPCs. The verifier itself
// does no I/O.
//
// The required trust model from the brief:
//
//	controller pushes desired
//	node_agent attempts apply
//	independent verifier checks reality   ← THIS PACKAGE
//	doctor consumes verifier proof
//	controller converges only on verifier proof
//
// Output layout: Verdicts are per (service, node) and carry both a
// ProofStatus level (verified / installed_verified / inventory_claim /
// mismatch / unknown) and a list of finding IDs the doctor pipeline
// raises. A summary Result rolls verdicts up across the sweep.
//
// Etcd persistence layout (suggested by the brief; the actual write is
// owned by the caller via EtcdKeyForVerification):
//
//	/globular/verification/runtime/<node_id>/<service_name>
package verifier

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/crossnodedrift"
	"github.com/globulario/services/golang/deploy"
	"github.com/globulario/services/golang/fallback"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ─────────────────────────────────────────────────────────────────────
// Constants — finding ids and proof-status levels. Pinned here, mirrored
// in docs/awareness/failure_modes.yaml.
// ─────────────────────────────────────────────────────────────────────

// Proof status levels — weakest to strongest. Match the rollout proof
// values in cluster_controller's release_proof_status.go so consumers
// that already key off those strings keep working unchanged.
const (
	ProofUnknown           = ""
	ProofInventoryClaim    = "inventory_claim"
	ProofInstalledVerified = "installed_verified"
	ProofRuntimeVerified   = "runtime_verified"
	ProofMismatch          = "mismatch"
)

// Finding ids the verifier emits. Each maps to a failure_modes.yaml
// entry that the doctor consumes.
const (
	// Phase 1 / Phase 2 drift findings.
	FindingInstalledBinaryHashMismatch = "package.installed_binary_hash_mismatch"
	FindingRunningBinaryHashMismatch   = "service.running_binary_hash_mismatch"
	FindingRunningVersionMismatch      = "service.running_version_mismatch"
	FindingOldPidAfterUpgrade          = "service.old_pid_after_upgrade"
	FindingBootstrapOrderingSkew       = "service.bootstrap_ordering_skew"
	FindingRuntimeIdentityUnproven     = "service.runtime_identity_unproven"

	// Phase 4 release-level finding (raised when the verifier sees the
	// release at AVAILABLE but the per-node floor isn't installed_verified).
	FindingRolloutPartialNotConverged = "rollout.partial_not_converged"

	// Phase 5b systemd drift.
	FindingSystemdEffectiveConfigDrift = "systemd.effective_config_drift"

	// Phase 6 silent fallback.
	FindingSilentFallbackActive = "service.silent_fallback_active"

	// Phase 7 cross-node drift.
	FindingCrossNodeFileDrift = "cluster.cross_node_file_drift"
	FindingAuthorityUndefined = "cluster.authority_undefined"
)

// Severity levels for findings. Mirrors the failure_modes.yaml severities.
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityDegraded = "degraded"
	SeverityInfo     = "info"
)

// ─────────────────────────────────────────────────────────────────────
// Inputs
// ─────────────────────────────────────────────────────────────────────

// Target describes one (service, node) pair to verify. Populated from
// desired state in etcd + the controller's view of which nodes the
// service is supposed to run on.
//
// HASH SCHEMA — non-negotiable, see docs/awareness/failure_modes.yaml
// entry verifier.hash_schema_confusion (and the fix brief at
// /home/dave/Downloads/claude_fix_verifier_hash_schema_false_positive.md).
//
//   DesiredEntrypointChecksum
//     sha256 of the installed service binary (e.g. /usr/lib/globular/bin/file_server).
//     Source: package manifest's `entrypoint_checksum`. This is what the
//     verifier compares against ServiceRuntimeProof.InstalledSha256 and
//     ServiceRuntimeProof.RunningExeSha256 — both of which are binary hashes.
//
//   DesiredPackageDigest
//     sha256 of the package tarball (e.g. file_1.2.56_linux_amd64.tgz).
//     Source: package manifest's `package_digest` / release status
//     `ResolvedArtifactDigest`. Kept separately so future checks can
//     compare against a tarball digest (e.g. artifact integrity audits)
//     without re-confusing the binary comparison surface.
//
// Comparing the binary on disk against the tarball digest is the exact
// false-positive bug v1.2.56 shipped. Don't.
type Target struct {
	Service                   string
	NodeID                    string
	DesiredVersion            string
	DesiredBuildID            string
	DesiredEntrypointChecksum string // binary sha256 — compares to InstalledSha256 / RunningExeSha256
	DesiredPackageDigest      string // package tarball sha256 — reserved for future tarball checks
	// RuntimeNeeded controls whether the verifier expects the package
	// to be running as a systemd unit. COMMAND-kind packages have no
	// service; leave false to skip runtime checks for those.
	RuntimeNeeded bool
	// ApplyTime is when the controller last commanded an apply for
	// this target. Used to detect old_pid_after_upgrade — a process
	// whose start time predates the apply is running stale bytes.
	// Zero disables the check.
	ApplyTime time.Time
	// IsFirstInstall is true when this target has never converged
	// before (the apply that wrote ApplyTime is the first one for this
	// service on this node). On a fresh install, install.sh starts
	// services before the controller bootstrap records the apply, so
	// a process whose start time predates ApplyTime is expected
	// sequencing — not stale bytes. The verifier downgrades the
	// finding to service.bootstrap_ordering_skew (degraded) instead
	// of service.old_pid_after_upgrade (critical).
	//
	// On upgrade (IsFirstInstall=false), an older-than-apply process
	// is still treated as critical because the restart didn't take.
	IsFirstInstall bool
}

// Evidence is the bag the verifier reconciles for one target. The
// caller populates it from node-agent RPCs and the controller's
// rendered state. Empty fields are treated as "unknown" — the verifier
// degrades to runtime_identity_unproven rather than asserting drift
// against a hole.
type Evidence struct {
	// Proof from GetServiceRuntimeProof (Phase 2). Nil disables every
	// process / systemd check.
	Proof *node_agentpb.ServiceRuntimeProof
	// Rendered systemd unit content the controller produced for this
	// service. Empty disables the Phase 5b effective-config drift
	// check.
	RenderedUnit string
}

// ─────────────────────────────────────────────────────────────────────
// Outputs
// ─────────────────────────────────────────────────────────────────────

// Finding is one assertion the verifier raises. Severity tracks the
// failure_modes.yaml entry; Evidence is structured so doctor / event
// consumers can render the operator-visible "why" without re-parsing
// the verdict.
type Finding struct {
	ID       string
	Severity string
	Service  string
	NodeID   string
	Detected time.Time
	// Evidence is the structured payload from the brief's schema for
	// service.silent_fallback_active (service, dependency, mode,
	// primary_error, affected_paths, node_id, since). The verifier
	// fills the keys that are relevant per finding; consumers should
	// not assume any specific key is always present.
	Evidence map[string]string
}

// Verdict is the per-target output. ProofStatus is the strongest
// claim the verifier is willing to make for this (service, node).
// Findings carries the IDs of every emitted finding so doctor can map
// them to failure_modes.yaml entries.
type Verdict struct {
	Target      Target
	ProofStatus string
	Findings    []Finding
	// Reason summarizes the verdict in one line for operator UIs.
	Reason string
}

// Result is the cluster-wide roll-up across all per-target verdicts +
// any cross-cutting findings (cross-node drift, active fallbacks).
type Result struct {
	Verdicts        []Verdict
	DriftVerdicts   []crossnodedrift.DriftVerdict
	Fallbacks       []fallback.Active
	CrossFindings   []Finding
	GeneratedAt     time.Time
	Summary         Summary
}

// Summary counts roll-ups for monitoring/alerting consumers.
type Summary struct {
	TotalTargets    int
	Verified        int
	InstalledOnly   int
	InventoryOnly   int
	Mismatched      int
	Unknown         int
	FallbacksActive int
	DriftedClasses  int
}

// ─────────────────────────────────────────────────────────────────────
// VerifyTarget — pure per-(service, node) reconciliation.
// ─────────────────────────────────────────────────────────────────────

// VerifyTarget reconciles a single target's claims against its
// evidence and returns a verdict. Pure function — no I/O. The
// orchestrator (Verify) calls this once per target after gathering
// evidence from RPCs.
//
// Decision order (strongest evidence first):
//
//  1. Proof missing entirely → ProofUnknown + runtime_identity_unproven.
//  2. Running exe sha256 vs installed sha256 → if differ, mismatch +
//     service.running_binary_hash_mismatch (the "new binary on disk,
//     old PID running" case).
//  3. Installed sha256 vs desired hash → if differ, mismatch +
//     package.installed_binary_hash_mismatch (the "apply said success
//     but on-disk bytes drifted" case).
//  4. Runtime version vs desired version → mismatch + running_version_mismatch.
//  5. Apply time vs process start time → mismatch + old_pid_after_upgrade.
//  6. Systemd effective vs rendered unit (Phase 5b) → drift =>
//     systemd.effective_config_drift.
//  7. Errors from collection → degrade to ProofUnknown + runtime_identity_unproven.
//  8. Everything agrees → ProofRuntimeVerified.
//
// The verdict's ProofStatus is the floor across each check. Findings
// is the union; doctor decides which to surface.
func VerifyTarget(target Target, ev Evidence, now time.Time) Verdict {
	v := Verdict{Target: target}

	if ev.Proof == nil {
		v.ProofStatus = ProofUnknown
		v.Findings = append(v.Findings, Finding{
			ID:       FindingRuntimeIdentityUnproven,
			Severity: SeverityDegraded,
			Service:  target.Service,
			NodeID:   target.NodeID,
			Detected: now,
			Evidence: map[string]string{
				"reason": "no ServiceRuntimeProof captured for this target",
			},
		})
		v.Reason = "no runtime proof available"
		return v
	}

	proof := ev.Proof
	// HASH SCHEMA — binary hashes only here. Tarball-digest comparisons
	// belong to a future audit check that consumes DesiredPackageDigest;
	// they are NOT mixed into the binary comparison surface.
	desiredEntrypoint := normalizeHash(target.DesiredEntrypointChecksum)
	installedEntrypoint := normalizeHash(proof.GetInstalledSha256())
	runningEntrypoint := normalizeHash(proof.GetRunningExeSha256())
	desiredVersion := strings.TrimSpace(target.DesiredVersion)
	runtimeVersion := strings.TrimSpace(proof.GetRuntimeVersion())

	// 1. installed-vs-desired (binary): Phase 1's post-install gate
	//    guarantees apply matched the entrypoint checksum; any
	//    disagreement now means tampering or out-of-band replacement.
	if desiredEntrypoint != "" && installedEntrypoint != "" && installedEntrypoint != desiredEntrypoint {
		v.Findings = append(v.Findings, Finding{
			ID:       FindingInstalledBinaryHashMismatch,
			Severity: SeverityCritical,
			Service:  target.Service,
			NodeID:   target.NodeID,
			Detected: now,
			Evidence: map[string]string{
				"installed_sha256": installedEntrypoint,
				"desired_sha256":   desiredEntrypoint,
				"installed_path":   proof.GetInstalledPath(),
				"hash_kind":        "entrypoint_checksum",
			},
		})
	}

	// 2. running-vs-installed (binary): Phase 2's signature failure —
	//    new binary on disk, old PID still serving.
	if installedEntrypoint != "" && runningEntrypoint != "" && installedEntrypoint != runningEntrypoint {
		v.Findings = append(v.Findings, Finding{
			ID:       FindingRunningBinaryHashMismatch,
			Severity: SeverityCritical,
			Service:  target.Service,
			NodeID:   target.NodeID,
			Detected: now,
			Evidence: map[string]string{
				"installed_sha256": installedEntrypoint,
				"running_sha256":   runningEntrypoint,
				"running_pid":      fmt.Sprintf("%d", proof.GetRunningPid()),
				"running_exe":      proof.GetRunningExePath(),
				"hash_kind":        "entrypoint_checksum",
			},
		})
	}

	// 3. runtime version vs desired: when the live process exposes a
	//    /version endpoint we compare; empty runtime_version is the
	//    "unproven" case and surfaces below.
	if runtimeVersion != "" && desiredVersion != "" && runtimeVersion != desiredVersion {
		v.Findings = append(v.Findings, Finding{
			ID:       FindingRunningVersionMismatch,
			Severity: SeverityCritical,
			Service:  target.Service,
			NodeID:   target.NodeID,
			Detected: now,
			Evidence: map[string]string{
				"running_version": runtimeVersion,
				"desired_version": desiredVersion,
			},
		})
	}

	// 4. apply-time vs process-start: a PID that predates the last
	//    apply is running stale bytes regardless of what its hash says
	//    — except on first install, where install.sh starts services
	//    before the controller bootstrap records the apply. We classify
	//    those two cases separately so operators see the right finding:
	//
	//      first install + process older than apply  → bootstrap_ordering_skew (degraded)
	//      upgrade       + process older than apply  → old_pid_after_upgrade   (critical)
	//
	//    Binary-hash checks above remain the authoritative drift signal
	//    for both cases; this finding is the timing tell, not the hash tell.
	if !target.ApplyTime.IsZero() && proof.GetProcessStartTime() != nil {
		started := proof.GetProcessStartTime().AsTime()
		if !started.IsZero() && started.Before(target.ApplyTime) {
			findingID := FindingOldPidAfterUpgrade
			severity := SeverityCritical
			if target.IsFirstInstall {
				findingID = FindingBootstrapOrderingSkew
				severity = SeverityDegraded
			}
			v.Findings = append(v.Findings, Finding{
				ID:       findingID,
				Severity: severity,
				Service:  target.Service,
				NodeID:   target.NodeID,
				Detected: now,
				Evidence: map[string]string{
					"running_pid":        fmt.Sprintf("%d", proof.GetRunningPid()),
					"process_start_time": started.Format(time.RFC3339),
					"last_apply_time":    target.ApplyTime.Format(time.RFC3339),
					"is_first_install":   fmt.Sprintf("%v", target.IsFirstInstall),
				},
			})
		}
	}

	// 5. systemd effective-vs-rendered drift (Phase 5b). Only check
	//    when both halves are available; absence is unknown, not drift.
	if ev.RenderedUnit != "" {
		eff := deploy.EffectiveUnitProperties{
			Type:           proof.GetEffectiveType(),
			ExecStart:      proof.GetEffectiveExecStart(),
			FragmentPath:   proof.GetSystemdUnitPath(),
			ActiveState:    proof.GetSystemdActiveState(),
			SubState:       proof.GetSystemdSubState(),
			UnitFileSHA256: proof.GetSystemdUnitSha256(),
		}
		duv := deploy.DetectEffectiveUnitDrift(ev.RenderedUnit, eff)
		if duv.Status == deploy.UnitDriftDrift {
			v.Findings = append(v.Findings, Finding{
				ID:       FindingSystemdEffectiveConfigDrift,
				Severity: SeverityHigh,
				Service:  target.Service,
				NodeID:   target.NodeID,
				Detected: now,
				Evidence: map[string]string{
					"drifts": strings.Join(duv.Drifts, "; "),
				},
			})
		}
	}

	// 6. Partial proof: collection errors don't raise drift on their
	//    own, but they do prevent claiming runtime_verified.
	if len(proof.GetErrors()) > 0 && len(v.Findings) == 0 {
		v.Findings = append(v.Findings, Finding{
			ID:       FindingRuntimeIdentityUnproven,
			Severity: SeverityDegraded,
			Service:  target.Service,
			NodeID:   target.NodeID,
			Detected: now,
			Evidence: map[string]string{
				"errors": strings.Join(proof.GetErrors(), "; "),
			},
		})
	}

	// 7. Final proof-status floor. Any critical/high finding caps
	//    the verdict at mismatch; degraded-only caps at unknown; no
	//    findings AND no errors AND running hash matches gives the
	//    coveted runtime_verified.
	v.ProofStatus = computeProofStatus(target, proof, v.Findings)
	v.Reason = buildReason(v.ProofStatus, v.Findings)
	return v
}

func computeProofStatus(target Target, proof *node_agentpb.ServiceRuntimeProof, findings []Finding) string {
	hasCritical := false
	hasDegraded := false
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical, SeverityHigh:
			hasCritical = true
		case SeverityDegraded:
			hasDegraded = true
		}
	}
	if hasCritical {
		return ProofMismatch
	}
	desiredEntrypoint := normalizeHash(target.DesiredEntrypointChecksum)
	installedEntrypoint := normalizeHash(proof.GetInstalledSha256())
	runningEntrypoint := normalizeHash(proof.GetRunningExeSha256())

	// runtime_verified requires: installed entrypoint matches desired AND
	// running entrypoint matches installed AND (runtime version probe
	// agrees OR was empty) AND runtime is needed and active OR not needed.
	installedOK := desiredEntrypoint != "" && installedEntrypoint == desiredEntrypoint
	runtimeOK := !target.RuntimeNeeded ||
		(runningEntrypoint != "" && runningEntrypoint == installedEntrypoint &&
			strings.EqualFold(strings.TrimSpace(proof.GetSystemdActiveState()), "active"))
	versionOK := target.DesiredVersion == "" ||
		proof.GetRuntimeVersion() == "" ||
		strings.TrimSpace(proof.GetRuntimeVersion()) == target.DesiredVersion
	if installedOK && runtimeOK && versionOK && !hasDegraded {
		return ProofRuntimeVerified
	}
	if installedOK && !hasDegraded {
		return ProofInstalledVerified
	}
	if hasDegraded {
		return ProofUnknown
	}
	return ProofInventoryClaim
}

func buildReason(status string, findings []Finding) string {
	if status == ProofRuntimeVerified {
		return "all proofs agree"
	}
	if len(findings) == 0 {
		return status
	}
	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		ids = append(ids, f.ID)
	}
	sort.Strings(ids)
	return status + ": " + strings.Join(ids, ", ")
}

// ─────────────────────────────────────────────────────────────────────
// AggregateResult — cluster-wide roll-up.
// ─────────────────────────────────────────────────────────────────────

// AggregateResult assembles per-target verdicts plus cross-cutting
// inputs (active fallbacks scraped from each node's registry, cross-node
// drift verdicts from the Phase 7 detector) into a single Result for
// doctor or operator-UI consumption.
//
// Fallbacks and DriftVerdicts are passed through verbatim; the
// orchestration value is the Summary roll-up and the CrossFindings
// list, which converts both into Finding shape so consumers don't
// need three parsing paths.
func AggregateResult(
	verdicts []Verdict,
	fallbacks []fallback.Active,
	driftVerdicts []crossnodedrift.DriftVerdict,
	now time.Time,
) Result {
	r := Result{
		Verdicts:      verdicts,
		Fallbacks:     fallbacks,
		DriftVerdicts: driftVerdicts,
		GeneratedAt:   now,
	}

	// Convert Phase 6 fallbacks into Finding shape.
	for _, fb := range fallbacks {
		r.CrossFindings = append(r.CrossFindings, Finding{
			ID:       FindingSilentFallbackActive,
			Severity: SeverityDegraded,
			Service:  fb.Service,
			NodeID:   fb.NodeID,
			Detected: now,
			Evidence: map[string]string{
				"dependency":     fb.Dependency,
				"mode":           fb.Mode,
				"primary_error":  fb.PrimaryError,
				"affected_paths": strings.Join(fb.AffectedPaths, ","),
				"since":          fb.Since.Format(time.RFC3339),
			},
		})
	}

	// Convert Phase 7 drift verdicts into Finding shape.
	for _, dv := range driftVerdicts {
		if dv.Status == crossnodedrift.DriftStatusConsistent ||
			dv.Status == crossnodedrift.DriftStatusUnknown {
			continue
		}
		r.CrossFindings = append(r.CrossFindings, Finding{
			ID:       dv.FindingID,
			Severity: SeverityDegraded,
			Service:  dv.PathClass,
			Detected: now,
			Evidence: map[string]string{
				"path":   dv.Path,
				"drifts": strings.Join(dv.Drifts, "; "),
			},
		})
	}

	// Roll-up summary.
	r.Summary.TotalTargets = len(verdicts)
	for _, v := range verdicts {
		switch v.ProofStatus {
		case ProofRuntimeVerified:
			r.Summary.Verified++
		case ProofInstalledVerified:
			r.Summary.InstalledOnly++
		case ProofInventoryClaim:
			r.Summary.InventoryOnly++
		case ProofMismatch:
			r.Summary.Mismatched++
		default:
			r.Summary.Unknown++
		}
	}
	r.Summary.FallbacksActive = len(fallbacks)
	driftedClasses := map[string]struct{}{}
	for _, dv := range driftVerdicts {
		if dv.Status == crossnodedrift.DriftStatusDrift ||
			dv.Status == crossnodedrift.DriftStatusAuthorityUndefined {
			driftedClasses[dv.PathClass] = struct{}{}
		}
	}
	r.Summary.DriftedClasses = len(driftedClasses)

	return r
}

// EtcdKeyForVerification returns the canonical etcd key under which a
// per-(node, service) verification result lives. The brief defines
// this path:
//
//	/globular/verification/runtime/<node_id>/<service_name>
//
// Callers serialize the Verdict (or a custom JSON payload) and write
// it at this key. Persistence itself is intentionally outside the
// verifier package — different consumers (doctor collector, future
// daemon) have different transaction requirements.
func EtcdKeyForVerification(nodeID, service string) string {
	return "/globular/verification/runtime/" +
		strings.TrimSpace(nodeID) + "/" + strings.TrimSpace(service)
}

func normalizeHash(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	return strings.TrimPrefix(h, "sha256:")
}
