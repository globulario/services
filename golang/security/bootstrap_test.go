package security

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/peer"
)

// TestBootstrapGate_Disabled verifies that bootstrap is denied when not enabled
func TestBootstrapGate_Disabled(t *testing.T) {
	gate := NewBootstrapGate()
	gate.flagFilePath = "/tmp/nonexistent-bootstrap-flag"

	// Ensure env var is not set
	os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/SetRoleBinding",
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false when bootstrap not enabled")
	}
	if reason != "bootstrap_not_enabled" {
		t.Errorf("reason = %q, want \"bootstrap_not_enabled\"", reason)
	}
}

// TestBootstrapGate_EnvVar_NoLongerWorks verifies env var no longer enables bootstrap
func TestBootstrapGate_EnvVar_NoLongerWorks(t *testing.T) {
	gate := NewBootstrapGate()
	gate.flagFilePath = "/tmp/nonexistent-bootstrap-flag"

	// Env var should NOT enable bootstrap (removed to prevent permanent insecurity)
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/SetRoleBinding",
		IsLoopback: true,
	}

	allowed, _ := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false — env var should no longer enable bootstrap")
	}
}

// createTestBootstrapFlag creates a valid flag file for test use and returns
// a gate configured to use it.
func createTestBootstrapFlag(t *testing.T) *BootstrapGate {
	t.Helper()
	tmpDir := t.TempDir()
	flagFile := filepath.Join(tmpDir, "bootstrap.enabled")
	now := time.Now().Unix()
	state := BootstrapState{
		EnabledAt: now,
		ExpiresAt: now + 1800,
		Nonce:     "test-nonce",
		CreatedBy: "test",
		Version:   "1.0",
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal bootstrap state: %v", err)
	}
	if err := os.WriteFile(flagFile, data, 0600); err != nil {
		t.Fatalf("write bootstrap flag: %v", err)
	}
	gate := NewBootstrapGate()
	gate.flagFilePath = flagFile
	gate.skipOwnershipCheck = true
	return gate
}

// TestBootstrapGate_FlagFile verifies bootstrap works with flag file
func TestBootstrapGate_FlagFile(t *testing.T) {
	// Security Fix #4: Create temp flag file with valid JSON state
	tmpDir := t.TempDir()
	flagFile := filepath.Join(tmpDir, "bootstrap.enabled")

	// Create valid bootstrap state (within time window)
	now := time.Now().Unix()
	state := BootstrapState{
		EnabledAt:  now,
		ExpiresAt:  now + 1800, // 30 minutes from now
		Nonce:      "test-nonce",
		CreatedBy:  "test",
		Version:    "1.0",
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}

	// Write with correct permissions (0600)
	if err := os.WriteFile(flagFile, stateJSON, 0600); err != nil {
		t.Fatalf("failed to create flag file: %v", err)
	}

	gate := NewBootstrapGate()
	gate.flagFilePath = flagFile
	gate.skipOwnershipCheck = true // Test mode: allow non-root ownership

	// Ensure env var is not set
	os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/SetRoleBinding",
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if !allowed {
		t.Errorf("ShouldAllow() = false, want true with flag file (reason: %s)", reason)
	}
	if reason != "bootstrap_allowed" {
		t.Errorf("reason = %q, want \"bootstrap_allowed\"", reason)
	}
}

