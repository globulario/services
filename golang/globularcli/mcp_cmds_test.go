package main

import (
	"testing"
)

func TestMCPCmdRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"mcp"})
	if err != nil || cmd == nil || cmd.Name() != "mcp" {
		t.Fatalf("'globular mcp' command not found: %v", err)
	}
}

func TestMCPToolsCmdRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"mcp", "tools"})
	if err != nil || cmd == nil || cmd.Name() != "tools" {
		t.Fatalf("'globular mcp tools' command not found: %v", err)
	}

	for _, flag := range []string{"url", "group"} {
		if f := cmd.Flags().Lookup(flag); f == nil {
			t.Errorf("'globular mcp tools' missing required flag --%s", flag)
		}
	}

	if cmd.Short == "" {
		t.Error("'globular mcp tools' has empty Short description")
	}
}

func TestMCPToolsCmdGroupFlag(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"mcp", "tools"})
	if err != nil || cmd == nil {
		t.Fatalf("'globular mcp tools' command not found: %v", err)
	}

	f := cmd.Flags().Lookup("group")
	if f == nil {
		t.Fatal("missing --group flag")
	}
	if f.Value.String() != "" {
		t.Errorf("--group default should be empty, got %q", f.Value.String())
	}
}

func TestResolveMCPURL_FallsBackToDefault(t *testing.T) {
	// resolveMCPURL must always return a non-empty fallback.
	// When etcd is unavailable (CI), it returns the default address.
	url := resolveMCPURL()
	if url == "" {
		t.Fatal("resolveMCPURL returned empty string — must return a non-empty fallback")
	}
	// Must contain a host and the /mcp path.
	if url == "" || url == "/mcp" {
		t.Errorf("resolveMCPURL returned malformed URL: %q", url)
	}
}

func TestAwarenessMCPCmdIsDeprecated(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"awareness", "mcp-server"})
	if err != nil || cmd == nil {
		t.Fatalf("awareness mcp-server command not found: %v", err)
	}
	// Cobra sets Deprecated to a non-empty string for deprecated commands.
	if cmd.Deprecated == "" {
		t.Error("awareness mcp-server should have a non-empty Deprecated message")
	}
}
