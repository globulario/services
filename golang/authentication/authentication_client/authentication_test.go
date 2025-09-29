package authentication_client

import (
	"log"
	"os"
	"testing"
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var (
	domain   = getenv("GLOBULAR_DOMAIN", "globule-ryzen.globular.io") // make sure this matches your running stack
	saUser   = getenv("GLOBULAR_SA_USER", "sa")
	saPwd    = getenv("GLOBULAR_SA_PWD", "adminadmin")
	rotateOK = getenv("AUTH_TEST_ALLOW_ROTATE", "") == "true"

	client, _ = NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
)

// Helper: authenticate and stash the token in the client context (for methods that
// expect auth via metadata rather than explicit args).
func mustAuth(t *testing.T) string {
	t.Helper()
	token, err := client.Authenticate(saUser, saPwd)
	if err != nil {
		t.Fatalf("Authenticate(%s) failed on domain %s: %v", saUser, domain, err)
	}
	// make the token available for subsequent calls that need metadata
	if err := client.SetToken(token); err != nil {
		t.Fatalf("SetToken failed: %v", err)
	}
	return token
}

func TestAuthenticationServiceLifecycle(t *testing.T) {
	t.Run("Authenticate_root", func(t *testing.T) {
		_ = mustAuth(t)
	})

	t.Run("Validate_and_Refresh_token", func(t *testing.T) {
		token := mustAuth(t)

		// If your client exposes ValidateToken and RefreshToken, test them; otherwise skip safely.
		if validate := client.ValidateToken; validate != nil {
			_, _, err := client.ValidateToken(token)
			if err != nil {
				t.Fatalf("ValidateToken failed: %v", err)
			}
		}

		if refresh := client.RefreshToken; refresh != nil {
			newTok, err := client.RefreshToken(token)
			if err != nil {
				t.Fatalf("RefreshToken failed: %v", err)
			}
			if newTok == "" {
				t.Fatalf("RefreshToken returned empty token")
			}
			_ = client.SetToken(newTok)
		}
	})

	t.Run("SetRootPassword_noop", func(t *testing.T) {
		_ = mustAuth(t) // ensure token metadata is present
		// no-op change: old == new
		if _, err := client.SetRootPassword(saPwd, saPwd); err != nil {
			t.Fatalf("SetRootPassword(no-op) failed: %v", err)
		}
	})

	t.Run("RotateRootPassword_and_revert_if_enabled", func(t *testing.T) {
		if !rotateOK {
			t.Skip("Skipping password rotation (set AUTH_TEST_ALLOW_ROTATE=true to enable)")
		}

		// 1) auth with current password
		_ = mustAuth(t)

		// 2) change to a temporary password
		tmp := saPwd + "_tmp123!"
		if _, err := client.SetRootPassword(saPwd, tmp); err != nil {
			t.Fatalf("SetRootPassword -> temp failed: %v", err)
		}

		// 3) authenticate with the new password and stash token
		{
			token, err := client.Authenticate(saUser, tmp)
			if err != nil {
				t.Fatalf("Authenticate with temp password failed: %v", err)
			}
			if err := client.SetToken(token); err != nil {
				t.Fatalf("SetToken(temp) failed: %v", err)
			}
		}

		// 4) change it back to the original
		if _, err := client.SetRootPassword(tmp, saPwd); err != nil {
			t.Fatalf("SetRootPassword revert failed: %v", err)
		}

		// 5) final sanity check: login with original works
		if _, err := client.Authenticate(saUser, saPwd); err != nil {
			t.Fatalf("Authenticate with original after revert failed: %v", err)
		}
	})

	t.Run("Logout_if_supported", func(t *testing.T) {
		if logout := client.Logout; logout == nil {
			t.Skip("Client has no Logout; skipping")
		}
		_ = mustAuth(t)
		if err := client.Logout(); err != nil {
			t.Fatalf("Logout failed: %v", err)
		}
	})
}

// Optional: quick benchmark for baseline token issuance perf
func BenchmarkAuthenticate(b *testing.B) {
	// Use env-driven creds; fail fast if auth breaks
	for n := 0; n < b.N; n++ {
		token, err := client.Authenticate(saUser, saPwd)
		if err != nil {
			b.Fatalf("Authenticate failed: %v", err)
		}
		if token == "" {
			b.Fatalf("Authenticate returned empty token")
		}
	}
	log.Println("BenchmarkAuthenticate completed")
}
