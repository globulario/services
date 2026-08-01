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
var satisfactionCatalog = []SatisfactionRule{
	{
		ID:               "sat.doctor.finding_observed",
		RequirementID:    "evidence.doctor.finding_observed",
		EvidenceKind:     "doctor_finding",
		SourceKind:       "doctor",
		SubjectMatch:     SubjectInvariantAndEntity,
		MinimumAuthority: api.ObservationAuthorityDiagnostic,
		AcceptedResults:  []string{"observed"},
		Freshness:        FreshnessMaxAge,
		MaxAge:           15 * time.Minute,
	},
	{
		ID:            "sat.remediation.fresh_convergence_verified",
		RequirementID: "evidence.remediation.fresh_convergence_verification",
		EvidenceKind:  "remediation_verification",
		SourceKind:    "workflow",
		SubjectMatch:  SubjectInvariantAndEntity,
		// DERIVED_EVIDENCE floor: this must be computed from observed cluster
		// state after the action. A DIAGNOSTIC_CLAIM would let the actor's own
		// report of success satisfy the requirement to prove success.
		MinimumAuthority: api.ObservationAuthorityDerived,
		AcceptedResults:  []string{"finding_resolved"},
		// Both bindings: after the action, and from this run. Either alone is
		// insufficient — a fresh verification from an unrelated remediation
		// would otherwise qualify.
		Freshness: FreshnessNewerThanAction,
		MaxAge:    10 * time.Minute,
	},
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
	sort.Slice(res.Rejected, func(i, j int) bool { return res.Rejected[i].EvidenceID < res.Rejected[j].EvidenceID })
	return res, nil
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
		if q.EntityRef == "" || ev.EntityRef != q.EntityRef {
			return &RejectedEvidence{Reason: RejectSubjectMismatch,
				Detail: fmt.Sprintf("entity: want %q got %q", q.EntityRef, ev.EntityRef)}
		}
		if q.ConditionRef == "" || ev.ConditionRef != q.ConditionRef {
			return &RejectedEvidence{Reason: RejectSubjectMismatch,
				Detail: fmt.Sprintf("condition: want %q got %q", q.ConditionRef, ev.ConditionRef)}
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

	switch rule.Freshness {
	case FreshnessSameWorkflowRun:
		if q.WorkflowRunID == "" || ev.Metadata["workflow_run_id"] != q.WorkflowRunID {
			return &RejectedEvidence{Reason: RejectNotBoundToAction,
				Detail: fmt.Sprintf("workflow_run_id: want %q got %q", q.WorkflowRunID, ev.Metadata["workflow_run_id"])}
		}
	case FreshnessNewerThanAction:
		if q.ActionDispatchedAt.IsZero() {
			return &RejectedEvidence{Reason: RejectNotBoundToAction,
				Detail: "rule requires evidence newer than the action, but ActionDispatchedAt was not supplied"}
		}
		// Evidence gathered before the action cannot show the action worked.
		if !observed.After(q.ActionDispatchedAt) {
			return &RejectedEvidence{Reason: RejectNotBoundToAction,
				Detail: fmt.Sprintf("observed_at %s predates action dispatch %s", observed, q.ActionDispatchedAt)}
		}
		if q.WorkflowRunID != "" && ev.Metadata["workflow_run_id"] != q.WorkflowRunID {
			return &RejectedEvidence{Reason: RejectNotBoundToAction,
				Detail: fmt.Sprintf("workflow_run_id: want %q got %q", q.WorkflowRunID, ev.Metadata["workflow_run_id"])}
		}
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
