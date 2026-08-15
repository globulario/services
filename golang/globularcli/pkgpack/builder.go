package pkgpack

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/globulario/services/golang/versionutil"
)

type BuildOptions struct {
	SpecPath           string
	SpecDir            string
	AssetsDir          string
	InstallerRoot      string
	Root               string
	BinDir             string
	ConfigDir          string
	ScriptsDir         string
	Version            string
	BuildNumber        int64
	Publisher          string
	Platform           string
	OutDir             string
	SkipMissingConfig  bool
	SkipMissingSystemd bool
	DebsDir            string // pre-downloaded .deb directory; skips apt-get download when set

	// PlatformBaselinePath declares the target platform bundled debs must be
	// installable on. Empty means "not declared", which refuses assembly of any
	// package that bundles debs rather than assembling one whose installability
	// nobody checked.
	PlatformBaselinePath string

	// AllowUnprovenDebProvenance is a deliberate, explicit escape hatch for
	// bootstrapping a package whose deb is not yet revision-backed. It is not a
	// default and not a fallback: it must be passed by a human who has read the
	// refusal. Leaving it off is what keeps ambient filesystem state out of a
	// release.
	AllowUnprovenDebProvenance bool
}

type BuildResult struct {
	SpecPath   string
	Service    string
	OutputPath string
	Err        error
}

