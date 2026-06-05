package interceptors

import (
	"context"
	"errors"
	"testing"

	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestIsTransportFailure_TransportCodesInvalidate pins the contract for the
// invalidateClient gate: only transport-level codes (Unavailable,
// Unauthenticated, DeadlineExceeded) should trigger cache invalidation.
// Application-level codes (NotFound, InvalidArgument, PermissionDenied) leave
// the connection valid and MUST NOT cause the cache to be dropped.
// Enforces meta.connection_errors_must_not_be_absorbed by ensuring the
// signal that triggers re-connect is well-typed.
func TestIsTransportFailure_TransportCodesInvalidate(t *testing.T) {
	for _, c := range []codes.Code{codes.Unavailable, codes.Unauthenticated, codes.DeadlineExceeded} {
		err := status.Error(c, "x")
		if !isTransportFailure(err) {
			t.Errorf("isTransportFailure(%v) = false, want true", c)
		}
	}
}

// TestIsTransportFailure_AppCodesDoNotInvalidate ensures we don't drop a
// healthy cached client over an application-level "no" answer. The codes
// listed below all leave the underlying gRPC connection valid and reusable.
func TestIsTransportFailure_AppCodesDoNotInvalidate(t *testing.T) {
	for _, c := range []codes.Code{
		codes.NotFound, codes.InvalidArgument, codes.PermissionDenied,
		codes.AlreadyExists, codes.FailedPrecondition, codes.Aborted,
		codes.OK,
	} {
		err := status.Error(c, "x")
		if isTransportFailure(err) {
			t.Errorf("isTransportFailure(%v) = true, want false", c)
		}
	}
}

// TestIsTransportFailure_NilAndNonStatus confirms the helper handles nil
// and plain (non-grpc) errors without crashing or false-positiving — both
// must return false (no invalidation).
func TestIsTransportFailure_NilAndNonStatus(t *testing.T) {
	if isTransportFailure(nil) {
		t.Error("isTransportFailure(nil) = true, want false")
	}
	if isTransportFailure(errors.New("plain")) {
		t.Error("isTransportFailure(plain) = true, want false")
	}
	// context errors come through plain, not wrapped in status.
	if isTransportFailure(context.Canceled) {
		t.Error("isTransportFailure(context.Canceled) = true, want false")
	}
}

// TestInvalidateClient_RemovesCachedEntry pins the cache-eviction contract.
// Without this, a peer restart or cert rotation leaves the cluster reusing
// a dead handle until process restart (forbidden.cache_nil_handle_permanently).
func TestInvalidateClient_RemovesCachedEntry(t *testing.T) {
	const addr, svc = "10.0.0.99:443", "test.FakeService"
	uuid := Utility.GenerateUUID(addr + svc)
	// Seed a sentinel value in the cache (we don't have a real client here,
	// but invalidateClient only calls cache.Delete which is type-agnostic).
	cache.Store(uuid, "sentinel-value")
	if _, ok := cache.Load(uuid); !ok {
		t.Fatal("setup: cache.Store did not take effect")
	}
	invalidateClient(addr, svc)
	if _, ok := cache.Load(uuid); ok {
		t.Errorf("invalidateClient did not remove cached entry for (%s, %s)", addr, svc)
	}
}

// TestInvalidateClient_AbsentIsNoOp ensures invalidating a non-cached entry
// is safe (sync.Map.Delete is documented as no-op for missing keys, but we
// pin the assumption so a future cache-type swap doesn't regress it).
func TestInvalidateClient_AbsentIsNoOp(t *testing.T) {
	const addr, svc = "10.0.0.99:443", "test.NeverCached"
	// Just make sure it doesn't panic.
	invalidateClient(addr, svc)
}
