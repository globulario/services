package rules

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/component_catalog"
)

// artifactStateRootPath is the directory whose top-level entries this rule
// inspects. Exposed as a var so tests can redirect to a temp dir.
var artifactStateRootPath = "/var/lib/globular"

// systemdUnitDirsForLayoutDrift is the search path for active systemd unit
// files. Exposed as a var so tests can redirect to a temp dir. Order matters:
// /run takes precedence over /etc in systemd's resolution, but for the
// "is this path referenced anywhere" check we union them.
var systemdUnitDirsForLayoutDrift = []string{
	"/etc/systemd/system",
	"/run/systemd/system",
	"/usr/lib/systemd/system",
}

// platformBaseAllowlist names the small set of platform-level directories
// that always belong directly under /var/lib/globular, regardless of which
// services are installed on this node. Service runtime directories are
// discovered from installed-state evidence (see Evaluate) rather than
// listed here — the allowlist must not become an ever-growing list of
// service names. See invariant doctor.layout_drift_must_reflect_real_risk
// and forbidden_fix forbidden.fix.layout_drift_by_expanding_allowlist_only.
var platformBaseAllowlist = map[string]bool{
	// Platform infrastructure — present on every node regardless of installed services.
	"awareness":             true,
	"backups":               true,
	// state/ is the platform-managed installation-state directory. It holds
	// post-install ownership records (e.g. state/scylladb/ownership.json) that
	// the package post-install scripts and install-day0.sh use to detect whether
	// infrastructure components were successfully installed on this node. It is
	// written by platform packages, not by Globular services, so it does not
	// appear in any service inventory. See packages/metadata/scylladb/scripts/
	// post-install.sh and scripts/release/install-day0.sh.
	"state":                 true,
	"bootstrap.enabled":     true,
	"config":                true,
	"config.json":           true,
	"data":                  true,
	"domains":               true,
	"etcd":                  true,
	"ingress":               true,
	"intent":                true,
	"inventory":             true,
	"keys":                  true,
	"minio":                 true,
	"objectstore":           true,
	"operational-knowledge": true,
	"packages":              true,
	"pki":                   true,
	"policy":                true,
	"recovery":              true,
	"release-index.json":    true,
	// releases/ is the repository service's local-upstream release mirror
	// ({local_root}/releases/{tag}/release-index.json — see golang/repository/
	// upstream/local_source.go). It is a platform release-artifact directory in
	// the same family as packages/, staging/, and release-index.json — NOT a
	// service runtime dir — created and torn down by the platform release
	// tooling (scripts/prepare-rejoin.sh removes it as "will be recreated").
	// Listing it here is consistent with the existing release-artifact entries
	// and does not introduce a service name, so it does not trip
	// forbidden.fix.layout_drift_by_expanding_allowlist_only.
	"releases":              true,
	"services":              true,
	"staging":               true,
	"tokens":                true,
	"webroot":               true,
	"workflows":             true,
	// Core platform services — part of the cluster's founding-quorum
	// minimum footprint (every node runs these per the storage/core/
	// control-plane profile contract). Listing here means they remain
	// silent at the layout layer even when the collector has not yet
	// populated an inventory (e.g. early boot, test fixtures with
	// snap=nil). When the inventory IS populated, discoverInstalledRuntimeDirs
	// will additionally see them as installed.
	"cluster-controller": true,
	"mcp":                true,
	"node-agent":         true,
	"prometheus":         true,
	"repository":         true,
	"scylla-manager":     true,
	"scylla-manager-agent": true,
	"sidekick":           true,
	"workflow":           true,
	"xds":                true,
}

// dirClassification is the verdict for one top-level entry.
type dirClassification int

const (
	classOK              dirClassification = iota // platform base or canonical service runtime — silent
	classCleanupEmpty                             // known legacy alias, empty → cleanup candidate (INFO)
	classCleanupTransient                         // backup/transient file (e.g. config.json.bak.*) → cleanup candidate
	classWarnDuplicate                            // known legacy alias with content (data-bearing) → operator review
	classWarnUnknown                              // truly unknown dir → WARN
	classWarnOrphanData                           // catalog-known service, not installed, data present → WARN (review before removal)
)

