package main

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// ─── Pure-logic decision tests ───────────────────────────────────────────────

// TestShouldSkipForAbandoned: dispatch must be refused for abandoned
// correlations and proceed for everything else (nil row, count below
// max, manually cleared).
func TestShouldSkipForAbandoned(t *testing.T) {
	if shouldSkipForAbandoned(nil) {
		t.Errorf("nil state must not trigger skip")
	}
	live := &CorrelationDeferState{Abandoned: false, DeferCount: 3, MaxDefers: 5}
	if shouldSkipForAbandoned(live) {
		t.Errorf("count below max must not trigger skip")
	}
	abandoned := &CorrelationDeferState{Abandoned: true, DeferCount: 5, MaxDefers: 5}
	if !shouldSkipForAbandoned(abandoned) {
		t.Errorf("abandoned must trigger skip")
	}
}

// TestNextDeferStateFirstDefer: first defer for a fresh correlation
// must produce defer_count=1, max from policy, abandoned=false (unless
// max_defers=1 in which case it abandons immediately).
func TestNextDeferStateFirstDefer(t *testing.T) {
	now := time.Now()
	ds := &engine.DeferState{
		StepID:      "verify_runtime",
		DeferUntil:  now.Add(60 * time.Second),
		DeferCount:  1,
		BlockerTags: []string{"runtime.active:keepalived@nuc"},
		Reason:      "inactive",
	}
	out := nextDeferState(nil, ds, now)
	if out == nil {
		t.Fatal("nextDeferState returned nil")
	}
	if out.DeferCount != 1 {
		t.Errorf("DeferCount = %d, want 1", out.DeferCount)
	}
	if out.MaxDefers != defaultB3MaxDefers {
		t.Errorf("MaxDefers = %d, want default %d", out.MaxDefers, defaultB3MaxDefers)
	}
	if out.Abandoned {
		t.Errorf("Abandoned = true on first defer with default max=%d", defaultB3MaxDefers)
	}
	if out.LastStepID != "verify_runtime" {
		t.Errorf("LastStepID = %q", out.LastStepID)
	}
	if len(out.LastBlockerTags) != 1 || out.LastBlockerTags[0] != "runtime.active:keepalived@nuc" {
		t.Errorf("LastBlockerTags = %v", out.LastBlockerTags)
	}
}

// TestNextDeferStateAbandonsAtMax: the count→max threshold flips
// abandoned=true and stamps abandoned_at, exactly once.
func TestNextDeferStateAbandonsAtMax(t *testing.T) {
	now := time.Now()
	ds := &engine.DeferState{StepID: "verify_runtime", DeferUntil: now, Reason: "inactive"}

	// Walk count from 0 → 5 by feeding the previous output back in.
	// The 5th defer should flip abandoned=true.
	state := nextDeferState(nil, ds, now) // 1
	if state.Abandoned {
		t.Fatal("abandoned at count=1 with default max=5")
	}
	state = nextDeferState(state, ds, now) // 2
	state = nextDeferState(state, ds, now) // 3
	state = nextDeferState(state, ds, now) // 4
	if state.Abandoned {
		t.Fatal("abandoned at count=4 with default max=5")
	}
	state = nextDeferState(state, ds, now) // 5
	if state.DeferCount != 5 {
		t.Errorf("DeferCount = %d, want 5", state.DeferCount)
	}
	if !state.Abandoned {
		t.Fatal("count=5/max=5 must flip abandoned=true")
	}
	if state.AbandonedAt.IsZero() {
		t.Error("AbandonedAt must be stamped on transition")
	}
	// Idempotent: another defer past the threshold doesn't flip the
	// AbandonedAt stamp around (preserves the original transition time).
	originalAt := state.AbandonedAt
	state = nextDeferState(state, ds, now.Add(time.Minute)) // 6
	if state.AbandonedAt != originalAt {
		t.Errorf("AbandonedAt drifted from %v to %v on subsequent defer", originalAt, state.AbandonedAt)
	}
}

// ─── Memory store contract tests ─────────────────────────────────────────────

