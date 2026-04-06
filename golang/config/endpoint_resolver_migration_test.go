package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// migratedPath lists the service-to-service dialer files that have
// been migrated to config.ResolveDialTarget. The invariants enforced
// below prevent regressions:
//
//  1. Each migrated file MUST import/use the shared resolver.
//  2. Each migrated file MUST NOT re-introduce its own
//     "127.0.0.1 → localhost" rewrite logic.
//  3. Each migrated file MUST NOT extract SNI via its own
//     net.SplitHostPort branch (use DialTarget.ServerName instead).
//
// These invariants mirror the acceptance criteria in `todo` and the
// policy in docs/endpoint_resolution_policy.md. If you hit a failure
// here, the fix is almost always "use config.ResolveDialTarget at the
// dial site", not "loosen the test".
type migratedPath struct {
	rel                       string // relative to repo golang/ dir
	mustUseResolver           bool
	forbidLoopbackRewrite     bool
	forbidAdHocSNIExtraction  bool
}

var migratedPaths = []migratedPath{
	{
		rel:                      "cluster_doctor/cluster_doctor_server/config.go",
		mustUseResolver:          true,
		forbidLoopbackRewrite:    true,
		forbidAdHocSNIExtraction: true,
	},
	{
		rel:                      "cluster_doctor/cluster_doctor_server/server.go",
		mustUseResolver:          true,
		forbidLoopbackRewrite:    true,
		forbidAdHocSNIExtraction: true,
	},
	{
		rel:                      "cluster_doctor/cluster_doctor_server/node_agent_dialer.go",
		mustUseResolver:          true,
		forbidLoopbackRewrite:    true,
		forbidAdHocSNIExtraction: true,
	},
	{
		rel:                      "cluster_controller/cluster_controller_server/agentclient.go",
		mustUseResolver:          true,
		forbidLoopbackRewrite:    true,
		forbidAdHocSNIExtraction: true,
	},
	{
		rel:                      "node_agent/node_agent_server/heartbeat.go",
		mustUseResolver:          true,
		forbidLoopbackRewrite:    true,
		forbidAdHocSNIExtraction: true,
	},
}

// Patterns for the ad-hoc rewrite logic we replaced. Each pattern is
// conservative — it matches only constructs we actually had in the
// pre-refactor code, not any legitimate use that might happen to
// contain the word "127.0.0.1".
var (
	// Loopback rewrites that transform the host. Raw string literals
	// that are just default-endpoint defaults are fine; these catch
	// the actual rewriting conditional.
	reLoopbackRewrite = regexp.MustCompile(`(?m)host\s*==\s*"127\.0\.0\.1"\s*\|\|\s*host\s*==\s*"::1"|return\s+"localhost:"\s*\+\s*port`)
	// Ad-hoc SNI extraction: the "net.SplitHostPort → use host as
	// ServerName" idiom that was duplicated across four files.
	reAdHocSNI = regexp.MustCompile(`(?m)net\.SplitHostPort\([^)]*\)[^}]*serverName\s*=`)
)

func TestMigratedDialerFilesUseResolver(t *testing.T) {
	root := repoGolangRoot(t)
	for _, p := range migratedPaths {
		t.Run(p.rel, func(t *testing.T) {
			full := filepath.Join(root, p.rel)
			data, err := os.ReadFile(full)
			if err != nil {
				t.Fatalf("read %s: %v", full, err)
			}
			src := string(data)

			if p.mustUseResolver {
				// Either direct call (ResolveDialTarget / NormalizeLoopback)
				// or the DialTarget type (propagated as function arg).
				if !strings.Contains(src, "config.ResolveDialTarget") &&
					!strings.Contains(src, "config.NormalizeLoopback") &&
					!strings.Contains(src, "config.DialTarget") {
					t.Errorf("%s must use config.ResolveDialTarget / NormalizeLoopback / DialTarget (endpoint policy)", p.rel)
				}
			}
			if p.forbidLoopbackRewrite {
				if reLoopbackRewrite.MatchString(src) {
					t.Errorf("%s re-introduced ad-hoc loopback rewrite — use config.ResolveDialTarget instead", p.rel)
				}
			}
			if p.forbidAdHocSNIExtraction {
				if reAdHocSNI.MatchString(src) {
					t.Errorf("%s re-introduced ad-hoc SNI extraction via net.SplitHostPort — use DialTarget.ServerName instead", p.rel)
				}
			}
		})
	}
}

// repoGolangRoot returns the path to the golang/ directory by
// walking up from this test file's location.
func repoGolangRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// thisFile = .../golang/config/endpoint_resolver_migration_test.go
	return filepath.Dir(filepath.Dir(thisFile))
}
