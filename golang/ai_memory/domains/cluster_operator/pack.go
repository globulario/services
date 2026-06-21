// Package cluster_operator is the first behavioral-memory domain pack. It supplies
// cluster-operation authorities, conditions, forbidden moves, required evidence,
// and seed principles as generic domain.Catalogs — turning the generic kernel
// into a cluster operator while keeping all cluster specifics OUT of the kernel.
//
// The generic kernel (behavioral/api, behavioral/core) must never import this
// package; this package depends on the kernel, not the reverse. Cluster/probe
// clients, if ever needed, live only in evidence_probes.go.
package cluster_operator

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"gopkg.in/yaml.v3"
)

// DomainName is the api.DomainRef value carried by every cluster_operator record.
const DomainName = "cluster_operator"

//go:embed seed/*.yaml
var seedFS embed.FS

// generatedFS holds the Operational Knowledge Compiler output (PR-5A). These are
// build artifacts (DO NOT EDIT); regenerate with the opscompile command. The pack
// loads them ALONGSIDE the hand-authored seed and validates the combined set.
//
//go:embed generated/*.generated.yaml
var generatedFS embed.FS

// Pack is the cluster_operator domain pack. It parses and validates its embedded
// seed at construction; New returns an error if the seed is malformed or any
// principle references an unknown catalog id.
type Pack struct {
	catalogs domain.Catalogs
}

var _ domain.Domain = (*Pack)(nil)

// Name implements domain.Domain.
func (p *Pack) Name() string { return DomainName }

// Catalogs implements domain.Domain.
func (p *Pack) Catalogs() domain.Catalogs { return p.catalogs }

// yaml shapes ----------------------------------------------------------------

type yamlPrinciple struct {
	ID                string   `yaml:"id"`
	Title             string   `yaml:"title"`
	AppliesWhen       []string `yaml:"applies_when"`
	Authorities       []string `yaml:"authorities"`
	RequiredEvidence  []string `yaml:"required_evidence"`
	ForbiddenMoves    []string `yaml:"forbidden_moves"`
	RecommendedAction string   `yaml:"recommended_action"`
	RiskLevel         string   `yaml:"risk_level"`
	RevocationRule    string   `yaml:"revocation_rule"`
	PromotionReason   string   `yaml:"promotion_reason"`
	SourceRefs        []string `yaml:"source_refs"`
	GeneratedFrom     []string `yaml:"generated_from"`
}

