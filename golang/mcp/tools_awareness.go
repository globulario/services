// @awareness namespace=globular.platform
// @awareness component=platform_mcp.awareness_bridge
// @awareness file_role=mcp_tool_bridge_to_awareness_graph_grpc_service
// @awareness implements=globular.platform:intent.awareness.graph_is_compiled_context_not_authority
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness risk=low
package main

// tools_awareness.go — thin MCP forwarders to the awareness-graph gRPC service.
//
// The four tools below mirror the awareness-graph proto contract one-to-one:
// awareness.briefing / impact / resolve / query. No business logic lives here
// — args are validated, the gRPC client is called, and the typed response is
// serialised into a JSON-friendly map.
//
// Lazy connection: the gRPC client is constructed on first use and cached.
// We do not dial at MCP server startup because awareness-graph may not yet
// be deployed on this cluster, and a hard startup failure would break the
// entire MCP surface for an optional observability dependency. On dial
// failure, every tool returns a `{status:"degraded", error:"..."}` map so
// agents see a clear DEGRADED state instead of a generic RPC error.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// awarenessCallTimeout bounds every MCP→awareness-graph gRPC call.
// Matches the pattern used in tools_composed.go for other gateway calls.
// Without this, a gRPC call would inherit the MCP request context (which
// in HTTP transport mode has no per-request deadline) and could hang
// indefinitely on a slow store or partially-failed connection.
const awarenessCallTimeout = 10 * time.Second

// FailureClass is the explicit taxonomy a degraded awareness response
// carries so operators can distinguish transport failure from semantic
// emptiness. Phase 6 — see docs/awareness/mcp_transport_reliability.md.
//
// The contract:
//
//	OK                  — request succeeded; treat fields as authoritative
//	EMPTY               — request succeeded; no direct anchors apply
//	DEGRADED            — request succeeded; server returned status=degraded
//	                       (e.g. preflight high-risk-no-anchor branch)
//	UNAVAILABLE         — gRPC Unavailable / endpoint resolution failure
//	TIMEOUT             — gRPC DeadlineExceeded or local timeout
//	STORE_ERROR         — gRPC Internal / Oxigraph backend error
//	TRANSPORT_ERROR     — TLS handshake / connection reset / mid-call drop
//	INVALID_ARGUMENT    — gRPC InvalidArgument (caller bug)
//	ENDPOINT_RESOLUTION — etcd lookup for awareness-graph Address failed
//
// EMPTY MUST NEVER be returned for transport failures. Transport
// failures must always carry an explicit FailureClass != "" so the
// caller can branch precisely.
type FailureClass string

const (
	FailureNone               FailureClass = ""
	FailureUnavailable        FailureClass = "UNAVAILABLE"
	FailureTimeout            FailureClass = "TIMEOUT"
	FailureStoreError         FailureClass = "STORE_ERROR"
	FailureTransportError     FailureClass = "TRANSPORT_ERROR"
	FailureInvalidArgument    FailureClass = "INVALID_ARGUMENT"
	FailureEndpointResolution FailureClass = "ENDPOINT_RESOLUTION"
)

// classifyFailure maps a Go error to a FailureClass. Order matters:
// gRPC status codes are checked first (most precise), then string
// matches for transport-layer errors that wrap into generic errors.
func classifyFailure(err error) FailureClass {
	if err == nil {
		return FailureNone
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable:
			return FailureUnavailable
		case codes.DeadlineExceeded:
			return FailureTimeout
		case codes.InvalidArgument:
			return FailureInvalidArgument
		case codes.Internal, codes.DataLoss:
			return FailureStoreError
		}
	}
	msg := err.Error()
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "context deadline exceeded") {
		return FailureTimeout
	}
	if errors.Is(err, context.Canceled) || strings.Contains(msg, "context canceled") {
		// Cancellation pre-call is treated as transport-level — caller
		// abandoned the request before semantics ran.
		return FailureTransportError
	}
	if strings.Contains(msg, "awareness-graph not found in etcd") ||
		strings.Contains(msg, "Address missing from etcd") {
		return FailureEndpointResolution
	}
	// "connection refused" and "Unavailable" are the canonical signals
	// for a service that isn't listening — check these before falling
	// back to generic transport-error classification so a dial-time
	// refusal is not lumped in with mid-call TLS/connection-reset faults.
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "Unavailable") {
		return FailureUnavailable
	}
	if isConnError(err) {
		return FailureTransportError
	}
	return FailureTransportError
}

