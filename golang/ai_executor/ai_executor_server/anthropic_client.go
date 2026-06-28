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
	clientv3 "go.etcd.io/etcd/client/v3"
)

// AnthropicConfig holds the configuration for direct Anthropic API access.
type AnthropicConfig struct {
	APIKey          string `json:"ApiKey"`          // Anthropic API key (sk-ant-api...) — standalone billing
	CredentialsFile string `json:"CredentialsFile"` // Path to Claude Code credentials (uses Max subscription)
	Model           string `json:"Model"`           // Model ID (default: claude-sonnet-4-6)
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
	etcdModRev      int64 // ModRevision of the etcd credential entry we last loaded; 0 if loaded from file
}

// Seams for testing credential precedence without a live etcd/filesystem.
// They wrap the real functions so production behavior is unchanged.
var (
	probeEtcdMaxCreds  = func(c *anthropicClient) error { return c.loadCredentialsFromEtcd() }
	locateCredFile     = func(configured string) string { return findCredentialsFile(configured) }
	persistCredsToEtcd = func(c *anthropicClient) { c.saveCredentials() }
	syncCLICreds       = func() { syncCLICredentialsFromEtcd() }
	// etcdWriteCredsCAS is the CAS write gate for refresh-token safety. Stubbed in tests.
	etcdWriteCredsCAS = func(data string, modRev int64) (bool, error) {
		return etcdCASPut(etcdCredentialsKey, data, modRev)
	}
	// etcdReloadCreds reloads credentials from etcd after a CAS loss. Stubbed in tests.
	etcdReloadCreds = func(c *anthropicClient) error { return c.loadCredentialsFromEtcd() }

	// maxCredUsable gates a loaded Max OAuth credential on USABILITY, not mere
	// presence: a non-empty access token that is either comfortably unexpired or
	// refreshes successfully right now. A stale/revoked Max blob must NOT be
	// selected over a configured API-key fallback — doing so builds a client
	// that fails every request with no recovery (a "configured but broken"
	// state worse than AI-off). See evidence.provenance_trust_levels.
	maxCredUsable = func(c *anthropicClient) bool {
		if c.accessToken == "" {
			return false
		}
		// Comfortably unexpired — same 5-min buffer as ensureValidToken.
		if c.expiresAt != 0 && time.Now().UnixMilli() < c.expiresAt-300_000 {
			return true
		}
		// Expired or unknown expiry: usable only if it can refresh now. A plain
		// API key never reaches here (it is the fallback, not a Max cred).
		if c.refreshToken == "" {
			return false
		}
		return c.refreshAccessToken() == nil
	}
)

