package schema_reference

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

//go:embed schema.json
var embeddedSchemaJSON []byte

// Registry is the runtime lookup surface over schema entries. It is
// cheap to construct and safe to reuse — all reads are lock-free after
// the first load. The embedded JSON is loaded lazily on first use.
type Registry struct {
	mu      sync.Mutex
	loaded  bool
	result  ExtractResult
	byKey   map[string]*Entry
	byType  map[string]*Entry
}

// DefaultRegistry returns a Registry backed by the JSON that was
// embedded at build time. Most callers want this.
func DefaultRegistry() *Registry {
	return &Registry{}
}

// NewRegistryFromJSON constructs a Registry from raw JSON — used by
// tests and by tools that want to load a file other than the embed.
func NewRegistryFromJSON(data []byte) (*Registry, error) {
	var res ExtractResult
	if len(data) > 0 {
		if err := json.Unmarshal(data, &res); err != nil {
			return nil, err
		}
	}
	r := &Registry{loaded: true, result: res}
	r.buildIndexes()
	return r, nil
}

func (r *Registry) ensureLoaded() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded {
		return
	}
	if len(embeddedSchemaJSON) > 0 {
		_ = json.Unmarshal(embeddedSchemaJSON, &r.result)
	}
	r.buildIndexes()
	r.loaded = true
}

func (r *Registry) buildIndexes() {
	r.byKey = make(map[string]*Entry, len(r.result.Entries))
	r.byType = make(map[string]*Entry, len(r.result.Entries))
	for i := range r.result.Entries {
		e := &r.result.Entries[i]
		r.byKey[e.KeyPattern] = e
		if e.TypeName != "" {
			r.byType[strings.ToLower(e.TypeName)] = e
		}
	}
}

// Entries returns the full registry contents, sorted by KeyPattern.
// Callers MUST NOT mutate the returned slice.
func (r *Registry) Entries() []Entry {
	r.ensureLoaded()
	return r.result.Entries
}

// Source / GeneratedAtUnix mirror the ExtractResult fields so callers
// can stamp freshness on their responses (clause 4) without reaching
// into the registry internals.
func (r *Registry) Source() string { r.ensureLoaded(); return r.result.Source }
func (r *Registry) GeneratedAtUnix() int64 {
	r.ensureLoaded()
	return r.result.GeneratedAtUnix
}

// LookupByKey returns the entry whose KeyPattern is an exact match for
// `pattern`. Nil if not found.
func (r *Registry) LookupByKey(pattern string) *Entry {
	r.ensureLoaded()
	return r.byKey[pattern]
}

// LookupByType returns the entry for a Go type name, case-insensitive.
// Nil if not found.
func (r *Registry) LookupByType(typeName string) *Entry {
	r.ensureLoaded()
	return r.byType[strings.ToLower(typeName)]
}

// Search returns every entry whose KeyPattern, TypeName, or Description
// contains `q` (case-insensitive substring). Useful when a caller has
// a fragment of an etcd key or a fuzzy memory of a struct name. Sorted
// by KeyPattern.
func (r *Registry) Search(q string) []Entry {
	r.ensureLoaded()
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []Entry
	for _, e := range r.result.Entries {
		if strings.Contains(strings.ToLower(e.KeyPattern), q) ||
			strings.Contains(strings.ToLower(e.TypeName), q) ||
			strings.Contains(strings.ToLower(e.Description), q) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].KeyPattern < out[j].KeyPattern })
	return out
}
