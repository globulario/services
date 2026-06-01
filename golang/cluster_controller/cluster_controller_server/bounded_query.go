package main

// bounded_query.go — Case 08: UNBOUNDED_CRITICAL_PATH_QUERY
//
// Control-plane queries that run without a timeout risk blocking reconcile
// progress indefinitely. Every query against etcd, ScyllaDB, or external
// services must complete within a bounded window.
//
// Policy:
//   short  (5s)  — listing, single-key reads, simple lookups
//   medium (10s) — multi-key scans, installed-state list across all nodes
//   long   (20s) — expensive scans (release reconcile, infra scan)
//
// Usage: replace context.Background() with boundedCtx(short) or boundedCtx(medium).

import (
	"context"
	"time"
)

const (
	boundedShort  = 5 * time.Second
	boundedMedium = 10 * time.Second
	boundedLong   = 20 * time.Second
)

// withBounded returns a new detached context + cancel function with the given
// timeout. The caller MUST call cancel() (usually via defer). Use this instead
// of context.Background() for any control-plane query to prevent unbounded hangs.
//
//	qctx, qcancel := withBounded(boundedMedium)
//	defer qcancel()
//	result, err := store.List(qctx, ...)
func withBounded(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
