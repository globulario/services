package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoLocalEngineInMigratedServices is a structural regression test that
// verifies no migrated service still instantiates a local engine.Engine for
// production workflow execution. All production workflows must execute
// through WorkflowService.ExecuteWorkflow.
//
// This is Track A.1 from test/strategy.md: "no migrated workflow still uses
// local embed/filesystem production path."
//
// The test scans service source files for patterns that indicate local engine
// usage (engine.Engine{, eng.Execute, engine.NewRouter()) and verifies they
// only appear in allowed contexts (test files, actor_service.go, the engine
// package itself).
func TestNoLocalEngineInMigratedServices(t *testing.T) {
	// Patterns that indicate local engine instantiation.
	forbidden := []string{
		"engine.Engine{",
		"eng.Execute(",
		"eng := &engine.Engine",
	}

	// Migrated service directories — these must NOT use local engine.
	scanDirs := []string{
		"../../cluster_controller/cluster_controller_server",
		"../../cluster_doctor/cluster_doctor_server",
	}

	// Files that are allowed to have these patterns.
	allowlist := map[string]bool{
		// The engine package itself (we're scanning service dirs, not engine)
		// Actor service files use engine.Router but not engine.Engine
		"actor_service.go": true,
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
			// Skip test files — tests may use local engine for unit testing.
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
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				for _, pattern := range forbidden {
					if strings.Contains(line, pattern) {
						violations = append(violations,
							filepath.Base(path)+":"+
								itoa(i+1)+" — contains '"+pattern+"'")
					}
				}
			}
			return nil
		})
	}

	if len(violations) > 0 {
		t.Errorf("found %d local engine instantiations in migrated services (should use WorkflowService.ExecuteWorkflow):", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v)
		}
		t.Error("Production workflows must execute through the centralized WorkflowService.")
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
