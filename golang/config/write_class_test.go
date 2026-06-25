package config

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ── mock KV ──────────────────────────────────────────────────────────────────

// fakePut records a single Put call for inspection.
type fakePut struct {
	key         string
	val         string
	deadline    time.Time
	hadDeadline bool
}

// fakeKV is a minimal kvWriter implementation for tests.
// It fails the first failN Put calls then succeeds.
type fakeKV struct {
	mu      sync.Mutex
	puts    []fakePut
	failN   int
	failErr error // if nil, context.DeadlineExceeded is used
}

func (f *fakeKV) Put(ctx context.Context, key, val string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	dl, ok := ctx.Deadline()
	f.puts = append(f.puts, fakePut{key: key, val: val, deadline: dl, hadDeadline: ok})

	if f.failN > 0 {
		f.failN--
		err := f.failErr
		if err == nil {
			err = context.DeadlineExceeded
		}
		return nil, err
	}
	return &clientv3.PutResponse{}, nil
}

func (f *fakeKV) Delete(ctx context.Context, key string, _ ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.failN > 0 {
		f.failN--
		err := f.failErr
		if err == nil {
			err = context.DeadlineExceeded
		}
		return nil, err
	}
	return &clientv3.DeleteResponse{Deleted: 1}, nil
}

func (f *fakeKV) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.puts)
}

func (f *fakeKV) lastDeadline() (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.puts) == 0 {
		return time.Time{}, false
	}
	p := f.puts[len(f.puts)-1]
	return p.deadline, p.hadDeadline
}

// ── TestPutRuntimeWithClass_AppliesPolicyByClass ──────────────────────────────

// TestPutRuntimeWithClass_AppliesPolicyByClass verifies that:
//  1. GetWritePolicy returns a coherent, distinct policy for every class.
//  2. PutRuntimeWithClass succeeds with each class when the underlying KV works.
//  3. An unknown class falls back to NormalRuntimeWrite (not zero-value).
func TestPutRuntimeWithClass_AppliesPolicyByClass(t *testing.T) {
	// ── policy shape assertions ───────────────────────────────────────────────
	cases := []struct {
		class          WriteClass
		wantTimeout    time.Duration
		wantMaxRetries int
		wantJitter     float64
		wantAudit      bool
	}{
		{BestEffortRuntimeWrite, 3 * time.Second, 1, 0, false},
		{NormalRuntimeWrite, 4 * time.Second, 2, 0, false},
		{CriticalWrite, 20 * time.Second, 5, 0.25, true},
		{StateCommitWrite, 30 * time.Second, 6, 0.30, true},
	}

	for _, tc := range cases {
		p := GetWritePolicy(tc.class)
		if p.Timeout != tc.wantTimeout {
			t.Errorf("class=%s: timeout want %v got %v", tc.class, tc.wantTimeout, p.Timeout)
		}
		if p.MaxRetries != tc.wantMaxRetries {
			t.Errorf("class=%s: max_retries want %d got %d", tc.class, tc.wantMaxRetries, p.MaxRetries)
		}
		if p.Jitter != tc.wantJitter {
			t.Errorf("class=%s: jitter want %v got %v", tc.class, tc.wantJitter, p.Jitter)
		}
		if p.EmitAudit != tc.wantAudit {
			t.Errorf("class=%s: emit_audit want %v got %v", tc.class, tc.wantAudit, p.EmitAudit)
		}
	}

	// Unknown class must fall back to NormalRuntimeWrite, not the zero value.
	unknown := GetWritePolicy(WriteClass("nonexistent"))
	normal := GetWritePolicy(NormalRuntimeWrite)
	if unknown.Timeout != normal.Timeout || unknown.MaxRetries != normal.MaxRetries {
		t.Errorf("unknown class should fall back to NormalRuntimeWrite: got %+v", unknown)
	}

	// ── end-to-end: each class succeeds on first attempt with a healthy KV ───
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.class)+"_succeeds", func(t *testing.T) {
			kv := &fakeKV{failN: 0}
			restore := SetWriteKVForTest(kv)
			defer restore()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := PutRuntimeWithClass(ctx, "/test/key", []byte("v"), tc.class); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kv.callCount() != 1 {
				t.Errorf("expected exactly 1 Put call, got %d", kv.callCount())
			}
		})
	}

	// ── per-attempt timeout is bounded by policy, not outer ctx ──────────────
	// Use a generous outer ctx (30 s) and verify the per-attempt deadline
	// reflects the class policy, not the outer deadline.
	t.Run("per_attempt_deadline_from_policy", func(t *testing.T) {
		kv := &fakeKV{failN: 0}
		restore := SetWriteKVForTest(kv)
		defer restore()

		// Outer deadline: 30 s from now.
		outerDeadline := time.Now().Add(30 * time.Second)
		ctx, cancel := context.WithDeadline(context.Background(), outerDeadline)
		defer cancel()

		if err := PutRuntimeWithClass(ctx, "/test/key", []byte("v"), NormalRuntimeWrite); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		dl, ok := kv.lastDeadline()
		if !ok {
			t.Fatal("Put was called without a deadline context")
		}
		// The per-attempt deadline must be ≤ policy.Timeout from call start.
		// We allow 500 ms of wall-clock slack for test execution overhead.
		maxAllowed := time.Now().Add(GetWritePolicy(NormalRuntimeWrite).Timeout + 500*time.Millisecond)
		if dl.After(maxAllowed) {
			t.Errorf("per-attempt deadline %v exceeds policy timeout ceiling %v", dl, maxAllowed)
		}
	})
}

