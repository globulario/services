package config

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// WriteClass identifies the intended semantics of an etcd write. The class
// determines the timeout, retry count, backoff, and error-propagation rules
// applied by PutRuntimeWithClass.
type WriteClass string

const (
	// BestEffortRuntimeWrite is for transient, optional state where losing the
	// write is acceptable (e.g. ephemeral metrics, scratchpad keys). Short
	// timeout, one retry. Callers may safely ignore the returned error.
	BestEffortRuntimeWrite WriteClass = "best_effort_runtime"

	// NormalRuntimeWrite matches the historical PutRuntime() behavior exactly.
	// It is the default fallback for any WriteClass not recognised by
	// GetWritePolicy. Existing callers that migrate to PutRuntimeWithClass
	// without changing their class will observe no behaviour change.
	NormalRuntimeWrite WriteClass = "normal_runtime"

	// CriticalWrite is for keys the cluster requires for correct operation:
	// ingress spec, system config, pki/ca, objectstore config. A failure on a
	// critical write must be propagated to the caller and must trigger a
	// watchdog republication attempt. Long timeout with jittered backoff.
	CriticalWrite WriteClass = "critical"

	// StateCommitWrite is the class for authoritative installed-state records
	// and convergence result entries. The caller MUST NOT declare success before
	// this write succeeds. Longest timeout, strictest retry policy.
	StateCommitWrite WriteClass = "state_commit"
)

// WritePolicy is the resolved execution parameters for a WriteClass.
type WritePolicy struct {
	// Timeout is the per-attempt deadline passed to the etcd Put call.
	Timeout time.Duration
	// MaxRetries is the number of additional attempts after the first failure
	// (total attempts = MaxRetries + 1).
	MaxRetries int
	// BaseBackoff is the minimum sleep between retry attempts.
	BaseBackoff time.Duration
	// Jitter is a multiplier in [0, 1) applied randomly to BaseBackoff on each
	// retry to spread out concurrent writes under contention. The actual sleep
	// is BaseBackoff + BaseBackoff*Jitter*rand, so the sleep is always at least
	// BaseBackoff and at most BaseBackoff*(1+Jitter).
	Jitter float64
	// EmitAudit requests that every write failure be recorded to the audit log.
	// Currently a no-op; reserved for future observability wiring.
	EmitAudit bool
	// EmitMetrics requests write-class counters to be incremented.
	// Currently a no-op; reserved for future metrics wiring.
	EmitMetrics bool
}

// GetWritePolicy returns the resolved WritePolicy for a WriteClass.
// An unrecognised class falls back to NormalRuntimeWrite — this prevents
// silent behaviour changes when new classes are added incrementally.
func GetWritePolicy(class WriteClass) WritePolicy {
	switch class {
	case BestEffortRuntimeWrite:
		return WritePolicy{
			Timeout:     3 * time.Second,
			MaxRetries:  1,
			BaseBackoff: 0,
			Jitter:      0,
		}
	case NormalRuntimeWrite:
		// Matches historical PutRuntime() exactly: 4 s timeout, 2 retries,
		// 150 ms fixed backoff, no jitter.
		return WritePolicy{
			Timeout:     4 * time.Second,
			MaxRetries:  2,
			BaseBackoff: 150 * time.Millisecond,
			Jitter:      0,
		}
	case CriticalWrite:
		return WritePolicy{
			Timeout:     20 * time.Second,
			MaxRetries:  5,
			BaseBackoff: 500 * time.Millisecond,
			Jitter:      0.25,
			EmitAudit:   true,
			EmitMetrics: true,
		}
	case StateCommitWrite:
		return WritePolicy{
			Timeout:     30 * time.Second,
			MaxRetries:  6,
			BaseBackoff: 1 * time.Second,
			Jitter:      0.30,
			EmitAudit:   true,
			EmitMetrics: true,
		}
	default:
		return GetWritePolicy(NormalRuntimeWrite)
	}
}

// kvWriter is the minimal interface needed by PutRuntimeWithClass.
// clientv3.Client and clientv3.KV both satisfy it.
type kvWriter interface {
	Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error)
	Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error)
}

var (
	writeKVMu       sync.Mutex
	writeKVOverride kvWriter
)

// SetWriteKVForTest replaces the KV client used by PutRuntimeWithClass.
// For use in tests only. Returns a restore function — always defer it.
func SetWriteKVForTest(kv kvWriter) func() {
	writeKVMu.Lock()
	old := writeKVOverride
	writeKVOverride = kv
	writeKVMu.Unlock()
	return func() {
		writeKVMu.Lock()
		writeKVOverride = old
		writeKVMu.Unlock()
	}
}

func resolveWriteKV() (kvWriter, error) {
	writeKVMu.Lock()
	ov := writeKVOverride
	writeKVMu.Unlock()
	if ov != nil {
		return ov, nil
	}
	return etcdClient()
}

// localWriterIdentity is the component name this process writes etcd as (e.g.
// "cluster-controller", "node-agent"). When non-empty, the critical-write
// primitives reject writes to a critical key this identity is not an authorized
// writer of (ValidateCriticalKeyOwner) — runtime owner-enforcement at the lowest
// layer (RT-3). It is fail-OPEN: a process that never registers an identity is
// unguarded (so tests, tools, and not-yet-registered binaries keep working), and
// only registered owners are held to the ownership table.
var (
	localWriterIdentityMu sync.RWMutex
	localWriterIdentity   string
)

