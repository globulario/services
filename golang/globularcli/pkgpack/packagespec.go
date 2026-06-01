package pkgpack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PackageSpec is the canonical definition of a Globular package spec file.
// It is the source of truth for the spec format — all fields are documented here.
type PackageSpec struct {
	Version  int              `yaml:"version"`  // Must be 1.
	Metadata PackageMetadata  `yaml:"metadata"` // Package identity and catalog fields.
	Service  *ServiceBlock    `yaml:"service"`  // Service identity (nil for commands).
	Steps    []InstallStep    `yaml:"steps"`    // Ordered install recipe.
}

// PackageMetadata describes a package's identity, classification, and catalog properties.
type PackageMetadata struct {
	// Identity
	Name        string `yaml:"name"`        // Required. Canonical package name (e.g. "authentication", "etcd").
	Kind        string `yaml:"kind"`        // "service", "infrastructure", "command", "application". Default: derived from filename.
	Description string `yaml:"description"` // Human-readable one-liner.
	Keywords    []string `yaml:"keywords"`  // Search/filter tags.
	License     string `yaml:"license"`     // SPDX identifier (e.g. "Apache-2.0").
	Channel     string `yaml:"channel"`     // Release channel: "stable" (default), "candidate", "canary", "dev", "bootstrap".

	// Build hints
	ExtraBinaries []string `yaml:"extra_binaries"` // Additional binaries to bundle alongside the main exec.
	Entrypoint    string   `yaml:"entrypoint"`     // Override: exec name when it differs from convention. "noop" for OS-managed packages.
	InstallBins   *bool    `yaml:"install_bins"`   // Override: false to skip bin/ extraction (OS-managed packages like scylladb).
	BundleDebs    []string `yaml:"bundle_debs"`    // OS package names to download and bundle as .deb files at build time. Installed via dpkg at install time (no internet needed).

	// Catalog — drives the cluster controller's dynamic component catalog.
	Profiles    []string `yaml:"profiles"`     // Deployment profiles that include this package (e.g. "core", "compute").
	Priority    int      `yaml:"priority"`     // Start order: lower starts first, stops last. 0 = default (1000).
	InstallMode string   `yaml:"install_mode"` // "repository" (default) or "day0_join".
	ManagedUnit bool     `yaml:"managed_unit"` // Include in profileUnitMap for bulk unit actions.
	SystemdUnit string   `yaml:"systemd_unit"` // Override systemd unit name (auto-derived from spec if empty).

	// Day-1 orchestration — profile-aware, dependency-gated convergence.
	ProvidesCapabilities []string         `yaml:"provides_capabilities"` // Capabilities this package gives the node (e.g. "local-db").
	HealthCheck          *HealthCheckHint `yaml:"health_check"`          // How to verify this package is healthy.

	// Typed dependency declarations.
	// HardDeps: install/activation blockers. Form directed graph edges.
	// RuntimeUses: informational API peers (gRPC names). Not graph edges.
	HardDeps    []string `yaml:"hard_deps"`    // e.g. ["etcd", "scylladb"]
	RuntimeUses []string `yaml:"runtime_uses"` // e.g. ["repository.PackageRepository"]

	// Deprecated: use hard_deps instead.
	InstallDependencies      []string `yaml:"install_dependencies"`       // Kept for reading legacy specs.
	RuntimeLocalDependencies []string `yaml:"runtime_local_dependencies"` // Kept for reading legacy specs.
}

// ServiceBlock identifies the service binary within the package.
type ServiceBlock struct {
	Name string `yaml:"name"` // Service name (usually matches metadata.name).
	Exec string `yaml:"exec"` // Executable filename (e.g. "authentication_server").
}

// InstallStep is one step in the install recipe.
// Common fields are typed; step-type-specific fields live in Args.
type InstallStep struct {
	ID   string `yaml:"id"`   // Unique step identifier (e.g. "install-authentication-payload").
	Type string `yaml:"type"` // Step type (e.g. "install_package_payload", "ensure_dirs").

	// Type-specific fields are captured as a raw map during parsing.
	// This keeps the struct open to new step types without code changes.
	Args map[string]any `yaml:"-"`
}

