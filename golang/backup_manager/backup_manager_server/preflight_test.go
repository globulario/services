package main

import "testing"

// TestScyllaManagerAPIHost verifies the URL → host extraction used by the
// LAN-address rollout gate. The extraction must be robust to URL shapes that
// older configs and admin UIs may have persisted: full URL with scheme,
// host:port without scheme, bare host, ipv6 with brackets, and the legacy
// 127.0.0.1:5080 sentinel.
func TestScyllaManagerAPIHost(t *testing.T) {
	cases := map[string]string{
		"http://10.0.0.63:5080":          "10.0.0.63",
		"http://10.0.0.63:5080/api/v1":   "10.0.0.63",
		"https://10.0.0.63:5443":         "10.0.0.63",
		"https://10.0.0.63:5443/api/v1":  "10.0.0.63",
		"http://127.0.0.1:5080":          "127.0.0.1",
		"http://localhost:5080":          "localhost",
		"http://[::1]:5080":              "::1",
		"10.0.0.63:5080":                 "10.0.0.63",
		"10.0.0.63":                      "10.0.0.63",
		"":                               "",
		"   ":                            "",
		"globule-ryzen.globular.internal:5080": "globule-ryzen.globular.internal",
	}
	for in, want := range cases {
		if got := scyllaManagerAPIHost(in); got != want {
			t.Errorf("scyllaManagerAPIHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIsLegacyScyllaManagerDefault covers the four sentinel forms recognized
// as the bad UI default. Trailing slashes are normalized.
func TestIsLegacyScyllaManagerDefault(t *testing.T) {
	cases := map[string]bool{
		"http://127.0.0.1:5080":            true,
		"http://127.0.0.1:5080/":           true,
		"http://127.0.0.1:5080/api/v1":     true,
		"http://localhost:5080":            true,
		"http://localhost:5080/api/v1":     true,
		"http://10.0.0.63:5080":            false,
		"https://10.0.0.63:5443":           false,
		"":                                 false,
		"  http://127.0.0.1:5080  ":        true, // tolerates surrounding whitespace
	}
	for in, want := range cases {
		if got := isLegacyScyllaManagerDefault(in); got != want {
			t.Errorf("isLegacyScyllaManagerDefault(%q) = %v, want %v", in, got, want)
		}
	}
}
