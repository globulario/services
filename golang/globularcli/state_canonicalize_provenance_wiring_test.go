package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalizeA2_EmitsDesiredWriteProvenance(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(".", "state_cmds.go"))
	if err != nil {
		t.Fatalf("read state_cmds.go: %v", err)
	}
	src := string(data)
	for _, pat := range []string{
		"audittrail.WriteDesiredWriteRecord",
		"Source:    \"state.canonicalize.fix-safe-A2\"",
		"Action:    \"backfill_build_id\"",
	} {
		if !strings.Contains(src, pat) {
			t.Fatalf("state_cmds.go missing provenance pattern %q", pat)
		}
	}
}

func TestCanonicalizeLegacyBackfill_EmitsDesiredWriteProvenance(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(".", "state_cmds.go"))
	if err != nil {
		t.Fatalf("read state_cmds.go: %v", err)
	}
	src := string(data)
	for _, pat := range []string{
		"repairDesiredWriteProvenanceLegacy",
		"Source:    \"state.canonicalize.provenance-backfill\"",
		"Action:    \"legacy_backfill_marker\"",
	} {
		if !strings.Contains(src, pat) {
			t.Fatalf("state_cmds.go missing provenance backfill pattern %q", pat)
		}
	}
}
