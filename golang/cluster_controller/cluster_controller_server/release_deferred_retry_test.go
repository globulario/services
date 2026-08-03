package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// F2: a DEFERRED release must re-enter PENDING before pending-phase work.
//
// validPhaseTransitions[DEFERRED] permits only {PENDING, FAILED, REMOVING}.
// reconcilePending is written for a release already in PENDING and writes
// RESOLVED ("already_resolved"), WAITING or FAILED. All three deferred
// dispatchers called it directly, so recovery attempted DEFERRED → RESOLVED.
// emitPhaseTransition enforces hard, so nothing illegal was persisted — the
// patch failed closed and the release stayed DEFERRED, recomputing the same
// rejected answer every backoff forever. Fail-closed deadlock, not corruption.

// recordingHandle captures the phase patches a retry attempts, in order.
func recordingHandle(t *testing.T, phase string, patchErr error) (*releaseHandle, *[]string) {
	t.Helper()
	seq := []string{}
	h := &releaseHandle{
		Name:         "core@globular.io/echo",
		ResourceType: "ServiceRelease",
		Phase:        phase,
	}
	h.PatchStatus = func(_ context.Context, p statusPatch) error {
		if p.Phase == "" {
			return nil
		}
		if patchErr != nil {
			return patchErr
		}
		// Enforce the real transition table so a test cannot pass on a
		// transition production would reject.
		if err := advancePhase(h.Phase, p.Phase); err != nil {
			return err
		}
		seq = append(seq, h.Phase+"→"+p.Phase)
		h.Phase = p.Phase
		return nil
	}
	return h, &seq
}

// The guard itself must stay strict — the repair must not relax it.
func TestDeferredRetry_TransitionGuardRemainsStrict(t *testing.T) {
	for _, target := range []string{
		cluster_controllerpb.ReleasePhaseResolved,
		cluster_controllerpb.ReleasePhaseWaiting,
	} {
		if err := advancePhase(cluster_controllerpb.ReleasePhaseDeferred, target); err == nil {
			t.Errorf("DEFERRED → %s must remain rejected; the repair must not widen the table", target)
		}
	}
	// The one legal recovery edge must exist.
	if err := advancePhase(cluster_controllerpb.ReleasePhaseDeferred,
		cluster_controllerpb.ReleasePhasePending); err != nil {
		t.Errorf("DEFERRED → PENDING must be legal: %v", err)
	}
}

// The PENDING patch must be persisted before any pending-phase work runs.
func TestDeferredRetry_PersistsPendingBeforeReconciling(t *testing.T) {
	h, seq := recordingHandle(t, cluster_controllerpb.ReleasePhaseDeferred, nil)
	srv := &server{}
	// reconcilePending needs no dependencies for the "already_resolved" branch
	// to be unreachable here; we only assert the phase patch that precedes it.
	_ = srv
	if err := (&server{}).resumeDeferredRelease(context.Background(), h); err != nil {
		// A nil-dependency reconcilePending may panic-free return; the phase
		// patch is what this test pins.
		t.Logf("resume returned: %v", err)
	}
	if len(*seq) == 0 || !strings.HasSuffix((*seq)[0], "→"+cluster_controllerpb.ReleasePhasePending) {
		t.Fatalf("first persisted transition must be DEFERRED → PENDING; got %v", *seq)
	}
	if h.Phase == cluster_controllerpb.ReleasePhaseDeferred {
		t.Error("phase must have advanced past DEFERRED after a successful retry")
	}
}

// THE ordering test: if the PENDING patch fails, no resolution work may run and
// the release must remain DEFERRED.
func TestDeferredRetry_PendingPersistFailureBlocksResolution(t *testing.T) {
	boom := errors.New("etcd unavailable")
	h, seq := recordingHandle(t, cluster_controllerpb.ReleasePhaseDeferred, boom)
	err := (&server{}).resumeDeferredRelease(context.Background(), h)
	if err == nil {
		t.Fatal("a failed PENDING patch must surface an error, not be swallowed")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error must wrap the underlying cause; got %v", err)
	}
	if h.Phase != cluster_controllerpb.ReleasePhaseDeferred {
		t.Errorf("phase must remain DEFERRED when PENDING could not be persisted; got %s", h.Phase)
	}
	if len(*seq) != 0 {
		t.Errorf("no transition may be recorded when the patch failed; got %v", *seq)
	}
}

