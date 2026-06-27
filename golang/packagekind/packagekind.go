// Package packagekind exposes the canonical package "kind" classification —
// service / infrastructure / command — sourced from packages/registry.yaml.
//
// WHY THIS PACKAGE EXISTS
//
// Package kind was historically hand-maintained in eight places across two repos
// (registry.yaml, package.json type, awareness.yaml package_kind, spec metadata.kind,
// validate-package-identity.py CATALOG_KIND, component_catalog.go, node-agent
// inferPackageKind, repository inferCorrectKind). They agreed only by discipline —
// the root cause of the recurring xds cross-kind incident (see
// docs/design/package-classification-single-source.md and ai-memory
// architecture/83b8f143). registry.yaml is the declared single author.
//
// This package holds a BUILD-TIME GENERATED projection of registry.yaml's name→kind
// map (kinds_generated.go) and is the single in-process classifier the services repo
// consumes. It is intentionally a leaf package with no heavy dependencies so both the
// repository server and the node-agent (including the Day-0 path, which runs before
// etcd exists) can use it as a build-time table — never a runtime fetch.
//
// To change a package's kind: edit packages/registry.yaml and run
// `make gen-package-kinds`. Do NOT hand-edit kinds_generated.go, and do NOT add a new
// hardcoded kind map (forbidden_fix in the scar).
package packagekind

import "strings"

// Kind string constants — these mirror registry.yaml's `kind` field values.
const (
	KindService        = "service"
	KindInfrastructure = "infrastructure"
	KindCommand        = "command"
)

// KindOf returns the registry-declared kind for a canonical package name and
// whether the name is known. Matching is case-insensitive and trims spaces.
// Unknown names return ("", false) — callers decide the fallback (the services
// convention is to treat an unknown package as a service; see IsInfrastructure /
// IsCommand for the inverse-safe predicates).
func KindOf(name string) (string, bool) {
	k, ok := kinds[strings.ToLower(strings.TrimSpace(name))]
	return k, ok
}

// IsInfrastructure reports whether the registry classifies name as infrastructure.
// Unknown names are NOT infrastructure (fail-open to service), matching the prior
// hardcoded behaviour of inferPackageKind / inferCorrectKind.
func IsInfrastructure(name string) bool {
	k, ok := KindOf(name)
	return ok && k == KindInfrastructure
}

// IsCommand reports whether the registry classifies name as a command (CLI tool,
// no systemd unit). Unknown names are NOT commands.
func IsCommand(name string) bool {
	k, ok := KindOf(name)
	return ok && k == KindCommand
}

// IsService reports whether the registry classifies name as a Globular gRPC
// service. Unknown names are NOT (explicitly) services here — use !IsInfrastructure
// && !IsCommand when you want the fail-open "treat unknown as service" behaviour.
func IsService(name string) bool {
	k, ok := KindOf(name)
	return ok && k == KindService
}

// Names returns the sorted set of package names known to the registry projection.
// Primarily for tests / drift checks.
func Names() []string {
	out := make([]string, 0, len(kinds))
	for n := range kinds {
		out = append(out, n)
	}
	return out
}
