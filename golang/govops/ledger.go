package govops

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

// LedgerStore is the persistence port for the operation ledger (Slice 6). Every
// governed operation writes one OperationLedgerEntry; the ledger is the single
// queryable record that audittrail.DesiredWriteRecord and the doctor remediation
// audit are projections of (see ProjectDesiredWrite). An etcd-backed implementation
// lives in ledger_etcd.go; MemLedgerStore is the in-memory implementation used by
// tests and dry-run tooling.
type LedgerStore interface {
	// Put appends one ledger entry.
	Put(ctx context.Context, e *pb.OperationLedgerEntry) error
	// List returns all entries (the caller filters with QueryLedger).
	List(ctx context.Context) ([]*pb.OperationLedgerEntry, error)
}

// MemLedgerStore is an in-memory LedgerStore.
type MemLedgerStore struct {
	mu      sync.Mutex
	entries []*pb.OperationLedgerEntry
}

// NewMemLedgerStore returns an empty in-memory ledger.
func NewMemLedgerStore() *MemLedgerStore { return &MemLedgerStore{} }

// Put appends a copy-by-reference entry.
func (m *MemLedgerStore) Put(_ context.Context, e *pb.OperationLedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

// List returns a snapshot of all entries.
func (m *MemLedgerStore) List(_ context.Context) ([]*pb.OperationLedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*pb.OperationLedgerEntry, len(m.entries))
	copy(out, m.entries)
	return out, nil
}

// LedgerFilter selects ledger entries. Zero-value fields are unconstrained, so a
// zero LedgerFilter matches everything. Result == OPERATION_RESULT_UNSPECIFIED means
// "any result".
type LedgerFilter struct {
	OperationID    string
	Actor          string
	Owner          string // matches target_owner
	Invariant      string // matches any of awg_invariants
	Result         pb.OperationResult
	OnlyRefused    bool
	OnlyBreakGlass bool
	Since          string // RFC3339(Nano); empty = unbounded
	Until          string // RFC3339(Nano); empty = unbounded
}

// MatchLedger reports whether one entry satisfies the filter.
func MatchLedger(e *pb.OperationLedgerEntry, f LedgerFilter) bool {
	if e == nil {
		return false
	}
	if f.OperationID != "" && e.GetOperationId() != f.OperationID {
		return false
	}
	if f.Actor != "" && e.GetActor() != f.Actor {
		return false
	}
	if f.Owner != "" && e.GetTargetOwner() != f.Owner {
		return false
	}
	if f.Invariant != "" && !containsStr(e.GetAwgInvariants(), f.Invariant) {
		return false
	}
	if f.Result != pb.OperationResult_OPERATION_RESULT_UNSPECIFIED && e.GetResult() != f.Result {
		return false
	}
	if f.OnlyRefused && e.GetResult() != pb.OperationResult_REFUSED {
		return false
	}
	if f.OnlyBreakGlass && e.GetResult() != pb.OperationResult_BREAK_GLASS_COMPLETED {
		return false
	}
	if !withinTimeRange(e.GetTimestamp(), f.Since, f.Until) {
		return false
	}
	return true
}

// QueryLedger returns the entries matching the filter, newest first (entries with an
// unparseable or empty timestamp sort last, preserving input order among themselves).
func QueryLedger(entries []*pb.OperationLedgerEntry, f LedgerFilter) []*pb.OperationLedgerEntry {
	var out []*pb.OperationLedgerEntry
	for _, e := range entries {
		if MatchLedger(e, f) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, oki := parseLedgerTime(out[i].GetTimestamp())
		tj, okj := parseLedgerTime(out[j].GetTimestamp())
		if oki && okj {
			return ti.After(tj)
		}
		return oki && !okj // parseable sorts before unparseable
	})
	return out
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func parseLedgerTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// withinTimeRange reports whether ts falls in [since, until]. An unparseable bound is
// ignored (treated as unbounded); an entry with an unparseable timestamp is included
// only when no bound is set.
func withinTimeRange(ts, since, until string) bool {
	t, ok := parseLedgerTime(ts)
	if since == "" && until == "" {
		return true
	}
	if !ok {
		return false
	}
	if since != "" {
		if s, sok := parseLedgerTime(since); sok && t.Before(s) {
			return false
		}
	}
	if until != "" {
		if u, uok := parseLedgerTime(until); uok && t.After(u) {
			return false
		}
	}
	return true
}
