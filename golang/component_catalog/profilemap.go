// Package component_catalog exposes a minimal, dependency-free view of the
// cluster's profile→package mapping so node-agent (Day-0) can derive its
// install set from a node's profiles instead of from a hardcoded YAML list.
//
// The rich Component catalog (with capabilities, runtime deps, install
// modes, health checks, etc.) lives in cluster_controller_server because
// it depends on repository_client and other server-internal types and
// must run as part of the controller. The node-agent does not have that
// catalog at Day-0 time — the controller isn't installed yet.
//
// What this package provides is the architectural floor: a hand-curated
// table that lists, for each profile, the canonical package names that
// belong to it. The cluster-controller validates this table against its
// own catalog at startup (see profilemap_consistency_test.go in
// cluster_controller_server) so drift between the two surfaces is caught
// before a release ships.
//
// Invariants:
//  1. Every profile listed in ProfilePackages must have at least one
//     package — "a profile with no services is not a profile."
//  2. ProfileNames() and the catalog's ProfileCapabilities must agree.
//  3. PackagesForProfiles is order-stable and deduplicates.
package component_catalog

import (
	"sort"
	"strings"
)

// ProfileInheritance defines implicit profile expansion rules.
// A control-plane node is always a core node; same for compute.
var ProfileInheritance = map[string][]string{
	"control-plane": {"core"},
	"compute":       {"core"},
	"media-server":  {"core"},
}

// ProfilePackages maps each profile to the set of package names that
// belong to it. Generated from the controller's catalog (every Component
// whose Profiles list contains the profile, regardless of Kind — services,
// infrastructure, and CLI tools all participate). Keep this sorted and
// deduplicated within each list so the table is reviewable by humans and
// stable for diffs.
//
// To regenerate after adding/removing components in the controller's
// catalog, run the consistency test in cluster_controller_server — it
// will print the expected table.
var ProfilePackages = map[string][]string{
	"compute": {
		"ai-executor",
		"ai-memory",
		"ai-router",
		"ai-watcher",
		"alertmanager",
		"authentication",
		"blog",
		"catalog",
		"claude",
		"codex",
		"conversation",
		"dns",
		"echo",
		"etcd",
		"etcdctl",
		"event",
		"file",
		"globular-cli",
		"ldap",
		"log",
		"mail",
		"mc",
		"minio",
		"monitoring",
		"node-agent",
		"node-exporter",
		"persistence",
		"prometheus",
		"rbac",
		"rclone",
		"repository",
		"restic",
		"sctool",
		"scylla-manager",
		"scylla-manager-agent",
		"search",
		"sha256sum",
		"sidekick",
		"sql",
		"workflow",
	},
	"control-plane": {
		"ai-executor",
		"ai-memory",
		"ai-watcher",
		"alertmanager",
		"backup-manager",
		"cluster-controller",
		"cluster-doctor",
		"dns",
		"envoy",
		"etcd",
		"etcdctl",
		"gateway",
		"keepalived",
		"mcp",
		"minio",
		"monitoring",
		"node-exporter",
		"prometheus",
		"resource",
		"sctool",
		"scylla-manager",
		"scylla-manager-agent",
		"scylladb",
		"workflow",
		"xds",
	},
	"core": {
		"ai-executor",
		"ai-memory",
		"ai-router",
		"ai-watcher",
		"alertmanager",
		"authentication",
		"claude",
		"codex",
		"dns",
		"etcd",
		"etcdctl",
		"event",
		"file",
		"globular-cli",
		"log",
		"mc",
		"minio",
		"monitoring",
		"node-agent",
		"node-exporter",
		"persistence",
		"prometheus",
		"rbac",
		"rclone",
		"repository",
		"restic",
		"sctool",
		"scylla-manager",
		"scylla-manager-agent",
		"search",
		"sha256sum",
		"sidekick",
		"workflow",
	},
	"database": {
		"ai-executor",
		"ai-memory",
		"ai-watcher",
		"scylladb",
		"workflow",
	},
	"dns": {
		"dns",
	},
	"gateway": {
		"envoy",
		"gateway",
		"keepalived",
		"xds",
	},
	"media-server": {
		"ffmpeg",
		"media",
		"title",
		"torrent",
		"yt-dlp",
	},
	"scylla": {
		"ai-executor",
		"ai-memory",
		"ai-watcher",
		"scylladb",
		"workflow",
	},
	"storage": {
		"file",
		"mc",
		"minio",
		"rclone",
		"restic",
		"scylladb",
		"sidekick",
		"storage",
	},
}

// ProfileNames returns the sorted list of all known profiles.
func ProfileNames() []string {
	out := make([]string, 0, len(ProfilePackages))
	for name := range ProfilePackages {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// PackagesForProfiles returns the union of packages claimed by any of the
// given profiles, sorted and deduplicated. Empty profile names and
// whitespace are trimmed; unknown profiles are silently ignored on the
// caller side, but the returned set is always a strict subset of the
// validated profile map.
//
// Returning an empty slice when no profile matches is intentional — the
// caller must decide whether that is an error in their context (Day-0
// bootstrap should treat it as fatal; idle reconciliation should not).
func PackagesForProfiles(profiles []string) []string {
	seen := make(map[string]struct{})
	for _, key := range NormalizeProfiles(profiles) {
		if key == "" {
			continue
		}
		pkgs, ok := ProfilePackages[key]
		if !ok {
			continue
		}
		for _, name := range pkgs {
			seen[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ProfilesForPackage returns the sorted set of profiles that include the given
// package name — the inverse of ProfilePackages. An empty result means the
// package is not in the placement map at all (unknown / not catalog-tracked);
// callers MUST treat that as a DISTINCT condition from "installed on a node
// whose profiles don't authorize it" — do not conflate an unknown package with
// a profile orphan.
//
// This is the package→required-profiles view that the cluster-doctor
// orphaned-install finding reports, kept consistent with the controller's
// catalog by the same consistency test that guards ProfilePackages.
func ProfilesForPackage(name string) []string {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil
	}
	var out []string
	for profile, pkgs := range ProfilePackages {
		for _, p := range pkgs {
			if p == key {
				out = append(out, profile)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// HasProfile reports whether the given profile exists in the map. Useful
// for caller-side validation before a Day-0 install.
func HasProfile(profile string) bool {
	_, ok := ProfilePackages[strings.ToLower(strings.TrimSpace(profile))]
	return ok
}

// NormalizeProfiles canonicalizes profile names:
// - trims whitespace
// - lowercases
// - deduplicates
// - expands inheritance (transitively)
// - sorts
func NormalizeProfiles(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	var out []string
	var visit func(string)
	visit = func(name string) {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, key)
		for _, inherited := range ProfileInheritance[key] {
			visit(inherited)
		}
	}
	for _, p := range raw {
		visit(p)
	}
	sort.Strings(out)
	return out
}

// UnknownProfiles returns the unknown profile names after canonicalization.
// It excludes empty/whitespace inputs.
func UnknownProfiles(raw []string) []string {
	normalized := NormalizeProfiles(raw)
	var unknown []string
	for _, p := range normalized {
		if !HasProfile(p) {
			unknown = append(unknown, p)
		}
	}
	return unknown
}