// realAwarenessStub is the production factory: resolve endpoint from
// etcd, get a pooled gRPC connection, wrap with the AwarenessGraph
// client. Tests swap awarenessStub to a fake.
func realAwarenessStub(ctx context.Context, s *server) (awarenesspb.AwarenessGraphClient, string, error) {
	ep, err := awarenessEndpoint()
	if err != nil {
		return nil, "", fmt.Errorf("awareness-graph: %w", err)
	}
	conn, err := s.clients.get(ctx, ep)
	if err != nil {
		return nil, ep, fmt.Errorf("awareness-graph: dial %s: %w", ep, err)
	}
	return awarenesspb.NewAwarenessGraphClient(conn), ep, nil
}

// awarenessStub is a package-level seam so tests can inject a fake
// AwarenessGraphClient without standing up a real server. Production
// callers see the real factory.
var awarenessStub = realAwarenessStub

// degradedResult is the canonical shape every awareness.* tool returns
// when the awareness-graph call fails. It carries:
//
//   - status:        always "degraded" (so a single field lets callers
//                    branch the same way regardless of which tool was
//                    invoked)
//   - failure_class: one of the FailureClass taxonomy values — never
//                    empty for a real failure; "" only when degraded is
//                    constructed by the server itself (and even then
//                    callers should see the server's failure_class)
//   - tool:          the tool name (awareness.briefing, etc.) for ops
//                    diagnostics
//   - target:        the file / id / class / task the call was for —
//                    helps an operator correlate a degraded response
//                    with a doctor finding or a session log
//   - error:         the underlying error string (already translated
//                    by clients.translateError where it helps)
//
// Every degraded result emits a single structured log line at INFO so
// operators can see "tool X for target Y failed with class Z" without
// having to grep the cluster logs for the gRPC error string.
func degradedResult(toolName, target string, err error) map[string]interface{} {
	cls := classifyFailure(err)
	log.Printf(
		"mcp: awareness degraded tool=%s target=%q class=%s err=%v",
		toolName, target, cls, err,
	)
	return map[string]interface{}{
		"status":        "degraded",
		"failure_class": string(cls),
		"tool":          toolName,
		"target":        target,
		"error":         err.Error(),
	}
}

func registerAwarenessTools(s *server) {
	registerAwarenessBriefingTool(s)
	registerAwarenessImpactTool(s)
	registerAwarenessResolveTool(s)
	registerAwarenessQueryTool(s)
	registerAwarenessPreflightTool(s)
	registerAwarenessDiagnoseTool(s)
}

// ─── awareness.briefing ──────────────────────────────────────────────────

func registerAwarenessBriefingTool(s *server) {
	s.register(toolDef{
		Name: "awareness.briefing",
		Description: "Compose a prose briefing of relevant rules, invariants, failure modes, " +
			"required tests, and forbidden fixes before editing a file or starting a task. " +
			"Exactly one of `file` or `task` must be set. Returns ~500 tokens by default; " +
			"`depth=standard` (~1500) or `depth=deep` (~4000) for more context. The response " +
			"includes `status` (ok|empty|degraded) — callers MUST check it before treating " +
			"prose as authoritative.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file": {
					Type:        "string",
					Description: "Repo-relative path, e.g. golang/foo/bar.go. Use when you know the target file.",
				},
				"task": {
					Type:        "string",
					Description: "Free-form task description. Use when no specific file is in hand.",
				},
				"depth": {
					Type:        "string",
					Description: "compact (default, ~500 tokens) | standard (~1500) | deep (~4000)",
					Enum:        []string{"compact", "standard", "deep"},
					Default:     "compact",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file, _ := args["file"].(string)
		task, _ := args["task"].(string)
		depth, _ := args["depth"].(string)
		if file == "" && task == "" {
			return nil, errors.New("awareness.briefing: either 'file' or 'task' must be set")
		}
		if file != "" && task != "" {
			return nil, errors.New("awareness.briefing: 'file' and 'task' are mutually exclusive — pass exactly one")
		}
		target := file
		if target == "" {
			target = "task:" + task
		}
		stub, ep, err := awarenessStub(ctx, s)
		if err != nil {
			return degradedResult("awareness.briefing", target, err), nil
		}
		callCtx, cancel := context.WithTimeout(ctx, awarenessCallTimeout)
		defer cancel()
		resp, err := stub.Briefing(callCtx, &awarenesspb.BriefingRequest{File: file, Task: task, Depth: depth})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(ep)
			}
			return degradedResult("awareness.briefing", target, err), nil
		}
		return briefingToMap(resp), nil
	})
}

