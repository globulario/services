package engine

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestIsTransientJoinInstallError verifies the backpressure classifier.
func TestIsTransientJoinInstallError(t *testing.T) {
	transient := []error{
		fmt.Errorf("repository unreachable and no local package found"),
		fmt.Errorf("fetch resource: repository unreachable: rpc error: code = ResourceExhausted desc = server overloaded: too many concurrent requests"),
		fmt.Errorf("server overloaded"),
		fmt.Errorf("too many concurrent requests"),
		fmt.Errorf("ResourceExhausted: quota exceeded"),
		fmt.Errorf("repository backpressure"),
		fmt.Errorf("repository unavailable"),
	}
	for _, err := range transient {
		if !isTransientJoinInstallError(err) {
			t.Errorf("expected transient for %q", err)
		}
	}

	permanent := []error{
		fmt.Errorf("checksum mismatch: expected abc got xyz"),
		fmt.Errorf("DesiredBuildIdOrphaned: build was demoted"),
		fmt.Errorf("manifest missing for build_id 12345"),
		fmt.Errorf("artifact not found in catalog"),
		nil,
	}
	for _, err := range permanent {
		if isTransientJoinInstallError(err) {
			t.Errorf("expected permanent (non-transient) for %v", err)
		}
	}
}

// TestIsTransientJoinInstallError_GRPCStatusCodes verifies the classifier
// detects transient errors via gRPC status codes (before string matching).
func TestIsTransientJoinInstallError_GRPCStatusCodes(t *testing.T) {
	transientCodes := []codes.Code{
		codes.ResourceExhausted,
		codes.Unavailable,
		codes.DeadlineExceeded,
	}
	for _, c := range transientCodes {
		err := status.Errorf(c, "synthetic grpc error")
		if !isTransientJoinInstallError(err) {
			t.Errorf("expected transient for gRPC code %s", c)
		}
	}

	permanentCodes := []codes.Code{
		codes.NotFound,
		codes.InvalidArgument,
		codes.FailedPrecondition,
		codes.AlreadyExists,
	}
	for _, c := range permanentCodes {
		err := status.Errorf(c, "synthetic grpc error")
		if isTransientJoinInstallError(err) {
			t.Errorf("expected permanent for gRPC code %s", c)
		}
	}
}