// TestMemoryStoreCounterSurvivesAcrossCalls: the most basic guarantee
// — sequential RecordDefer calls increment correctly. Models multiple
// runs of the same correlation_id deferring without restart.
func TestMemoryStoreCounterSurvivesAcrossCalls(t *testing.T) {
	store := newMemoryDeferStateStore()
	ctx := context.Background()
	now := time.Now()
	ds := &engine.DeferState{StepID: "s", DeferUntil: now, Reason: "r"}

	for i := 1; i <= 4; i++ {
		state, err := store.RecordDefer(ctx, "globular.internal", "correlation-A", ds)
		if err != nil {
			t.Fatalf("call %d RecordDefer: %v", i, err)
		}
		if state.DeferCount != i {
			t.Errorf("after call %d: DeferCount = %d, want %d", i, state.DeferCount, i)
		}
		if state.Abandoned {
			t.Errorf("after call %d (count=%d/max=%d): unexpected abandoned",
				i, state.DeferCount, state.MaxDefers)
		}
	}
}

// TestMemoryStoreSurvivesWorkflowServerRestart is the regression guard
// for "in-memory counter would lose state on restart". We model two
// distinct workflow_server processes by creating two separate "server"
// fixtures that BOTH point at the same store (same Scylla in prod).
// Counter incremented by writer-1 must be visible to writer-2.
func TestMemoryStoreSurvivesWorkflowServerRestart(t *testing.T) {
	store := newMemoryDeferStateStore()
	ctx := context.Background()
	cluster := "globular.internal"
	corr := "InfrastructureRelease/core@globular.io/keepalived"
	ds := &engine.DeferState{StepID: "verify_runtime", Reason: "inactive"}

	// "Original workflow_server": records 3 defers.
	for i := 1; i <= 3; i++ {
		if _, err := store.RecordDefer(ctx, cluster, corr, ds); err != nil {
			t.Fatalf("pre-restart record %d: %v", i, err)
		}
	}

	// "Workflow_server restarts" — fresh code path holding the same
	// store reference. New reads must see the 3 defers.
	state, err := store.Get(ctx, cluster, corr)
	if err != nil {
		t.Fatalf("post-restart Get: %v", err)
	}
	if state == nil {
		t.Fatal("post-restart Get returned nil — counter lost across restart")
	}
	if state.DeferCount != 3 {
		t.Errorf("post-restart DeferCount = %d, want 3", state.DeferCount)
	}
	if state.Abandoned {
		t.Errorf("post-restart abandoned at count=3/max=%d", state.MaxDefers)
	}

	// Continuing from the restored state, the 5th defer must still be
	// the one that flips abandoned (i.e. counter resumed correctly,
	// did NOT reset to 0).
	if _, err := store.RecordDefer(ctx, cluster, corr, ds); err != nil {
		t.Fatalf("post-restart 4th defer: %v", err)
	}
	state, _ = store.Get(ctx, cluster, corr)
	if state.Abandoned {
		t.Errorf("abandoned at count=4 — should still be below max %d", state.MaxDefers)
	}
	if _, err := store.RecordDefer(ctx, cluster, corr, ds); err != nil {
		t.Fatalf("post-restart 5th defer: %v", err)
	}
	state, _ = store.Get(ctx, cluster, corr)
	if !state.Abandoned {
		t.Errorf("count=5/max=%d must be abandoned post-restart", state.MaxDefers)
	}
}

// TestMemoryStoreClearOnSuccess: a successful run resets the counter
// so the NEXT defer starts at 1, not at the prior count.
func TestMemoryStoreClearOnSuccess(t *testing.T) {
	store := newMemoryDeferStateStore()
	ctx := context.Background()
	cluster := "c1"
	corr := "release/foo"
	ds := &engine.DeferState{StepID: "s", Reason: "r"}

	// Build up some history.
	for i := 0; i < 3; i++ {
		_, _ = store.RecordDefer(ctx, cluster, corr, ds)
	}

	if err := store.ClearOnSuccess(ctx, cluster, corr); err != nil {
		t.Fatalf("ClearOnSuccess: %v", err)
	}
	state, _ := store.Get(ctx, cluster, corr)
	if state != nil {
		t.Errorf("expected nil state after clear, got %+v", state)
	}

	// Next defer starts at 1.
	state, _ = store.RecordDefer(ctx, cluster, corr, ds)
	if state.DeferCount != 1 {
		t.Errorf("post-clear first defer count = %d, want 1", state.DeferCount)
	}
}