// classifyEntry returns the verdict for a single top-level entry name + path.
// The classifier never mutates state; it only inspects.
//
// pinnedPaths is the set of absolute paths actively pinned by some systemd
// unit (WorkingDirectory= or ExecStartPre mkdir). When a legacy-alias dir
// is pinned by a unit, it is NOT a cleanup candidate — deleting it would
// be undone by systemd on next service start AND would silently keep the
// dir under its non-canonical name. The right fix in that case is to
// update the unit templates, not to flag this dir for cleanup. The rule
// silently classifies pinned aliases as OK so it doesn't issue actionable-
// looking findings against state that is actively allocated.
//
// See invariant doctor.layout_drift_must_reflect_real_risk and
// failure_mode filesystem.service_runtime_dir_naming_drift.
func classifyEntry(name, path string, installedDirSet, pinnedPaths map[string]bool) dirClassification {
	// 1. Platform base — always OK.
	if platformBaseAllowlist[name] {
		return classOK
	}
	// 2. Hidden entries (.lock, .git, etc) — silent.
	if strings.HasPrefix(name, ".") {
		return classOK
	}
	// 3. Transient/backup files — common pattern is *.bak.* or *-install.jsonl.
	if isTransientFileName(name) {
		return classCleanupTransient
	}
	// 4. Canonical service runtime dir derived from installed-state — OK.
	canonical := CanonicalRuntimeDir(name)
	if canonical != "" && installedDirSet[canonical] {
		// `name` is either the canonical itself OR a known legacy alias that
		// maps to a canonical the node has installed.
		if name == canonical {
			return classOK
		}
		// Legacy alias of an installed service. Reference-safety guard:
		// if a systemd unit actively pins this exact path (WorkingDirectory=
		// or ExecStartPre mkdir), we must NOT flag for cleanup — deletion
		// would be undone by systemd, and the alias is operationally
		// canonical for the running service. Operator should fix the unit
		// template (separate concern, not this rule's scope).
		if pinnedPaths[path] {
			return classOK
		}
		// Not pinned by any unit — genuinely safe to surface as cleanup.
		// Empty = silent cleanup (INFO); non-empty = operator review (WARN).
		if dirIsEmpty(path) {
			return classCleanupEmpty
		}
		return classWarnDuplicate
	}
	// 5. Known legacy alias for a service NOT installed on this node — still
	//    cleanup candidate. Same reference-safety guard applies: if some
	//    unit still pins this dir, we cannot safely flag it.
	if canon, ok := IsKnownRuntimeDirAlias(name); ok && canon != "" {
		if pinnedPaths[path] {
			return classOK
		}
		if dirIsEmpty(path) {
			return classCleanupEmpty
		}
		return classWarnDuplicate
	}
	// 5b. Catalog-known service name with no install on this node. The entry is
	//     explainable by the service catalog (component_catalog placement map),
	//     so it is the uninstalled-service runtime dir / orphan class — NOT
	//     truly-unknown drift. It is recognized via the catalog rather than a
	//     hand-maintained allowlist, so this respects
	//     forbidden_fix forbidden.fix.layout_drift_by_expanding_allowlist_only
	//     and invariant doctor.layout_drift_must_reflect_real_risk (distinguish
	//     cleanup candidates from real risk). Same reference-safety guard and
	//     empty/non-empty split as the alias path: pinned → OK; empty → INFO
	//     cleanup candidate; non-empty (data present) → WARN, review before
	//     removal. The signal is downgraded in severity but never silenced.
	if isCatalogKnownServiceName(canonical) {
		if pinnedPaths[path] {
			return classOK
		}
		if dirIsEmpty(path) {
			return classCleanupEmpty
		}
		return classWarnOrphanData
	}
	// 6. Truly unknown — WARN.
	return classWarnUnknown
}

