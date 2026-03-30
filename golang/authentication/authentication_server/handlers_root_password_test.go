package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/metadata"
)

// seedRootCreds writes root credentials to etcd for testing.
// Returns a cleanup function that removes the key.
func seedRootCreds(t *testing.T, password, email string) {
	t.Helper()
	creds := &config.RootCredentials{
		RootPassword: password,
		AdminEmail:   email,
	}
	if err := config.SetRootCredentials(creds); err != nil {
		t.Skipf("etcd unavailable, skipping: %v", err)
	}
	t.Cleanup(func() {
		// Reset to empty so tests don't leak state.
		_ = config.SetRootCredentials(&config.RootCredentials{})
	})
}

func readRootPassword(t *testing.T) string {
	t.Helper()
	creds, err := config.GetRootCredentials()
	if err != nil {
		t.Fatalf("read root credentials: %v", err)
	}
	return creds.RootPassword
}

func newTestServer(t *testing.T, rootPassword string) *server {
	t.Helper()
	tmp := t.TempDir()
	oldDataPath := dataPath
	dataPath = tmp
	t.Cleanup(func() {
		dataPath = oldDataPath
	})
	seedRootCreds(t, rootPassword, "root@example.com")
	return &server{
		Domain:         "example.com",
		SessionTimeout: 60,
		Address:        "",
	}
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
