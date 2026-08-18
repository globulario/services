package actions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A SERVICE package that links a native library ships the providing .deb in
// debs/. install_payload used to extract bin/, systemd/, config/, scripts/,
// data/ and policy/ and silently drop debs/, so the library never arrived and
// the ldd preflight aborted the very install it exists to protect.
//
// Observed on every compute-profile join (2026-08-10): sql shipped
// libodbc2 + libltdl7 in debs/, yet install_workloads_compute failed with
// NATIVE_LIBRARY_DEPENDENCY_MISSING for libodbc.so.2. node.join failed on
// node-4/node-5, leaving globular-sql.service on disk with no installed_state
// receipt — cluster-doctor unit_receipt_drift (CRITICAL) plus
// installed_state_runtime_mismatch (WARN) that no convergence pass could clear.
// (invariant:install.join_path_must_complete_install_contract)

// TestInstallPayloadExtractsAndInstallsBundledDebs pins both halves in the
// source: debs/ must be extracted, and the install must happen BEFORE the
// native-library preflight that would otherwise reject the install.
func TestInstallPayloadExtractsAndInstallsBundledDebs(t *testing.T) {
	src, err := os.ReadFile("artifact.go")
	if err != nil {
		t.Fatalf("read artifact.go: %v", err)
	}
	body := string(src)

	// The entry-kind mapping moved out of an inline switch in artifact.go and
	// into the extraction planner, so that every destination inherits the same
	// path-containment enforcement. The requirement is unchanged — debs/ must be
	// extracted — so it is asserted where that decision now lives rather than at
	// the file it used to live in.
	safety, err := os.ReadFile("archive_safety.go")
	if err != nil {
		t.Fatalf("read archive_safety.go: %v", err)
	}
	if !strings.Contains(string(safety), `strings.HasPrefix(name, "debs/")`) {
		t.Fatal(`install_payload must extract the package's debs/ entries: a bundled native-library .deb that is never unpacked cannot satisfy the ldd preflight`)
	}
	// And the planner must actually be given somewhere to put them, or the
	// entries classify as debs and then map to an empty destination.
	if !strings.Contains(body, "debsDir") {
		t.Fatal("install_payload must stage a debs/ directory and pass it to the extraction planner")
	}

	install := strings.Index(body, "installBundledDebs(")
	preflight := strings.Index(body, "MissingNativeLibs(")
	if install < 0 {
		t.Fatal("install_payload must install the bundled debs (installBundledDebs call missing)")
	}
	if preflight >= 0 && install > preflight {
		t.Fatal("installBundledDebs must run BEFORE the MissingNativeLibs preflight — otherwise the preflight rejects the install whose dependency the debs were meant to provide")
	}
}

// TestBundledDebPathsSelectsAndSorts pins the selection rule: only .deb files,
// in a stable order (dpkg is handed them in one call, so ordering must not
// vary run to run).
func TestBundledDebPathsSelectsAndSorts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"libodbc2_2.3.12-1ubuntu0.24.04.1_amd64.deb",
		"libltdl7_2.4.7-7build1_amd64.deb",
		"README.txt",
		"notes.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := bundledDebPaths(dir)
	if err != nil {
		t.Fatalf("bundledDebPaths: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly the 2 .deb files, got %v", got)
	}
	if filepath.Base(got[0]) != "libltdl7_2.4.7-7build1_amd64.deb" {
		t.Errorf("expected sorted order, got %v", got)
	}
}

// TestBundledDebPathsMissingDirIsNoOp pins that the overwhelmingly common case
// — a pure-Go service with no debs/ — is silent and non-fatal.
func TestBundledDebPathsMissingDirIsNoOp(t *testing.T) {
	if debs, _ := bundledDebPaths(filepath.Join(t.TempDir(), "absent")); len(debs) != 0 {
		t.Fatalf("a missing debs/ dir must yield no debs, got %v", debs)
	}
	if err := installBundledDebs(context.Background(), filepath.Join(t.TempDir(), "absent"), "echo"); err != nil {
		t.Fatalf("a service with no bundled debs must install cleanly, got %v", err)
	}
	if err := installBundledDebs(context.Background(), "", "echo"); err != nil {
		t.Fatalf("an empty debs dir path must be a no-op, got %v", err)
	}
}

func TestDebPackageNameFromPath(t *testing.T) {
	cases := map[string]string{
		"libodbc2_2.3.12-1ubuntu0.24.04.1_amd64.deb": "libodbc2",
		"/tmp/staging/debs/libltdl7_2.4.7_amd64.deb": "libltdl7",
		"noseparator.deb": "",
	}
	for in, want := range cases {
		if got := debPackageNameFromPath(in); got != want {
			t.Errorf("debPackageNameFromPath(%q)=%q want %q", in, got, want)
		}
	}
}

// node.join installs a compute node's workload packages concurrently, so a
// bundled-deb install routinely collides with another package's dpkg run.
// dpkg has no --wait and exits immediately when the lock is held; treating
// that first collision as a hard failure took down install_workloads_compute
// (and therefore node.join), leaving sql at status=artifact_present with no
// install receipt while its six siblings installed cleanly.
func TestDpkgLockContentionIsRecognisedAsRetryable(t *testing.T) {
	busy := []string{
		"E: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 5220 (dpkg)",
		"dpkg: error: dpkg frontend lock is locked by another process",
		"E: Could not get lock /var/lib/dpkg/lock - open (11: Resource temporarily unavailable)",
		"Waiting for cache lock: Another process is using it",
	}
	for _, out := range busy {
		if !dpkgLockBusy(out) {
			t.Errorf("lock contention not recognised, so the install fails instead of waiting:\n  %s", out)
		}
	}
}

// A genuinely broken package must still fail on the first attempt — retrying
// real errors would turn a fast failure into a multi-minute stall and could
// mask a package that can never install.
func TestRealDpkgErrorsAreNotTreatedAsLockContention(t *testing.T) {
	real := []string{
		"dpkg: error processing archive /tmp/x.deb (--install): cannot access archive: No such file or directory",
		"dpkg: dependency problems prevent configuration of libodbc2",
		"dpkg: error: cannot scan updates directory",
		"",
	}
	for _, out := range real {
		if dpkgLockBusy(out) {
			t.Errorf("real dpkg error misread as lock contention (would retry for minutes):\n  %s", out)
		}
	}
}

func TestDpkgLockWaitIsBounded(t *testing.T) {
	if dpkgLockWait <= 0 {
		t.Fatal("lock wait must be positive")
	}
	if dpkgLockWait > 10*time.Minute {
		t.Fatalf("lock wait %v is too long — a stuck dpkg would hold up the whole join", dpkgLockWait)
	}
}
