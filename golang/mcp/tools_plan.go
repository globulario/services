package main

import (
	"context"
	"fmt"
)

func registerPlanTools(s *server) {

	// ── globular_cli.plan ───────────────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.plan",
		Description: `Pre-install impact analysis and admission gate. Evaluates a requested operation against the current cluster state and predicts its impact BEFORE execution.

Returns:
- operation_plan: normalized representation of the intended action
- impact_report: predicted effects (dependencies, port conflicts, package status, risk level)
- admission_decision: deterministic verdict — allow, allow_with_approval, block, or requires_remediation

Checks performed:
- package exists in repository
- required dependencies are installed
- port conflicts with running services
- approval policy evaluation

This tool is read-only. It does not execute anything.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"command": {Type: "string", Description: "The full CLI command to evaluate (e.g. \"globular services desired set ldap 0.0.1\")"},
				"args":    {Type: "array", Description: "Optional: command arguments as separate array items"},
			},
			Required: []string{"command"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		command := getStr(args, "command")
		if command == "" {
			return nil, fmt.Errorf("command is required")
		}

		// Parse optional args array
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

		// Normalize the command into an OperationPlan
		plan := NormalizePlan(command, cmdArgs)

		// Run the admission pipeline (descriptor-first, static fallback)
		planner := NewPlannerWithDescriptor(
			NewStaticDependencySource(),
			NewRepoDescriptorSource(s.clients),
			s.clients,
		)
		report, decision := planner.Evaluate(ctx, plan)

		// Format output
		out := map[string]interface{}{
			"operation_plan": map[string]interface{}{
				"command":        plan.Command,
				"args":           plan.Args,
				"target_service": plan.TargetService,
				"target_version": plan.TargetVersion,
				"operation":      plan.Operation,
				"timestamp":      plan.Timestamp,
			},
			"impact_report": map[string]interface{}{
				"package_found":         report.PackageFound,
				"package_status":        report.PackageStatus,
				"dependency_source":     report.DependencySource,
				"dependencies_required": report.DependenciesRequired,
				"dependencies_missing":  report.DependenciesMissing,
				"transitive_deps":       report.TransitiveDeps,
				"dependency_cycle":      report.DependencyCycle,
				"reverse_dependents":    report.ReverseDependents,
				"remediation_plan":      report.RemediationPlan,
				"services_affected":     report.ServicesAffected,
				"ports_required":        report.PortsRequired,
				"port_conflicts":        report.PortConflicts,
				"risk_level":            report.RiskLevel,
				"warnings":              report.Warnings,
			},
			"admission_decision": map[string]interface{}{
				"status":                decision.Status,
				"reasons":               decision.Reasons,
				"missing_requirements":  decision.MissingRequirements,
				"suggested_remediation": decision.SuggestedRemediation,
				"requires_approval":     decision.RequiresApproval,
			},
		}

		return out, nil
	})

	// ── globular_cli.execute_plan ───────────────────────────────────────
	s.register(toolDef{
		Name: "globular_cli.execute_plan",
		Description: `Executes a remediation plan step by step. Defaults to dry-run mode (simulation only).

For each step:
1. Validates the command
2. Checks approval (if required)
3. Executes or simulates the command
4. Verifies the target service was removed
5. Records the result

Stops immediately on any failure and returns partial results.

Use globular_cli.plan first to get the remediation plan, then pass it here.

Requires explicit dry_run=false and approved=true for real execution.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"command":  {Type: "string", Description: "The original command that produced the remediation plan (e.g. \"globular services desired remove persistence\")"},
				"dry_run":  {Type: "boolean", Description: "If true (default), simulate execution without making changes"},
				"approved": {Type: "boolean", Description: "Set to true to approve all steps that require confirmation"},
			},
			Required: []string{"command"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		command := getStr(args, "command")
		if command == "" {
			return nil, fmt.Errorf("command is required")
		}

		dryRun := getBool(args, "dry_run", true)
		approved := getBool(args, "approved", false)

		// First, compute the plan
		plan := NormalizePlan(command, nil)
		if plan.Operation != "remove" {
			return map[string]interface{}{
				"error": "execute_plan currently supports only remove operations",
			}, nil
		}

		// Get installed services to build the remediation plan
		planner := NewPlannerWithDescriptor(
			NewStaticDependencySource(),
			NewRepoDescriptorSource(s.clients),
			s.clients,
		)

		// Get installed list for reverse dep check
		installed := planner.getInstalledServices(ctx)
		if installed == nil {
			return map[string]interface{}{
				"error": "cannot query installed services — node agent unavailable",
			}, nil
		}

		// Build the remediation plan
		remPlan := BuildRemovalPlan(plan.TargetService, installed, planner.deps)
		if remPlan.Status == "blocked" {
			return map[string]interface{}{
				"status":   "blocked",
				"reason":   remPlan.Reason,
				"warnings": remPlan.Warnings,
			}, nil
		}

		// Execute
		executor := NewRemediationExecutor(s.clients, s.cfg.ReadOnly)
		workflow := executor.Execute(ctx, remPlan, dryRun, approved)

		// Format output
		stepResults := make([]map[string]interface{}, 0, len(workflow.StepResults))
		for _, sr := range workflow.StepResults {
			step := map[string]interface{}{
				"order":       sr.StepOrder,
				"action":      sr.Action,
				"target":      sr.Target,
				"success":     sr.Success,
				"duration_ms": sr.DurationMs,
			}
			if sr.Output != "" {
				step["output"] = sr.Output
			}
			if sr.Error != "" {
				step["error"] = sr.Error
			}
			if sr.DryRun {
				step["dry_run"] = true
			}
			stepResults = append(stepResults, step)
		}

		out := map[string]interface{}{
			"workflow_id":  workflow.ID,
			"status":      workflow.Status,
			"dry_run":     workflow.DryRun,
			"current_step": workflow.CurrentStep,
			"total_steps":  len(remPlan.OrderedSteps),
			"started_at":   workflow.StartedAt,
			"step_results": stepResults,
		}
		if workflow.CompletedAt != "" {
			out["completed_at"] = workflow.CompletedAt
		}
		if workflow.Error != "" {
			out["error"] = workflow.Error
		}

		return out, nil
	})
}