func briefingToMap(r *awarenesspb.BriefingResponse) map[string]interface{} {
	if r == nil {
		return map[string]interface{}{"status": "empty"}
	}
	return map[string]interface{}{
		"status":          briefingStatusStr(r.GetStatus()),
		"prose":           r.GetProse(),
		"referenced_ids":  r.GetReferencedIds(),
		"generated_in_ms": r.GetGeneratedInMs(),
	}
}

func briefingStatusStr(s awarenesspb.BriefingStatus) string {
	switch s {
	case awarenesspb.BriefingStatus_BRIEFING_STATUS_OK:
		return "ok"
	case awarenesspb.BriefingStatus_BRIEFING_STATUS_EMPTY:
		return "empty"
	case awarenesspb.BriefingStatus_BRIEFING_STATUS_DEGRADED:
		return "degraded"
	default:
		return "unknown"
	}
}

// ─── awareness.impact ────────────────────────────────────────────────────

func registerAwarenessImpactTool(s *server) {
	s.register(toolDef{
		Name: "awareness.impact",
		Description: "Return the direct and inferred awareness anchors that touch a given file. " +
			"Direct nodes name the file in their protects/enforces edges; inferred nodes are " +
			"reached via package, symbol, or service walks. Use this when you know the file " +
			"path and want a structured list of related invariants, failure modes, incident " +
			"patterns, required tests, forbidden fixes, and design intents.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file": {
					Type:        "string",
					Description: "Repo-relative path, e.g. golang/cluster_controller/cluster_controller_server/server.go",
				},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file, _ := args["file"].(string)
		if file == "" {
			return nil, errors.New("awareness.impact: 'file' is required")
		}
		stub, ep, err := awarenessStub(ctx, s)
		if err != nil {
			return degradedResult("awareness.impact", file, err), nil
		}
		callCtx, cancel := context.WithTimeout(ctx, awarenessCallTimeout)
		defer cancel()
		resp, err := stub.Impact(callCtx, &awarenesspb.ImpactRequest{File: file})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(ep)
			}
			return degradedResult("awareness.impact", file, err), nil
		}
		return impactToMap(resp), nil
	})
}

func impactToMap(r *awarenesspb.ImpactResponse) map[string]interface{} {
	if r == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"direct_invariants":          nodesToMaps(r.GetDirectInvariants()),
		"direct_failure_modes":       nodesToMaps(r.GetDirectFailureModes()),
		"direct_incident_patterns":   nodesToMaps(r.GetDirectIncidentPatterns()),
		"direct_intents":             nodesToMaps(r.GetDirectIntents()),
		"inferred_invariants":        nodesToMaps(r.GetInferredInvariants()),
		"inferred_failure_modes":     nodesToMaps(r.GetInferredFailureModes()),
		"inferred_incident_patterns": nodesToMaps(r.GetInferredIncidentPatterns()),
		"inferred_intents":           nodesToMaps(r.GetInferredIntents()),
		"required_tests":             nodesToMaps(r.GetRequiredTests()),
		"forbidden_fixes":            nodesToMaps(r.GetForbiddenFixes()),
	}
}

// ─── awareness.resolve ───────────────────────────────────────────────────

