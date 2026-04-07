package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

// claudeClient invokes the Claude Code CLI per-incident using --print mode.
// Each call spawns a fresh subprocess with the prompt on stdin and reads
// a single JSON result from stdout. Claude Code runs under the user's
// subscription and has MCP tools wired up (memory, cluster health, etc.).
type claudeClient struct {
	cliBinary string
	mu        sync.Mutex
}

func newClaudeClient() *claudeClient {
	binary := os.Getenv("CLAUDE_CLI_PATH")
	if binary == "" {
		for _, path := range []string{
			"/usr/local/bin/claude",
			"/usr/bin/claude",
			"/home/dave/.local/bin/claude",
			os.ExpandEnv("$HOME/.claude/bin/claude"),
		} {
			if _, err := os.Stat(path); err == nil {
				binary = path
				break
			}
		}
	}

	if binary != "" {
		logger.Info("claude: CLI configured", "binary", binary)
		// Ensure CLI credentials exist for the service user by syncing from etcd.
		syncCLICredentialsFromEtcd()
	} else {
		logger.Info("claude: CLI not found, using deterministic fallback")
	}

	return &claudeClient{
		cliBinary: binary,
	}
}

// syncCLICredentialsFromEtcd reads OAuth credentials from etcd and writes them
// to ~/.claude/.credentials.json so the Claude CLI can authenticate.
// The service runs as the "globular" user whose home is /var/lib/globular,
// so without this the CLI has no credentials.
func syncCLICredentialsFromEtcd() {
	val, err := etcdGet(etcdCredentialsKey)
	if err != nil || val == "" {
		return
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/var/lib/globular"
	}

	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		logger.Warn("claude: failed to create credentials dir", "path", dir, "err", err)
		return
	}

	credPath := filepath.Join(dir, ".credentials.json")
	if err := os.WriteFile(credPath, []byte(val), 0600); err != nil {
		logger.Warn("claude: failed to write credentials file", "path", credPath, "err", err)
		return
	}

	logger.Info("claude: synced credentials from etcd", "path", credPath)
}

// isAvailable returns true if the Claude CLI is found.
func (c *claudeClient) isAvailable() bool {
	return c.cliBinary != ""
}

// cliResult is the JSON object returned by `claude --print --output-format json`.
type cliResult struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
}

// sendPrompt invokes Claude CLI with the given prompt and returns the result text.
func (c *claudeClient) sendPrompt(ctx context.Context, prompt string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Resolve MCP config path for the globular user.
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/var/lib/globular"
	}
	mcpConfig := filepath.Join(home, ".claude", ".mcp.json")

	args := []string{
		"--print",
		"--output-format", "json",
		"--permission-mode", "bypassPermissions",
		"--no-session-persistence",
		"--model", "sonnet",
		"--system-prompt", "You are the AI operations engine for a Globular cluster. " +
			"You have MCP tools available to query cluster health, memory, node status, and RBAC. " +
			"Always respond with structured JSON analysis when asked to diagnose incidents. " +
			"Safety first — when uncertain, recommend observe_and_record.",
	}
	if _, err := os.Stat(mcpConfig); err == nil {
		args = append(args, "--mcp-config", mcpConfig)
	}
	cmd := exec.CommandContext(ctx, c.cliBinary, args...)

	cmd.Env = os.Environ()

	// Set working directory so Claude picks up MCP configuration.
	workDir := "/var/lib/globular/services"
	if _, err := os.Stat(workDir); err == nil {
		cmd.Dir = workDir
	}

	// Send prompt on stdin.
	cmd.Stdin = strings.NewReader(prompt)

	// Capture stdout and stderr.
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Info("claude: invoking CLI", "prompt_len", len(prompt))

	// Run and capture output. Claude CLI may exit 1 but still return valid
	// JSON with is_error=true (e.g. auth failures), so check stdout first.
	runErr := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if runErr != nil && out == "" {
		se := strings.TrimSpace(stderr.String())
		logger.Warn("claude: CLI failed with no output", "err", runErr, "stderr", se)
		return "", fmt.Errorf("claude CLI failed: %w (stderr: %s)", runErr, se)
	}

	var result cliResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		if out != "" {
			return out, nil
		}
		return "", fmt.Errorf("parse claude output: %w", err)
	}

	if result.IsError {
		return "", fmt.Errorf("claude returned error: %s", result.Result)
	}

	// After a successful CLI call, the CLI may have refreshed the OAuth token.
	// Re-read the credentials file and sync to etcd so other nodes get the fresh token.
	go syncRefreshedCredentialsToEtcd()

	return result.Result, nil
}

// syncRefreshedCredentialsToEtcd re-reads the local credentials file and
// pushes to etcd if the token has changed (e.g., after CLI auto-refresh).
func syncRefreshedCredentialsToEtcd() {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/var/lib/globular"
	}
	credPath := filepath.Join(home, ".claude", ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return
	}

	// Check if etcd already has the same content.
	existing, _ := etcdGet(etcdCredentialsKey)
	if existing == string(data) {
		return // No change
	}

	if err := etcdPut(etcdCredentialsKey, string(data)); err != nil {
		logger.Warn("claude: failed to sync refreshed credentials to etcd", "err", err)
	} else {
		logger.Info("claude: synced refreshed credentials to etcd")
	}
}