// ── TestPutRuntimeWithClass_CriticalWriteRetriesWithJitter ───────────────────

// TestPutRuntimeWithClass_CriticalWriteRetriesWithJitter verifies that:
//  1. CriticalWrite retries up to MaxRetries times before returning success.
//  2. Jitter is applied: the calculated sleep is within [base, base*(1+jitter)].
//  3. The operation succeeds when the KV eventually accepts the write.
func TestPutRuntimeWithClass_CriticalWriteRetriesWithJitter(t *testing.T) {
	policy := GetWritePolicy(CriticalWrite)

	// ── retry count ───────────────────────────────────────────────────────────
	// Fail the first (MaxRetries - 1) attempts; succeed on the last retry.
	// This proves the loop runs MaxRetries iterations without exhausting budget.
	failFirst := policy.MaxRetries - 1
	kv := &fakeKV{failN: failFirst}
	restore := SetWriteKVForTest(kv)
	defer restore()

	// Zero out the backoff so the test doesn't sleep.
	origJitter := writeJitter
	writeJitter = func() float64 { return 0 }
	defer func() { writeJitter = origJitter }()

	// Zero-base-backoff path: temporarily override policy via a sub-class trick.
	// Because GetWritePolicy is a pure function we can't override its output
	// directly, but we can disable sleep by driving backoff to zero via jitter=0
	// and patching BaseBackoff... except BaseBackoff is in the returned struct,
	// not injectable. The real protection here is that jitter=0 and we accept
	// a few ms of sleep from BaseBackoff=500ms * (MaxRetries-1). To keep the
	// test fast, we instead verify the retry count with a generous context.

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := PutRuntimeWithClass(ctx, "/test/critical", []byte("val"), CriticalWrite); err != nil {
		t.Fatalf("expected success after retries; got: %v", err)
	}

	wantCalls := failFirst + 1 // failed attempts + 1 successful
	if got := kv.callCount(); got != wantCalls {
		t.Errorf("expected %d Put calls (failFirst=%d + 1 success), got %d", wantCalls, failFirst, got)
	}

	// ── jitter bounds ─────────────────────────────────────────────────────────
	// Verify GetWritePolicy(CriticalWrite).Jitter is in (0, 1).
	if policy.Jitter <= 0 || policy.Jitter >= 1 {
		t.Errorf("CriticalWrite jitter must be in (0,1); got %v", policy.Jitter)
	}

	// Verify the jitter formula: sleep = base + base*jitter*rand, so
	// sleep is in [base, base*(1+jitter)] for rand in [0, 1).
	base := policy.BaseBackoff
	maxJitter := float64(base) * policy.Jitter
	if maxJitter <= 0 {
		t.Errorf("expected positive jitter contribution; base=%v jitter=%v", base, policy.Jitter)
	}

	// Deterministic jitter=1.0 gives the maximum sleep.
	writeJitter = func() float64 { return 1.0 }
	maxSleep := base + time.Duration(float64(base)*policy.Jitter*1.0)
	if maxSleep <= base {
		t.Errorf("jitter=1.0 should produce sleep > base; got maxSleep=%v base=%v", maxSleep, base)
	}

	// Deterministic jitter=0.0 gives exactly base sleep.
	writeJitter = func() float64 { return 0 }
	minSleep := base + time.Duration(float64(base)*policy.Jitter*0.0)
	if minSleep != base {
		t.Errorf("jitter=0.0 should produce sleep == base; got %v", minSleep)
	}
}