// newAnthropicClient creates a client for direct API access.
//
// Credential precedence is COST-AWARE: the flat-rate Max subscription (OAuth —
// from etcd, then the local credentials file) is ALWAYS preferred over the
// metered standalone API key. The API key is a FALLBACK, selected only when no
// usable Max credential exists, so an accidentally-configured key can never
// silently preempt the subscription and start per-token billing. When the key
// is used, the client logs a loud warning.
// (Operator decision 2026-06-28; see failure_mode
// ai_executor.repeat_diagnosis_drains_personal_subscription.)
//
// Environment variables are NOT used for credentials (etcd is the sole config source).
// Returns nil if no auth method is available.
func newAnthropicClient(cfg AnthropicConfig) *anthropicClient {
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-6"
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

	// 1. etcd shared Max credentials (synced from any node that has the token).
	// Gated on USABILITY, not presence — a stale/revoked blob falls through to
	// the API-key fallback rather than building a client that fails every call.
	if err := probeEtcdMaxCreds(c); err == nil {
		if maxCredUsable(c) {
			logger.Info("anthropic: using Max subscription from etcd (cluster-shared)",
				"expires_in", time.Until(time.UnixMilli(c.expiresAt)).Round(time.Minute))
			// Also find the local credentials path for write-back on refresh.
			c.credentialsPath = locateCredFile(cfg.CredentialsFile)
			return c
		}
		logger.Warn("anthropic: etcd Max credential present but unusable (expired/unrefreshable) — falling through to next credential source")
	}

	// 2. Local Claude Code credentials file (Max subscription — $0 extra cost).
	credPath := locateCredFile(cfg.CredentialsFile)
	if credPath != "" {
		if err := c.loadCredentials(credPath); err == nil {
			if maxCredUsable(c) {
				c.credentialsPath = credPath
				// Sync to etcd so other nodes can use it.
				persistCredsToEtcd(c)
				// Also sync to the service user's CLI credentials path so the
				// Claude CLI subprocess can authenticate (fixes startup timing:
				// claudeClient inits before anthropicClient seeds etcd).
				syncCLICreds()
				logger.Info("anthropic: using Max subscription from Claude Code credentials",
					"path", credPath,
					"subscription", "max",
					"expires_in", time.Until(time.UnixMilli(c.expiresAt)).Round(time.Minute))
				return c
			}
			logger.Warn("anthropic: local Max credential present but unusable (expired/unrefreshable) — falling through to API key",
				"path", credPath)
		}
	}

	// 3. Standalone API key (metered, separately billed) — FALLBACK ONLY.
	// Reached only when no Max subscription credential is available, so the
	// flat-rate subscription is never preempted by a configured key.
	if cfg.APIKey != "" {
		c.accessToken = cfg.APIKey
		logger.Warn("anthropic: using standalone API key — METERED per-token billing. " +
			"No Max subscription credential found (etcd or service-user credentials file); " +
			"provision one to use the flat-rate subscription instead.")
		return c
	}

	logger.Info("anthropic: no credentials found (provision a Max subscription credential, or set APIKey in config as a metered fallback)")
	return nil
}

