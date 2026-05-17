package main

// awareness_semantic_cmds.go — stubs after semantic package was removed from
// standalone awareness module. The semantic commands (related, nearest, path,
// why-related, semantic-neighborhood) are not available in this build.
// Use the MCP tools 'awareness_related', 'awareness_nearest', 'awareness_path',
// 'awareness_why_related', 'awareness_semantic_neighborhood' instead.

import (
	"fmt"

	"github.com/spf13/cobra"
)

var semCfg = struct {
	from      string
	to        string
	node      string
	nodeType  string
	dimension string
	format    string
	maxItems  int
	maxDepth  int
	maxCost   float64
}{}

func makeSemanticStub(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short + " (not available — use MCP tool)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s is not available: semantic package removed — use corresponding MCP tool instead", use)
		},
	}
}

var awarenessRelatedCmd = makeSemanticStub("related", "Find nodes semantically related to a given node")
var awarenessNearestCmd = makeSemanticStub("nearest", "Find the nearest nodes of a specific type")
var awarenessPathCmd = makeSemanticStub("path", "Find the lowest-cost semantic path between two nodes")
var awarenessWhyRelatedCmd = makeSemanticStub("why-related", "Explain why two nodes are semantically related")
var awarenessSemanticNeighborhoodCmd = makeSemanticStub("semantic-neighborhood", "Show all semantically related nodes")

func init() {
	// Minimal flags to avoid "unknown flag" errors from scripts.
	for _, cmd := range []*cobra.Command{
		awarenessRelatedCmd, awarenessNearestCmd, awarenessPathCmd,
		awarenessWhyRelatedCmd, awarenessSemanticNeighborhoodCmd,
	} {
		cmd.Flags().StringVar(&semCfg.dimension, "dimension", "", "Semantic dimension")
		cmd.Flags().StringVar(&semCfg.format, "format", "markdown", "Output format")
		cmd.Flags().IntVar(&semCfg.maxDepth, "max-depth", 4, "Maximum traversal depth")
		cmd.Flags().Float64Var(&semCfg.maxCost, "max-cost", 20, "Maximum traversal cost")
	}
	awarenessRelatedCmd.Flags().StringVar(&semCfg.node, "node", "", "Node ID or name")
	awarenessRelatedCmd.Flags().IntVar(&semCfg.maxItems, "max-results", 10, "Maximum results")
	awarenessNearestCmd.Flags().StringVar(&semCfg.node, "node", "", "Node ID or name")
	awarenessNearestCmd.Flags().StringVar(&semCfg.nodeType, "type", "", "Target node type")
	awarenessNearestCmd.Flags().IntVar(&semCfg.maxItems, "max-results", 10, "Maximum results")
	awarenessPathCmd.Flags().StringVar(&semCfg.from, "from", "", "Source node ID or name")
	awarenessPathCmd.Flags().StringVar(&semCfg.to, "to", "", "Destination node ID or name")
	awarenessWhyRelatedCmd.Flags().StringVar(&semCfg.from, "from", "", "Source node ID or name")
	awarenessWhyRelatedCmd.Flags().StringVar(&semCfg.to, "to", "", "Destination node ID or name")
	awarenessSemanticNeighborhoodCmd.Flags().StringVar(&semCfg.node, "node", "", "Node ID or name")
	awarenessSemanticNeighborhoodCmd.Flags().IntVar(&semCfg.maxItems, "max-results", 20, "Maximum results")

	awarenessCmd.AddCommand(awarenessRelatedCmd)
	awarenessCmd.AddCommand(awarenessNearestCmd)
	awarenessCmd.AddCommand(awarenessPathCmd)
	awarenessCmd.AddCommand(awarenessWhyRelatedCmd)
	awarenessCmd.AddCommand(awarenessSemanticNeighborhoodCmd)
}
