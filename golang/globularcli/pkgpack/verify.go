package pkgpack

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

type VerificationSummary struct {
	Name         string
	Version      string
	BuildNumber  int64
	Platform     string
	Publisher    string
	Type         string // "service" (default), "application", "infrastructure"
	Entrypoint   string
	ConfigCount  int
	SystemdCount int
	ScriptsCount int
	Description  string
	Keywords     []string
	License      string

	// Catalog metadata
	Profiles                 []string
	Priority                 int
	InstallMode              string
	ManagedUnit              bool
	SystemdUnit              string
	ProvidesCapabilities     []string
	InstallDependencies      []string
	RuntimeLocalDependencies []string
	HealthCheckUnit          string
	HealthCheckPort          int
}

// VerifyTGZ validates package contents and returns a summary.
func VerifyTGZ(tgzPath string) (*VerificationSummary, error) {
	file, err := os.Open(tgzPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	required := map[string]bool{"package.json": false}
	dirs := make(map[string]struct{})
	files := make(map[string]struct{})
	var binFiles []string
	var specFiles []string
	var configCount, systemdCount, scriptsCount int
	var manifest Manifest
	fileSHA := make(map[string]string)
	systemdContent := make(map[string]string)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := path.Clean(hdr.Name)
		if strings.HasSuffix(name, "/") {
			dirs[strings.TrimSuffix(name, "/")] = struct{}{}
		} else {
			files[name] = struct{}{}
		}
		if strings.HasPrefix(name, "bin/") && !strings.HasSuffix(name, "/") {
			binFiles = append(binFiles, name)
		}
		if strings.HasPrefix(name, "specs/") && !strings.HasSuffix(name, "/") {
			specFiles = append(specFiles, name)
		}
		if strings.HasPrefix(name, "config/") && !strings.HasSuffix(name, "/") {
			configCount++
		}
		if strings.HasPrefix(name, "systemd/") && !strings.HasSuffix(name, "/") {
			systemdCount++
		}
		if strings.HasPrefix(name, "scripts/") && !strings.HasSuffix(name, "/") {
			scriptsCount++
			if hdr.FileInfo().Mode().Perm()&0o111 == 0 {
				return nil, fmt.Errorf("script %s is not executable (mode %o)", name, hdr.FileInfo().Mode().Perm())
			}
		}

		if name == "package.json" {
			required["package.json"] = true
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, err
			}
			continue
		}

		// Hash all regular files so we can verify entrypoint_checksum later.
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		fileSHA[name] = hex.EncodeToString(sum[:])

		if strings.HasPrefix(name, "systemd/") {
			systemdContent[name] = string(data)
		}
	}

	if !required["package.json"] {
		return nil, fmt.Errorf("package.json missing from archive")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return nil, fmt.Errorf("manifest name is required")
	}
	if strings.TrimSpace(manifest.Platform) == "" {
		return nil, fmt.Errorf("manifest platform is required")
	}
	if strings.TrimSpace(manifest.Publisher) == "" {
		return nil, fmt.Errorf("manifest publisher is required")
	}
	if err := ValidateVersionBuildSemantics(manifest.Version, manifest.BuildNumber); err != nil {
		return nil, err
	}

	pkgType := strings.ToLower(strings.TrimSpace(manifest.Type))
	if pkgType == "" {
		pkgType = "service"
	}

	// Validation rules depend on package type.
	switch pkgType {
	case "application":
		// Applications contain web content — bin/ and specs/ are not required.
		// At least one content file must be present (besides package.json).
		if len(files) <= 1 {
			return nil, fmt.Errorf("application archive contains no content files")
		}
	case "infrastructure":
		// Infrastructure packages require bin/ but specs/ is optional
		// (they use systemd/ and config/ instead).
		if len(binFiles) == 0 {
			return nil, fmt.Errorf("no bin entries found")
		}
	default:
		// Service packages require bin/ and exactly one spec.
		if len(binFiles) == 0 {
			return nil, fmt.Errorf("no bin entries found")
		}
		if len(specFiles) != 1 {
			return nil, fmt.Errorf("expected exactly one spec file, found %d", len(specFiles))
		}
	}

	if manifest.Entrypoint != "" {
		entrypoint := path.Clean(manifest.Entrypoint)
		if _, ok := files[entrypoint]; !ok {
			return nil, fmt.Errorf("entrypoint %s missing in archive", manifest.Entrypoint)
		}
		if strings.TrimSpace(manifest.EntrypointChecksum) != "" {
			d := strings.TrimSpace(manifest.EntrypointChecksum)
			if !strings.HasPrefix(strings.ToLower(d), "sha256:") {
				return nil, fmt.Errorf("entrypoint_checksum must start with sha256:")
			}
			want := strings.TrimPrefix(strings.ToLower(d), "sha256:")
			if len(want) != 64 {
				return nil, fmt.Errorf("entrypoint_checksum must be 64 hex chars")
			}
			got := fileSHA[entrypoint]
			if got == "" {
				return nil, fmt.Errorf("entrypoint checksum unavailable for %s", entrypoint)
			}
			if got != want {
				return nil, fmt.Errorf("entrypoint_checksum mismatch for %s: manifest=%s actual=%s", entrypoint, want, got)
			}
		}
	}
	if manifest.Defaults.ConfigDir != "" {
		cfg := path.Clean(manifest.Defaults.ConfigDir)
		if _, ok := dirs[cfg]; !ok {
			if _, ok := files[cfg]; !ok {
				return nil, fmt.Errorf("config dir %s missing in archive", cfg)
			}
		}
	}

	for unit, content := range systemdContent {
		if err := validateSystemdUnitContent(content); err != nil {
			return nil, fmt.Errorf("invalid systemd unit %s: %w", unit, err)
		}
	}

	return &VerificationSummary{
		Name:         manifest.Name,
		Version:      manifest.Version,
		BuildNumber:  manifest.BuildNumber,
		Platform:     manifest.Platform,
		Publisher:    manifest.Publisher,
		Type:         pkgType,
		Entrypoint:   manifest.Entrypoint,
		ConfigCount:  configCount,
		SystemdCount: systemdCount,
		ScriptsCount: scriptsCount,
		Description:  manifest.Description,
		Keywords:     manifest.Keywords,
		License:      manifest.License,

		Profiles:                 manifest.Profiles,
		Priority:                 manifest.Priority,
		InstallMode:              manifest.InstallMode,
		ManagedUnit:              manifest.ManagedUnit,
		SystemdUnit:              manifest.SystemdUnit,
		ProvidesCapabilities:     manifest.ProvidesCapabilities,
		InstallDependencies:      manifest.InstallDependencies,
		RuntimeLocalDependencies: manifest.RuntimeLocalDependencies,
		HealthCheckUnit:          manifest.HealthCheckUnit,
		HealthCheckPort:          manifest.HealthCheckPort,
	}, nil
}

var singletonServiceDirectivesPkg = map[string]struct{}{
	"Type":            {},
	"ExecStart":       {},
	"ExecReload":      {},
	"ExecStop":        {},
	"ExecStopPost":    {},
	"WorkingDirectory": {},
	"User":            {},
	"Group":           {},
	"Restart":         {},
	"RestartSec":      {},
	"KillMode":        {},
	"TimeoutStartSec": {},
	"TimeoutStopSec":  {},
}

func validateSystemdUnitContent(content string) error {
	section := ""
	counts := make(map[string]int)
	for lineNo, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		if section != "Service" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		name := line[:eq]
		if _, ok := singletonServiceDirectivesPkg[name]; !ok {
			continue
		}
		counts[name]++
		if counts[name] > 1 {
			return fmt.Errorf("line %d: duplicate singleton directive %s= in [Service]", lineNo+1, name)
		}
	}
	return nil
}
