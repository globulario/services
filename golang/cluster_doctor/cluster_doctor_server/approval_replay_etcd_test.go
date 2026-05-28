package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/globulario/services/golang/security"
)

// withStubbedEtcdReplay installs an in-memory implementation of
// etcdReplayPutFn so tests exercise the etcdReplayStore semantics
// (first-use accepted, replay rejected, errors propagated) without
// needing a real etcd. Restores the production function on cleanup.
func withStubbedEtcdReplay(t *testing.T) *stubReplayBackend {
	t.Helper()
	backend := &stubReplayBackend{used: make(map[string]time.Time)}
	prev := etcdReplayPutFn
	etcdReplayPutFn = backend.put
	t.Cleanup(func() { etcdReplayPutFn = prev })
	return backend
}

type stubReplayBackend struct {
	mu       sync.Mutex
	used     map[string]time.Time
	forceErr error // when non-nil, every put returns this — simulates etcd unreachable
}

func (s *stubReplayBackend) put(_ context.Context, jti string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.forceErr != nil {
		return s.forceErr
	}
	if exp, ok := s.used[jti]; ok && exp.After(time.Now()) {
		return security.ErrTokenAlreadyUsed
	}
	s.used[jti] = time.Now().Add(ttl)
	return nil
}

// TestEtcdReplayStore_FirstUseAccepted — sanity check the happy path.
func TestEtcdReplayStore_FirstUseAccepted(t *testing.T) {
	backend := withStubbedEtcdReplay(t)
	_ = backend // no inspection needed; assertion is via no error
	store := newEtcdReplayStore()
	if err := store.MarkUsed("jti-fresh", time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("first use must be accepted, got: %v", err)
	}
}

// TestEtcdReplayStore_ReplayRejected — the contract this store exists
// to enforce: presenting the same jti twice must fail with the sentinel
// error so the handler reports a clear replay rejection.
func TestEtcdReplayStore_ReplayRejected(t *testing.T) {
	withStubbedEtcdReplay(t)
	store := newEtcdReplayStore()
	exp := time.Now().Add(5 * time.Minute)
	if err := store.MarkUsed("jti-rep", exp); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := store.MarkUsed("jti-rep", exp); !errors.Is(err, security.ErrTokenAlreadyUsed) {
		t.Fatalf("replay must be rejected with ErrTokenAlreadyUsed, got: %v", err)
	}
}

// TestEtcdReplayStore_FailsClosedWhenEtcdUnavailable — when etcd is
// unreachable we MUST refuse the token. Accepting it without a durable
// replay record would silently break the single-use contract.
func TestEtcdReplayStore_FailsClosedWhenEtcdUnavailable(t *testing.T) {
	backend := withStubbedEtcdReplay(t)
	backend.forceErr = errors.New("etcd: context deadline exceeded")
	store := newEtcdReplayStore()
	err := store.MarkUsed("jti-closed", time.Now().Add(5*time.Minute))
	if err == nil {
		t.Fatal("etcd unavailable must reject the token, got nil error")
	}
	if errors.Is(err, security.ErrTokenAlreadyUsed) {
		t.Fatalf("etcd error must not surface as ErrTokenAlreadyUsed, got: %v", err)
	}
}

// TestEtcdReplayStore_EmptyJTIRejected — defense in depth.
func TestEtcdReplayStore_EmptyJTIRejected(t *testing.T) {
	withStubbedEtcdReplay(t)
	store := newEtcdReplayStore()
	if err := store.MarkUsed("", time.Now().Add(5*time.Minute)); err == nil {
		t.Fatal("empty jti must be rejected")
	}
}