func BuildPackages(opts BuildOptions) ([]BuildResult, error) {
	if err := ValidateVersionBuildSemantics(opts.Version, opts.BuildNumber); err != nil {
		return nil, err
	}
	canonical, _ := versionutil.NormalizeExact(opts.Version)
	opts.Version = canonical
	if opts.OutDir == "" {
		return nil, fmt.Errorf("out directory is required")
	}
	if (opts.SpecPath == "" && opts.SpecDir == "") || (opts.SpecPath != "" && opts.SpecDir != "") {
		return nil, fmt.Errorf("spec or spec-dir must be set")
	}
	rootMode := opts.Root != ""
	explicitMode := opts.BinDir != "" || opts.ConfigDir != ""
	installerMode := opts.InstallerRoot != "" || opts.AssetsDir != ""
	modeCount := 0
	for _, active := range []bool{rootMode, explicitMode, installerMode} {
		if active {
			modeCount++
		}
	}
	if modeCount == 0 {
		return nil, fmt.Errorf("one of installer-root/assets, root, or bin-dir+config-dir is required")
	}
	if modeCount > 1 {
		return nil, fmt.Errorf("choose only one of installer-root/assets, root, or bin-dir+config-dir")
	}
	if explicitMode && (opts.BinDir == "" || opts.ConfigDir == "") {
		return nil, fmt.Errorf("bin-dir and config-dir must both be set when using explicit roots")
	}

	if opts.InstallerRoot != "" {
		if opts.AssetsDir == "" {
			opts.AssetsDir = filepath.Join(opts.InstallerRoot, "internal", "assets")
		} else if !filepath.IsAbs(opts.AssetsDir) {
			opts.AssetsDir = filepath.Join(opts.InstallerRoot, opts.AssetsDir)
		}
		if opts.SpecDir != "" && !filepath.IsAbs(opts.SpecDir) {
			opts.SpecDir = filepath.Join(opts.InstallerRoot, opts.SpecDir)
		}
		if opts.SpecPath != "" && !filepath.IsAbs(opts.SpecPath) {
			opts.SpecPath = filepath.Join(opts.InstallerRoot, opts.SpecPath)
		}
	}

	var binRoot, configRoot string
	switch {
	case rootMode:
		binRoot = opts.BinDir
		if binRoot == "" {
			binRoot = filepath.Join(opts.Root, "bin")
		}
		configRoot = opts.ConfigDir
		if configRoot == "" {
			configRoot = filepath.Join(opts.Root, "config")
		}
	case explicitMode:
		binRoot = opts.BinDir
		configRoot = opts.ConfigDir
	default:
		if opts.AssetsDir == "" {
			return nil, fmt.Errorf("assets directory is required (use --assets or --installer-root)")
		}
		binRoot = filepath.Join(opts.AssetsDir, "bin")
		configRoot = filepath.Join(opts.AssetsDir, "config")
	}
	if opts.Publisher == "" {
		opts.Publisher = "core@globular.io"
	}

	goos, goarch, err := resolvePlatform(opts.Platform)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.OutDir, 0755); err != nil {
		return nil, err
	}

	specs, err := collectSpecPaths(opts.SpecPath, opts.SpecDir)
	if err != nil {
		return nil, err
	}

	var results []BuildResult
	var hadErr bool
	for _, spec := range specs {
		res := BuildResult{SpecPath: spec}
		scriptsRoot := opts.ScriptsDir
		if scriptsRoot == "" && opts.Root != "" {
			// Auto-discover scripts from root/scripts/ if present.
			candidate := filepath.Join(opts.Root, "scripts")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				scriptsRoot = candidate
			}
		}
		debsRoot := ""
		if opts.Root != "" {
			debsRoot = filepath.Join(opts.Root, "debs")
		}
		roots := AssetRoots{BinRoot: binRoot, ConfigRoot: configRoot, ScriptsRoot: scriptsRoot, DebsRoot: debsRoot}
		info, err := ScanSpec(spec, roots, ScanOptions{SkipMissingConfig: opts.SkipMissingConfig, SkipMissingSystemd: opts.SkipMissingSystemd})
		if err != nil {
			res.Err = err
			results = append(results, res)
			fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", spec, err)
			hadErr = true
			continue
		}
		res.Service = info.ServiceName

		// Resolve .deb packages if bundle_debs is set in the spec metadata.
		// A package-local root/debs directory takes precedence and acts as the
		// checked-in authoritative source, matching the scylladb package layout.
		if len(info.Metadata.BundleDebs) > 0 && len(info.DebPaths) == 0 {
			if opts.DebsDir != "" {
				// Use pre-downloaded debs; skip apt-get download.
				debPaths, err := collectPrebuiltDebs(opts.DebsDir, goarch)
				if err != nil {
					res.Err = fmt.Errorf("collect prebuilt debs from %s: %w", opts.DebsDir, err)
					results = append(results, res)
					fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", info.ServiceName, err)
					hadErr = true
					continue
				}
				log.Printf("  using %d pre-downloaded .deb files from %s", len(debPaths), opts.DebsDir)
				info.DebPaths = debPaths
			} else {
				debDir, err := os.MkdirTemp("", "pkg-debs-")
				if err != nil {
					res.Err = fmt.Errorf("create debs temp dir: %w", err)
					results = append(results, res)
					hadErr = true
					continue
				}
				debPaths, err := DownloadDebs(info.Metadata.BundleDebs, debDir)
				if err != nil {
					os.RemoveAll(debDir)
					res.Err = fmt.Errorf("download debs: %w", err)
					results = append(results, res)
					fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", info.ServiceName, err)
					hadErr = true
					continue
				}
				info.DebPaths = filterDebsForArch(debPaths, goarch)
				defer os.RemoveAll(debDir)
			}
		}

		// Single choke point for the architecture filter. Applied here rather
		// than only inside the collectors because a package-local root/debs
		// directory (the scylladb layout) is scanned by specscan and arrives
		// with DebPaths already populated, which skips both collectors
		// entirely — that is how 12 i386 .debs kept shipping after the
		// collectors were filtered. Whatever the source, nothing foreign-arch
		// reaches the artifact.
		info.DebPaths = filterDebsForArch(info.DebPaths, goarch)

		// Same choke point, two further proofs — for the same reason the arch
		// filter lives here rather than in the collectors: a package-local
		// debs/ directory arrives with DebPaths already populated and skips
		// both collectors entirely.
		//
		// PRODUCER: the assembled bytes must be the bytes the owning repository
		// holds at that path in its revision. Not "the checkout is clean", not
		// "the path is tracked" — the bytes. This is what makes a deleted
		// tracked file plus an untracked replacement at the same pathname a
		// refusal instead of a silent substitution.
		//
		// CONSUMER: those bytes must be installable on the declared target
		// baseline. Independent property — wrong bytes can be perfectly
		// installable, which is how the 2026-08-14 deb passed unnoticed on the
		// builder that produced it.
		if len(info.DebPaths) > 0 {
			if err := verifyBundledDebs(info.DebPaths, opts); err != nil {
				res.Err = err
				results = append(results, res)
				fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", info.ServiceName, err)
				hadErr = true
				continue
			}
		}

		archiveName := buildArchiveName(info.ServiceName, opts.Version, goos, goarch)
		outputPath := filepath.Join(opts.OutDir, archiveName)
		summary, err := BuildPackage(info, opts, outputPath, goos, goarch)
		res.OutputPath = outputPath
		res.Err = err
		if err != nil {
			fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", info.ServiceName, err)
			hadErr = true
		} else {
			fmt.Fprintf(os.Stdout, "[OK] %s -> %s\n", info.ServiceName, outputPath)
			if summary != nil {
				fmt.Fprintf(os.Stdout, "  manifest: name=%s version=%s platform=%s entrypoint=%s configs=%d systemd=%d scripts=%d\n",
					summary.Name, summary.Version, summary.Platform, summary.Entrypoint, summary.ConfigCount, summary.SystemdCount, summary.ScriptsCount)
			}
		}
		results = append(results, res)
	}

	if hadErr {
		return results, fmt.Errorf("one or more packages failed")
	}
	return results, nil
}

