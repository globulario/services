package main

import (
	"context"
	"encoding/json"

	"github.com/globulario/awareness/failuregraph"
	"github.com/globulario/awareness/failurelearning"
	"github.com/globulario/awareness/incidentpattern"
	"github.com/globulario/awareness/sessionoracle"
)

func registerAwarenessFailureLearningTools(s *server, st *awarenessState) {
	// ── awareness.failure_learning.propose ──────────────────────────────────
	s.register(toolDef{
		Name: "awareness.failure_learning.propose",
		Description: "Propose a Failure Graph update from an incident, session, or closure. " +
			"Extracts reusable knowledge, deduplicates against the existing graph, and queues a " +
			"reviewable proposal. Does NOT auto-apply — call awareness.failure_learning.review then apply.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"source_type": {Type: "string", Description: "incident | session | closure"},
				"source_id":   {Type: "string", Description: "Incident ID, session ID, or closure ID"},
				"created_by":  {Type: "string", Description: "Who is proposing (e.g. claude, dave)"},
				"raw_errors":  {Type: "array", Description: "Observed raw error strings", Items: &propSchema{Type: "string"}},
				"symptoms":    {Type: "array", Description: "Observed symptoms", Items: &propSchema{Type: "string"}},
				"causes":      {Type: "array", Description: "Root causes identified", Items: &propSchema{Type: "string"}},
				"resolutions": {Type: "array", Description: "Resolutions applied", Items: &propSchema{Type: "string"}},
				"wrong_fixes": {Type: "array", Description: "Wrong fixes to avoid", Items: &propSchema{Type: "string"}},
				"tests":       {Type: "array", Description: "Required regression tests", Items: &propSchema{Type: "string"}},
				"files":       {Type: "array", Description: "Related files", Items: &propSchema{Type: "string"}},
				"components":  {Type: "array", Description: "Related components/packages", Items: &propSchema{Type: "string"}},
			},
			Required: []string{"source_type", "source_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded", "message": "awareness graph unavailable"}, nil
		}
		s := failurelearning.New(st.g)
		fg := failuregraph.New(st.g)

		req := failurelearning.ProposeRequest{
			SourceType:   strArg(args, "source_type"),
			SourceID:     strArg(args, "source_id"),
			CreatedBy:    strArg(args, "created_by"),
			RawErrors:    strSliceArg(args, "raw_errors"),
			Symptoms:     strSliceArg(args, "symptoms"),
			RootCauses:   strSliceArg(args, "causes"),
			Resolutions:  strSliceArg(args, "resolutions"),
			WrongFixes:   strSliceArg(args, "wrong_fixes"),
			Tests:        strSliceArg(args, "tests"),
			Files:        strSliceArg(args, "files"),
			Components:   strSliceArg(args, "components"),
		}

		p, err := failurelearning.ProposeUpdate(ctx, req, s, fg)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return proposalSummary(p), nil
	})

	// ── awareness.failure_learning.propose_from_incident ────────────────────
	s.register(toolDef{
		Name: "awareness.failure_learning.propose_from_incident",
		Description: "Propose a Failure Graph update by looking up an existing incident pattern record. " +
			"Reads the incident from the Incident Pattern store and extracts reusable knowledge.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"incident_id": {Type: "string", Description: "Incident ID (e.g. INC-2026-0012)"},
				"created_by":  {Type: "string", Description: "Who is proposing"},
			},
			Required: []string{"incident_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}
		ls := failurelearning.New(st.g)
		fg := failuregraph.New(st.g)
		ip := incidentpattern.NewStore(st.g)

		incidentID := strArg(args, "incident_id")
		extract, err := failurelearning.ExtractFromIncident(ctx, incidentID, ip, fg)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		if extract == nil {
			return map[string]interface{}{"error": "incident not found: " + incidentID}, nil
		}

		req := failurelearning.ProposeRequest{
			SourceType:   failurelearning.SourceIncident,
			SourceID:     incidentID,
			CreatedBy:    strArg(args, "created_by"),
			RawErrors:    extract.RawErrors,
			Symptoms:     extract.Symptoms,
			RootCauses:   extract.RootCauses,
			Resolutions:  extract.Resolutions,
			WrongFixes:   extract.WrongFixes,
			Tests:        extract.RegressionTests,
			Files:        extract.RelatedFiles,
			Components:   extract.RelatedComponents,
		}
		p, err := failurelearning.ProposeUpdate(ctx, req, ls, fg)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return proposalSummary(p), nil
	})

	// ── awareness.failure_learning.propose_from_session ──────────────────────
	s.register(toolDef{
		Name: "awareness.failure_learning.propose_from_session",
		Description: "Propose a Failure Graph update by extracting knowledge from a session oracle record. " +
			"Reads decisions, test results, and warnings from the session.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Session ID from the session oracle"},
				"created_by": {Type: "string", Description: "Who is proposing"},
			},
			Required: []string{"session_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}
		ls := failurelearning.New(st.g)
		fg := failuregraph.New(st.g)
		oracle := sessionoracle.New(st.g)

		sessionID := strArg(args, "session_id")
		extract, err := failurelearning.ExtractFromSession(ctx, sessionID, oracle, fg)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		if extract == nil {
			return map[string]interface{}{"error": "session not found: " + sessionID}, nil
		}

		req := failurelearning.ProposeRequest{
			SourceType:   failurelearning.SourceSession,
			SourceID:     sessionID,
			CreatedBy:    strArg(args, "created_by"),
			RawErrors:    extract.RawErrors,
			Symptoms:     extract.Symptoms,
			RootCauses:   extract.RootCauses,
			Resolutions:  extract.Resolutions,
			WrongFixes:   extract.WrongFixes,
			Tests:        extract.RegressionTests,
		}
		p, err := failurelearning.ProposeUpdate(ctx, req, ls, fg)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return proposalSummary(p), nil
	})

	// ── awareness.failure_learning.list_pending ──────────────────────────────
	s.register(toolDef{
		Name:        "awareness.failure_learning.list_pending",
		Description: "List pending Failure Graph learning proposals awaiting review.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"limit": {Type: "integer", Description: "Maximum results (default 20)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}
		ls := failurelearning.New(st.g)
		proposals, err := ls.ListPending(ctx)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		limit := 20
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if len(proposals) > limit {
			proposals = proposals[:limit]
		}
		summaries := make([]map[string]interface{}, len(proposals))
		for i, p := range proposals {
			summaries[i] = proposalSummary(&p)
		}
		return map[string]interface{}{"proposals": summaries, "count": len(summaries)}, nil
	})

	// ── awareness.failure_learning.show ─────────────────────────────────────
	s.register(toolDef{
		Name:        "awareness.failure_learning.show",
		Description: "Show the full detail of a Failure Graph learning proposal including patch and seed YAML.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"proposal_id": {Type: "string", Description: "Proposal ID (FLP-...)"},
			},
			Required: []string{"proposal_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}
		ls := failurelearning.New(st.g)
		p, err := ls.GetProposal(ctx, strArg(args, "proposal_id"))
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		patchJSON, _ := json.Marshal(p.Patch)
		extractJSON, _ := json.Marshal(p.Extracted)
		return map[string]interface{}{
			"proposal_id":   p.ID,
			"kind":          p.ProposalKind,
			"status":        p.Status,
			"title":         p.Title,
			"summary":       p.Summary,
			"confidence":    p.Confidence,
			"source_type":   p.SourceType,
			"source_id":     p.SourceID,
			"target":        p.TargetCategoryID,
			"extracted":     json.RawMessage(extractJSON),
			"patch":         json.RawMessage(patchJSON),
			"seed_yaml":     p.Patch.SeedYAML,
			"created_by":    p.CreatedBy,
			"created_at":    p.CreatedAt,
		}, nil
	})

	// ── awareness.failure_learning.review ───────────────────────────────────
	s.register(toolDef{
		Name: "awareness.failure_learning.review",
		Description: "Review a Failure Graph learning proposal: approve, approve_with_edits, reject, or defer. " +
			"Approval does not apply — call awareness.failure_learning.apply separately.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"proposal_id": {Type: "string", Description: "Proposal ID (FLP-...)"},
				"reviewer":    {Type: "string", Description: "Reviewer identity (e.g. dave, claude)"},
				"decision":    {Type: "string", Description: "approve | approve_with_edits | reject | defer"},
				"notes":       {Type: "string", Description: "Review notes or reason"},
			},
			Required: []string{"proposal_id", "reviewer", "decision"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}
		ls := failurelearning.New(st.g)
		p, err := failurelearning.ReviewProposal(ctx,
			strArg(args, "proposal_id"),
			strArg(args, "reviewer"),
			strArg(args, "decision"),
			strArg(args, "notes"),
			nil, ls)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return map[string]interface{}{
			"status":      p.Status,
			"proposal_id": p.ID,
			"reviewed_by": p.ReviewedBy,
		}, nil
	})

	// ── awareness.failure_learning.apply ────────────────────────────────────
	s.register(toolDef{
		Name: "awareness.failure_learning.apply",
		Description: "Apply an approved Failure Graph learning proposal: patches SQLite graph and writes YAML seed. " +
			"Proposal must be in approved status. Idempotent if already applied.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"proposal_id": {Type: "string", Description: "Proposal ID (FLP-...)"},
			},
			Required: []string{"proposal_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}
		ls := failurelearning.New(st.g)
		fg := failuregraph.New(st.g)
		result, err := failurelearning.ApplyProposal(ctx, strArg(args, "proposal_id"), ls, fg, st.docsDir)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return map[string]interface{}{
			"status":        "applied",
			"proposal_id":   result.ProposalID,
			"created_nodes": result.CreatedNodes,
			"created_edges": result.CreatedEdges,
			"seed_path":     result.SeedPath,
			"content_hash":  result.ContentHash,
		}, nil
	})

	// ── awareness.failure_learning.reject ───────────────────────────────────
	s.register(toolDef{
		Name:        "awareness.failure_learning.reject",
		Description: "Reject a Failure Graph learning proposal. Stores the reason for audit. Does not mutate the graph.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"proposal_id": {Type: "string", Description: "Proposal ID (FLP-...)"},
				"reviewer":    {Type: "string", Description: "Reviewer identity"},
				"reason":      {Type: "string", Description: "Rejection reason"},
			},
			Required: []string{"proposal_id", "reviewer"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}
		ls := failurelearning.New(st.g)
		if err := failurelearning.RejectProposal(ctx, strArg(args, "proposal_id"), strArg(args, "reviewer"), strArg(args, "reason"), ls); err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return map[string]interface{}{"status": "rejected", "proposal_id": strArg(args, "proposal_id")}, nil
	})

	// ── awareness.failure_learning.check_closure ─────────────────────────────
	s.register(toolDef{
		Name: "awareness.failure_learning.check_closure",
		Description: "Check whether a closure has a Failure Graph learning proposal. " +
			"Returns 'clean' or 'closed_with_learning_pending'. " +
			"Call this at the end of a bug fix to ensure the scar is captured.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"closure_id":     {Type: "string", Description: "Closure or incident ID"},
				"source_type":    {Type: "string", Description: "runtime_bug | workflow_bug | incident"},
				"has_root_cause": {Type: "boolean", Description: "Whether a root cause was identified"},
				"has_resolution": {Type: "boolean", Description: "Whether a resolution was applied"},
				"has_proof":      {Type: "boolean", Description: "Whether proof/test was recorded"},
				"raw_errors":     {Type: "array", Description: "Observed errors", Items: &propSchema{Type: "string"}},
				"root_causes":    {Type: "array", Description: "Root causes", Items: &propSchema{Type: "string"}},
				"resolutions":    {Type: "array", Description: "Resolutions applied", Items: &propSchema{Type: "string"}},
			},
			Required: []string{"closure_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}
		ls := failurelearning.New(st.g)
		fg := failuregraph.New(st.g)
		info := failurelearning.ClosureInfo{
			ClosureID:     strArg(args, "closure_id"),
			SourceType:    strArg(args, "source_type"),
			HasRootCause:  getBool(args, "has_root_cause", false),
			HasResolution: getBool(args, "has_resolution", false),
			HasProof:      getBool(args, "has_proof", false),
			RawErrors:     strSliceArg(args, "raw_errors"),
			RootCauses:    strSliceArg(args, "root_causes"),
			Resolutions:   strSliceArg(args, "resolutions"),
		}
		result, err := failurelearning.CheckClosure(ctx, info, ls, fg)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return map[string]interface{}{
			"closure_status":       result.Status,
			"requires_learning":    result.RequiresLearning,
			"existing_proposal_id": result.ExistingProposalID,
			"reason":               result.Reason,
		}, nil
	})
}

// proposalSummary converts a FailureLearningProposal to a concise MCP output map.
func proposalSummary(p *failurelearning.FailureLearningProposal) map[string]interface{} {
	return map[string]interface{}{
		"proposal_id":     p.ID,
		"status":          p.Status,
		"proposal_kind":   p.ProposalKind,
		"target_category": p.TargetCategoryID,
		"title":           p.Title,
		"summary":         p.Summary,
		"confidence":      p.Confidence,
		"source_type":     p.SourceType,
		"source_id":       p.SourceID,
	}
}

