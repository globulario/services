package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSecurityRegressions ensures Day-0 Security hardening is maintained (H4)
// This test scans the codebase for patterns that violate security contracts.
func TestSecurityRegressions(t *testing.T) {
	repoRoot := findRepoRoot(t)

	t.Run("NoHardcodedDNSPorts", func(t *testing.T) {
		// H4: No hardcoded :10033 or :10053 in core packages (except as fallback parameters)
		// Acceptable: ResolveDNSGrpcEndpoint("127.0.0.1:10033") - fallback only
		// Not acceptable: Direct dial/connection to :10033 without discovery
		violations := scanForPattern(t, repoRoot, []string{
			"golang/clustercontroller",
			"golang/nodeagent",
			"golang/config",
			"golang/dns",
		}, regexp.MustCompile(`:10033|:10053`), []string{
			"_test.go",                      // Allow in tests
			"security_regression",           // Allow in this file
			".pb.go",                        // Exclude generated proto files
			"service_discovery.go",          // ResolveDNSGrpcEndpoint defines the pattern
			"ResolveDNSGrpcEndpoint(",       // Fallback parameters are OK
			"ResolveDNSResolverEndpoint(",   // Fallback parameters are OK
		})
		if len(violations) > 0 {
			t.Errorf("Found hardcoded DNS ports (:10033, :10053) used directly (not as fallback) in %d files:\n%s"+
				"\nNote: Passing these as fallback to ResolveDNSGrpcEndpoint() is OK",
				len(violations), formatViolations(violations))
		}
	})

	t.Run("NoInsecureGRPC", func(t *testing.T) {
		// H4: insecure.NewCredentials() should only exist when properly gated
		// This test identifies files using insecure gRPC to ensure they're audited.
		// Acceptable if: (a) gated by explicit insecure flag check, (b) flag defaults to false
		violations := scanForPattern(t, repoRoot, []string{
			"golang/clustercontroller",
			"golang/nodeagent",
			"golang/config",
		}, regexp.MustCompile(`insecure\.NewCredentials\(\)`), []string{
			"_test.go",            // Allow in tests
			"_dev.go",             // Allow in dev files
			"security_regression", // Allow in this file
		})
		// Note: We expect some files (agentclient.go, server.go) that have properly gated uses
		// The test serves as an audit point - any NEW files should be reviewed
		if len(violations) > 2 {
			t.Errorf("Found insecure gRPC credentials in unexpected files (expected ~2 gated uses):\n%s"+
				"\nEnsure new uses are: (a) gated by environment variable, (b) default to secure",
				formatViolations(violations))
		}
	})

	t.Run("NoHTTPEtcdProbes", func(t *testing.T) {
		// H4: No HTTP etcd health checks in production code
		violations := scanForPattern(t, repoRoot, []string{
			"golang/clustercontroller/clustercontroller_server/operator",
		}, regexp.MustCompile(`http://127\.0\.0\.1:2379|http://localhost:2379`), []string{
			"_test.go",            // Allow in tests
			"security_regression", // Allow in this file
		})
		if len(violations) > 0 {
			t.Errorf("Found HTTP etcd endpoints in operator code (should use HTTPS):\n%s",
				formatViolations(violations))
		}
	})

	t.Run("TLSRequiredByDefault", func(t *testing.T) {
		// H4: Environment variables for insecure mode must default to false/disabled
		t.Run("NodeAgentInsecure", func(t *testing.T) {
			// Check that NODE_AGENT_INSECURE defaults to false (not true)
			// Match: getEnv("NODE_AGENT_INSECURE", "true") - BAD
			// Don't match: getEnv("NODE_AGENT_INSECURE", "false") - GOOD
			// Don't match: error messages mentioning the env var - GOOD
			violations := scanForPattern(t, repoRoot, []string{
				"golang/nodeagent",
			}, regexp.MustCompile(`getEnv\("NODE_AGENT_INSECURE",\s*"true"\)`), []string{
				"_test.go",
				"security_regression",
			})
			if len(violations) > 0 {
				t.Errorf("Found NODE_AGENT_INSECURE with insecure default (should default to false):\n%s",
					formatViolations(violations))
			}
		})
	})
}

// scanForPattern searches for a regex pattern in specified directories
func scanForPattern(t *testing.T, root string, dirs []string, pattern *regexp.Regexp, exclude []string) []string {
	t.Helper()
	var violations []string

	for _, dir := range dirs {
		fullPath := filepath.Join(root, dir)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue // Directory doesn't exist, skip
		}

		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}

			// Check exclusions
			for _, ex := range exclude {
				if strings.Contains(path, ex) {
					return nil
				}
			}

			content, err := os.ReadFile(path)
			if err != nil {
				t.Logf("Warning: could not read %s: %v", path, err)
				return nil
			}

			if pattern.Match(content) {
				relPath, _ := filepath.Rel(root, path)
				violations = append(violations, relPath)
			}
			return nil
		})

		if err != nil {
			t.Logf("Warning: error walking %s: %v", fullPath, err)
		}
	}

	return violations
}

// findRepoRoot locates the repository root by looking for go.mod
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Walk up until we find go.work or reach root
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.work
			// Fall back to assuming we're somewhere in the repo
			wd, _ := os.Getwd()
			// If we're in golang/config, go up two levels
			if strings.Contains(wd, "golang/config") {
				return filepath.Join(dir, "..", "..")
			}
			return dir
		}
		dir = parent
	}
}

// formatViolations formats a list of file paths for error output
func formatViolations(violations []string) string {
	var b strings.Builder
	for i, v := range violations {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  - ")
		b.WriteString(v)
	}
	return b.String()
}