func BuildPackage(info *SpecInfo, opts BuildOptions, outputPath, goos, goarch string) (*VerificationSummary, error) {
	stagingDir, err := os.MkdirTemp(opts.OutDir, ".pkg-staging-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stagingDir)

	if err := os.MkdirAll(filepath.Join(stagingDir, "bin"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(stagingDir, "specs"), 0755); err != nil {
		return nil, err
	}

	// Binary-less packages (entrypoint: none) bundle no Globular executable — skip
	// the entrypoint copy entirely. Their install is proven by other means
	// (OS package / .deb / fetch-at-install), not a Go binary hash.
	execDest := ""
	if !info.NoEntrypoint {
		execDest = filepath.Join(stagingDir, "bin", info.ExecName)
		if err := copyFile(info.ExecPath, execDest); err != nil {
			return nil, err
		}
		if err := os.Chmod(execDest, 0755); err != nil {
			return nil, err
		}
	}

	// Copy extra binaries (e.g. helper tools bundled with the package).
	for _, extra := range info.ExtraBinaries {
		dest := filepath.Join(stagingDir, "bin", extra.Name)
		if err := copyFile(extra.Path, dest); err != nil {
			return nil, fmt.Errorf("extra binary %s: %w", extra.Name, err)
		}
		if err := os.Chmod(dest, 0755); err != nil {
			return nil, err
		}
	}

	copiedConfig := 0
	if len(info.ConfigDirs) > 0 {
		configRoot := filepath.Join(stagingDir, "config", info.ServiceName)
		var err error
		copiedConfig, err = copyConfigDirs(info.ConfigDirs, configRoot)
		if err != nil {
			return nil, err
		}
		if copiedConfig == 0 {
			_ = os.RemoveAll(filepath.Join(stagingDir, "config"))
		}
	}

	specDest := filepath.Join(stagingDir, "specs", info.SpecFile)
	if err := copyFile(info.SpecPath, specDest); err != nil {
		return nil, err
	}
	if err := os.Chmod(specDest, 0644); err != nil {
		return nil, err
	}

	if len(info.Systemd) > 0 {
		systemdRoot := filepath.Join(stagingDir, "systemd")
		if err := os.MkdirAll(systemdRoot, 0755); err != nil {
			return nil, err
		}
		for _, unit := range info.Systemd {
			target := filepath.Join(systemdRoot, unit.Name)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, err
			}
			if len(unit.Content) > 0 {
				if err := os.WriteFile(target, unit.Content, 0644); err != nil {
					return nil, err
				}
			} else if unit.SourcePath != "" {
				if err := copyFile(unit.SourcePath, target); err != nil {
					return nil, err
				}
			}
			if err := os.Chmod(target, 0644); err != nil {
				return nil, err
			}
		}
	}

	if len(info.Scripts) > 0 {
		scriptsRoot := filepath.Join(stagingDir, "scripts")
		if err := os.MkdirAll(scriptsRoot, 0755); err != nil {
			return nil, err
		}
		for _, script := range info.Scripts {
			target := filepath.Join(scriptsRoot, script.Name)
			if err := copyFile(script.SourcePath, target); err != nil {
				return nil, err
			}
			if err := os.Chmod(target, 0755); err != nil {
				return nil, err
			}
		}
	}

	// Bundle data directory (e.g. workflow definitions).
	if info.DataDir != "" {
		dataRoot := filepath.Join(stagingDir, "data")
		if err := copyDir(info.DataDir, dataRoot); err != nil {
			return nil, fmt.Errorf("bundle data dir: %w", err)
		}
		log.Printf("  bundled data/ directory from %s", info.DataDir)
	}

	// Bundle .deb files for offline installation.
	if len(info.DebPaths) > 0 {
		debsRoot := filepath.Join(stagingDir, "debs")
		if err := os.MkdirAll(debsRoot, 0755); err != nil {
			return nil, err
		}
		for _, debPath := range info.DebPaths {
			target := filepath.Join(debsRoot, filepath.Base(debPath))
			if err := copyFile(debPath, target); err != nil {
				return nil, fmt.Errorf("bundle deb %s: %w", debPath, err)
			}
		}
		log.Printf("  bundled %d .deb files in debs/", len(info.DebPaths))
	}

	// Bundle generated per-service authorization policy when the payload root
	// already staged it under policy/. This is the contract consumed by the
	// node-agent install path, which deploys these files to
	// /var/lib/globular/policy/services/<service>/.
	if opts.Root != "" {
		policyRoot := filepath.Join(opts.Root, "policy")
		if info, err := os.Stat(policyRoot); err == nil && info.IsDir() {
			targetRoot := filepath.Join(stagingDir, "policy")
			if err := copyDir(policyRoot, targetRoot); err != nil {
				return nil, fmt.Errorf("bundle policy dir: %w", err)
			}
			log.Printf("  bundled policy/ directory from %s", policyRoot)
		}
	}

	pkgType := info.Metadata.Kind
	if pkgType == "" {
		pkgType = "service"
	}

	// Auto-derive systemd unit name from the first .service file if not set in metadata.
	systemdUnit := info.Metadata.SystemdUnit
	if systemdUnit == "" && len(info.Systemd) > 0 {
		for _, u := range info.Systemd {
			if strings.HasSuffix(u.Name, ".service") {
				systemdUnit = u.Name
				break
			}
		}
	}

	// Auto-derive health check unit from systemd unit if health check has no unit set.
	healthCheckUnit := ""
	healthCheckPort := 0
	if info.Metadata.HealthCheck != nil {
		healthCheckUnit = info.Metadata.HealthCheck.Unit
		healthCheckPort = info.Metadata.HealthCheck.Port
	}
	if healthCheckUnit == "" && systemdUnit != "" {
		healthCheckUnit = systemdUnit
	}

	// Compute SHA256 of the entrypoint binary for reverse-lookup. Binary-less
	// packages (entrypoint: none) carry entrypoint "none" and an EMPTY checksum;
	// the node-agent verifier reads "none" and returns BinaryNotApplicable.
	manifestEntrypoint := "none"
	manifestEntrypointChecksum := ""
	manifestIdentityProof := ""
	manifestIdentityInstalledPath := ""
	// An explicit identity block is a PROMISE the node-agent must be able to
	// keep. Validate before writing the archive so an artifact carrying an
	// unfulfillable declaration never exists.
	if err := validateDeclaredIdentity(info.Metadata.Identity, info.NoEntrypoint); err != nil {
		return nil, fmt.Errorf("package %s: %w", info.ServiceName, err)
	}
	if info.Metadata.Identity != nil {
		manifestIdentityProof = strings.ToLower(strings.TrimSpace(info.Metadata.Identity.Proof))
		// Verbatim from the spec — never derived from the package name,
		// entrypoint, unit, or anything discovered on the build host.
		manifestIdentityInstalledPath = normalizeIdentityInstalledPath(info.Metadata.Identity.InstalledPath)
	} else if !info.NoEntrypoint {
		// Shipped-binary packages are binary_sha256 by construction even without an
		// explicit identity block (the checksum below is the proof).
		manifestIdentityProof = "binary_sha256"
	}
	if !info.NoEntrypoint {
		entrypointChecksum, err := sha256File(execDest)
		if err != nil {
			return nil, fmt.Errorf("checksum entrypoint binary: %w", err)
		}
		manifestEntrypoint = path.Join("bin", info.ExecName)
		manifestEntrypointChecksum = "sha256:" + entrypointChecksum
	} else if id := info.Metadata.Identity; id != nil && strings.EqualFold(strings.TrimSpace(id.Proof), ProofBinarySHA256) {
		// Noop package (curl/wrapper, .deb, OS-repo) that DECLARES a binary_sha256
		// identity: the build never sees the installed binary, so the manifest
		// carries the package's DECLARED pinned checksum verbatim. The node-agent
		// re-hashes the installed binary at identity.installed_path and compares it
		// against this value — a single declared canonical identity, never one
		// recovered by hashing whatever happens to be on disk
		// (forbidden_fix:recompute_identity_from_secondary_source).
		manifestEntrypointChecksum = normalizeIdentityChecksum(id.Checksum)
	}

	manifest := Manifest{
		Type:                  pkgType,
		Name:                  info.ServiceName,
		Version:               opts.Version,
		BuildNumber:           opts.BuildNumber,
		Platform:              fmt.Sprintf("%s_%s", goos, goarch),
		Publisher:             opts.Publisher,
		Entrypoint:            manifestEntrypoint,
		EntrypointChecksum:    manifestEntrypointChecksum,
		IdentityProof:         manifestIdentityProof,
		IdentityInstalledPath: manifestIdentityInstalledPath,
		Defaults: ManifestDefault{
			ConfigDir: "",
			Spec:      path.Join("specs", info.SpecFile),
		},
		Description: info.Metadata.Description,
		Keywords:    info.Metadata.Keywords,
		License:     info.Metadata.License,
		Channel:     info.Metadata.Channel,

		// Catalog metadata from spec.
		Profiles:             info.Metadata.Profiles,
		Priority:             info.Metadata.Priority,
		InstallMode:          info.Metadata.InstallMode,
		ManagedUnit:          info.Metadata.ManagedUnit,
		SystemdUnit:          systemdUnit,
		ProvidesCapabilities: info.Metadata.ProvidesCapabilities,
		HealthCheckUnit:      healthCheckUnit,
		HealthCheckPort:      healthCheckPort,

		// Typed dependency declarations.
		HardDeps:    info.Metadata.HardDeps,
		RuntimeUses: info.Metadata.RuntimeUses,
	}
	if copiedConfig > 0 {
		manifest.Defaults.ConfigDir = path.Join("config", info.ServiceName)
	}
	if len(info.Scripts) > 0 {
		manifest.Defaults.ScriptsDir = "scripts"
	}
	if err := WriteManifest(filepath.Join(stagingDir, "package.json"), manifest); err != nil {
		return nil, err
	}

	if err := WriteTgz(outputPath, stagingDir); err != nil {
		return nil, err
	}

	if err := assertPackageGuards(outputPath, info); err != nil {
		return nil, err
	}

	return VerifyTGZ(outputPath)
}

// assertPackageGuards ensures critical payloads are present to prevent broken packages.
func assertPackageGuards(pkgPath string, info *SpecInfo) error {
	// 1) binary present — skipped for binary-less packages (entrypoint: none),
	// which intentionally bundle no Globular executable.
	if !info.NoEntrypoint {
		wantBin := filepath.ToSlash(filepath.Join("bin", info.ExecName))
		if ok, err := tgzContains(pkgPath, wantBin); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("package %s missing binary %s", pkgPath, wantBin)
		}
	}

	// 2) spec present and contains install_package_payload
	specEntry := filepath.ToSlash(filepath.Join("specs", info.SpecFile))
	specData, err := readEntryFromTgz(pkgPath, specEntry)
	if err != nil {
		return fmt.Errorf("read spec from package: %w", err)
	}
	if !strings.Contains(string(specData), "install_package_payload") {
		return fmt.Errorf("package %s spec %s missing install_package_payload", pkgPath, specEntry)
	}
	return nil
}

