package pkgpack

import (
	"encoding/json"
	"os"
)

// Manifest describes the packaged service.
type Manifest struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	BuildNumber int64           `json:"build_number,omitempty"` // Display-only monotonic counter (NOT used in convergence)
	BuildID     string          `json:"build_id,omitempty"`     // Repository-allocated UUIDv7 (populated after upload)
	Platform    string          `json:"platform"`
	Publisher   string          `json:"publisher"`
	Entrypoint  string          `json:"entrypoint"`
	Defaults    ManifestDefault `json:"defaults"`
	Description string          `json:"description,omitempty"`
	Keywords    []string        `json:"keywords,omitempty"`
	License     string          `json:"license,omitempty"`

	// Catalog metadata — drives dynamic component catalog in the cluster controller.
	Profiles             []string `json:"profiles,omitempty"`
	Priority             int      `json:"priority,omitempty"`
	InstallMode          string   `json:"install_mode,omitempty"`
	ManagedUnit          bool     `json:"managed_unit,omitempty"`
	SystemdUnit          string   `json:"systemd_unit,omitempty"`
	ProvidesCapabilities []string `json:"provides_capabilities,omitempty"`
	HealthCheckUnit      string   `json:"health_check_unit,omitempty"`
	HealthCheckPort      int      `json:"health_check_port,omitempty"`

	// Typed dependency declarations.
	//
	// HardDeps: install/activation blockers. Form graph edges for cycle detection
	// and the reachability engine. This artifact cannot start until all hard deps
	// are installed and healthy. The uninstaller will not remove a package that
	// is a hard dep of any installed/desired artifact.
	//
	// RuntimeUses: informational API peers (gRPC service names or package names).
	// Never graph edges. Never block uninstall. Documentation and mesh routing hints.
	HardDeps    []string `json:"hard_deps,omitempty"`
	RuntimeUses []string `json:"runtime_uses,omitempty"`

	// Deprecated: use HardDeps instead. Kept for reading legacy packages.
	InstallDependencies      []string `json:"install_dependencies,omitempty"`
	RuntimeLocalDependencies []string `json:"runtime_local_dependencies,omitempty"`

	// SHA256 of the entrypoint binary. Enables reverse lookup:
	// binary on disk → checksum → repository version.
	EntrypointChecksum string `json:"entrypoint_checksum,omitempty"`

	// Channel declares which release channel this artifact belongs to.
	// Valid values: "stable", "candidate", "canary", "dev", "bootstrap".
	// Empty or omitted defaults to "stable" on the repository side.
	Channel string `json:"channel,omitempty"`
}

// ManifestDefault provides default paths inside the package.
type ManifestDefault struct {
	ConfigDir  string `json:"configDir"`
	Spec       string `json:"spec"`
	ScriptsDir string `json:"scriptsDir,omitempty"`
}

func WriteManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
