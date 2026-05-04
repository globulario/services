package main

import "testing"

func TestSanitizePoolNodes_FiltersStaleAndPreservesOrder(t *testing.T) {
	pool := []string{"10.0.0.63", "10.0.0.9", "10.0.0.20", "10.0.0.63"}
	nodes := map[string]ccNodeLite{
		"ryzen": {
			Status: "healthy",
			Identity: struct {
				Ips []string `json:"ips"`
			}{Ips: []string{"10.0.0.63"}},
		},
		"dell": {
			Status: "active",
			Identity: struct {
				Ips []string `json:"ips"`
			}{Ips: []string{"10.0.0.20"}},
		},
	}
	got := sanitizePoolNodes(pool, nodes)
	if len(got) != 2 || got[0] != "10.0.0.63" || got[1] != "10.0.0.20" {
		t.Fatalf("unexpected sanitized pool: %v", got)
	}
}

func TestSanitizePoolNodes_EmptyNodesFallbackToValidIPs(t *testing.T) {
	pool := []string{"10.0.0.63", "bad-hostname", "10.0.0.20", "10.0.0.63"}
	got := sanitizePoolNodes(pool, nil)
	if len(got) != 2 || got[0] != "10.0.0.63" || got[1] != "10.0.0.20" {
		t.Fatalf("unexpected fallback sanitized pool: %v", got)
	}
}

