package pkgpack

// Producer-side provenance proof for bundled .deb files.
//
// Why this exists
//
// Bundled debs reach the artifact from three sources (package-local debs/,
// --debs-dir, apt-get download) that all converge in Build. Until this gate,
// the package-local directory was treated as "the checked-in authoritative
// source" purely because of WHERE IT SITS ON DISK — the builder never asked
// git whether those bytes were actually checked in. Tracked, locally modified,
// and untracked-with-the-same-name files were indistinguishable.
//
// That is not hypothetical. On 2026-08-10 a joining node failed because the
// committed libnss-resolve deb pinned systemd-resolved (= 8.16) and the hosts
// had been patched. The remedy was to drop a newer 8.17 deb into the package
// directory. It was never committed. On 2026-08-14 that untracked file shipped
// and broke joins on hosts at 8.15/8.16 — the same defect with the version
// window moved, delivered by the remedy for the first occurrence. A clean
// checkout built 8.16 while the cluster installed 8.17, so the release input
// had no authority in git at all.
//
// The load-bearing assertion
//
// Not "the checkout is clean". Not "HEAD == R". Not even "the path is
// tracked". It is:
//
//	bytes actually assembled == bytes stored at path P in revision R
//
// Filesystem name equality carries no weight: the builder asks git what
// revision R says belongs at that path and compares the actual bytes to it. A
// deleted-tracked-file-plus-untracked-replacement substitution fails here even
// though the pathname is identical, which is precisely the 2026-08-14 vector.
//
// Deliberately narrow: this is NOT "build only from a clean repository", which
// would entangle unrelated source edits with package acquisition. Legitimate
// in-progress source work may coexist in a checkout. The rule is only that
// untracked or modified filesystem content must not silently substitute for a
// declared package input.
//
// Owning repo, not building repo
//
// SourceRevision identifies the revision of the repository that OWNS the deb
// (for Globular that is globulario/packages), discovered by walking up from the
// deb itself — never the repository that happens to be running the build.
// Otherwise the record is precise provenance for the wrong authority domain.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DebProvenance is the per-deb record emitted by the producer gate.
type DebProvenance struct {
	// Identity, read from the actual bundled file.
	Package      string
	Version      string
	Architecture string
	Depends      []string
	PreDepends   []string

	// SHA256 of the bytes actually assembled into the artifact.
	SHA256 string

	// Declared source: which repository, at which revision, at which path.
	SourceRepo     string
	SourceRevision string
	DeclaredSource string

	// Diagnostic evidence. Not required for the trust property — that rests on
	// the byte comparison — but it makes a refusal explainable at a glance
	// instead of "the hashes differ, good luck".
	SourceBlobOID string
	SourceSHA256  string
}

// ProvenanceError reports a deb whose assembled bytes are not the bytes its
// declared source revision holds.
type ProvenanceError struct {
	Path   string
	Reason string
	Detail string
}

func (e *ProvenanceError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("bundled deb %s: %s", e.Path, e.Reason)
	}
	return fmt.Sprintf("bundled deb %s: %s\n%s", e.Path, e.Reason, e.Detail)
}

