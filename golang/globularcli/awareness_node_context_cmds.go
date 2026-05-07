package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	awarectx "github.com/globulario/services/golang/awareness/context"
)

var nodeCtxCfg = struct {
	node        string
	symbol      string
	file        string
	invariant   string
	failureMode string
	format      string
	maxItems    int
	depth       int
}{}

var awarenessNodeContextCmd = &cobra.Command{
	Use:   "node-context",
	Short: "Show full architectural context for a graph node",
	Long: `Resolves a node reference and prints its full context: invariants, failure modes,
forbidden fixes, state reads/writes, required tests, edit warnings, and recommended searches.

Node reference may be: node ID, service name, symbol name, file path, invariant ID, or failure mode ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ref := resolveNodeRef()
		if ref == "" {
			return fmt.Errorf("one of --node, --symbol, --file, --invariant, --failure-mode is required")
		}

		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		r, err := awarectx.ResolveNode(ctx, g, ref)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", ref, err)
		}
		if r.Exact == nil {
			if len(r.Candidates) > 0 {
				fmt.Fprintf(os.Stdout, "Node %q not found exactly. Candidates:\n", ref)
				for _, c := range r.Candidates {
					fmt.Fprintf(os.Stdout, "  %s (%s) — %s\n", c.Name, c.Type, c.ID)
				}
			} else {
				fmt.Fprintf(os.Stdout, "Node %q not found. Run 'globular awareness build' first.\n", ref)
			}
			return nil
		}

		opts := awarectx.Options{MaxItems: nodeCtxCfg.maxItems, Depth: nodeCtxCfg.depth}
		nc, err := awarectx.Build(ctx, g, r.Exact.ID, opts)
		if err != nil {
			return fmt.Errorf("build context: %w", err)
		}

		fmt.Fprint(os.Stdout, awarectx.FormatNodeContext(nc, nodeCtxCfg.format))
		return nil
	},
}

var awarenessNeighborhoodCmd = &cobra.Command{
	Use:   "neighborhood",
	Short: "Show the BFS neighborhood of a graph node",
	Long: `Performs BFS from a node up to a given depth and prints all reachable nodes
partitioned by type (services, symbols, files, invariants, failure modes, tests).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if nodeCtxCfg.node == "" {
			return fmt.Errorf("--node is required")
		}

		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		r, err := awarectx.ResolveNode(ctx, g, nodeCtxCfg.node)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", nodeCtxCfg.node, err)
		}
		if r.Exact == nil {
			fmt.Fprintf(os.Stdout, "Node %q not found.\n", nodeCtxCfg.node)
			return nil
		}

		depth := nodeCtxCfg.depth
		if depth <= 0 {
			depth = 1
		}
		if depth > 4 {
			depth = 4
		}

		nr, err := awarectx.Neighborhood(ctx, g, r.Exact.ID, depth)
		if err != nil {
			return fmt.Errorf("neighborhood: %w", err)
		}

		fmt.Fprint(os.Stdout, awarectx.FormatNeighborhood(nr, nodeCtxCfg.format))
		return nil
	},
}

var awarenessExplainNodeCmd = &cobra.Command{
	Use:   "explain-node",
	Short: "Explain a graph node's role, risks, and edit warnings in natural language",
	RunE: func(cmd *cobra.Command, args []string) error {
		if nodeCtxCfg.node == "" {
			return fmt.Errorf("--node is required")
		}

		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		r, err := awarectx.ResolveNode(ctx, g, nodeCtxCfg.node)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", nodeCtxCfg.node, err)
		}
		if r.Exact == nil {
			fmt.Fprintf(os.Stdout, "Node %q not found.\n", nodeCtxCfg.node)
			return nil
		}

		opts := awarectx.Options{MaxItems: nodeCtxCfg.maxItems}
		ex, err := awarectx.ExplainNode(ctx, g, r.Exact.ID, opts)
		if err != nil {
			return fmt.Errorf("explain: %w", err)
		}

		fmt.Fprint(os.Stdout, awarectx.FormatExplanation(ex, nodeCtxCfg.format))
		return nil
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
	// node-context flags.
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.node, "node", "", "Node ID or name")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.symbol, "symbol", "", "Symbol name")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.file, "file", "", "File path")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.invariant, "invariant", "", "Invariant ID")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.failureMode, "failure-mode", "", "Failure mode ID")
	awarenessNodeContextCmd.Flags().StringVar(&nodeCtxCfg.format, "format", "markdown", "Output format: markdown, json, agent")
	awarenessNodeContextCmd.Flags().IntVar(&nodeCtxCfg.maxItems, "max-items", 20, "Max items per list")
	awarenessNodeContextCmd.Flags().IntVar(&nodeCtxCfg.depth, "depth", 2, "Traversal depth")
	awarenessNodeContextCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessNodeContextCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// neighborhood flags.
	awarenessNeighborhoodCmd.Flags().StringVar(&nodeCtxCfg.node, "node", "", "Node ID or name (required)")
	awarenessNeighborhoodCmd.Flags().IntVar(&nodeCtxCfg.depth, "depth", 1, "BFS depth (max 4)")
	awarenessNeighborhoodCmd.Flags().StringVar(&nodeCtxCfg.format, "format", "markdown", "Output format: markdown, json, agent")
	awarenessNeighborhoodCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessNeighborhoodCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// explain-node flags.
	awarenessExplainNodeCmd.Flags().StringVar(&nodeCtxCfg.node, "node", "", "Node ID or name (required)")
	awarenessExplainNodeCmd.Flags().StringVar(&nodeCtxCfg.format, "format", "markdown", "Output format: markdown, json, agent")
	awarenessExplainNodeCmd.Flags().IntVar(&nodeCtxCfg.maxItems, "max-items", 20, "Max items per list")
	awarenessExplainNodeCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessExplainNodeCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	awarenessCmd.AddCommand(awarenessNodeContextCmd)
	awarenessCmd.AddCommand(awarenessNeighborhoodCmd)
	awarenessCmd.AddCommand(awarenessExplainNodeCmd)
}