func tgzContains(pkgPath, entry string) (bool, error) {
	f, err := os.Open(pkgPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return false, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		name := filepath.ToSlash(hdr.Name)
		if name == entry {
			return true, nil
		}
	}
}

func readEntryFromTgz(pkgPath, entry string) ([]byte, error) {
	f, err := os.Open(pkgPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("entry %s not found", entry)
		}
		if err != nil {
			return nil, err
		}
		name := filepath.ToSlash(hdr.Name)
		if name == entry {
			data, err := io.ReadAll(tr)
			return data, err
		}
	}
}

// tgzContainsPrefix returns true if any entry starts with the given prefix.
func tgzContainsPrefix(pkgPath, prefix string) (bool, error) {
	f, err := os.Open(pkgPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return false, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if strings.HasPrefix(filepath.ToSlash(hdr.Name), prefix) {
			return true, nil
		}
	}
}

func collectSpecPaths(specPath, specDir string) ([]string, error) {
	if specPath != "" {
		return []string{specPath}, nil
	}
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return nil, err
	}
	var specs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			specs = append(specs, filepath.Join(specDir, name))
		}
	}
	sort.Strings(specs)
	if len(specs) == 0 {
		return nil, fmt.Errorf("no spec files found in %s", specDir)
	}
	return specs, nil
}

