package main

import (
	"context"
	"fmt"
	"time"

	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

func workflowEndpoint() string {
	return gatewayEndpoint()
}

func registerWorkflowTools(s *server) {

	// ── workflow_list_runs ─────────────────────────────────────────────
	s.register(toolDef{
		Name: "workflow_list_runs",
		Description: "List recent workflow runs. Filter by component, node, or status. " +
			"Use this to see what reconciliation activity is happening or has failed.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"cluster_id":     {Type: "string", Description: "Cluster ID (default: globular.internal)"},
				"component_name": {Type: "string", Description: "Filter by component/service name (e.g. 'dns', 'ai-memory')"},
				"node_id":        {Type: "string", Description: "Filter by node ID"},
				"active_only":    {Type: "boolean", Description: "Only show active (non-terminal) runs"},
				"failed_only":    {Type: "boolean", Description: "Only show failed runs"},
				"limit":          {Type: "number", Description: "Max results (default 10)"},
			},
			Required: []string{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, workflowEndpoint())
		if err != nil {
			return nil, fmt.Errorf("workflow_list_runs: connect: %w", err)
		}
		client := workflowpb.NewWorkflowServiceClient(conn)

		clusterID := strArg(args, "cluster_id")
		if clusterID == "" {
			clusterID = "globular.internal"
		}
		limit := int32(10)
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int32(l)
		}

		req := &workflowpb.ListRunsRequest{
			ClusterId:     clusterID,
			ComponentName: strArg(args, "component_name"),
			NodeId:        strArg(args, "node_id"),
			Limit:         limit,
		}
		if v, ok := args["active_only"].(bool); ok && v {
			req.ActiveOnly = true
		}
		if v, ok := args["failed_only"].(bool); ok && v {
			req.FailedOnly = true
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.ListRuns(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("workflow_list_runs: %w", err)
		}

		runs := make([]map[string]interface{}, 0, len(resp.GetRuns()))
		for _, r := range resp.GetRuns() {
			ctx := r.GetContext()
			runs = append(runs, map[string]interface{}{
				"id":         r.GetId(),
				"component":  ctx.GetComponentName(),
				"node":       ctx.GetNodeHostname(),
				"node_id":    ctx.GetNodeId(),
				"version":    ctx.GetComponentVersion(),
				"status":     r.GetStatus().String(),
				"failure":    r.GetFailureClass().String(),
				"summary":    r.GetSummary(),
				"error":      r.GetErrorMessage(),
				"retries":    r.GetRetryCount(),
				"started_at": r.GetStartedAt().AsTime().Format(time.RFC3339),
			})
		}

		return map[string]interface{}{
			"total": resp.GetTotal(),
			"runs":  runs,
		}, nil
	})

	// ── workflow_get_run ───────────────────────────────────────────────
	s.register(toolDef{
		Name: "workflow_get_run",
		Description: "Get full details of a workflow run including all steps and artifacts. " +
			"Use this to understand exactly what happened during a reconciliation — " +
			"which steps succeeded, which failed, and why.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"cluster_id": {Type: "string", Description: "Cluster ID (default: globular.internal)"},
				"run_id":     {Type: "string", Description: "Workflow run ID"},
			},
			Required: []string{"run_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, workflowEndpoint())
		if err != nil {
			return nil, fmt.Errorf("workflow_get_run: connect: %w", err)
		}
		client := workflowpb.NewWorkflowServiceClient(conn)

		clusterID := strArg(args, "cluster_id")
		if clusterID == "" {
			clusterID = "globular.internal"
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetRun(callCtx, &workflowpb.GetRunRequest{
			ClusterId: clusterID,
			Id:        strArg(args, "run_id"),
		})
		if err != nil {
			return nil, fmt.Errorf("workflow_get_run: %w", err)
		}

		run := resp.GetRun()
		rctx := run.GetContext()

		steps := make([]map[string]interface{}, 0, len(resp.GetSteps()))
		for _, s := range resp.GetSteps() {
			step := map[string]interface{}{
				"seq":      s.GetSeq(),
				"key":      s.GetStepKey(),
				"title":    s.GetTitle(),
				"actor":    s.GetActor().String(),
				"phase":    s.GetPhase().String(),
				"status":   s.GetStatus().String(),
				"duration": fmt.Sprintf("%dms", s.GetDurationMs()),
			}
			if s.GetErrorCode() != "" {
				step["error_code"] = s.GetErrorCode()
			}
			if s.GetErrorMessage() != "" {
				step["error"] = s.GetErrorMessage()
			}
			if s.GetActionHint() != "" {
				step["hint"] = s.GetActionHint()
			}
			steps = append(steps, step)
		}

		artifacts := make([]map[string]interface{}, 0, len(resp.GetArtifacts()))
		for _, a := range resp.GetArtifacts() {
			artifacts = append(artifacts, map[string]interface{}{
				"kind":    a.GetKind().String(),
				"name":    a.GetName(),
				"version": a.GetVersion(),
				"path":    a.GetPath(),
			})
		}

		return map[string]interface{}{
			"run": map[string]interface{}{
				"id":            run.GetId(),
				"component":     rctx.GetComponentName(),
				"node":          rctx.GetNodeHostname(),
				"version":       rctx.GetComponentVersion(),
				"plan_id":       rctx.GetPlanId(),
				"status":        run.GetStatus().String(),
				"failure_class": run.GetFailureClass().String(),
				"summary":       run.GetSummary(),
				"error":         run.GetErrorMessage(),
				"retries":       run.GetRetryCount(),
				"acknowledged":  run.GetAcknowledged(),
			},
			"steps":     steps,
			"artifacts": artifacts,
		}, nil
	})

	// ── workflow_diagnose ──────────────────────────────────────────────
	s.register(toolDef{
		Name: "workflow_diagnose",
		Description: "Diagnose a failed workflow run. Returns structured analysis with " +
			"failure classification, suggested action, confidence level, and related failures. " +
			"Use this when a reconciliation fails to understand root cause.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"cluster_id": {Type: "string", Description: "Cluster ID (default: globular.internal)"},
				"run_id":     {Type: "string", Description: "Workflow run ID to diagnose"},
			},
			Required: []string{"run_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, workflowEndpoint())
		if err != nil {
			return nil, fmt.Errorf("workflow_diagnose: connect: %w", err)
		}
		client := workflowpb.NewWorkflowServiceClient(conn)

		clusterID := strArg(args, "cluster_id")
		if clusterID == "" {
			clusterID = "globular.internal"
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.DiagnoseRun(callCtx, &workflowpb.DiagnoseRunRequest{
			ClusterId: clusterID,
			RunId:     strArg(args, "run_id"),
		})
		if err != nil {
			return nil, fmt.Errorf("workflow_diagnose: %w", err)
		}

		return map[string]interface{}{
			"diagnosis":        resp.GetDiagnosis(),
			"confidence":       resp.GetConfidence(),
			"suggested_action": resp.GetSuggestedAction(),
			"related_run_ids":  resp.GetRelatedRunIds(),
		}, nil
	})

	// ── workflow_get_service_status ────────────────────────────────────
	s.register(toolDef{
		Name: "workflow_get_service_status",
		Description: "Get the latest workflow run for a specific service. Quick way to check " +
			"if a service's last reconciliation succeeded or failed, and why.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"cluster_id": {Type: "string", Description: "Cluster ID (default: globular.internal)"},
				"service":    {Type: "string", Description: "Service name (e.g. 'dns', 'ai-memory', 'workflow')"},
				"node_id":    {Type: "string", Description: "Specific node (optional)"},
			},
			Required: []string{"service"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, workflowEndpoint())
		if err != nil {
			return nil, fmt.Errorf("workflow_get_service_status: connect: %w", err)
		}
		client := workflowpb.NewWorkflowServiceClient(conn)

		clusterID := strArg(args, "cluster_id")
		if clusterID == "" {
			clusterID = "globular.internal"
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.ListRuns(callCtx, &workflowpb.ListRunsRequest{
			ClusterId:     clusterID,
			ComponentName: strArg(args, "service"),
			NodeId:        strArg(args, "node_id"),
			Limit:         1,
		})
		if err != nil {
			return nil, fmt.Errorf("workflow_get_service_status: %w", err)
		}

		if len(resp.GetRuns()) == 0 {
			return map[string]interface{}{
				"service": strArg(args, "service"),
				"status":  "no_runs",
				"message": "No workflow runs found for this service",
			}, nil
		}

		r := resp.GetRuns()[0]
		rctx := r.GetContext()
		result := map[string]interface{}{
			"service":    rctx.GetComponentName(),
			"node":       rctx.GetNodeHostname(),
			"version":    rctx.GetComponentVersion(),
			"status":     r.GetStatus().String(),
			"run_id":     r.GetId(),
			"started_at": r.GetStartedAt().AsTime().Format(time.RFC3339),
		}
		if r.GetErrorMessage() != "" {
			result["error"] = r.GetErrorMessage()
			result["failure_class"] = r.GetFailureClass().String()
		}
		if r.GetSummary() != "" {
			result["summary"] = r.GetSummary()
		}

		return result, nil
	})
}
