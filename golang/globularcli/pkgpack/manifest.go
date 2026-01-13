package pkgpack

import (
	"encoding/json"
	"os"
)

// Manifest describes the packaged service.
type Manifest struct {
	Type       string          `json:"type"`
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	Platform   string          `json:"platform"`
	Publisher  string          `json:"publisher"`
	Entrypoint string          `json:"entrypoint"`
	Defaults   ManifestDefault `json:"defaults"`
}

// ManifestDefault provides default paths inside the package.
type ManifestDefault struct {
	ConfigDir string `json:"configDir"`
	Spec      string `json:"spec"`
}

func WriteManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
