// dev_cmds.go: Developer environment setup commands.
//
//	globular dev setup   — detect and configure Claude Code settings for this project

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/spf13/cobra"
)

var (
	devCmd = &cobra.Command{
		Use:   "dev",
		Short: "Developer environment tools",
	}

	devSetupYes bool

	devSetupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Configure Claude Code settings for Globular development",
		Long: `Detects Claude Code and proposes optimal permission settings
for Globular development. Creates .claude/settings.local.json with:

  - Auto mode (AI classifies safe vs risky commands)
  - Pre-approved: go build/test, git, make, systemctl, journalctl
  - Pre-approved: MCP tools, file reads, script execution
  - Risky commands still prompt (rm -rf, git push --force, etc.)

Run from the services/ repository root.`,
		RunE: runDevSetup,
	}
)

// claudeSettings represents the .claude/settings.local.json structure.
type claudeSettings struct {
	Permissions *claudePermissions        `json:"permissions,omitempty"`
	MCPServers  map[string]*mcpServerConf `json:"mcpServers,omitempty"`
}

type claudePermissions struct {
	DefaultMode string   `json:"defaultMode,omitempty"`
	Allow       []string `json:"allow,omitempty"`
	Deny        []string `json:"deny,omitempty"`
}

type mcpServerConf struct {
	URL     string   `json:"url,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// defaultMCPURL returns the MCP URL, resolving from etcd if available.
func defaultMCPURL() string {
	if addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", ""); addr != "" {
		return fmt.Sprintf("http://%s/mcp", addr)
	}
	return fmt.Sprintf("http://%s:10050/mcp", config.GetRoutableIPv4())
}

// proposedSettings returns the recommended Claude Code settings for Globular dev.
func proposedSettings() *claudeSettings {
	return &claudeSettings{
		Permissions: &claudePermissions{
			DefaultMode: "auto",
			Allow: []string{
				// Go toolchain
				"Bash(go:*)",
				"Bash(GEN_TS=0 bash:*)",
				"Bash(GLOBULAR_SKIP_EXTERNAL_TESTS=true go test:*)",

				// Git
				"Bash(git:*)",
				"Bash(gh:*)",

				// Build & codegen
				"Bash(make:*)",
				"Bash(bash:*)",
				"Bash(protoc:*)",
				"Bash(./generateCode.sh:*)",
				"Bash(./build-all-packages.sh:*)",
				"Bash(./build.sh:*)",

				// CLI tools
				"Bash(globular:*)",
				"Bash(globularcli:*)",
				"Bash(./globularcli:*)",
				"Bash(./globular:*)",

				// System inspection
				"Bash(systemctl:*)",
				"Bash(journalctl:*)",
				"Bash(sudo systemctl:*)",
				"Bash(sudo journalctl:*)",
				"Bash(ss:*)",
				"Bash(curl:*)",
				"Bash(dig:*)",
				"Bash(nslookup:*)",
				"Bash(openssl:*)",
				"Bash(grpcurl:*)",
				"Bash(etcdctl:*)",
				"Bash(ETCDCTL_API=3 etcdctl:*)",

				// File utilities
				"Bash(ls:*)",
				"Bash(cat:*)",
				"Bash(head:*)",
				"Bash(tail:*)",
				"Bash(tree:*)",
				"Bash(find:*)",
				"Bash(grep:*)",
				"Bash(wc:*)",
				"Bash(sort:*)",
				"Bash(jq:*)",
				"Bash(file:*)",
				"Bash(stat:*)",
				"Bash(tar:*)",
				"Bash(sha256sum:*)",
				"Bash(chmod:*)",
				"Bash(ln:*)",
				"Bash(echo:*)",
				"Bash(tee:*)",
				"Bash(xargs:*)",
				"Bash(xxd:*)",
				"Bash(du:*)",
				"Bash(env:*)",

				// Sudo read-only
				"Bash(sudo cat:*)",
				"Bash(sudo ls:*)",
				"Bash(sudo grep:*)",
				"Bash(sudo find:*)",
				"Bash(sudo jq:*)",
				"Bash(sudo test:*)",
				"Bash(sudo tail:*)",
				"Bash(sudo openssl:*)",

				// Node/TS
				"Bash(node:*)",
				"Bash(npm run:*)",
				"Bash(npx:*)",
				"Bash(pnpm:*)",
				"Bash(dotnet:*)",

				// MCP tools
				"mcp__globular__*",

				// IDE diagnostics
				"mcp__ide__getDiagnostics",

				// Broad file reads
				"Read(//var/lib/globular/**)",
				"Read(//usr/lib/globular/**)",
				"Read(//usr/local/bin/**)",
				"Read(//tmp/**)",
				"Read(//home/dave/**)",
			},
			Deny: []string{
				// Never auto-approve these
				"Bash(rm -rf /)",
				"Bash(git push --force:*)",
			},
		},
	}
}

func runDevSetup(cmd *cobra.Command, args []string) error {
	// Find project root (look for .claude/ directory or go.mod)
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("cannot find project root: %w", err)
	}

	settingsDir := filepath.Join(projectRoot, ".claude")
	settingsPath := filepath.Join(settingsDir, "settings.local.json")

	// Check if Claude Code is likely installed
	home, _ := os.UserHomeDir()
	claudeDir := filepath.Join(home, ".claude")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		fmt.Println("Claude Code does not appear to be installed (~/.claude not found).")
		fmt.Println("Install it from: https://claude.ai/code")
		return nil
	}

	fmt.Println("Globular Developer Setup — Claude Code Configuration")
	fmt.Println()

	// Check existing settings
	if data, err := os.ReadFile(settingsPath); err == nil {
		var existing claudeSettings
		if json.Unmarshal(data, &existing) == nil && existing.Permissions != nil {
			ruleCount := len(existing.Permissions.Allow)
			fmt.Printf("Existing settings found: %s (%d permission rules)\n", settingsPath, ruleCount)
			fmt.Printf("  Current mode: %s\n", existing.Permissions.DefaultMode)
			fmt.Println()

			if existing.Permissions.DefaultMode == "auto" && ruleCount < 150 {
				fmt.Println("Settings look good — no changes needed.")
				return nil
			}

			if ruleCount > 150 {
				fmt.Printf("You have %d accumulated rules — this can be simplified.\n", ruleCount)
			}
		}
	}

	// Show proposed settings
	proposed := proposedSettings()
	fmt.Println("Proposed settings:")
	fmt.Printf("  Mode: %s (AI classifies safe vs risky commands)\n", proposed.Permissions.DefaultMode)
	fmt.Printf("  Allow rules: %d (covers go, git, make, systemctl, etc.)\n", len(proposed.Permissions.Allow))
	fmt.Printf("  Deny rules: %d (blocks rm -rf /, force push)\n", len(proposed.Permissions.Deny))
	fmt.Println()
	fmt.Println("This means Claude Code can freely:")
	fmt.Println("  - Build and test Go code")
	fmt.Println("  - Read/edit project files")
	fmt.Println("  - Run git commands (except force push)")
	fmt.Println("  - Inspect systemd services and logs")
	fmt.Println("  - Use Globular MCP tools (cluster, RBAC, backup, node management)")
	fmt.Println()
	fmt.Println("It will still prompt for:")
	fmt.Println("  - Destructive operations (rm -rf, git reset --hard)")
	fmt.Println("  - Pushing to remote repositories")
	fmt.Println("  - Running unknown executables")
	fmt.Println()

	if !devSetupYes {
		fmt.Print("Apply these settings? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if !strings.EqualFold(strings.TrimSpace(answer), "y") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Write settings
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("create .claude/: %w", err)
	}

	data, err := json.MarshalIndent(proposed, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	fmt.Printf("\nWrote %s\n", settingsPath)

	// Ensure MCP server is configured in project-level settings.json
	projectSettingsPath := filepath.Join(settingsDir, "settings.json")
	mcpConfigured := ensureMCPConfig(projectSettingsPath)

	fmt.Println()
	if mcpConfigured {
		fmt.Println("Claude Code will use these settings on next session.")
	} else {
		fmt.Println("Permissions configured. Claude Code will use these on next session.")
	}

	// Check .gitignore
	gitignorePath := filepath.Join(settingsDir, ".gitignore")
	if data, err := os.ReadFile(gitignorePath); err == nil {
		if !strings.Contains(string(data), "settings.local.json") {
			fmt.Println("\nNote: Add 'settings.local.json' to .claude/.gitignore to keep personal settings out of git.")
		}
	}

	return nil
}

// ensureMCPConfig checks .claude/settings.json for the Globular MCP server
// config and adds it if missing. Returns true if MCP was configured (new or existing).
func ensureMCPConfig(projectSettingsPath string) bool {
	// Read existing project settings
	var projectSettings map[string]json.RawMessage
	if data, err := os.ReadFile(projectSettingsPath); err == nil {
		if err := json.Unmarshal(data, &projectSettings); err != nil {
			projectSettings = make(map[string]json.RawMessage)
		}
	} else {
		projectSettings = make(map[string]json.RawMessage)
	}

	// Check if MCP is already configured
	if raw, ok := projectSettings["mcpServers"]; ok {
		var servers map[string]json.RawMessage
		if json.Unmarshal(raw, &servers) == nil {
			if _, hasGlobular := servers["globular"]; hasGlobular {
				fmt.Println("MCP server: already configured (globular → " + defaultMCPURL() + ")")
				return true
			}
		}
	}

	// MCP not configured — add it
	fmt.Println("MCP server: adding Globular MCP server config")
	mcpServers := map[string]*mcpServerConf{
		"globular": {URL: defaultMCPURL()},
	}
	mcpData, _ := json.Marshal(mcpServers)
	projectSettings["mcpServers"] = mcpData

	// Write back
	data, err := json.MarshalIndent(projectSettings, "", "  ")
	if err != nil {
		fmt.Printf("  Warning: failed to marshal project settings: %v\n", err)
		return false
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(projectSettingsPath), 0755); err != nil {
		fmt.Printf("  Warning: failed to create directory: %v\n", err)
		return false
	}
	if err := os.WriteFile(projectSettingsPath, data, 0644); err != nil {
		fmt.Printf("  Warning: failed to write %s: %v\n", projectSettingsPath, err)
		return false
	}

	fmt.Printf("  Wrote MCP config to %s\n", projectSettingsPath)
	fmt.Println("  The Globular MCP server provides cluster health, RBAC, backup,")
	fmt.Println("  and node management tools directly in Claude Code.")
	fmt.Println()
	fmt.Println("  Make sure the MCP server is running:")
	fmt.Println("    systemctl status globular-mcp.service")
	return true
}

// findProjectRoot walks up from cwd looking for go.mod or .claude/
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".claude")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod or .claude/ found in any parent directory")
		}
		dir = parent
	}
}

func init() {
	devSetupCmd.Flags().BoolVarP(&devSetupYes, "yes", "y", false, "Apply without prompting")

	devCmd.AddCommand(devSetupCmd)
	rootCmd.AddCommand(devCmd)
}