// TestBootstrapGate_Expired verifies time window enforcement
func TestBootstrapGate_Expired(t *testing.T) {
	// Security Fix #4: Create temp flag file with expired JSON state
	tmpDir := t.TempDir()
	flagFile := filepath.Join(tmpDir, "bootstrap.enabled")

	// Create bootstrap state that expired 1 minute ago
	now := time.Now().Unix()
	state := BootstrapState{
		EnabledAt:  now - 1860, // 31 minutes ago
		ExpiresAt:  now - 60,   // 1 minute ago (EXPIRED)
		Nonce:      "test-nonce",
		CreatedBy:  "test",
		Version:    "1.0",
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}

	// Write with correct permissions (0600)
	if err := os.WriteFile(flagFile, stateJSON, 0600); err != nil {
		t.Fatalf("failed to create flag file: %v", err)
	}

	gate := NewBootstrapGate()
	gate.flagFilePath = flagFile
	gate.skipOwnershipCheck = true // Test mode: allow non-root ownership

	// Ensure env var is not set
	os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/SetRoleBinding",
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false for expired bootstrap")
	}
	if reason != "bootstrap_expired" {
		t.Errorf("reason = %q, want \"bootstrap_expired\"", reason)
	}
}

// TestBootstrapGate_NonLoopback verifies loopback-only enforcement
func TestBootstrapGate_NonLoopback(t *testing.T) {
	gate := createTestBootstrapFlag(t)

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/SetRoleBinding",
		IsLoopback: false, // Remote request
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false for non-loopback request")
	}
	if reason != "bootstrap_remote" {
		t.Errorf("reason = %q, want \"bootstrap_remote\"", reason)
	}
}

// TestBootstrapGate_MethodNotAllowed verifies method allowlist enforcement
func TestBootstrapGate_MethodNotAllowed(t *testing.T) {
	gate := createTestBootstrapFlag(t)

	// Try to access a non-allowlisted method
	authCtx := &AuthContext{
		GRPCMethod: "/file.FileService/DeleteFile", // NOT in allowlist
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false for non-allowlisted method")
	}
	if reason != "bootstrap_method_blocked" {
		t.Errorf("reason = %q, want \"bootstrap_method_blocked\"", reason)
	}
}

// TestBootstrapGate_AllowedMethods verifies that all allowlisted methods work
func TestBootstrapGate_AllowedMethods(t *testing.T) {
	gate := createTestBootstrapFlag(t)

	allowedMethods := []string{
		"/grpc.health.v1.Health/Check",
		"/rbac.RbacService/SetRoleBinding",
		"/rbac.RbacService/ListRoleBindings",
		"/authentication.AuthenticationService/Authenticate",
		"/resource.ResourceService/UpsertNodeIdentity",
		"/dns.DnsService/CreateZone",
	}

	for _, method := range allowedMethods {
		t.Run(method, func(t *testing.T) {
			authCtx := &AuthContext{
				GRPCMethod: method,
				IsLoopback: true,
			}

			allowed, reason := gate.ShouldAllow(authCtx)
			if !allowed {
				t.Errorf("ShouldAllow() = false for allowed method %q (reason: %s)", method, reason)
			}
		})
	}
}

