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
)

const codexCredentialsKey = "/globular/secrets/codex-credentials"

// CodexConfig holds the configuration for Codex CLI access.
type CodexConfig struct {
	APIKey          string `json:"ApiKey"`
	CredentialsFile string `json:"CredentialsFile"`
	Model           string `json:"Model"`
	SystemPrompt    string `json:"SystemPrompt"`
}

type codexAuthFile struct {
	AuthMode     string `json:"auth_mode"`
	OpenAIAPIKey string `json:"OPENAI_API_KEY,omitempty"`
	Tokens       struct {
		AccessToken  string `json:"access_token,omitempty"`
		RefreshToken string `json:"refresh_token,omitempty"`
		AccountID    string `json:"account_id,omitempty"`
	} `json:"tokens,omitempty"`
	LastRefresh string `json:"last_refresh,omitempty"`
}

type codexClient struct {
	cfg         CodexConfig
	cliBinary   string
	authPath    string
	hasAuth     bool
	accessToken string
	apiKey      string
	etcdModRev  int64
	mu          sync.Mutex
}

func newCodexClient(cfg CodexConfig) *codexClient {
	if cfg.Model == "" {
		cfg.Model = "gpt-5-codex"
	}
	if cfg.SystemPrompt == "" {
		if rules := loadClusterRules(); rules != "" {
			cfg.SystemPrompt = rules
		} else {
			cfg.SystemPrompt = "You are the AI operations engine for a Globular cluster. " +
				"You help diagnose incidents and answer operator questions. " +
				"Be specific, concise, and prefer safe, low-risk actions when uncertain."
		}
	}

	c := &codexClient{
		cfg:       cfg,
		cliBinary: findCodexBinary(),
		authPath:  findCodexAuthFile(cfg.CredentialsFile),
	}
	if c.cliBinary == "" {
		logger.Info("codex: CLI not found")
		return c
	}

	if err := c.loadAuthFromEtcd(); err == nil && c.hasCredentials() {
		syncCodexCredentialsFromEtcd()
		logger.Info("codex: using shared credentials from etcd")
		return c
	}

	if c.authPath != "" {
		if err := c.loadAuthFile(c.authPath); err == nil && c.hasCredentials() {
			c.syncLocalAuthToEtcd()
			logger.Info("codex: using local auth file", "path", c.authPath)
			return c
		}
	}

	if cfg.APIKey != "" {
		if err := c.loginWithAPIKey(cfg.APIKey); err == nil {
			_ = c.loadAuthFile(c.authPath)
			c.syncLocalAuthToEtcd()
			logger.Warn("codex: using configured API key fallback")
			return c
		} else {
			logger.Warn("codex: API key fallback failed", "err", err)
		}
	}

	logger.Info("codex: no credentials found")
	return c
}

func findCodexBinary() string {
	for _, path := range []string{
		"/usr/local/bin/codex",
		"/usr/bin/codex",
		os.ExpandEnv("$HOME/.local/bin/codex"),
		os.ExpandEnv("$HOME/.nvm/versions/node/v24.4.0/bin/codex"),
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func findCodexAuthFile(configured string) string {
	if configured != "" {
		return configured
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/var/lib/globular"
	}
	return filepath.Join(home, ".codex", "auth.json")
}

func (c *codexClient) binaryPresent() bool {
	return c != nil && c.cliBinary != ""
}

func (c *codexClient) credentialsPresent() bool {
	return c != nil && c.hasAuth
}

func (c *codexClient) isAvailable() bool {
	return c != nil && c.cliBinary != "" && c.hasAuth
}

func (c *codexClient) hasCredentials() bool {
	return c.accessToken != "" || c.apiKey != ""
}

func (c *codexClient) loadAuthFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	accessToken, apiKey, err := parseCodexAuth(data)
	if err != nil {
		return err
	}
	c.authPath = path
	c.accessToken = accessToken
	c.apiKey = apiKey
	c.hasAuth = true
	return nil
}

func parseCodexAuth(data []byte) (accessToken, apiKey string, err error) {
	var auth codexAuthFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return "", "", err
	}
	accessToken = strings.TrimSpace(auth.Tokens.AccessToken)
	apiKey = strings.TrimSpace(auth.OpenAIAPIKey)
	if accessToken == "" && apiKey == "" {
		return "", "", fmt.Errorf("no Codex credentials in auth file")
	}
	return accessToken, apiKey, nil
}

