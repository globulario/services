package domain

// loader.go writes a domain pack's catalogs into the behavioral-memory store.
// It is GENERIC: it maps generic CatalogEntry / PrincipleSeed into api types and
// persists them via the store port. It contains no cluster knowledge and no
// driver code.
//
// Loading is idempotent and NON-DESTRUCTIVE: re-running re-writes catalog rows
// (same data) and re-proposes seed principles only while they are still merely
// proposed — a seed principle that has since been promoted or revoked is left
// semantically untouched, so load never silently demotes or imports arbitrary
// new governed knowledge.
//
// One narrow migration exception exists for already-promoted seed-backed
// principles: if a newer seed adds condition.always, the loader canonically adds
// that technical sentinel plus an explicit migration marker to the promoted row,
// then repairs the active condition index. No other new seed field/condition is
// adopted. Keeping the sentinel on the canonical principle means explanation and
// later revocation observe exactly the same reach as runtime lookup.
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
	Authorities                 int
	Conditions                  int
	PrinciplesSeeded            int
	PrinciplesSkipped           int // already promoted/revoked — left as governed state
	PromotedPrinciplesReindexed int // seed-backed promoted rows migrated to canonical condition.always reach
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

func cloneMetadata(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func containsString(in []string, want string) bool {
	for _, v := range in {
		if v == want {
			return true
		}
	}
	return false
}

func seedBackedByDomain(p *api.Principle, domainName string) bool {
	return p != nil && p.Metadata != nil &&
		p.Metadata["source"] == "seed" && p.Metadata["domain_pack"] == domainName
}

// migratePromotedAlwaysReach repairs one upgrade-only seam. An older seed-backed
// principle may already be PROMOTED before its domain pack gains condition.always.
// Because active runtime reach lives in the principles_by_condition index, the
// old row would otherwise remain invisible to universal forbidden-action matching.
//
// This is deliberately not a general seed refresh:
//   - only already-PROMOTED, seed-backed principles are eligible;
//   - only the technical condition.always sentinel may be copied from the seed;
//   - authorities, evidence, forbidden moves, approval, risk and status stay put;
//   - no other newly-authored seed condition/ref/field is adopted.
//
// The sentinel and migration marker are persisted on the canonical principle
// before indexing. ExplainPrinciple therefore reports the migrated reach, and
// RevokePrinciple later deindexes it through the ordinary canonical AppliesWhen.
func migratePromotedAlwaysReach(ctx context.Context, st store.Store, domainName string, existing *api.Principle, seed PrincipleSeed) (bool, error) {
	if existing == nil || existing.Status != api.StatusPromotedPrinciple {
		return false, nil
	}
	if !seedBackedByDomain(existing, domainName) {
		return false, nil
	}
	if !containsString(seed.AppliesWhen, AlwaysConditionID) {
		return false, nil
	}
	always := api.ConditionRef(AlwaysConditionID)
	if principleDeclaresCondition(existing, always) {
		return false, nil
	}

	migrated := *existing
	migrated.AppliesWhen = append(append([]api.ConditionRef(nil), existing.AppliesWhen...), always)
	migrated.Metadata = cloneMetadata(existing.Metadata)
	migrated.Metadata[RuntimeAlwaysReachMetadataKey] = RuntimeAlwaysReachSeedMigration
	if err := st.CreatePrinciple(ctx, &migrated); err != nil {
		return false, fmt.Errorf("persist runtime reach migration: %w", err)
	}
	if err := st.IndexPromotedPrinciple(ctx, &migrated); err != nil {
		return false, fmt.Errorf("index runtime reach migration: %w", err)
	}
	return true, nil
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
				if migrated, err := migratePromotedAlwaysReach(ctx, st, d.Name(), existing, ps); err != nil {
					return res, fmt.Errorf("load principle %q: migrate always reach: %w", ps.ID, err)
				} else if migrated {
					res.PromotedPrinciplesReindexed++
				}
				res.PrinciplesSkipped++
				continue
			}
		} else if !errors.Is(err, store.ErrNotFound) {
			return res, fmt.Errorf("load principle %q: pre-check: %w", ps.ID, err)
		}
		p := PrincipleFromSeed(project, d, ps)
		if err := st.CreatePrinciple(ctx, &p); err != nil {
			return res, fmt.Errorf("load principle %q: %w", ps.ID, err)
		}
		res.PrinciplesSeeded++
	}
	return res, nil
}
