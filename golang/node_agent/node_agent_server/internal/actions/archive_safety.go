// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.archive_safety
// @awareness file_role=validate_before_mutate_gate_for_package_archive_extraction
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=critical
package actions

// archive_safety.go — the admission gate for package archive extraction.
//
// Package extraction runs as root. Before this gate existed, `config/`, `data/`
// and `policy/` entries were mapped to their destination roots with a bare
// strings.TrimPrefix, so an entry named `data/../../etc/cron.d/x` resolved to
// /etc/cron.d/x and was written as root. `bin/` and `systemd/` were safe only
// because they happen to use filepath.Base.
//
// Two non-negotiable properties:
//
//  1. VALIDATE BEFORE MUTATE. The whole archive is planned and checked before
//     the first byte is written. Discovering an unsafe entry halfway through
//     extraction, after earlier entries have already landed, produces a partial
//     install whose contents no longer match the published artifact — the
//     package identity is then unprovable and rollback evidence is muddy.
//
//  2. FAIL CLOSED, WHOLE-ARCHIVE. A package is admitted in full or rejected in
//     full. There is deliberately no skip-and-log mode: silently dropping an
//     unsafe entry while advancing the version marker and restarting the
//     service is exactly the ambiguous outcome this gate exists to prevent.
//
// The planner is the SINGLE source of the archive-path → destination mapping.
// serviceInstallPayloadAction consumes the plan it produces rather than
// recomputing destinations, so the validator and the extractor cannot drift
// apart. The audit command (golang/cmd/audit-package-archives) calls the same
// planner, so an audit result is evidence about the real installer.

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Bounded so a hostile or corrupt archive cannot exhaust the node before the
// gate has an opinion. Sized well above the largest real Globular package
// (scylladb, envoy) so a legitimate payload is never near the cap — the audit
// over the published corpus is what proves that claim.
//
// These are vars rather than consts only so tests can shrink them; production
// never reassigns them. A test that lowers a limit must restore it.
var (
	maxArtifactEntries       = 65536
	maxArtifactDeclaredBytes = int64(4) << 30
	maxArtifactFileBytes     = int64(2) << 30
)

// ArchiveViolation is one reason a package archive is inadmissible.
type ArchiveViolation struct {
	// Entry is the archive path exactly as the tar header declared it, before
	// any normalization — the audit needs the bytes the publisher shipped.
	Entry string
	// Reason is a stable, greppable classification.
	Reason string
	// Detail explains the specific failure for an operator.
	Detail string
	// Mapped reports whether this entry would have been extracted. An unmapped
	// entry is inert under the current installer, so the audit can separate
	// "this package would have written outside its root" from "this package
	// carries historical debris in a path the installer ignores".
	Mapped bool
}

func (v ArchiveViolation) String() string {
	scope := "unmapped"
	if v.Mapped {
		scope = "MAPPED"
	}
	return fmt.Sprintf("%s [%s] %s: %s", v.Entry, scope, v.Reason, v.Detail)
}

// Violation reasons.
const (
	ViolationBackslash      = "backslash_in_path"
	ViolationAbsolutePath   = "absolute_path"
	ViolationTraversal      = "parent_traversal"
	ViolationEmptyPath      = "empty_path"
	ViolationNonRegular     = "non_regular_entry"
	ViolationEscapesRoot    = "escapes_destination_root"
	ViolationDuplicateDest  = "duplicate_destination"
	ViolationEntryCount     = "entry_count_exceeded"
	ViolationArchiveTooBig  = "declared_bytes_exceeded"
	ViolationEntryTooBig    = "entry_bytes_exceeded"
	ViolationUnreadableName = "unreadable_path"
)

// extractionRoots is the node-owned destination layout. Every field is an
// absolute host path chosen by the node agent, never by the package.
type extractionRoots struct {
	service string
	bin     string
	systemd string
	config  string
	scripts string
	debs    string
	state   string
	policy  string
}

