package actions

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// installBundledDebs installs the .deb files a package ships in its debs/
// directory, so a service that links a native library has that library present
// before anything tries to run or inspect its binary.
//
// Offline by contract: the .debs are resolved at BUILD time (bundle_debs in the
// package spec) and installed here with dpkg. There is no live apt-get fetch —
// `apt-get -f install` is only ever used to settle dependencies among the debs
// already on disk, mirroring the installer engine's install_local_debs step
// that Day-0 and the join path use for the same purpose.
//
// A missing or empty debs/ directory is a no-op: almost every Globular service
// is a static Go binary with nothing to install.
//
// Exec note: dpkg/apt-get are domain tools, not systemd unit actions, so they
// are outside the internal/supervisor allowlist (EX-2) — the same boundary the
// installer engine and the scylladb post-install script already operate under.
func installBundledDebs(ctx context.Context, debsDir, service string) error {
	debs, err := bundledDebPaths(debsDir)
	if err != nil || len(debs) == 0 {
		return nil // nothing bundled — the common case
	}

	log.Printf("install_payload: installing %d bundled .deb(s) for %s from %s", len(debs), service, debsDir)

	dctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// --force-confmiss: install config files a previous wipe removed. dpkg
	// otherwise reads "deleted" as "operator removed it deliberately" and skips
	// re-creating them.
	args := append([]string{"-i", "--force-confmiss"}, debs...)
	out, runErr := runDpkgAwaitingLock(dctx, service, args)
	if runErr != nil {
		log.Printf("install_payload: dpkg -i for %s returned %v; attempting apt-get -f install to settle dependencies\n%s",
			service, runErr, strings.TrimSpace(string(out)))
		// DPkg::Lock::Timeout makes apt wait for the dpkg lock instead of
		// failing instantly — same reason as the retry loop below.
		fix := exec.CommandContext(dctx, "apt-get",
			"-o", fmt.Sprintf("DPkg::Lock::Timeout=%d", int(dpkgLockWait.Seconds())),
			"install", "-f", "-y", "--no-install-recommends")
		fix.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		if fixOut, fixErr := fix.CombinedOutput(); fixErr != nil {
			return fmt.Errorf("dpkg -i failed (%v) and apt-get -f install failed (%v): %s",
				runErr, fixErr, strings.TrimSpace(string(fixOut)))
		}
	}

	// Verify rather than trust the exit code: dpkg can report success for a
	// package left unconfigured. Reporting an install that did not happen is
	// what the ldd preflight downstream exists to catch — do not hand it a lie.
	var missing []string
	for _, deb := range debs {
		pkg := debPackageNameFromPath(deb)
		if pkg != "" && !debIsInstalled(dctx, pkg) {
			missing = append(missing, pkg)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("packages not installed after dpkg: %s", strings.Join(missing, ", "))
	}

	log.Printf("install_payload: %d bundled .deb(s) installed for %s", len(debs), service)
	return nil
}

// dpkgLockWait bounds how long an install waits for another dpkg to finish.
// The workflow installs a node's workload packages concurrently, so contention
// is normal, not exceptional.
const dpkgLockWait = 3 * time.Minute

// dpkgLockBusy reports whether output is dpkg/apt refusing to start because
// another process holds the packaging lock. That is a "try again shortly",
// not a broken package.
func dpkgLockBusy(output string) bool {
	s := strings.ToLower(output)
	return strings.Contains(s, "could not get lock") ||
		strings.Contains(s, "dpkg frontend lock") ||
		strings.Contains(s, "lock-frontend") ||
		strings.Contains(s, "is held by process") ||
		strings.Contains(s, "temporarily unavailable") ||
		strings.Contains(s, "another process is using")
}

// runDpkgAwaitingLock runs dpkg, retrying while the packaging lock is held.
//
// dpkg has no --wait: it exits immediately when another process holds
// /var/lib/dpkg/lock-frontend. node.join installs the compute workload packages
// concurrently, so a bundled-deb install routinely lands on top of another
// package's dpkg run. Failing on that first collision took down the whole
// install_workloads_compute step and therefore node.join, and left sql at
// status=artifact_present with no install receipt while its six sibling
// packages installed cleanly — surfacing as the doctor's
// unit_receipt_drift.installed_state_missing_or_unproven CRITICAL for
// globular-sql.service. Observed 2026-08-11 on node-4:
//
//	dpkg -i failed (exit status 2) and apt-get -f install failed (exit
//	status 100): E: Could not get lock /var/lib/dpkg/lock-frontend.
//	It is held by process 5220 (dpkg)
//
// Only lock contention is retried. A genuinely broken package still fails on
// the first attempt, so this cannot mask a real install error.
func runDpkgAwaitingLock(ctx context.Context, service string, args []string) ([]byte, error) {
	deadline := time.Now().Add(dpkgLockWait)
	var out []byte
	var err error
	for attempt := 1; ; attempt++ {
		cmd := exec.CommandContext(ctx, "dpkg", args...)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		out, err = cmd.CombinedOutput()
		if err == nil || !dpkgLockBusy(string(out)) || time.Now().After(deadline) {
			return out, err
		}
		log.Printf("install_payload: dpkg for %s blocked on the packaging lock (attempt %d); retrying in 5s",
			service, attempt)
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// bundledDebPaths returns the .deb files in dir, sorted so a run is
// reproducible and dpkg sees a stable ordering.
func bundledDebPaths(dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var debs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".deb") {
			debs = append(debs, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(debs)
	return debs, nil
}

// debPackageNameFromPath extracts the package name from a .deb filename,
// e.g. "libodbc2_2.3.12-1ubuntu0.24.04.1_amd64.deb" → "libodbc2".
func debPackageNameFromPath(debPath string) string {
	base := filepath.Base(debPath)
	name, _, found := strings.Cut(base, "_")
	if !found {
		return ""
	}
	return name
}

// debIsInstalled reports whether dpkg considers pkg fully installed.
func debIsInstalled(ctx context.Context, pkg string) bool {
	out, err := exec.CommandContext(ctx, "dpkg", "-s", pkg).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Status: install ok installed")
}
