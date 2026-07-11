package main

// D1c 1a — PlacementGeneration: controller-owned per-node placement-intent
// version, bumped once per real (set-level) placement change via the single
// canonical owner mutation, and durable across restart.

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Test 1 (mechanism) + 3 + 4 + 5 + 6: the canonical mutation.
func TestApplyNodePlacementProfilesLocked(t *testing.T) {
	n := &nodeState{} // fresh: gen 0, no profiles (as a legacy/never-established node loads)

	// Establishing placement on a gen-0 node advances to 1 (test 1 mechanism).
	if !applyNodePlacementProfilesLocked(n, []string{"core", "storage"}) {
		t.Fatalf("establishing placement must report a change")
	}
	if n.PlacementGeneration != 1 {
		t.Fatalf("first established placement must be generation 1, got %d", n.PlacementGeneration)
	}

	// A real profile change increments exactly once (test 3).
	if !applyNodePlacementProfilesLocked(n, []string{"core", "storage", "control-plane"}) {
		t.Fatalf("a real change must report changed")
	}
	if n.PlacementGeneration != 2 {
		t.Fatalf("a real change increments exactly once: want 2, got %d", n.PlacementGeneration)
	}

	// Reapplying the SAME set does not increment (test 4).
	before := n.PlacementGeneration
	if applyNodePlacementProfilesLocked(n, []string{"core", "storage", "control-plane"}) {
		t.Errorf("reapplying the same set must report no change")
	}
	if n.PlacementGeneration != before {
		t.Errorf("idempotent re-apply must not bump: was %d, now %d", before, n.PlacementGeneration)
	}

	// Reordering an equivalent set does not increment (test 5).
	if applyNodePlacementProfilesLocked(n, []string{"control-plane", "core", "storage"}) {
		t.Errorf("reordering an equivalent set must report no change")
	}
	if n.PlacementGeneration != before {
		t.Errorf("reorder must not bump: was %d, now %d", before, n.PlacementGeneration)
	}

	// Coupling (test 6): the generation never advances without a profile change,
	// and a change always advances it — proven by the assertions above (every
	// bump accompanied a real change; every no-op left it fixed).
}

// Test 2: a zero generation is unestablished and can never authorize grants.
func TestPlacementFreshnessEstablished(t *testing.T) {
	if placementFreshnessEstablished(&nodeState{PlacementGeneration: 0}) {
		t.Errorf("generation 0 must be UNESTABLISHED (cannot authorize grants)")
	}
	if placementFreshnessEstablished(nil) {
		t.Errorf("nil node is not established")
	}
	if !placementFreshnessEstablished(&nodeState{PlacementGeneration: 1}) {
		t.Errorf("generation 1 must be established")
	}
}

// Test 8: the generation is persisted (survives controller restart via the
// nodeState JSON round-trip that persistState uses).
func TestPlacementGenerationSurvivesRestart(t *testing.T) {
	orig := &nodeState{NodeID: "n1", Profiles: []string{"core"}, PlacementGeneration: 7}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got nodeState
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.PlacementGeneration != 7 {
		t.Fatalf("PlacementGeneration must survive persist round-trip: want 7, got %d", got.PlacementGeneration)
	}
	// And a legacy record (no field) loads as 0 = unestablished (idempotent migration).
	var legacy nodeState
	if err := json.Unmarshal([]byte(`{"node_id":"old","profiles":["core"]}`), &legacy); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if legacy.PlacementGeneration != 0 {
		t.Fatalf("legacy node must load as generation 0 (unestablished), got %d", legacy.PlacementGeneration)
	}
}

// Test 7: every nodeState.Profiles MUTATION routes through the canonical owner
// mutation. No handler may assign `node.Profiles =` directly — a direct write
// would change placement without advancing PlacementGeneration, breaking the
// coupling. The only permitted `node.Profiles =` is inside the canonical
// function itself (placement_generation.go).
func TestNoDirectNodeProfilesMutationOutsideCanonicalPath(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	// Matches `node.Profiles =` (assignment) but not `==`, and not the creation
	// literals (`Profiles:`), which initialize a coupled (profiles, generation)
	// pair rather than mutate an existing placement.
	directWrite := regexp.MustCompile(`\bnode\.Profiles\s*=[^=]`)
	var offenders []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == "placement_generation.go" {
			continue // the canonical function is the ONE permitted writer
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if directWrite.MatchString(line) {
				offenders = append(offenders, name+":"+strconv.Itoa(i+1)+" "+strings.TrimSpace(line))
			}
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("node.Profiles must only be mutated via applyNodePlacementProfilesLocked "+
			"(else placement changes without advancing PlacementGeneration). Direct writes found:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