func currentExtractionRoots(service, scriptsDir, debsDir string) extractionRoots {
	binDir, systemdDir, configDir, _ := installPaths()
	return extractionRoots{
		service: service,
		bin:     binDir,
		systemd: systemdDir,
		config:  configDir,
		scripts: scriptsDir,
		debs:    debsDir,
		state:   ActionStateDir,
		policy:  ActionPolicyDir,
	}
}

// archiveEntryPlan is one admitted entry and the destination it may be written
// to. Nothing outside the planner is permitted to compute a destination.
type archiveEntryPlan struct {
	Name string // normalized, archive-relative
	Dest string // absolute host path
	Kind string // bin | systemd | config | scripts | data | policy
}

// planArtifactExtraction reads the archive without writing anything and
// returns the admitted extraction plan plus every violation found. A caller
// that receives any violation MUST NOT extract.
//
// err is reserved for "the archive could not be read at all". A malformed or
// hostile archive that reads fine is reported through violations so the audit
// can classify it.
func planArtifactExtraction(artifactPath string, roots extractionRoots) ([]archiveEntryPlan, []ArchiveViolation, error) {
	f, err := os.Open(artifactPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open artifact: %w", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var (
		plan       []archiveEntryPlan
		violations []ArchiveViolation
		entries    int
		declared   int64
	)
	destSeen := map[string]string{}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, violations, fmt.Errorf("read tar: %w", err)
		}
		entries++
		if entries > maxArtifactEntries {
			violations = append(violations, ArchiveViolation{
				Entry: hdr.Name, Reason: ViolationEntryCount, Mapped: true,
				Detail: fmt.Sprintf("archive declares more than %d entries", maxArtifactEntries),
			})
			break
		}
		if hdr.Size > maxArtifactFileBytes {
			violations = append(violations, ArchiveViolation{
				Entry: hdr.Name, Reason: ViolationEntryTooBig, Mapped: true,
				Detail: fmt.Sprintf("entry declares %d bytes (limit %d)", hdr.Size, maxArtifactFileBytes),
			})
		}
		declared += hdr.Size
		if declared > maxArtifactDeclaredBytes {
			violations = append(violations, ArchiveViolation{
				Entry: hdr.Name, Reason: ViolationArchiveTooBig, Mapped: true,
				Detail: fmt.Sprintf("archive declares more than %d uncompressed bytes", maxArtifactDeclaredBytes),
			})
			break
		}

		name, nameViolation := normalizeArchiveEntryName(hdr.Name)
		if nameViolation != nil {
			// A name-level defect is judged before mapping: an archive that
			// carries a traversal or absolute path is not a valid package even
			// when the installer would have ignored that prefix. Mapped is set
			// from a best-effort classification so the audit can still tell the
			// dangerous case from inert debris.
			nameViolation.Mapped = archivePrefixIsMapped(hdr.Name)
			violations = append(violations, *nameViolation)
			continue
		}

		// Directories carry no payload and are recreated by MkdirAll at write
		// time. They are still name-checked above.
		if hdr.FileInfo().IsDir() {
			continue
		}

		// The archive root reached here as something other than a directory.
		// A regular file cannot be named "." — refuse it rather than guess.
		if name == "" {
			violations = append(violations, ArchiveViolation{
				Entry: hdr.Name, Reason: ViolationEmptyPath, Mapped: false,
				Detail: fmt.Sprintf("archive root used for a non-directory entry (typeflag %q)", string(hdr.Typeflag)),
			})
			continue
		}

		kind, mapped := classifyArchiveEntry(name)
		if hdr.Typeflag != tar.TypeReg {
			// Symlinks, hard links, devices, FIFOs and sockets are refused.
			// The current installer never materializes them as links — it
			// os.Create()s the name and copies an empty body, silently
			// producing a stray zero-byte file — so refusing them takes away
			// no behavior any working package actually relies on.
			violations = append(violations, ArchiveViolation{
				Entry: hdr.Name, Reason: ViolationNonRegular, Mapped: mapped,
				Detail: fmt.Sprintf("typeflag %q (%s) is not a regular file", string(hdr.Typeflag), tarTypeName(hdr.Typeflag)),
			})
			continue
		}
		if !mapped {
			// Unsupported prefix with a safe name: inert, ignored by the
			// installer exactly as before this gate existed.
			continue
		}

		dest, root := archiveEntryDestination(name, kind, roots)
		if !pathContainedBy(dest, root) {
			violations = append(violations, ArchiveViolation{
				Entry: hdr.Name, Reason: ViolationEscapesRoot, Mapped: true,
				Detail: fmt.Sprintf("resolves to %s, outside %s", dest, root),
			})
			continue
		}
		if prior, dup := destSeen[dest]; dup {
			violations = append(violations, ArchiveViolation{
				Entry: hdr.Name, Reason: ViolationDuplicateDest, Mapped: true,
				Detail: fmt.Sprintf("destination %s already claimed by %s", dest, prior),
			})
			continue
		}
		destSeen[dest] = hdr.Name
		plan = append(plan, archiveEntryPlan{Name: name, Dest: dest, Kind: kind})
	}

	return plan, violations, nil
}