// parseEntries parses an entry-catalog file whose values are all scalars into
// generic CatalogEntry rows (id + title + Fields bag). A missing file yields no
// entries (a generated catalog may be absent if the corpus produced none).
func parseEntries(fsys fs.FS, path string) ([]domain.CatalogEntry, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		if _, isPathErr := err.(*fs.PathError); isPathErr {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var raw []map[string]string
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make([]domain.CatalogEntry, 0, len(raw))
	for i, m := range raw {
		id := m["id"]
		if id == "" {
			return nil, fmt.Errorf("%s entry %d: missing id", path, i)
		}
		fields := make(map[string]string, len(m))
		for k, v := range m {
			if k == "id" || k == "title" {
				continue
			}
			fields[k] = v
		}
		out = append(out, domain.CatalogEntry{ID: id, Title: m["title"], Fields: fields})
	}
	return out, nil
}

// parsePrinciples parses a principle-seed file into generic PrincipleSeed rows.
func parsePrinciples(fsys fs.FS, path string) ([]domain.PrincipleSeed, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		if _, isPathErr := err.(*fs.PathError); isPathErr {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var yps []yamlPrinciple
	if err := yaml.Unmarshal(data, &yps); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make([]domain.PrincipleSeed, 0, len(yps))
	for _, yp := range yps {
		if yp.ID == "" {
			return nil, fmt.Errorf("%s: principle with empty id", path)
		}
		out = append(out, domain.PrincipleSeed{
			ID: yp.ID, Title: yp.Title, AppliesWhen: yp.AppliesWhen, Authorities: yp.Authorities,
			RequiredEvidence: yp.RequiredEvidence, ForbiddenMoves: yp.ForbiddenMoves,
			RecommendedAction: yp.RecommendedAction, RiskLevel: yp.RiskLevel,
			RevocationRule: yp.RevocationRule, PromotionReason: yp.PromotionReason,
			SourceRefs: yp.SourceRefs, GeneratedFrom: yp.GeneratedFrom,
		})
	}
	return out, nil
}

// New loads + merges the hand-authored seed (PR-5) and the compiler-generated
// catalogs (PR-5A), validates the combined set as one, and returns the pack.
// validates the combined set, and returns the pack.
func New() (*Pack, error) {
	cats := domain.Catalogs{}

	specs := []struct {
		dst  *[]domain.CatalogEntry
		seed string
		gen  string
	}{
		{&cats.Authorities, "seed/authorities.yaml", "generated/authorities.generated.yaml"},
		{&cats.Conditions, "seed/conditions.yaml", "generated/conditions.generated.yaml"},
		{&cats.ForbiddenMoves, "seed/forbidden_moves.yaml", "generated/forbidden_moves.generated.yaml"},
		{&cats.RequiredEvidence, "seed/required_evidence.yaml", "generated/required_evidence.generated.yaml"},
	}
	for _, s := range specs {
		hand, err := parseEntries(seedFS, s.seed)
		if err != nil {
			return nil, err
		}
		gen, err := parseEntries(generatedFS, s.gen)
		if err != nil {
			return nil, err
		}
		*s.dst = append(append([]domain.CatalogEntry{}, hand...), gen...)
	}

	handP, err := parsePrinciples(seedFS, "seed/principles.seed.yaml")
	if err != nil {
		return nil, err
	}
	genP, err := parsePrinciples(generatedFS, "generated/principles.generated.yaml")
	if err != nil {
		return nil, err
	}
	cats.Principles = append(append([]domain.PrincipleSeed{}, handP...), genP...)

	p := &Pack{catalogs: cats}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// MustNew is New that panics on error — for package-level wiring where a bad
// embedded seed is a build-time bug.
func MustNew() *Pack {
	p, err := New()
	if err != nil {
		panic("cluster_operator: invalid seed: " + err.Error())
	}
	return p
}

func idSet(entries []domain.CatalogEntry) (map[string]bool, error) {
	s := make(map[string]bool, len(entries))
	for _, e := range entries {
		if s[e.ID] {
			return nil, fmt.Errorf("duplicate catalog id %q", e.ID)
		}
		s[e.ID] = true
	}
	return s, nil
}

// validate enforces: unique ids, every principle ref resolves within the pack,
// and every forbidden move carries a generative pairing.
func (p *Pack) validate() error {
	auth, err := idSet(p.catalogs.Authorities)
	if err != nil {
		return fmt.Errorf("authorities: %w", err)
	}
	cond, err := idSet(p.catalogs.Conditions)
	if err != nil {
		return fmt.Errorf("conditions: %w", err)
	}
	forb, err := idSet(p.catalogs.ForbiddenMoves)
	if err != nil {
		return fmt.Errorf("forbidden_moves: %w", err)
	}
	reqEv, err := idSet(p.catalogs.RequiredEvidence)
	if err != nil {
		return fmt.Errorf("required_evidence: %w", err)
	}

	// Generative-pairing rule: every forbidden move must offer a constructive
	// behavior (recommended_behavior / safe_next_step / required_evidence).
	for _, fm := range p.catalogs.ForbiddenMoves {
		if fm.Fields["recommended_behavior"] == "" && fm.Fields["safe_next_step"] == "" && fm.Fields["required_evidence"] == "" {
			return fmt.Errorf("forbidden move %q has no paired generative behavior", fm.ID)
		}
	}

	seenP := map[string]bool{}
	for _, ps := range p.catalogs.Principles {
		if seenP[ps.ID] {
			return fmt.Errorf("duplicate principle id %q", ps.ID)
		}
		seenP[ps.ID] = true
		for _, r := range ps.AppliesWhen {
			if !cond[r] {
				return fmt.Errorf("principle %q references unknown condition %q", ps.ID, r)
			}
		}
		for _, r := range ps.Authorities {
			if !auth[r] {
				return fmt.Errorf("principle %q references unknown authority %q", ps.ID, r)
			}
		}
		for _, r := range ps.RequiredEvidence {
			if !reqEv[r] {
				return fmt.Errorf("principle %q references unknown required evidence %q", ps.ID, r)
			}
		}
		for _, r := range ps.ForbiddenMoves {
			if !forb[r] {
				return fmt.Errorf("principle %q references unknown forbidden move %q", ps.ID, r)
			}
		}
	}
	return nil
}

// sortedIDs returns the sorted ids of an entry catalog (deterministic).
func sortedIDs(entries []domain.CatalogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	sort.Strings(out)
	return out
}
