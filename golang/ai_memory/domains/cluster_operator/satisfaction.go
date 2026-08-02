package cluster_operator

// satisfaction.go decides whether stored evidence is ELIGIBLE to satisfy a
// behavioral requirement. Commit 3 answered "what evidence exists for this
// reference"; this answers "does any of it actually count".
//
// The distinction is the whole point of a governor. Possessing evidence and
// possessing the CORRECT evidence — from the right authority, about the right
// subject, in the right cluster, with the right result, at the right time — are
// different claims, and conflating them produces a gate that approves on the
// strength of something irrelevant.
//
// POLICY LIVES HERE, NOT IN THE KERNEL
//
// The behavioral kernel supplies generic primitives (lookup, authority
// ordering, deterministic ordering). Which evidence kind satisfies which
// requirement, what authority a requirement demands, and how fresh it must be
// are cluster semantics and belong to this domain pack. Pushing any of it into
// the kernel would hardcode cluster knowledge into machinery meant to serve
// other domains.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// ── rejection reasons ───────────────────────────────────────────────────────

// RejectionReason is a stable constant, not prose. CheckAction must be able to
// explain WHY governance was not satisfied without parsing a message, and an
// operator must be able to tell "you have no evidence" from "you have evidence
// that does not count".
type RejectionReason string

const (
	// Must equal domain.ReasonRequirementNotDeclared: the kernel reads this one
	// code to decide whether caller self-assertion may still cover the
	// requirement. Asserted by TestQualifyRequirement_NotDeclaredMatchesKernelContract.
	RejectRequirementNotDeclared RejectionReason = "requirement_not_declared"
	RejectClusterMismatch        RejectionReason = "cluster_mismatch"
	RejectSubjectMismatch        RejectionReason = "subject_mismatch"
	RejectAuthorityInsufficient  RejectionReason = "authority_insufficient"
	RejectAuthorityUnknown       RejectionReason = "authority_unknown"
	RejectResultNotAccepted      RejectionReason = "result_not_accepted"
	RejectEvidenceStale          RejectionReason = "evidence_stale"
	RejectTimestampMissing       RejectionReason = "evidence_timestamp_missing"
	RejectTimestampInFuture      RejectionReason = "evidence_timestamp_in_future"
	RejectNotBoundToAction       RejectionReason = "evidence_not_bound_to_action"
)

// ErrRequirementNotDeclared distinguishes "this requirement is not configured"
// from "this requirement has no matching evidence". Collapsing them would let an
// unconfigured requirement read as an unsatisfied one — the governor would block
// a lawful action and blame missing evidence, sending the operator to gather
// evidence that could never have qualified.
var ErrRequirementNotDeclared = errors.New("requirement not declared in satisfaction catalog")

// ── freshness ───────────────────────────────────────────────────────────────

// FreshnessPolicy states how recency is judged. Time alone is insufficient for
// verification evidence: a fresh timestamp from an UNRELATED remediation must
// not satisfy a requirement about THIS one, which is why binding policies exist
// alongside the age-based one.
type FreshnessPolicy string

const (
	// FreshnessNone accepts any age. Only for evidence whose truth does not
	// decay (a signed artifact digest, say).
	FreshnessNone FreshnessPolicy = "no_expiration"
	// FreshnessMaxAge accepts evidence observed within MaxAge of evaluation.
	FreshnessMaxAge FreshnessPolicy = "max_age"
	// FreshnessSameWorkflowRun additionally requires the evidence to carry the
	// evaluating workflow run id.
	FreshnessSameWorkflowRun FreshnessPolicy = "same_workflow_run"
	// FreshnessNewerThanAction additionally requires the evidence to have been
	// observed after the governed action was dispatched. This is what makes
	// post-remediation verification meaningful: evidence gathered BEFORE the
	// action cannot show the action worked.
	FreshnessNewerThanAction FreshnessPolicy = "newer_than_action"
	// FreshnessBoundToAction requires BOTH: the evidence carries the evaluating
	// action's ref, AND it was observed after that action was dispatched.
	//
	// Distinct from FreshnessNewerThanAction, which relaxes the run check when
	// the caller omits a run id. That relaxation is acceptable for a rule whose
	// discrimination rests elsewhere; it is not acceptable for a rule whose
	// entire claim is "this action worked". Here a query without an action
	// identity is REJECTED rather than quietly evaluated on time alone —
	// otherwise a caller who forgets the binding gets the more permissive
	// answer, which is the wrong direction to fail in.
	FreshnessBoundToAction FreshnessPolicy = "bound_to_action"
)

