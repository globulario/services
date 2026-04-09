package deploy

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds resolved paths relative to the repository root.
type Paths struct {
	Root      string // Repository root (e.g., /home/user/services)
	Golang    string // Go source directory
	StageBin  string // Staged binaries directory
	Generated string // Generated output directory
	SpecsDir  string // Generated specs directory
	Catalog   string // Service catalog YAML path
}

// ResolvePaths discovers the repository root from the current working directory
// and resolves all standard paths.
func ResolvePaths() (*Paths, error) {
	root, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	return &Paths{
		Root:      root,
		Golang:    filepath.Join(root, "golang"),
		StageBin:  filepath.Join(root, "golang", "tools", "stage", "linux-amd64", "usr", "local", "bin"),
		Generated: filepath.Join(root, "generated"),
		SpecsDir:  filepath.Join(root, "generated", "specs"),
		Catalog:   filepath.Join(root, "golang", "service_catalog.yaml"),
	}, nil
}

// GoPackageDir returns the Go source directory for a service.
// It tries several conventional locations.
func (p *Paths) GoPackageDir(serviceName, execName string) (string, error) {
	candidates := []string{
		filepath.Join(p.Golang, serviceName, execName),
		filepath.Join(p.Golang, serviceName, serviceName+"_server"),
		filepath.Join(p.Golang, execName),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("Go package directory not found for %s (tried %v)", serviceName, candidates)
}

// GoPackageRelative returns the Go import-style relative path for go build.
func (p *Paths) GoPackageRelative(absDir string) string {
	rel, err := filepath.Rel(p.Golang, absDir)
	if err != nil {
		return absDir
	}
	return "./" + rel
}

// PayloadDir returns the payload directory for a service.
func (p *Paths) PayloadDir(serviceName string) string {
	return filepath.Join(p.Generated, "payload", serviceName)
}

// SpecFile returns the spec file path for a service.
func (p *Paths) SpecFile(serviceName string) string {
	return filepath.Join(p.SpecsDir, serviceName+"_service.yaml")
}

// findRepoRoot walks up from the current directory looking for go.mod in golang/.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		// Check for the golang/go.mod marker.
		if _, err := os.Stat(filepath.Join(dir, "golang", "go.mod")); err == nil {
			return dir, nil
		}
		// Check for proto/ directory as a secondary marker.
		if _, err := os.Stat(filepath.Join(dir, "proto")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "golang")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository root (no golang/go.mod found)")
		}
		dir = parent
	}
}
