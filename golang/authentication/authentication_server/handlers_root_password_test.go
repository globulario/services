package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/security"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/metadata"
)

func writeConfig(t *testing.T, path, rootPassword string) {
	t.Helper()
	cfg := map[string]any{
		"AdminEmail":   "root@example.com",
		"RootPassword": rootPassword,
		"Mac":          "00:11:22:33:44:55",
		"Address":      "127.0.0.1:10004",
		"Domain":       "example.com",
	}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func newTestServer(t *testing.T, rootPassword string) *server {
	t.Helper()
	tmp := t.TempDir()
	if err := os.Setenv("GLOBULAR_STATE_DIR", tmp); err != nil {
		t.Fatalf("set env: %v", err)
	}
	oldDataPath, oldConfigPath := dataPath, configPath
	dataPath = tmp
	configPath = filepath.Join(tmp, "config.json")
	t.Cleanup(func() {
		dataPath = oldDataPath
		configPath = oldConfigPath
		_ = os.Unsetenv("GLOBULAR_STATE_DIR")
	})
	writeConfig(t, configPath, rootPassword)
	return &server{
		Domain:         "example.com",
		SessionTimeout: 60,
		Address:        "",
	}
}

func readRootPassword(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	cfg := map[string]any{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	pw, _ := cfg["RootPassword"].(string)
	return pw
}

func TestAuthenticateUpgradesDefaultPasswordToBcrypt(t *testing.T) {
	srv := newTestServer(t, "")
	if _, err := srv.authenticate("sa", "adminadmin", "issuer"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	stored := readRootPassword(t)
	if stored == "adminadmin" || !isBcryptHash(stored) {
		t.Fatalf("expected bcrypt hash persisted, got %q", stored)
	}
}

func TestAuthenticateUpgradesPlaintextToBcrypt(t *testing.T) {
	srv := newTestServer(t, "plain")
	if _, err := srv.authenticate("sa", "plain", "issuer"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	stored := readRootPassword(t)
	if stored == "plain" || !isBcryptHash(stored) {
		t.Fatalf("expected bcrypt hash, got %q", stored)
	}
}

func TestAuthenticateKeepsExistingBcrypt(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	srv := newTestServer(t, string(hash))
	if _, err := srv.authenticate("sa", "secret", "issuer"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	stored := readRootPassword(t)
	if stored != string(hash) {
		t.Fatalf("bcrypt hash should remain unchanged")
	}
	// Critical: raw password must never appear in the config file after login.
	raw, _ := os.ReadFile(configPath)
	if strings.Contains(string(raw), "secret") {
		t.Fatalf("raw password found in config file after bcrypt login")
	}
}

func TestSetRootPasswordStoresBcryptAndPolicy(t *testing.T) {
	srv := newTestServer(t, "oldplain")
	token, err := security.GenerateToken(30, "issuer", "sa", "sa", "admin@example.com")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("token", token))
	req := &authenticationpb.SetRootPasswordRequest{OldPassword: "oldplain", NewPassword: "StrongerPass123!"}
	if _, err := srv.SetRootPassword(ctx, req); err != nil {
		t.Fatalf("SetRootPassword: %v", err)
	}
	stored := readRootPassword(t)
	if !isBcryptHash(stored) {
		t.Fatalf("expected bcrypt hash, got %q", stored)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte("StrongerPass123!")); err != nil {
		t.Fatalf("stored hash does not match new password: %v", err)
	}
}

func TestSetRootPasswordPolicyEnforced(t *testing.T) {
	srv := newTestServer(t, "oldplain")
	token, _ := security.GenerateToken(30, "issuer", "sa", "sa", "admin@example.com")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("token", token))
	req := &authenticationpb.SetRootPasswordRequest{OldPassword: "oldplain", NewPassword: "short"}
	if _, err := srv.SetRootPassword(ctx, req); err == nil {
		t.Fatalf("expected policy error for short password")
	}
}