func (c *codexClient) loadAuthFromEtcd() error {
	data, modRev, err := etcdGetWithRevision(codexCredentialsKey)
	if err != nil || data == "" {
		return fmt.Errorf("no Codex credentials in etcd")
	}
	accessToken, apiKey, err := parseCodexAuth([]byte(data))
	if err != nil {
		return err
	}
	if err := ensureDir(filepath.Dir(c.authPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(c.authPath, []byte(data), 0600); err != nil {
		return err
	}
	c.accessToken = accessToken
	c.apiKey = apiKey
	c.hasAuth = true
	c.etcdModRev = modRev
	return nil
}

func syncCodexCredentialsFromEtcd() {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/var/lib/globular"
	}
	authPath := filepath.Join(home, ".codex", "auth.json")
	data, err := etcdGet(codexCredentialsKey)
	if err != nil || data == "" {
		return
	}
	if _, _, err := parseCodexAuth([]byte(data)); err != nil {
		logger.Warn("codex: ignoring invalid credentials from etcd", "err", err)
		return
	}
	if err := ensureDir(filepath.Dir(authPath), 0700); err != nil {
		logger.Warn("codex: failed to create auth dir", "err", err)
		return
	}
	if err := os.WriteFile(authPath, []byte(data), 0600); err != nil {
		logger.Warn("codex: failed to write auth file", "err", err)
		return
	}
	logger.Info("codex: synced credentials from etcd", "path", authPath)
}

func (c *codexClient) syncLocalAuthToEtcd() {
	if c.authPath == "" {
		return
	}
	data, err := os.ReadFile(c.authPath)
	if err != nil || len(data) == 0 {
		return
	}
	if c.etcdModRev > 0 {
		ok, err := etcdCASPut(codexCredentialsKey, string(data), c.etcdModRev)
		if err != nil {
			logger.Warn("codex: failed to CAS credentials to etcd", "err", err)
			return
		}
		if !ok {
			logger.Info("codex: CAS write lost to peer node — reloading their credentials from etcd")
			_ = c.loadAuthFromEtcd()
			return
		}
	} else if err := etcdPut(codexCredentialsKey, string(data)); err != nil {
		logger.Warn("codex: failed to seed credentials to etcd", "err", err)
		return
	}

	if _, modRev, err := etcdGetWithRevision(codexCredentialsKey); err == nil {
		c.etcdModRev = modRev
	}
}

func (c *codexClient) loginWithAPIKey(apiKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ensureDir(filepath.Dir(c.authPath), 0700); err != nil {
		return err
	}

	cmd := exec.Command(c.cliBinary, "login", "--with-api-key")
	cmd.Stdin = strings.NewReader(apiKey)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codex login --with-api-key: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (c *codexClient) sendPrompt(ctx context.Context, prompt string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isAvailable() {
		return "", fmt.Errorf("codex CLI unavailable")
	}

	tmp, err := os.CreateTemp("", "codex-last-message-*.txt")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--sandbox", "read-only",
		"--color", "never",
		"-o", tmpPath,
	}
	if c.cfg.Model != "" {
		args = append(args, "--model", c.cfg.Model)
	}
	if _, err := os.Stat("/var/lib/globular/services"); err == nil {
		args = append(args, "--cd", "/var/lib/globular/services")
	}
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, c.cliBinary, args...)
	cmd.Stdin = strings.NewReader(c.cfg.SystemPrompt + "\n\n" + prompt)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	reply, readErr := os.ReadFile(tmpPath)
	if runErr != nil && (readErr != nil || strings.TrimSpace(string(reply)) == "") {
		return "", fmt.Errorf("codex exec failed: %w (stderr: %s)", runErr, strings.TrimSpace(stderr.String()))
	}

	text := strings.TrimSpace(string(reply))
	if text == "" {
		text = strings.TrimSpace(stdout.String())
	}
	if text == "" {
		return "", fmt.Errorf("codex produced no response")
	}

	c.syncLocalAuthToEtcd()
	return text, nil
}

func (c *codexClient) shutdown() {}

func ensureDir(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}
