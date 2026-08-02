package observation

// remediation.go turns a completed remediation into behavioral evidence.
//
// This closes the gap satisfaction.go recorded deliberately: the workflow
// computed a remediation.Outcome and surfaced it in run output, but nothing
// constructed an Evidence row from it, so the governor had no verification
// evidence to find. A derived report may explain artifact truth; it may never
// substitute for it, and a status string in a workflow output map is a report.
//
// THE ELIGIBILITY RULE IS STRUCTURAL, NOT ADVISORY
//
// Two independent questions must both be answered yes before a remediation can
// be cited as verification evidence:
//
//	IsSuccess()        — did the repair actually work (dispatched, verified,
//	                     finding cleared)?
//	LineageComplete()  — can this outcome be attributed to a specific subject in
//	                     a specific cluster, after a specific dispatch?
//
// Either alone is insufficient. A successful repair with unattributable
// provenance is not evidence ABOUT anything; complete provenance around a failed
// repair is not evidence that anything worked.
//
// The distinction is carried by the evidence KIND rather than left to the
// qualifier. BindSatisfies keys on kind and source, so a non-qualifying outcome
// emitted under a different kind cannot acquire the verification requirement
// even if a future caller re-applies the mapper defensively. Emitting both under
// one kind and relying on the qualifier to reject would leave the ineligible row
// one relaxed rule away from counting.

import (
	"strings"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	"github.com/globulario/services/golang/remediation"
)

const (
	// KindRemediationVerification is the kind carried ONLY by a remediation that
	// both succeeded and is fully attributable. The satisfaction catalog keys
	// sat.remediation.fresh_convergence_verified on exactly this value.
	KindRemediationVerification = "remediation_verification_evidence"

	// KindRemediationDiagnostic is the kind carried by every other outcome:
	// recordable history, never verification. No catalog rule describes it, so
	// BindSatisfies leaves it unbound — it is visible to an operator reading
	// evidence and invisible to a governor asking whether convergence was
	// verified, which is the correct pair of answers.
	KindRemediationDiagnostic = "remediation_outcome_diagnostic"

	// SourceKindDoctorRemediationWorkflow identifies the emitting pipeline:
	// remediate.doctor.finding, resolve → execute → verify.
	SourceKindDoctorRemediationWorkflow = "doctor_remediation_workflow"

	// ResultFindingResolved means the post-action doctor sweep found the
	// original finding gone. It is deliberately NOT "dispatched",
	// "verification_started" or "succeeded": those describe an attempt or a
	// status, and a requirement about verification must not accept them.
	ResultFindingResolved = "finding_resolved"
)