// UnmarshalYAML implements custom unmarshaling for InstallStep.
// It extracts id and type into typed fields and captures everything else in Args.
func (s *InstallStep) UnmarshalYAML(node *yaml.Node) error {
	// Decode the full map.
	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return err
	}

	if v, ok := raw["id"].(string); ok {
		s.ID = v
	}
	if v, ok := raw["type"].(string); ok {
		s.Type = v
	}

	// Everything except id/type goes into Args.
	s.Args = make(map[string]any, len(raw))
	for k, v := range raw {
		if k == "id" || k == "type" {
			continue
		}
		s.Args[k] = v
	}
	return nil
}

// serviceNameFromPath derives a package name from the spec filename.
// e.g. "etcd_service.yaml" → "etcd", "mc_cmd.yaml" → "mc".
func serviceNameFromPath(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	if base == "." || base == "/" {
		return ""
	}
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = strings.TrimSuffix(name, "_service")
	name = strings.TrimSuffix(name, "-service")
	name = strings.TrimSuffix(name, "_cmd")
	name = strings.TrimSuffix(name, "_command")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

// ParseSpec reads a YAML spec file and returns a typed PackageSpec.
func ParseSpec(path string) (*PackageSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", path, err)
	}
	return ParseSpecBytes(data, path)
}

// ParseSpecBytes parses spec YAML bytes into a PackageSpec.
// The path argument is used only for error messages.
func ParseSpecBytes(data []byte, path string) (*PackageSpec, error) {
	var spec PackageSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse spec %s: %w", path, err)
	}
	return &spec, nil
}

// ValidateSpec checks a PackageSpec for structural correctness.
// It returns all errors found (not just the first one).
func ValidateSpec(spec *PackageSpec, path string) []error {
	var errs []error
	add := func(msg string, args ...any) {
		errs = append(errs, fmt.Errorf("spec %s: "+msg, append([]any{path}, args...)...))
	}

	// Version
	if spec.Version != 1 {
		add("version must be 1 (got %d)", spec.Version)
	}

	// Metadata.Name is required but can be derived from service.name or filename.
	name := spec.Metadata.Name
	if name == "" && spec.Service != nil {
		name = spec.Service.Name
	}
	if name == "" {
		// Derive from filename as last resort (matches ScanSpec behavior).
		name = serviceNameFromPath(path)
	}
	if name == "" {
		add("metadata.name is required (and could not be derived from service block or filename)")
	}

	// Kind validation.
	kind := strings.ToLower(spec.Metadata.Kind)
	if kind != "" {
		switch kind {
		case "service", "infrastructure", "command", "application":
			// ok
		default:
			add("metadata.kind %q is not valid (must be service, infrastructure, command, or application)", kind)
		}
	}

	// InstallMode validation.
	if im := spec.Metadata.InstallMode; im != "" {
		switch im {
		case "repository", "day0_join":
			// ok
		default:
			add("metadata.install_mode %q is not valid (must be repository or day0_join)", im)
		}
	}

	// Priority range.
	if spec.Metadata.Priority < 0 {
		add("metadata.priority must be >= 0 (got %d)", spec.Metadata.Priority)
	}

	// Steps validation.
	if len(spec.Steps) == 0 {
		add("steps list is empty")
	}
	stepIDs := make(map[string]int)
	hasInstallPayload := false
	for i, step := range spec.Steps {
		if step.ID == "" {
			add("steps[%d]: id is required", i)
		} else if prev, dup := stepIDs[step.ID]; dup {
			add("steps[%d]: duplicate id %q (first at steps[%d])", i, step.ID, prev)
		} else {
			stepIDs[step.ID] = i
		}
		if step.Type == "" {
			add("steps[%d] (%s): type is required", i, step.ID)
		}
		if step.Type == "install_package_payload" {
			hasInstallPayload = true
		}
	}

	// Kind-specific rules.
	effectiveKind := kind
	if effectiveKind == "" {
		effectiveKind = "service"
	}
	switch effectiveKind {
	case "service":
		if !hasInstallPayload {
			add("service spec must have an install_package_payload step")
		}
	case "infrastructure":
		// Infrastructure specs should also have install_package_payload unless
		// they're OS-managed (entrypoint: noop / install_bins: false).
		if !hasInstallPayload && spec.Metadata.Entrypoint != "noop" {
			add("infrastructure spec must have an install_package_payload step (or set entrypoint: noop)")
		}
	}

	return errs
}
