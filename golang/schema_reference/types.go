// Package schema_reference is the Phase 4a schema-discovery surface.
//
// It answers the question operators and AI agents keep hitting head-first:
// *"What does this etcd key mean and who owns it?"* Answers are extracted
// from `+globular:schema:` pragmas on Go types (see docs/schema_pragmas.md)
// so the authoritative source is the code itself — there is no separate
// registry to drift out of sync.
//
// Runtime lookup is O(n) over a small in-memory slice built from a JSON
// file embedded at build time. No ScyllaDB, no etcd, no cache. That keeps
// the surface boring and the projection-clauses happy:
//
//   - Clause 1 (Single Source of Truth): pragmas in Go code.
//   - Clause 2 (Minimal Surface): one question, flat entries.
//   - Clause 3 (Reader-Fallback): if the embed is empty or stale, the
//     caller can re-run the extractor.
//   - Clause 4 (Freshness): the registry stamps generated_at/source on
//     every response.
//   - Clause 5 (Scoped Query): every lookup requires a name or pattern.
//   - Clause 11 (AI Consumption): flat fields, explicit meaning.
package schema_reference

// Entry is one schema pragma block. All fields except Key and Writer are
// optional. The shape is intentionally flat — no nesting — so callers can
// render it verbatim without walking structure.
//
// The extractor produces one Entry per Go type carrying a
// `+globular:schema:key=...` pragma. A type with no key pragma is skipped
// entirely; a type with a key pragma but no writer pragma is a schema-lint
// violation (enforced at extractor-run time).
type Entry struct {
	// KeyPattern is the etcd/ScyllaDB key template this schema describes.
	// Interpolation slots use `{name}` notation so operators can grep the
	// prefix without regex. Example:
	//   "/globular/resources/ServiceDesiredVersion/{name}"
	KeyPattern string `json:"key_pattern"`

	// Writer is the single service that MAY write this key. Schema
	// hygiene depends on single-writer ownership (clause 1): if two
	// services wrote to the same key, drift is impossible to reason
	// about. The extractor fails if this is missing.
	Writer string `json:"writer"`

	// Readers is the set of services that read this key. Optional, but
	// strongly encouraged — it tells operators which subsystems will
	// notice a change when they edit the value.
	Readers []string `json:"readers,omitempty"`

	// Description is a one-sentence human summary. Taken from the
	// `+globular:schema:description=` pragma, OR from the first line of
	// the type's doc comment when the pragma is absent.
	Description string `json:"description,omitempty"`

	// Invariants is a free-text field describing constraints that MUST
	// hold for this key's value. Kept as a single string so the
	// rendering is stable; use semicolons for multiple invariants.
	Invariants string `json:"invariants,omitempty"`

	// SinceVersion is the Globular version in which this key was added.
	// Optional — if present, lets operators reason about migration.
	SinceVersion string `json:"since_version,omitempty"`

	// TypeName is the Go type that carries this pragma. Useful for
	// developers jumping from the schema reference back to the code.
	TypeName string `json:"type_name,omitempty"`

	// SourceFile / SourceLine give a `file:line` pointer back to the
	// pragma. The extractor sets these; never hand-edit them in the
	// generated JSON.
	SourceFile string `json:"source_file,omitempty"`
	SourceLine int    `json:"source_line,omitempty"`
}