// TestBootstrapGate_FourGatesOrdered verifies gates are checked in order
func TestBootstrapGate_FourGatesOrdered(t *testing.T) {
	tests := []struct {
		name           string
		setupGate      func(*BootstrapGate)
		setupEnv       func()
		cleanupEnv     func()
		authCtx        *AuthContext
		wantAllowed    bool
		wantReason     string
		description    string
	}{
		{
			name: "Gate 1 fails - not enabled",
			setupGate: func(g *BootstrapGate) {
				g.flagFilePath = "/tmp/nonexistent"
			},
			setupEnv:   func() { os.Unsetenv("GLOBULAR_BOOTSTRAP") },
			cleanupEnv: func() {},
			authCtx: &AuthContext{
				GRPCMethod: "/rbac.RbacService/SetRoleBinding",
				IsLoopback: true,
			},
			wantAllowed: false,
			wantReason:  "bootstrap_not_enabled",
			description: "Should fail at gate 1 (enablement)",
		},
		{
			name: "Gate 2 fails - expired",
			setupGate: func(g *BootstrapGate) {
				// Create expired flag file
				tmpDir := os.TempDir()
				flagFile := filepath.Join(tmpDir, "test-bootstrap-expired")
				os.WriteFile(flagFile, []byte(""), 0644)
				oldTime := time.Now().Add(-31 * time.Minute)
				os.Chtimes(flagFile, oldTime, oldTime)
				g.flagFilePath = flagFile
			},
			setupEnv:   func() { os.Unsetenv("GLOBULAR_BOOTSTRAP") },
			cleanupEnv: func() {},
			authCtx: &AuthContext{
				GRPCMethod: "/rbac.RbacService/SetRoleBinding",
				IsLoopback: true,
			},
			wantAllowed: false,
			wantReason:  "bootstrap_expired",
			description: "Should fail at gate 2 (time window)",
		},
		{
			name: "Gate 3 fails - remote",
			setupGate: func(g *BootstrapGate) {
				// Valid flag file, but request is remote
				tmpDir, _ := os.MkdirTemp("", "bootstrap-test-*")
				flagFile := filepath.Join(tmpDir, "bootstrap.enabled")
				now := time.Now().Unix()
				data, _ := json.Marshal(BootstrapState{EnabledAt: now, ExpiresAt: now + 1800, CreatedBy: "test", Version: "1.0"})
				os.WriteFile(flagFile, data, 0600)
				g.flagFilePath = flagFile
				g.skipOwnershipCheck = true
			},
			setupEnv:   func() {},
			cleanupEnv: func() {},
			authCtx: &AuthContext{
				GRPCMethod: "/rbac.RbacService/SetRoleBinding",
				IsLoopback: false, // Remote!
			},
			wantAllowed: false,
			wantReason:  "bootstrap_remote",
			description: "Should fail at gate 3 (loopback-only)",
		},
		{
			name: "Gate 4 fails - method blocked",
			setupGate: func(g *BootstrapGate) {
				tmpDir, _ := os.MkdirTemp("", "bootstrap-test-*")
				flagFile := filepath.Join(tmpDir, "bootstrap.enabled")
				now := time.Now().Unix()
				data, _ := json.Marshal(BootstrapState{EnabledAt: now, ExpiresAt: now + 1800, CreatedBy: "test", Version: "1.0"})
				os.WriteFile(flagFile, data, 0600)
				g.flagFilePath = flagFile
				g.skipOwnershipCheck = true
			},
			setupEnv:   func() {},
			cleanupEnv: func() {},
			authCtx: &AuthContext{
				GRPCMethod: "/file.FileService/DeleteFile", // Not allowed!
				IsLoopback: true,
			},
			wantAllowed: false,
			wantReason:  "bootstrap_method_blocked",
			description: "Should fail at gate 4 (method allowlist)",
		},
		{
			name: "All gates pass",
			setupGate: func(g *BootstrapGate) {
				tmpDir, _ := os.MkdirTemp("", "bootstrap-test-*")
				flagFile := filepath.Join(tmpDir, "bootstrap.enabled")
				now := time.Now().Unix()
				data, _ := json.Marshal(BootstrapState{EnabledAt: now, ExpiresAt: now + 1800, CreatedBy: "test", Version: "1.0"})
				os.WriteFile(flagFile, data, 0600)
				g.flagFilePath = flagFile
				g.skipOwnershipCheck = true
			},
			setupEnv:   func() {},
			cleanupEnv: func() {},
			authCtx: &AuthContext{
				GRPCMethod: "/rbac.RbacService/SetRoleBinding",
				IsLoopback: true,
			},
			wantAllowed: true,
			wantReason:  "bootstrap_allowed",
			description: "Should pass all 4 gates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gate := NewBootstrapGate()
			tt.setupGate(gate)
			tt.setupEnv()
			defer tt.cleanupEnv()

			allowed, reason := gate.ShouldAllow(tt.authCtx)

			if allowed != tt.wantAllowed {
				t.Errorf("%s: allowed = %v, want %v", tt.description, allowed, tt.wantAllowed)
			}
			if reason != tt.wantReason {
				t.Errorf("%s: reason = %q, want %q", tt.description, reason, tt.wantReason)
			}
		})
	}
}

