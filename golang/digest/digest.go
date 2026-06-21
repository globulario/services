// Package digest defines the one canonical comparison form for SHA-256 digest
// strings used across Globular.
//
// Two owner surfaces speak different digest dialects: the repository manifest
// exposes "sha256:<hex>", while node-agent / runtime surfaces expose bare
// "<hex>". Comparing the two verbatim produces false drift on byte-identical
// binaries. This package is the single source of truth for the contract:
//
//	Digest identity must have one canonical comparison form.
//
// It is pure: trim + lowercase + strip an optional "sha256:" prefix. It does no
// hashing and no I/O, so any package may import it without a cycle. It does not
// mutate stored data — equality is canonicalized only at comparison
// boundaries; external storage and display formats are unchanged.
package digest

import "strings"

const sha256Prefix = "sha256:"

// CanonicalSHA256 returns the bare comparison form of a SHA-256 digest string:
// trimmed, lowercased, with an optional "sha256:" prefix stripped. Lowercasing
// happens before prefix stripping, so an uppercase "SHA256:" prefix is also
// removed. Empty input returns "". It performs no hashing and no I/O.
func CanonicalSHA256(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimPrefix(s, sha256Prefix)
}

// EqualSHA256 reports whether two SHA-256 digest strings identify the same
// digest despite differences in whitespace, case, or an optional "sha256:"
// prefix.
func EqualSHA256(a, b string) bool {
	return CanonicalSHA256(a) == CanonicalSHA256(b)
}
