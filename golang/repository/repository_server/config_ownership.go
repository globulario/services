package main

// config_ownership.go — Phase CLI-D package config ownership classification.
//
// This file is the repository-side authoring layer. The full
// install / upgrade / rollback policy execution (snapshot, three-way merge,
// fail-on-local-modification, secret redaction) is the node-agent's job and
// is wired by the install/upgrade workflows; that work is explicitly
// out-of-scope for this pass — search "TODO(config-exec)" in node_agent.
//
// What this file provides:
//   - Default merge strategy per ConfigKind (used when a manifest entry
//     omits merge_strategy).
//   - Classification helper that tells operator tooling whether a given
//     config entry is sensitive / preservable / restorable.
//   - Diff helper that computes "modified vs install-time" status from a
//     PackageConfigFile row carrying current_checksum + checksum_at_install.

import (
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// DefaultMergeStrategy returns the default for a config kind when the manifest
// entry left merge_strategy unset. These defaults match the spec:
//
//	DEFAULT             → REPLACE   (replaceable on upgrade if unmodified)
//	OPERATOR_OVERRIDE   → PRESERVE  (never overwritten silently)
//	GENERATED           → TEMPLATE_RENDER (regenerable from desired state)
//	SECRET              → SECRET_EXTERNAL (never logged, never in plain manifest)
//	RUNTIME_STATE       → APPEND_ONLY (treated as state, not config)
func DefaultMergeStrategy(kind repopb.ConfigKind) repopb.MergeStrategy {
	switch kind {
	case repopb.ConfigKind_CONFIG_DEFAULT:
		return repopb.MergeStrategy_MERGE_REPLACE
	case repopb.ConfigKind_CONFIG_OPERATOR_OVERRIDE:
		return repopb.MergeStrategy_MERGE_PRESERVE
	case repopb.ConfigKind_CONFIG_GENERATED:
		return repopb.MergeStrategy_MERGE_TEMPLATE_RENDER
	case repopb.ConfigKind_CONFIG_SECRET:
		return repopb.MergeStrategy_MERGE_SECRET_EXTERNAL
	case repopb.ConfigKind_CONFIG_RUNTIME_STATE:
		return repopb.MergeStrategy_MERGE_APPEND_ONLY
	default:
		return repopb.MergeStrategy_MERGE_REPLACE
	}
}

// ResolveConfigEntry returns a clone of the input PackageConfigFile with
// defaults filled in: missing merge_strategy is resolved from ConfigKind, and
// `sensitive` is forced true for SECRET entries regardless of the manifest.
func ResolveConfigEntry(in *repopb.PackageConfigFile) *repopb.PackageConfigFile {
	if in == nil {
		return nil
	}
	out := &repopb.PackageConfigFile{
		Path:                in.GetPath(),
		ConfigKind:          in.GetConfigKind(),
		OwnerPackage:        in.GetOwnerPackage(),
		ChecksumAtInstall:   in.GetChecksumAtInstall(),
		CurrentChecksum:     in.GetCurrentChecksum(),
		LastModifiedUnix:    in.GetLastModifiedUnix(),
		MergeStrategy:       in.GetMergeStrategy(),
		PreserveOnUpgrade:   in.GetPreserveOnUpgrade(),
		RestoreOnRollback:   in.GetRestoreOnRollback(),
		Sensitive:           in.GetSensitive(),
	}
	if out.MergeStrategy == repopb.MergeStrategy_MERGE_STRATEGY_UNSPECIFIED {
		out.MergeStrategy = DefaultMergeStrategy(out.ConfigKind)
	}
	if out.ConfigKind == repopb.ConfigKind_CONFIG_SECRET {
		out.Sensitive = true
	}
	// OPERATOR_OVERRIDE implies preserve_on_upgrade=true unless explicitly
	// overridden by the manifest (which is unusual; operators rarely
	// downgrade their own override to "replace me").
	if out.ConfigKind == repopb.ConfigKind_CONFIG_OPERATOR_OVERRIDE && !out.PreserveOnUpgrade {
		out.PreserveOnUpgrade = true
	}
	return out
}

// ConfigDiffStatus is the operator-facing status of one config file: whether
// the file on disk has drifted from what the package shipped.
type ConfigDiffStatus string

const (
	ConfigStatusUnknown    ConfigDiffStatus = "UNKNOWN"
	ConfigStatusUnchanged  ConfigDiffStatus = "UNCHANGED"
	ConfigStatusModified   ConfigDiffStatus = "MODIFIED"
	ConfigStatusMissing    ConfigDiffStatus = "MISSING"
	ConfigStatusGenerated  ConfigDiffStatus = "GENERATED" // GENERATED kind — drift is normal
	ConfigStatusRedacted   ConfigDiffStatus = "REDACTED"  // SECRET — checksum reported, content withheld
)

// ClassifyConfigDiff returns the diff status based on checksum-at-install vs
// current-checksum (both populated by the node-agent). Empty checksums mean
// "not yet captured" — caller should report UNKNOWN.
func ClassifyConfigDiff(c *repopb.PackageConfigFile) ConfigDiffStatus {
	if c == nil {
		return ConfigStatusUnknown
	}
	switch c.GetConfigKind() {
	case repopb.ConfigKind_CONFIG_GENERATED:
		return ConfigStatusGenerated
	case repopb.ConfigKind_CONFIG_SECRET:
		// Even for SECRET we can compare checksums — content isn't exposed.
		// But fall through to the unchanged/modified check below.
	}
	at := strings.TrimSpace(c.GetChecksumAtInstall())
	cur := strings.TrimSpace(c.GetCurrentChecksum())
	if at == "" && cur == "" {
		return ConfigStatusUnknown
	}
	if cur == "" {
		return ConfigStatusMissing
	}
	if at == "" {
		// First time we've seen a checksum (e.g. legacy install). Treat as
		// modified-relative-to-unknown; operator can `accept-new` to lock in.
		return ConfigStatusModified
	}
	if digestEqual(at, cur) {
		return ConfigStatusUnchanged
	}
	return ConfigStatusModified
}

// RedactConfig returns a clone safe to log / print: SECRET file paths keep
// their checksum but lose the path/owner — operator can still see "something
// changed" without the file location leaking into operator output.
func RedactConfig(c *repopb.PackageConfigFile) *repopb.PackageConfigFile {
	r := ResolveConfigEntry(c)
	if r == nil {
		return nil
	}
	if r.GetSensitive() || r.GetConfigKind() == repopb.ConfigKind_CONFIG_SECRET {
		// Strip path + owner_package — keep checksums (they don't leak content).
		r.Path = "[REDACTED]"
		r.OwnerPackage = "[REDACTED]"
	}
	return r
}

// PolicyAllowsUpgrade returns (allowed, reason) for one config entry given
// the current and target configs. Caller iterates this for every config file
// before issuing a package upgrade; FAIL_ON_LOCAL_MODIFICATION blocks the
// whole upgrade with a clear message.
func PolicyAllowsUpgrade(current *repopb.PackageConfigFile) (bool, string) {
	if current == nil {
		return true, "ok"
	}
	if ClassifyConfigDiff(current) != ConfigStatusModified {
		return true, "ok"
	}
	switch current.GetMergeStrategy() {
	case repopb.MergeStrategy_MERGE_FAIL_ON_LOCAL_MODIFICATION:
		return false, "config has local modifications and merge_strategy=FAIL_ON_LOCAL_MODIFICATION"
	case repopb.MergeStrategy_MERGE_PRESERVE,
		repopb.MergeStrategy_MERGE_THREE_WAY,
		repopb.MergeStrategy_MERGE_APPEND_ONLY,
		repopb.MergeStrategy_MERGE_TEMPLATE_RENDER,
		repopb.MergeStrategy_MERGE_SECRET_EXTERNAL,
		repopb.MergeStrategy_MERGE_REPLACE:
		return true, "ok"
	}
	return true, "ok"
}