// SetLocalWriterIdentity registers the component name this process writes as.
// Call once at startup, before any critical write. Passing "" clears it
// (returns to unguarded/fail-open).
func SetLocalWriterIdentity(id string) {
	localWriterIdentityMu.Lock()
	localWriterIdentity = id
	localWriterIdentityMu.Unlock()
}

// LocalWriterIdentity returns the registered writer identity ("" if unset).
func LocalWriterIdentity() string {
	localWriterIdentityMu.RLock()
	defer localWriterIdentityMu.RUnlock()
	return localWriterIdentity
}

// guardLocalWriterOwnership enforces ValidateCriticalKeyOwner for the registered
// process identity. Fail-open when no identity is registered.
func guardLocalWriterOwnership(key string) error {
	id := LocalWriterIdentity()
	if id == "" {
		return nil
	}
	return ValidateCriticalKeyOwner(key, id)
}

// writeJitter returns a float64 in [0, 1) used for backoff randomisation.
// Overridable in tests for deterministic behaviour.
var writeJitter = rand.Float64

// PutRuntimeWithClass writes key=value to etcd using the retry and timeout
// policy of the given WriteClass.
//
// For StateCommitWrite and CriticalWrite the error is always returned to the
// caller; retries are bounded and use jittered backoff to avoid thundering herd
// under contention.
//
// For NormalRuntimeWrite and BestEffortRuntimeWrite the behaviour matches
// historical PutRuntime() semantics.
//
// The outer ctx may carry a deadline shorter than policy.Timeout; each
// per-attempt sub-context respects whichever deadline is earlier. ctx
// cancellation is also checked between retries so callers can abort promptly.
func PutRuntimeWithClass(ctx context.Context, key string, value []byte, class WriteClass) error {
	if err := guardLocalWriterOwnership(key); err != nil {
		return fmt.Errorf("PutRuntimeWithClass(%s): %w", class, err)
	}
	kv, err := resolveWriteKV()
	if err != nil {
		return fmt.Errorf("PutRuntimeWithClass(%s): etcd connect: %w", class, err)
	}

	policy := GetWritePolicy(class)
	var lastErr error

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		// Check outer context before each attempt.
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("PutRuntimeWithClass(%s): context done after %d attempt(s): %w (last: %v)",
					class, attempt, ctx.Err(), lastErr)
			}
			return fmt.Errorf("PutRuntimeWithClass(%s): context done before first attempt: %w", class, ctx.Err())
		default:
		}

		tctx, cancel := context.WithTimeout(ctx, policy.Timeout)
		_, err = kv.Put(tctx, key, string(value))
		cancel()

		if err == nil {
			return nil
		}
		lastErr = err

		if attempt < policy.MaxRetries {
			sleep := policy.BaseBackoff
			if policy.Jitter > 0 && sleep > 0 {
				sleep += time.Duration(float64(sleep) * policy.Jitter * writeJitter())
			}
			if sleep > 0 {
				select {
				case <-ctx.Done():
					return fmt.Errorf("PutRuntimeWithClass(%s): context cancelled during backoff: %w (last: %v)",
						class, ctx.Err(), lastErr)
				case <-time.After(sleep):
				}
			}
		}
	}

	return fmt.Errorf("PutRuntimeWithClass(%s): etcd put %s after %d attempt(s): %w",
		class, key, policy.MaxRetries+1, lastErr)
}

// DeleteRuntimeWithClass removes a single etcd key using the retry/timeout policy
// of the given WriteClass — the delete-side counterpart of PutRuntimeWithClass.
// It reports whether a key was actually removed (deleted count > 0). Owners use
// this to retract critical keys (e.g. a controller resetting its acc-config) through
// the same governed write seam rather than a raw clientv3 Delete.
func DeleteRuntimeWithClass(ctx context.Context, key string, class WriteClass) (bool, error) {
	if err := guardLocalWriterOwnership(key); err != nil {
		return false, fmt.Errorf("DeleteRuntimeWithClass(%s): %w", class, err)
	}
	kv, err := resolveWriteKV()
	if err != nil {
		return false, fmt.Errorf("DeleteRuntimeWithClass(%s): etcd connect: %w", class, err)
	}

	policy := GetWritePolicy(class)
	var lastErr error

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return false, fmt.Errorf("DeleteRuntimeWithClass(%s): context done after %d attempt(s): %w (last: %v)",
					class, attempt, ctx.Err(), lastErr)
			}
			return false, fmt.Errorf("DeleteRuntimeWithClass(%s): context done before first attempt: %w", class, ctx.Err())
		default:
		}

		tctx, cancel := context.WithTimeout(ctx, policy.Timeout)
		resp, err := kv.Delete(tctx, key)
		cancel()

		if err == nil {
			return resp != nil && resp.Deleted > 0, nil
		}
		lastErr = err

		if attempt < policy.MaxRetries {
			sleep := policy.BaseBackoff
			if policy.Jitter > 0 && sleep > 0 {
				sleep += time.Duration(float64(sleep) * policy.Jitter * writeJitter())
			}
			if sleep > 0 {
				select {
				case <-ctx.Done():
					return false, fmt.Errorf("DeleteRuntimeWithClass(%s): context cancelled during backoff: %w (last: %v)",
						class, ctx.Err(), lastErr)
				case <-time.After(sleep):
				}
			}
		}
	}

	return false, fmt.Errorf("DeleteRuntimeWithClass(%s): etcd delete %s after %d attempt(s): %w",
		class, key, policy.MaxRetries+1, lastErr)
}
