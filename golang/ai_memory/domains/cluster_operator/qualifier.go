package cluster_operator

// qualifier.go connects the satisfaction catalog to CheckAction.
//
// Everything from 4A through 4D built the rules and proved they discriminate.
// Nothing consulted them at the gate: CheckAction resolved required evidence by
// listing rows whose TargetID happened to be the principle, which answers "does
// evidence for this ref exist somewhere in this project" — not "does evidence
// that COUNTS exist, for this cluster, this subject, this action, now".
//
// This is the adapter that makes the gate ask the second question.

import (
	"context"
	"errors"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// Pack implements domain.EvidenceQualifier.
var _ domain.EvidenceQualifier = (*Pack)(nil)

// QualifyRequirement answers whether stored evidence satisfies one requirement
// for one concrete action.
//
// FAILS CLOSED, in three distinct ways that must not be collapsed:
//
//   - A store error propagates. A governor that cannot read its evidence has
//     not learned the evidence is absent.
//   - A requirement with no catalog rule is UNSATISFIED, not "unconstrained".
//     A principle may only demand evidence this domain knows how to judge;
//     otherwise the requirement would be silently waived and the principle would
//     read as satisfied on the strength of nothing.
//   - No qualifying candidate is unsatisfied, carrying the first rejection so
//     the caller can distinguish "nothing stored" from "stored and refused".
func (p *Pack) QualifyRequirement(ctx context.Context, s store.Store, q domain.RequirementQuery) (domain.RequirementVerdict, error) {
	if q.EvaluatedAt == 0 {
		return domain.RequirementVerdict{}, errors.New("qualify requirement: evaluated_at is required (inject the clock)")
	}

	res, err := QualifyEvidence(ctx, s, SatisfactionQuery{
		Project:       q.Project,
		Domain:        DomainName,
		ClusterID:     q.ClusterID,
		RequirementID: q.RequirementID,
		EntityRef:     q.EntityRef,
		ConditionRef:  q.ConditionRef,
		SourceRef:     q.SourceRef,
		WorkflowRunID: q.ActionRef,
		ActionDispatchedAt: func() time.Time {
			if q.ActionDispatchedAt == 0 {
				return time.Time{}
			}
			return time.Unix(q.ActionDispatchedAt, 0)
		}(),
		EvaluatedAt: time.Unix(q.EvaluatedAt, 0),
	})
	switch {
	case errors.Is(err, ErrRequirementNotDeclared):
		// Undeclared is not unconstrained. Reported as a distinct, stable reason
		// so an operator sees a CATALOG gap rather than being sent to gather
		// evidence that no rule could ever accept.
		return domain.RequirementVerdict{
			Satisfied: false,
			Reason:    string(RejectRequirementNotDeclared),
			Detail:    "no satisfaction rule declares how " + q.RequirementID + " may be satisfied in this domain",
		}, nil
	case err != nil:
		return domain.RequirementVerdict{}, err
	}

	if res.Satisfied() {
		ids := make([]string, 0, len(res.Qualified))
		for _, e := range res.Qualified {
			ids = append(ids, e.ID)
		}
		return domain.RequirementVerdict{Satisfied: true, EvidenceIDs: ids}, nil
	}

	v := domain.RequirementVerdict{Satisfied: false}
	if len(res.Rejected) > 0 {
		// Rejections are sorted most-informative first (see rejectionRank), with
		// evidence id as a tie-break, so this is both deterministic for the same
		// stored state AND the near miss rather than an arbitrary wrong-subject
		// row. Only one reason reaches the operator, so which one it is decides
		// whether the message helps or misleads.
		v.Reason, v.Detail = string(res.Rejected[0].Reason), res.Rejected[0].Detail
	}
	return v, nil
}
