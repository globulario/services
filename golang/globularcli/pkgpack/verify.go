package pkgpack

import (
	"archive/tar"
	"compress/gzip"
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
	Platform     string
	Publisher    string
	Entrypoint   string
	ConfigCount  int
	SystemdCount int
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
	var configCount, systemdCount int
	var manifest Manifest

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

		if name == "package.json" {
			required["package.json"] = true
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, err
			}
		}
	}

	if !required["package.json"] {
		return nil, fmt.Errorf("package.json missing from archive")
	}
	if len(binFiles) == 0 {
		return nil, fmt.Errorf("no bin entries found")
	}
	if len(specFiles) != 1 {
		return nil, fmt.Errorf("expected exactly one spec file, found %d", len(specFiles))
	}
	if manifest.Entrypoint != "" {
		if _, ok := files[path.Clean(manifest.Entrypoint)]; !ok {
			return nil, fmt.Errorf("entrypoint %s missing in archive", manifest.Entrypoint)
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

	return &VerificationSummary{
		Name:         manifest.Name,
		Version:      manifest.Version,
		Platform:     manifest.Platform,
		Publisher:    manifest.Publisher,
		Entrypoint:   manifest.Entrypoint,
		ConfigCount:  configCount,
		SystemdCount: systemdCount,
	}, nil
}
