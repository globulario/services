package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func registerGovernorTools(s *server) {

	// ── globular_cli.validate ────────────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.validate",
		Description: `Validates a CLI command BEFORE execution. Returns structured status: allowed, blocked, needs_confirmation, invalid, or out_of_order.

Checks performed:
- command exists in knowledge base
- required flags are present
- flag values match allowed enums
- approval policies (destructive/publish/production operations)
- state-aware preconditions (build compiles, tests pass, proto exists)
- workflow step ordering (if workflow_id provided)
- generated file protection

Always call this before executing any CLI command.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"command":     {Type: "string", Description: "The full CLI command to validate (e.g. \"globular generate service --proto proto/echo.proto --lang go --out ./echo\")"},
				"args":        {Type: "array", Description: "Optional: command arguments as separate array items"},
				"workflow_id": {Type: "string", Description: "Optional: active workflow session ID for step ordering enforcement"},
				"with_state":  {Type: "boolean", Description: "If true, collect project state for precondition checks (slower but more thorough)"},
				"project_dir": {Type: "string", Description: "Project directory for state collection (default: current dir)"},
			},
			Required: []string{"command"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		command := getStr(args, "command")
		if command == "" {
			return nil, fmt.Errorf("command is required")
		}

		workflowID := getStr(args, "workflow_id")
		withState := getBool(args, "with_state", false)
		projectDir := getStr(args, "project_dir")

		// Parse args from the command string or from the args array
		var cmdArgs []string
		if rawArgs, ok := args["args"]; ok {
			if arr, ok := rawArgs.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						cmdArgs = append(cmdArgs, s)
					}
				}
			}
		}

		// If no separate args, parse from command string
		if len(cmdArgs) == 0 {
			parts := strings.Fields(command)
			if len(parts) > 1 {
				for i, p := range parts {
					if strings.HasPrefix(p, "-") {
						command = strings.Join(parts[:i], " ")
						cmdArgs = parts[i:]
						break
					}
				}
			}
		}

		req := ValidationRequest{
			Command:      command,
			Args:         cmdArgs,
			WorkflowStep: workflowID,
		}

		// Optionally collect state for precondition checks
		var state *StateSnapshot
		if withState {
			dir := projectDir
			if dir == "" {
				dir = "."
			}
			snap := CollectStateSnapshot(dir)
			state = &snap
		}

		result := ValidateCommandWithState(req, state)

		out := map[string]interface{}{
			"status":               string(result.Status),
			"reason":               result.Reason,
			"missing_requirements": result.MissingRequirements,
			"suggested_next_step":  result.SuggestedNextStep,
			"requires_approval":    result.RequiresApproval,
			"matched_command":      result.MatchedCommand,
		}

		// Include precondition details if state was checked
		if withState {
			cmdPath := extractCommandPath(command, cmdArgs)
			precondResults := EvaluatePreconditions(cmdPath, cmdArgs, state)
			if len(precondResults) > 0 {
				out["preconditions"] = precondResults
			}
		}

		return out, nil
	})

	// ── globular_cli.execute ─────────────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.execute",
		Description: `Executes a CLI command through the Execution Governor. The command is validated first — if validation fails, no execution occurs.

Returns structured result with:
- success, exit_code, stdout, stderr, duration_ms
- produced_artifacts (detected .tgz, .tar.gz, etc.)
- changed_files (git diff pre/post)
- detected_state_changes (generated, published, installed, etc.)
- warnings, errors
- recommended_next_actions
- branch decision (continue/retry/stop/remediate/request_approval)
- validation result

Requires read_only=false in MCP config.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"command":     {Type: "string", Description: "The full CLI command to execute"},
				"workflow_id": {Type: "string", Description: "Optional: active workflow session ID — step will be advanced on success"},
				"approved":    {Type: "boolean", Description: "Set to true to bypass approval gates for commands that need confirmation"},
			},
			Required: []string{"command"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if s.cfg.ReadOnly {
			return map[string]interface{}{
				"success":    false,
				"exit_code":  -1,
				"error":      "execution blocked: MCP server is in read-only mode",
				"suggestion": "Set read_only=false in MCP config to enable command execution",
			}, nil
		}

		command := getStr(args, "command")
		if command == "" {
			return nil, fmt.Errorf("command is required")
		}

		workflowID := getStr(args, "workflow_id")
		approved := getBool(args, "approved", false)

		// Parse command into parts
		parts := strings.Fields(command)
		var cmdPath string
		var cmdArgs []string
		for i, p := range parts {
			if strings.HasPrefix(p, "-") {
				cmdPath = strings.Join(parts[:i], " ")
				cmdArgs = parts[i:]
				break
			}
		}
		if cmdPath == "" {
			cmdPath = command
		}

		// If approved, temporarily allow the command past approval gates
		req := ValidationRequest{
			Command:      cmdPath,
			Args:         cmdArgs,
			WorkflowStep: workflowID,
		}

		// Pre-validate with approval bypass if approved
		if approved {
			validation := ValidateCommand(req)
			if validation.Status == StatusNeedsConfirmation {
				// User explicitly approved — proceed with execution
				// We'll skip the governor's validation in ExecuteCommand and run directly
				return executeApproved(req, workflowID)
			}
		}

		result := ExecuteCommand(req)

		// If within a workflow, advance or fail the step
		if workflowID != "" {
			if result.Success {
				activeWorkflows.AdvanceStep(workflowID, "success")
			} else if result.Branch != nil && result.Branch.Action == "stop" {
				activeWorkflows.FailStep(workflowID, strings.Join(result.Errors, "; "))
			}
		}

		return formatExecutionResult(result), nil
	})

	// ── globular_cli.state ───────────────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.state",
		Description: `Returns a snapshot of the current project and system state.

Inspects:
- project_state: go.mod, proto files, service dirs, git status/branch
- build_state: compiles, tests pass, errors
- generation_state: generated vs handwritten file inventory

Use before and after operations to verify state transitions.
Use with globular_cli.validate --with_state for precondition enforcement.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project_dir": {Type: "string", Description: "Project root directory to inspect (default: current working directory)"},
				"skip_build":  {Type: "boolean", Description: "Skip build/test check for faster response (default: false)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		projectDir := getStr(args, "project_dir")
		skipBuild := getBool(args, "skip_build", false)

		if projectDir == "" {
			projectDir = "."
		}

		snap := CollectStateSnapshot(projectDir)

		if skipBuild {
			snap.BuildState = BuildState{
				LastChecked: "skipped",
			}
		}

		return map[string]interface{}{
			"timestamp":        snap.Timestamp,
			"project_state":    snap.ProjectState,
			"build_state":      snap.BuildState,
			"generation_state": snap.GenerationState,
		}, nil
	})

	// ── globular_cli.workflow_start ───────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.workflow_start",
		Description: `Starts a new tracked workflow session. Returns a workflow_id that should be passed to subsequent validate/execute calls for step ordering enforcement.

