package pkgpack

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Build-time validation of an explicitly declared identity block.
//
// A package may declare `identity.proof: binary_sha256` while shipping no
// entrypoint of its own — the payload arrives via a bundled .deb, an OS
// package, or a script. That is a legitimate and useful shape, but it is a
// PROMISE: it tells the node-agent "re-hash the binary at this path and compare
// it to this checksum". If the archive carries the promise without the subject,
// the node-agent has nothing to verify and the package can be recorded as
// installed while the binary it claims to identify was never checked.
//
// So the promise is validated here, before the archive is created. An artifact
// containing an unfulfillable identity declaration must never exist.

// ProofBinarySHA256 re-hashes the installed binary and compares it to the
// declared checksum.
const ProofBinarySHA256 = "binary_sha256"

// ProofVersion proves identity by the installed version rather than by bytes.
const ProofVersion = "version"

var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// normalizeIdentityChecksum accepts an optional "sha256:" prefix and returns
// the canonical prefixed form. Returns "" when the value is not a valid
// SHA-256.
func normalizeIdentityChecksum(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	v = strings.TrimPrefix(v, "sha256:")
	if !sha256HexRe.MatchString(v) {
		return ""
	}
	return "sha256:" + v
}

// normalizeIdentityInstalledPath returns the cleaned absolute path, or "" when
// the value is unusable. Rejects relative paths and anything whose cleaned form
// still escapes or is ambiguous.
func normalizeIdentityInstalledPath(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	// Reject before cleaning: path.Clean would silently resolve "/usr/../etc"
	// into a different, plausible-looking path, turning an ambiguous
	// declaration into a confident wrong answer.
	if strings.Contains(v, "..") {
		return ""
	}
	if !strings.HasPrefix(v, "/") {
		return ""
	}
	cleaned := path.Clean(v)
	if cleaned == "/" || strings.HasSuffix(v, "/") {
		// A directory is not a hashable subject.
		return ""
	}
	return cleaned
}

// validateDeclaredIdentity checks an explicit identity block against the
// package's layout. noEntrypoint reports whether the package ships its own
// entrypoint binary.
//
// Deliberately does NOT require the file to exist on the build machine: the
// whole point of this shape is that the payload is installed later, on the
// target node, by a mechanism the build never runs.
func validateDeclaredIdentity(id *PackageIdentity, noEntrypoint bool) error {
	if id == nil {
		return nil
	}
	proof := strings.ToLower(strings.TrimSpace(id.Proof))
	if proof == "" {
		return fmt.Errorf("identity block declares no proof mode (want %q or %q)",
			ProofBinarySHA256, ProofVersion)
	}

	switch proof {
	case ProofVersion:
		// Version proof carries no binary subject; nothing further to validate.
		return nil

	case ProofBinarySHA256:
		if normalizeIdentityChecksum(id.Checksum) == "" {
			if strings.TrimSpace(id.Checksum) == "" {
				return fmt.Errorf("identity.proof=%s requires identity.checksum", ProofBinarySHA256)
			}
			return fmt.Errorf("identity.checksum %q is not a valid sha256 (want 64 hex chars, optional %q prefix)",
				id.Checksum, "sha256:")
		}
		if noEntrypoint {
			// No shipped entrypoint means the node-agent cannot derive the
			// subject from the archive; the spec must name it.
			raw := strings.TrimSpace(id.InstalledPath)
			if raw == "" {
				return fmt.Errorf("identity.proof=%s with entrypoint: none requires identity.installed_path "+
					"(the node-agent has no shipped binary to derive the subject from)", ProofBinarySHA256)
			}
			if normalizeIdentityInstalledPath(raw) == "" {
				return fmt.Errorf("identity.installed_path %q must be an absolute path to a file, "+
					"with no %q segments", raw, "..")
			}
		} else if raw := strings.TrimSpace(id.InstalledPath); raw != "" {
			// Optional for entrypoint-bearing packages, but if declared it must
			// still be well-formed rather than silently ignored.
			if normalizeIdentityInstalledPath(raw) == "" {
				return fmt.Errorf("identity.installed_path %q must be an absolute path to a file, "+
					"with no %q segments", raw, "..")
			}
		}
		return nil

	default:
		// Fail closed. An unrecognized mode must never fall through to legacy
		// entrypoint behavior, which would silently grant a weaker proof than
		// the package asked for.
		return fmt.Errorf("identity.proof %q is not a supported mode (want %q or %q)",
			id.Proof, ProofBinarySHA256, ProofVersion)
	}
}
