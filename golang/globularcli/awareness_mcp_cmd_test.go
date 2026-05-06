package main

import (
	"testing"
)

func TestAwarenessMCPServerCmdRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"awareness", "mcp-server"})
	if err != nil || cmd == nil || cmd.Name() != "mcp-server" {
		t.Fatalf("awareness mcp-server command not found: %v", err)
	}

	for _, flag := range []string{"db", "repo", "docs", "node-id"} {
		if f := cmd.Flags().Lookup(flag); f == nil {
			t.Errorf("awareness mcp-server missing required flag --%s", flag)
		}
	}

	if cmd.Short == "" {
		t.Error("mcp-server command has empty Short description")
	}
}
