// @awareness namespace=globular.platform
// @awareness component=platform_binhash.canonical_binary_identity
// @awareness file_role=single_source_of_truth_for_on_disk_binary_sha256
// @awareness risk=critical
//
// Package binhash is the single canonical source for "what is the sha256
// of this binary on disk?" Every component that needs to compare binary
// identity MUST go through this package — no scattered sha256.Sum256
// calls, no per-package normalize helpers, no cached metadata trust.
//
// The user-visible failure that justified the centralization (2026-06-03):
// installed_state.metadata["entrypoint_checksum"] was cached at install
// time and never refreshed. The actual binary on disk could be swapped
// out (or fail to swap) while the cache claimed the new identity. Multiple
// controller paths trusted the cache → false-converged decisions →
// node-agent stuck on old bytes for hours. Same root cause manifested as:
//
//   - drift-reconciler comparing (version, buildId) only (Phase 37)
//   - release-workflow skip-node comparing version/hash/buildId/runtime only (Phase 38)
//   - cluster-services-drift hash stuck because InstalledVersions cached
//     a stale value (Phase 39 — this package)
//
// Contract:
//   - Hash(path) reads the file at `path` and returns lowercase hex
//     sha256 with NO "sha256:" prefix. No caching. Each call hits disk.
//   - Normalize(s) returns the canonical form of any input
//     ("SHA256:ABCdef" → "abcdef") so any comparison is robust to
//     formatting variance from manifests / package.json / etcd / log
//     lines / operator typing.
//   - Equal(a, b) normalizes both sides, then compares.
//   - Short(s) returns the first 16 hex chars of the normalized form
//     for log-friendly display.
package binhash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Hash returns the sha256 of the file at `path` as lowercase hex
// with no "sha256:" prefix. Each call reads the file fresh — there is
// no caching. Callers that need binary identity for a convergence
// decision MUST use this, not a cached field on installed_state /
// manifest / package.json.
func Hash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashOrEmpty is the same as Hash but swallows errors and returns
// "" — convenient for paths where missing-file/permission-denied
// should be treated as "no proof available" (e.g. inventory reports
// that may legitimately encounter packages with no entrypoint file).
//
// Non-NotExist errors (e.g. EPERM) are logged at Debug level so that
// permission problems are visible when debug logging is enabled, while
// remaining silent in normal operation where missing files are expected.
func HashOrEmpty(path string) string {
	h, err := Hash(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("binhash.HashOrEmpty: non-transient error reading file", "path", path, "err", err)
		}
		return ""
	}
	return h
}

// Normalize canonicalizes a checksum string. Strips optional "sha256:"
// prefix, trims whitespace, lowercases. Result: bare lowercase hex.
// Empty input → empty output.
func Normalize(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	return strings.TrimPrefix(v, "sha256:")
}

// Equal returns true iff Normalize(a) == Normalize(b). Empty is never
// equal to non-empty — callers that want "no opinion when missing"
// must check for empty themselves.
func Equal(a, b string) bool {
	na, nb := Normalize(a), Normalize(b)
	return na != "" && nb != "" && na == nb
}

// Short returns the first 16 hex chars of the normalized form for
// log-friendly display. Sha256 collision probability at 8+ chars is
// negligible for log identification purposes.
func Short(s string) string {
	n := Normalize(s)
	if len(n) > 16 {
		return n[:16]
	}
	return n
}

// EntrypointPath returns the canonical on-disk path for a service's
// entrypoint binary on this node. Globular's install convention is
// /usr/lib/globular/bin/<entrypoint>. Empty entrypoint → empty path
// (caller should treat as "no opinion").
func EntrypointPath(entrypoint string) string {
	e := strings.TrimSpace(entrypoint)
	if e == "" {
		return ""
	}
	// `entrypoint` from package.json is typically "bin/node_agent_server"
	// (a relative path within the artifact). We only care about the
	// basename for the install location.
	if idx := strings.LastIndex(e, "/"); idx >= 0 {
		e = e[idx+1:]
	}
	return "/usr/lib/globular/bin/" + e
}

// ResolveServiceBinaryPath finds the on-disk path of an installed
// service's primary binary, given the canonical service name as it
// appears in installed_state (e.g. "cluster-controller", "node-agent",
// "scylladb") and an optional hint pulled from the InstalledPackage
// metadata (typically the `proof_binary_path` written by the install
// path).
//
// Resolution order:
//  1. metaHint, if non-empty AND the file exists.
//  2. /usr/lib/globular/bin/<canon_with_underscores>_server  (the
//     Go-service convention: "cluster-controller" -> "cluster_controller_server").
//  3. /usr/lib/globular/bin/<canon_with_underscores>          (CLI-only tools).
//  4. /usr/lib/globular/bin/<canon>                           (rare, fallback).
//
// Returns "" if nothing matches — Phase 39 callers treat empty as "no
// opinion" and let the original behaviour apply (the typical case for
// wrapper INFRASTRUCTURE packages like scylladb / minio / keepalived
// that do not own a binary in /usr/lib/globular/bin/).
func ResolveServiceBinaryPath(canon, metaHint string) string {
	if h := strings.TrimSpace(metaHint); h != "" {
		if _, err := os.Stat(h); err == nil {
			return h
		}
	}
	c := strings.TrimSpace(canon)
	if c == "" {
		return ""
	}
	u := strings.ReplaceAll(c, "-", "_")
	for _, candidate := range []string{
		"/usr/lib/globular/bin/" + u + "_server",
		"/usr/lib/globular/bin/" + u,
		"/usr/lib/globular/bin/" + c,
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