// TestMemoryStoreClearByOperator: operator clear resets defer_count
// AND clears the abandoned flag, so dispatch can resume.
func TestMemoryStoreClearByOperator(t *testing.T) {
	store := newMemoryDeferStateStore()
	ctx := context.Background()
	cluster := "c1"
	corr := "release/abandoned"
	ds := &engine.DeferState{StepID: "s", Reason: "r"}

	// Defer 5 times to get abandoned=true.
	for i := 0; i < 5; i++ {
		_, _ = store.RecordDefer(ctx, cluster, corr, ds)
	}
	state, _ := store.Get(ctx, cluster, corr)
	if !state.Abandoned {
		t.Fatal("setup: expected abandoned after 5 defers")
	}

	if err := store.ClearByOperator(ctx, cluster, corr, "dave"); err != nil {
		t.Fatalf("ClearByOperator: %v", err)
	}
	state, _ = store.Get(ctx, cluster, corr)
	if state.Abandoned {
		t.Error("operator clear must flip abandoned=false")
	}
	if state.DeferCount != 0 {
		t.Errorf("operator clear DeferCount = %d, want 0", state.DeferCount)
	}
	if state.ClearedBy != "dave" {
		t.Errorf("ClearedBy = %q, want dave", state.ClearedBy)
	}
	if state.ClearedAt.IsZero() {
		t.Error("ClearedAt must be stamped")
	}
}

// TestMemoryStoreUnrelatedCorrelationsIndependent: a correlation hitting
// abandoned must not affect any other correlation. The whole point of
// the per-correlation circuit breaker pattern.
func TestMemoryStoreUnrelatedCorrelationsIndependent(t *testing.T) {
	store := newMemoryDeferStateStore()
	ctx := context.Background()
	ds := &engine.DeferState{StepID: "s", Reason: "r"}

	// Push correlation A to abandoned.
	for i := 0; i < 5; i++ {
		_, _ = store.RecordDefer(ctx, "c1", "A", ds)
	}
	stateA, _ := store.Get(ctx, "c1", "A")
	if !stateA.Abandoned {
		t.Fatal("setup: A not abandoned")
	}

	// Independent correlation B in the same cluster.
	stateB, err := store.Get(ctx, "c1", "B")
	if err != nil {
		t.Fatalf("Get B: %v", err)
	}
	if stateB != nil {
		t.Errorf("B should be untouched, got %+v", stateB)
	}
	stateB, _ = store.RecordDefer(ctx, "c1", "B", ds)
	if stateB.Abandoned {
		t.Errorf("B abandoned after one defer — A's state leaked")
	}
	if stateB.DeferCount != 1 {
		t.Errorf("B DeferCount = %d, want 1", stateB.DeferCount)
	}

	// Independent correlation B' in a different cluster.
	stateBPrime, _ := store.RecordDefer(ctx, "c2", "B", ds)
	if stateBPrime.DeferCount != 1 {
		t.Errorf("c2/B DeferCount = %d, want 1 — cluster scoping leaked", stateBPrime.DeferCount)
	}
}

// TestDispatchOrderAbandonedBeforeCooldown: when a correlation is BOTH
// abandoned AND has an active cooldown row, the abandonment check must
// fire FIRST (most specific signal). The composition of B2 and B3 is
// abandoned-first, cooldown-second.
func TestDispatchOrderAbandonedBeforeCooldown(t *testing.T) {
	now := time.Now()

	// B2 cooldown row says "skip until t+60s".
	cool := &workflowpb.WorkflowRun{
		Status:         workflowpb.RunStatus_RUN_STATUS_DEFERRED,
		BackoffUntilMs: now.Add(60 * time.Second).UnixMilli(),
		RetryAttempt:   1,
	}
	if !shouldSkipForDeferral(cool, now) {
		t.Fatal("setup: B2 should say skip-cooldown")
	}

	// B3 abandoned state says "abandoned forever".
	abandoned := &CorrelationDeferState{Abandoned: true, DeferCount: 5, MaxDefers: 5}
	if !shouldSkipForAbandoned(abandoned) {
		t.Fatal("setup: B3 should say skip-abandoned")
	}

	// The integration is in ExecuteWorkflow — the abandoned branch
	// must return BEFORE the cooldown branch. Here we just assert
	// both signals would independently fire, so the executor's
	// ordered chain (B3 → B2 → engine) is the correct composition.
	// The dispatch test (executor_defer_test) covers the runtime
	// ordering once integrated.
}

