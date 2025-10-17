package persistence_store

import "strings"

// canonicalize a pair of entity/table names (lowercased, alphabetical)
func canonicalPair(a, b string) (string, string) {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	if a <= b {
		return a, b
	}
	return b, a
}

// Given a base table (e.g. "accounts") and a field (e.g. "organizations"),
// return the canonical link table name and whether base is first in that name.
func canonicalRefTable(base, field string) (table string, baseIsFirst bool) {
	lbase := strings.ToLower(base)
	lf := strings.ToLower(field)
	if !strings.HasSuffix(lbase, "s") {
		lbase += "s"
	}
	if !strings.HasSuffix(lf, "s") {
		lf += "s"
	}
	left, right := canonicalPair(lbase, lf)
	return left + "_" + right, left == lbase
}