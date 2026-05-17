package main

// awareness_debug_session_cmd.go — stub after debugsession package was removed from
// standalone awareness module. The debug-session command is not available in this build.
// Use the MCP tool 'awareness_debug_session' instead.

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dbsCfg = struct {
	task           string
	files          []string
	packagePath    string
	phase          string
	format         string
	includeRuntime bool
	runtimeWindow  string
}{}

var awarenessDebugSessionCmd = &cobra.Command{
	Use:   "debug-session",
	Short: "Produce a guided debugging plan (not available — use MCP tool awareness_debug_session)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("debug-session is not available: debugsession package removed — use MCP tool awareness_debug_session instead")
	},
}

func init() {
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.task, "task", "", "Task description (required)")
	awarenessDebugSessionCmd.Flags().StringArrayVar(&dbsCfg.files, "file", nil, "File path to include in impact analysis (repeatable)")
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.packagePath, "package", "", "Path to package dir with awareness.yaml")
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.phase, "phase", "", "Dependency phase for cycle detection")
	awarenessDebugSessionCmd.Flags().BoolVar(&dbsCfg.includeRuntime, "include-runtime", false, "Include live runtime snapshot")
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.runtimeWindow, "runtime-window", "15m", "Lookback window for runtime evidence")
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.format, "format", "agent", "Output format: agent, markdown, json")

	awarenessCmd.AddCommand(awarenessDebugSessionCmd)
}
