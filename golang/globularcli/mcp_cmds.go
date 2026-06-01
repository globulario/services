// mcp_cmds.go: CLI commands for the Globular MCP service.
//
// Usage:
//
//	globular mcp tools                     List all tools exposed by the MCP service
//	globular mcp tools --group awareness   List only awareness.* tools
//	globular mcp tools --url <url>         Override MCP service URL
// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=mcp_management_commands
// @awareness risk=medium
package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
)

const defaultMCPListenPort = "10260"

var mcpCmdFlags = struct {
	url   string
	group string
}{}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP service commands",
	Long: `Commands for the Globular MCP service.

The Globular MCP service exposes cluster tools over the Model Context Protocol
(JSON-RPC 2.0 / HTTP). AI agents and Claude Code use it to query cluster state,
run awareness preflights, and coordinate remediation.

Examples:
  globular mcp tools                     # List all registered tools
  globular mcp tools --group awareness   # List awareness.* tools only
  globular mcp tools --group cluster     # List cluster.* tools only`,
}

var mcpToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List tools exposed by the Globular MCP service",
	Long: `List all tools registered in the Globular MCP service.

Connects to the running MCP service and fetches the tools/list via JSON-RPC 2.0.
Use --group to filter by tool name prefix (e.g. "awareness" lists awareness.* tools).

Examples:
  globular mcp tools
  globular mcp tools --group awareness
  globular mcp tools --group cluster
  globular mcp tools --url https://10.0.0.100:10260/mcp`,
	RunE: runMCPTools,
}

func init() {
	mcpToolsCmd.Flags().StringVar(&mcpCmdFlags.url, "url", "", "MCP service URL (default: resolved from etcd or https://globular.internal:10260/mcp)")
	mcpToolsCmd.Flags().StringVar(&mcpCmdFlags.group, "group", "", "Filter tools by group prefix (e.g. awareness, cluster, repository)")
	mcpCmd.AddCommand(mcpToolsCmd)
	rootCmd.AddCommand(mcpCmd)
}

// runMCPTools fetches tools/list from the MCP service and prints them.
func runMCPTools(cmd *cobra.Command, args []string) error {
	url := mcpCmdFlags.url
	if url == "" {
		url = resolveMCPURL()
	}
	if url == "" {
		return fmt.Errorf("MCP service not found — provide --url or ensure the MCP service is registered in etcd")
	}

	tools, err := fetchMCPToolsList(url)
	if err != nil {
		return fmt.Errorf("fetch tools/list from %s: %w", url, err)
	}

	// Filter by group prefix if requested.
	group := strings.TrimSpace(strings.ToLower(mcpCmdFlags.group))
	if group != "" {
		prefix := group + "."
		var filtered []mcpToolEntry
		for _, t := range tools {
			if strings.HasPrefix(t.Name, prefix) {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}

	if len(tools) == 0 {
		if group != "" {
			fmt.Fprintf(os.Stderr, "No tools found for group %q. Run 'globular mcp tools' to see all tools.\n", group)
		} else {
			fmt.Fprintln(os.Stderr, "No tools registered in the MCP service.")
		}
		return nil
	}

	// Sort for stable output.
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TOOL\tDESCRIPTION")
	for _, t := range tools {
		desc := t.Description
		// Truncate long descriptions for table display.
		const maxDesc = 80
		if len(desc) > maxDesc {
			desc = desc[:maxDesc-3] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\n", t.Name, desc)
	}
	_ = tw.Flush()

	fmt.Fprintf(os.Stderr, "\n%d tool(s)", len(tools))
	if group != "" {
		fmt.Fprintf(os.Stderr, " in group %q", group)
	}
	fmt.Fprintln(os.Stderr, ".")
	return nil
}

// resolveMCPURL resolves the MCP service URL from etcd, then falls back to
// the default address on the standard port.
func resolveMCPURL() string {
	addr := config.ResolveServiceAddr("mcp.McpService", "")
	if addr != "" {
		// Always use HTTPS — the MCP service has TLS enabled by default.
		if !strings.Contains(addr, "://") {
			addr = "https://" + addr
		}
		if !strings.HasSuffix(addr, "/mcp") {
			addr = strings.TrimRight(addr, "/") + "/mcp"
		}
		return addr
	}
	return fmt.Sprintf("https://globular.internal:%s/mcp", defaultMCPListenPort)
}

// mcpToolEntry is a subset of the MCP tool definition returned by tools/list.
type mcpToolEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// fetchMCPToolsList sends a JSON-RPC tools/list request to the MCP service
// and returns the list of tool definitions.
func fetchMCPToolsList(serviceURL string) ([]mcpToolEntry, error) {
	client := mcpHTTPClient()

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, serviceURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rpcResp struct {
		Result struct {
			Tools []mcpToolEntry `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result.Tools, nil
}

// mcpHTTPClient returns an HTTP client configured for the MCP service,
// respecting --insecure and --ca flags from the root command.
func mcpHTTPClient() *http.Client {
	tlsCfg := &tls.Config{}

	if rootCfg.insecure {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // user explicitly requested
	} else if rootCfg.caFile != "" {
		pem, err := os.ReadFile(rootCfg.caFile)
		if err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(pem) {
				tlsCfg.RootCAs = pool
			}
		}
	}

	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}
}
