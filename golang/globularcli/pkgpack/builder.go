package pkgpack

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type BuildOptions struct {
	SpecPath           string
	SpecDir            string
	AssetsDir          string
	InstallerRoot      string
	Root               string
	BinDir             string
	ConfigDir          string
	Version            string
	Publisher          string
	Platform           string
	OutDir             string
	SkipMissingConfig  bool
	SkipMissingSystemd bool
}

type BuildResult struct {
	SpecPath   string
	Service    string
	OutputPath string
	Err        error
}

func BuildPackages(opts BuildOptions) ([]BuildResult, error) {
	if opts.Version == "" {
		return nil, fmt.Errorf("version is required")
	}
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
		roots := AssetRoots{BinRoot: binRoot, ConfigRoot: configRoot}
		info, err := ScanSpec(spec, roots, ScanOptions{SkipMissingConfig: opts.SkipMissingConfig, SkipMissingSystemd: opts.SkipMissingSystemd})
		if err != nil {
			res.Err = err
			results = append(results, res)
			fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", spec, err)
			hadErr = true
			continue
		}
		res.Service = info.ServiceName

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
				fmt.Fprintf(os.Stdout, "  manifest: name=%s version=%s platform=%s entrypoint=%s configs=%d systemd=%d\n",
					summary.Name, summary.Version, summary.Platform, summary.Entrypoint, summary.ConfigCount, summary.SystemdCount)
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

	execDest := filepath.Join(stagingDir, "bin", info.ExecName)
	if err := copyFile(info.ExecPath, execDest); err != nil {
		return nil, err
	}
	if err := os.Chmod(execDest, 0755); err != nil {
		return nil, err
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

	manifest := Manifest{
		Type:       "service",
		Name:       info.ServiceName,
		Version:    opts.Version,
		Platform:   fmt.Sprintf("%s_%s", goos, goarch),
		Publisher:  opts.Publisher,
		Entrypoint: path.Join("bin", info.ExecName),
		Defaults: ManifestDefault{
			ConfigDir: "",
			Spec:      path.Join("specs", info.SpecFile),
		},
	}
	if copiedConfig > 0 {
		manifest.Defaults.ConfigDir = path.Join("config", info.ServiceName)
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
	// 1) binary present
	wantBin := filepath.ToSlash(filepath.Join("bin", info.ExecName))
	if ok, err := tgzContains(pkgPath, wantBin); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("package %s missing binary %s", pkgPath, wantBin)
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
	return fmt.Sprintf("service.%s_%s_%s_%s.tgz", serviceName, version, goos, goarch)
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