// SubjectMatch states how tightly evidence must be bound to the thing being
// governed. Deliberately a small closed set — a generic fuzzy matcher is how
// unrelated evidence starts qualifying.
type SubjectMatch string

const (
	// SubjectCluster requires only that the cluster matches (already enforced
	// structurally by the index partition).
	SubjectCluster SubjectMatch = "same_cluster"
	// SubjectEntity requires the same entity ref.
	SubjectEntity SubjectMatch = "same_entity"
	// SubjectInvariantAndEntity requires both the condition/invariant and the
	// entity to match. A verification for one service instance must not satisfy
	// a requirement about another merely because they share an invariant.
	SubjectInvariantAndEntity SubjectMatch = "same_invariant_and_entity"
	// SubjectFindingLineage additionally requires the evidence to name the same
	// originating finding (SourceRef).
	//
	// Invariant plus entity is not the same claim as "this finding". One entity
	// can raise the same invariant repeatedly, and a verification of an earlier
	// occurrence must not stand in for a later one. Checking it HERE rather than
	// trusting the producer matters because a governor must be able to reject a
	// row it did not write.
	SubjectFindingLineage SubjectMatch = "same_invariant_entity_and_finding"
)

// ── catalog ─────────────────────────────────────────────────────────────────

// SatisfactionRule declares that a class of evidence may satisfy a requirement,
// and under exactly what conditions. Every field is explicit: nothing about
// satisfaction is inferred from string similarity, shared prefixes, source
// component names, or evidence descriptions.
type SatisfactionRule struct {
	// ID is stable and reviewable; it appears in traces so a decision can be
	// traced back to the rule that allowed it.
	ID string
	// RequirementID is the required-evidence ref this rule can satisfy.
	RequirementID string
	// EvidenceKind must match exactly.
	EvidenceKind string
	// SourceKind, when set, must match exactly. Empty means any source.
	SourceKind string

	SubjectMatch     SubjectMatch
	MinimumAuthority api.ObservationAuthorityLevel
	// AcceptedResults is exhaustive and exact. Existence is never satisfaction:
	// "verification_started" or "dispatch_accepted" describe an attempt, not an
	// outcome, and must not appear here for a verification requirement.
	AcceptedResults []string

	Freshness FreshnessPolicy
	MaxAge    time.Duration
}

