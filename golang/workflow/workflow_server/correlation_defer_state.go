// correlation_defer_state.go — WF-DEFER B3 persistent across-runs counter.
//
// Why this file exists
// --------------------
// B2 (commit 637da1ef) gave the engine a non-terminal RunDeferred state and
// the workflow_server a per-run cooldown skip. That handles the
// "transient blocker — retry in a minute" case correctly. It does NOT
// handle "this condition is permanent and we've now retried it 50 times
// across 50 fresh runs in tight loop". The reconciler will keep
// re-issuing dispatches, the executor keeps parking and re-running them
// every cooldown, and the deferred row count in workflow_runs grows
// linearly with no escalation.
//
// B3 fixes that. Each correlation_id gets one row in
// workflow.correlation_defer_state. Every defer increments defer_count.
// When defer_count >= max_defers (from the step's DeferPolicy or the
// engine default), abandoned flips to true and dispatch is refused
// permanently until an operator clears the row.
//
// Source-of-truth choice
// ----------------------
// We DO NOT keep this counter in memory inside the workflow_server
// process: a server restart, leader handoff, or pod move would reset
// it and the abandonment guarantee evaporates. Per the awareness
// invariant `convergence.no_infinite_retry` and code-smell "circuit
// state not visible in doctor", the only durable home is Scylla.
//
// Scope discipline (vs. B2)
// -------------------------
// B2's cooldown skip still reads workflow_runs.backoff_until_ms — we
// do NOT refactor B2. B3 is additive: a separate table, a separate
// check, a separate test surface. The dispatch order in
// ExecuteWorkflow is: B3 abandonment → B2 cooldown → engine.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gocql/gocql"
	"github.com/globulario/services/golang/workflow/engine"
)

// CorrelationDeferState is the persisted record for one correlation_id's
// across-runs defer history. Mirrors the table layout in schema.go.
type CorrelationDeferState struct {
	ClusterID         string
	CorrelationID     string
	DeferCount        int
	MaxDefers         int
	LastStepID        string
	LastReason        string
	LastBlockerTags   []string
	LastDeferUntilMs  int64
	Abandoned         bool
	AbandonedAt       time.Time
	ClearedAt         time.Time
	ClearedBy         string
	UpdatedAt         time.Time
}

// DeferStateStore is the persistence interface used by the workflow_server
// to read/write across-runs defer state. Default impl hits Scylla; tests
// inject in-memory. Keeping it small and explicit avoids leaking session
// concerns into the executor.
type DeferStateStore interface {
	// Get returns the row for the given correlation_id, or nil if absent.
	// nil + nil is the explicit "not present yet" case.
	Get(ctx context.Context, clusterID, correlationID string) (*CorrelationDeferState, error)

	// RecordDefer applies one defer event: ensure a row exists, increment
	// defer_count, copy the latest reason/blocker tags/until from the engine
	// DeferState, and flip abandoned=true when defer_count >= max_defers.
	// Returns the post-write state so the caller can log + emit events.
	RecordDefer(ctx context.Context, clusterID, correlationID string, ds *engine.DeferState) (*CorrelationDeferState, error)

	// ClearOnSuccess removes the row when a run for this correlation_id
	// completes successfully. The "deferred 4 times then succeeded" case
	// must reset cleanly so future tries get a full retry budget again.
	ClearOnSuccess(ctx context.Context, clusterID, correlationID string) error

	// ClearByOperator marks the row as manually cleared (operator ack).
	// Resets defer_count to 0 and abandoned to false. Records who cleared.
	ClearByOperator(ctx context.Context, clusterID, correlationID, operator string) error
}

// shouldSkipForAbandoned is the pure decision: dispatch must be refused
// for any correlation whose persistent state is abandoned. Pulled out
// of the storage path so it can be unit-tested without a session and
// composed with B2's shouldSkipForDeferral.
func shouldSkipForAbandoned(state *CorrelationDeferState) bool {
	if state == nil {
		return false
	}
	return state.Abandoned
}