Available tasks: create_service, publish_package, bootstrap_cluster.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {Type: "string", Description: "The workflow task to start (e.g. \"create_service\", \"publish_package\", \"bootstrap_cluster\")"},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task := getStr(args, "task")
		if task == "" {
			return nil, fmt.Errorf("task is required")
		}

		session, err := activeWorkflows.StartWorkflow(task)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"workflow_id":  session.ID,
			"task":         session.Task,
			"status":       string(session.Status),
			"current_step": session.CurrentStep,
			"total_steps":  session.TotalSteps,
			"steps":        session.Steps,
		}, nil
	})

	// ── globular_cli.workflow_status ──────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.workflow_status",
		Description: `Returns the current status of a workflow session including all step statuses, current step, and overall progress. Can also list all active workflows.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"workflow_id": {Type: "string", Description: "Workflow session ID to query (omit to list all active workflows)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		workflowID := getStr(args, "workflow_id")

		if workflowID == "" {
			// List all sessions
			sessions := activeWorkflows.ListSessions()
			summaries := make([]map[string]interface{}, 0, len(sessions))
			for _, s := range sessions {
				summaries = append(summaries, map[string]interface{}{
					"workflow_id":  s.ID,
					"task":         s.Task,
					"status":       string(s.Status),
					"current_step": s.CurrentStep,
					"total_steps":  s.TotalSteps,
					"started_at":   s.StartedAt,
				})
			}
			return map[string]interface{}{
				"workflows": summaries,
				"count":     len(summaries),
			}, nil
		}

		session, ok := activeWorkflows.GetSession(workflowID)
		if !ok {
			return nil, fmt.Errorf("workflow session %q not found", workflowID)
		}

		return map[string]interface{}{
			"workflow_id":  session.ID,
			"task":         session.Task,
			"status":       string(session.Status),
			"started_at":   session.StartedAt,
			"current_step": session.CurrentStep,
			"total_steps":  session.TotalSteps,
			"steps":        session.Steps,
			"context":      session.Context,
		}, nil
	})

	// ── globular_cli.workflow_advance ─────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.workflow_advance",
		Description: `Manually advance, fail, or abort a workflow step. Use this for non-command steps (e.g. "propose", "implement", "wait_approval") that don't go through globular_cli.execute.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"workflow_id": {Type: "string", Description: "Workflow session ID"},
				"action":      {Type: "string", Description: "Action to take", Enum: []string{"complete", "fail", "abort", "skip"}},
				"result":      {Type: "string", Description: "Result description or error message"},
			},
			Required: []string{"workflow_id", "action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		workflowID := getStr(args, "workflow_id")
		action := getStr(args, "action")
		result := getStr(args, "result")

		if workflowID == "" {
			return nil, fmt.Errorf("workflow_id is required")
		}

		var session *WorkflowSession
		var err error

		switch action {
		case "complete":
			session, err = activeWorkflows.AdvanceStep(workflowID, result)
		case "fail":
			session, err = activeWorkflows.FailStep(workflowID, result)
		case "abort":
			session, err = activeWorkflows.AbortWorkflow(workflowID)
		case "skip":
			// Skip current step and advance
			session, err = activeWorkflows.AdvanceStep(workflowID, "skipped")
			if err == nil {
				// Mark the completed step as skipped
				idx := session.CurrentStep - 2 // previous step (just advanced)
				if idx >= 0 && idx < len(session.Steps) {
					session.Steps[idx].Skipped = true
				}
			}
		default:
			return nil, fmt.Errorf("invalid action %q — use: complete, fail, abort, skip", action)
		}

		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"workflow_id":  session.ID,
			"status":       string(session.Status),
			"current_step": session.CurrentStep,
			"total_steps":  session.TotalSteps,
			"steps":        session.Steps,
		}, nil
	})

	// ── globular_cli.check_approval ──────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.check_approval",
		Description: `Checks whether a command requires user approval before execution. Returns the approval requirement with scope, reason, and whether confirmation is required. Use this before globular_cli.execute for commands that might be destructive or affect production.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"command": {Type: "string", Description: "The command to check (e.g. \"globular pkg publish\", \"globular services repair\")"},
			},
			Required: []string{"command"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		command := getStr(args, "command")
		if command == "" {
			return nil, fmt.Errorf("command is required")
		}

		cmdPath := extractCommandPath(command, nil)
		approval := CheckApproval(cmdPath)

		if approval == nil {
			return map[string]interface{}{
				"requires_approval": false,
				"command":           cmdPath,
			}, nil
		}

		return map[string]interface{}{
			"requires_approval":          approval.RequiresUserConfirmation,
			"command":                    cmdPath,
			"scope":                      approval.Scope,
			"reason":                     approval.Reason,
			"action":                     approval.Action,
		}, nil
	})

	// ── globular_cli.state_diff ──────────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.state_diff",
		Description: `Compares two state snapshots and returns the differences. Use this to evaluate state transitions after operations.

