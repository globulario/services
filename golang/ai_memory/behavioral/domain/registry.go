package domain

// Registry holds the registered domain packs. The kernel uses it to resolve a
// DomainRef to its pack. PR-1 provides registration/lookup only; packs are
// registered by later PRs (cluster_operator first).
//
// Registry is not safe for concurrent registration; packs are expected to be
// registered once at startup before the service begins serving.
type Registry struct {
	domains map[string]Domain
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{domains: make(map[string]Domain)}
}

// Register adds a domain pack. A later registration for the same name replaces
// the earlier one.
func (r *Registry) Register(d Domain) {
	if d == nil {
		return
	}
	r.domains[d.Name()] = d
}

// Lookup returns the pack for name, and whether it was found.
func (r *Registry) Lookup(name string) (Domain, bool) {
	d, ok := r.domains[name]
	return d, ok
}

// Names returns the registered domain names (unordered).
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.domains))
	for n := range r.domains {
		names = append(names, n)
	}
	return names
}
