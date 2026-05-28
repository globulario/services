package rules

import "strings"

// CanonicalRuntimeDir returns the canonical filesystem directory name for a
// service's runtime state under /var/lib/globular/. The canonical form is the
// service's package name (hyphenated). Callers that have a non-canonical form
// (underscore, no separator, mixed case) should pass it through this function
// so layout drift detection and cleanup can consistently address one identity.
//
// See docs/intent/service_runtime_paths_must_be_canonical.yaml and
// invariant service.runtime_dir_name_must_be_canonical.
//
// Examples:
//
//	CanonicalRuntimeDir("cluster-doctor")  -> "cluster-doctor"
//	CanonicalRuntimeDir("cluster_doctor")  -> "cluster-doctor"
//	CanonicalRuntimeDir("clusterdoctor")   -> "cluster-doctor"   (only via alias map)
//	CanonicalRuntimeDir("ai_executor")     -> "ai-executor"
func CanonicalRuntimeDir(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	// Reverse-lookup the legacy alias map first: if `name` is a known legacy
	// alias (e.g. clusterdoctor, no separator), return the canonical form.
	if canon, ok := legacyToCanonical[trimmed]; ok {
		return canon
	}
	// Cheap transform: underscore → hyphen is a well-known no-data alias.
	return strings.ReplaceAll(trimmed, "_", "-")
}

// LegacyRuntimeAliases returns the known legacy directory names for a
// canonical service runtime dir. Used to classify "extra" dirs as cleanup
// candidates rather than scary unknown drift.
//
// The returned slice does NOT include the canonical name itself. Returns nil
// when the name has no registered aliases.
func LegacyRuntimeAliases(canonical string) []string {
	return canonicalToLegacy[strings.TrimSpace(canonical)]
}

// AllRuntimeDirAliases returns every directory name (canonical + aliases) that
// is recognized as the same logical service. Caller passes either canonical or
// alias; result includes canonical first followed by legacy aliases.
//
// Example: AllRuntimeDirAliases("cluster_doctor") ->
//
//	["cluster-doctor", "cluster_doctor", "clusterdoctor"]
func AllRuntimeDirAliases(name string) []string {
	canon := CanonicalRuntimeDir(name)
	if canon == "" {
		return nil
	}
	aliases := LegacyRuntimeAliases(canon)
	out := make([]string, 0, len(aliases)+1)
	out = append(out, canon)
	out = append(out, aliases...)
	return out
}

// IsKnownRuntimeDirAlias reports whether name appears anywhere in the
// canonical-or-alias model for some service. Used by the layout drift rule to
// distinguish "totally unknown dir" from "known service, just non-canonical
// name".
func IsKnownRuntimeDirAlias(name string) (canonical string, isAlias bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", false
	}
	if canon, ok := legacyToCanonical[trimmed]; ok {
		return canon, true
	}
	// A canonical name is its own "alias" — true if it has aliases registered.
	if _, ok := canonicalToLegacy[trimmed]; ok {
		return trimmed, true
	}
	return "", false
}

// canonicalToLegacy maps a canonical service runtime dir to its known legacy
// alias names. Only services with multiple historical names need an entry.
// Services whose only known name is the canonical (no aliases ever existed)
// do not appear here.
var canonicalToLegacy = map[string][]string{
	"cluster-doctor":     {"cluster_doctor", "clusterdoctor"},
	"cluster-controller": {"cluster_controller", "clustercontroller"},
	"node-agent":         {"node_agent", "nodeagent"},
	"ai-executor":        {"ai_executor"},
	"ai-memory":          {"ai_memory"},
	"ai-router":          {"ai_router"},
	"ai-watcher":         {"ai_watcher"},
	"backup-manager":     {"backup_manager"},
	"scylla-manager":     {"scylla_manager"},
	"scylla-manager-agent": {"scylla_manager_agent"},
}

// legacyToCanonical is the reverse index built once at init. Built deterministically
// from canonicalToLegacy so the two maps cannot drift.
var legacyToCanonical = buildLegacyToCanonical()

func buildLegacyToCanonical() map[string]string {
	out := make(map[string]string, 16)
	for canon, aliases := range canonicalToLegacy {
		for _, a := range aliases {
			out[a] = canon
		}
	}
	return out
}