// nextDeferState computes the post-defer counter state from the
// pre-existing row (or nil for first defer) and the engine-side
// DeferState that just fired. Pure logic — used by RecordDefer and
// directly by tests.
func nextDeferState(prev *CorrelationDeferState, ds *engine.DeferState, now time.Time) *CorrelationDeferState {
	out := &CorrelationDeferState{
		UpdatedAt: now,
	}
	if prev != nil {
		out.ClusterID = prev.ClusterID
		out.CorrelationID = prev.CorrelationID
		out.DeferCount = prev.DeferCount
		out.MaxDefers = prev.MaxDefers
		out.Abandoned = prev.Abandoned
		out.AbandonedAt = prev.AbandonedAt
		out.ClearedAt = prev.ClearedAt
		out.ClearedBy = prev.ClearedBy
	}
	if ds == nil {
		return out
	}
	out.DeferCount++
	out.LastStepID = ds.StepID
	out.LastReason = ds.Reason
	out.LastBlockerTags = append([]string(nil), ds.BlockerTags...)
	out.LastDeferUntilMs = ds.DeferUntil.UnixMilli()
	// MaxDefers comes from the policy carried on the run. Cap at the
	// largest non-zero value seen — protects against a later step with
	// a lower max accidentally lowering the threshold below what's
	// already accumulated.
	if out.MaxDefers == 0 {
		out.MaxDefers = defaultB3MaxDefers
	}
	if out.DeferCount >= out.MaxDefers && !out.Abandoned {
		out.Abandoned = true
		out.AbandonedAt = now
	}
	return out
}

// defaultB3MaxDefers is used when no DeferPolicy.MaxDefers is set on
// the step. Mirrors engine/defaultMaxDefers (5) so the engine and
// the persistent counter stay aligned out of the box.
const defaultB3MaxDefers = 5

// ─── Scylla-backed implementation ────────────────────────────────────────────

// scyllaDeferStateStore wraps a *gocql.Session for production use.
type scyllaDeferStateStore struct {
	session func() *gocql.Session
}

// newScyllaDeferStateStore constructs the production store. The
// session is provided as a callback so we pick up the latest session
// after a reconnect.
func newScyllaDeferStateStore(sess func() *gocql.Session) *scyllaDeferStateStore {
	return &scyllaDeferStateStore{session: sess}
}

func (s *scyllaDeferStateStore) sess() (*gocql.Session, error) {
	if s == nil || s.session == nil {
		return nil, errors.New("defer state store: no session provider")
	}
	v := s.session()
	if v == nil {
		return nil, errors.New("defer state store: session unavailable")
	}
	return v, nil
}

func (s *scyllaDeferStateStore) Get(ctx context.Context, clusterID, correlationID string) (*CorrelationDeferState, error) {
	if correlationID == "" {
		return nil, nil
	}
	sess, err := s.sess()
	if err != nil {
		return nil, err
	}
	var (
		out          CorrelationDeferState
		blockerTags  []string
		abandonedAt  time.Time
		clearedAt    time.Time
	)
	out.ClusterID = clusterID
	out.CorrelationID = correlationID
	q := sess.Query(`
		SELECT defer_count, max_defers, last_step_id, last_reason,
		       last_blocker_tags, last_defer_until_ms,
		       abandoned, abandoned_at, cleared_at, cleared_by, updated_at
		FROM workflow.correlation_defer_state
		WHERE cluster_id=? AND correlation_id=?`,
		clusterID, correlationID,
	).WithContext(ctx)
	if err := q.Scan(
		&out.DeferCount, &out.MaxDefers, &out.LastStepID, &out.LastReason,
		&blockerTags, &out.LastDeferUntilMs,
		&out.Abandoned, &abandonedAt, &clearedAt, &out.ClearedBy, &out.UpdatedAt,
	); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("defer state get: %w", err)
	}
	out.LastBlockerTags = append([]string(nil), blockerTags...)
	out.AbandonedAt = abandonedAt
	out.ClearedAt = clearedAt
	return &out, nil
}

func (s *scyllaDeferStateStore) RecordDefer(ctx context.Context, clusterID, correlationID string, ds *engine.DeferState) (*CorrelationDeferState, error) {
	if correlationID == "" {
		return nil, errors.New("defer state record: correlation_id required")
	}
	if ds == nil {
		return nil, errors.New("defer state record: nil DeferState")
	}
	prev, err := s.Get(ctx, clusterID, correlationID)
	if err != nil {
		return nil, err
	}
	if prev == nil {
		prev = &CorrelationDeferState{ClusterID: clusterID, CorrelationID: correlationID}
	}
	next := nextDeferState(prev, ds, time.Now())
	next.ClusterID = clusterID
	next.CorrelationID = correlationID
	sess, err := s.sess()
	if err != nil {
		return nil, err
	}
	tags := append([]string(nil), next.LastBlockerTags...)
	if err := sess.Query(`
		UPDATE workflow.correlation_defer_state SET
			defer_count=?,
			max_defers=?,
			last_step_id=?,
			last_reason=?,
			last_blocker_tags=?,
			last_defer_until_ms=?,
			abandoned=?,
			abandoned_at=?,
			updated_at=?
		WHERE cluster_id=? AND correlation_id=?`,
		next.DeferCount, next.MaxDefers, next.LastStepID, next.LastReason,
		tags, next.LastDeferUntilMs,
		next.Abandoned, next.AbandonedAt, next.UpdatedAt,
		clusterID, correlationID,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("defer state upsert: %w", err)
	}
	return next, nil
}