// TestBootstrapGate_GetBootstrapStatus verifies status reporting
func TestBootstrapGate_GetBootstrapStatus(t *testing.T) {
	gate := NewBootstrapGate()
	gate.flagFilePath = "/tmp/nonexistent"

	// Disabled
	status := gate.GetBootstrapStatus()
	if status != "disabled" {
		t.Errorf("GetBootstrapStatus() = %q, want \"disabled\"", status)
	}

	// Enabled via flag file
	gate2 := createTestBootstrapFlag(t)
	status = gate2.GetBootstrapStatus()
	if !strings.HasPrefix(status, "enabled (flag_file)") {
		t.Errorf("GetBootstrapStatus() = %q, want prefix \"enabled (flag_file)\"", status)
	}
}

// TestBootstrapGate_CorruptGateFile verifies that malformed, partial, or
// structurally invalid gate files are rejected — the gate fails closed.
func TestBootstrapGate_CorruptGateFile(t *testing.T) {
	now := time.Now().Unix()

	cases := []struct {
		name    string
		content []byte
	}{
		{"empty file", []byte{}},
		{"plain text (old format)", []byte("2026-04-18T12:00:00Z\n")},
		{"not JSON", []byte("not-json\n")},
		{"truncated JSON", []byte(`{"enabled_at_unix": 1`)},
		{"missing enabled_at", []byte(`{"expires_at_unix": 9999999999, "version": "1", "created_by": "x"}`)},
		{"missing expires_at", []byte(`{"enabled_at_unix": 1, "version": "1", "created_by": "x"}`)},
		{"expires_before_enabled", []byte(`{"enabled_at_unix": 1000, "expires_at_unix": 500, "version": "1", "created_by": "x"}`)},
		{"zero timestamps", []byte(`{"enabled_at_unix": 0, "expires_at_unix": 0, "version": "1"}`)},
		{"future enabled_at", func() []byte {
			data, _ := json.Marshal(BootstrapState{
				EnabledAt: now + 3600, // 1 hour in the future
				ExpiresAt: now + 7200,
				CreatedBy: "x",
				Version:   "1",
			})
			return data
		}()},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			flagFile := filepath.Join(tmpDir, "bootstrap.enabled")
			if err := os.WriteFile(flagFile, c.content, 0600); err != nil {
				t.Fatalf("write: %v", err)
			}

			gate := NewBootstrapGateWithPath(flagFile)
			gate.SetSkipOwnershipCheck(true)

			// isWithinTimeWindow must return false — gate fails closed on bad state
			if gate.isWithinTimeWindow() {
				t.Errorf("isWithinTimeWindow() = true for %q — security gap", c.name)
			}

			// ShouldAllow must also deny (isEnabled passes since file exists,
			// then isWithinTimeWindow fails → bootstrap_expired reason)
			authCtx := &AuthContext{
				GRPCMethod: "/rbac.RbacService/SetRoleBinding",
				IsLoopback: true,
			}
			allowed, reason := gate.ShouldAllow(authCtx)
			if allowed {
				t.Errorf("ShouldAllow() = true for corrupt file %q — security gap", c.name)
			}
			if reason != "bootstrap_expired" {
				t.Errorf("reason = %q for %q, want \"bootstrap_expired\"", reason, c.name)
			}
		})
	}
}

