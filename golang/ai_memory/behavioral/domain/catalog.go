package domain

// catalog.go defines the GENERIC domain-pack catalog model. It carries no
// cluster-specific fields — a CatalogEntry is just an id, a title, and an opaque
// string-keyed Fields bag. A concrete pack (e.g. cluster_operator) populates
// these from its own seed data; the kernel only ever sees generic shapes.

// CatalogEntry is a generic domain catalog item (authority, condition,
// forbidden-move, or required-evidence). Fields holds entry-kind-specific extras
// (e.g. owner_kind, severity, recommended_behavior) without typing them here.
type CatalogEntry struct {
	ID     string
	Title  string
	Fields map[string]string
}

// PrincipleSeed is a generic proposed-principle seed. Refs are opaque strings
// resolved within the owning pack's catalogs.
type PrincipleSeed struct {
	ID                string
	Title             string
	AppliesWhen       []string
	Authorities       []string
	RequiredEvidence  []string
	ForbiddenMoves    []string
	RecommendedAction string
	RiskLevel         string
	RevocationRule    string
	PromotionReason   string
	SourceRefs        []string
	GeneratedFrom     []string
}

// Catalogs is a domain pack's full catalog set.
type Catalogs struct {
	Authorities      []CatalogEntry
	Conditions       []CatalogEntry
	ForbiddenMoves   []CatalogEntry
	RequiredEvidence []CatalogEntry
	Principles       []PrincipleSeed
}