Detects:
- build status changes (compiles before/after, tests before/after)
- new generated files
- added/removed proto files and service directories
- git clean status changes
- unexpected regressions (build broke, tests broke)

Call globular_cli.state before an operation, then again after, and pass both timestamps to this tool. Or provide two project directories to compare.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project_dir":  {Type: "string", Description: "Project directory to inspect (default: current dir)"},
				"before_state": {Type: "object", Description: "The 'before' state snapshot (from a previous globular_cli.state call)"},
			},
			Required: []string{"before_state"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		projectDir := getStr(args, "project_dir")
		if projectDir == "" {
			projectDir = "."
		}

		// The before_state is expected as a nested object with the same structure
		// as a StateSnapshot. We extract key fields from it.
		beforeRaw, ok := args["before_state"]
		if !ok {
			return nil, fmt.Errorf("before_state is required")
		}

		// Parse the before state from the raw map
		beforeSnap := parseStateFromArgs(beforeRaw)

		// Collect current state
		afterSnap := CollectStateSnapshot(projectDir)

		// Compare
		diff := CompareSnapshots(beforeSnap, afterSnap)

		out := map[string]interface{}{
			"timestamp":          diff.Timestamp,
			"before_timestamp":   diff.Before,
			"after_timestamp":    diff.After,
			"build_changed":      diff.BuildChanged,
			"build_before":       diff.BuildBefore,
			"build_after":        diff.BuildAfter,
			"tests_changed":      diff.TestsChanged,
			"tests_before":       diff.TestsBefore,
			"tests_after":        diff.TestsAfter,
			"git_clean_changed":  diff.GitCleanChanged,
		}
		if len(diff.FilesAdded) > 0 {
			out["files_added"] = diff.FilesAdded
		}
		if len(diff.FilesRemoved) > 0 {
			out["files_removed"] = diff.FilesRemoved
		}
		if len(diff.NewGeneratedFiles) > 0 {
			out["new_generated_files"] = diff.NewGeneratedFiles
		}
		if len(diff.Unexpected) > 0 {
			out["unexpected_changes"] = diff.Unexpected
		}

		return out, nil
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// executeApproved runs a command that was explicitly approved, bypassing the
// needs_confirmation validation status.
func executeApproved(req ValidationRequest, workflowID string) (interface{}, error) {
	cmdArgs := append([]string{}, req.Args...)
	fullCmd := req.Command
	if len(cmdArgs) > 0 {
		fullCmd = req.Command + " " + strings.Join(cmdArgs, " ")
	}

	result := executeRaw(fullCmd, req.Command, req.Args)

	// Workflow advancement
	if workflowID != "" {
		if result.Success {
			activeWorkflows.AdvanceStep(workflowID, "success (approved)")
		} else {
			activeWorkflows.FailStep(workflowID, strings.Join(result.Errors, "; "))
		}
	}

	return formatExecutionResult(result), nil
}

// executeRaw runs a shell command without validation, capturing full structured output.
func executeRaw(fullCmd, command string, args []string) ExecutionResult {
	result := ExecutionResult{
		Command: command,
		Args:    args,
	}

	preGitStatus := captureGitStatus()

	start := time.Now()
	cmd := exec.Command("sh", "-c", fullCmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.DurationMs = time.Since(start).Milliseconds()
	result.Stdout = truncateOutput(stdout.String(), 64*1024)
	result.Stderr = truncateOutput(stderr.String(), 16*1024)
	result.Timestamp = time.Now().UTC().Format(time.RFC3339)

	if err != nil {
		result.Success = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Errors = []string{err.Error()}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	result.ProducedArtifacts = detectArtifacts(result.Stdout)
	result.ChangedFiles = detectChangedFiles(preGitStatus)
	result.DetectedStateChanges = detectStateChanges(result.Stdout, result.Stderr)
	branch := DecideBranch(result, "")
	result.Branch = &branch

	return result
}

// parseStateFromArgs reconstructs a StateSnapshot from a raw MCP argument map.
// This is used to accept a previous state snapshot for comparison.
func parseStateFromArgs(raw interface{}) StateSnapshot {
	snap := StateSnapshot{}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return snap
	}

	if ts, ok := m["timestamp"].(string); ok {
		snap.Timestamp = ts
	}

	// Parse project_state
	if ps, ok := m["project_state"].(map[string]interface{}); ok {
		if v, ok := ps["has_go_mod"].(bool); ok {
			snap.ProjectState.HasGoMod = v
		}
		if v, ok := ps["has_proto_dir"].(bool); ok {
			snap.ProjectState.HasProtoDir = v
		}
		if v, ok := ps["git_clean"].(bool); ok {
			snap.ProjectState.GitClean = v
		}
		if v, ok := ps["git_branch"].(string); ok {
			snap.ProjectState.GitBranch = v
		}
		if arr, ok := ps["proto_files"].([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					snap.ProjectState.ProtoFiles = append(snap.ProjectState.ProtoFiles, s)
				}
			}
		}
		if arr, ok := ps["service_dirs"].([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					snap.ProjectState.ServiceDirs = append(snap.ProjectState.ServiceDirs, s)
				}
			}
		}
	}

	// Parse build_state
	if bs, ok := m["build_state"].(map[string]interface{}); ok {
		if v, ok := bs["compiles"].(bool); ok {
			snap.BuildState.Compiles = v
		}
		if v, ok := bs["tests_passed"].(bool); ok {
			snap.BuildState.TestsPassed = v
		}
		if v, ok := bs["last_checked"].(string); ok {
			snap.BuildState.LastChecked = v
		}
	}

	// Parse generation_state
	if gs, ok := m["generation_state"].(map[string]interface{}); ok {
		if arr, ok := gs["generated_files"].([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					snap.GenerationState.GeneratedFiles = append(snap.GenerationState.GeneratedFiles, s)
				}
			}
		}
	}

	return snap
}

