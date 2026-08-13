package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// cluster.invariant.enforcement owns node reachability classification, partition
// fencing, the quorum-loss emergency alert and the PKI invariant. It was fully
// authored, seeded into etcd, posture-classified (WorkClassLiveness) and unit
// tested — and never dispatched by anything, so on a live cluster it had run
// exactly zero times. Two nodes stopped for 17.5 minutes were never fenced, and
// fm:failure.expired_service_cert_is_not_re_issued... has the same root.
//
// Unit tests over the handlers cannot catch that: they call the handlers
// directly. What was missing was the CALL SITE, so that is what these assert.

func TestInvariantEnforcementWorkflowHasACallSite(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	callRe := regexp.MustCompile(`RunInvariantEnforcementWorkflow\s*\(`)
	declRe := regexp.MustCompile(`func\s+\(srv\s+\*server\)\s+RunInvariantEnforcementWorkflow`)

	callers := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || declRe.MatchString(line) {
				continue
			}
			if callRe.MatchString(line) {
				callers++
			}
		}
	}

	if callers == 0 {
		t.Fatal("nothing calls RunInvariantEnforcementWorkflow — partition fencing, " +
			"the quorum-loss alert and the PKI invariant would never run on a live " +
			"cluster, no matter how well their handlers are tested")
	}
}

// The lane must be started from the server's startup path, not merely defined.
func TestInvariantEnforcementLaneIsStartedAtBoot(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	// Skip comments. A substring match alone is satisfied by the very line
	// this test exists to catch being commented out, which makes the test
	// incapable of failing — the same shape of defect as the bug it guards.
	started := false
	for _, line := range strings.Split(string(src), "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "//") {
			continue
		}
		if strings.Contains(code, "StartInvariantEnforcementLane(") {
			started = true
			break
		}
	}
	if !started {
		t.Fatal("controller startup never starts the invariant enforcement lane; " +
			"defining the lane without starting it leaves the workflow dormant")
	}
}

// Cadence has to be well inside the workflow's own CRITICAL threshold (15 min),
// or a node crosses the line and waits a long time to be fenced.
func TestInvariantLaneCadenceIsInsideTheCriticalWindow(t *testing.T) {
	const criticalThreshold = 15 * time.Minute
	if invariantLaneInterval <= 0 {
		t.Fatal("lane interval must be positive")
	}
	if invariantLaneInterval >= criticalThreshold {
		t.Fatalf("lane interval %v is not inside the %v CRITICAL window — fencing "+
			"would lag the condition it detects", invariantLaneInterval, criticalThreshold)
	}
	if invariantLaneRunTimeout <= 0 {
		t.Fatal("each run must be bounded, or a wedged dispatch silences the lane forever")
	}
}