func registerAwarenessResolveTool(s *server) {
	s.register(toolDef{
		Name: "awareness.resolve",
		Description: "Fetch a single awareness node by class + bare ID. Returns the node's " +
			"label, severity, status, description, code anchor, and outgoing related_ids. " +
			"Use this to expand any referenced_id that another tool returned. " +
			"Supported classes: Invariant, FailureMode, IncidentPattern, Intent, " +
			"ForbiddenFix, Test, SourceFile, Symbol.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"class": {
					Type:        "string",
					Description: "Unqualified class name from the awareness ontology.",
					Enum:        []string{"Invariant", "FailureMode", "IncidentPattern", "Intent", "ForbiddenFix", "Test", "SourceFile", "Symbol", "EtcdKey", "SystemdUnit"},
				},
				"id": {
					Type:        "string",
					Description: "Bare ID without class prefix, e.g. 'reconcile.dep_block_records_must_be_cleared_when_dep_satisfies' for an Invariant.",
				},
			},
			Required: []string{"class", "id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		class, _ := args["class"].(string)
		id, _ := args["id"].(string)
		if class == "" || id == "" {
			return nil, errors.New("awareness.resolve: both 'class' and 'id' are required")
		}
		target := class + ":" + id
		stub, ep, err := awarenessStub(ctx, s)
		if err != nil {
			return degradedResult("awareness.resolve", target, err), nil
		}
		callCtx, cancel := context.WithTimeout(ctx, awarenessCallTimeout)
		defer cancel()
		resp, err := stub.Resolve(callCtx, &awarenesspb.ResolveRequest{Class: class, Id: id})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(ep)
			}
			return degradedResult("awareness.resolve", target, err), nil
		}
		return resolveToMap(resp), nil
	})
}

func resolveToMap(r *awarenesspb.ResolveResponse) map[string]interface{} {
	out := map[string]interface{}{"found": r.GetFound()}
	if r.GetFound() && r.GetNode() != nil {
		out["node"] = nodeToMap(r.GetNode())
	}
	return out
}

// ─── awareness.query ─────────────────────────────────────────────────────

func registerAwarenessQueryTool(s *server) {
	s.register(toolDef{
		Name: "awareness.query",
		Description: "Structured browse of the awareness graph in one of four typed modes. " +
			"BY_FILE: nodes whose anchor names the given file. BY_ID: nodes matching a " +
			"class-qualified ID (e.g. invariant:foo.bar). BY_CLASS: all nodes of a class " +
			"(use sparingly; pair with `limit`). RELATED: nodes pointed at by the given ID. " +
			"Returns a flat list of QueryRows. Use sparingly — do not dump the whole graph " +
			"into context.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"mode": {
					Type:        "string",
					Description: "by_file | by_id | by_class | related",
					Enum:        []string{"by_file", "by_id", "by_class", "related"},
				},
				"file":  {Type: "string", Description: "Required for mode=by_file. Repo-relative path."},
				"id":    {Type: "string", Description: "Required for mode=by_id and mode=related. Class-qualified ID, e.g. invariant:foo.bar."},
				"class": {
					Type:        "string",
					Description: "Required for mode=by_class.",
					Enum:        []string{"invariant", "failure_mode", "incident_pattern", "intent", "symbol", "source_file"},
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum rows to return. Server enforces an upper bound.",
					Default:     50,
				},
			},
			Required: []string{"mode"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		modeStr, _ := args["mode"].(string)
		mode, ok := queryModeFromString(modeStr)
		if !ok {
			return nil, fmt.Errorf("awareness.query: unknown mode %q (expected by_file|by_id|by_class|related)", modeStr)
		}

		req := &awarenesspb.QueryRequest{Mode: mode}
		if v, ok := args["file"].(string); ok {
			req.File = v
		}
		if v, ok := args["id"].(string); ok {
			req.Id = v
		}
		if v, ok := args["class"].(string); ok && v != "" {
			cls, ok := queryClassFromString(v)
			if !ok {
				return nil, fmt.Errorf("awareness.query: unknown class %q", v)
			}
			req.Class = cls
		}
		if v, ok := args["limit"]; ok {
			switch n := v.(type) {
			case float64:
				req.Limit = int32(n)
			case int:
				req.Limit = int32(n)
			}
		}

		// Sanity-check required-args-per-mode here so the user gets a clear
		// error instead of an opaque server-side rejection.
		switch mode {
		case awarenesspb.QueryMode_QUERY_MODE_BY_FILE:
			if req.File == "" {
				return nil, errors.New("awareness.query: mode=by_file requires 'file'")
			}
		case awarenesspb.QueryMode_QUERY_MODE_BY_ID, awarenesspb.QueryMode_QUERY_MODE_RELATED:
			if req.Id == "" {
				return nil, fmt.Errorf("awareness.query: mode=%s requires 'id'", modeStr)
			}
		case awarenesspb.QueryMode_QUERY_MODE_BY_CLASS:
			if req.Class == awarenesspb.QueryClass_QUERY_CLASS_UNSPECIFIED {
				return nil, errors.New("awareness.query: mode=by_class requires 'class'")
			}
		}

		// Compose a human-readable target for ops diagnostics — covers all 4 modes.
		target := "mode=" + modeStr
		if req.File != "" {
			target += " file=" + req.File
		}
		if req.Id != "" {
			target += " id=" + req.Id
		}
		if req.Class != awarenesspb.QueryClass_QUERY_CLASS_UNSPECIFIED {
			target += " class=" + req.Class.String()
		}
		stub, ep, err := awarenessStub(ctx, s)
		if err != nil {
			return degradedResult("awareness.query", target, err), nil
		}
		callCtx, cancel := context.WithTimeout(ctx, awarenessCallTimeout)
		defer cancel()
		resp, err := stub.Query(callCtx, req)
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(ep)
			}
			return degradedResult("awareness.query", target, err), nil
		}
		return queryToMap(resp), nil
	})
}

