package main

import (
	"testing"
)

func TestAwarenessMCPServerCmdRegistered(t *testing.T) {
	// The standalone awareness MCP server was removed in v1.2.20.
	// The command is now a stub that redirects users to the main MCP service.
	cmd, _, err := rootCmd.Find([]string{"awareness", "mcp-server"})
	if err != nil || cmd == nil || cmd.Name() != "mcp-server" {
		t.Fatalf("awareness mcp-server command not found: %v", err)
	}
	if cmd.Short == "" {
		t.Error("mcp-server command has empty Short description")
	}
}
