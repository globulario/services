// Package opsknowledge is the cluster_operator Operational Knowledge Compiler.
//
// It turns the structured operational-knowledge corpus (docs/operational-knowledge/:
// stages, runbooks, service-roles, incidents) into deterministic behavioral-memory
// seed objects — authority/condition/forbidden-move/required-evidence catalogs and
// PROPOSED principle candidates — emitted as generated YAML that the cluster_operator
// pack loads alongside its hand-authored seed.
//
// It is DOMAIN-SPECIFIC to cluster_operator and deterministic: same corpus → same
// output, stable ordering, stable human-readable ids, no timestamps, NO LLM. The
// generic behavioral kernel never imports this package.
//
// Extraction is conservative: it reads explicit STRUCTURED fields (file_kind,
// procedure phases, step warnings/descriptions, success_criteria, links) — it does
// not attempt free-form Markdown NLP.
package opsknowledge

import "github.com/globulario/services/golang/ai_memory/behavioral/domain"

// SourcePrefix namespaces stable source refs, e.g. "opsknowledge:runbook.ops.recover.foo".
const SourcePrefix = "opsknowledge:"

// Source is a provenance record for one corpus entry that fed the compiler.
type Source struct {
	Ref   string `yaml:"ref"`   // opsknowledge:<kind>.<id-or-slug>
	Kind  string `yaml:"kind"`  // service_role | stage | runbook | incident
	Path  string `yaml:"path"`  // repo-relative source path
	Hash  string `yaml:"hash"`  // deterministic content hash
	Title string `yaml:"title"`
}

// Bundle is the full deterministic compiler output. Catalog entries reuse the
// generic domain types so the pack can load them with no extra conversion.
type Bundle struct {
	Authorities      []domain.CatalogEntry
	Conditions       []domain.CatalogEntry
	ForbiddenMoves   []domain.CatalogEntry
	RequiredEvidence []domain.CatalogEntry
	Principles       []domain.PrincipleSeed
	Sources          []Source
}
