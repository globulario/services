package main

// awareness_context_cmds.go — stubs after contextfreshness package was removed from
// standalone awareness module. The stale-context detection commands are not available
// in this build. Use the MCP tools 'awareness_check_stale_context',
// 'awareness_record_context_read', 'awareness_check_session_freshness' instead.

import (
	"fmt"

	"github.com/spf13/cobra"
)

var contextFreshCfg = struct {
	sessionID string
	filePath  string
	reason    string
	tool      string
	turnIndex int
	all       bool
	output    string
}{output: "table"}

var awarenessContextReadCmd = &cobra.Command{
	Use:   "context-read",
	Short: "Record a context read for stale-context detection (not available — use MCP tool)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("context-read is not available: contextfreshness package removed — use MCP tool awareness_record_context_read instead")
	},
}

var awarenessContextCheckCmd = &cobra.Command{
	Use:   "context-check",
	Short: "Check whether context files are stale (not available — use MCP tool)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("context-check is not available: contextfreshness package removed — use MCP tool awareness_check_stale_context instead")
	},
}

var awarenessContextStaleCmd = &cobra.Command{
	Use:   "context-stale",
	Short: "Show all stale context files for a session (not available — use MCP tool)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("context-stale is not available: contextfreshness package removed — use MCP tool awareness_check_session_freshness instead")
	},
}

func init() {
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.sessionID, "session", "", "Session ID (required)")
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.filePath, "file", "", "File path (required)")
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.reason, "reason", "", "Reason for reading")
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.tool, "tool", "", "Tool name")
	awarenessContextReadCmd.Flags().IntVar(&contextFreshCfg.turnIndex, "turn", 0, "Turn index")

	awarenessContextCheckCmd.Flags().StringVar(&contextFreshCfg.sessionID, "session", "", "Session ID (required)")
	awarenessContextCheckCmd.Flags().StringVar(&contextFreshCfg.filePath, "file", "", "File path")
	awarenessContextCheckCmd.Flags().BoolVar(&contextFreshCfg.all, "all", false, "Check all files in the session")
	awarenessContextCheckCmd.Flags().IntVar(&contextFreshCfg.turnIndex, "turn", 0, "Turn index")
	awarenessContextCheckCmd.Flags().StringVar(&contextFreshCfg.output, "output", "table", "Output format: table, json")

	awarenessContextStaleCmd.Flags().StringVar(&contextFreshCfg.sessionID, "session", "", "Session ID (required)")
	awarenessContextStaleCmd.Flags().IntVar(&contextFreshCfg.turnIndex, "turn", 0, "Turn index")
	awarenessContextStaleCmd.Flags().StringVar(&contextFreshCfg.output, "output", "table", "Output format: table, json")

	awarenessCmd.AddCommand(awarenessContextReadCmd)
	awarenessCmd.AddCommand(awarenessContextCheckCmd)
	awarenessCmd.AddCommand(awarenessContextStaleCmd)
}
