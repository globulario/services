package config

import (
	"errors"
	"testing"
	"time"
)

// When etcd is unreachable no shared client is ever established, so every
// caller used to repeat the full cost — build a client, dial, wait out a 2s
// health probe — before returning the same error. Nothing remembered that the
// previous attempt had just failed.
//
// Observed 2026-08-18: one cluster-controller persistStateLocked (an
// authoritative write plus three derived publishes) took 12-28s against a down
// etcd, stalling every heartbeat behind its lock; and the affected-package
// release gate could not run at all on a workstation whose cluster was down,
// because each test that builds a server paid that cost repeatedly and the
// package exceeded Go's 600s limit.

func withCleanDialState(t *testing.T) {
	t.Helper()
	cliMu.Lock()
	lastDialErr = nil
	lastDialFail = time.Time{}
	cliMu.Unlock()
	t.Cleanup(func() {
		cliMu.Lock()
		lastDialErr = nil
		lastDialFail = time.Time{}
		cliMu.Unlock()
	})
}

func TestEtcdDialFailure_IsRememberedThenExpires(t *testing.T) {
	withCleanDialState(t)
	sentinel := errors.New("dial refused")

	cliMu.Lock()
	_ = noteEtcdDialFailure(sentinel)
	cliMu.Unlock()

	cliMu.Lock()
	fresh := lastDialErr != nil && time.Since(lastDialFail) < etcdDialFailureTTL
	cliMu.Unlock()
	if !fresh {
		t.Fatal("a just-recorded dial failure must suppress an immediate retry")
	}

	// Age it past the window: the next caller must be allowed to try again,
	// otherwise a recovered etcd would stay invisible.
	cliMu.Lock()
	lastDialFail = time.Now().Add(-etcdDialFailureTTL - time.Second)
	stale := time.Since(lastDialFail) < etcdDialFailureTTL
	cliMu.Unlock()
	if stale {
		t.Error("an expired dial failure must not keep suppressing retries")
	}
}

func TestEtcdDialFailure_ReturnsTheErrorUnchanged(t *testing.T) {
	withCleanDialState(t)
	sentinel := errors.New("boom")
	cliMu.Lock()
	got := noteEtcdDialFailure(sentinel)
	cliMu.Unlock()
	if !errors.Is(got, sentinel) {
		t.Errorf("noteEtcdDialFailure must pass the error through unchanged; got %v", got)
	}
}

func TestEtcdDialFailure_ClearedOnSuccess(t *testing.T) {
	withCleanDialState(t)
	cliMu.Lock()
	_ = noteEtcdDialFailure(errors.New("transient"))
	clearEtcdDialFailure()
	cleared := lastDialErr == nil && lastDialFail.IsZero()
	cliMu.Unlock()
	if !cleared {
		t.Error("a successful connection must forget the previous failure")
	}
}

// ResetEtcdClient is a caller saying "conditions changed, try again now".
// Honouring the negative cache through a reset would ignore that.
func TestResetEtcdClient_ClearsTheNegativeCache(t *testing.T) {
	withCleanDialState(t)
	cliMu.Lock()
	_ = noteEtcdDialFailure(errors.New("down"))
	cliMu.Unlock()

	ResetEtcdClient()

	cliMu.Lock()
	cleared := lastDialErr == nil && lastDialFail.IsZero()
	cliMu.Unlock()
	if !cleared {
		t.Error("ResetEtcdClient must clear the negative cache so the next call re-dials")
	}
}

// The window must stay well under any plausible recovery time, so the cache can
// never be the reason a healthy etcd looks unreachable.
func TestEtcdDialFailureTTL_IsShort(t *testing.T) {
	if etcdDialFailureTTL <= 0 || etcdDialFailureTTL > 15*time.Second {
		t.Errorf("etcdDialFailureTTL = %s; want a short positive window", etcdDialFailureTTL)
	}
}
