package rules

import "github.com/globulario/services/golang/runtimedirs"

// The canonical-hyphen ↔ legacy-underscore runtime-dir alias model now lives in
// the shared leaf package github.com/globulario/services/golang/runtimedirs so
// it can be used by both consumers without a second handwritten list:
//   - cluster-doctor (here) is the DETECTOR — it classifies layout drift.
//   - node-agent is the SELF-HEALER — it canonicalizes legacy dirs on startup.
//
// These thin delegators preserve the rules-package API (CanonicalRuntimeDir,
// LegacyRuntimeAliases, AllRuntimeDirAliases, IsKnownRuntimeDirAlias) so the
// detector's call sites and tests are unchanged. See invariant
// service.runtime_dir_name_must_be_canonical.

// CanonicalRuntimeDir returns the canonical filesystem directory name for a
// service's runtime state under /var/lib/globular/. See runtimedirs for detail.
func CanonicalRuntimeDir(name string) string {
	return runtimedirs.CanonicalRuntimeDir(name)
}

// LegacyRuntimeAliases returns the known legacy directory names for a canonical
// service runtime dir (excluding the canonical name itself).
func LegacyRuntimeAliases(canonical string) []string {
	return runtimedirs.LegacyRuntimeAliases(canonical)
}

// AllRuntimeDirAliases returns every directory name (canonical + aliases) that
// is recognized as the same logical service, canonical first.
func AllRuntimeDirAliases(name string) []string {
	return runtimedirs.AllRuntimeDirAliases(name)
}

// IsKnownRuntimeDirAlias reports whether name appears anywhere in the
// canonical-or-alias model for some service.
func IsKnownRuntimeDirAlias(name string) (canonical string, isAlias bool) {
	return runtimedirs.IsKnownRuntimeDirAlias(name)
}
