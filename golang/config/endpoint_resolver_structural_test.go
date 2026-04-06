package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoAdHocLoopbackRewrites is a structural regression test that scans
// Go source files for patterns indicating ad-hoc loopback rewriting or
// custom SNI extraction in service-to-service dialers. These patterns
// must use config.ResolveDialTarget instead.
//
// See docs/endpoint_resolution_policy.md.
func TestNoAdHocLoopbackRewrites(t *testing.T) {
	// Patterns that indicate ad-hoc loopback/SNI handling. Each is a
	// substring match on Go source lines (excluding comments and test files).
	forbidden := []struct {
		pattern string
		reason  string
	}{
		{
			pattern: `== "127.0.0.1"`,
			reason:  "ad-hoc loopback detection — use config.ResolveDialTarget or config.IsLoopbackEndpoint",
		},
		{
			pattern: `== "::1"`,
			reason:  "ad-hoc loopback detection — use config.ResolveDialTarget or config.IsLoopbackEndpoint",
		},
		{
			pattern: `host == "localhost"`,
			reason:  "ad-hoc loopback detection — use config.ResolveDialTarget or config.IsLoopbackEndpoint",
		},
	}

	// Directories to scan. We focus on service implementations, not
	// third-party code, generated code, or the resolver itself.
	scanDirs := []string{
		"../cluster_controller/cluster_controller_server",
		"../cluster_doctor/cluster_doctor_server",
		"../node_agent/node_agent_server",
		"../ai_executor/ai_executor_server",
		"../backup_manager/backup_manager_server",
		"../workflow/workflow_server",
	}

	// Files that are allowed to have these patterns. Includes the resolver
	// itself, known I-class sites (awaiting cert-SAN changes), and
	// non-dialer code that legitimately checks loopback for other reasons
	// (validation, service config, etcd member management).
	allowlist := map[string]bool{
		"endpoint_resolver.go":      true,
		"endpoint_resolver_test.go": true,
		// I-class: InsecureSkipVerify sites awaiting cert-SAN changes
		"release_resolver.go": true,
		"dns_reconciler.go":   true,
		// Non-dialer loopback checks (validation, config, discovery)
		"etcd_members.go":      true,
		"service_config.go":    true,
		"validation.go":        true,
		"service_discovery.go": true,
		// Non-dialer: local-node detection in backup_manager
		"node_tasks.go": true,
	}

	var violations []string

	for _, dir := range scanDirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			base := filepath.Base(path)
			if allowlist[base] {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				// Skip comments.
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				for _, f := range forbidden {
					if strings.Contains(line, f.pattern) {
						violations = append(violations,
							filepath.Base(path)+":"+
								string(rune('0'+i/100))+string(rune('0'+(i/10)%10))+string(rune('0'+i%10))+
								" — "+f.reason)
					}
				}
			}
			return nil
		})
	}

	if len(violations) > 0 {
		t.Errorf("found %d ad-hoc loopback/SNI patterns that should use config.ResolveDialTarget:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v)
		}
	}
}
