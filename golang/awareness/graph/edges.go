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

	// Design pattern layer edge kinds.
	EdgeImplements  = "implements"   // source_file implements a design_pattern
	EdgeExhibits    = "exhibits"     // source_file exhibits an anti_pattern
	EdgeSmellsLike  = "smells_like"  // anti_pattern → code_smell
	EdgeMitigates   = "mitigates"    // design_pattern mitigates failure_mode / anti_pattern
	EdgePreventedBy = "prevented_by" // anti_pattern prevented_by design_pattern

	// Design / documentation edge kinds (Task 12).
	EdgeExplains      = "explains"
	EdgeDecides       = "decides"
	EdgeRationalizes  = "rationalizes"
	EdgeContradicts   = "contradicts"
	EdgeDocuments     = "documents"
	EdgeMentionedIn   = "mentioned_in"
	EdgeCausedBy      = "caused_by"
	EdgeFixedBy       = "fixed_by"
	EdgeValidatedBy   = "validated_by"
	EdgeGeneralizesTo = "generalizes_to"
	EdgeSpecializes   = "specializes"

	// Runtime bridge edge kinds (Task 6).
	EdgeCapturedIn = "captured_in"
	EdgeReports           = "reports"
	EdgeEvidences         = "evidences"
	EdgeMatchesInvariant  = "matches_invariant"
	EdgeMatchesFailureMode = "matches_failure_mode"
	EdgeHasStateDelta     = "has_state_delta"
	EdgeCurrentStatusOf   = "current_status_of"
	EdgeRuntimeDependsOn  = "runtime_depends_on"

	// Observation edge kinds.
	EdgeObserves = "observes" // source_file observes/detects an invariant (diagnostic/reporting)

	// Precise file→invariant relationship edges (Phase 3).
	EdgeConfigures = "configures" // source_file configures/defines data for invariant
	EdgeMayAffect  = "may_affect" // source_file may indirectly affect invariant

	// Service design graph edge kinds (Phase 2-8).
	EdgeHasAuthz         = "has_authz"          // rpc_method → authz_annotation
	EdgeHasStreamingMode = "has_streaming_mode"  // rpc_method → streaming_mode
	EdgeImplementedBy    = "implemented_by"      // rpc_method → symbol (Go method)
	EdgeGovernedBy       = "governed_by"         // rpc_method → invariant
	EdgeProvidesService  = "provides_service"    // package → proto_service
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

// ProvenanceEdge is an Edge that carries full provenance metadata describing
// where the edge came from and how well it is verified.
// Use AddEdgeWithProvenance to write provenance into the graph.
type ProvenanceEdge struct {
	Edge
	// SourceType is one of the SourceXxx constants in the integrity package.
	SourceType string
	// SourceFile is the YAML or source file from which this edge was extracted.
	SourceFile string
	// SourceCommit is the git SHA when the edge was last written.
	SourceCommit string
	// CreatedBy identifies the extractor or tool that created this edge.
	CreatedBy string
	// LastVerifiedAt is the Unix timestamp of the last verification.
	LastVerifiedAt int64
	// LastVerifiedBy identifies the verifier (e.g., "ci-check", "test-discovery").
	LastVerifiedBy string
	// VerificationLevel is one of the TrustXxx constants in the integrity package.
	VerificationLevel string
	// StalePolicy lists the conditions under which this edge becomes stale.
	StalePolicy []string
}

// AddEdgeWithProvenance writes an edge with full provenance metadata.
// The provenance is encoded in the edge's metadata_json under "provenance_json".
func (g *Graph) AddEdgeWithProvenance(ctx context.Context, pe ProvenanceEdge) error {
	if pe.Edge.Metadata == nil {
		pe.Edge.Metadata = make(map[string]any)
	}
	prov := map[string]any{
		"source_type":        pe.SourceType,
		"source_file":        pe.SourceFile,
		"source_commit":      pe.SourceCommit,
		"created_by":         pe.CreatedBy,
		"last_verified_at":   pe.LastVerifiedAt,
		"last_verified_by":   pe.LastVerifiedBy,
		"verification_level": pe.VerificationLevel,
		"stale_policy":       pe.StalePolicy,
	}
	provJSON, err := marshalMeta(prov)
	if err != nil {
		return fmt.Errorf("AddEdgeWithProvenance: encode provenance: %w", err)
	}
	pe.Edge.Metadata["provenance_json"] = provJSON
	return g.AddEdge(ctx, pe.Edge)
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