func formatExecutionResult(result ExecutionResult) map[string]interface{} {
	out := map[string]interface{}{
		"command":     result.Command,
		"args":        result.Args,
		"timestamp":   result.Timestamp,
		"success":     result.Success,
		"exit_code":   result.ExitCode,
		"duration_ms": result.DurationMs,
	}

	if result.Stdout != "" {
		out["stdout"] = result.Stdout
	}
	if result.Stderr != "" {
		out["stderr"] = result.Stderr
	}
	if len(result.ProducedArtifacts) > 0 {
		out["produced_artifacts"] = result.ProducedArtifacts
	}
	if len(result.ChangedFiles) > 0 {
		out["changed_files"] = result.ChangedFiles
	}
	if len(result.DetectedStateChanges) > 0 {
		out["detected_state_changes"] = result.DetectedStateChanges
	}
	if len(result.Warnings) > 0 {
		out["warnings"] = result.Warnings
	}
	if len(result.Errors) > 0 {
		out["errors"] = result.Errors
	}
	if len(result.RecommendedNextActions) > 0 {
		out["recommended_next_actions"] = result.RecommendedNextActions
	}
	if result.Validation != nil {
		out["validation"] = map[string]interface{}{
			"status":  string(result.Validation.Status),
			"reason":  result.Validation.Reason,
			"command": result.Validation.MatchedCommand,
		}
	}
	if result.Branch != nil {
		out["branch"] = map[string]interface{}{
			"action":       result.Branch.Action,
			"reason":       result.Branch.Reason,
			"next_command": result.Branch.NextCommand,
		}
	}

	return out
}