// findCredentialsFile returns the path to an *explicitly provisioned* Claude
// Code credentials file: the configured path, or the service user's own home.
//
// It deliberately does NOT scan /home/* for other users' logins. That scan used
// to harvest a developer's personal Claude Max subscription token, copy it into
// cluster etcd (replicated to every node), and spend it on every incident — an
// unbounded, silent drain of a personal account plus a credential-exposure. AI
// diagnosis is now opt-in: an operator places a dedicated, separately-billed
// credential at the service-user path or sets Anthropic.ApiKey in config. With
// neither, the client is unavailable and the diagnoser uses deterministic
// analysis — AI is supplementary, never required.
func findCredentialsFile(configured string) string {
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured
		}
	}

	// The service user's own home — an operator-provisioned credential, not a
	// harvested personal login.
	home, _ := os.UserHomeDir()
	if home != "" {
		path := filepath.Join(home, ".claude", ".credentials.json")
		if _, err := os.Stat(path); err == nil {
			return path
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

	// Try re-reading the local credentials file first — Claude Code may have
	// refreshed the token already (it runs in the background).
	if c.credentialsPath != "" {
		if err := c.loadCredentials(c.credentialsPath); err == nil {
			if time.Now().UnixMilli() < c.expiresAt-300_000 {
				logger.Info("anthropic: token refreshed from credentials file")
				return nil
			}
		}
	}

	// Check etcd before calling the Anthropic API. A peer node may have already
	// consumed the shared refresh_token and written a fresh access_token. Since
	// OAuth refresh tokens are single-use, calling the API with an already-consumed
	// token causes a permanent 401 — not a transient error, not self-healing.
	// Reusing the peer's fresh token costs nothing and avoids destroying credentials.
	if err := c.loadCredentialsFromEtcd(); err == nil {
		if time.Now().UnixMilli() < c.expiresAt-300_000 {
			logger.Info("anthropic: token reloaded from etcd (refreshed by peer node)")
			return nil
		}
	}

	// Refresh via Anthropic OAuth endpoint.
	return c.refreshAccessToken()
}

// claudeOAuthClientID is the public OAuth client ID used by Claude Code.
// Required for the token refresh endpoint.
const claudeOAuthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

// refreshAccessToken calls the Anthropic OAuth token refresh endpoint.
func (c *anthropicClient) refreshAccessToken() error {
	reqBody, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": c.refreshToken,
		"client_id":     claudeOAuthClientID,
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
//
// When etcdModRev > 0 (credentials were originally loaded from etcd) a CAS
// write is used: only the node whose view of the etcd revision matches wins.
// The losing node reloads the winner's credentials — this is the refresh_token
// safety contract: a spent refresh_token must never overwrite a valid one.
//
// When etcdModRev == 0 (initial seed from a local credentials file) a plain
// Put is used — no peer race is possible at seed time.
func (c *anthropicClient) saveCredentials() {
	var data []byte

	// Prefer raw file content to preserve all fields (scopes, rateLimitTier, etc.)
	// that the CLI needs, and which would be lost if we reconstructed from parsed fields.
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

	if c.etcdModRev > 0 {
		// CAS write: only succeeds if no other node has written since we loaded.
		// If two nodes both called refreshAccessToken() with the same (single-use)
		// refresh_token, only one CAS wins. The loser reloads the winner's valid
		// credentials instead of holding a permanently broken token.
		ok, err := etcdWriteCredsCAS(string(data), c.etcdModRev)
		if err != nil {
			logger.Warn("anthropic: failed to CAS credentials to etcd", "err", err)
			return
		}
		if !ok {
			logger.Info("anthropic: CAS write lost to peer node — reloading their credentials from etcd")
			_ = etcdReloadCreds(c)
			return
		}
		logger.Info("anthropic: credentials saved to etcd (CAS)", "key", etcdCredentialsKey)
	} else {
		// Initial seed from local file — plain Put.
		if err := etcdPut(etcdCredentialsKey, string(data)); err != nil {
			logger.Warn("anthropic: failed to seed credentials to etcd", "err", err)
		} else {
			logger.Info("anthropic: credentials seeded to etcd", "key", etcdCredentialsKey)
		}
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
	val, _, err := etcdGetWithRevision(key)
	return val, err
}

// etcdGetWithRevision reads a value and its ModRevision from etcd.
// The revision is used for CAS writes to detect concurrent modifications.
func etcdGetWithRevision(key string) (string, int64, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return "", 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return "", 0, err
	}
	if len(resp.Kvs) == 0 {
		return "", 0, fmt.Errorf("key not found")
	}
	return string(resp.Kvs[0].Value), resp.Kvs[0].ModRevision, nil
}

// etcdCASPut writes key=value only if the key's current ModRevision matches
// expectedModRev. Returns (true, nil) on success, (false, nil) if another
// writer won the race, or (false, err) on transport failure.
func etcdCASPut(key, value string, expectedModRev int64) (bool, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", expectedModRev)).
		Then(clientv3.OpPut(key, value)).
		Commit()
	if err != nil {
		return false, err
	}
	return resp.Succeeded, nil
}

// loadCredentialsFromEtcd tries to load OAuth credentials from etcd.
// Captures the ModRevision so saveCredentials can use CAS on the next write.
func (c *anthropicClient) loadCredentialsFromEtcd() error {
	val, modRev, err := etcdGetWithRevision(etcdCredentialsKey)
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
	c.etcdModRev = modRev
	return nil
}

// isAvailable returns true if the client has valid credentials (API key or OAuth token).
func (c *anthropicClient) isAvailable() bool {
	return c != nil && (c.cfg.APIKey != "" || c.accessToken != "")
}

// credentialsPresent reports whether any credential material is configured,
// independent of whether it is currently usable (a token may be present but
// expired). This is the weaker "credentials_present" signal in the readiness
// model; "backend_ready" requires isAvailable().
func (c *anthropicClient) credentialsPresent() bool {
	return c != nil && (c.cfg.APIKey != "" || c.accessToken != "" || c.refreshToken != "")
}

// --- Anthropic Messages API types ---

type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []message `json:"messages"`
	Tools     []toolDef `json:"tools,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []contentBlock
}

type contentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

type messagesResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []contentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      usageInfo      `json:"usage"`
}

type usageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type apiError struct {
	Type  string `json:"type"`
	Error struct {
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