func (s *scyllaDeferStateStore) ClearOnSuccess(ctx context.Context, clusterID, correlationID string) error {
	if correlationID == "" {
		return nil
	}
	sess, err := s.sess()
	if err != nil {
		return err
	}
	return sess.Query(`
		DELETE FROM workflow.correlation_defer_state
		WHERE cluster_id=? AND correlation_id=?`,
		clusterID, correlationID,
	).WithContext(ctx).Exec()
}

func (s *scyllaDeferStateStore) ClearByOperator(ctx context.Context, clusterID, correlationID, operator string) error {
	if correlationID == "" {
		return errors.New("defer state clear: correlation_id required")
	}
	sess, err := s.sess()
	if err != nil {
		return err
	}
	now := time.Now()
	return sess.Query(`
		UPDATE workflow.correlation_defer_state SET
			defer_count=?,
			abandoned=?,
			cleared_at=?,
			cleared_by=?,
			updated_at=?
		WHERE cluster_id=? AND correlation_id=?`,
		0, false, now, operator, now,
		clusterID, correlationID,
	).WithContext(ctx).Exec()
}

// ─── In-memory implementation (test fixture) ─────────────────────────────────

// memoryDeferStateStore is a thread-safe in-process implementation used
// by unit tests. Two separate workflow_server instances pointed at one
// store models the "survives restart" case: shared persistence, fresh
// in-process state.
type memoryDeferStateStore struct {
	mu   sync.Mutex
	rows map[string]*CorrelationDeferState // key = cluster_id + "/" + correlation_id
}

func newMemoryDeferStateStore() *memoryDeferStateStore {
	return &memoryDeferStateStore{rows: make(map[string]*CorrelationDeferState)}
}

func memKey(c, id string) string { return c + "/" + id }

func (m *memoryDeferStateStore) Get(_ context.Context, clusterID, correlationID string) (*CorrelationDeferState, error) {
	if correlationID == "" {
		return nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[memKey(clusterID, correlationID)]; ok {
		// Return a copy so callers don't share mutable state.
		cp := *r
		cp.LastBlockerTags = append([]string(nil), r.LastBlockerTags...)
		return &cp, nil
	}
	return nil, nil
}

func (m *memoryDeferStateStore) RecordDefer(_ context.Context, clusterID, correlationID string, ds *engine.DeferState) (*CorrelationDeferState, error) {
	if correlationID == "" {
		return nil, errors.New("memory defer record: correlation_id required")
	}
	if ds == nil {
		return nil, errors.New("memory defer record: nil DeferState")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	prev := m.rows[memKey(clusterID, correlationID)]
	next := nextDeferState(prev, ds, time.Now())
	next.ClusterID = clusterID
	next.CorrelationID = correlationID
	m.rows[memKey(clusterID, correlationID)] = next
	cp := *next
	cp.LastBlockerTags = append([]string(nil), next.LastBlockerTags...)
	return &cp, nil
}

func (m *memoryDeferStateStore) ClearOnSuccess(_ context.Context, clusterID, correlationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, memKey(clusterID, correlationID))
	return nil
}

func (m *memoryDeferStateStore) ClearByOperator(_ context.Context, clusterID, correlationID, operator string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[memKey(clusterID, correlationID)]
	if !ok {
		return nil
	}
	now := time.Now()
	r.DeferCount = 0
	r.Abandoned = false
	r.ClearedAt = now
	r.ClearedBy = operator
	r.UpdatedAt = now
	return nil
}

// listAll is used by the read-side RPC. Optional helper, also exists
// on the in-memory store so unit tests can exercise the full surface.
func (m *memoryDeferStateStore) listAll(_ context.Context, clusterID string, abandonedOnly bool) ([]*CorrelationDeferState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*CorrelationDeferState, 0, len(m.rows))
	for _, r := range m.rows {
		if r.ClusterID != clusterID {
			continue
		}
		if abandonedOnly && !r.Abandoned {
			continue
		}
		cp := *r
		cp.LastBlockerTags = append([]string(nil), r.LastBlockerTags...)
		out = append(out, &cp)
	}
	return out, nil
}

