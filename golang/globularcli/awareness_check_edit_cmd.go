package main

// awareness_check_edit_cmd.go — stub after checkedit package was removed from
// standalone awareness module. The check-edit command is not available in this build.
// Use the MCP tool 'awareness_pre_edit_context' or 'awareness_scan_violations' instead.

import (
	"fmt"

	"github.com/spf13/cobra"
)

var checkEditCfg = struct {
	file   string
	format string
}{}

var awarenessCheckEditCmd = &cobra.Command{
	Use:   "check-edit",
	Short: "Post-edit awareness check (not available — use MCP tool awareness_pre_edit_context)",
	Long: `check-edit is not available in this build.
The checkedit package has been removed from the standalone awareness module.
Use the MCP tool 'awareness_pre_edit_context' or 'awareness_scan_violations' instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("check-edit is not available: checkedit package removed — use MCP tool awareness_pre_edit_context or awareness_scan_violations instead")
	},
}

func init() {
	awarenessCheckEditCmd.Flags().StringVar(&checkEditCfg.file, "file", "", "Repo-relative path of the edited file")
	awarenessCheckEditCmd.Flags().StringVar(&checkEditCfg.format, "format", "agent", "Output format: agent | markdown | json")

	awarenessCmd.AddCommand(awarenessCheckEditCmd)
}
