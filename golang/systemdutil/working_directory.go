// Package systemdutil contains shared systemd unit-file helpers used by both
// the node-agent install path (internal/actions/artifact.go) and the CLI
// install path (globularcli/services_cmds.go). Putting the helpers in one
// place is the documented antidote to the duplicate-install-path drift class
// (see INC-2026-0015, Project L; the WorkingDirectory variant is Project O).
package systemdutil

import (
	"bytes"
	"strings"

	"github.com/globulario/services/golang/runtimedirs"
)

// stateRootPrefix is the canonical state-dir prefix every Globular service
// renders into its systemd unit. NormalizeUnitWorkingDirectory only rewrites
// WorkingDirectory lines that point under this prefix — non-Globular units
// (e.g. distro-installed services bundled as wrappers) are left untouched.
const stateRootPrefix = "/var/lib/globular/"

// NormalizeUnitWorkingDirectory rewrites `WorkingDirectory=` lines that target
// the Globular state root so they (1) use the optional `-` prefix and (2) point
// at the CANONICAL hyphenated per-service runtime dir.
//
// (1) The `-` prefix: systemd evaluates WorkingDirectory= before ExecStartPre,
// so a missing dir causes status=200/CHDIR before any recovery mkdir can run.
// The `-` prefix makes the dir optional: systemd falls back to "/" if it's
// missing and ExecStartPre can recreate it.
//
// (2) Canonicalization: a unit that pins a legacy underscore alias (e.g.
// `/var/lib/globular/cluster_doctor`) makes systemd re-materialize that dir on
// every start, which permanently defeats the node-agent's one-shot
// CanonicalizeRuntimeDirsOnce sweep — the legacy dir keeps coming back and
// cluster-doctor keeps flagging it as layout drift. Rewriting the first path
// segment under the state root to its canonical hyphenated form (via the shared
// runtimedirs alias model) stops the dir from ever being re-created. Only the
// per-service segment is canonicalized; deeper path components are left intact
// (they may legitimately contain underscores).
//
// Behavior:
//   - `WorkingDirectory=/var/lib/globular/foo`            → `WorkingDirectory=-/var/lib/globular/foo`
//   - `WorkingDirectory=/var/lib/globular/cluster_doctor` → `WorkingDirectory=-/var/lib/globular/cluster-doctor`
//   - `WorkingDirectory=-/var/lib/globular/ai_router`     → `WorkingDirectory=-/var/lib/globular/ai-router`
//   - `WorkingDirectory=-/var/lib/globular/cluster-doctor`→ unchanged (idempotent)
//   - `WorkingDirectory=/etc/something`                   → unchanged (non-Globular)
//   - `# WorkingDirectory=/var/lib/globular/x`            → unchanged (commented out)
//   - lines with only whitespace before the directive are preserved
//
// Returns the (possibly rewritten) unit content. If nothing changed, returns
// content unchanged (compare by pointer or length to detect no-op when
// useful).
func NormalizeUnitWorkingDirectory(content []byte) []byte {
	lines := bytes.Split(content, []byte{'\n'})
	changed := false
	for i, line := range lines {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) == 0 || trimmed[0] == '#' || trimmed[0] == ';' {
			continue // comment or blank
		}
		if !bytes.HasPrefix(trimmed, []byte("WorkingDirectory=")) {
			continue
		}
		val := bytes.TrimPrefix(trimmed, []byte("WorkingDirectory="))
		// Split off any existing optional `-` prefix so we evaluate the path.
		path := val
		if len(path) > 0 && path[0] == '-' {
			path = path[1:]
		}
		// Only rewrite Globular state-root paths.
		if !bytes.HasPrefix(path, []byte(stateRootPrefix)) {
			continue
		}
		// Canonical form is always optional (`-`) + canonical runtime-dir segment.
		newVal := append([]byte{'-'}, canonicalizeStateRootPath(path)...)
		if bytes.Equal(newVal, val) {
			continue // already optional + canonical
		}
		// Preserve any leading whitespace from the original line.
		leadLen := len(line) - len(trimmed)
		newLine := make([]byte, 0, leadLen+len("WorkingDirectory=")+len(newVal))
		newLine = append(newLine, line[:leadLen]...)
		newLine = append(newLine, []byte("WorkingDirectory=")...)
		newLine = append(newLine, newVal...)
		lines[i] = newLine
		changed = true
	}
	if !changed {
		return content
	}
	return bytes.Join(lines, []byte{'\n'})
}

// canonicalizeStateRootPath rewrites the first path segment after the Globular
// state-root prefix to its canonical hyphenated runtime-dir name, leaving the
// prefix and any deeper path components untouched. The input MUST already start
// with stateRootPrefix.
func canonicalizeStateRootPath(path []byte) []byte {
	rest := path[len(stateRootPrefix):]
	seg, tail := rest, []byte(nil)
	if idx := bytes.IndexByte(rest, '/'); idx >= 0 {
		seg, tail = rest[:idx], rest[idx:] // tail keeps its leading '/'
	}
	canon := runtimedirs.CanonicalRuntimeDir(string(seg))
	out := make([]byte, 0, len(stateRootPrefix)+len(canon)+len(tail))
	out = append(out, []byte(stateRootPrefix)...)
	out = append(out, []byte(canon)...)
	out = append(out, tail...)
	return out
}

// NormalizeUnitWorkingDirectoryString is the string convenience wrapper.
func NormalizeUnitWorkingDirectoryString(content string) string {
	return string(NormalizeUnitWorkingDirectory([]byte(content)))
}

// HasBareGlobularWorkingDirectory reports whether content contains any
// non-commented `WorkingDirectory=/var/lib/globular/...` line missing the
// `-` prefix. Useful for the cluster-doctor invariant guard
// (Project O.5: systemd.working_directory.must_be_optional).
func HasBareGlobularWorkingDirectory(content []byte) bool {
	for _, line := range bytes.Split(content, []byte{'\n'}) {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) == 0 || trimmed[0] == '#' || trimmed[0] == ';' {
			continue
		}
		if !bytes.HasPrefix(trimmed, []byte("WorkingDirectory=")) {
			continue
		}
		val := bytes.TrimPrefix(trimmed, []byte("WorkingDirectory="))
		if len(val) > 0 && val[0] == '-' {
			continue
		}
		if bytes.HasPrefix(val, []byte(stateRootPrefix)) {
			return true
		}
	}
	return false
}

// stripTemplatePrefix is exposed for tests that want to assert that the
// stateRootPrefix matches the artifact.go default. Keep this exported so
// callers can audit the constant if it ever needs to change.
func StateRootPrefix() string { return strings.TrimSuffix(stateRootPrefix, "/") }
