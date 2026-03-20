package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

// claudeClient maintains a long-running Claude Code CLI subprocess.
// Prompts are sent via stdin (stream-json) and responses read from stdout,
// avoiding cold-start overhead on every incident. Claude Code runs under
// the user's subscription and has MCP tools wired up (memory, cluster
// health, node agent, RBAC, etc.) — no separate API key needed.
type claudeClient struct {
	cliBinary string

	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	running bool
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

// streamMessage is the JSON envelope sent to Claude CLI via stream-json input.
type streamMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

// streamResponse is one chunk from Claude CLI's stream-json output.
type streamResponse struct {
	Type    string `json:"type"`              // "assistant", "result", "error", etc.
	Content string `json:"content,omitempty"` // text content for assistant messages
	Result  string `json:"result,omitempty"`  // final result text
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message,omitempty"`
	Subtype string `json:"subtype,omitempty"`
}

// ensureRunning starts the Claude CLI subprocess if not already running.
func (c *claudeClient) ensureRunning() error {
	if c.running {
		// Check if process is still alive.
		if c.cmd != nil && c.cmd.Process != nil {
			if c.cmd.ProcessState != nil && c.cmd.ProcessState.Exited() {
				c.running = false
			} else {
				return nil
			}
		}
	}

	cmd := exec.Command(c.cliBinary,
		"--print",
		"--verbose",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--permission-mode", "bypassPermissions",
		"--no-session-persistence",
		"--model", "sonnet",
		"--system-prompt", "You are the AI operations engine for a Globular cluster. "+
			"You have MCP tools available to query cluster health, memory, node status, and RBAC. "+
			"Always respond with structured JSON analysis when asked to diagnose incidents. "+
			"Safety first — when uncertain, recommend observe_and_record.",
	)

	cmd.Env = os.Environ()

	// Set working directory to the services repo so Claude picks up
	// the MCP configuration and has access to all Globular MCP tools.
	workDir := os.Getenv("GLOBULAR_SERVICES_DIR")
	if workDir == "" {
		workDir = "/home/dave/Documents/github.com/globulario/services"
	}
	if _, err := os.Stat(workDir); err == nil {
		cmd.Dir = workDir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("create stdout pipe: %w", err)
	}

	// Capture stderr for diagnostics.
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("create stderr pipe: %w", err)
	}
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			logger.Warn("claude: stderr", "line", scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("start claude CLI: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.scanner = bufio.NewScanner(stdout)
	c.scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large responses
	c.running = true

	logger.Info("claude: subprocess started", "pid", cmd.Process.Pid)
	return nil
}

// sendPrompt sends a prompt to the running Claude subprocess and reads the response.
func (c *claudeClient) sendPrompt(ctx context.Context, prompt string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureRunning(); err != nil {
		return "", err
	}

	// Send the user message as stream-json.
	msg := streamMessage{
		Type:    "user",
		Content: prompt,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal prompt: %w", err)
	}
	msgBytes = append(msgBytes, '\n')

	if _, err := c.stdin.Write(msgBytes); err != nil {
		c.running = false
		return "", fmt.Errorf("write to claude stdin: %w", err)
	}

	// Read response lines until we get a result message.
	var responseText strings.Builder
	deadline := time.Now().Add(60 * time.Second)

	for c.scanner.Scan() {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting for claude response")
		}

		line := c.scanner.Text()
		if line == "" {
			continue
		}

		var resp streamResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue // Skip malformed lines.
		}

		switch resp.Type {
		case "assistant":
			// Accumulate assistant text chunks.
			if resp.Content != "" {
				responseText.WriteString(resp.Content)
			}
			for _, block := range resp.Message.Content {
				if block.Type == "text" {
					responseText.WriteString(block.Text)
				}
			}

		case "result":
			// Final result — use it if available, otherwise use accumulated text.
			if resp.Result != "" {
				return resp.Result, nil
			}
			result := responseText.String()
			if result != "" {
				return result, nil
			}
			return "", fmt.Errorf("empty result from claude")

		case "error":
			c.running = false
			return "", fmt.Errorf("claude error: %s", resp.Content)
		}
	}

	if err := c.scanner.Err(); err != nil {
		c.running = false
		return "", fmt.Errorf("read claude stdout: %w", err)
	}

	// Scanner ended — process exited.
	c.running = false
	result := responseText.String()
	if result != "" {
		return result, nil
	}
	return "", fmt.Errorf("claude process exited unexpectedly")
}

// shutdown cleanly stops the Claude subprocess.
func (c *claudeClient) shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			c.cmd.Process.Kill()
		}
	}
	c.running = false
	logger.Info("claude: subprocess stopped")
}

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