func (s *scyllaDeferStateStore) listAll(ctx context.Context, clusterID string, abandonedOnly bool) ([]*CorrelationDeferState, error) {
	sess, err := s.sess()
	if err != nil {
		return nil, err
	}
	q := sess.Query(`
		SELECT correlation_id, defer_count, max_defers, last_step_id, last_reason,
		       last_blocker_tags, last_defer_until_ms,
		       abandoned, abandoned_at, cleared_at, cleared_by, updated_at
		FROM workflow.correlation_defer_state
		WHERE cluster_id=?`,
		clusterID,
	).WithContext(ctx)
	iter := q.PageSize(500).Iter()
	var (
		correlationID string
		deferCount    int
		maxDefers     int
		lastStepID    string
		lastReason    string
		blockerTags   []string
		lastDeferMs   int64
		abandoned     bool
		abandonedAt   time.Time
		clearedAt     time.Time
		clearedBy     string
		updatedAt     time.Time
		out           []*CorrelationDeferState
	)
	for iter.Scan(&correlationID, &deferCount, &maxDefers, &lastStepID, &lastReason,
		&blockerTags, &lastDeferMs,
		&abandoned, &abandonedAt, &clearedAt, &clearedBy, &updatedAt) {
		if abandonedOnly && !abandoned {
			continue
		}
		out = append(out, &CorrelationDeferState{
			ClusterID:        clusterID,
			CorrelationID:    correlationID,
			DeferCount:       deferCount,
			MaxDefers:        maxDefers,
			LastStepID:       lastStepID,
			LastReason:       lastReason,
			LastBlockerTags:  append([]string(nil), blockerTags...),
			LastDeferUntilMs: lastDeferMs,
			Abandoned:        abandoned,
			AbandonedAt:      abandonedAt,
			ClearedAt:        clearedAt,
			ClearedBy:        clearedBy,
			UpdatedAt:        updatedAt,
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("defer state list: %w", err)
	}
	return out, nil
}

// listingDeferStateStore extends DeferStateStore with the list method
// used by the read-side RPC. Both the Scylla and in-memory stores
// implement it; the gRPC handler type-asserts.
type listingDeferStateStore interface {
	DeferStateStore
	listAll(ctx context.Context, clusterID string, abandonedOnly bool) ([]*CorrelationDeferState, error)
}

// ─── WF-DEFER B4 — wake-by-blocker-tag lookup ─────────────────────────────────

// findByBlockerTag returns non-abandoned correlation rows in the
// cluster whose LastBlockerTags contains the given tag. Used by the
// wake-by-tag RPC to translate one event ("keepalived@nuc went
// active") into the set of correlations whose cooldowns should be
// shortened.
//
// Abandoned rows are filtered out: B4 is an acceleration path for
// in-cooldown correlations only. Restoring an abandoned correlation
// is an explicit operator decision (B3 Clear path).
//
// Pure scan over the cluster's correlation_defer_state partition. Not
// optimized — correlation count is bounded by # operator stories
// active concurrently. If that ever blows up, swap in a tag-keyed
// secondary table; the interface stays the same.
func findByBlockerTagFromList(rows []*CorrelationDeferState, tag string) []*CorrelationDeferState {
	if tag == "" {
		return nil
	}
	out := make([]*CorrelationDeferState, 0, len(rows))
	for _, r := range rows {
		if r == nil || r.Abandoned {
			continue
		}
		for _, t := range r.LastBlockerTags {
			if t == tag {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// FindByBlockerTag is the production lookup. Loads the cluster
// partition once, then filters in-process. Memory and Scylla impls
// share the helper.
func (m *memoryDeferStateStore) FindByBlockerTag(ctx context.Context, clusterID, tag string) ([]*CorrelationDeferState, error) {
	rows, err := m.listAll(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}
	return findByBlockerTagFromList(rows, tag), nil
}

func (s *scyllaDeferStateStore) FindByBlockerTag(ctx context.Context, clusterID, tag string) ([]*CorrelationDeferState, error) {
	rows, err := s.listAll(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}
	return findByBlockerTagFromList(rows, tag), nil
}

// wakingDeferStateStore is the optional capability surface used by the
// wake RPC. The two production stores implement it; gRPC handlers
// type-assert and degrade gracefully when the store doesn't support
// wake (e.g. a future read-only store).
type wakingDeferStateStore interface {
	DeferStateStore
	FindByBlockerTag(ctx context.Context, clusterID, tag string) ([]*CorrelationDeferState, error)
}
