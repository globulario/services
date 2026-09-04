package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// A step that waits for a dependency on a COLD node may never be given a
// smaller budget than the step that waits for the same dependency on a WARM
// one.
//
// SCAR — 2026-08-19, 5-node simulation. node.join's verify_prerequisites
// allowed 6 attempts at 5s = 30s for etcd to become active, while
// day0.bootstrap's verify_etcd_healthy allowed 30 attempts at 5s = 150s for
// the same condition. The join path is the harder case — it runs on a node
// joining for the first time, or rejoining after a wipe, whose etcd is
// starting from an empty data dir — and it had one fifth the budget.
//
// Measured: first attempt 02:30:31, gave up 02:30:56, etcd active 02:31:09.
// The join failed by 13 seconds on a node that was working correctly.
//
// The cost was not a retry. node.join FAILED, so the install owner never
// stamped installed-state receipts; the heartbeat then backfilled records from
// the local package cache with no installed_by, and cluster-doctor fail-closed
// CRITICAL on seven units. Five upgrade scenarios and two authority scenarios
// failed on doctor cleanliness, all from those 13 seconds.
//
// This test compares the two budgets rather than asserting a magic number, so
// it keeps holding if either path's timing is retuned.
func TestJoinEtcdPrerequisiteBudgetIsNotSmallerThanDay0(t *testing.T) {
	joinBudget, joinLabel := stepRetryBudget(t, "node.join.yaml", "verify_prerequisites")
	day0Budget, day0Label := stepRetryBudget(t, "day0.bootstrap.yaml", "verify_etcd_healthy")

	if joinBudget < day0Budget {
		t.Fatalf("the cold-start path has a smaller etcd budget than the warm one:\n"+
			"  node.join/verify_prerequisites   %s (%s)\n"+
			"  day0.bootstrap/verify_etcd_healthy %s (%s)\n"+
			"A node joining from an empty data dir needs at least as long as one that is already running.",
			joinBudget, joinLabel, day0Budget, day0Label)
	}
}

// Whatever else changes, the join must tolerate an etcd cold start that was
// measured at ~38s, with margin for a loaded host.
func TestJoinEtcdPrerequisiteToleratesAMeasuredColdStart(t *testing.T) {
	const measuredColdStart = 38 * time.Second
	const requiredMargin = 2

	budget, label := stepRetryBudget(t, "node.join.yaml", "verify_prerequisites")
	if budget < measuredColdStart*requiredMargin {
		t.Fatalf("verify_prerequisites allows %s (%s); a cold etcd start measured %s, so the budget must be at least %s",
			budget, label, measuredColdStart, measuredColdStart*requiredMargin)
	}
}

// stepRetryBudget returns maxAttempts x backoff for one step of one workflow
// definition. It fails the test rather than returning a zero value: a step that
// cannot be found is a renamed or deleted guard, which must be noticed, not
// silently treated as a budget of zero.
func stepRetryBudget(t *testing.T, definition, stepID string) (time.Duration, string) {
	t.Helper()

	path := filepath.Join("definitions", definition)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	step := findStep(document, stepID)
	if step == nil {
		t.Fatalf("%s has no step %q — if it was renamed, update this test rather than deleting it: "+
			"the budget it guards is why a cold join stopped failing", definition, stepID)
	}
	retry, ok := step["retry"].(map[string]any)
	if !ok {
		t.Fatalf("%s step %q has no retry policy, so it gets one attempt at a dependency that starts cold", definition, stepID)
	}
	attempts, ok := asInt(retry["maxAttempts"])
	if !ok || attempts < 1 {
		t.Fatalf("%s step %q has an unusable maxAttempts: %v", definition, stepID, retry["maxAttempts"])
	}
	backoff, err := parseDuration(fmt.Sprint(retry["backoff"]))
	if err != nil {
		t.Fatalf("%s step %q has an unparseable backoff %v: %v", definition, stepID, retry["backoff"], err)
	}
	return time.Duration(attempts) * backoff, fmt.Sprintf("%dx%s", attempts, retry["backoff"])
}

func findStep(node any, stepID string) map[string]any {
	switch typed := node.(type) {
	case map[string]any:
		if id, ok := typed["id"].(string); ok && id == stepID {
			return typed
		}
		for _, value := range typed {
			if found := findStep(value, stepID); found != nil {
				return found
			}
		}
	case []any:
		for _, value := range typed {
			if found := findStep(value, stepID); found != nil {
				return found
			}
		}
	}
	return nil
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	}
	return 0, false
}

var durationPattern = regexp.MustCompile(`^(\d+)(ms|s|m|h)$`)

func parseDuration(value string) (time.Duration, error) {
	match := durationPattern.FindStringSubmatch(value)
	if match == nil {
		return 0, fmt.Errorf("unsupported duration %q", value)
	}
	amount, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	unit := map[string]time.Duration{
		"ms": time.Millisecond,
		"s":  time.Second,
		"m":  time.Minute,
		"h":  time.Hour,
	}[match[2]]
	return time.Duration(amount) * unit, nil
}
