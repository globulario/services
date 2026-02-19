package main

import (
	"testing"
)

// TestCanonicalizeAction verifies path normalization for security
// Security Fix #6: Prevent bypass via path tricks
func TestCanonicalizeAction(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		wantOk    bool
		wantClean string
	}{
		{
			name:      "valid gRPC method",
			action:    "/rbac.RbacService/CreateAccount",
			wantOk:    true,
			wantClean: "/rbac.RbacService/CreateAccount",
		},
		{
			name:      "global wildcard",
			action:    "/*",
			wantOk:    true,
			wantClean: "/*",
		},
		{
			name:      "service wildcard",
			action:    "/rbac.RbacService/*",
			wantOk:    true,
			wantClean: "/rbac.RbacService/*",
		},
		{
			name:      "double slash normalized",
			action:    "/rbac.RbacService//CreateAccount",
			wantOk:    true,
			wantClean: "/rbac.RbacService/CreateAccount",
		},
		{
			name:      "triple slash normalized",
			action:    "/rbac.RbacService///CreateAccount",
			wantOk:    true,
			wantClean: "/rbac.RbacService/CreateAccount",
		},
		{
			name:   "relative path component rejected",
			action: "/rbac.RbacService/./CreateAccount",
			wantOk: false,
		},
		{
			name:   "parent directory rejected",
			action: "/rbac.RbacService/../dns.DnsService/CreateZone",
			wantOk: false,
		},
		{
			name:   "null byte rejected",
			action: "/rbac.RbacService/Create\x00Account",
			wantOk: false,
		},
		{
			name:   "missing leading slash rejected",
			action: "rbac.RbacService/CreateAccount",
			wantOk: false,
		},
		{
			name:   "empty action rejected",
			action: "",
			wantOk: false,
		},
		{
			name:   "newline rejected",
			action: "/rbac.RbacService/Create\nAccount",
			wantOk: false,
		},
		{
			name:   "tab rejected",
			action: "/rbac.RbacService/Create\tAccount",
			wantOk: false,
		},
		{
			name:   "too many segments rejected",
			action: "/rbac/RbacService/CreateAccount/Extra",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clean, err := canonicalizeAction(tt.action)

			if tt.wantOk {
				if err != nil {
					t.Errorf("canonicalizeAction() error = %v, want nil", err)
					return
				}
				if clean != tt.wantClean {
					t.Errorf("canonicalizeAction() = %q, want %q", clean, tt.wantClean)
				}
			} else {
				if err == nil {
					t.Errorf("canonicalizeAction() succeeded with %q, want error", clean)
				}
			}
		})
	}
}

// TestMatchesAction verifies wildcard matching and bypass prevention
// Security Fix #6: Ensure no bypass via path normalization
func TestMatchesAction(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		action  string
		want    bool
	}{
		// Exact matches
		{
			name:    "exact match",
			pattern: "/rbac.RbacService/CreateAccount",
			action:  "/rbac.RbacService/CreateAccount",
			want:    true,
		},
		{
			name:    "different methods no match",
			pattern: "/rbac.RbacService/CreateAccount",
			action:  "/rbac.RbacService/DeleteAccount",
			want:    false,
		},

		// Global wildcard
		{
			name:    "global wildcard matches all",
			pattern: "/*",
			action:  "/rbac.RbacService/CreateAccount",
			want:    true,
		},
		{
			name:    "global wildcard matches any service",
			pattern: "/*",
			action:  "/dns.DnsService/CreateZone",
			want:    true,
		},

		// Service wildcard
		{
			name:    "service wildcard matches method in service",
			pattern: "/rbac.RbacService/*",
			action:  "/rbac.RbacService/CreateAccount",
			want:    true,
		},
		{
			name:    "service wildcard matches another method",
			pattern: "/rbac.RbacService/*",
			action:  "/rbac.RbacService/DeleteAccount",
			want:    true,
		},
		{
			name:    "service wildcard doesn't match other service",
			pattern: "/rbac.RbacService/*",
			action:  "/dns.DnsService/CreateZone",
			want:    false,
		},

		// Security Fix #6: Bypass prevention
		{
			name:    "double slash normalized and matched",
			pattern: "/rbac.RbacService/CreateAccount",
			action:  "/rbac.RbacService//CreateAccount",
			want:    true,
		},
		{
			name:    "relative path rejected (no match)",
			pattern: "/rbac.RbacService/CreateAccount",
			action:  "/rbac.RbacService/./CreateAccount",
			want:    false,
		},
		{
			name:    "parent directory rejected (no match)",
			pattern: "/rbac.RbacService/CreateAccount",
			action:  "/rbac.RbacService/../rbac.RbacService/CreateAccount",
			want:    false,
		},
		{
			name:    "null byte rejected (no match)",
			pattern: "/rbac.RbacService/CreateAccount",
			action:  "/rbac.RbacService/Create\x00Account",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAction(tt.pattern, tt.action)
			if got != tt.want {
				t.Errorf("matchesAction(%q, %q) = %v, want %v",
					tt.pattern, tt.action, got, tt.want)
			}
		})
	}
}

// TestMatchesAction_InvalidPatterns verifies invalid patterns are rejected
func TestMatchesAction_InvalidPatterns(t *testing.T) {
	invalidPatterns := []string{
		"",                                    // empty
		"rbac.RbacService/CreateAccount",     // no leading slash
		"/rbac.RbacService/../CreateAccount", // traversal
		"/rbac.RbacService/./CreateAccount",  // relative
	}

	validAction := "/rbac.RbacService/CreateAccount"

	for _, pattern := range invalidPatterns {
		t.Run("pattern="+pattern, func(t *testing.T) {
			// Invalid patterns should return false (fail closed)
			got := matchesAction(pattern, validAction)
			if got {
				t.Errorf("matchesAction(%q, %q) = true, want false (invalid pattern should fail closed)",
					pattern, validAction)
			}
		})
	}
}