// TestEnableBootstrapGate_RoundTrip verifies that EnableBootstrapGate writes a file
// that readBootstrapState can parse and isWithinTimeWindow accepts.
func TestEnableBootstrapGate_RoundTrip(t *testing.T) {
	tests := []struct {
		name          string
		ttl           time.Duration
		createdBy     string
		wantActive    bool
		wantReason    string
		adjustState   func(path string) // optional: corrupt/expire the file after writing
	}{
		{
			name:       "valid TTL → bootstrap active",
			ttl:        30 * time.Minute,
			createdBy:  "node-agent-day0",
			wantActive: true,
			wantReason: "bootstrap_allowed",
		},
		{
			name:      "expired → denied",
			ttl:       30 * time.Minute,
			createdBy: "node-agent-day0",
			// Overwrite the file with an already-expired state after writing
			adjustState: func(path string) {
				now := time.Now().Unix()
				state := BootstrapState{
					EnabledAt: now - 1860,
					ExpiresAt: now - 60,
					CreatedBy: "node-agent-day0",
					Version:   "1",
				}
				data, _ := json.Marshal(state)
				os.WriteFile(path, data, 0600)
			},
			wantActive: false,
			wantReason: "bootstrap_expired",
		},
		{
			name:      "missing file → denied",
			ttl:       30 * time.Minute,
			createdBy: "node-agent-day0",
			// Remove the file after writing
			adjustState: func(path string) { os.Remove(path) },
			wantActive: false,
			wantReason: "bootstrap_not_enabled",
		},
		{
			name:      "wrong caller identity → path-2 blocked",
			ttl:       30 * time.Minute,
			createdBy: "node-agent-day0",
			wantActive: true, // gate is active …
			wantReason: "bootstrap_allowed", // … and loopback caller still passes gate
			// Subject-level check is in requireAdmin, not in ShouldAllow — tested separately below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			flagFile := filepath.Join(tmpDir, "bootstrap.enabled")

			// Write using the internal helper (same code path as EnableBootstrapGate)
			if err := writeBootstrapStateTo(flagFile, tt.ttl, tt.createdBy); err != nil {
				t.Fatalf("writeBootstrapStateTo: %v", err)
			}

			if tt.adjustState != nil {
				tt.adjustState(flagFile)
			}

			gate := NewBootstrapGateWithPath(flagFile)
			gate.SetSkipOwnershipCheck(true)

			authCtx := &AuthContext{
				GRPCMethod: "/rbac.RbacService/SetRoleBinding",
				IsLoopback: true,
			}
			allowed, reason := gate.ShouldAllow(authCtx)

			if allowed != tt.wantActive {
				t.Errorf("ShouldAllow() = %v, want %v", allowed, tt.wantActive)
			}
			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

// TestIsBootstrapSubject verifies the subject allowlist for path-2 bootstrap access.
func TestIsBootstrapSubject(t *testing.T) {
	allowed := []string{"globular-node-agent", "globular-controller", "globular-gateway"}
	for _, s := range allowed {
		if !IsBootstrapSubject(s) {
			t.Errorf("IsBootstrapSubject(%q) = false, want true", s)
		}
	}

	denied := []string{"", "attacker", "globular-admin", "root", "anonymous", "globular-node-agent-sa"}
	for _, s := range denied {
		if IsBootstrapSubject(s) {
			t.Errorf("IsBootstrapSubject(%q) = true, want false", s)
		}
	}
}

// TestBootstrapGate_IntegrationWithAuthContext verifies end-to-end flow
func TestBootstrapGate_IntegrationWithAuthContext(t *testing.T) {
	// Enable bootstrap mode via flag file
	gate := createTestBootstrapFlag(t)

	// Create loopback peer context
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	// Create AuthContext
	authCtx, err := NewAuthContext(ctx, "/rbac.RbacService/SetRoleBinding")
	if err != nil {
		t.Fatalf("NewAuthContext() error = %v", err)
	}

	// Verify loopback detected
	if !authCtx.IsLoopback {
		t.Error("AuthContext.IsLoopback = false, want true for 127.0.0.1")
	}

	// Check bootstrap gate
	allowed, reason := gate.ShouldAllow(authCtx)
	if !allowed {
		t.Errorf("ShouldAllow() = false, want true (reason: %s)", reason)
	}
	if reason != "bootstrap_allowed" {
		t.Errorf("reason = %q, want \"bootstrap_allowed\"", reason)
	}
}
