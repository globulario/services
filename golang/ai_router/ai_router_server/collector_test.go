package main

import (
	"os"
	"testing"
)

func TestPrometheusEndpointDefault(t *testing.T) {
	t.Setenv("PROMETHEUS_ENDPOINT", "")
	if got := prometheusEndpoint(); got != defaultPrometheusEndpoint {
		t.Fatalf("prometheusEndpoint() = %q, want %q", got, defaultPrometheusEndpoint)
	}
}

func TestPrometheusEndpointOverride(t *testing.T) {
	const override = "http://10.0.0.42:9191"
	t.Setenv("PROMETHEUS_ENDPOINT", override)
	if got := prometheusEndpoint(); got != override {
		t.Fatalf("prometheusEndpoint() = %q, want %q", got, override)
	}
}

func TestNewCollectorUsesPrometheusEndpoint(t *testing.T) {
	const override = "http://127.0.0.1:9191"
	t.Setenv("PROMETHEUS_ENDPOINT", override)

	c := newCollector()
	if c.promURL != override {
		t.Fatalf("collector promURL = %q, want %q", c.promURL, override)
	}
	if c.client == nil {
		t.Fatal("collector client is nil")
	}
}

func TestPrometheusEndpointIgnoresWhitespace(t *testing.T) {
	t.Setenv("PROMETHEUS_ENDPOINT", "   ")
	if got := prometheusEndpoint(); got != defaultPrometheusEndpoint {
		t.Fatalf("prometheusEndpoint() with whitespace override = %q, want %q", got, defaultPrometheusEndpoint)
	}
}

func TestPrometheusEndpointUsesProcessEnv(t *testing.T) {
	const override = "http://127.0.0.1:9393"
	old := os.Getenv("PROMETHEUS_ENDPOINT")
	t.Cleanup(func() {
		if old == "" {
			_ = os.Unsetenv("PROMETHEUS_ENDPOINT")
			return
		}
		_ = os.Setenv("PROMETHEUS_ENDPOINT", old)
	})
	_ = os.Setenv("PROMETHEUS_ENDPOINT", override)
	if got := prometheusEndpoint(); got != override {
		t.Fatalf("prometheusEndpoint() = %q, want %q", got, override)
	}
}
