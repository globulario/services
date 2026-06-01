package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
)

// ── Skill Definition ────────────────────────────────────────────────────────

// Skill is a reusable prompt template that guides an AI agent through a
// multi-step operational task. Unlike CLI workflows (which are command
// sequences), skills are tool-oriented playbooks — each step tells the
// agent which MCP tool to call and how to interpret the result.
type Skill struct {
	ID          string      `json:"id,omitempty"` // memory ID (empty for builtins)
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Tags        []string    `json:"tags,omitempty"`
	Steps       []SkillStep `json:"steps"`
	Context     string      `json:"context,omitempty"`
	OutputFormat string     `json:"output_format,omitempty"`
	Source      string      `json:"source"` // "builtin" or "custom"
}

// SkillStep is a single step in a skill playbook.
type SkillStep struct {
	Order       int               `json:"order"`
	Instruction string            `json:"instruction"`
	Tool        string            `json:"tool,omitempty"`
	Args        map[string]string `json:"args,omitempty"`
	OnError     string            `json:"on_error,omitempty"`
	Condition   string            `json:"condition,omitempty"`
}

// ── Built-in Skills ─────────────────────────────────────────────────────────

var builtinSkills = map[string]Skill{
	"diagnose": {
		Name:        "diagnose",
		Description: "Full cluster health diagnosis — checks nodes, services, drift, and doctor findings.",
		Tags:        []string{"ops", "health", "troubleshooting"},
		Context:     "This skill performs a top-down health assessment: cluster overview, per-node health, drift detection, and doctor analysis. Present findings grouped by severity.",
		OutputFormat: "Structured report with sections: Summary, Nodes, Drift, Findings. Use severity levels: critical, warning, info.",
		Source:      "builtin",
		Steps: []SkillStep{
			{Order: 1, Instruction: "Get cluster health overview", Tool: "cluster_get_health"},
			{Order: 2, Instruction: "List all nodes and their status", Tool: "cluster_list_nodes"},
			{Order: 3, Instruction: "For each node that is not healthy, get detailed health info", Tool: "cluster_get_node_health_detail", Condition: "any node reports unhealthy"},
			{Order: 4, Instruction: "Check for drift between desired and installed state", Tool: "cluster_get_drift_report"},
			{Order: 5, Instruction: "Run doctor analysis for deeper issues", Tool: "cluster_get_doctor_report"},
			{Order: 6, Instruction: "If critical findings exist, explain the most severe one", Tool: "cluster_explain_finding", Condition: "doctor report has critical findings"},
			{Order: 7, Instruction: "Summarize all findings into a structured report, grouped by severity. Recommend next actions for anything critical or warning-level."},
		},
	},
	"deploy-check": {
		Name:        "deploy-check",
		Description: "Pre-deployment readiness check — verifies cluster health, reconciliation status, and convergence.",
		Tags:        []string{"ops", "deploy", "safety"},
		Context:     "Run this before deploying new service versions or making cluster changes. It verifies the cluster is in a good state to accept changes.",
		OutputFormat: "Go/no-go decision with supporting evidence.",
		Source:      "builtin",
		Steps: []SkillStep{
			{Order: 1, Instruction: "Check overall cluster health", Tool: "cluster_get_health"},
			{Order: 2, Instruction: "Verify no active reconciliation is in progress", Tool: "cluster_get_reconcile_status"},
			{Order: 3, Instruction: "Check convergence — are all nodes converged?", Tool: "cluster_get_convergence_detail"},
			{Order: 4, Instruction: "Check for existing drift that should be resolved first", Tool: "cluster_get_drift_report"},
			{Order: 5, Instruction: "Verify backup system is healthy", Tool: "backup_get_recovery_posture", OnError: "Note backup status as degraded but don't block deployment"},
			{Order: 6, Instruction: "Issue a GO or NO-GO recommendation. NO-GO if: any node is down, active reconciliation in progress, or critical drift exists."},
		},
	},
	"recover-etcd": {
		Name:        "recover-etcd",
		Description: "Guided etcd cluster recovery — diagnoses etcd health and walks through recovery steps.",
		Tags:        []string{"ops", "etcd", "recovery", "troubleshooting"},
		Context:     "etcd is the distributed key-value store underlying all cluster state. Recovery must be done carefully to avoid data loss.",
		OutputFormat: "Step-by-step recovery plan with commands to run. Always confirm with user before destructive steps.",
		Source:      "builtin",
		Steps: []SkillStep{
			{Order: 1, Instruction: "Check cluster health to understand overall state", Tool: "cluster_get_health"},
			{Order: 2, Instruction: "List nodes to identify which ones have etcd issues", Tool: "cluster_list_nodes"},
			{Order: 3, Instruction: "For each node, check the etcd service status via node agent", Tool: "nodeagent_get_service_logs", Args: map[string]string{"service": "globular-etcd", "lines": "50"}},
			{Order: 4, Instruction: "Check if the etcd package is installed on affected nodes", Tool: "nodeagent_get_installed_package", Args: map[string]string{"package_id": "globular-etcd"}},
			{Order: 5, Instruction: "Based on findings, determine failure mode and present a recovery plan. ALWAYS ask user for confirmation before destructive steps."},
		},
	},
	"service-status": {
		Name:        "service-status",
		Description: "Deep status check for a specific service — desired state, installed state, health, logs, and recent events.",
		Tags:        []string{"ops", "service", "troubleshooting"},
		Context:     "Use when investigating a specific service. Pulls information from multiple sources for a complete picture.",
		OutputFormat: "Service status card: desired version, installed version, health, recent logs, drift status.",
		Source:      "builtin",
		Steps: []SkillStep{
			{Order: 1, Instruction: "Get the desired state for this service", Tool: "cluster_get_desired_state"},
			{Order: 2, Instruction: "Check what version is actually installed on each node", Tool: "cluster_get_drift_report"},
			{Order: 3, Instruction: "Get the operational snapshot for runtime health", Tool: "cluster_get_operational_snapshot"},
			{Order: 4, Instruction: "Get recent logs for the service", Tool: "nodeagent_get_service_logs", Args: map[string]string{"lines": "30"}},
			{Order: 5, Instruction: "Check if there's an active plan for this service", Tool: "cluster_get_service_workflow_status"},
			{Order: 6, Instruction: "Present a unified status card. Flag any discrepancies between desired, installed, and runtime state."},
		},
	},
	"investigate-incident": {
		Name:        "investigate-incident",
		Description: "Investigate a cluster incident — gather evidence, check memories for similar past issues, propose root cause.",
		Tags:        []string{"ops", "incident", "troubleshooting", "ai"},
		Context:     "Combines live cluster data with historical knowledge from the AI memory service.",
		OutputFormat: "Incident report: timeline, evidence, similar past incidents, probable root cause, recommended fix.",
		Source:      "builtin",
		Steps: []SkillStep{
			{Order: 1, Instruction: "Get current cluster health and operational snapshot", Tool: "cluster_get_health"},
			{Order: 2, Instruction: "Get the doctor report for automated findings", Tool: "cluster_get_doctor_report"},
			{Order: 3, Instruction: "Search memory for similar past incidents", Tool: "memory_query", Args: map[string]string{"project": "globular-services", "type": "debug"}},
			{Order: 4, Instruction: "Check for relevant architecture knowledge", Tool: "memory_query", Args: map[string]string{"project": "globular-services", "type": "architecture"}},
			{Order: 5, Instruction: "Get node-level details for any affected nodes", Tool: "cluster_get_node_full_status", Condition: "specific nodes are affected"},
			{Order: 6, Instruction: "Correlate findings into an incident report. Reference matching past memories. Propose root cause and fix."},
		},
	},
	"rbac-audit": {
		Name:        "rbac-audit",
		Description: "Audit RBAC permissions for a user or service account — shows what they can access and why.",
		Tags:        []string{"security", "rbac", "audit"},
		Context:     "Shows the full permission chain: subject -> role bindings -> roles -> allowed actions.",
		OutputFormat: "Permission matrix: subject, role bindings, effective permissions, access gaps.",
		Source:      "builtin",
		Steps: []SkillStep{
			{Order: 1, Instruction: "Get permissions for the subject", Tool: "rbac_get_permissions_by_subject"},
			{Order: 2, Instruction: "List all role bindings", Tool: "rbac_list_role_bindings"},
			{Order: 3, Instruction: "Get the subject's identity context for group memberships", Tool: "resource_get_account_identity_context"},
			{Order: 4, Instruction: "Explain the effective access snapshot", Tool: "rbac_explain_access_snapshot"},
			{Order: 5, Instruction: "Present a clear permission matrix. Flag overly broad permissions or suspicious gaps."},
		},
	},
}

