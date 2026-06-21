package rdf

import (
	"context"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// Bundle is the full set of behavioral-memory rows to project. It mirrors the
// Scylla tables; a Reader populates it for a project/domain scope.
type Bundle struct {
	Signals            []api.Signal
	Claims             []api.Claim
	Evidence           []api.Evidence
	Authorities        []api.Authority
	Conditions         []api.Condition
	Contradictions     []api.Contradiction
	Principles         []api.Principle
	PromotionDecisions []api.PromotionDecisionRecord
	RevocationRules    []api.RevocationRule
	ActionChecks       []api.ActionCheck
	Outcomes           []api.Outcome
}

// ReadOptions scopes a projection read.
type ReadOptions struct {
	Project           string
	Domain            string // optional; empty = all domains
	Since             int64  // optional; 0 = all
	IncludeBackfilled bool   // include rows whose provenance is an ai-memory backfill
	IncludeGenerated  bool   // include compiler-generated rows
}

// Reader reads behavioral-memory rows for projection. The production
// implementation (a one-shot Scylla full-table scan) lives OUTSIDE behavioral/,
// so this package stays free of any database driver. Tests use MemoryReader.
type Reader interface {
	Read(ctx context.Context, opts ReadOptions) (*Bundle, error)
}

// MemoryReader is an in-memory Reader over a fixed Bundle (for tests and
// local-first use).
type MemoryReader struct{ B *Bundle }

func (m MemoryReader) Read(context.Context, ReadOptions) (*Bundle, error) { return m.B, nil }
