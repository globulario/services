package cluster_operator

import "github.com/globulario/services/golang/ai_memory/behavioral/domain"

// ForbiddenMoveIDs returns the sorted forbidden-move catalog ids.
func (p *Pack) ForbiddenMoveIDs() []string { return sortedIDs(p.catalogs.ForbiddenMoves) }

// GenerativeBehavior returns the constructive behavior paired with a forbidden
// move (the safe thing to prefer). Every forbidden move has one — validated at
// construction. Returns "" for an unknown ref.
func (p *Pack) GenerativeBehavior(forbiddenMoveID string) string {
	for _, fm := range p.catalogs.ForbiddenMoves {
		if fm.ID == forbiddenMoveID {
			if b := fm.Fields["recommended_behavior"]; b != "" {
				return b
			}
			return fm.Fields["safe_next_step"]
		}
	}
	return ""
}

// ForbiddenMoves returns the forbidden-move catalog entries.
func (p *Pack) ForbiddenMoves() []domain.CatalogEntry { return p.catalogs.ForbiddenMoves }
