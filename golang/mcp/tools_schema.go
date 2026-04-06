package main

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/schema_reference"
)

// registerSchemaTools wires the Phase 4a schema-discovery tools into the
// MCP server. The registry is backed by an embedded JSON file generated
// by golang/tools/schema-extractor, so these tools work even with the
// controller / etcd / scylla fully offline.
func registerSchemaTools(s *server) {
	reg := schema_reference.DefaultRegistry()

	// ── schema_describe ──────────────────────────────────────────────
	// Scoped (Clause 5): requires a query. Flat (Clause 11): returns
	// the entry fields verbatim. Freshness (Clause 4): every response
	// carries source + generated_at from the extractor run.
	s.register(toolDef{
		Name: "schema_describe",
		Description: "Looks up Globular's etcd/scylla key schema by key pattern, Go type name, or substring. Answers 'what does this key mean, who writes it, who reads it?'. The schema is extracted from +globular:schema: pragmas on Go types — the code IS the source, the MCP tool just reads an embedded index of it. Chain to etcd_get when you need the actual value under a resolved key.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query": {
					Type:        "string",
					Description: "Exact key pattern, Go type name, or substring. Examples: '/globular/resources/ServiceDesiredVersion/{name}', 'InfrastructureRelease', 'desired'.",
				},
			},
			Required: []string{"query"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		q, _ := args["query"].(string)
		if q == "" {
			return nil, fmt.Errorf("query is required")
		}
		// Exact match first (key pattern, then type name), then fall
		// back to substring search. This keeps common lookups cheap
		// and deterministic.
		if e := reg.LookupByKey(q); e != nil {
			return schemaEntryPayload(reg, []schema_reference.Entry{*e}, "exact_key"), nil
		}
		if e := reg.LookupByType(q); e != nil {
			return schemaEntryPayload(reg, []schema_reference.Entry{*e}, "exact_type"), nil
		}
		hits := reg.Search(q)
		if len(hits) == 0 {
			return nil, fmt.Errorf("no schema entry matches %q", q)
		}
		return schemaEntryPayload(reg, hits, "substring"), nil
	})

	// ── schema_list ──────────────────────────────────────────────────
	// Full enumeration (bounded — the registry caps out at a few dozen
	// entries so this never token-explodes). Used when an operator or
	// AI wants to browse the whole surface.
	s.register(toolDef{
		Name:        "schema_list",
		Description: "Returns every known Globular key-schema entry (all etcd/scylla keys with +globular:schema: pragmas). Small, bounded, stable. Use this to discover the surface before diving into schema_describe.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return schemaEntryPayload(reg, reg.Entries(), "all"), nil
	})
}

// schemaEntryPayload stamps freshness + match shape onto the entries
// list so AI callers can decide whether to trust the result. The
// `source` and `generated_at` come straight from the extractor run;
// `match_kind` tells the caller how the query resolved.
func schemaEntryPayload(reg *schema_reference.Registry, entries []schema_reference.Entry, matchKind string) map[string]interface{} {
	rendered := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		rendered = append(rendered, map[string]interface{}{
			"key_pattern":   e.KeyPattern,
			"writer":        e.Writer,
			"readers":       e.Readers,
			"description":   e.Description,
			"invariants":    e.Invariants,
			"type_name":     e.TypeName,
			"since_version": e.SinceVersion,
			"source_file":   e.SourceFile,
			"source_line":   e.SourceLine,
		})
	}
	return map[string]interface{}{
		"count":        len(rendered),
		"entries":      rendered,
		"match_kind":   matchKind,
		"source":       reg.Source(),
		"generated_at": reg.GeneratedAtUnix(),
	}
}
