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
	"sync"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	"github.com/globulario/services/golang/awareness_graph_client"
)

// awarenessClientHolder caches one *Client per MCP server process. The
// holder is process-global because the MCP server is a single process and
// the awareness-graph service address is stable for the cluster's lifetime
// (re-registration in etcd would require an MCP restart to pick up — that's
// acceptable for now).
type awarenessClientHolder struct {
	mu     sync.Mutex
	client *awareness_graph_client.Client
	// failed records the last dial failure. We don't cache it permanently —
	// every call re-attempts so awareness-graph can come up after MCP starts.
	// The field exists for diagnostics, not for short-circuiting.
	lastErr error
}

func (h *awarenessClientHolder) get() (*awareness_graph_client.Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.client != nil {
		return h.client, nil
	}
	cli, err := awareness_graph_client.New("", awareness_graph_client.WithInsecure())
	if err != nil {
		h.lastErr = err
		return nil, err
	}
	h.client = cli
	h.lastErr = nil
	return cli, nil
}

var awarenessClient awarenessClientHolder

// degradedResult is the canonical shape every awareness.* tool returns when
// the awareness-graph service is unreachable. Mirrors the BRIEFING_STATUS_DEGRADED
// contract so callers can branch on a single field regardless of which tool
// they invoked.
func degradedResult(err error) map[string]interface{} {
	return map[string]interface{}{
		"status": "degraded",
		"error":  err.Error(),
	}
}

func registerAwarenessTools(s *server) {
	registerAwarenessBriefingTool(s)
	registerAwarenessImpactTool(s)
	registerAwarenessResolveTool(s)
	registerAwarenessQueryTool(s)
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
		cli, err := awarenessClient.get()
		if err != nil {
			return degradedResult(err), nil
		}
		resp, err := cli.Briefing(ctx, file, task, depth)
		if err != nil {
			return degradedResult(err), nil
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
		cli, err := awarenessClient.get()
		if err != nil {
			return degradedResult(err), nil
		}
		resp, err := cli.Impact(ctx, file)
		if err != nil {
			return degradedResult(err), nil
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
		cli, err := awarenessClient.get()
		if err != nil {
			return degradedResult(err), nil
		}
		resp, err := cli.Resolve(ctx, class, id)
		if err != nil {
			return degradedResult(err), nil
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

		cli, err := awarenessClient.get()
		if err != nil {
			return degradedResult(err), nil
		}
		resp, err := cli.Query(ctx, req)
		if err != nil {
			return degradedResult(err), nil
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
