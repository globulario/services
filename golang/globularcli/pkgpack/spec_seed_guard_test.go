package pkgpack

import (
	"os"
	"path/filepath"
	"testing"
)

// seedGuardCase describes a cluster-owned config file that must never be
// overwritten by a package reinstall. Each entry maps a spec file to the
// exact file path (as declared in the spec's install_files step) that must
// carry skip_if_exists: true.
//
// Ownership mode reference:
//
//	seed-only      — package writes on first install only; runtime owner is
//	                 the operator or an external system (prometheus, alertmanager).
//	contract-rendered — package seeds default; controller/node-agent may overwrite
//	                 from cluster desired state (etcd.yaml, minio.env).
//	identity-owned — cluster identity; join/renewal flows update it; reinstall never.
var seedGuardCases = []struct {
	spec     string // basename of the spec file
	filePath string // {{.StateDir}} rendered as the literal template (must match spec)
	mode     string // ownership mode for documentation
}{
	// etcd.yaml — contract-rendered by cluster controller on every topology change.
	{"etcd_service.yaml", "{{.StateDir}}/config/etcd.yaml", "contract-rendered"},
	// minio.env — contract-rendered by cluster controller on objectstore topology change.
	{"minio_service.yaml", "{{.StateDir}}/minio/minio.env", "contract-rendered"},
	// MinIO credentials — identity-owned; generated on first join, never rotated by package.
	{"minio_service.yaml", "{{.StateDir}}/minio/credentials", "identity-owned"},
	// prometheus.yml — seed-only; operator may add scrape jobs, alert rules, etc.
	{"prometheus_service.yaml", "{{.StateDir}}/prometheus/prometheus.yml", "seed-only"},
	// alertmanager.yml — seed-only; operator must configure real receivers.
	{"alertmanager_service.yaml", "{{.StateDir}}/alertmanager/alertmanager.yml", "seed-only"},
	// xds.yaml — seed-only; static gRPC/TLS/logging config for the xDS process.
	{"xds_service.yaml", "{{.StateDir}}/xds/xds.yaml", "seed-only"},
	// xds/config.json — seed-only; etcd bootstrap endpoints, operator may extend.
	{"xds_service.yaml", "{{.StateDir}}/xds/config.json", "seed-only"},
	// mcp/config.json — seed-only; operator enables/disables tool groups.
	{"mcp_service.yaml", "{{.StateDir}}/mcp/config.json", "seed-only"},
	// sidekick.env — seed-only; operator may retarget MinIO backends/listener.
	{"sidekick_service.yaml", "{{.StateDir}}/sidekick/sidekick.env", "seed-only"},
	// scylla-manager-agent.yaml — seed-only; auth_token written by post-install script.
	{"scylla_manager_agent_service.yaml", "{{.StateDir}}/scylla-manager-agent/scylla-manager-agent.yaml", "seed-only"},
}

// TestSeedConfigsHaveSkipIfExists ensures that every cluster-owned config file
// declared in a package spec carries skip_if_exists: true. This is the
// regression guard for Guardrail 1 (config ownership / seed file protection):
// a package reinstall must never overwrite live cluster configuration.
func TestSeedConfigsHaveSkipIfExists(t *testing.T) {
	for _, tc := range seedGuardCases {
		tc := tc
		t.Run(tc.spec+"/"+filepath.Base(tc.filePath), func(t *testing.T) {
			specPath := findPackageSpec(t, tc.spec)
			spec, err := ParseSpec(specPath)
			if err != nil {
				t.Fatalf("spec found but unparseable (%s): %v", specPath, err)
			}

			found := false
			guarded := false
			for _, step := range spec.Steps {
				if step.Type != "install_files" {
					continue
				}
				files, _ := step.Args["files"].([]any)
				for _, f := range files {
					fm, ok := f.(map[string]any)
					if !ok {
						continue
					}
					path, _ := fm["path"].(string)
					if path != tc.filePath {
						continue
					}
					found = true
					skip, _ := fm["skip_if_exists"].(bool)
					if skip {
						guarded = true
					}
				}
			}

			if !found {
				t.Errorf("file %q not found in any install_files step of %s — spec changed?", tc.filePath, tc.spec)
				return
			}
			if !guarded {
				t.Errorf(
					"cluster config %q in %s is missing skip_if_exists: true\n"+
						"ownership mode: %s\n"+
						"without this flag, package reinstall will overwrite operator/controller-managed config",
					tc.filePath, tc.spec, tc.mode,
				)
			}
		})
	}
}

func findPackageSpec(t *testing.T, specName string) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	var metadataMatches []string
	var legacyMatches []string
	for dir := wd; ; dir = filepath.Dir(dir) {
		packagesRoot := filepath.Join(filepath.Dir(dir), "packages")
		metadataMatches = append(metadataMatches, existingMetadataPackageSpecMatches(t, packagesRoot, specName)...)
		legacyMatches = append(legacyMatches, existingLegacyPackageSpecMatches(t, packagesRoot, specName)...)

		next := filepath.Dir(dir)
		if next == dir {
			break
		}
	}

	switch len(metadataMatches) {
	case 1:
		return metadataMatches[0]
	default:
		if len(metadataMatches) > 1 {
			t.Fatalf("spec %s is ambiguous; found multiple metadata matches: %v", specName, metadataMatches)
		}
	}

	switch len(legacyMatches) {
	case 0:
		t.Fatalf("spec %s not found in adjacent packages repo (searched packages/metadata/*/specs with legacy packages/specs fallback)", specName)
	case 1:
		return legacyMatches[0]
	default:
		t.Fatalf("spec %s is ambiguous; found multiple legacy matches: %v", specName, legacyMatches)
	}
	return ""
}

func existingMetadataPackageSpecMatches(t *testing.T, packagesRoot, specName string) []string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(packagesRoot, "metadata", "*", "specs", specName))
	if err != nil {
		t.Fatalf("glob package specs under %s: %v", packagesRoot, err)
	}
	return existingPaths(t, matches)
}

func existingLegacyPackageSpecMatches(t *testing.T, packagesRoot, specName string) []string {
	t.Helper()

	return existingPaths(t, []string{filepath.Join(packagesRoot, "specs", specName)})
}

func existingPaths(t *testing.T, candidates []string) []string {
	t.Helper()

	var paths []string
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat package spec candidate %s: %v", candidate, err)
		}
	}
	return paths
}
