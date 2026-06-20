package domain

// loader.go writes a domain pack's catalogs into the behavioral-memory store.
// It is GENERIC: it maps generic CatalogEntry / PrincipleSeed into api types and
// persists them via the store port. It contains no cluster knowledge and no
// driver code.
//
// Loading is idempotent and NON-DESTRUCTIVE: re-running re-writes catalog rows
// (same data) and re-proposes seed principles only while they are still merely
// proposed — a seed principle that has since been promoted or revoked is left
// untouched, so load never silently demotes governed state.
//
// Seed principles are written at PROPOSED_PRINCIPLE. The loader NEVER promotes —
// promotion stays behind the gate (see core/governance.go).

import (
	"context"
	"errors"
	"fmt"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// LoadResult reports what a load wrote.
type LoadResult struct {
	Authorities      int
	Conditions       int
	PrinciplesSeeded int
	PrinciplesSkipped int // already promoted/revoked — left as-is
}

func toRefs[T ~string](in []string) []T {
	out := make([]T, len(in))
	for i, v := range in {
		out[i] = T(v)
	}
	return out
}

func seedMeta(domainName string) map[string]string {
	return map[string]string{"source": "seed", "immutable": "true", "domain_pack": domainName}
}

// LoadCatalogs persists a domain's authority/condition catalog rows and proposes
// its seed principles into the store under the given project. Forbidden-move and
// required-evidence catalogs have no store tables (they are validated in-pack and
// referenced by principles), so they are not persisted here.
func LoadCatalogs(ctx context.Context, st store.Store, project string, d Domain) (LoadResult, error) {
	var res LoadResult
	if project == "" {
		return res, fmt.Errorf("load catalogs: project is required")
	}
	cats := d.Catalogs()
	dom := api.DomainRef(d.Name())

	for _, a := range cats.Authorities {
		au := &api.Authority{
			ID: a.ID, Project: project, Domain: dom, Title: a.Title,
			Governs: a.Fields["governs"], OwnerKind: a.Fields["owner_kind"],
			ReadPath: a.Fields["read_path"], WritePath: a.Fields["write_path"], IdentitySource: a.Fields["identity_source"],
			Metadata: seedMeta(d.Name()),
		}
		if err := st.PutAuthority(ctx, au); err != nil {
			return res, fmt.Errorf("load authority %q: %w", a.ID, err)
		}
		res.Authorities++
	}

	for _, c := range cats.Conditions {
		cond := &api.Condition{
			ID: c.ID, Project: project, Domain: dom, Title: c.Title,
			DetectSpec: c.Fields["detect_spec"], Severity: c.Fields["severity"],
			Metadata: seedMeta(d.Name()),
		}
		if err := st.PutCondition(ctx, cond); err != nil {
			return res, fmt.Errorf("load condition %q: %w", c.ID, err)
		}
		res.Conditions++
	}

	for _, ps := range cats.Principles {
		// Non-destructive: do not reset an already-governed principle to PROPOSED.
		if existing, err := st.GetPrinciple(ctx, project, d.Name(), ps.ID); err == nil {
			if existing.Status != api.StatusProposedPrinciple && existing.Status != api.StatusUnspecified {
				res.PrinciplesSkipped++
				continue
			}
		} else if !errors.Is(err, store.ErrNotFound) {
			return res, fmt.Errorf("load principle %q: pre-check: %w", ps.ID, err)
		}
		p := &api.Principle{
			ID: ps.ID, Project: project, Domain: dom, Title: ps.Title,
			AppliesWhen:      toRefs[api.ConditionRef](ps.AppliesWhen),
			Authorities:      toRefs[api.AuthorityRef](ps.Authorities),
			RequiredEvidence: toRefs[api.RequiredEvidenceRef](ps.RequiredEvidence),
			ForbiddenMoves:   toRefs[api.ForbiddenMoveRef](ps.ForbiddenMoves),
			RecommendedAction: ps.RecommendedAction, RiskLevel: ps.RiskLevel,
			RevocationRule: ps.RevocationRule, PromotionReason: ps.PromotionReason,
			Status: api.StatusProposedPrinciple, Version: 1, ProposedBy: "seed:" + d.Name(),
			SourceRefs: ps.SourceRefs, GeneratedFrom: ps.GeneratedFrom,
			Provenance: api.Provenance{AgentID: "seed:" + d.Name()},
			Metadata:   seedMeta(d.Name()),
		}
		if err := st.CreatePrinciple(ctx, p); err != nil {
			return res, fmt.Errorf("load principle %q: %w", ps.ID, err)
		}
		res.PrinciplesSeeded++
	}
	return res, nil
}
