package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	} else {
		logger.Info("claude: CLI not found, using deterministic fallback")
	}

	return &claudeClient{
		cliBinary: binary,
	}
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

	cmd := exec.CommandContext(ctx, c.cliBinary,
		"--print",
		"--output-format", "json",
		"--permission-mode", "bypassPermissions",
		"--no-session-persistence",
		"--model", "sonnet",
		"--system-prompt", "You are the AI operations engine for a Globular cluster. "+
			"You have MCP tools available to query cluster health, memory, node status, and RBAC. "+
			"Always respond with structured JSON analysis when asked to diagnose incidents. "+
			"Safety first — when uncertain, recommend observe_and_record.",
	)

	cmd.Env = os.Environ()

	// Set working directory so Claude picks up MCP configuration.
	workDir := os.Getenv("GLOBULAR_SERVICES_DIR")
	if workDir == "" {
		workDir = "/home/dave/Documents/github.com/globulario/services"
	}
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

	if err := cmd.Run(); err != nil {
		if se := stderr.String(); se != "" {
			logger.Warn("claude: stderr", "output", se)
		}
		return "", fmt.Errorf("claude CLI failed: %w", err)
	}

	// Parse the JSON result.
	var result cliResult
	if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
		// If JSON parsing fails, return raw stdout as the result.
		raw := strings.TrimSpace(stdout.String())
		if raw != "" {
			return raw, nil
		}
		return "", fmt.Errorf("parse claude output: %w", err)
	}

	if result.IsError {
		return "", fmt.Errorf("claude returned error: %s", result.Result)
	}

	return result.Result, nil
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
