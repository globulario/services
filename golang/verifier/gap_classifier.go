package verifier

import (
	"fmt"
	"strings"
	"time"
)

// RuntimeIdentityGapClass categorizes WHY a runtime_identity_unproven finding
// was emitted for a given (node, service) pair.
type RuntimeIdentityGapClass string

const (
	// GapClassPendingSweep — the verifier swept recently but the verdict hasn't
	// arrived yet (or the verdict exists but proof is not yet captured). This is
	// the expected transient state on restart or first install.
	GapClassPendingSweep RuntimeIdentityGapClass = "pending_sweep"

	// GapClassMissingVerdict — no verdict has ever been written for this
	// (node, service) pair and sweep age is unknown.
	GapClassMissingVerdict RuntimeIdentityGapClass = "missing_verdict"

	// GapClassStaleVerdict — a verdict exists but it is older than SweepCadence
	// and the proof is still unknown/inventory_claim.
	GapClassStaleVerdict RuntimeIdentityGapClass = "stale_verdict"

	// GapClassVerifierStuck — the verifier hasn't swept in more than 3×SweepCadence,
	// indicating cluster-doctor may be unhealthy.
	GapClassVerifierStuck RuntimeIdentityGapClass = "verifier_stuck"

	// GapClassChecksumMismatch — the verifier wrote a mismatch verdict, meaning
	// the runtime binary differs from the repository record. Requires operator
	// investigation; must NOT be auto-cleared.
	GapClassChecksumMismatch RuntimeIdentityGapClass = "checksum_mismatch"

	// GapClassUnknown — cannot determine gap class from available evidence.
	GapClassUnknown RuntimeIdentityGapClass = "unknown"
)

// SweepCadence is the expected maximum interval between verifier sweeps.
// The doctor collector runs every ~60–90 s; we allow 90 s as the nominal cadence.
const SweepCadence = 90 * time.Second

// EtcdSweepRequestPrefix is the etcd key prefix under which targeted sweep
// requests are written by the controller and consumed by the doctor collector.
// Key shape: /globular/verification/requests/<nodeID>/<service>
const EtcdSweepRequestPrefix = "/globular/verification/requests/"

// EtcdKeyForSweepRequest returns the etcd key for a targeted sweep request
// for the given (nodeID, service) pair. Controller writes here; doctor reads
// and clears at the start of each sweep.
func EtcdKeyForSweepRequest(nodeID, service string) string {
	return EtcdSweepRequestPrefix + strings.TrimSpace(nodeID) + "/" + strings.TrimSpace(service)
}

// SweepRequest is the payload written at EtcdKeyForSweepRequest.
// The doctor collector unmarshals this to log context and route the request.
type SweepRequest struct {
	NodeID      string `json:"node_id"`
	Service     string `json:"service"`
	Reason      string `json:"reason"`
	RequestedBy string `json:"requested_by"`
	RequestedAt string `json:"requested_at"` // RFC3339
}

// GapClassification is the result of classifying a runtime identity gap.
type GapClassification struct {
	// Class is the primary classification of why the gap exists.
	Class RuntimeIdentityGapClass
	// AgeSeconds is the age of the existing verdict in seconds (0 if no verdict).
	AgeSeconds float64
	// SweepAgeSeconds is the age of the last verifier sweep in seconds (0 if unknown).
	SweepAgeSeconds float64
	// Details is a human-readable explanation suitable for operator logs.
	Details string
	// SweepRequested records whether a targeted sweep request has already been
	// sent for this (node, service) pair.
	SweepRequested bool
}

// ClassifyGap classifies WHY a runtime_identity_unproven finding was emitted.
// Pure function — no I/O.
//
// Parameters:
//
//	verdictAge          — age of the existing verdict (0 means no verdict written or no prior sweep).
//	sweepAge            — time since the last verifier sweep (0 means unknown).
//	hasVerdict          — true if any verdict key exists in etcd for this (node, service).
//	verdictProofStatus  — the ProofStatus field from the existing verdict (empty string if none).
//	sweepRequested      — true if a targeted sweep request is already pending.
//
// Classification priority (highest to lowest):
//  1. verdictProofStatus == ProofMismatch → checksum_mismatch (never auto-cleared)
//  2. sweepAge > 3×SweepCadence          → verifier_stuck
//  3. sweepAge <= SweepCadence && !hasVerdict → pending_sweep (sweep just ran, verdict en route)
//  4. hasVerdict && (ProofUnknown || ProofInventoryClaim) && verdictAge > SweepCadence → stale_verdict
//  5. hasVerdict && (ProofUnknown || ProofInventoryClaim) && verdictAge <= SweepCadence → pending_sweep
//  6. !hasVerdict (and sweepAge unknown)  → missing_verdict
//  7. else                                → unknown
func ClassifyGap(verdictAge, sweepAge time.Duration, hasVerdict bool, verdictProofStatus string, sweepRequested bool) GapClassification {
	g := GapClassification{
		AgeSeconds:      verdictAge.Seconds(),
		SweepAgeSeconds: sweepAge.Seconds(),
		SweepRequested:  sweepRequested,
	}

	// Priority 1: checksum mismatch — operator must investigate.
	if verdictProofStatus == ProofMismatch {
		g.Class = GapClassChecksumMismatch
		g.Details = "verifier wrote a mismatch verdict — runtime binary differs from repository record; operator investigation required"
		return g
	}

	// Priority 2: verifier is stuck (no sweep in 3× cadence).
	if sweepAge > 3*SweepCadence {
		g.Class = GapClassVerifierStuck
		g.Details = fmt.Sprintf("verifier last swept %.0fs ago (expected < %.0fs); cluster-doctor may be unhealthy",
			sweepAge.Seconds(), SweepCadence.Seconds())
		return g
	}

	// Priority 3: sweep ran recently, verdict not yet written.
	if sweepAge > 0 && sweepAge <= SweepCadence && !hasVerdict {
		g.Class = GapClassPendingSweep
		g.Details = fmt.Sprintf("verifier swept %.0fs ago; verdict not yet written", sweepAge.Seconds())
		return g
	}

	// Priority 4+5: verdict exists but proof is unknown/inventory_claim.
	if hasVerdict && (verdictProofStatus == ProofUnknown || verdictProofStatus == ProofInventoryClaim) {
		if verdictAge > SweepCadence {
			g.Class = GapClassStaleVerdict
			g.Details = fmt.Sprintf("verdict age %.0fs > sweep cadence %.0fs; awaiting next sweep",
				verdictAge.Seconds(), SweepCadence.Seconds())
			return g
		}
		g.Class = GapClassPendingSweep
		g.Details = "verdict present but proof not yet captured; waiting for next sweep"
		return g
	}

	// Priority 6: no verdict, sweep age unknown.
	if !hasVerdict {
		g.Class = GapClassMissingVerdict
		g.Details = "no verdict written for this (node, service) pair"
		return g
	}

	g.Class = GapClassUnknown
	g.Details = "cannot determine gap class from available evidence"
	return g
}