// ─── WF-DEFER B4 — wake-by-blocker-tag pure logic ────────────────────────────

// TestFindByBlockerTag_MatchesNonAbandoned: wake should target rows
// whose tags contain the wake key AND are not abandoned. Abandoned
// rows are skipped — the operator-clear path is the only way to
// resume an abandoned correlation.
func TestFindByBlockerTag_MatchesNonAbandoned(t *testing.T) {
	tag := "runtime.active:nuc:keepalived"
	rows := []*CorrelationDeferState{
		{
			CorrelationID:    "match-A",
			LastBlockerTags:  []string{tag, "deps.installed:nuc:keepalived"},
		},
		{
			CorrelationID:    "match-B",
			LastBlockerTags:  []string{tag},
		},
		{
			CorrelationID:    "non-match",
			LastBlockerTags:  []string{"runtime.active:dell:scylla"},
		},
		{
			CorrelationID:    "abandoned-with-tag",
			LastBlockerTags:  []string{tag},
			Abandoned:        true,
		},
		nil,
	}
	out := findByBlockerTagFromList(rows, tag)
	if len(out) != 2 {
		t.Fatalf("matched %d, want 2", len(out))
	}
	got := map[string]bool{}
	for _, r := range out {
		got[r.CorrelationID] = true
	}
	if !got["match-A"] || !got["match-B"] {
		t.Errorf("missing expected matches: got %v", got)
	}
	if got["abandoned-with-tag"] {
		t.Errorf("abandoned correlation must not be returned for wake")
	}
	if got["non-match"] {
		t.Errorf("non-matching tag must not be returned")
	}
}

// TestFindByBlockerTag_EmptyTagIsNoop: callers pass exactly one tag
// per wake call. An empty tag is a programming error but must not
// match every row — the safer behavior is "match none".
func TestFindByBlockerTag_EmptyTagIsNoop(t *testing.T) {
	rows := []*CorrelationDeferState{
		{CorrelationID: "x", LastBlockerTags: []string{"some.tag"}},
	}
	if out := findByBlockerTagFromList(rows, ""); len(out) != 0 {
		t.Errorf("empty tag matched %d rows; want 0", len(out))
	}
}

// TestMemoryStoreFindByBlockerTag: integration test against the
// in-memory store. Records two defers with rendered tags, then asks
// the store to find them by tag. Documents the production read flow.
func TestMemoryStoreFindByBlockerTag(t *testing.T) {
	ctx := context.Background()
	store := newMemoryDeferStateStore()

	store.RecordDefer(ctx, "c1", "rel/keepalived/nuc", &engine.DeferState{
		StepID:      "verify_runtime",
		DeferUntil:  time.Now().Add(60 * time.Second),
		BlockerTags: []string{"runtime.active:nuc:keepalived"},
	})
	store.RecordDefer(ctx, "c1", "rel/keepalived/ryzen", &engine.DeferState{
		StepID:      "verify_runtime",
		DeferUntil:  time.Now().Add(60 * time.Second),
		BlockerTags: []string{"runtime.active:ryzen:keepalived"},
	})
	store.RecordDefer(ctx, "c1", "rel/scylla/dell", &engine.DeferState{
		StepID:      "verify_runtime",
		DeferUntil:  time.Now().Add(60 * time.Second),
		BlockerTags: []string{"runtime.active:dell:scylla"},
	})

	rows, err := store.FindByBlockerTag(ctx, "c1", "runtime.active:nuc:keepalived")
	if err != nil {
		t.Fatalf("FindByBlockerTag: %v", err)
	}
	if len(rows) != 1 || rows[0].CorrelationID != "rel/keepalived/nuc" {
		t.Errorf("FindByBlockerTag = %v; want [rel/keepalived/nuc]", rows)
	}

	// A wake tag matching nothing is fine — wakes are noisy by design.
	rows, _ = store.FindByBlockerTag(ctx, "c1", "runtime.active:does-not-exist")
	if len(rows) != 0 {
		t.Errorf("non-matching tag returned %d rows; want 0", len(rows))
	}

	// Cluster scoping: a tag that exists in c1 must not leak into c2.
	rows, _ = store.FindByBlockerTag(ctx, "c2", "runtime.active:nuc:keepalived")
	if len(rows) != 0 {
		t.Errorf("cluster scoping leaked: %v", rows)
	}
}

