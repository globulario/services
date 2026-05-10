package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// TestStepExhaustionWithDeferYieldsRunDeferred locks down WF-DEFER's
// smallest observable loop: a step that always fails AND has a defer:
// block must produce Run.Status == RunDeferred (not RunFailed) and
// populate Run.Defer with cooldown/blocker tags / step id.
func TestStepExhaustionWithDeferYieldsRunDeferred(t *testing.T) {
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test-defer"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:     "always_fails",
					Actor:  v1alpha1.ActorInstaller,
					Action: "installer.fail",
					Retry: &v1alpha1.RetryPolicy{
						MaxAttempts: 3,
						Backoff:     &v1alpha1.ScalarString{Raw: "1ms"},
					},
					Defer: &v1alpha1.DeferPolicy{
						Cooldown:    &v1alpha1.ScalarString{Raw: "30s"},
						MaxDefers:   4,
						BlockerTags: []string{"runtime.active:keepalived@nuc"},
					},
				},
			},
		},
	}

	attempts := 0
	router := NewRouter()
	router.Register(v1alpha1.ActorInstaller, "installer.fail", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		attempts++
		return nil, fmt.Errorf("transient failure %d", attempts)
	})

	eng := &Engine{Router: router}
	before := time.Now()
	run, err := eng.Execute(context.Background(), def, nil)

	if err != nil {
		t.Fatalf("Execute should swallow defer (returned ok), got err: %v", err)
	}
	if run.Status != RunDeferred {
		t.Fatalf("expected RunDeferred, got %s", run.Status)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if run.Defer == nil {
		t.Fatal("expected non-nil Run.Defer")
	}
	if run.Defer.StepID != "always_fails" {
		t.Errorf("Defer.StepID = %q, want always_fails", run.Defer.StepID)
	}
	if run.Defer.DeferCount != 1 {
		t.Errorf("Defer.DeferCount = %d, want 1 (first defer)", run.Defer.DeferCount)
	}
	wantUntil := before.Add(30 * time.Second)
	if run.Defer.DeferUntil.Before(wantUntil) {
		t.Errorf("Defer.DeferUntil = %v, expected ≥ %v (now+30s cooldown)",
			run.Defer.DeferUntil, wantUntil)
	}
	if len(run.Defer.BlockerTags) != 1 || run.Defer.BlockerTags[0] != "runtime.active:keepalived@nuc" {
		t.Errorf("BlockerTags = %v, want [runtime.active:keepalived@nuc]", run.Defer.BlockerTags)
	}
	if run.Defer.Reason == "" {
		t.Error("expected non-empty Defer.Reason")
	}
}

// TestStepExhaustionWithoutDeferStillFails confirms the legacy path: a
// step without a defer: block exhausting retries still produces
// RunFailed (no behavior change for un-flagged definitions).
func TestStepExhaustionWithoutDeferStillFails(t *testing.T) {
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test-no-defer"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:     "always_fails",
					Actor:  v1alpha1.ActorInstaller,
					Action: "installer.fail",
					Retry: &v1alpha1.RetryPolicy{
						MaxAttempts: 2,
						Backoff:     &v1alpha1.ScalarString{Raw: "1ms"},
					},
				},
			},
		},
	}
	router := NewRouter()
	router.Register(v1alpha1.ActorInstaller, "installer.fail", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return nil, fmt.Errorf("nope")
	})
	eng := &Engine{Router: router}
	run, err := eng.Execute(context.Background(), def, nil)
	if err == nil {
		t.Fatal("expected error from exhausted retries without defer")
	}
	if errors.Is(err, ErrStepDeferred) {
		t.Errorf("err should NOT match ErrStepDeferred when no defer: block: %v", err)
	}
	if run.Status != RunFailed {
		t.Errorf("expected RunFailed, got %s", run.Status)
	}
	if run.Defer != nil {
		t.Errorf("expected nil Run.Defer for non-deferred run, got %+v", run.Defer)
	}
}

// TestSchedulerSkipsDeferredBeforeUntil verifies the smallest scheduler
// behavior: a deferred run is NOT eligible while now < DeferUntil.
func TestSchedulerSkipsDeferredBeforeUntil(t *testing.T) {
	now := time.Now()
	r := &Run{
		ID:     "r1",
		Status: RunDeferred,
		Defer: &DeferState{
			StepID:     "verify_runtime",
			DeferUntil: now.Add(60 * time.Second),
			DeferCount: 1,
		},
	}
	if r.IsDeferEligible(now) {
		t.Errorf("run must NOT be eligible at now (60s before DeferUntil)")
	}
	if r.IsDeferEligible(now.Add(59 * time.Second)) {
		t.Errorf("run must NOT be eligible 1s before DeferUntil")
	}
	if got := PickEligibleDeferred([]*Run{r}, now); len(got) != 0 {
		t.Errorf("PickEligibleDeferred returned %d runs, want 0 before cooldown", len(got))
	}
}

// TestSchedulerResumesAfterDeferUntil verifies the symmetric case:
// once now >= DeferUntil, the run is eligible again.
func TestSchedulerResumesAfterDeferUntil(t *testing.T) {
	now := time.Now()
	r := &Run{
		ID:     "r1",
		Status: RunDeferred,
		Defer: &DeferState{
			StepID:     "verify_runtime",
			DeferUntil: now.Add(-1 * time.Second), // cooldown elapsed
			DeferCount: 2,
		},
	}
	if !r.IsDeferEligible(now) {
		t.Errorf("run must be eligible after DeferUntil has elapsed")
	}
	if !r.IsDeferEligible(r.Defer.DeferUntil) {
		t.Errorf("run must be eligible exactly at DeferUntil (>=, not >)")
	}
	got := PickEligibleDeferred([]*Run{r}, now)
	if len(got) != 1 || got[0] != r {
		t.Errorf("PickEligibleDeferred returned %d runs, want 1 after cooldown", len(got))
	}
}