// FromRemediationOutcome maps a completed remediation into a behavioral evidence
// bundle.
//
// Returns the bundle and whether the row qualifies as verification evidence —
// exactly equivalent to Kind == KindRemediationVerification, returned explicitly
// so a caller does not have to compare kinds to learn what it was handed.
//
// An outcome carrying no identity at all yields an empty Bundle: there is
// nothing to key a stable evidence id on, and a row identified by the hash of
// nothing would collide with every other such row.
func FromRemediationOutcome(project string, domain api.DomainRef, o remediation.Outcome) (Bundle, bool) {
	if strings.TrimSpace(o.FindingID) == "" && strings.TrimSpace(o.WorkflowRunID) == "" {
		return Bundle{}, false
	}

	qualifies := o.IsSuccess() && o.LineageComplete()

	kind, result := KindRemediationDiagnostic, diagnosticResult(o)
	if qualifies {
		kind, result = KindRemediationVerification, ResultFindingResolved
	}

	// Keyed on the run and the subject, so re-recording the same verified
	// outcome overwrites rather than accumulating duplicates. VerifiedAt is
	// excluded on purpose: it is stable for a given outcome, and including it
	// would let a re-record with a re-stamped clock create a second index row
	// under a second observed_at for the same fact.
	//
	// This is what makes replay safe. A workflow that is resumed, retried, or
	// re-reported produces the same identity, so both the evidence row and its
	// satisfaction-index row are upserts, not additions.
	key := stableID(
		project, string(domain), o.ClusterID, o.WorkflowRunID,
		o.FindingID, o.InvariantID, o.EntityRef,
	)
	id := "evidence.remediation." + key
	signalID := "signal.remediation." + key

	meta := map[string]string{
		"finding_id":         o.FindingID,
		"workflow_run_id":    o.WorkflowRunID,
		"remediation_status": string(o.Status()),
		"dispatched":         boolStr(o.Dispatched),
		"verified":           boolStr(o.Verified),
		"finding_resolved":   boolStr(o.FindingResolved),
	}
	if o.NodeID != "" {
		// The node the action ran on. Kept separate from EntityRef, which is the
		// subject the finding is about — they coincide for node-scoped findings
		// and diverge for service- and cluster-scoped ones.
		meta["node_id"] = o.NodeID
	}
	if !o.DispatchedAt.IsZero() {
		meta["dispatched_at"] = o.DispatchedAt.UTC().Format(rfc3339Nano)
	}
	if o.DispatchError != "" {
		meta["dispatch_error"] = o.DispatchError
	}
	if !qualifies {
		// Say why, in stable terms, so an operator reading a non-qualifying row
		// sees whether the repair failed or merely could not be attributed.
		meta["not_qualifying_reason"] = nonQualifyingReason(o)
	}

	// The Signal is the observation ("this remediation was verified against this
	// subject"); the Evidence supports it. Both are required: the bounded
	// recorder rejects a bundle without a Signal, and its delivery path uses the
	// recorded signal id as the evidence target. An evidence-only bundle would
	// be silently dropped at Enqueue as malformed.
	sig := api.Signal{
		ID:             signalID,
		Project:        project,
		Domain:         domain,
		Kind:           api.SignalAutomatedHealth,
		SourceKind:     SourceKindDoctorRemediationWorkflow,
		SourceRef:      o.FindingID,
		EntityRef:      o.EntityRef,
		Scope:          o.ClusterID,
		ClusterID:      o.ClusterID,
		ConditionRef:   o.InvariantID,
		Severity:       outcomeSeverity(o),
		AuthorityLevel: api.ObservationAuthorityDerived,
		ObservedAt:     observedAt(o),
		Payload:        o.Reason(),
		Status:         api.StatusRawSignal,
		Metadata:       meta,
	}

	return Bundle{
		Signal: sig,
		Evidence: []api.Evidence{{
			ID:      id,
			Project: project,
			Domain:  domain,
			// Bound to the signal above. The recorder overwrites these with the
			// id the server actually assigned, so setting them keeps the
			// in-process bundle self-consistent for callers that persist
			// directly to a store without going through the recorder.
			TargetKind:     "signal",
			TargetID:       signalID,
			ObservedFrom:   signalID,
			Kind:           kind,
			Lane:           api.LaneRuntimeRequired,
			Result:         result,
			SourceKind:     SourceKindDoctorRemediationWorkflow,
			SourceRef:      o.FindingID,
			EntityRef:      o.EntityRef,
			ClusterID:      o.ClusterID,
			ConditionRef:   o.InvariantID,
			Severity:       outcomeSeverity(o),
			AuthorityLevel: api.ObservationAuthorityDerived,
			// Computed from a post-action doctor sweep of collected cluster
			// state — derived, not asserted by the actor that ran the repair.
			// The executor's own report of its own success is exactly what this
			// authority level must not be confused with.
			ObservedAt: observedAt(o),
			Payload:    marshalPayload(outcomePayload(o)),
			// The governed action this verification is bound to. Without it a
			// fresh, correct-looking verification from an UNRELATED remediation
			// would satisfy a requirement about this one.
			ActionRef: o.WorkflowRunID,
			Metadata:  meta,
			Provenance: api.Provenance{
				SourceRef: o.WorkflowRunID,
				CreatedAt: observedAt(o),
			},
		}},
	}, qualifies
}

// BindRemediationEvidence applies the catalog mapper to a bundle produced by
// FromRemediationOutcome and returns the bound rows.
//
// Kept separate from the constructor rather than folded into it, unlike
// FromDoctorFinding: the eligibility decision above is what makes a row bindable
// at all, and a caller that wants to record a non-qualifying outcome should not
// have to reason about whether construction quietly stamped it. Binding a
// diagnostic row is harmless — no rule describes its kind, so it receives
// nothing — but the call site now says so out loud.
func BindRemediationEvidence(b *Bundle) {
	if b == nil {
		return
	}
	for i := range b.Evidence {
		cluster_operator.BindSatisfies(&b.Evidence[i])
	}
}

