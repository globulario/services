package main

import (
	"net"
	"testing"
)

func loopbackAPIURL(host string) string {
	return "http://" + net.JoinHostPort(host, "5080")
}

// TestScyllaManagerAPIHost verifies the URL → host extraction used by the
// LAN-address rollout gate. The extraction must be robust to URL shapes that
// older configs and admin UIs may have persisted: full URL with scheme,
// host:port without scheme, bare host, ipv6 with brackets, and loopback
// sentinels that must be rejected later.
func TestScyllaManagerAPIHost(t *testing.T) {
	cases := map[string]string{
		"http://10.0.0.63:5080":                     "10.0.0.63",
		"http://10.0.0.63:5080/api/v1":              "10.0.0.63",
		"https://10.0.0.63:5443":                    "10.0.0.63",
		"https://10.0.0.63:5443/api/v1":             "10.0.0.63",
		loopbackAPIURL("127.0.0.1"):                 "127.0.0.1",
		loopbackAPIURL("localhost"):                 "localhost",
		"http://" + net.JoinHostPort("::1", "5080"): "::1",
		"10.0.0.63:5080":                            "10.0.0.63",
		"10.0.0.63":                                 "10.0.0.63",
		"":                                          "",
		"   ":                                       "",
		"globule-ryzen.globular.internal:5080":      "globule-ryzen.globular.internal",
	}
	for in, want := range cases {
		if got := scyllaManagerAPIHost(in); got != want {
			t.Errorf("scyllaManagerAPIHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIsInvalidScyllaManagerAPIURL rejects loopback/non-LAN endpoints and
// accepts routable manager URLs.
func TestIsInvalidScyllaManagerAPIURL(t *testing.T) {
	cases := map[string]bool{
		loopbackAPIURL("127.0.0.1"):               true,
		loopbackAPIURL("127.0.0.1") + "/":         true,
		loopbackAPIURL("127.0.0.1") + "/api/v1":   true,
		loopbackAPIURL("localhost"):               true,
		loopbackAPIURL("localhost") + "/api/v1":   true,
		"http://0.0.0.0:5080":                     true,
		"http://169.254.1.10:5080":                true,
		"http://10.0.0.63:5080":                   false,
		"https://10.0.0.63:5443":                  false,
		"":                                        false,
		"  " + loopbackAPIURL("127.0.0.1") + "  ": true,
	}
	for in, want := range cases {
		if got := isInvalidScyllaManagerAPIURL(in); got != want {
			t.Errorf("isInvalidScyllaManagerAPIURL(%q) = %v, want %v", in, got, want)
		}
	}
}