// ── ScyllaDB-backed skill loading ───────────────────────────────────────────

// skillFromMemory converts a Memory entry (type=SKILL) into a Skill.
// The memory content is a JSON-encoded Skill struct.
func skillFromMemory(m *ai_memorypb.Memory) (Skill, error) {
	var sk Skill
	if err := json.Unmarshal([]byte(m.GetContent()), &sk); err != nil {
		return sk, fmt.Errorf("invalid skill JSON in memory %s: %w", m.GetId(), err)
	}
	sk.ID = m.GetId()
	sk.Source = "custom"
	return sk, nil
}

// skillToContent serializes a Skill to JSON for storage in memory content.
func skillToContent(sk Skill) (string, error) {
	data, err := json.MarshalIndent(sk, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// loadCustomSkills fetches all SKILL-type memories from ScyllaDB.
func loadCustomSkills(ctx context.Context, s *server) (map[string]Skill, error) {
	conn, err := s.clients.get(ctx, memoryEndpoint())
	if err != nil {
		return nil, err
	}
	client := ai_memorypb.NewAiMemoryServiceClient(conn)

	callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
	defer cancel()

	rsp, err := client.Query(callCtx, &ai_memorypb.QueryRqst{
		Project: "globular-services",
		Type:    ai_memorypb.MemoryType_SKILL,
		Limit:   100,
	})
	if err != nil {
		return nil, err
	}

	customs := make(map[string]Skill)
	for _, m := range rsp.GetMemories() {
		sk, err := skillFromMemory(m)
		if err != nil {
			continue // skip malformed entries
		}
		customs[sk.Name] = sk
	}
	return customs, nil
}

// mergedSkills returns builtins merged with custom skills (custom wins on name collision).
func mergedSkills(ctx context.Context, s *server) map[string]Skill {
	result := make(map[string]Skill, len(builtinSkills))
	for k, v := range builtinSkills {
		result[k] = v
	}

	customs, err := loadCustomSkills(ctx, s)
	if err != nil {
		// ScyllaDB unavailable — just use builtins.
		return result
	}
	for k, v := range customs {
		result[k] = v
	}
	return result
}

// ── MCP Tool Registration ───────────────────────────────────────────────────

func registerSkillsTools(srv *server) {

	// ── skill_list ──────────────────────────────────────────────────────
	srv.register(toolDef{
		Name:        "skill_list",
		Description: "List available operational skills (prompt playbooks). Skills guide you through multi-step tasks using other MCP tools. Includes both built-in and custom skills stored in ScyllaDB. Filter by tag to find relevant skills.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"tag": {Type: "string", Description: "Filter skills by tag (e.g. \"ops\", \"troubleshooting\", \"security\"). Empty returns all."},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		tag := getStr(args, "tag")
		all := mergedSkills(ctx, srv)

		summaries := make([]map[string]interface{}, 0, len(all))
		for _, sk := range all {
			if tag != "" {
				found := false
				for _, t := range sk.Tags {
					if t == tag {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			summaries = append(summaries, map[string]interface{}{
				"name":        sk.Name,
				"description": sk.Description,
				"tags":        sk.Tags,
				"steps":       len(sk.Steps),
				"source":      sk.Source,
			})
		}
		return map[string]interface{}{
			"total":  len(summaries),
			"skills": summaries,
		}, nil
	})

	// ── skill_execute ───────────────────────────────────────────────────
	srv.register(toolDef{
		Name: "skill_execute",
		Description: "Get a full skill playbook to execute. Returns the complete prompt template with steps, tools to call, and output format. " +
			"Follow the steps in order, calling the specified MCP tools. " +
			"Pass context_args to customize (e.g. {\"node\":\"nuc\",\"service\":\"dns\"}). " +
			"Set track=true to create a workflow session — this enables step tracking, approval gates for CLI commands, " +
			"and failure branching. When tracked, pass the returned workflow_id to globular_cli.execute and " +
			"use globular_cli.workflow_advance for non-CLI steps.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"name":         {Type: "string", Description: "Skill name (from skill_list)"},
				"context_args": {Type: "string", Description: "JSON object of context arguments to customize the skill"},
				"track":        {Type: "boolean", Description: "If true, create a workflow session for step tracking and governor integration (default: false)"},
			},
			Required: []string{"name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name := getStr(args, "name")
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}

		all := mergedSkills(ctx, srv)
		sk, ok := all[name]
		if !ok {
			available := make([]string, 0, len(all))
			for k := range all {
				available = append(available, k)
			}
			return map[string]interface{}{
				"error":            fmt.Sprintf("unknown skill: %s", name),
				"available_skills": available,
			}, nil
		}

		track := getBool(args, "track", false)
		contextArgs := parseContextArgs(getStr(args, "context_args"))

		steps := make([]map[string]interface{}, 0, len(sk.Steps))
		for _, step := range sk.Steps {
			s := map[string]interface{}{
				"order":       step.Order,
				"instruction": substituteContext(step.Instruction, contextArgs),
			}
			if step.Tool != "" {
				s["tool"] = step.Tool
				// Flag steps that go through the governor.
				if isGovernedTool(step.Tool) {
					s["governor"] = true
				}
			}
			if len(step.Args) > 0 {
				merged := make(map[string]string, len(step.Args))
				for k, v := range step.Args {
					merged[k] = substituteContext(v, contextArgs)
				}
				for k, v := range contextArgs {
					if _, exists := merged[k]; exists {
						merged[k] = v
					}
				}
				s["args"] = merged
			}
			if step.OnError != "" {
				s["on_error"] = step.OnError
			}
			if step.Condition != "" {
				s["condition"] = step.Condition
			}
			steps = append(steps, s)
		}

		result := map[string]interface{}{
			"name":          sk.Name,
			"description":   sk.Description,
			"context":       sk.Context,
			"output_format": sk.OutputFormat,
			"source":        sk.Source,
			"steps":         steps,
			"context_args":  contextArgs,
		}

		// Create a tracked workflow session if requested.
		if track {
			wfSteps := make([]WorkflowStepStatus, 0, len(sk.Steps))
			for _, step := range sk.Steps {
				wfSteps = append(wfSteps, WorkflowStepStatus{
					StepName: substituteContext(step.Instruction, contextArgs),
					Order:    step.Order,
					Status:   StepPending,
				})
			}
			wfCtx := map[string]string{"source": "skill", "skill_name": name}
			for k, v := range contextArgs {
				wfCtx[k] = v
			}
			session, err := activeWorkflows.StartCustomWorkflow(name, wfSteps, wfCtx)
			if err != nil {
				result["track_error"] = err.Error()
			} else {
				result["workflow_id"] = session.ID
				result["tracking"] = map[string]interface{}{
					"status":       string(session.Status),
					"current_step": session.CurrentStep,
					"total_steps":  session.TotalSteps,
					"usage": "Pass workflow_id to globular_cli.execute for governed CLI steps. " +
						"Use globular_cli.workflow_advance to complete/fail/skip non-CLI steps.",
				}
			}
		}

		return result, nil
	})

	// ── skill_create ────────────────────────────────────────────────────
	srv.register(toolDef{
		Name: "skill_create",
		Description: "Create a new custom skill and store it in ScyllaDB. The skill becomes immediately available to all agents in the cluster. " +
			"Provide the skill definition as a JSON object with name, description, tags, steps, context, and output_format.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"name":          {Type: "string", Description: "Unique skill name (lowercase, hyphens OK)"},
				"description":   {Type: "string", Description: "What the skill does (shown in skill_list)"},
				"tags":          {Type: "string", Description: "Comma-separated tags (e.g. \"ops,troubleshooting\")"},
				"context":       {Type: "string", Description: "Background knowledge the agent needs before starting"},
				"output_format": {Type: "string", Description: "How the agent should present the final result"},
				"steps":         {Type: "string", Description: "JSON array of steps: [{\"order\":1,\"instruction\":\"...\",\"tool\":\"...\",\"args\":{},\"on_error\":\"...\",\"condition\":\"...\"}]"},
			},
			Required: []string{"name", "description", "steps"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name := getStr(args, "name")
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}

		// Parse steps from JSON.
		var steps []SkillStep
		if err := json.Unmarshal([]byte(getStr(args, "steps")), &steps); err != nil {
			return nil, fmt.Errorf("invalid steps JSON: %w", err)
		}
		if len(steps) == 0 {
			return nil, fmt.Errorf("at least one step is required")
		}

		sk := Skill{
			Name:         name,
			Description:  getStr(args, "description"),
			Context:      getStr(args, "context"),
			OutputFormat: getStr(args, "output_format"),
			Steps:        steps,
			Source:       "custom",
		}
		if tags := getStr(args, "tags"); tags != "" {
			for _, t := range strings.Split(tags, ",") {
				if t = strings.TrimSpace(t); t != "" {
					sk.Tags = append(sk.Tags, t)
				}
			}
		}

		content, err := skillToContent(sk)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize skill: %w", err)
		}

		// Store as a SKILL-type memory.
		conn, err := srv.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		memory := &ai_memorypb.Memory{
			Project:   "globular-services",
			Type:      ai_memorypb.MemoryType_SKILL,
			Title:     fmt.Sprintf("skill:%s", name),
			Content:   content,
			Tags:      append([]string{"skill"}, sk.Tags...),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
			AgentId:   "claude-mcp",
			Metadata:  map[string]string{"skill_name": name},
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.Store(callCtx, &ai_memorypb.StoreRqst{Memory: memory})
		if err != nil {
			return nil, fmt.Errorf("skill_create: %w", err)
		}

		return map[string]interface{}{
			"id":     rsp.GetId(),
			"name":   name,
			"status": "created",
			"source": "custom",
			"steps":  len(steps),
		}, nil
	})

	// ── skill_delete ────────────────────────────────────────────────────
	srv.register(toolDef{
		Name:        "skill_delete",
		Description: "Delete a custom skill from ScyllaDB. Built-in skills cannot be deleted (but can be overridden with skill_create).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"name": {Type: "string", Description: "Skill name to delete"},
			},
			Required: []string{"name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name := getStr(args, "name")
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}

		// Find the custom skill's memory ID.
		conn, err := srv.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.Query(callCtx, &ai_memorypb.QueryRqst{
			Project:    "globular-services",
			Type:       ai_memorypb.MemoryType_SKILL,
			TextSearch: fmt.Sprintf("skill:%s", name),
			Limit:      5,
		})
		if err != nil {
			return nil, fmt.Errorf("skill_delete: query failed: %w", err)
		}

		// Find the exact match.
		var memoryID string
		for _, m := range rsp.GetMemories() {
			if m.GetTitle() == fmt.Sprintf("skill:%s", name) {
				memoryID = m.GetId()
				break
			}
		}
		if memoryID == "" {
			return map[string]interface{}{
				"error": fmt.Sprintf("custom skill %q not found (built-in skills cannot be deleted)", name),
			}, nil
		}

		delCtx, delCancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer delCancel()

		delRsp, err := client.Delete(delCtx, &ai_memorypb.DeleteRqst{
			Id:      memoryID,
			Project: "globular-services",
		})
		if err != nil {
			return nil, fmt.Errorf("skill_delete: %w", err)
		}

		return map[string]interface{}{
			"success": delRsp.GetSuccess(),
			"name":    name,
			"id":      memoryID,
		}, nil
	})
}

// parseContextArgs extracts key-value pairs from a JSON string.
func parseContextArgs(raw string) map[string]string {
	result := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result
	}
	// Try proper JSON first.
	if err := json.Unmarshal([]byte(raw), &result); err == nil {
		return result
	}
	// Fallback: simple key:value parsing.
	if strings.HasPrefix(raw, "{") {
		pairs := strings.TrimRight(strings.TrimLeft(raw, "{"), "}")
		for _, pair := range strings.Split(pairs, ",") {
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) == 2 {
				k := strings.Trim(strings.TrimSpace(kv[0]), "\"")
				v := strings.Trim(strings.TrimSpace(kv[1]), "\"")
				result[k] = v
			}
		}
	}
	return result
}

// substituteContext replaces {{key}} placeholders with context values.
func substituteContext(s string, ctx map[string]string) string {
	for k, v := range ctx {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

// isGovernedTool returns true if a tool name routes through the CLI governor
// (approval gates, preconditions, branching).
func isGovernedTool(tool string) bool {
	switch tool {
	case "globular_cli_execute", "globular_cli.execute",
		"globular_cli_execute_plan", "globular_cli.execute_plan":
		return true
	}
	return false
}