func queryToMap(r *awarenesspb.QueryResponse) map[string]interface{} {
	if r == nil {
		return map[string]interface{}{"rows": []interface{}{}, "count": 0}
	}
	rows := make([]map[string]interface{}, 0, len(r.GetRows()))
	for _, row := range r.GetRows() {
		m := map[string]interface{}{
			"id":    row.GetId(),
			"class": row.GetClass(),
		}
		if v := row.GetLabel(); v != "" {
			m["label"] = v
		}
		if v := row.GetSeverity(); v != "" {
			m["severity"] = v
		}
		if v := row.GetStatus(); v != "" {
			m["status"] = v
		}
		if v := row.GetRelation(); v != "" {
			m["relation"] = v
		}
		if v := row.GetSourceFile(); v != "" {
			m["source_file"] = v
		}
		rows = append(rows, m)
	}
	return map[string]interface{}{
		"rows":  rows,
		"count": len(rows),
	}
}

func queryModeFromString(s string) (awarenesspb.QueryMode, bool) {
	switch s {
	case "by_file":
		return awarenesspb.QueryMode_QUERY_MODE_BY_FILE, true
	case "by_id":
		return awarenesspb.QueryMode_QUERY_MODE_BY_ID, true
	case "by_class":
		return awarenesspb.QueryMode_QUERY_MODE_BY_CLASS, true
	case "related":
		return awarenesspb.QueryMode_QUERY_MODE_RELATED, true
	}
	return 0, false
}

func queryClassFromString(s string) (awarenesspb.QueryClass, bool) {
	switch s {
	case "invariant":
		return awarenesspb.QueryClass_QUERY_CLASS_INVARIANT, true
	case "failure_mode":
		return awarenesspb.QueryClass_QUERY_CLASS_FAILURE_MODE, true
	case "incident_pattern":
		return awarenesspb.QueryClass_QUERY_CLASS_INCIDENT_PATTERN, true
	case "intent":
		return awarenesspb.QueryClass_QUERY_CLASS_INTENT, true
	case "symbol":
		return awarenesspb.QueryClass_QUERY_CLASS_SYMBOL, true
	case "source_file":
		return awarenesspb.QueryClass_QUERY_CLASS_SOURCE_FILE, true
	}
	return 0, false
}

// ─── awareness.preflight ─────────────────────────────────────────────────

