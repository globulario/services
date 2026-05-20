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

// TestHasScyllaAPIURL_RejectsDuplicateBlocks pins the regression where the
// agent yaml accumulated multiple scylla: blocks (e.g. one with the LAN IP,
// then trailing ones with docker0). YAML's last-wins meant the agent silently
// routed to an unreachable address, breaking scylla-manager → agent →
// ScyllaDB connectivity. hasScyllaAPIURL must report "needs rewrite" when
// duplicates exist so the next reconcile collapses them.
func TestHasScyllaAPIURL_RejectsDuplicateBlocks(t *testing.T) {
	corrupted := "scylla:\n  api_address: \"10.0.0.63\"\n  api_port: \"10000\"\n\n" +
		"scylla:\n  api_address: 172.17.0.1\n  api_port: 10000\n\n" +
		"scylla:\n  api_address: 172.17.0.1\n  api_port: 10000\n"
	if hasScyllaAPIURL(corrupted, "10.0.0.63") {
		t.Fatal("expected false when duplicate scylla: blocks exist — must trigger rewrite")
	}
}

// TestUpsertScyllaAPIURL_StripsDuplicates verifies the upsert collapses
// accumulated scylla: blocks into exactly one canonical block with the
// supplied IP. Without this, the agent reconcile loop would keep appending.
func TestUpsertScyllaAPIURL_StripsDuplicates(t *testing.T) {
	corrupted := "auth_token: abc\n" +
		"https: 10.0.0.63:56001\n" +
		"scylla:\n  api_address: \"10.0.0.63\"\n  api_port: \"10000\"\n\n" +
		"scylla:\n  api_address: 172.17.0.1\n  api_port: 10000\n\n" +
		"scylla:\n  api_address: 172.17.0.1\n  api_port: 10000\n"
	got := upsertScyllaAPIURL(corrupted, "10.0.0.63")

	blocks := extractScyllaBlocks(got)
	if len(blocks) != 1 {
		t.Fatalf("expected exactly 1 scylla: block after upsert, got %d:\n%s", len(blocks), got)
	}
	if blocks[0].apiAddress != "10.0.0.63" {
		t.Fatalf("expected api_address=10.0.0.63, got %q:\n%s", blocks[0].apiAddress, got)
	}
	if blocks[0].apiPort != "10000" {
		t.Fatalf("expected api_port=10000, got %q:\n%s", blocks[0].apiPort, got)
	}
	// And the result must now be detected as healthy (no more duplicates).
	if !hasScyllaAPIURL(got, "10.0.0.63") {
		t.Fatalf("after upsert hasScyllaAPIURL should return true, got false:\n%s", got)
	}
	// Preserve unrelated keys.
	if !contains(got, "auth_token: abc") {
		t.Fatalf("upsert should preserve unrelated keys, got:\n%s", got)
	}
	if !contains(got, "https: 10.0.0.63:56001") {
		t.Fatalf("upsert should preserve https: line, got:\n%s", got)
	}
}

func TestHasScyllaAgentPorts(t *testing.T) {
	const ip = "10.0.0.8"
	wantHTTPS := "https: " + ip + ":" + scyllaAgentHTTPSPort
	wantProm := "prometheus: :" + scyllaAgentPrometheusPort
	wantDebug := "debug: 127.0.0.1:" + scyllaAgentDebugPort
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
	wrongIP := "https: 10.0.0.63:" + scyllaAgentHTTPSPort +
		"\nprometheus: :" + scyllaAgentPrometheusPort +
		"\ndebug: 127.0.0.1:" + scyllaAgentDebugPort + "\n"
	if hasScyllaAgentPorts(wrongIP, ip) {
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

// TestScyllaAgentPortsBelowEphemeralRange pins the invariant that the agent
// ports must sit below Linux's ephemeral range (32768-60999). The original
// 56001-56003 choice silently raced against any local outbound connection
// that happened to grab 56002 as its source port — agent crashed with
// "bind: address already in use" on the first such race. Picking sub-32768
// ports is the only way to guarantee the agent can always bind.
func TestScyllaAgentPortsBelowEphemeralRange(t *testing.T) {
	const linuxEphemeralFloor = 32768
	atoi := func(s string) int {
		n := 0
		for _, c := range s {
			if c < '0' || c > '9' {
				t.Fatalf("port constant %q is not numeric", s)
			}
			n = n*10 + int(c-'0')
		}
		return n
	}
	for _, c := range []struct{ name, port string }{
		{"https", scyllaAgentHTTPSPort},
		{"prometheus", scyllaAgentPrometheusPort},
		{"debug", scyllaAgentDebugPort},
	} {
		if p := atoi(c.port); p >= linuxEphemeralFloor {
			t.Errorf("%s port %s is inside Linux ephemeral range (>=%d) — will race against outbound connections",
				c.name, c.port, linuxEphemeralFloor)
		}
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