func resolvePlatform(platform string) (string, string, error) {
	if platform == "" {
		return runtime.GOOS, runtime.GOARCH, nil
	}
	p := strings.ReplaceAll(platform, "/", "_")
	parts := strings.SplitN(p, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid platform %q (expected goos_goarch)", platform)
	}
	return parts[0], parts[1], nil
}

func buildArchiveName(serviceName, version, goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tgz", serviceName, version, goos, goarch)
}

func copyConfigDirs(dirs []string, destRoot string) (int, error) {
	seen := make(map[string]string)
	total := 0
	for _, dir := range dirs {
		n, err := copyDirNoOverwrite(dir, destRoot, seen)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func copyDirNoOverwrite(src, destRoot string, seen map[string]string) (int, error) {
	count := 0
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(destRoot, rel)
		if prev, ok := seen[target]; ok {
			return fmt.Errorf("config path collision: %s from %s and %s", target, prev, p)
		}
		seen[target] = p
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := copyFile(p, target); err != nil {
			return err
		}
		if err := os.Chmod(target, 0644); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

// copyDir recursively copies src directory to dest.
func copyDir(src, dest string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// sha256File computes the SHA256 hex digest of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifyBundledDebs runs both halves of the release-input proof over the debs
// that will actually be assembled, and fails closed on either.
//
// Order matters. Provenance is checked first because satisfiability computed
// over substituted bytes answers a question nobody asked: it would tell you
// whether the WRONG deb installs. Establish what the bytes are, then ask what
// they can do.
func verifyBundledDebs(debPaths []string, opts BuildOptions) error {
	provs := make([]*DebProvenance, 0, len(debPaths))
	for _, p := range debPaths {
		prov, err := VerifyDebProvenance(p)
		if err != nil {
			if opts.AllowUnprovenDebProvenance {
				fmt.Fprintf(os.Stderr, "  WARNING: unproven deb provenance accepted via explicit override: %v\n", err)
				continue
			}
			return fmt.Errorf("release input provenance: %w", err)
		}
		provs = append(provs, prov)
	}
	if len(provs) == 0 {
		return nil
	}

	if opts.PlatformBaselinePath == "" {
		return fmt.Errorf("release input satisfiability: no platform baseline declared, "+
			"so the installability of %d bundled deb(s) cannot be established; "+
			"pass a baseline rather than shipping an unchecked bundle", len(provs))
	}
	baseline, err := LoadPlatformBaseline(opts.PlatformBaselinePath)
	if err != nil {
		return fmt.Errorf("release input satisfiability: %w", err)
	}

	// Debs shipped together can satisfy each other.
	bundled := make(map[string]string, len(provs))
	for _, p := range provs {
		bundled[p.Package] = p.Version
	}

	var refused []string
	for _, p := range provs {
		sat := CheckSatisfiable(p, baseline, bundled)
		fmt.Fprint(os.Stdout, FormatProvenanceRecord(p, sat))
		if !sat.Satisfied {
			refused = append(refused, p.Package)
		}
	}
	if len(refused) > 0 {
		return fmt.Errorf("release input satisfiability: %s cannot install on declared baseline %s (%s) — "+
			"assembly refused", strings.Join(refused, ", "), baseline.ID, baseline.Image)
	}
	return nil
}
