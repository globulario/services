package main

// version_regression_test.go — Regression for
// docs/awareness/failure_modes.yaml#service.runtime_version_empty_from_main.
//
// Before 2026-05-21, DefaultConfig() set Version: "" — wiping the
// ldflags-injected build-time Version variable. The verifier surfaced
// service.runtime_identity_unproven on every Day-0 boot of torrent.
// This test pins the contract: DefaultConfig().Version must come from
// the package-level Version variable.

import "testing"

func TestDefaultConfig_VersionFromBuildTime(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != Version {
		t.Errorf("DefaultConfig().Version = %q, want package Version %q — Day-0 verifier will flag runtime_identity_unproven", cfg.Version, Version)
	}
	if cfg.Version == "" {
		t.Fatal("DefaultConfig().Version is empty — ldflags pipeline broken; binary would report empty version via --describe and trigger service.runtime_version_empty_from_main")
	}
}
