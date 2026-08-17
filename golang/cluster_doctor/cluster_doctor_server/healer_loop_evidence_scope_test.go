package main

import "testing"

func TestDecideHealerExecutionPreservesEnforceUnderReducedHarvest(t *testing.T) {
	got := decideHealerExecution("enforce", true)
	if got.mode != "enforce" {
		t.Fatalf("mode = %q, want enforce: aggregate snapshot incompleteness must not become a cluster-wide remediation veto", got.mode)
	}
	if !got.reducedHarvest {
		t.Fatalf("reducedHarvest = false, want true: the diagnostic downgrade must remain visible")
	}
}

func TestDecideHealerExecutionPreservesNonEnforceModes(t *testing.T) {
	for _, mode := range []string{"observe", "dry_run"} {
		got := decideHealerExecution(mode, true)
		if got.mode != mode {
			t.Fatalf("mode = %q, want %q", got.mode, mode)
		}
		if !got.reducedHarvest {
			t.Fatalf("mode %q lost the reduced-harvest diagnostic signal", mode)
		}
	}
}
