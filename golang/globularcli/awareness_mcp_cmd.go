package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	mcpaware "github.com/globulario/services/golang/awareness/mcp"
)

var awarenessMCPCfg = struct {
	nodeID  string
	docsDir string
}{}

var awarenessMCPCmd = &cobra.Command{
	Use:        "mcp-server",
	Short:      "Start the awareness MCP server (stdin/stdout JSON-RPC 2.0) [DEPRECATED]",
	Deprecated: "Use the main Globular MCP service instead (tool_groups.awareness=true in /var/lib/globular/mcp/config.json). Run 'globular mcp tools --group awareness' to list awareness tools.",
	Long: `[DEPRECATED] Starts a standalone MCP server that exposes all 12 awareness graph tools.

The awareness tools are now part of the main Globular MCP service (golang/mcp).
Enable them with tool_groups.awareness=true in /var/lib/globular/mcp/config.json.
Use 'globular mcp tools --group awareness' to list all available awareness tools.

This standalone server remains for local development only and will be removed
in a future release. For production, use the main MCP service.

---

Starts a standalone MCP server that exposes all 12 awareness graph tools.

The server speaks the Model Context Protocol over stdin/stdout with
Content-Length framing (same as Language Server Protocol).

Claude Code or any MCP-compatible client can call these tools before
editing Globular code:

  awareness.preflight         Full architecture preflight (primary entry point)
  awareness.agent_context     Invariants + forbidden fixes for a task
  awareness.impact_file       Graph impact for a specific file
  awareness.did_we_fix        Fix-ledger lookup
  awareness.pattern_status    All fix cases matching a pattern
  awareness.fix_status        Fix case by ID or pattern
  awareness.runtime_snapshot  Read-only live cluster snapshot
  awareness.validate_package  Package admission check
  awareness.package_context   Package architectural context
  awareness.propose_from_incident  Generate draft proposal (DRAFT status only)
  awareness.validate_proposal     Validate a proposal (12 rules, optional --strict)
  awareness.approve_proposal      Approve a validated proposal

promote-proposal is intentionally NOT exposed over MCP.
Promotion remains a CLI-only operation.

Add to your MCP client configuration:

  {
    "command": "globular",
    "args": ["awareness", "mcp-server"],
    "name": "globular-awareness"
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		dbPath := awareCfg.dbPath
		if dbPath == "" {
			dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
		}

		docsDir := awarenessMCPCfg.docsDir
		if docsDir == "" {
			docsDir = filepath.Join(repoRoot, "docs", "awareness")
		}

		cfg := mcpaware.Config{
			DBPath:   dbPath,
			RepoPath: repoRoot,
			DocsDir:  docsDir,
			NodeID:   awarenessMCPCfg.nodeID,
		}

		s := mcpaware.New(cfg)
		defer s.Close()

		fmt.Fprintf(os.Stderr, "globular-awareness-mcp: ready (%d tools)\n", len(s.ToolNames()))

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		return s.ServeStdio(ctx)
	},
}

func init() {
	awarenessMCPCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (default: .globular/awareness/graph.db)")
	awarenessMCPCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")
	awarenessMCPCmd.Flags().StringVar(&awarenessMCPCfg.docsDir, "docs", "", "Path to docs/awareness directory (default: <repo>/docs/awareness)")
	awarenessMCPCmd.Flags().StringVar(&awarenessMCPCfg.nodeID, "node-id", "", "Optional local node ID for runtime bridge labelling")
	awarenessCmd.AddCommand(awarenessMCPCmd)
}