// Stale handle / concurrent retry: another reconciler already advanced it.
func TestDeferredRetry_StaleHandleIsNoOp(t *testing.T) {
	for _, phase := range []string{
		cluster_controllerpb.ReleasePhasePending,
		cluster_controllerpb.ReleasePhaseResolved,
		cluster_controllerpb.ReleasePhaseAvailable,
	} {
		h, seq := recordingHandle(t, phase, nil)
		if err := (&server{}).resumeDeferredRelease(context.Background(), h); err != nil {
			t.Errorf("phase %s: stale handle must be a no-op, not an error: %v", phase, err)
		}
		if len(*seq) != 0 {
			t.Errorf("phase %s: must not re-persist PENDING for an advanced release; got %v", phase, *seq)
		}
		if h.Phase != phase {
			t.Errorf("phase %s: must not be reset; got %s", phase, h.Phase)
		}
	}
}

// Idempotency: a second retry after advancement must not repeat the transition.
func TestDeferredRetry_RepeatedRetryIsIdempotent(t *testing.T) {
	h, seq := recordingHandle(t, cluster_controllerpb.ReleasePhaseDeferred, nil)
	srv := &server{}
	_ = srv.resumeDeferredRelease(context.Background(), h)
	first := len(*seq)
	_ = srv.resumeDeferredRelease(context.Background(), h)
	if len(*seq) != first {
		t.Errorf("second retry must not persist another transition; got %v", *seq)
	}
}

// A nil handle must not panic in any dispatcher.
func TestDeferredRetry_NilHandleSafe(t *testing.T) {
	if err := (&server{}).resumeDeferredRelease(context.Background(), nil); err != nil {
		t.Errorf("nil handle must be a safe no-op; got %v", err)
	}
}

// Structural ratchet: EVERY deferred dispatcher must route through the helper.
//
// The original review named two dispatchers; there are three — the third is in
// reconcileInfraRelease, so InfrastructureRelease had the identical deadlock.
// This scans source rather than pinning line numbers, so a fourth dispatcher
// added later fails here instead of silently reintroducing the bug.
func TestDeferredRetry_AllDispatchersUseHelper(t *testing.T) {
	src, err := os.ReadFile("release_reconciler.go")
	if err != nil {
		t.Fatalf("read release_reconciler.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")
	cases, routed := 0, 0
	for i, l := range lines {
		if !strings.Contains(l, "case cluster_controllerpb.ReleasePhaseDeferred:") {
			continue
		}
		cases++
		// Scan the case body up to the next `case ` at the same level.
		body := []string{}
		for j := i + 1; j < len(lines) && j < i+20; j++ {
			if strings.HasPrefix(strings.TrimSpace(lines[j]), "case ") {
				break
			}
			body = append(body, lines[j])
		}
		joined := strings.Join(body, "\n")
		if strings.Contains(joined, "srv.resumeDeferredRelease(") {
			routed++
		}
		if strings.Contains(joined, "srv.reconcilePending(ctx, h)") {
			t.Errorf("deferred dispatcher at line %d calls reconcilePending directly — "+
				"DEFERRED → RESOLVED/WAITING is rejected by validPhaseTransitions; "+
				"route through resumeDeferredRelease", i+1)
		}
	}
	if cases == 0 {
		t.Fatal("no deferred dispatcher found — test is stale")
	}
	if routed != cases {
		t.Errorf("%d of %d deferred dispatchers route through resumeDeferredRelease", routed, cases)
	}
	if cases < 3 {
		t.Errorf("expected at least 3 deferred dispatchers (service, app, infra); found %d", cases)
	}
}
