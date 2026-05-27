package main

import "testing"

func TestAllowUnverifiedFallback_DefaultFalse(t *testing.T) {
	t.Setenv(mcpAllowUnverifiedFallbackEnv, "")
	if allowUnverifiedFallback() {
		t.Fatalf("allowUnverifiedFallback should be false by default")
	}
}

func TestAllowUnverifiedFallback_TrueValues(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv(mcpAllowUnverifiedFallbackEnv, v)
		if !allowUnverifiedFallback() {
			t.Fatalf("expected true for %q", v)
		}
	}
}