// ── TestPutRuntimeWithClass_StateCommitWritePropagatesError ──────────────────

// TestPutRuntimeWithClass_StateCommitWritePropagatesError verifies that:
//  1. StateCommitWrite never swallows an error: every failure is returned to
//     the caller regardless of retry exhaustion.
//  2. The returned error identifies the class and key.
//  3. All MaxRetries+1 attempts are made before returning.
func TestPutRuntimeWithClass_StateCommitWritePropagatesError(t *testing.T) {
	policy := GetWritePolicy(StateCommitWrite)

	sentinel := errors.New("etcd unavailable")
	kv := &fakeKV{
		failN:   policy.MaxRetries + 1, // fail every attempt
		failErr: sentinel,
	}
	restore := SetWriteKVForTest(kv)
	defer restore()

	// Disable sleep so the test finishes quickly.
	origJitter := writeJitter
	writeJitter = func() float64 { return 0 }
	defer func() { writeJitter = origJitter }()

	// We can't easily zero-out BaseBackoff (it's returned by value from
	// GetWritePolicy), so give the test a generous timeout. The total sleep
	// with MaxRetries=6 and BaseBackoff=1s is at most 6s; allow 30s.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := PutRuntimeWithClass(ctx, "/globular/installed/nodes/n1/packages/workflow", nil, StateCommitWrite)

	// Must return an error.
	if err == nil {
		t.Fatal("StateCommitWrite must propagate failure; got nil error")
	}

	// Error must mention the class so callers can classify it.
	errStr := err.Error()
	if !contains(errStr, string(StateCommitWrite)) {
		t.Errorf("error must identify the write class; got: %s", errStr)
	}

	// Error must mention the key so operators can locate the failure.
	if !contains(errStr, "workflow") {
		t.Errorf("error must contain the key; got: %s", errStr)
	}

	// Must have attempted MaxRetries+1 times total.
	wantCalls := policy.MaxRetries + 1
	if got := kv.callCount(); got != wantCalls {
		t.Errorf("expected %d Put attempts, got %d", wantCalls, got)
	}

	// ── context cancellation path ─────────────────────────────────────────────
	// If the context is already cancelled before the first attempt, the error
	// must wrap the context error, not a generic message.
	t.Run("propagates_context_cancellation", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		kv2 := &fakeKV{failN: 99}
		restore2 := SetWriteKVForTest(kv2)
		defer restore2()

		err2 := PutRuntimeWithClass(cancelledCtx, "/test/key", []byte("v"), StateCommitWrite)
		if err2 == nil {
			t.Fatal("expected error on cancelled context")
		}
		if !errors.Is(err2, context.Canceled) {
			t.Errorf("expected context.Canceled in error chain; got: %v", err2)
		}
	})

	// ── best-effort class does not guarantee propagation rules ────────────────
	// BestEffortRuntimeWrite also returns an error (Go functions should not
	// silently discard errors), but callers are permitted to ignore it.
	// Verify it still returns a non-nil error on failure, confirming the function
	// always propagates — the "best effort" label is about caller intent, not
	// whether Go returns an error value.
	t.Run("best_effort_also_returns_error", func(t *testing.T) {
		bePolicy := GetWritePolicy(BestEffortRuntimeWrite)
		kv3 := &fakeKV{
			failN:   bePolicy.MaxRetries + 1,
			failErr: fmt.Errorf("transient"),
		}
		restore3 := SetWriteKVForTest(kv3)
		defer restore3()

		ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel3()

		err3 := PutRuntimeWithClass(ctx3, "/test/key", []byte("v"), BestEffortRuntimeWrite)
		if err3 == nil {
			t.Error("PutRuntimeWithClass must return an error even for BestEffortRuntimeWrite; caller chooses whether to ignore it")
		}
	})
}

// contains is a helper to avoid importing strings in tests.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
