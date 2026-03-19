package main

import (
	"context"
	"fmt"
)

func registerCLITools(s *server) {

	// ── globular_cli.help ────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "globular_cli.help",
		Description: "Returns structured help for a Globular CLI command including description, flags, allowed values, examples, rules, and follow-up commands. Use this before running any CLI command to ensure correct usage.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"command_path": {Type: "string", Description: "The command path to look up (e.g. \"generate service\", \"pkg build\", \"cluster bootstrap\")"},
				"format":       {Type: "string", Description: "Output format", Enum: []string{"json", "text"}, Default: "json"},
			},
			Required: []string{"command_path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		path := getStr(args, "command_path")
		if path == "" {
			return nil, fmt.Errorf("command_path is required")
		}

		cmd, ok := lookupCommand(path)
		if !ok {
			// Return available commands as a hint
			available := make([]string, 0, len(cliCommands))
			for k := range cliCommands {
				available = append(available, k)
			}
			return map[string]interface{}{
				"error":              fmt.Sprintf("unknown command: %s", path),
				"available_commands": available,
			}, nil
		}

		return map[string]interface{}{
			"command":     cmd.Path,
			"description": cmd.Description,
			"long":        cmd.Long,
			"flags":       cmd.Flags,
			"examples":    cmd.Examples,
			"rules":       cmd.Rules,
			"follow_up":   cmd.FollowUp,
		}, nil
	})

	// ── globular_cli.workflow ────────────────────────────────────────────
	s.register(toolDef{
		Name:        "globular_cli.workflow",
		Description: "Returns a step-by-step workflow for a task (e.g. create_service, publish_package, bootstrap_cluster). Each step includes the action type, command, description, and expected output.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {Type: "string", Description: "The task to get workflow for (e.g. \"create_service\", \"publish_package\", \"bootstrap_cluster\")"},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task := getStr(args, "task")
		if task == "" {
			return nil, fmt.Errorf("task is required")
		}

		wf, ok := lookupWorkflow(task)
		if !ok {
			available := make([]string, 0, len(cliWorkflows))
			for k := range cliWorkflows {
				available = append(available, k)
			}
			return map[string]interface{}{
				"error":           fmt.Sprintf("unknown task: %s", task),
				"available_tasks": available,
			}, nil
		}

		return map[string]interface{}{
			"task":        wf.Task,
			"description": wf.Description,
			"steps":       wf.Steps,
		}, nil
	})

	// ── globular_cli.rules ──────────────────────────────────────────────
	s.register(toolDef{
		Name:        "globular_cli.rules",
		Description: "Returns core AI rules for operating the Globular CLI. Rules include: proto is source of truth, use CLI for generation, don't edit generated files, propose before executing, and more.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"scope": {Type: "string", Description: "Optional filter by scope (e.g. \"generation\", \"safety\", \"usage\", \"dns\", \"csharp\")"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		scope := getStr(args, "scope")
		rules := filterRules(scope)
		return map[string]interface{}{
			"rules": rules,
			"count": len(rules),
		}, nil
	})

	// ── globular_cli.examples ───────────────────────────────────────────
	s.register(toolDef{
		Name:        "globular_cli.examples",
		Description: "Returns validated CLI examples with descriptions. Filter by command category (generate, pkg, cluster, services, dns).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"command": {Type: "string", Description: "Optional category filter (e.g. \"generate\", \"pkg\", \"cluster\", \"services\", \"dns\")"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		category := getStr(args, "command")
		examples := filterExamples(category)
		return map[string]interface{}{
			"examples": examples,
			"count":    len(examples),
		}, nil
	})
}
