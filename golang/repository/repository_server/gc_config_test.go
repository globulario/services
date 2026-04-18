package main

import "testing"

// ── reachabilityConfig ────────────────────────────────────────────────────

func TestReachabilityConfig_DefaultsTo3WhenZero(t *testing.T) {
	srv := &server{GCRetentionWindow: 0}
	cfg := srv.reachabilityConfig()
	if cfg.RetentionWindow != defaultRetentionWindow {
		t.Errorf("RetentionWindow = %d, want %d (defaultRetentionWindow)", cfg.RetentionWindow, defaultRetentionWindow)
	}
}

func TestReachabilityConfig_DefaultsTo3WhenNegative(t *testing.T) {
	srv := &server{GCRetentionWindow: -5}
	cfg := srv.reachabilityConfig()
	if cfg.RetentionWindow != defaultRetentionWindow {
		t.Errorf("RetentionWindow = %d, want %d (defaultRetentionWindow)", cfg.RetentionWindow, defaultRetentionWindow)
	}
}

func TestReachabilityConfig_RespectsConfiguredWindow(t *testing.T) {
	for _, w := range []int{1, 5, 10} {
		srv := &server{GCRetentionWindow: w}
		cfg := srv.reachabilityConfig()
		if cfg.RetentionWindow != w {
			t.Errorf("GCRetentionWindow=%d: RetentionWindow = %d, want %d", w, cfg.RetentionWindow, w)
		}
	}
}

// ── Config struct ─────────────────────────────────────────────────────────

func TestConfig_DefaultGCRetentionWindow(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.GCRetentionWindow != defaultRetentionWindow {
		t.Errorf("DefaultConfig().GCRetentionWindow = %d, want %d", cfg.GCRetentionWindow, defaultRetentionWindow)
	}
}

func TestConfig_Clone_PreservesGCRetentionWindow(t *testing.T) {
	original := DefaultConfig()
	original.GCRetentionWindow = 7
	clone := original.Clone()
	if clone.GCRetentionWindow != 7 {
		t.Errorf("Clone().GCRetentionWindow = %d, want 7", clone.GCRetentionWindow)
	}
}
