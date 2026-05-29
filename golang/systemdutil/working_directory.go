// Package systemdutil contains shared systemd unit-file helpers used by both
// the node-agent install path (internal/actions/artifact.go) and the CLI
// install path (globularcli/services_cmds.go). Putting the helpers in one
// place is the documented antidote to the duplicate-install-path drift class
// (see INC-2026-0015, Project L; the WorkingDirectory variant is Project O).
package systemdutil

import (
	"bytes"
	"strings"
)

// stateRootPrefix is the canonical state-dir prefix every Globular service
// renders into its systemd unit. NormalizeUnitWorkingDirectory only rewrites
// WorkingDirectory lines that point under this prefix — non-Globular units
// (e.g. distro-installed services bundled as wrappers) are left untouched.
const stateRootPrefix = "/var/lib/globular/"

// NormalizeUnitWorkingDirectory rewrites bare `WorkingDirectory=` lines that
// target the Globular state root so they use the optional `-` prefix.
// systemd evaluates WorkingDirectory= before ExecStartPre, so a missing dir
// causes status=200/CHDIR before any recovery mkdir can run. The `-` prefix
// makes the dir optional: systemd falls back to "/" if it's missing and
// ExecStartPre can recreate it.
//
// Behavior:
//   - `WorkingDirectory=/var/lib/globular/foo`  → `WorkingDirectory=-/var/lib/globular/foo`
//   - `WorkingDirectory=-/var/lib/globular/foo` → unchanged (idempotent)
//   - `WorkingDirectory=/etc/something`         → unchanged (non-Globular)
//   - `# WorkingDirectory=/var/lib/globular/x`  → unchanged (commented out)
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
		// Already optional? leave alone.
		if len(val) > 0 && val[0] == '-' {
			continue
		}
		// Only rewrite Globular state-root paths.
		if !bytes.HasPrefix(val, []byte(stateRootPrefix)) {
			continue
		}
		// Preserve any leading whitespace from the original line.
		leadLen := len(line) - len(trimmed)
		newLine := make([]byte, 0, leadLen+len("WorkingDirectory=-")+len(val))
		newLine = append(newLine, line[:leadLen]...)
		newLine = append(newLine, []byte("WorkingDirectory=-")...)
		newLine = append(newLine, val...)
		lines[i] = newLine
		changed = true
	}
	if !changed {
		return content
	}
	return bytes.Join(lines, []byte{'\n'})
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