// TestForeachWithDeferYieldsRunDeferred is the regression test for the
// WF-DEFER B2 hot-deploy live miss: when a foreach sub-DAG step defers,
// the foreach aggregator was wrapping the StepDeferredError in a
// generic "N/M items failed" string, which broke errors.As at the
// run-level catch and produced RunFailed instead of RunDeferred. Lock
// it down — any deferred sub-DAG must propagate as deferred when
// no item hard-failed.
func TestForeachWithDeferYieldsRunDeferred(t *testing.T) {
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test-foreach-defer"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
			Defaults: map[string]any{"items": []any{"a", "b"}},
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:      "outer",
					Foreach: &v1alpha1.ScalarString{Raw: "$.items"},
					Steps: []v1alpha1.WorkflowStepSpec{
						{
							ID:     "inner_fails",
							Actor:  v1alpha1.ActorInstaller,
							Action: "installer.fail",
							Retry: &v1alpha1.RetryPolicy{
								MaxAttempts: 2,
								Backoff:     &v1alpha1.ScalarString{Raw: "1ms"},
							},
							Defer: &v1alpha1.DeferPolicy{
								Cooldown:    &v1alpha1.ScalarString{Raw: "30s"},
								MaxDefers:   3,
								BlockerTags: []string{"runtime.active:thing"},
							},
						},
					},
				},
			},
		},
	}
	router := NewRouter()
	router.Register(v1alpha1.ActorInstaller, "installer.fail", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return nil, fmt.Errorf("nope")
	})
	eng := &Engine{Router: router}
	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("Execute should swallow defer (returned ok), got err: %v", err)
	}
	if run.Status != RunDeferred {
		t.Fatalf("expected RunDeferred from foreach with all-deferred items, got %s", run.Status)
	}
	if run.Defer == nil {
		t.Fatal("expected non-nil Run.Defer")
	}
	if run.Defer.StepID != "inner_fails" {
		t.Errorf("Defer.StepID = %q, want inner_fails (the deferred sub-step)", run.Defer.StepID)
	}
	if len(run.Defer.BlockerTags) != 1 || run.Defer.BlockerTags[0] != "runtime.active:thing" {
		t.Errorf("BlockerTags = %v, want [runtime.active:thing]", run.Defer.BlockerTags)
	}
}

// TestSchedulerOnlyConsidersDeferredStatus verifies that PickEligibleDeferred
// ignores runs in other statuses, even if a Defer struct is left lying
// around (e.g. a run that previously deferred and then succeeded on
// retry should not be re-picked just because Defer.DeferUntil elapsed).
func TestSchedulerOnlyConsidersDeferredStatus(t *testing.T) {
	now := time.Now()
	stale := &Run{
		ID:     "r-stale",
		Status: RunSucceeded,
		Defer: &DeferState{
			StepID:     "old",
			DeferUntil: now.Add(-1 * time.Hour),
		},
	}
	live := &Run{
		ID:     "r-live",
		Status: RunDeferred,
		Defer: &DeferState{
			StepID:     "new",
			DeferUntil: now.Add(-1 * time.Second),
		},
	}
	got := PickEligibleDeferred([]*Run{stale, live}, now)
	if len(got) != 1 || got[0] != live {
		t.Errorf("PickEligibleDeferred = %v, want [live]", got)
	}
}

// TestRenderBlockerTags_Templated verifies WF-DEFER B4: blocker tag
// templates with {{ .target.node_id }} / {{ .package_name }} get
// rendered against the run's input/output context at defer time.
// Without this, persisted tags are useless for event-driven wakeup
// because they still contain "{{ ... }}" rather than concrete values.
func TestRenderBlockerTags_Templated(t *testing.T) {
	ctx := map[string]any{
		"package_name": "keepalived",
		"target": map[string]any{
			"node_id": "nuc-uuid-1",
		},
	}
	raw := []string{
		"runtime.active:{{ .target.node_id }}:{{ .package_name }}",
		"deps.installed:{{ .target.node_id }}:{{ .package_name }}",
		"static_tag_with_no_template",
	}
	got := renderBlockerTags(raw, ctx)
	want := []string{
		"runtime.active:nuc-uuid-1:keepalived",
		"deps.installed:nuc-uuid-1:keepalived",
		"static_tag_with_no_template",
	}
	if len(got) != len(want) {
		t.Fatalf("len got=%d want=%d (got=%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestRenderBlockerTags_KeepsRawOnRenderError documents the resilient
// fallback: if a template references a missing nested field, the raw
// template string is preserved (not dropped). The persisted tag is
// then operator-readable for manual diagnosis even if event matching
// is degraded for that tag.
func TestRenderBlockerTags_KeepsRawOnRenderError(t *testing.T) {
	// `target` is missing entirely — accessing .target.node_id errors.
	ctx := map[string]any{"package_name": "keepalived"}
	raw := []string{"runtime.active:{{ .target.node_id }}:{{ .package_name }}"}
	got := renderBlockerTags(raw, ctx)
	if len(got) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(got))
	}
	if got[0] != raw[0] {
		t.Errorf("expected raw template kept on render error, got %q", got[0])
	}
}