// VerifyDebProvenance proves that debPath's bytes are the bytes its owning
// repository holds at the same path in its current revision, and returns the
// provenance record. It fails closed: any inability to establish the binding is
// a refusal, never a pass. An unknown provenance cannot improve a verdict.
func VerifyDebProvenance(debPath string) (*DebProvenance, error) {
	abs, err := filepath.Abs(debPath)
	if err != nil {
		return nil, &ProvenanceError{Path: debPath, Reason: "cannot resolve absolute path", Detail: err.Error()}
	}

	actual, err := os.ReadFile(abs)
	if err != nil {
		return nil, &ProvenanceError{Path: debPath, Reason: "cannot read bundled deb", Detail: err.Error()}
	}
	actualSum := debSHA256Hex(actual)

	// Owning repository: walk up from the deb, not from the build cwd.
	top, err := gitToplevel(filepath.Dir(abs))
	if err != nil {
		return nil, &ProvenanceError{
			Path:   debPath,
			Reason: "no owning git repository — a bundled deb must come from a declared, revision-backed source, not ambient filesystem state",
			Detail: err.Error(),
		}
	}

	rel, err := filepath.Rel(top, abs)
	if err != nil {
		return nil, &ProvenanceError{Path: debPath, Reason: "cannot compute repo-relative path", Detail: err.Error()}
	}
	rel = filepath.ToSlash(rel)

	rev, err := gitOutput(top, "rev-parse", "HEAD")
	if err != nil {
		return nil, &ProvenanceError{Path: debPath, Reason: "cannot resolve owning repository revision", Detail: err.Error()}
	}

	// The declared bytes: what revision R says belongs at P.
	declared, err := gitBlob(top, rev, rel)
	if err != nil {
		return nil, &ProvenanceError{
			Path:   debPath,
			Reason: "not present at this path in the declared source revision — untracked or newly added content must not substitute for a declared package input",
			Detail: fmt.Sprintf("repo:     %s\nrevision: %s\npath:     %s\ngit:      %v", top, rev, rel, err),
		}
	}
	declaredSum := debSHA256Hex(declared)

	if declaredSum != actualSum {
		blobOID, _ := gitOutput(top, "rev-parse", rev+":"+rel)
		return nil, &ProvenanceError{
			Path:   debPath,
			Reason: "assembled bytes differ from the declared source revision — source substitution",
			Detail: fmt.Sprintf(
				"repo:                  %s\nrevision:              %s\npath:                  %s\ndeclared revision blob: %s\ndeclared bytes sha256:  %s\nfilesystem bytes sha256: %s\nbundle refused: source substitution",
				top, rev, rel, blobOID, declaredSum, actualSum),
		}
	}

	blobOID, _ := gitOutput(top, "rev-parse", rev+":"+rel)

	prov := &DebProvenance{
		SHA256:         actualSum,
		SourceRepo:     gitRepoIdentity(top),
		SourceRevision: rev,
		DeclaredSource: rel,
		SourceBlobOID:  blobOID,
		SourceSHA256:   declaredSum,
	}
	if err := prov.readControl(abs); err != nil {
		return nil, &ProvenanceError{Path: debPath, Reason: "cannot read deb control metadata", Detail: err.Error()}
	}
	return prov, nil
}

// readControl fills identity and dependency fields from the deb's own control
// data. Read from the actual bundled file, never assumed from its filename —
// a filename is a label, the control block is the claim the package makes.
func (p *DebProvenance) readControl(debPath string) error {
	out, err := exec.Command("dpkg-deb", "-f", debPath,
		"Package", "Version", "Architecture", "Depends", "Pre-Depends").Output()
	if err != nil {
		return fmt.Errorf("dpkg-deb -f %s: %w", debPath, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "Package":
			p.Package = val
		case "Version":
			p.Version = val
		case "Architecture":
			p.Architecture = val
		case "Depends":
			p.Depends = splitDepends(val)
		case "Pre-Depends":
			p.PreDepends = splitDepends(val)
		}
	}
	if p.Package == "" {
		return fmt.Errorf("deb %s declares no Package field", debPath)
	}
	return nil
}

func splitDepends(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func debSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func gitToplevel(dir string) (string, error) {
	return gitOutput(dir, "rev-parse", "--show-toplevel")
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// gitBlob returns the bytes git holds at rel in rev. Binary-safe: no trimming.
func gitBlob(dir, rev, rel string) ([]byte, error) {
	cmd := exec.Command("git", "cat-file", "blob", rev+":"+rel)
	cmd.Dir = dir
	return cmd.Output()
}

// gitRepoIdentity prefers the origin remote so the record names the repository
// as the world knows it, falling back to the directory name.
func gitRepoIdentity(top string) string {
	if url, err := gitOutput(top, "config", "--get", "remote.origin.url"); err == nil && url != "" {
		url = strings.TrimSuffix(url, ".git")
		if i := strings.LastIndex(url, ":"); i >= 0 && !strings.Contains(url[i+1:], "/") {
			return url[i+1:]
		}
		parts := strings.Split(url, "/")
		if len(parts) >= 2 {
			return strings.Join(parts[len(parts)-2:], "/")
		}
		return url
	}
	return filepath.Base(top)
}