// TestWakePreservesDeferCount: the wake API explicitly does NOT
// decrement defer_count. Operator stories: a transient blip wakes the
// run, it succeeds, ClearOnSuccess resets the counter (existing path).
// If the blip wakes the run but the underlying condition is still
// bad, the next defer keeps counting toward abandonment — the
// max_defers budget is the same whether we waited the full cooldown
// or a wake fired early.
func TestWakePreservesDeferCount(t *testing.T) {
	ctx := context.Background()
	store := newMemoryDeferStateStore()
	state, _ := store.RecordDefer(ctx, "c1", "rel/x", &engine.DeferState{
		StepID:      "verify_runtime",
		DeferUntil:  time.Now().Add(60 * time.Second),
		BlockerTags: []string{"runtime.active:x"},
	})
	if state.DeferCount != 1 {
		t.Fatalf("setup: DeferCount = %d, want 1", state.DeferCount)
	}

	// Simulate the wake path: FindByBlockerTag returns the row, the
	// caller (WakeDeferredRunsByBlockerTag handler) clears
	// backoff_until_ms on workflow_runs. The B3 row stays put.
	matches, _ := store.FindByBlockerTag(ctx, "c1", "runtime.active:x")
	if len(matches) != 1 {
		t.Fatalf("setup: expected 1 match, got %d", len(matches))
	}
	post, _ := store.Get(ctx, "c1", "rel/x")
	if post.DeferCount != 1 {
		t.Errorf("DeferCount changed after FindByBlockerTag: %d (want 1)", post.DeferCount)
	}

	// And a second defer (e.g. wake fired but condition still bad)
	// continues counting.
	state2, _ := store.RecordDefer(ctx, "c1", "rel/x", &engine.DeferState{
		StepID:      "verify_runtime",
		DeferUntil:  time.Now().Add(60 * time.Second),
		BlockerTags: []string{"runtime.active:x"},
	})
	if state2.DeferCount != 2 {
		t.Errorf("DeferCount after second defer = %d; want 2 (budget preserved through wake)", state2.DeferCount)
	}
}

// TestWakeIgnoresAbandoned: a tag matching an abandoned row must not
// produce a wake match. B3 abandonment is the explicit "stop trying"
// signal; B4 wake is acceleration *within* the budget, not a
// resurrection mechanism. Operator must Clear an abandoned
// correlation explicitly.
func TestWakeIgnoresAbandoned(t *testing.T) {
	ctx := context.Background()
	store := newMemoryDeferStateStore()
	// Burn the budget to force abandoned=true.
	for i := 0; i < defaultB3MaxDefers; i++ {
		store.RecordDefer(ctx, "c1", "rel/dead", &engine.DeferState{
			StepID:      "verify_runtime",
			DeferUntil:  time.Now().Add(60 * time.Second),
			BlockerTags: []string{"runtime.active:dead:thing"},
		})
	}
	state, _ := store.Get(ctx, "c1", "rel/dead")
	if !state.Abandoned {
		t.Fatalf("setup: expected abandoned=true after %d defers (got count=%d max=%d)",
			defaultB3MaxDefers, state.DeferCount, state.MaxDefers)
	}

	matches, err := store.FindByBlockerTag(ctx, "c1", "runtime.active:dead:thing")
	if err != nil {
		t.Fatalf("FindByBlockerTag: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("wake matched abandoned correlation; want 0 matches, got %d", len(matches))
	}
}