// shutdown is a no-op — each invocation is a fresh subprocess.
func (c *claudeClient) shutdown() {}

// analyzeIncident sends evidence to Claude and gets a reasoned diagnosis.
func (c *claudeClient) analyzeIncident(ctx context.Context, req *ai_executorpb.ProcessIncidentRequest, evidence []string, clusterHealth string) (*claudeAnalysis, error) {
	if !c.isAvailable() {
		return nil, fmt.Errorf("claude CLI not found")
	}

	prompt := buildAnalysisPrompt(req, evidence, clusterHealth)

	response, err := c.sendPrompt(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("claude analysis failed: %w", err)
	}

	analysis, err := parseAnalysis(response)
	if err != nil {
		return &claudeAnalysis{
			RootCause:      "analysis_available",
			Confidence:     0.5,
			Summary:        response,
			ProposedAction: "observe_and_record",
			Rationale:      "Claude provided analysis but structured parsing failed",
		}, nil
	}

	return analysis, nil
}

// claudeAnalysis is the structured output from Claude's reasoning.
type claudeAnalysis struct {
	RootCause      string  `json:"root_cause"`
	Confidence     float64 `json:"confidence"`
	Summary        string  `json:"summary"`
	Detail         string  `json:"detail"`
	ProposedAction string  `json:"proposed_action"`
	Rationale      string  `json:"rationale"`
	RiskLevel      string  `json:"risk_level"` // low, medium, high
}

func buildAnalysisPrompt(req *ai_executorpb.ProcessIncidentRequest, evidence []string, clusterHealth string) string {
	var b strings.Builder

	b.WriteString("Analyze this incident and provide a structured diagnosis.\n\n")

	b.WriteString("## Incident\n")
	fmt.Fprintf(&b, "- Rule: %s\n", req.GetRuleId())
	fmt.Fprintf(&b, "- Trigger event: %s\n", req.GetTriggerEventName())
	fmt.Fprintf(&b, "- Events in batch: %d\n", len(req.GetEventBatch()))
	fmt.Fprintf(&b, "- Tier: %d (0=observe, 1=auto-fix, 2=needs approval)\n\n", req.GetTier())

	// Include the trigger event payload so Claude knows which service/unit is affected.
	if len(req.GetTriggerEventData()) > 0 {
		b.WriteString("## Trigger Event Data\n")
		b.WriteString("```json\n")
		b.Write(req.GetTriggerEventData())
		b.WriteString("\n```\n\n")
	}

	if len(req.GetMetadata()) > 0 {
		b.WriteString("## Metadata\n")
		for k, v := range req.GetMetadata() {
			fmt.Fprintf(&b, "- %s: %s\n", k, v)
		}
		b.WriteString("\n")
	}

	if len(evidence) > 0 {
		b.WriteString("## Evidence Gathered\n")
		for _, e := range evidence {
			fmt.Fprintf(&b, "- %s\n", e)
		}
		b.WriteString("\n")
	}

	if clusterHealth != "" {
		b.WriteString("## Cluster Health\n")
		b.WriteString(clusterHealth)
		b.WriteString("\n\n")
	}

	b.WriteString("## Required Response Format\n")
	b.WriteString("Respond with ONLY a JSON object (no markdown, no explanation outside JSON):\n")
	b.WriteString("```json\n")
	b.WriteString(`{
  "root_cause": "brief identifier (e.g. brute_force_attack, service_crash, cascade_failure)",
  "confidence": 0.0-1.0,
  "summary": "one sentence diagnosis",
  "detail": "detailed analysis paragraph",
  "proposed_action": "action identifier (restart_service:name, block_ip:addr, drain_endpoint, tighten_circuit_breakers, observe_and_record, notify_admin)",
  "rationale": "why this action is appropriate",
  "risk_level": "low|medium|high"
}`)
	b.WriteString("\n```\n")

	b.WriteString("\nUse your MCP tools to query cluster health and memory for additional context. ")
	b.WriteString("Be specific. Use the evidence. If confidence is low, recommend observe_and_record. ")
	b.WriteString("If risk is high, recommend notify_admin rather than auto-fix. ")
	b.WriteString("Safety first — when uncertain, observe.")

	return b.String()
}

// parseAnalysis extracts structured analysis from Claude's response.
func parseAnalysis(response string) (*claudeAnalysis, error) {
	text := response
	if i := strings.Index(text, "{"); i >= 0 {
		text = text[i:]
	}
	if i := strings.LastIndex(text, "}"); i >= 0 {
		text = text[:i+1]
	}

	var analysis claudeAnalysis
	if err := json.Unmarshal([]byte(text), &analysis); err != nil {
		return nil, fmt.Errorf("parse analysis JSON: %w", err)
	}

	if analysis.Confidence < 0 {
		analysis.Confidence = 0
	}
	if analysis.Confidence > 1 {
		analysis.Confidence = 1
	}
	if analysis.RootCause == "" {
		analysis.RootCause = "unknown"
	}
	if analysis.ProposedAction == "" {
		analysis.ProposedAction = "observe_and_record"
	}

	return &analysis, nil
}