// normalizeArchiveEntryName converts a tar header name into a safe,
// archive-relative path, or explains why it cannot be one.
//
// It deliberately does NOT use strings.TrimLeft(name, "./"), which the previous
// implementation used: that cutset eats the dots of a leading "../" and turns
// an obvious traversal into an innocent-looking relative path.
func normalizeArchiveEntryName(raw string) (string, *ArchiveViolation) {
	if strings.Contains(raw, `\`) {
		return "", &ArchiveViolation{Entry: raw, Reason: ViolationBackslash,
			Detail: "backslash is not a legal separator in a package archive"}
	}
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "/") {
		return "", &ArchiveViolation{Entry: raw, Reason: ViolationAbsolutePath,
			Detail: "package entries must be archive-relative"}
	}
	// Strip repeated leading "./" only — never bare dots.
	for strings.HasPrefix(trimmed, "./") {
		trimmed = strings.TrimPrefix(trimmed, "./")
	}
	if trimmed == "" || trimmed == "." {
		// The archive root itself. `tar -C dir -czf out .` — which is how
		// scripts/build-release.sh packages every artifact — emits a "./"
		// directory entry for it. It is a container, not a payload entry, and
		// designates no destination, so it is returned as the empty name for
		// the caller to skip. The caller still rejects it if it arrives as
		// something other than a directory.
		return "", nil
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == ".." {
			return "", &ArchiveViolation{Entry: raw, Reason: ViolationTraversal,
				Detail: "entry contains a .. segment"}
		}
	}
	clean := path.Clean(trimmed)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", &ArchiveViolation{Entry: raw, Reason: ViolationEscapesRoot,
			Detail: fmt.Sprintf("normalizes to %q, which escapes the archive root", clean)}
	}
	return clean, nil
}

// classifyArchiveEntry reports the payload class and whether the installer maps
// it to a destination at all. The prefix set mirrors serviceInstallPayloadAction
// exactly; unknown prefixes are ignored as they always were.
func classifyArchiveEntry(name string) (kind string, mapped bool) {
	switch {
	case strings.HasPrefix(name, "bin/"):
		return "bin", true
	case strings.HasPrefix(name, "systemd/"), strings.HasPrefix(name, "units/"):
		return "systemd", true
	case strings.HasPrefix(name, "config/"):
		return "config", true
	case strings.HasPrefix(name, "scripts/"):
		return "scripts", true
	case strings.HasPrefix(name, "debs/"):
		// Bundled OS packages carrying a service's native library dependencies,
		// dpkg-installed before the ldd preflight. Added on master after this
		// branch was cut, and mapped here rather than in an inline switch so it
		// inherits the same containment enforcement as every other destination.
		return "debs", true
	case strings.HasPrefix(name, "data/"):
		return "data", true
	case strings.HasPrefix(name, "policy/"):
		return "policy", true
	default:
		return "", false
	}
}

// archivePrefixIsMapped is the best-effort classification used when a name is
// too malformed to normalize. It looks at the raw prefix so the audit can rank
// a traversal under data/ above one in an ignored directory.
func archivePrefixIsMapped(raw string) bool {
	trimmed := strings.TrimPrefix(strings.TrimSpace(raw), "./")
	_, mapped := classifyArchiveEntry(trimmed)
	return mapped
}

// archiveEntryDestination is the ONLY place an archive path becomes a host
// path. It returns the destination and the root that destination must stay
// inside.
func archiveEntryDestination(name, kind string, roots extractionRoots) (dest, root string) {
	switch kind {
	case "bin":
		return filepath.Join(roots.bin, filepath.Base(name)), roots.bin
	case "systemd":
		return filepath.Join(roots.systemd, filepath.Base(name)), roots.systemd
	case "config":
		root = filepath.Join(roots.config, roots.service)
		return filepath.Join(root, strings.TrimPrefix(name, "config/")), root
	case "scripts":
		return filepath.Join(roots.scripts, filepath.Base(name)), roots.scripts
	case "debs":
		return filepath.Join(roots.debs, filepath.Base(name)), roots.debs
	case "data":
		return filepath.Join(roots.state, strings.TrimPrefix(name, "data/")), roots.state
	case "policy":
		root = filepath.Join(roots.policy, roots.service)
		return filepath.Join(root, strings.TrimPrefix(name, "policy/")), root
	default:
		return "", ""
	}
}

// pathContainedBy reports whether dest is root or lives beneath it, comparing
// lexically after Clean. Both sides are node-owned absolute paths at this
// point, so no symlink resolution is required here; the roots themselves are
// root-owned and are not attacker-controlled.
func pathContainedBy(dest, root string) bool {
	if dest == "" || root == "" {
		return false
	}
	dest = filepath.Clean(dest)
	root = filepath.Clean(root)
	if dest == root {
		return false // a payload entry may never overwrite the root itself
	}
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func tarTypeName(flag byte) string {
	switch flag {
	case tar.TypeSymlink:
		return "symlink"
	case tar.TypeLink:
		return "hard link"
	case tar.TypeChar:
		return "character device"
	case tar.TypeBlock:
		return "block device"
	case tar.TypeFifo:
		return "fifo"
	case tar.TypeDir:
		return "directory"
	default:
		return "unsupported"
	}
}

// ValidateArtifactArchive is the exported audit entry point. It answers, for
// one artifact, exactly what the installer would refuse — using the installer's
// own planner, not a reimplementation.
//
// service is the package name the artifact would be installed as; it only
// affects the config/ and policy/ destination roots.
func ValidateArtifactArchive(artifactPath, service string) ([]ArchiveViolation, error) {
	roots := extractionRoots{
		service: service,
		bin:     ActionBinDir,
		systemd: ActionSystemdDir,
		config:  ActionConfigDir,
		scripts: filepath.Join(ActionStateDir, "staging", service, "scripts"),
		state:   ActionStateDir,
		policy:  ActionPolicyDir,
	}
	_, violations, err := planArtifactExtraction(artifactPath, roots)
	if err != nil {
		return violations, err
	}
	sort.SliceStable(violations, func(i, j int) bool {
		if violations[i].Mapped != violations[j].Mapped {
			return violations[i].Mapped // dangerous first
		}
		return violations[i].Entry < violations[j].Entry
	})
	return violations, nil
}

// describeViolations renders violations for an installer error message. It is
// bounded so a pathological archive cannot produce an unbounded log line.
func describeViolations(violations []ArchiveViolation) string {
	const maxShown = 10
	parts := make([]string, 0, maxShown+1)
	for i, v := range violations {
		if i == maxShown {
			parts = append(parts, fmt.Sprintf("… and %d more", len(violations)-maxShown))
			break
		}
		parts = append(parts, v.String())
	}
	return strings.Join(parts, "; ")
}