// satisfactionCatalog is the declarative mapping. It is intentionally small and
// grounded in evidence the cluster_operator domain already produces; this is not
// a universal evidence ontology.
//
// A rule is added only when its producer can honestly supply every field. Where
// a producer cannot, the gap is recorded rather than papered over with a
// permissive rule — a lenient catalog entry is indistinguishable from no
// governance at all, but looks like governance.
// CORRECTION (commit 4B): the first version of this catalog was written against
// producers that did not exist. Every field of the doctor rule was wrong —
// kind "doctor_finding" vs the emitted "cluster_doctor_evidence", source
// "doctor" vs "cluster_doctor_evidence", result "observed" vs "claim", floor
// DIAGNOSTIC_CLAIM vs the emitted DERIVED_EVIDENCE — and the remediation rule
// described an evidence shape no code produces.
//
// It passed its tests because those tests seeded evidence matching the invented
// shape: catalog and tests agreed with each other and neither agreed with
// production. That is the same semantic guessing this catalog exists to forbid,
// so the rules below are now transcribed from the emitting code, not imagined.
var satisfactionCatalog = []SatisfactionRule{
	{
		// Producer: observation.FromDoctorFinding (cluster_operator/observation
		// /ingest.go). Every field below is read off that constructor.
		ID:            "sat.doctor.finding_observed",
		RequirementID: "evidence.doctor.finding_observed",
		EvidenceKind:  "cluster_doctor_evidence", // observation.SourceKindClusterDoctorEvidence
		SourceKind:    "cluster_doctor_evidence", // literal, not the const: observation imports THIS package for the mapper, so importing it back would cycle
		SubjectMatch:  SubjectInvariantAndEntity,
		// The producer stamps DERIVED_EVIDENCE: a doctor finding is computed
		// from collected cluster state, not asserted by the actor. The floor
		// matches what is actually emitted rather than a level chosen in the
		// abstract — a floor no producer can reach is an unsatisfiable rule.
		MinimumAuthority: api.ObservationAuthorityDerived,
		// "claim" is the producer's result for a finding's evidence rows. It is
		// honest for THIS requirement, which asks only that a finding was
		// observed — not that anything was verified. A requirement about
		// verification must never accept it.
		AcceptedResults: []string{"claim"},
		Freshness:       FreshnessMaxAge,
		MaxAge:          15 * time.Minute,
	},

	{
		// GAP CLOSED (commit 4C). The producer is
		// observation.FromRemediationOutcome (cluster_operator/observation
		// /remediation.go), fed by the real remediate.doctor.finding chain:
		// resolve → execute (stamps dispatched_at) → verify →
		// buildRemediationOutcome. Every field below is transcribed from that
		// constructor, not chosen in the abstract — the 4A failure was a rule
		// whose fields described an imagined producer and whose tests were
		// built from the same imagination.
		ID:            "sat.remediation.fresh_convergence_verified",
		RequirementID: "evidence.remediation.fresh_convergence_verification",
		// The producer emits this kind ONLY when the outcome both succeeded and
		// carries complete lineage. Every other outcome is emitted under
		// KindRemediationDiagnostic, which no rule describes — so an ineligible
		// remediation is not merely rejected here, it can never become a
		// candidate.
		EvidenceKind: "remediation_verification_evidence", // observation.KindRemediationVerification
		SourceKind:   "doctor_remediation_workflow",       // observation.SourceKindDoctorRemediationWorkflow
		// A verification for one subject must not satisfy a requirement about
		// another merely because they share an invariant or a cluster — nor may
		// a verification of an EARLIER occurrence of the same invariant on the
		// same entity stand in for the finding actually being governed.
		SubjectMatch: SubjectFindingLineage,
		// Computed by a post-action doctor sweep over collected cluster state.
		// Not TRUTH_PLANE: the doctor interprets collected state, it does not
		// own it. Not DIAGNOSTIC_CLAIM either — that is what the executor's
		// report of its own success would be, and this floor exists to keep
		// such a report from ever qualifying.
		MinimumAuthority: api.ObservationAuthorityDerived,
		// Exhaustive and exact. "succeeded", "dispatched" and
		// "verification_started" describe a status or an attempt; only
		// "finding_resolved" states that the post-action sweep found the
		// original finding gone.
		AcceptedResults: []string{"finding_resolved"}, // observation.ResultFindingResolved
		// Both halves are mandatory: the evidence must carry THIS workflow
		// run's ref, and must have been observed after the action was
		// dispatched. Time alone would let a stale verification of an earlier
		// repair authorize a later one; the run binding alone would let a
		// verification stamped before dispatch count as proof the action
		// worked.
		Freshness: FreshnessBoundToAction,
		// A verification more than 15 minutes old is history, not current
		// convergence state — matched to the doctor rule so the two halves of
		// the loop age out together.
		MaxAge: 15 * time.Minute,
	},

	// GAP CLOSED (commit 4D): finding lineage used to be producer-guaranteed
	// only, because source_ref was absent from the index projection. It is now
	// projected and required by SubjectFindingLineage, so a row this pipeline
	// did not write is checked on the same terms as one it did.
}

// ── query and result ────────────────────────────────────────────────────────

