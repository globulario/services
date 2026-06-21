// Package domain defines the pluggable domain-pack boundary for the behavioral-
// memory kernel and a registry to hold packs.
//
// A domain pack (e.g. cluster_operator) supplies the catalogs of authorities,
// conditions, and forbidden moves for one domain, and knows how to gather the
// runtime evidence its principles require. The generic kernel resolves opaque
// refs (ConditionRef, AuthorityRef, …) through registered domains; it never
// hardcodes cluster concepts.
//
// PR-1 defines the boundary only. The cluster_operator implementation lands in a
// later PR under domains/cluster_operator/ — NOT under behavioral/, to keep the
// kernel generic.
package domain

// Domain is a pluggable knowledge pack. It supplies the generic catalogs
// (authorities, conditions, forbidden moves, required evidence) and seed
// principles for one domain. The kernel resolves opaque refs against these
// catalogs (and the store) — it never hardcodes a domain's meaning.
type Domain interface {
	// Name returns the domain identifier, e.g. "cluster_operator". It is the
	// value carried by api.DomainRef on governed records.
	Name() string
	// Catalogs returns the pack's generic catalog set (validated by the pack).
	Catalogs() Catalogs
}
