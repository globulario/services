package cluster_operator

import "github.com/globulario/services/golang/ai_memory/behavioral/domain"

// PrincipleSeeds returns the pack's seed principles (authored PROPOSED; promotion
// stays behind the gate).
func (p *Pack) PrincipleSeeds() []domain.PrincipleSeed { return p.catalogs.Principles }

// PrincipleIDs returns the seed principle ids in declaration order.
func (p *Pack) PrincipleIDs() []string {
	out := make([]string, len(p.catalogs.Principles))
	for i, ps := range p.catalogs.Principles {
		out[i] = ps.ID
	}
	return out
}