// rfc3339Nano matches the format the execute step stamps dispatched_at with, so
// the two are comparable without reparsing conventions.
const rfc3339Nano = "2006-01-02T15:04:05.999999999Z07:00"

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// observedAt is the verification time and nothing else. An outcome that was
// never verified has no observation time, and zero is the truthful answer —
// falling back to DispatchedAt would date the observation to the action, making
// an unverified remediation look like it was checked at dispatch.
func observedAt(o remediation.Outcome) int64 {
	if o.VerifiedAt.IsZero() {
		return 0
	}
	return o.VerifiedAt.Unix()
}

// diagnosticResult reports the outcome's own status, lowercased. Deliberately
// never ResultFindingResolved: a status of SUCCEEDED with thin lineage is still
// "succeeded", not "finding_resolved", so the accepted-result set of the
// verification rule cannot be reached through this path.
func diagnosticResult(o remediation.Outcome) string {
	return strings.ToLower(string(o.Status()))
}

func outcomeSeverity(o remediation.Outcome) string {
	switch o.Status() {
	case remediation.StatusSucceeded:
		return "info"
	case remediation.StatusPending:
		return "warning"
	case remediation.StatusDegraded:
		return "warning"
	default:
		return "error"
	}
}

// nonQualifyingReason distinguishes the two ways eligibility fails, and names
// the specific lineage defects when that is the cause. "Not verification
// evidence" without a reason sends an operator looking for a broken repair when
// the repair worked and only the provenance was thin.
func nonQualifyingReason(o remediation.Outcome) string {
	var parts []string
	if !o.IsSuccess() {
		parts = append(parts, "remediation_not_successful:"+strings.ToLower(string(o.Status())))
	}
	if defects := o.LineageDefects(); len(defects) > 0 {
		names := make([]string, 0, len(defects))
		for _, d := range defects {
			names = append(names, string(d))
		}
		parts = append(parts, "lineage_incomplete:"+strings.Join(names, ","))
	}
	return strings.Join(parts, ";")
}

// outcomePayloadWire is the bounded projection stored as the evidence payload.
// It is an explicit struct rather than the Outcome itself so the stored shape
// cannot silently widen with whatever fields Outcome grows later. It carries the
// verdict and its inputs — not the doctor snapshot the verdict was computed
// from, which belongs to the finding evidence rows.
type outcomePayloadWire struct {
	FindingID       string `json:"finding_id"`
	WorkflowRunID   string `json:"workflow_run_id"`
	ClusterID       string `json:"cluster_id"`
	InvariantID     string `json:"invariant_id"`
	EntityRef       string `json:"entity_ref"`
	NodeID          string `json:"node_id,omitempty"`
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	Dispatched      bool   `json:"dispatched"`
	Verified        bool   `json:"verified"`
	FindingResolved bool   `json:"finding_resolved"`
	DispatchedAt    string `json:"dispatched_at,omitempty"`
	VerifiedAt      string `json:"verified_at,omitempty"`
}

func outcomePayload(o remediation.Outcome) outcomePayloadWire {
	w := outcomePayloadWire{
		FindingID: o.FindingID, WorkflowRunID: o.WorkflowRunID,
		ClusterID: o.ClusterID, InvariantID: o.InvariantID,
		EntityRef: o.EntityRef, NodeID: o.NodeID,
		Status: string(o.Status()), Reason: o.Reason(),
		Dispatched: o.Dispatched, Verified: o.Verified,
		FindingResolved: o.FindingResolved,
	}
	if !o.DispatchedAt.IsZero() {
		w.DispatchedAt = o.DispatchedAt.UTC().Format(rfc3339Nano)
	}
	if !o.VerifiedAt.IsZero() {
		w.VerifiedAt = o.VerifiedAt.UTC().Format(rfc3339Nano)
	}
	return w
}
