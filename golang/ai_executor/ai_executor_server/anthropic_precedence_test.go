package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These tests pin the COST-AWARE credential precedence: the flat-rate Max
// subscription (etcd OAuth, then local credentials file) must always win over
// the metered standalone API key. A configured API key must never preempt an
// available Max credential — otherwise the cluster silently switches to
// per-token billing. (Operator decision 2026-06-28.)

// stubSeams installs safe default stubs for the external probes and restores
// the originals after the test. Default: no etcd creds, no file, no-op writes.
func stubSeams(t *testing.T) {
	t.Helper()
	oEtcd, oFile, oPersist, oSync, oUsable := probeEtcdMaxCreds, locateCredFile, persistCredsToEtcd, syncCLICreds, maxCredUsable
	t.Cleanup(func() {
		probeEtcdMaxCreds, locateCredFile, persistCredsToEtcd, syncCLICreds, maxCredUsable = oEtcd, oFile, oPersist, oSync, oUsable
	})
	probeEtcdMaxCreds = func(c *anthropicClient) error { return errors.New("no etcd creds") }
	locateCredFile = func(string) string { return "" }
	persistCredsToEtcd = func(*anthropicClient) {}
	syncCLICreds = func() {}
	maxCredUsable = func(*anthropicClient) bool { return true } // loaded Max creds usable unless a test overrides
}

func writeMaxCredFile(t *testing.T, accessToken string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".credentials.json")
	body, _ := json.Marshal(map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":      accessToken,
			"refreshToken":     "refresh-xyz",
			"expiresAt":        int64(4102444800000), // year 2100, far future
			"subscriptionType": "max",
		},
	})
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatalf("write temp creds: %v", err)
	}
	return path
}

const expensiveKey = "sk-ant-api03-EXPENSIVE-METERED-KEY"

// The core cost-safety assertion: even with an API key configured, an available
// Max credential from etcd must be the one actually used.
func TestCredentialPrecedence_EtcdMaxBeatsApiKey(t *testing.T) {
	stubSeams(t)
	probeEtcdMaxCreds = func(c *anthropicClient) error {
		c.accessToken = "oat-from-etcd"
		c.refreshToken = "r"
		return nil
	}

	c := newAnthropicClient(AnthropicConfig{APIKey: expensiveKey, SystemPrompt: "x"})
	if c == nil {
		t.Fatal("client is nil")
	}
	if c.accessToken != "oat-from-etcd" {
		t.Fatalf("accessToken=%q — API key preempted the Max(etcd) subscription (cost regression)", c.accessToken)
	}
	if c.accessToken == expensiveKey {
		t.Fatal("the metered API key is the active token despite a Max credential being available")
	}
}

func TestCredentialPrecedence_FileMaxBeatsApiKey(t *testing.T) {
	stubSeams(t)
	credPath := writeMaxCredFile(t, "oat-from-file")
	locateCredFile = func(string) string { return credPath }

	c := newAnthropicClient(AnthropicConfig{APIKey: expensiveKey, SystemPrompt: "x"})
	if c == nil {
		t.Fatal("client is nil")
	}
	if c.accessToken != "oat-from-file" {
		t.Fatalf("accessToken=%q — API key preempted the Max(file) subscription (cost regression)", c.accessToken)
	}
}

// The API key works, but only as a fallback when no Max credential exists.
func TestCredentialPrecedence_ApiKeyIsFallbackWhenNoMax(t *testing.T) {
	stubSeams(t) // no etcd, no file

	c := newAnthropicClient(AnthropicConfig{APIKey: expensiveKey, SystemPrompt: "x"})
	if c == nil {
		t.Fatal("client is nil — API key fallback must still produce a usable client")
	}
	if c.accessToken != expensiveKey {
		t.Fatalf("accessToken=%q, want the API key used as fallback", c.accessToken)
	}
	if !c.isAvailable() {
		t.Fatal("isAvailable() false with an API key configured")
	}
}

func TestCredentialPrecedence_NoCredentialsReturnsNil(t *testing.T) {
	stubSeams(t) // no etcd, no file, no api key

	c := newAnthropicClient(AnthropicConfig{SystemPrompt: "x"})
	if c != nil {
		t.Fatalf("expected nil client with no credentials, got accessToken=%q", c.accessToken)
	}
}

// #1 regression: a stale/unusable Max credential (expired, unrefreshable) must
// NOT shadow a valid configured API key. Presence is not usability — the gate
// falls through to the fallback so AI stays available.
func TestCredentialPrecedence_StaleMaxFallsBackToApiKey(t *testing.T) {
	stubSeams(t)
	// etcd returns a Max blob (a token loads) but it is NOT usable.
	probeEtcdMaxCreds = func(c *anthropicClient) error {
		c.accessToken = "oat-stale-etcd"
		c.refreshToken = "revoked"
		return nil
	}
	maxCredUsable = func(*anthropicClient) bool { return false }

	c := newAnthropicClient(AnthropicConfig{APIKey: expensiveKey, SystemPrompt: "x"})
	if c == nil {
		t.Fatal("client is nil — must fall back to the API key when the Max cred is unusable")
	}
	if c.accessToken != expensiveKey {
		t.Fatalf("accessToken=%q — a stale Max cred shadowed the valid API-key fallback (#1 regression)", c.accessToken)
	}
}

// The usability gate itself, covering the branches reachable without a network
// refresh. (The expired-but-refreshes-OK branch needs a live refresh and is
// exercised at runtime, not here.)
func TestMaxCredUsable_NoNetworkBranches(t *testing.T) {
	if maxCredUsable(&anthropicClient{}) {
		t.Fatal("empty access token must be unusable")
	}
	future := time.Now().Add(time.Hour).UnixMilli()
	if !maxCredUsable(&anthropicClient{accessToken: "t", expiresAt: future}) {
		t.Fatal("comfortably-unexpired token must be usable without a refresh")
	}
	past := time.Now().Add(-time.Hour).UnixMilli()
	if maxCredUsable(&anthropicClient{accessToken: "t", expiresAt: past}) {
		t.Fatal("expired token with no refresh token must be unusable")
	}
}
