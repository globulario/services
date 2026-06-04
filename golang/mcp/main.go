// @awareness namespace=globular.platform
// @awareness component=platform_mcp.main
// @awareness file_role=mcp_server_entrypoint_dispatching_stdio_or_http_transport_per_mcpconfig
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness risk=high
//
// rebuild-marker: v1.2.156 — force CI change-detection to re-pack the mcp
// package so the corrected scripts/post-install.sh (mode 0o755, packages
// repo commit 54b0195) ships in the release tarball. CI's detect-changes.py
// hashes golang/<go_target>/*.go + packages/metadata/<name>/{package.json,
// specs/, systemd/} but NOT packages/metadata/<name>/scripts/, so the
// mode-only fix in mcp's post-install.sh was invisible to change detection
// and v1.2.154 shipped the old broken tarball unchanged. Once CI's
// change detector is taught to consider scripts/ content this comment can
// be removed.
//
// Package main implements a Globular MCP server that exposes read-only
// operator tools over stdio for AI assistants (Claude Code).
//
// Usage:
//
//	globular-mcp-server
//
// The server communicates via JSON-RPC 2.0 over stdin/stdout (or HTTP
// per cfg.Transport). All tools are strictly read-only / diagnostic /
// preview by default; mutating tool groups (etcd put/delete, package
// build/publish, governor execute) are enabled by explicit
// MCPConfig.ToolGroups flags only.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/globulario/services/golang/config"
)

func main() {
	// Suppress log output to stderr (MCP uses stdout for JSON-RPC).
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()
	if !cfg.Enabled {
		log.Println("globular-mcp-server: disabled by config")
		return
	}

	initAuditLog(cfg)

	srv := newServer(cfg)
	registerAllTools(srv)

	toolCount := len(srv.tools)
	log.Printf("globular-mcp-server: starting (tools=%d, read_only=%v, transport=%s)",
		toolCount, cfg.ReadOnly, cfg.Transport)

	switch cfg.Transport {
	case "http":
		addr := cfg.HTTPListenAddr
		if addr == "" {
			// Port must come from etcd — no hardcoded default.
			sc, err := config.GetServiceConfigurationById("mcp.McpService")
			if err != nil {
				log.Fatalf("mcp: cannot determine listen port: service not configured in etcd: %v", err)
			}
			p, ok := sc["Port"].(float64)
			if !ok || p <= 0 {
				log.Fatalf("mcp: etcd config for mcp.McpService has no valid Port field")
			}
			addr = fmt.Sprintf("0.0.0.0:%d", int(p))
		}
		if err := srv.serveHTTP(ctx, addr); err != nil {
			log.Fatalf("globular-mcp-server: %v", err)
		}
	default: // "stdio"
		if err := srv.serveStdio(ctx); err != nil {
			log.Fatalf("globular-mcp-server: %v", err)
		}
	}
}
