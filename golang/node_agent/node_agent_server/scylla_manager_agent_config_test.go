package main

import "testing"

func TestHasNonEmptyAuthToken(t *testing.T) {
	if hasNonEmptyAuthToken("auth_token: abc123\n") != true {
		t.Fatal("expected true for non-empty token")
	}
	if hasNonEmptyAuthToken("auth_token: \n") != false {
		t.Fatal("expected false for empty token")
	}
	if hasNonEmptyAuthToken("# comment\nauth_token: \"tok\"\n") != true {
		t.Fatal("expected true for quoted token")
	}
}

func TestUpsertAuthToken(t *testing.T) {
	const token = "newtoken"

	got := upsertAuthToken("https: :10001\n", token)
	if !hasNonEmptyAuthToken(got) {
		t.Fatalf("upsert should add auth_token, got:\n%s", got)
	}

	got = upsertAuthToken("auth_token: old\nhttps: :10001\n", token)
	if got == "" || got[:len("auth_token: newtoken")] != "auth_token: newtoken" {
		t.Fatalf("upsert should replace existing token, got:\n%s", got)
	}
}

func TestHasScyllaAPIURL(t *testing.T) {
	cases := []struct {
		content    string
		expectedIP string
		want       bool
	}{
		{"scylla:\n  api_address: 10.0.0.20\n  api_port: 10000\n", "10.0.0.20", true},
		{"scylla:\n  api_address: 10.0.0.20\n  api_port: 10000\n", "10.0.0.8", false},
		{"scylla:\n  api_address: 10.0.0.20\n  api_port: 10000\n", "", true},
		{"scylla:\n  api_address: \n", "10.0.0.20", false},
		{"auth_token: abc\n", "10.0.0.20", false},
	}
	for _, tc := range cases {
		if got := hasScyllaAPIURL(tc.content, tc.expectedIP); got != tc.want {
			t.Errorf("hasScyllaAPIURL(%q, %q) = %v, want %v", tc.content, tc.expectedIP, got, tc.want)
		}
	}
}

func TestUpsertScyllaAPIURL(t *testing.T) {
	const ip = "10.0.0.20"
	const wantBlock = "scylla:\n  api_address: 10.0.0.20\n  api_port: 10000"

	got := upsertScyllaAPIURL("auth_token: abc\n", ip)
	if !contains(got, wantBlock) {
		t.Fatalf("expected scylla block in output, got:\n%s", got)
	}

	// Legacy top-level api_url should be stripped
	got = upsertScyllaAPIURL("api_url: http://0.0.0.0:10000\nauth_token: abc\n", ip)
	if !contains(got, wantBlock) {
		t.Fatalf("expected scylla block in output, got:\n%s", got)
	}
	if contains(got, "api_url:") {
		t.Fatalf("legacy api_url still present in output:\n%s", got)
	}
}

func TestHasScyllaAgentPorts(t *testing.T) {
	const ip = "10.0.0.8"
	wantHTTPS := "https: 10.0.0.8:56001"
	wantProm := "prometheus: :56002"
	wantDebug := "debug: 127.0.0.1:56003"
	full := wantHTTPS + "\n" + wantProm + "\n" + wantDebug + "\n"

	if !hasScyllaAgentPorts(full, ip) {
		t.Fatal("expected true when all ports present")
	}
	if hasScyllaAgentPorts("auth_token: abc\n", ip) {
		t.Fatal("expected false when ports absent")
	}
	if hasScyllaAgentPorts(wantHTTPS+"\n"+wantProm+"\n", ip) {
		t.Fatal("expected false when debug missing")
	}
	if hasScyllaAgentPorts(wantHTTPS+"\n"+wantDebug+"\n", ip) {
		t.Fatal("expected false when prometheus missing")
	}
	// wrong IP
	if hasScyllaAgentPorts("https: 10.0.0.63:56001\nprometheus: :56002\ndebug: 127.0.0.1:56003\n", ip) {
		t.Fatal("expected false for wrong IP")
	}
}

func TestUpsertScyllaAgentPorts(t *testing.T) {
	const ip = "10.0.0.8"

	// Fresh config — all ports appended
	got := upsertScyllaAgentPorts("auth_token: abc\n", ip)
	if !hasScyllaAgentPorts(got, ip) {
		t.Fatalf("expected all ports in output, got:\n%s", got)
	}

	// Replace existing conflicting ports
	existing := "https: 10.0.0.8:10001\nprometheus: :5090\ndebug: 127.0.0.1:5112\nauth_token: abc\n"
	got = upsertScyllaAgentPorts(existing, ip)
	if !hasScyllaAgentPorts(got, ip) {
		t.Fatalf("expected corrected ports, got:\n%s", got)
	}
	if contains(got, ":10001") {
		t.Fatalf("old https port still present:\n%s", got)
	}
	if contains(got, ":5090") {
		t.Fatalf("old prometheus port still present:\n%s", got)
	}
	if contains(got, ":5112") {
		t.Fatalf("old debug port still present:\n%s", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