// isCatalogKnownServiceName reports whether name is a service/package the
// component catalog knows about — i.e. it appears in the placement map for at
// least one profile. This is the principled, build-sourced way to recognize a
// service runtime directory without maintaining a hand-edited allowlist (see
// forbidden_fix forbidden.fix.layout_drift_by_expanding_allowlist_only). A
// catalog-known name whose service is NOT installed on this node is an
// uninstalled-service runtime dir (orphan), not "unknown" drift. Callers pass
// the canonicalized name.
func isCatalogKnownServiceName(name string) bool {
	return len(component_catalog.ProfilesForPackage(name)) > 0
}

// systemdWorkingDirRE matches `WorkingDirectory=` settings in unit files.
// The optional `-` prefix means "directory may be missing at start"
// (systemd.exec(5)). Captures the absolute path. We do not interpret
// systemd specifiers (%t, %h, etc.) — paths that depend on them won't
// match here, which is fail-closed for our purpose.
var systemdWorkingDirRE = regexp.MustCompile(`^\s*WorkingDirectory\s*=\s*-?\s*(\S+)\s*$`)

// systemdMkdirInExecStartPreRE matches the common Globular pattern in
// ExecStartPre lines: `mkdir -p <abs-path>`. Captures the first absolute
// path argument after `mkdir`. Multi-arg mkdir (`mkdir -p a b c`) only
// captures the first; that's acceptable for the "is this path pinned?"
// question — additional dirs are independently considered in their own
// rule passes.
var systemdMkdirInExecStartPreRE = regexp.MustCompile(`mkdir\s+(?:-[a-zA-Z]+\s+)*([/][^\s'";|&)]+)`)

// pathsPinnedBySystemdUnits scans the configured systemd unit directories
// for any unit file that references an absolute path under stateRoot via
// WorkingDirectory= or an ExecStartPre `mkdir`. Returns the set of
// referenced absolute paths.
//
// Fail-safe semantics: if a directory cannot be read or a file cannot be
// parsed, the function skips it and continues. The returned set is a
// LOWER BOUND on what is pinned (we may miss exotic syntaxes); the
// classifier treats absence as "no opinion → may flag as cleanup", so
// the rule errs on the side of NOT recommending cleanup when in doubt
// only because the systemd scan IS reliable for the common Globular
// pattern (WorkingDirectory + mkdir in ExecStartPre).
func pathsPinnedBySystemdUnits(stateRoot string) map[string]bool {
	pinned := make(map[string]bool)
	stateRoot = strings.TrimRight(stateRoot, "/")
	if stateRoot == "" {
		return pinned
	}
	prefix := stateRoot + "/"
	for _, dir := range systemdUnitDirsForLayoutDrift {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".service") {
				continue
			}
			f, err := os.Open(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if m := systemdWorkingDirRE.FindStringSubmatch(line); m != nil {
					if p := strings.TrimSpace(m[1]); strings.HasPrefix(p, prefix) || p == stateRoot {
						pinned[p] = true
					}
				}
				if matches := systemdMkdirInExecStartPreRE.FindAllStringSubmatch(line, -1); matches != nil {
					for _, m := range matches {
						if p := strings.TrimSpace(m[1]); strings.HasPrefix(p, prefix) || p == stateRoot {
							pinned[p] = true
						}
					}
				}
			}
			_ = f.Close()
		}
	}
	return pinned
}

// isTransientFileName recognizes patterns that are by-convention backup or
// ephemeral entries the platform writes under /var/lib/globular. They are not
// runtime state and should be classified as cleanup candidates.
func isTransientFileName(name string) bool {
	// Common backup pattern: <file>.bak.<unix-ns>
	if strings.Contains(name, ".bak.") {
		return true
	}
	// Day-0 install transcript: day0-install.jsonl
	if name == "day0-install.jsonl" {
		return true
	}
	return false
}

