package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesiredWriteProvenance_ControllerWiring(t *testing.T) {
	cases := []struct {
		path    string
		pattern string
	}{
		{"desired_state_handlers.go", "audittrail.WriteDesiredWriteRecord"},
		{"desired_state_handlers.go", "Source:    \"upsertOne\""},
		{"reconcile_runtime.go", "Source:    \"reconcileDesiredFromRepository\""},
		{"handlers_status.go", "Source:    \"ApplyServiceDesiredVersion\""},
	}
	for _, tc := range cases {
		data, err := os.ReadFile(filepath.Join(".", tc.path))
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		if !strings.Contains(string(data), tc.pattern) {
			t.Fatalf("%s missing required provenance wiring pattern %q", tc.path, tc.pattern)
		}
	}
}
