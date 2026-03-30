package pkgpack

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DownloadDebs uses apt-get download to fetch .deb files for the listed
// packages and all their dependencies into outDir. Returns the paths of
// downloaded .deb files.
//
// This runs at build time on a machine with the right apt sources configured.
// The resulting .deb files are bundled into the package artifact so that
// install-time doesn't need internet access.
func DownloadDebs(packages []string, outDir string) ([]string, error) {
	if len(packages) == 0 {
		return nil, nil
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create debs dir: %w", err)
	}

	log.Printf("  downloading .deb files for: %s", strings.Join(packages, ", "))

	// Use apt-get download to fetch the packages themselves.
	// Then use apt-cache depends to find all dependencies and download those too.
	allPkgs, err := resolveDebDependencies(packages)
	if err != nil {
		// Fall back to just the listed packages if dependency resolution fails.
		log.Printf("  WARN: dependency resolution failed (%v), downloading listed packages only", err)
		allPkgs = packages
	}

	// Download all packages into outDir.
	args := append([]string{"download"}, allPkgs...)
	cmd := exec.Command("apt-get", args...)
	cmd.Dir = outDir
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("apt-get download: %v\n%s", err, string(out))
	}

	// Collect downloaded .deb paths.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".deb") {
			paths = append(paths, filepath.Join(outDir, e.Name()))
		}
	}

	log.Printf("  downloaded %d .deb files (%d packages resolved)", len(paths), len(allPkgs))
	return paths, nil
}

// resolveDebDependencies uses apt-cache to find all transitive dependencies
// for the given packages. Returns a deduplicated list of all package names.
func resolveDebDependencies(packages []string) ([]string, error) {
	args := append([]string{"depends", "--recurse", "--no-recommends", "--no-suggests", "--no-conflicts", "--no-breaks", "--no-replaces", "--no-enhances"}, packages...)
	out, err := exec.Command("apt-cache", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("apt-cache depends: %w", err)
	}

	seen := make(map[string]bool)
	for _, pkg := range packages {
		seen[pkg] = true
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Depends:") || strings.HasPrefix(line, "PreDepends:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				dep := parts[1]
				// Skip virtual packages (those with angle brackets).
				if !strings.HasPrefix(dep, "<") && !isBaseOSPackage(dep) {
					seen[dep] = true
				}
			}
		}
	}

	result := make([]string, 0, len(seen))
	for pkg := range seen {
		result = append(result, pkg)
	}
	return result, nil
}

// isBaseOSPackage returns true for packages that are part of the base Ubuntu/Debian
// installation and should NOT be bundled. Reinstalling these via dpkg -i can break
// the running system (e.g. libgcrypt20 post-install script fails, cascading to
// libsystemd0 → procps → scylla-server dependency chain).
func isBaseOSPackage(pkg string) bool {
	// Exact matches for critical system packages.
	switch pkg {
	case "libc6", "libc-bin", "libc-dev-bin", "libc6-dev", "libc6-i386", "libc6-dbg",
		"libgcrypt20", "libgpg-error0", "libsystemd0", "libsystemd-shared",
		"libudev1", "systemd", "systemd-sysv", "systemd-dev", "systemd-resolved",
		"systemd-timesyncd", "systemd-coredump",
		"init-system-helpers", "procps", "udev",
		"libcap2", "liblz4-1", "liblzma5", "libzstd1",
		"libtinfo6", "libncursesw6", "libproc2-0",
		"libgcc-s1", "gcc-14-base", "libstdc++6",
		"libatomic1", "libasan8", "libtsan2", "libubsan1", "liblsan0",
		"libgomp1", "libhwasan0", "libitm1", "libquadmath0",
		"libcc1-0", "libgfortran5", "libobjc4",
		"lib32gcc-s1", "lib32stdc++6",
		"libnss-systemd", "libpam-systemd":
		return true
	}
	// Prefix matches for library families that are always part of the base OS.
	for _, prefix := range []string{
		"libc6-", "libgcc-", "libstdc++-",
	} {
		if strings.HasPrefix(pkg, prefix) {
			return true
		}
	}
	return false
}
