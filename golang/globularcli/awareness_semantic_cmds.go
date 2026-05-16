package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/globulario/awareness/semantic"
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

// ── related ──────────────────────────────────────────────────────────────────

var awarenessRelatedCmd = &cobra.Command{
	Use:   "related",
	Short: "Find nodes semantically related to a given node, ranked by distance",
	Long: `Runs a weighted Dijkstra traversal from the given node and returns all
reachable nodes ranked by semantic distance in the requested dimension.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if semCfg.node == "" {
			return fmt.Errorf("--node is required")
		}
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		opts := semantic.RelatedOptions{
			Dimension:  semCfg.dimension,
			MaxResults: semCfg.maxItems,
			MaxDepth:   semCfg.maxDepth,
			MaxCost:    semCfg.maxCost,
		}
		results, err := semantic.Related(ctx, g, semCfg.node, opts)
		if err != nil {
			return fmt.Errorf("related: %w", err)
		}

		dim := semCfg.dimension
		if dim == "" {
			dim = semantic.DimensionAll
		}
		fmt.Fprint(os.Stdout, semantic.FormatRelated(results, semCfg.node, dim, semCfg.format))
		return nil
	},
}

// ── nearest ───────────────────────────────────────────────────────────────────

var awarenessNearestCmd = &cobra.Command{
	Use:   "nearest",
	Short: "Find the nearest nodes of a specific type to a given node",
	RunE: func(cmd *cobra.Command, args []string) error {
		if semCfg.node == "" {
			return fmt.Errorf("--node is required")
		}
		if semCfg.nodeType == "" {
			return fmt.Errorf("--type is required")
		}
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		opts := semantic.RelatedOptions{
			Dimension:  semCfg.dimension,
			MaxResults: semCfg.maxItems,
			MaxDepth:   semCfg.maxDepth,
			MaxCost:    semCfg.maxCost,
		}
		results, err := semantic.Nearest(ctx, g, semCfg.node, semCfg.nodeType, opts)
		if err != nil {
			return fmt.Errorf("nearest: %w", err)
		}

		dim := semCfg.dimension
		if dim == "" {
			dim = semantic.DimensionAll
		}
		fmt.Fprint(os.Stdout, semantic.FormatRelated(results, semCfg.node, dim, semCfg.format))
		return nil
	},
}

// ── path ──────────────────────────────────────────────────────────────────────

var awarenessPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Find the lowest-cost semantic path between two nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		if semCfg.from == "" || semCfg.to == "" {
			return fmt.Errorf("--from and --to are required")
		}
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		opts := semantic.PathOptions{
			Dimension: semCfg.dimension,
			MaxDepth:  semCfg.maxDepth,
			MaxCost:   semCfg.maxCost,
		}
		p, err := semantic.ShortestPath(ctx, g, semCfg.from, semCfg.to, opts)
		if err != nil {
			return fmt.Errorf("path: %w", err)
		}

		fmt.Fprint(os.Stdout, semantic.FormatPath(p, semCfg.format))
		return nil
	},
}

// ── why-related ───────────────────────────────────────────────────────────────

var awarenessWhyRelatedCmd = &cobra.Command{
	Use:   "why-related",
	Short: "Explain why two nodes are semantically related, with edit warnings and risks",
	RunE: func(cmd *cobra.Command, args []string) error {
		if semCfg.from == "" || semCfg.to == "" {
			return fmt.Errorf("--from and --to are required")
		}
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		opts := semantic.WhyOptions{
			Dimension: semCfg.dimension,
			MaxDepth:  semCfg.maxDepth,
			MaxCost:   semCfg.maxCost,
		}
		r, err := semantic.WhyRelated(ctx, g, semCfg.from, semCfg.to, opts)
		if err != nil {
			return fmt.Errorf("why-related: %w", err)
		}

		fmt.Fprint(os.Stdout, semantic.FormatWhy(r, semCfg.format))
		return nil
	},
}

// ── semantic-neighborhood ─────────────────────────────────────────────────────

var awarenessSemanticNeighborhoodCmd = &cobra.Command{
	Use:   "semantic-neighborhood",
	Short: "Show all semantically related nodes ranked by distance, across all types",
	RunE: func(cmd *cobra.Command, args []string) error {
		if semCfg.node == "" {
			return fmt.Errorf("--node is required")
		}
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		opts := semantic.RelatedOptions{
			Dimension:  semCfg.dimension,
			MaxResults: semCfg.maxItems,
			MaxDepth:   semCfg.maxDepth,
			MaxCost:    semCfg.maxCost,
		}
		results, err := semantic.SemanticNeighborhood(ctx, g, semCfg.node, opts)
		if err != nil {
			return fmt.Errorf("semantic-neighborhood: %w", err)
		}

		dim := semCfg.dimension
		if dim == "" {
			dim = semantic.DimensionAll
		}
		fmt.Fprint(os.Stdout, semantic.FormatSemanticNeighborhood(results, semCfg.node, dim, semCfg.format))
		return nil
	},
}

func init() {
	dims := "code, module, service, package, state, workflow, architecture, runtime, history, test, all"

	// shared flags helper
	addSharedSemFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&semCfg.dimension, "dimension", "", "Semantic dimension: "+dims)
		cmd.Flags().StringVar(&semCfg.format, "format", "markdown", "Output format: markdown, json, agent")
		cmd.Flags().IntVar(&semCfg.maxDepth, "max-depth", 4, "Maximum traversal depth")
		cmd.Flags().Float64Var(&semCfg.maxCost, "max-cost", 20, "Maximum traversal cost")
		cmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
		cmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	}

	// related
	awarenessRelatedCmd.Flags().StringVar(&semCfg.node, "node", "", "Node ID or name (required)")
	awarenessRelatedCmd.Flags().IntVar(&semCfg.maxItems, "max-results", 10, "Maximum results to return")
	addSharedSemFlags(awarenessRelatedCmd)

	// nearest
	awarenessNearestCmd.Flags().StringVar(&semCfg.node, "node", "", "Node ID or name (required)")
	awarenessNearestCmd.Flags().StringVar(&semCfg.nodeType, "type", "", "Target node type (required)")
	awarenessNearestCmd.Flags().IntVar(&semCfg.maxItems, "max-results", 10, "Maximum results to return")
	addSharedSemFlags(awarenessNearestCmd)

	// path
	awarenessPathCmd.Flags().StringVar(&semCfg.from, "from", "", "Source node ID or name (required)")
	awarenessPathCmd.Flags().StringVar(&semCfg.to, "to", "", "Destination node ID or name (required)")
	addSharedSemFlags(awarenessPathCmd)

	// why-related
	awarenessWhyRelatedCmd.Flags().StringVar(&semCfg.from, "from", "", "Source node ID or name (required)")
	awarenessWhyRelatedCmd.Flags().StringVar(&semCfg.to, "to", "", "Destination node ID or name (required)")
	addSharedSemFlags(awarenessWhyRelatedCmd)

	// semantic-neighborhood
	awarenessSemanticNeighborhoodCmd.Flags().StringVar(&semCfg.node, "node", "", "Node ID or name (required)")
	awarenessSemanticNeighborhoodCmd.Flags().IntVar(&semCfg.maxItems, "max-results", 20, "Maximum results to return")
	addSharedSemFlags(awarenessSemanticNeighborhoodCmd)

	awarenessCmd.AddCommand(awarenessRelatedCmd)
	awarenessCmd.AddCommand(awarenessNearestCmd)
	awarenessCmd.AddCommand(awarenessPathCmd)
	awarenessCmd.AddCommand(awarenessWhyRelatedCmd)
	awarenessCmd.AddCommand(awarenessSemanticNeighborhoodCmd)
}
