package main

// awareness_node_context_cmds.go — stubs after the context package was removed from
// standalone awareness module. The node-context, neighborhood, and explain-node commands
// are not available in this build.
// Use the MCP tools 'awareness_node_context', 'awareness_neighborhood', 'awareness_explain_node' instead.

import (
	"fmt"

	"github.com/spf13/cobra"
)

var nodeCtxCfg = struct {
	node        string
	symbol      string
	file        string
	invariant   string
	failureMode string
	zoom        string
	format      string
	maxItems    int
	depth       int
}{}

var awarenessNodeContextCmd = &cobra.Command{
	Use:   "node-context",
	Short: "Show full architectural context for a graph node (not available — use MCP tool awareness_node_context)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("node-context is not available: context package removed — use MCP tool awareness_node_context instead")
	},
}

var awarenessNeighborhoodCmd = &cobra.Command{
	Use:   "neighborhood",
	Short: "Show the BFS neighborhood of a graph node (not available — use MCP tool awareness_neighborhood)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("neighborhood is not available: context package removed — use MCP tool awareness_neighborhood instead")
	},
}

var awarenessExplainNodeCmd = &cobra.Command{
	Use:   "explain-node",
	Short: "Explain a graph node's role (not available — use MCP tool awareness_explain_node)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("explain-node is not available: context package removed — use MCP tool awareness_explain_node instead")
	},
}

// resolveNodeRef picks the first non-empty flag value.
func resolveNodeRef() string {
	for _, v := range []string{
		nodeCtxCfg.node,
		nodeCtxCfg.symbol,
		nodeCtxCfg.file,
		nodeCtxCfg.invariant,
		nodeCtxCfg.failureMode,
	} {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.node, "node", "", "Node ID or name")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.symbol, "symbol", "", "Symbol name")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.file, "file", "", "File path")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.invariant, "invariant", "", "Invariant ID")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.failureMode, "failure-mode", "", "Failure mode ID")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.zoom, "zoom", "", "Semantic zoom level")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.format, "format", "markdown", "Output format: markdown, json, agent")
	awarenessNodeContextCmd.Flags().IntVar(&nodeCtxCfg.maxItems, "max-items", 20, "Max items per list")
	awarenessNodeContextCmd.Flags().IntVar(&nodeCtxCfg.depth, "depth", 2, "Traversal depth")

	awarenessNeighborhoodCmd.Flags().StringVar(&nodeCtxCfg.node, "node", "", "Node ID or name (required)")
	awarenessNeighborhoodCmd.Flags().IntVar(&nodeCtxCfg.depth, "depth", 1, "BFS depth (max 4)")
	awarenessNeighborhoodCmd.Flags().StringVar(&nodeCtxCfg.format, "format", "markdown", "Output format: markdown, json, agent")

	awarenessExplainNodeCmd.Flags().StringVar(&nodeCtxCfg.node, "node", "", "Node ID or name (required)")
	awarenessExplainNodeCmd.Flags().StringVar(&nodeCtxCfg.format, "format", "markdown", "Output format: markdown, json, agent")
	awarenessExplainNodeCmd.Flags().IntVar(&nodeCtxCfg.maxItems, "max-items", 20, "Max items per list")

	awarenessCmd.AddCommand(awarenessNodeContextCmd)
	awarenessCmd.AddCommand(awarenessNeighborhoodCmd)
	awarenessCmd.AddCommand(awarenessExplainNodeCmd)
}