// dirIsEmpty reports whether the given path is a directory with no children.
// Non-directories and unreadable paths return false (we cannot conclude empty).
func dirIsEmpty(path string) bool {
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return false
	}
	ents, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(ents) == 0
}

type artifactLayoutDriftLocal struct{}

func (artifactLayoutDriftLocal) ID() string       { return "artifact.layout_drift_local" }
func (artifactLayoutDriftLocal) Category() string { return "convergence" }
func (artifactLayoutDriftLocal) Scope() string    { return "cluster" }

// Evaluate inspects /var/lib/globular/'s top-level entries and classifies each
// against the platform base allowlist plus the discovered set of canonical
// service runtime directories from installed-state. Findings are scoped:
//
//   - WARN: truly unknown dir, OR data-bearing duplicate of a known service
//   - INFO: cleanup candidates (empty legacy aliases, transient backup files)
//   - silent: platform base + canonical runtime dirs for installed services
//
// Critically: a known service runtime dir is NOT silenced for permission
// invariants — those are enforced by separate rules (e.g. etcd 0700). This
// rule only addresses layout/naming; it does not suppress sensitive-dir
// permission findings. See invariant sensitive_runtime_dirs_require_strict_permissions.
func (r artifactLayoutDriftLocal) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	ents, err := os.ReadDir(artifactStateRootPath)
	if err != nil {
		return nil
	}

	// Build the set of canonical runtime dirs the node has actually
	// installed. We accept entries from any node's inventory in the snapshot
	// since this rule runs against the local filesystem (intent is "is this
	// dir explainable by some installed service").
	installedDirs := discoverInstalledRuntimeDirs(snap)

	// Reference-safety guard: scan loaded systemd unit files for paths
	// under stateRoot that they actively pin via WorkingDirectory= or
	// ExecStartPre `mkdir`. Legacy-alias dirs pinned by a unit MUST NOT
	// be reported as cleanup candidates — deleting them would be undone
	// by systemd on next service start, AND the misleading finding
	// distracts operators from the real fix (update the unit template
	// to use the canonical dash-form path). See
	// invariant doctor.layout_drift_must_reflect_real_risk.
	pinnedPaths := pathsPinnedBySystemdUnits(artifactStateRootPath)

	var warnUnknown []string
	var warnDuplicate []string
	var warnOrphanData []string
	var cleanupCandidates []string

	for _, e := range ents {
		name := strings.TrimSpace(e.Name())
		if name == "" {
			continue
		}
		path := filepath.Join(artifactStateRootPath, name)
		switch classifyEntry(name, path, installedDirs, pinnedPaths) {
		case classOK:
			continue
		case classCleanupEmpty, classCleanupTransient:
			cleanupCandidates = append(cleanupCandidates, name)
		case classWarnDuplicate:
			warnDuplicate = append(warnDuplicate, name)
		case classWarnOrphanData:
			warnOrphanData = append(warnOrphanData, name)
		case classWarnUnknown:
			warnUnknown = append(warnUnknown, name)
		}
	}

	var findings []Finding

	if len(warnUnknown) > 0 {
		sort.Strings(warnUnknown)
		findings = append(findings, Finding{
			FindingID:       FindingID("artifact.layout_drift_local.unknown", artifactStateRootPath, strings.Join(warnUnknown, ",")),
			InvariantID:     "artifact.layout_drift_local",
			Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:        "convergence",
			EntityRef:       artifactStateRootPath,
			Summary:         fmt.Sprintf("unknown top-level entries under %s: %s", artifactStateRootPath, strings.Join(warnUnknown, ", ")),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("local_fs", "readdir", map[string]string{
					"path":    artifactStateRootPath,
					"unknown": strings.Join(warnUnknown, ","),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Inspect each unknown entry — non-empty unknown dirs may indicate a service installed via a non-canonical path.", "sudo ls -la "+filepath.Clean(artifactStateRootPath)),
			},
		})
	}

	if len(warnOrphanData) > 0 {
		sort.Strings(warnOrphanData)
		findings = append(findings, Finding{
			FindingID:       FindingID("artifact.layout_drift_local.orphan_data", artifactStateRootPath, strings.Join(warnOrphanData, ",")),
			InvariantID:     "artifact.layout_drift_local",
			Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:        "convergence",
			EntityRef:       artifactStateRootPath,
			Summary:         fmt.Sprintf("runtime dirs for catalog services not installed on this node (data present — review before removal): %s", strings.Join(warnOrphanData, ", ")),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("local_fs", "uninstalled_service_dirs", map[string]string{
					"path":                artifactStateRootPath,
					"uninstalled_service": strings.Join(warnOrphanData, ","),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Either install the service so the dir is backed by a running unit, or migrate/remove the leftover data after review. Do not auto-delete data-bearing dirs.", ""),
			},
		})
	}

	if len(warnDuplicate) > 0 {
		sort.Strings(warnDuplicate)
		findings = append(findings, Finding{
			FindingID:       FindingID("artifact.layout_drift_local.duplicate", artifactStateRootPath, strings.Join(warnDuplicate, ",")),
			InvariantID:     "service.runtime_dir_name_must_be_canonical",
			Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:        "convergence",
			EntityRef:       artifactStateRootPath,
			Summary:         fmt.Sprintf("non-empty legacy-alias runtime dirs found alongside canonical names: %s", strings.Join(warnDuplicate, ", ")),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("local_fs", "duplicate_runtime_dirs", map[string]string{
					"path":      artifactStateRootPath,
					"duplicate": strings.Join(warnDuplicate, ","),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Inspect duplicate dirs for data. Migrate content to the canonical name; do not delete non-empty dirs without operator review.", ""),
			},
		})
	}

	if len(cleanupCandidates) > 0 {
		sort.Strings(cleanupCandidates)
		findings = append(findings, Finding{
			FindingID:       FindingID("artifact.layout_drift_local.cleanup", artifactStateRootPath, strings.Join(cleanupCandidates, ",")),
			InvariantID:     "artifact.layout_drift_local",
			Severity:        cluster_doctorpb.Severity_SEVERITY_INFO,
			Category:        "convergence",
			EntityRef:       artifactStateRootPath,
			Summary:         fmt.Sprintf("cleanup-candidate entries under %s (empty legacy aliases, empty uninstalled-service dirs, or backup files): %s", artifactStateRootPath, strings.Join(cleanupCandidates, ", ")),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("local_fs", "cleanup_candidates", map[string]string{
					"path":              artifactStateRootPath,
					"cleanup_candidate": strings.Join(cleanupCandidates, ","),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "These entries are safe to remove after evidence review. Do not auto-delete; confirm each is empty and inactive.", ""),
			},
		})
	}

	return findings
}

// discoverInstalledRuntimeDirs returns the set of canonical runtime dir names
// derived from the collector snapshot's installed packages. A name is added
// when ANY node's inventory carries an installed SERVICE/INFRASTRUCTURE/COMMAND
// package with that name (canonicalized). The rule runs against the local
// filesystem, so cross-node discovery is harmless — we are only answering
// "is this dir explainable by some installed package known to the platform?"
//
// When snap is nil OR no inventories are populated, returns an empty set; in
// that case every non-platform-base entry will be classified as unknown.
// Callers should ensure the collector has populated Inventories before this
// rule runs in production.
func discoverInstalledRuntimeDirs(snap *collector.Snapshot) map[string]bool {
	out := make(map[string]bool)
	if snap == nil {
		return out
	}
	for _, inv := range snap.Inventories {
		if inv == nil {
			continue
		}
		for _, comp := range inv.GetComponents() {
			if comp == nil {
				continue
			}
			name := strings.TrimSpace(comp.GetName())
			if name == "" {
				continue
			}
			out[CanonicalRuntimeDir(name)] = true
		}
	}
	return out
}
