// Package main implements a Globular MCP server that exposes read-only
// operator tools over stdio for AI assistants (Claude Code).
//
// Usage:
//
//	globular-mcp-server
//
// The server communicates via JSON-RPC 2.0 over stdin/stdout.
// All tools are strictly read-only / diagnostic / preview.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
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
			addr = "127.0.0.1:10050"
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
