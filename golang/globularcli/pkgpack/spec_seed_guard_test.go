package pkgpack

import (
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
	// scylla-manager-agent.yaml — seed-only; auth_token written by post-install script.
	{"scylla_manager_agent_service.yaml", "{{.StateDir}}/scylla-manager-agent/scylla-manager-agent.yaml", "seed-only"},
}

// TestSeedConfigsHaveSkipIfExists ensures that every cluster-owned config file
// declared in a package spec carries skip_if_exists: true. This is the
// regression guard for Guardrail 1 (config ownership / seed file protection):
// a package reinstall must never overwrite live cluster configuration.
func TestSeedConfigsHaveSkipIfExists(t *testing.T) {
	// Spec files live in packages/specs/ — four levels up from this package.
	specsDir := filepath.Join("..", "..", "..", "..", "packages", "specs")

	for _, tc := range seedGuardCases {
		tc := tc
		t.Run(tc.spec+"/"+filepath.Base(tc.filePath), func(t *testing.T) {
			specPath := filepath.Join(specsDir, tc.spec)
			spec, err := ParseSpec(specPath)
			if err != nil {
				t.Skipf("spec not found or unparseable (%s): %v", tc.spec, err)
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