func registerAwarenessPreflightTool(s *server) {
	s.register(toolDef{
		Name: "awareness.preflight",
		Description: "Pre-edit decision support: returns a single risk_class (LOW_RISK | " +
			"ARCHITECTURE_SENSITIVE | CONVERGENCE_RISK | SECURITY_RISK | DATA_LOSS_RISK | " +
			"UNKNOWN_IMPACT), a confidence tier (HIGH | MEDIUM | LOW), bounded " +
			"required_actions / files_to_read / tests_to_run / forbidden_fixes, and a " +
			"coverage summary. Combines Briefing's anchor matcher with a pure risk " +
			"classifier so an agent can branch on risk before writing code. At least one " +
			"of `task` or `files` must be set. Store unavailable returns " +
			"status=degraded with risk_class=UNKNOWN_IMPACT, never a hard error.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {
					Type:        "string",
					Description: "Free-form task description. Matched against pattern activation_triggers.",
				},
				"files": {
					Type:        "array",
					Description: "Repo-relative paths the agent intends to edit. Each is run through Impact + high-risk-directory check.",
					Items:       &propSchema{Type: "string"},
				},
				"mode": {
					Type:        "string",
					Description: "compact (default, top-3 entries) | standard (top-7 entries, ≤10 action items)",
					Enum:        []string{"compact", "standard"},
					Default:     "compact",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task, _ := args["task"].(string)
		files := stringSliceArg(args["files"])
		if task == "" && len(files) == 0 {
			return nil, errors.New("awareness.preflight: at least one of 'task' or 'files' must be set")
		}
		modeStr, _ := args["mode"].(string)
		mode := preflightModeFromString(modeStr)

		target := preflightTargetForLog(task, files)
		stub, ep, err := awarenessStub(ctx, s)
		if err != nil {
			return degradedResult("awareness.preflight", target, err), nil
		}
		callCtx, cancel := context.WithTimeout(ctx, awarenessCallTimeout)
		defer cancel()
		resp, err := stub.Preflight(callCtx, &awarenesspb.PreflightRequest{
			Task:  task,
			Files: files,
			Mode:  mode,
		})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(ep)
			}
			return degradedResult("awareness.preflight", target, err), nil
		}
		return preflightToMap(resp), nil
	})
}

func preflightToMap(r *awarenesspb.PreflightResponse) map[string]interface{} {
	if r == nil {
		return map[string]interface{}{"status": "empty"}
	}
	out := map[string]interface{}{
		"status":                  preflightStatusStr(r.GetStatus()),
		"risk_class":              riskClassStr(r.GetRiskClass()),
		"confidence":              confidenceStr(r.GetConfidence()),
		"direct_invariants":       nodesToMaps(r.GetDirectInvariants()),
		"direct_failure_modes":    nodesToMaps(r.GetDirectFailureModes()),
		"direct_intents":          nodesToMaps(r.GetDirectIntents()),
		"direct_forbidden_fixes":  nodesToMaps(r.GetDirectForbiddenFixes()),
		"direct_required_tests":   nodesToMaps(r.GetDirectRequiredTests()),
		"implementation_patterns": patternsToMaps(r.GetImplementationPatterns()),
		"required_actions":        r.GetRequiredActions(),
		"files_to_read":           r.GetFilesToRead(),
		"tests_to_run":            r.GetTestsToRun(),
		"forbidden_fixes":         r.GetForbiddenFixes(),
		"blind_spots":             r.GetBlindSpots(),
		"generated_in_ms":         r.GetGeneratedInMs(),
	}
	if cov := r.GetCoverage(); cov != nil {
		out["coverage"] = map[string]interface{}{
			"sufficient":          cov.GetSufficient(),
			"direct_anchor_count": cov.GetDirectAnchorCount(),
			"file_count":          cov.GetFileCount(),
			"indexed_file_count":  cov.GetIndexedFileCount(),
			"note":                cov.GetNote(),
		}
	}
	return out
}

func preflightStatusStr(s awarenesspb.PreflightStatus) string {
	switch s {
	case awarenesspb.PreflightStatus_PREFLIGHT_STATUS_OK:
		return "ok"
	case awarenesspb.PreflightStatus_PREFLIGHT_STATUS_EMPTY:
		return "empty"
	case awarenesspb.PreflightStatus_PREFLIGHT_STATUS_DEGRADED:
		return "degraded"
	default:
		return "unknown"
	}
}

