package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

// claudeClient calls the Anthropic Messages API for incident reasoning.
// When the API key is unavailable, falls back to deterministic analysis.
type claudeClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

const (
	claudeAPIURL       = "https://api.anthropic.com/v1/messages"
	claudeDefaultModel = "claude-sonnet-4-20250514"
	claudeMaxTokens    = 1024
)

func newClaudeClient() *claudeClient {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		// Try reading from file (deployed clusters store keys in config).
		if data, err := os.ReadFile("/var/lib/globular/config/anthropic_api_key"); err == nil {
			key = strings.TrimSpace(string(data))
		}
	}

	model := os.Getenv("GLOBULAR_AI_MODEL")
	if model == "" {
		model = claudeDefaultModel
	}

	if key != "" {
		logger.Info("claude: API key configured", "model", model)
	} else {
		logger.Info("claude: no API key, using deterministic fallback")
	}

	return &claudeClient{
		apiKey:     key,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// isAvailable returns true if the Claude API is configured.
func (c *claudeClient) isAvailable() bool {
	return c.apiKey != ""
}

// analyzeIncident sends evidence to Claude and gets a reasoned diagnosis.
func (c *claudeClient) analyzeIncident(ctx context.Context, req *ai_executorpb.ProcessIncidentRequest, evidence []string, clusterHealth string) (*claudeAnalysis, error) {
	if !c.isAvailable() {
		return nil, fmt.Errorf("claude API not configured")
	}

	// Build the prompt with all gathered evidence.
	prompt := buildAnalysisPrompt(req, evidence, clusterHealth)

	response, err := c.callAPI(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("claude API call failed: %w", err)
	}

	// Parse Claude's structured response.
	analysis, err := parseAnalysis(response)
	if err != nil {
		// If parsing fails, use the raw text as the diagnosis.
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

	b.WriteString("You are the AI operations engine for a Globular cluster. ")
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

	b.WriteString("\nBe specific. Use the evidence. If confidence is low, recommend observe_and_record. ")
	b.WriteString("If risk is high, recommend notify_admin rather than auto-fix. ")
	b.WriteString("Safety first — when uncertain, observe.")

	return b.String()
}

// callAPI calls the Anthropic Messages API.
func (c *claudeClient) callAPI(ctx context.Context, prompt string) (string, error) {
	body := map[string]interface{}{
		"model":      c.model,
		"max_tokens": claudeMaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", claudeAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the Messages API response.
	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	for _, block := range apiResp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("no text content in response")
}

// parseAnalysis extracts structured analysis from Claude's response.
func parseAnalysis(response string) (*claudeAnalysis, error) {
	// Strip markdown code fences if present.
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

	// Validate.
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