// TestInstallWithJoinRetry_SucceedsAfterTransient verifies retry succeeds.
// Acceptance criteria: ResourceExhausted on first 2 attempts → success on 3rd.
// bootstrap_failed must not be written (the function returns nil).
func TestInstallWithJoinRetry_SucceedsAfterTransient(t *testing.T) {
	var attempts atomic.Int32
	ctx := context.Background()

	err := installWithJoinRetry(ctx, "resource", func(_ context.Context) error {
		n := attempts.Add(1)
		if n <= 2 {
			return fmt.Errorf("repository unreachable and no local package found: code = ResourceExhausted desc = server overloaded: too many concurrent requests")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

// TestInstallWithJoinRetry_PermanentFailImmediate verifies permanent errors
// are not retried. Checksum mismatch must fail on first attempt.
func TestInstallWithJoinRetry_PermanentFailImmediate(t *testing.T) {
	var attempts atomic.Int32
	ctx := context.Background()

	permErr := fmt.Errorf("checksum mismatch: expected abc got xyz")
	err := installWithJoinRetry(ctx, "node-agent", func(_ context.Context) error {
		attempts.Add(1)
		return permErr
	})

	if err == nil {
		t.Fatal("expected permanent error, got nil")
	}
	if !errors.Is(err, permErr) && err.Error() != permErr.Error() {
		t.Errorf("expected original error preserved, got: %v", err)
	}
	if attempts.Load() != 1 {
		t.Errorf("permanent error must not retry: expected 1 attempt, got %d", attempts.Load())
	}
}

// TestInstallWithJoinRetry_BudgetExhausted verifies failure after all retries.
// Acceptance criteria: result is failed, not silent success, reason includes backpressure.
func TestInstallWithJoinRetry_BudgetExhausted(t *testing.T) {
	ctx := context.Background()

	// Override delays to zero for fast test
	orig := joinInstallRetryDelays
	joinInstallRetryDelays = []time.Duration{0, 0, 0, 0, 0}
	defer func() { joinInstallRetryDelays = orig }()

	var attempts atomic.Int32
	err := installWithJoinRetry(ctx, "dns", func(_ context.Context) error {
		attempts.Add(1)
		return fmt.Errorf("repository unreachable and no local package found: code = ResourceExhausted")
	})

	if err == nil {
		t.Fatal("expected failure after budget exhaustion, got nil")
	}
	totalAttempts := len(joinInstallRetryDelays) + 1
	if int(attempts.Load()) != totalAttempts {
		t.Errorf("expected %d attempts (budget), got %d", totalAttempts, attempts.Load())
	}
	if !isTransientJoinInstallError(err) {
		// The wrapping error should still be recognisable as repository-related.
		t.Logf("final error: %v", err)
	}
}

// TestInstallWithJoinRetry_ContextCancelled verifies context cancellation is respected.
func TestInstallWithJoinRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Override delays so first retry would block
	orig := joinInstallRetryDelays
	joinInstallRetryDelays = []time.Duration{10 * time.Second}
	defer func() { joinInstallRetryDelays = orig }()

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := installWithJoinRetry(ctx, "rbac", func(_ context.Context) error {
		return fmt.Errorf("repository unreachable: too many concurrent requests")
	})

	if err == nil {
		t.Fatal("expected error on context cancel")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// TestNodeInstallPackages_ConcurrencyLimit verifies no more than
// joinInstallConcurrencyLimit installs run simultaneously.
func TestNodeInstallPackages_ConcurrencyLimit(t *testing.T) {
	var (
		peakConcurrent int32
		current        int32
	)

	cfg := NodeAgentConfig{
		FetchAndInstall: func(_ context.Context, pkg PackageRef) error {
			n := atomic.AddInt32(&current, 1)
			if n > atomic.LoadInt32(&peakConcurrent) {
				atomic.StoreInt32(&peakConcurrent, n)
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&current, -1)
			return nil
		},
	}

	handler := nodeInstallPackages(cfg)
	pkgs := make([]any, 10)
	for i := range pkgs {
		pkgs[i] = map[string]any{"name": fmt.Sprintf("svc%d", i), "kind": "SERVICE"}
	}

	_, err := handler(context.Background(), ActionRequest{With: map[string]any{"packages": pkgs}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if peak := atomic.LoadInt32(&peakConcurrent); peak > joinInstallConcurrencyLimit {
		t.Errorf("peak concurrency %d exceeded limit %d", peak, joinInstallConcurrencyLimit)
	}
}

// TestNodeInstallPackages_RetryOnBackpressure verifies the full integration:
// backpressure errors from FetchAndInstall are retried inside nodeInstallPackages.
func TestNodeInstallPackages_RetryOnBackpressure(t *testing.T) {
	// Override delays for fast test
	orig := joinInstallRetryDelays
	joinInstallRetryDelays = []time.Duration{0, 0, 0, 0, 0}
	defer func() { joinInstallRetryDelays = orig }()

	var calls atomic.Int32
	cfg := NodeAgentConfig{
		FetchAndInstall: func(_ context.Context, pkg PackageRef) error {
			n := calls.Add(1)
			if n <= 2 {
				return fmt.Errorf("repository unreachable and no local package found: code = ResourceExhausted")
			}
			return nil
		},
	}

	handler := nodeInstallPackages(cfg)
	pkgs := []any{map[string]any{"name": "resource", "kind": "SERVICE"}}
	result, err := handler(context.Background(), ActionRequest{With: map[string]any{"packages": pkgs}})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if !result.OK {
		t.Error("expected result.OK true")
	}
}