func riskClassStr(r awarenesspb.RiskClass) string {
	switch r {
	case awarenesspb.RiskClass_LOW_RISK:
		return "LOW_RISK"
	case awarenesspb.RiskClass_ARCHITECTURE_SENSITIVE:
		return "ARCHITECTURE_SENSITIVE"
	case awarenesspb.RiskClass_CONVERGENCE_RISK:
		return "CONVERGENCE_RISK"
	case awarenesspb.RiskClass_SECURITY_RISK:
		return "SECURITY_RISK"
	case awarenesspb.RiskClass_DATA_LOSS_RISK:
		return "DATA_LOSS_RISK"
	case awarenesspb.RiskClass_UNKNOWN_IMPACT:
		return "UNKNOWN_IMPACT"
	default:
		return "UNSPECIFIED"
	}
}

func confidenceStr(c awarenesspb.Confidence) string {
	switch c {
	case awarenesspb.Confidence_CONFIDENCE_HIGH:
		return "HIGH"
	case awarenesspb.Confidence_CONFIDENCE_MEDIUM:
		return "MEDIUM"
	case awarenesspb.Confidence_CONFIDENCE_LOW:
		return "LOW"
	default:
		return "UNSPECIFIED"
	}
}

// preflightTargetForLog builds a short, single-line summary of what the
// preflight call was for, used only in degradedResult's structured log.
// Truncates file lists at 3 entries to keep the log line bounded.
func preflightTargetForLog(task string, files []string) string {
	parts := []string{}
	if task != "" {
		t := task
		if len(t) > 60 {
			t = t[:57] + "..."
		}
		parts = append(parts, "task="+t)
	}
	if len(files) > 0 {
		shown := files
		suffix := ""
		if len(files) > 3 {
			shown = files[:3]
			suffix = fmt.Sprintf("+%d", len(files)-3)
		}
		parts = append(parts, "files=["+strings.Join(shown, ",")+suffix+"]")
	}
	if len(parts) == 0 {
		return "(empty request)"
	}
	return strings.Join(parts, " ")
}

func preflightModeFromString(s string) awarenesspb.PreflightMode {
	switch s {
	case "standard":
		return awarenesspb.PreflightMode_PREFLIGHT_STANDARD
	default:
		return awarenesspb.PreflightMode_PREFLIGHT_COMPACT
	}
}

func patternsToMaps(patterns []*awarenesspb.MatchedImplementationPattern) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(patterns))
	for _, p := range patterns {
		if p == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":              p.GetId(),
			"label":           p.GetLabel(),
			"match_strength":  p.GetMatchStrength(),
			"match_reason":    p.GetMatchReason(),
			"reference_files": p.GetReferenceFiles(),
			"required_calls":  p.GetRequiredCalls(),
			"forbidden_calls": p.GetForbiddenCalls(),
		})
	}
	return out
}

func stringSliceArg(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, x := range raw {
		if s, ok := x.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ─── Shared node/anchor serialisation ────────────────────────────────────

func nodesToMaps(nodes []*awarenesspb.KnowledgeNode) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		if m := nodeToMap(n); m != nil {
			out = append(out, m)
		}
	}
	return out
}

func nodeToMap(n *awarenesspb.KnowledgeNode) map[string]interface{} {
	if n == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":    n.GetId(),
		"class": n.GetClass(),
	}
	if v := n.GetLabel(); v != "" {
		m["label"] = v
	}
	if v := n.GetSeverity(); v != "" {
		m["severity"] = v
	}
	if v := n.GetStatus(); v != "" {
		m["status"] = v
	}
	if v := n.GetDescription(); v != "" {
		m["description"] = v
	}
	if v := n.GetIri(); v != "" {
		m["iri"] = v
	}
	if rel := n.GetRelatedIds(); len(rel) > 0 {
		m["related_ids"] = rel
	}
	if a := n.GetAnchor(); a != nil {
		anchor := map[string]interface{}{}
		if v := a.GetSourceYaml(); v != "" {
			anchor["source_yaml"] = v
		}
		if v := a.GetFile(); v != "" {
			anchor["file"] = v
		}
		if v := a.GetSymbol(); v != "" {
			anchor["symbol"] = v
		}
		if v := a.GetLineStart(); v != 0 {
			anchor["line_start"] = v
		}
		if v := a.GetLineEnd(); v != 0 {
			anchor["line_end"] = v
		}
		if len(anchor) > 0 {
			m["anchor"] = anchor
		}
	}
	return m
}
