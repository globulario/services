package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var awarenessMCPCmd = &cobra.Command{
	Use:        "mcp-server",
	Short:      "Start the awareness MCP server (stdin/stdout JSON-RPC 2.0) [REMOVED]",
	Deprecated: "The standalone awareness MCP server has been removed. Awareness tools are now part of the main Globular MCP service. Enable with tool_groups.awareness=true in /var/lib/globular/mcp/config.json.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The standalone awareness MCP server has been removed.")
		fmt.Println("Awareness tools are now part of the main Globular MCP service.")
		fmt.Println("Enable them with tool_groups.awareness=true in /var/lib/globular/mcp/config.json.")
		return nil
	},
}

func init() {
	awarenessCmd.AddCommand(awarenessMCPCmd)
}
