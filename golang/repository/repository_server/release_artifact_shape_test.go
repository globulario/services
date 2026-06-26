package main

// CG-3 proof for invariant publish.release_artifact_must_be_stripped (strip half).
//
// The gate (release_artifact_shape.go) rejects a release-channel upload whose
// entrypoint ELF binary is unstripped. These tests prove it end-to-end against
// REAL ELF binaries produced by the Go toolchain (stripped vs unstripped), plus
// the pure section-name policy and the out-of-scope passthroughs.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// tgzWithBin packages binary bytes as an executable under bin/svc in a .tgz,
// matching the entrypoint discovery rule the gate uses.
func tgzWithBin(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: "bin/svc", Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// buildLinuxELF compiles a trivial main into a linux/amd64 ELF binary, stripped
// (-trimpath -ldflags "-s -w") or not. Skips the test if the Go toolchain is
// unavailable.
func buildLinuxELF(t *testing.T, stripped bool) []byte {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available; skipping real-ELF strip-gate test")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tinybin\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	out := filepath.Join(dir, "svc")
	args := []string{"build", "-buildvcs=false", "-o", out}
	if stripped {
		args = append(args, "-trimpath", "-ldflags", "-s -w")
	}
	args = append(args, ".")
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0", "GOFLAGS=")
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build (stripped=%v) failed: %v\n%s", stripped, err, combined)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read built binary: %v", err)
	}
	return b
}

// (positive guard) An unstripped release binary is rejected.
func TestReleaseArtifactStripped_RejectsUnstrippedELF(t *testing.T) {
	pkg := tgzWithBin(t, buildLinuxELF(t, false))
	err := validateReleaseArtifactStripped(pkg)
	if err == nil {
		t.Fatal("expected an unstripped release binary to be rejected")
	}
	if !strings.Contains(err.Error(), "stripped") {
		t.Fatalf("expected a strip-gate rejection, got: %v", err)
	}
}

// (counter-case) A properly stripped release binary passes.
func TestReleaseArtifactStripped_AcceptsStrippedELF(t *testing.T) {
	pkg := tgzWithBin(t, buildLinuxELF(t, true))
	if err := validateReleaseArtifactStripped(pkg); err != nil {
		t.Fatalf("expected a stripped release binary to pass, got: %v", err)
	}
}

// (out of scope) A non-ELF entrypoint passes — the gate is ELF-only and does
// not implicitly trust or reject formats it cannot inspect.
func TestReleaseArtifactStripped_NonELFPasses(t *testing.T) {
	pkg := tgzWithBin(t, []byte("#!/bin/sh\necho hi\n"))
	if err := validateReleaseArtifactStripped(pkg); err != nil {
		t.Fatalf("expected a non-ELF entrypoint to pass the ELF strip gate, got: %v", err)
	}
}

// (policy) The section-name policy flags a symbol table / DWARF and clears a
// stripped section set.
func TestElfDebugSectionName_Policy(t *testing.T) {
	if got := elfDebugSectionName([]string{".text", ".rodata", ".gopclntab"}); got != "" {
		t.Fatalf("stripped section set must be clean, got %q", got)
	}
	if got := elfDebugSectionName([]string{".text", ".symtab"}); got != ".symtab" {
		t.Fatalf("symbol table must be flagged, got %q", got)
	}
	if got := elfDebugSectionName([]string{".text", ".debug_info"}); got != ".debug_info" {
		t.Fatalf("DWARF must be flagged, got %q", got)
	}
}
