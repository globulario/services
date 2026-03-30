package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
)

// AnthropicConfig holds the configuration for direct Anthropic API access.
type AnthropicConfig struct {
	APIKey          string `json:"ApiKey"`          // Anthropic API key (sk-ant-api...) — standalone billing
	CredentialsFile string `json:"CredentialsFile"` // Path to Claude Code credentials (uses Max subscription)
	Model           string `json:"Model"`           // Model ID (default: claude-sonnet-4-20250514)
	MaxTokens       int    `json:"MaxTokens"`       // Max response tokens (default: 4096)
	BaseURL         string `json:"BaseURL"`         // API base URL (default: https://api.anthropic.com)
	SystemPrompt    string `json:"SystemPrompt"`    // Override system prompt (optional)
}

// oauthCredentials mirrors the Claude Code credentials file structure.
type oauthCredentials struct {
	ClaudeAIOAuth struct {
		AccessToken      string `json:"accessToken"`
		RefreshToken     string `json:"refreshToken"`
		ExpiresAt        int64  `json:"expiresAt"` // unix ms
		SubscriptionType string `json:"subscriptionType"`
	} `json:"claudeAiOauth"`
}

// anthropicClient calls the Anthropic Messages API directly over HTTPS.
// Supports both standalone API keys and Claude Code Max subscription OAuth tokens.
type anthropicClient struct {
	cfg             AnthropicConfig
	http            *http.Client
	mu              sync.Mutex
	accessToken     string
	refreshToken    string
	expiresAt       int64 // unix ms
	credentialsPath string
}

// newAnthropicClient creates a client for direct API access.
// Priority: ANTHROPIC_API_KEY env > config APIKey > Claude Code credentials file.
// Returns nil if no auth method is available.
func newAnthropicClient(cfg AnthropicConfig) *anthropicClient {
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.SystemPrompt == "" {
		// Try loading shared CLAUDE.md from MinIO (cluster-wide rules).
		if rules := loadClusterRules(); rules != "" {
			cfg.SystemPrompt = rules
		} else {
			cfg.SystemPrompt = "You are the AI operations engine for a Globular cluster. " +
				"You have tools available to query cluster health, memory, node status, and manage services. " +
				"Always respond with structured JSON analysis when asked to diagnose incidents. " +
				"Safety first — when uncertain, recommend observe_and_record."
		}
	}

	c := &anthropicClient{
		cfg: cfg,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}

	// 1. Env var API key (highest priority)
	if key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); key != "" {
		c.accessToken = key
		logger.Info("anthropic: using API key from ANTHROPIC_API_KEY env")
		return c
	}

	// 2. Config API key
	if cfg.APIKey != "" {
		c.accessToken = cfg.APIKey
		logger.Info("anthropic: using API key from config")
		return c
	}

	// 3. etcd shared credentials (synced from any node that has the token)
	if err := c.loadCredentialsFromEtcd(); err == nil {
		logger.Info("anthropic: using Max subscription from etcd (cluster-shared)",
			"expires_in", time.Until(time.UnixMilli(c.expiresAt)).Round(time.Minute))
		// Also try to find local credentials path for writes.
		c.credentialsPath = findCredentialsFile(cfg.CredentialsFile)
		return c
	}

	// 4. Claude Code credentials file (Max subscription — $0 extra cost)
	credPath := findCredentialsFile(cfg.CredentialsFile)
	if credPath != "" {
		if err := c.loadCredentials(credPath); err == nil {
			c.credentialsPath = credPath
			// Sync to etcd so other nodes can use it.
			c.saveCredentials()
			// Also sync to the service user's CLI credentials path so the
			// Claude CLI subprocess can authenticate (fixes startup timing:
			// claudeClient inits before anthropicClient seeds etcd).
			syncCLICredentialsFromEtcd()
			logger.Info("anthropic: using Max subscription from Claude Code credentials",
				"path", credPath,
				"subscription", "max",
				"expires_in", time.Until(time.UnixMilli(c.expiresAt)).Round(time.Minute))
			return c
		}
	}

	logger.Info("anthropic: no credentials found (set ANTHROPIC_API_KEY or install Claude Code)")
	return nil
}

