package infra_truth

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// MinIO deployment modes (mirror config.ObjectStoreMode string values).
const (
	MinioModeStandalone  = "standalone"
	MinioModeDistributed = "distributed"
)

// MinioRenderedConfig is the parsed /var/lib/globular/minio/minio.env — the
// rendered config artifact owned by config.RenderMinioEnv (driven by the
// controller-published ObjectStoreDesiredState). Empty/zero means absent.
type MinioRenderedConfig struct {
	Path    string
	Present bool

	// Volumes is the parsed MINIO_VOLUMES list (space-separated). Each entry is
	// either a local path (standalone) or an https://host:9000/path URL
	// (distributed). VolumeCount is len(Volumes).
	Volumes     []string
	VolumeCount int

	// Mode is derived from the volume shape: any https:// entry → distributed,
	// otherwise standalone.
	Mode string

	// Endpoints is the deduplicated host set parsed from the distributed volume
	// URLs (empty in standalone mode).
	Endpoints []string

	HasRootUser bool // MINIO_ROOT_USER is set (value never captured — it is a secret)
	CICD        bool // MINIO_CI_CD=1 (set for distributed mode)
}

// parseMinioEnv reads and parses the rendered minio.env at path. A missing file
// is NOT an error: it returns Present=false so the lifecycle FSM can place the
// component at INFRA_PACKAGE_INSTALLED rather than fabricating config truth.
func parseMinioEnv(path string) (*MinioRenderedConfig, error) {
	cfg := &MinioRenderedConfig{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Present stays false
		}
		return cfg, fmt.Errorf("read minio env %s: %w", path, err)
	}

	cfg.Present = true
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = stripQuotes(val)
		switch key {
		case "MINIO_VOLUMES":
			cfg.Volumes = splitMinioVolumes(val)
			cfg.VolumeCount = len(cfg.Volumes)
		case "MINIO_ROOT_USER":
			cfg.HasRootUser = val != ""
		case "MINIO_CI_CD":
			cfg.CICD = val == "1" || strings.EqualFold(val, "true")
		}
	}

	cfg.Mode = MinioModeStandalone
	for _, v := range cfg.Volumes {
		if isURLVolume(v) {
			cfg.Mode = MinioModeDistributed
			if h := hostFromURL(v); h != "" {
				cfg.Endpoints = appendUniqueStr(cfg.Endpoints, h)
			}
		}
	}

	return cfg, nil
}

// splitMinioVolumes splits the MINIO_VOLUMES value on whitespace (MinIO accepts a
// space-separated list, optionally with brace expansion which the renderer does
// not use).
func splitMinioVolumes(val string) []string {
	var out []string
	for _, f := range strings.Fields(val) {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// isURLVolume reports whether a volume entry is an http(s) endpoint URL
// (distributed) rather than a local filesystem path (standalone).
func isURLVolume(v string) bool {
	return strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://")
}

func appendUniqueStr(in []string, v string) []string {
	for _, e := range in {
		if e == v {
			return in
		}
	}
	return append(in, v)
}

// volumeHost returns the bare host of a volume URL, or "" for a local path.
func volumeHost(v string) string {
	if !isURLVolume(v) {
		return ""
	}
	if u, err := url.Parse(v); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return ""
}

// renderedMap projects the parsed config into the InfraProbeResult.rendered map.
// The root credential VALUE is never projected — only its presence.
func (c *MinioRenderedConfig) renderedMap() map[string]string {
	m := map[string]string{
		"present": fmt.Sprintf("%t", c.Present),
		"path":    c.Path,
	}
	if !c.Present {
		return m
	}
	m["mode"] = c.Mode
	m["volume_count"] = fmt.Sprintf("%d", c.VolumeCount)
	m["volumes"] = strings.Join(c.Volumes, " ")
	m["endpoints"] = strings.Join(c.Endpoints, ",")
	m["has_root_user"] = fmt.Sprintf("%t", c.HasRootUser)
	return m
}