type SatisfactionQuery struct {
	Project       string
	Domain        api.DomainRef
	ClusterID     string
	RequirementID string

	// Subject narrows to the governed thing.
	EntityRef    string
	ConditionRef string
	// SourceRef is the originating finding. Required by SubjectFindingLineage.
	SourceRef string

	// WorkflowRunID and ActionDispatchedAt bind evidence to THIS operation.
	// Required by the binding freshness policies.
	WorkflowRunID      string
	ActionDispatchedAt time.Time

	// EvaluatedAt is the injected clock. Callers must set it; tests must not
	// depend on wall-clock time.
	EvaluatedAt time.Time
}

type RejectedEvidence struct {
	EvidenceID string
	RuleID     string
	Reason     RejectionReason
	// Detail carries the comparison that failed (required vs observed) without
	// the evidence payload — enough to explain, not enough to leak.
	Detail string
}

type SatisfactionResult struct {
	RequirementID string
	RuleIDs       []string
	Qualified     []api.Evidence
	Rejected      []RejectedEvidence
}

// Satisfied reports whether at least one candidate qualified.
func (r SatisfactionResult) Satisfied() bool { return len(r.Qualified) > 0 }

// ── evaluation ──────────────────────────────────────────────────────────────

// RulesForRequirement returns the catalog rules for a requirement, in stable
// order. Catalog iteration never depends on Go map order.
func RulesForRequirement(requirementID string) []SatisfactionRule {
	var out []SatisfactionRule
	for _, r := range satisfactionCatalog {
		if r.RequirementID == requirementID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// QualifyEvidence finds candidates for a requirement and evaluates each against
// the catalog.
//
// The four outcomes are deliberately distinct and must not be collapsed:
//
//	malformed query                → error
//	requirement not in catalog     → ErrRequirementNotDeclared
//	no stored candidates           → empty result, no error
//	candidates none qualifying     → result carrying rejection diagnostics
func QualifyEvidence(ctx context.Context, s store.Store, q SatisfactionQuery) (SatisfactionResult, error) {
	if q.Project == "" || q.Domain == "" || q.RequirementID == "" {
		return SatisfactionResult{}, fmt.Errorf("satisfaction query requires project, domain and requirement_id")
	}
	if q.EvaluatedAt.IsZero() {
		return SatisfactionResult{}, fmt.Errorf("satisfaction query requires EvaluatedAt (inject the clock)")
	}

	rules := RulesForRequirement(q.RequirementID)
	if len(rules) == 0 {
		return SatisfactionResult{}, fmt.Errorf("%w: %s", ErrRequirementNotDeclared, q.RequirementID)
	}

	res := SatisfactionResult{RequirementID: q.RequirementID}
	for _, r := range rules {
		res.RuleIDs = append(res.RuleIDs, r.ID)
	}

	// Cluster scope is structural: ClusterID participates in the index
	// partition, so this cannot silently widen to other clusters.
	candidates, err := s.ListEvidenceSatisfying(ctx, store.EvidenceSatisfactionQuery{
		Project:             q.Project,
		Domain:              string(q.Domain),
		RequiredEvidenceRef: q.RequirementID,
		ClusterID:           q.ClusterID,
	})
	if err != nil {
		return SatisfactionResult{}, fmt.Errorf("evidence lookup: %w", err)
	}

	for _, ev := range candidates {
		qualified := false
		var lastRejection *RejectedEvidence
		for _, rule := range rules {
			if rej := evaluate(rule, ev, q); rej != nil {
				rej.EvidenceID, rej.RuleID = ev.ID, rule.ID
				lastRejection = rej
				continue
			}
			res.Qualified = append(res.Qualified, ev)
			qualified = true
			break
		}
		if !qualified && lastRejection != nil {
			res.Rejected = append(res.Rejected, *lastRejection)
		}
	}

	// Newest first, stable ID tie-break — identical across stores and repeatable
	// for the same state and clock.
	sortEvidence(res.Qualified)
	// Most informative first, evidence id as the deterministic tie-break.
	//
	// This used to sort by evidence id alone. Deterministic, but the id is a
	// UUID, so which rejection got reported was effectively arbitrary — and only
	// the first one is surfaced to the operator. On 2026-08-01 a refused
	// remediation reported
	//
	//   subject_mismatch: entity: want ".../globular-torrent.service"
	//                      got  ".../sha256sum"
	//
	// which says nothing except that some unrelated row exists. The rejection
	// that mattered — evidence about the RIGHT subject that failed a later
	// discriminator — was somewhere further down the slice, unread. Hours went
	// into chasing the wrong subject.
	//
	// A subject mismatch is the least informative rejection there is: it means
	// "this evidence is about something else", which is true of most rows in a
	// shared partition and tells the operator nothing about their action. A
	// rejection that got PAST the subject and failed on authority, freshness,
	// result or action-binding is a near miss, and a near miss is the one thing
	// worth reporting.
	sort.Slice(res.Rejected, func(i, j int) bool {
		ri, rj := rejectionRank(res.Rejected[i].Reason), rejectionRank(res.Rejected[j].Reason)
		if ri != rj {
			return ri < rj
		}
		return res.Rejected[i].EvidenceID < res.Rejected[j].EvidenceID
	})
	return res, nil
}

// rejectionRank orders rejections by how much they tell an operator about the
// action they asked about. Lower is more informative.
//
// Deliberately coarse: two buckets, not a per-reason ranking. The distinction
// that matters is "this evidence was about your subject and still did not
// count" versus "this evidence was about something else entirely". Finer
// ordering between the near-miss reasons would be a preference dressed up as a
// rule, and would have to be re-litigated every time a reason is added.
func rejectionRank(r RejectionReason) int {
	switch r {
	case RejectSubjectMismatch, RejectClusterMismatch:
		return 1 // about a different subject — says nothing about this action
	default:
		return 0 // right subject, failed a real discriminator
	}
}

// evaluate returns nil when the evidence qualifies under the rule, else the
// first reason it did not.
func evaluate(rule SatisfactionRule, ev api.Evidence, q SatisfactionQuery) *RejectedEvidence {
	if rule.EvidenceKind != "" && ev.Kind != rule.EvidenceKind {
		return &RejectedEvidence{Reason: RejectSubjectMismatch,
			Detail: fmt.Sprintf("evidence_kind: want %q got %q", rule.EvidenceKind, ev.Kind)}
	}
	if rule.SourceKind != "" && ev.SourceKind != rule.SourceKind {
		return &RejectedEvidence{Reason: RejectSubjectMismatch,
			Detail: fmt.Sprintf("source_kind: want %q got %q", rule.SourceKind, ev.SourceKind)}
	}
	if ev.ClusterID != q.ClusterID {
		return &RejectedEvidence{Reason: RejectClusterMismatch,
			Detail: fmt.Sprintf("cluster: want %q got %q", q.ClusterID, ev.ClusterID)}
	}

	switch rule.SubjectMatch {
	case SubjectEntity:
		if q.EntityRef == "" || ev.EntityRef != q.EntityRef {
			return &RejectedEvidence{Reason: RejectSubjectMismatch,
				Detail: fmt.Sprintf("entity: want %q got %q", q.EntityRef, ev.EntityRef)}
		}
	case SubjectInvariantAndEntity:
		if rej := matchInvariantAndEntity(ev, q); rej != nil {
			return rej
		}
	case SubjectFindingLineage:
		if rej := matchInvariantAndEntity(ev, q); rej != nil {
			return rej
		}
		// Mandatory: a query that names no finding cannot be answered by
		// evidence about one, and failing open here would let a verification of
		// an earlier occurrence of the same invariant satisfy a later one.
		if q.SourceRef == "" {
			return &RejectedEvidence{Reason: RejectSubjectMismatch,
				Detail: "rule requires finding lineage, but the query supplied no source_ref"}
		}
		if ev.SourceRef != q.SourceRef {
			return &RejectedEvidence{Reason: RejectSubjectMismatch,
				Detail: fmt.Sprintf("finding: want %q got %q", q.SourceRef, ev.SourceRef)}
		}
	case SubjectCluster:
		// cluster already checked above
	}

	if _, ok := api.AuthorityRank(ev.AuthorityLevel); !ok {
		return &RejectedEvidence{Reason: RejectAuthorityUnknown,
			Detail: fmt.Sprintf("authority %q is not rankable", ev.AuthorityLevel)}
	}
	if !api.AuthorityAtLeast(ev.AuthorityLevel, rule.MinimumAuthority) {
		return &RejectedEvidence{Reason: RejectAuthorityInsufficient,
			Detail: fmt.Sprintf("authority: floor %q got %q", rule.MinimumAuthority, ev.AuthorityLevel)}
	}

	// Existence is not satisfaction. An unlisted result never qualifies, so an
	// unknown or future result value fails closed.
	accepted := false
	for _, r := range rule.AcceptedResults {
		if ev.Result == r {
			accepted = true
			break
		}
	}
	if !accepted {
		return &RejectedEvidence{Reason: RejectResultNotAccepted,
			Detail: fmt.Sprintf("result %q not in accepted set", ev.Result)}
	}

	return evaluateFreshness(rule, ev, q)
}

func evaluateFreshness(rule SatisfactionRule, ev api.Evidence, q SatisfactionQuery) *RejectedEvidence {
	if rule.Freshness == FreshnessNone {
		return nil
	}
	if ev.ObservedAt == 0 {
		return &RejectedEvidence{Reason: RejectTimestampMissing,
			Detail: "freshness required but observed_at is unset"}
	}
	observed := time.Unix(ev.ObservedAt, 0)

	// A timestamp after evaluation is not "very fresh" — it is a clock problem
	// or a fabricated record, and must not be treated as the freshest evidence.
	if observed.After(q.EvaluatedAt.Add(clockSkewTolerance)) {
		return &RejectedEvidence{Reason: RejectTimestampInFuture,
			Detail: fmt.Sprintf("observed_at %s is after evaluation %s", observed, q.EvaluatedAt)}
	}

	if rule.MaxAge > 0 {
		if age := q.EvaluatedAt.Sub(observed); age > rule.MaxAge {
			return &RejectedEvidence{Reason: RejectEvidenceStale,
				Detail: fmt.Sprintf("age %s exceeds max %s", age.Round(time.Second), rule.MaxAge)}
		}
	}

	// The action binding is read from the first-class ActionRef, never from
	// Metadata. The satisfaction index projects a fixed column set and does not
	// carry Metadata, so a binding kept there would hold in memory and vanish
	// across a Scylla round trip — enforcement that passes every unit test and
	// silently never fires in production.
	switch rule.Freshness {
	case FreshnessSameWorkflowRun:
		if q.WorkflowRunID == "" || ev.ActionRef != q.WorkflowRunID {
			return &RejectedEvidence{Reason: RejectNotBoundToAction,
				Detail: fmt.Sprintf("action_ref: want %q got %q", q.WorkflowRunID, ev.ActionRef)}
		}
	case FreshnessNewerThanAction:
		if rej := requireObservedAfterDispatch(observed, q); rej != nil {
			return rej
		}
		if q.WorkflowRunID != "" && ev.ActionRef != q.WorkflowRunID {
			return &RejectedEvidence{Reason: RejectNotBoundToAction,
				Detail: fmt.Sprintf("action_ref: want %q got %q", q.WorkflowRunID, ev.ActionRef)}
		}
	case FreshnessBoundToAction:
		// Mandatory, not best-effort: an unidentified action cannot be the one
		// this evidence supposedly verified.
		if q.WorkflowRunID == "" {
			return &RejectedEvidence{Reason: RejectNotBoundToAction,
				Detail: "rule requires an action binding, but the query supplied no workflow run id"}
		}
		if ev.ActionRef != q.WorkflowRunID {
			return &RejectedEvidence{Reason: RejectNotBoundToAction,
				Detail: fmt.Sprintf("action_ref: want %q got %q", q.WorkflowRunID, ev.ActionRef)}
		}
		if rej := requireObservedAfterDispatch(observed, q); rej != nil {
			return rej
		}
	}
	return nil
}

// matchInvariantAndEntity is the shared subject check. Both refs are mandatory
// on the query side: an unspecified subject must not widen the match.
func matchInvariantAndEntity(ev api.Evidence, q SatisfactionQuery) *RejectedEvidence {
	if q.EntityRef == "" || ev.EntityRef != q.EntityRef {
		return &RejectedEvidence{Reason: RejectSubjectMismatch,
			Detail: fmt.Sprintf("entity: want %q got %q", q.EntityRef, ev.EntityRef)}
	}
	if q.ConditionRef == "" || ev.ConditionRef != q.ConditionRef {
		return &RejectedEvidence{Reason: RejectSubjectMismatch,
			Detail: fmt.Sprintf("condition: want %q got %q", q.ConditionRef, ev.ConditionRef)}
	}
	return nil
}

// requireObservedAfterDispatch enforces that the observation postdates the
// action. Evidence gathered before the action cannot show the action worked, and
// a missing dispatch time makes the comparison unanswerable — which is a
// rejection, not a pass.
func requireObservedAfterDispatch(observed time.Time, q SatisfactionQuery) *RejectedEvidence {
	if q.ActionDispatchedAt.IsZero() {
		return &RejectedEvidence{Reason: RejectNotBoundToAction,
			Detail: "rule requires evidence newer than the action, but ActionDispatchedAt was not supplied"}
	}
	if !observed.After(q.ActionDispatchedAt) {
		return &RejectedEvidence{Reason: RejectNotBoundToAction,
			Detail: fmt.Sprintf("observed_at %s predates action dispatch %s", observed, q.ActionDispatchedAt)}
	}
	return nil
}

// clockSkewTolerance bounds how far ahead of the evaluator an observation may be
// stamped before it is treated as invalid rather than fresh.
const clockSkewTolerance = 30 * time.Second

func sortEvidence(evs []api.Evidence) {
	sort.Slice(evs, func(i, j int) bool {
		if evs[i].ObservedAt != evs[j].ObservedAt {
			return evs[i].ObservedAt > evs[j].ObservedAt
		}
		return evs[i].ID < evs[j].ID
	})
}

// ── producer-side binding ───────────────────────────────────────────────────

// BindSatisfies stamps an evidence row with the requirement IDs the catalog says
// its shape can satisfy. Producers call this BEFORE persistence, so the
// satisfaction index is fed from the same declaration the qualifier later reads.
//
// Without it the index holds nothing for a requirement and the evidence is not
// rejected — it is never a CANDIDATE. That distinction matters: a governor
// looking at an unfed index reports "no evidence" for evidence that plainly
// exists, and sends an operator to gather what is already there.
//
// Deliberately narrow:
//
//   - Deterministic. Matching is exact on kind and source; nothing is inferred
//     from similarity, prefixes, descriptions or tags.
//   - Idempotent. Applying it twice cannot duplicate a binding, so a producer
//     that maps defensively and a caller that maps again stay consistent.
//   - Silent on unknown shapes. Evidence no rule describes stays recordable and
//     unqualified rather than receiving an invented binding — an unknown shape
//     that acquired a requirement ID would be worse than one carrying none.
//   - Additive only. Existing bindings are preserved; this never removes a
//     requirement another producer or a migration set deliberately.
//
// It does NOT reinterpret stored evidence. Historical rows without bindings stay
// historical and unqualified until an explicit migration decides otherwise.
func BindSatisfies(e *api.Evidence) {
	if e == nil {
		return
	}
	have := make(map[string]bool, len(e.Satisfies))
	for _, r := range e.Satisfies {
		have[string(r)] = true
	}

	var added []string
	for _, rule := range satisfactionCatalog {
		if rule.EvidenceKind != "" && e.Kind != rule.EvidenceKind {
			continue
		}
		if rule.SourceKind != "" && e.SourceKind != rule.SourceKind {
			continue
		}
		if have[rule.RequirementID] {
			continue
		}
		have[rule.RequirementID] = true
		added = append(added, rule.RequirementID)
	}
	// Sorted so the emitted set never depends on catalog iteration order — the
	// stored row and its index entries must be reproducible.
	sort.Strings(added)
	for _, ref := range added {
		e.Satisfies = append(e.Satisfies, api.RequiredEvidenceRef(ref))
	}
}