// findCredentialsFile returns the path to the Claude Code credentials file.
// Searches the service user's home, then scans /home/* for any user who has
// logged into Claude Code — so the ai_executor auto-discovers credentials
// even when running as the globular system user.
func findCredentialsFile(configured string) string {
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured
		}
	}

	// 1. Check the service user's own home.
	home, _ := os.UserHomeDir()
	if home != "" {
		path := filepath.Join(home, ".claude", ".credentials.json")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 2. Scan /home/* for any user with Claude Code credentials.
	entries, err := os.ReadDir("/home")
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join("/home", entry.Name(), ".claude", ".credentials.json")
			if _, err := os.Stat(path); err == nil {
				logger.Info("anthropic: found Claude Code credentials", "path", path)
				return path
			}
		}
	}

	return ""
}

// loadCredentials reads the Claude Code credentials file.
func (c *anthropicClient) loadCredentials(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var creds oauthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return fmt.Errorf("parse credentials: %w", err)
	}
	if creds.ClaudeAIOAuth.AccessToken == "" {
		return fmt.Errorf("no access token in credentials")
	}
	c.accessToken = creds.ClaudeAIOAuth.AccessToken
	c.refreshToken = creds.ClaudeAIOAuth.RefreshToken
	c.expiresAt = creds.ClaudeAIOAuth.ExpiresAt
	return nil
}

// ensureValidToken checks if the OAuth token is expired and refreshes if needed.
// For plain API keys (no refresh token), this is a no-op.
func (c *anthropicClient) ensureValidToken() error {
	if c.refreshToken == "" {
		return nil // plain API key, never expires
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Token still valid (with 5 min buffer)?
	if time.Now().UnixMilli() < c.expiresAt-300_000 {
		return nil
	}

	// Try re-reading the credentials file first — Claude Code may have
	// refreshed the token already (it runs in the background).
	if c.credentialsPath != "" {
		if err := c.loadCredentials(c.credentialsPath); err == nil {
			if time.Now().UnixMilli() < c.expiresAt-300_000 {
				logger.Info("anthropic: token refreshed from credentials file")
				return nil
			}
		}
	}

	// Refresh via Anthropic OAuth endpoint.
	return c.refreshAccessToken()
}

// refreshAccessToken calls the Anthropic OAuth token refresh endpoint.
func (c *anthropicClient) refreshAccessToken() error {
	reqBody, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": c.refreshToken,
	})

	resp, err := c.http.Post(
		"https://console.anthropic.com/v1/oauth/token",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return fmt.Errorf("refresh token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("refresh token failed: %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"` // seconds
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse refresh response: %w", err)
	}

	c.accessToken = result.AccessToken
	if result.RefreshToken != "" {
		c.refreshToken = result.RefreshToken
	}
	c.expiresAt = time.Now().UnixMilli() + result.ExpiresIn*1000

	// Write back to credentials file so Claude Code picks it up too.
	if c.credentialsPath != "" {
		c.saveCredentials()
	}

	logger.Info("anthropic: token refreshed via OAuth",
		"expires_in", time.Duration(result.ExpiresIn)*time.Second)
	return nil
}

// saveCredentials writes updated tokens back to the credentials file AND etcd.
// etcd makes the token available to all nodes in the cluster.
// When the credentials were loaded from a file, saves the raw file content
// to preserve all fields (scopes, rateLimitTier, etc.) that the CLI needs.
func (c *anthropicClient) saveCredentials() {
	var data []byte

	// Prefer raw file content to preserve all fields the CLI needs.
	if c.credentialsPath != "" {
		if raw, err := os.ReadFile(c.credentialsPath); err == nil && len(raw) > 0 {
			data = raw
		}
	}

	// Fallback: reconstruct from parsed fields.
	if len(data) == 0 {
		creds := oauthCredentials{}
		creds.ClaudeAIOAuth.AccessToken = c.accessToken
		creds.ClaudeAIOAuth.RefreshToken = c.refreshToken
		creds.ClaudeAIOAuth.ExpiresAt = c.expiresAt
		creds.ClaudeAIOAuth.SubscriptionType = "max"

		var err error
		data, err = json.MarshalIndent(creds, "", "  ")
		if err != nil {
			return
		}
	}

	// Write to local file (keeps Claude Code CLI in sync).
	if c.credentialsPath != "" {
		_ = os.WriteFile(c.credentialsPath, data, 0600)
	}

	// Write to etcd (shares token with all nodes in the cluster).
	if err := etcdPut(etcdCredentialsKey, string(data)); err != nil {
		logger.Warn("anthropic: failed to save credentials to etcd", "err", err)
	} else {
		logger.Info("anthropic: credentials synced to etcd")
	}
}

const etcdCredentialsKey = "/globular/secrets/anthropic-credentials"

// loadClusterRules reads CLAUDE.md — the shared AI operational rules.
// Priority: MinIO config bucket > etcd > local file > empty.
func loadClusterRules() string {
	// 1. MinIO cluster config bucket (canonical source, replicated).
	if data, err := config.GetClusterConfig(config.ConfigKeyClaudeMD); err == nil && len(data) > 0 {
		logger.Info("anthropic: loaded cluster rules from MinIO", "size", len(data))
		return string(data)
	}

	// 2. etcd (fallback for early bootstrap before MinIO is ready).
	if val, err := etcdGet("/globular/ai/claude-md"); err == nil && val != "" {
		logger.Info("anthropic: loaded cluster rules from etcd", "size", len(val))
		return val
	}

	// 3. Local file (dev/testing).
	for _, path := range []string{
		"/var/lib/globular/ai/CLAUDE.md",
		"/var/lib/globular/CLAUDE.md",
	} {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			logger.Info("anthropic: loaded cluster rules from file", "path", path)
			return string(data)
		}
	}

	return ""
}

// etcdPut writes a key-value pair to etcd.
func etcdPut(key, value string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Put(ctx, key, value)
	return err
}

// etcdGet reads a value from etcd. Returns "" if not found.
func etcdGet(key string) (string, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("key not found")
	}
	return string(resp.Kvs[0].Value), nil
}

// loadCredentialsFromEtcd tries to load OAuth credentials from etcd.
// This allows any node in the cluster to use the token without having
// the local credentials file.
func (c *anthropicClient) loadCredentialsFromEtcd() error {
	val, err := etcdGet(etcdCredentialsKey)
	if err != nil {
		return err
	}
	if val == "" {
		return fmt.Errorf("no credentials in etcd")
	}
	var creds oauthCredentials
	if err := json.Unmarshal([]byte(val), &creds); err != nil {
		return fmt.Errorf("parse etcd credentials: %w", err)
	}
	if creds.ClaudeAIOAuth.AccessToken == "" {
		return fmt.Errorf("empty access token in etcd")
	}
	c.accessToken = creds.ClaudeAIOAuth.AccessToken
	c.refreshToken = creds.ClaudeAIOAuth.RefreshToken
	c.expiresAt = creds.ClaudeAIOAuth.ExpiresAt
	return nil
}

// isAvailable returns true if the client has valid credentials (API key or OAuth token).
func (c *anthropicClient) isAvailable() bool {
	return c != nil && (c.cfg.APIKey != "" || c.accessToken != "")
}

// --- Anthropic Messages API types ---

type messagesRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []message     `json:"messages"`
	Tools     []toolDef     `json:"tools,omitempty"`
}

type message struct {
	Role    string        `json:"role"`
	Content any           `json:"content"` // string or []contentBlock
}

type contentBlock struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Input   any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

type messagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []contentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	Usage        usageInfo      `json:"usage"`
}

type usageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type apiError struct {
	Type    string `json:"type"`
	Error   struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// sendConversation sends a multi-turn conversation and returns the full API response.
func (c *anthropicClient) sendConversation(ctx context.Context, systemPrompt string, messages []message) (*messagesResponse, error) {
	req := messagesRequest{
		Model:     c.cfg.Model,
		MaxTokens: c.cfg.MaxTokens,
		System:    systemPrompt,
		Messages:  messages,
	}
	return c.callAPI(ctx, req)
}

// sendPrompt sends a simple text prompt and returns the text response.
func (c *anthropicClient) sendPrompt(ctx context.Context, prompt string) (string, error) {
	req := messagesRequest{
		Model:     c.cfg.Model,
		MaxTokens: c.cfg.MaxTokens,
		System:    c.cfg.SystemPrompt,
		Messages: []message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := c.callAPI(ctx, req)
	if err != nil {
		return "", err
	}

	// Extract text from response content blocks.
	var texts []string
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}

	if len(texts) == 0 {
		return "", fmt.Errorf("no text in response (stop_reason=%s)", resp.StopReason)
	}

	return strings.Join(texts, "\n"), nil
}

// callAPI makes a raw Messages API call.
func (c *anthropicClient) callAPI(ctx context.Context, req messagesRequest) (*messagesResponse, error) {
	// Refresh OAuth token if expired.
	if err := c.ensureValidToken(); err != nil {
		return nil, fmt.Errorf("token refresh: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.cfg.BaseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Use the token — works for both API keys (x-api-key) and OAuth tokens (Bearer).
	token := c.accessToken
	if strings.HasPrefix(token, "sk-ant-oat") {
		// OAuth token from Max subscription
		httpReq.Header.Set("Authorization", "Bearer "+token)
	} else {
		// Standalone API key
		httpReq.Header.Set("x-api-key", token)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		var apiErr apiError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result messagesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	logger.Info("anthropic: API call completed",
		"model", result.Model,
		"input_tokens", result.Usage.InputTokens,
		"output_tokens", result.Usage.OutputTokens,
		"stop_reason", result.StopReason)

	return &result, nil
}
