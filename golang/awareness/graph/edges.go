package graph

import (
	"context"
	"fmt"
)

// Edge kinds supported in V1.
const (
	EdgeDefines      = "defines"
	EdgeCalls        = "calls"
	EdgeImports      = "imports"
	EdgeReads        = "reads"
	EdgeWrites       = "writes"
	EdgeOwns         = "owns"
	EdgeDependsOn    = "depends_on"
	EdgeProduces     = "produces"
	EdgeRunsAs       = "runs_as"
	EdgeEmits        = "emits"
	EdgeSubscribes   = "subscribes"
	EdgeProtects     = "protects"
	EdgeEnforces     = "enforces"
	EdgeViolates     = "violates"
	EdgeTestedBy     = "tested_by"
	EdgeRemediatedBy = "remediated_by"
	EdgeRecords      = "records"
	EdgeRecalls      = "recalls"
	EdgeAffects      = "affects"
	EdgeBlocks       = "blocks"
	EdgeUnblocks     = "unblocks"
	EdgeRequires     = "requires"
	EdgeForbids      = "forbids"
	EdgeSafeWhen     = "safe_when"
	EdgeUnsafeWhen   = "unsafe_when"

	// Learning edge kinds (Task 3).
	EdgeObservedDuring = "observed_during"
	EdgeProposes       = "proposes"
	EdgeDerivedFrom    = "derived_from"
	EdgeSupportedBy    = "supported_by"
	EdgePromotedTo     = "promoted_to"
	EdgeSupersedes     = "supersedes"
	EdgeAliases        = "aliases"
	EdgeNeedsReview    = "needs_review"
	EdgeApprovedBy     = "approved_by"
	EdgeRejectedBy     = "rejected_by"

	// Fix ledger edge kinds (Task 4).
	EdgeFixes               = "fixes"
	EdgePartiallyFixes      = "partially_fixes"
	EdgeVerifiedBy          = "verified_by"
	EdgeStillMissing        = "still_missing"
	EdgeDuplicates          = "duplicates"
	EdgeRegressedBy         = "regressed_by"
	EdgeImplementsGuardrail = "implements_guardrail"
	EdgeRequiresTest        = "requires_test"
	EdgeTouchesFile         = "touches_file"
	EdgeTouchesSymbol       = "touches_symbol"
	EdgeCoversPattern       = "covers_pattern"

	// Protocol annotation edge kinds (Task 8).
	EdgeControls = "controls"

	// Runtime bridge edge kinds (Task 6).
	EdgeCapturedIn = "captured_in"
	EdgeReports           = "reports"
	EdgeEvidences         = "evidences"
	EdgeMatchesInvariant  = "matches_invariant"
	EdgeMatchesFailureMode = "matches_failure_mode"
	EdgeHasStateDelta     = "has_state_delta"
	EdgeCurrentStatusOf   = "current_status_of"
	EdgeRuntimeDependsOn  = "runtime_depends_on"
)

// Edge is a directed relationship between two graph nodes.
type Edge struct {
	Src        string
	Kind       string
	Dst        string
	Phase      string
	Required   bool
	Confidence float64
	Metadata   map[string]any
}

// AddEdge upserts an edge. The (src, kind, dst, phase) tuple is the primary key.
func (g *Graph) AddEdge(ctx context.Context, e Edge) error {
	meta, err := marshalMeta(e.Metadata)
	if err != nil {
		return fmt.Errorf("AddEdge %s -[%s]-> %s: %w", e.Src, e.Kind, e.Dst, err)
	}
	conf := e.Confidence
	if conf == 0 {
		conf = 1.0
	}
	req := 0
	if e.Required {
		req = 1
	}
	_, err = g.db.ExecContext(ctx, `
		INSERT INTO edges (src, kind, dst, phase, required, confidence, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(src, kind, dst, phase) DO UPDATE SET
			required      = excluded.required,
			confidence    = excluded.confidence,
			metadata_json = excluded.metadata_json
	`, e.Src, e.Kind, e.Dst, e.Phase, req, conf, meta)
	if err != nil {
		return fmt.Errorf("AddEdge %s -[%s]-> %s: %w", e.Src, e.Kind, e.Dst, err)
	}
	return nil
}
