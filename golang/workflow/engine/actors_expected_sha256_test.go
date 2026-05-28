package engine

// v1.2.119 — actor-level regression tests for the
// controller.apply_package_release_must_carry_expected_sha256 invariant.
//
// These tests verify that workflow actors propagate expected_sha256 from the
// step's `with:` block (or workflow inputs) into the callback that ultimately
// builds the ApplyPackageReleaseRequest.

import (
	"context"
	"testing"
)

// TestDispatch_PopulatesExpectedSha256FromManifest asserts that
// nodeInstallPackage reads expected_sha256 from req.With and forwards it
// verbatim to the InstallPackage callback. This is the wire from the
// release.apply.package workflow's install_package step (which now sets
// expected_sha256: $.resolved_entrypoint_checksum) into the dispatch path.
func TestDispatch_PopulatesExpectedSha256FromManifest(t *testing.T) {
	const wantHash = "deadbeef0123456789abcdef0123456789abcdef0123456789abcdef01234567"

	var seen string
	cfg := NodeDirectApplyConfig{
		InstallPackage: func(ctx context.Context, name, version, kind, buildID, desiredHash, expectedSha256 string, buildNumber int64) error {
			seen = expectedSha256
			return nil
		},
	}
	handler := nodeInstallPackage(cfg)

	req := ActionRequest{
		With: map[string]any{
			"package_name":    "repository",
			"version":         "1.2.119",
			"kind":            "SERVICE",
			"build_id":        "01J0001",
			"desired_hash":    "convergence-identity-hash",
			"expected_sha256": wantHash,
			"build_number":    int64(371),
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if seen != wantHash {
		t.Fatalf("InstallPackage callback expected_sha256 = %q, want %q", seen, wantHash)
	}
}

// TestDispatch_EmptyManifestChecksumPropagatesAsEmpty asserts the actor passes
// an empty string through when the manifest had no entrypoint_checksum. The
// node-agent verify gate then writes installed_unverified honestly. The forbidden
// outcome is the actor silently filling in any non-empty value.
func TestDispatch_EmptyManifestChecksumPropagatesAsEmpty(t *testing.T) {
	var seen string
	called := false
	cfg := NodeDirectApplyConfig{
		InstallPackage: func(ctx context.Context, name, version, kind, buildID, desiredHash, expectedSha256 string, buildNumber int64) error {
			seen = expectedSha256
			called = true
			return nil
		},
	}
	handler := nodeInstallPackage(cfg)

	// expected_sha256 omitted entirely.
	req := ActionRequest{
		With: map[string]any{
			"package_name": "legacy-pkg",
			"version":      "1.0.0",
			"kind":         "SERVICE",
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if !called {
		t.Fatalf("InstallPackage callback not invoked")
	}
	if seen != "" {
		t.Fatalf("InstallPackage callback expected_sha256 = %q, want empty; actor must not synthesize a value", seen)
	}
}

// TestDispatch_ControllerDeployPropagatesExpectedSha256 asserts that the
// controller deploy actor reads expected_sha256 from req.With (set by
// release.apply.controller.yaml: expected_sha256: $.expected_sha256) and
// forwards it to the ApplyPackageRelease callback that builds the dispatch.
func TestDispatch_ControllerDeployPropagatesExpectedSha256(t *testing.T) {
	const wantHash = "1111222233334444555566667777888899990000aaaabbbbccccddddeeeeffff"

	var seen string
	cfg := ControllerDeployConfig{
		ApplyPackageRelease: func(ctx context.Context, nodeID, agentEndpoint, pkgName, pkgKind, version, publisher, repoAddr string, buildNumber int64, force bool, buildID, expectedSha256 string) error {
			seen = expectedSha256
			return nil
		},
	}
	handler := deployApplyPackageRelease(cfg)

	req := ActionRequest{
		Inputs: map[string]any{
			"resolved_build_id": "01J0002",
			"expected_sha256":   wantHash, // set at workflow input level
		},
		With: map[string]any{
			"node_id":         "node-a",
			"agent_endpoint":  "node-a:11000",
			"package_name":    "cluster-controller",
			"package_kind":    "SERVICE",
			"version":         "1.2.119",
			"publisher":       "core@globular.io",
			"repository_addr": "repository.globular.internal:10009",
			"force":           false,
			"build_number":    float64(371),
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if seen != wantHash {
		t.Fatalf("ApplyPackageRelease expected_sha256 = %q, want %q", seen, wantHash)
	}
}

// TestDispatch_ControllerDeployEmptyExpectedSha256PropagatesAsEmpty mirrors the
// nodeInstallPackage empty-propagation test for the deploy actor.
func TestDispatch_ControllerDeployEmptyExpectedSha256PropagatesAsEmpty(t *testing.T) {
	var seen string
	called := false
	cfg := ControllerDeployConfig{
		ApplyPackageRelease: func(ctx context.Context, nodeID, agentEndpoint, pkgName, pkgKind, version, publisher, repoAddr string, buildNumber int64, force bool, buildID, expectedSha256 string) error {
			seen = expectedSha256
			called = true
			return nil
		},
	}
	handler := deployApplyPackageRelease(cfg)

	req := ActionRequest{
		Inputs: map[string]any{},
		With: map[string]any{
			"node_id":      "node-a",
			"package_name": "cluster-controller",
			"version":      "1.2.119",
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if !called {
		t.Fatalf("ApplyPackageRelease callback not invoked")
	}
	if seen != "" {
		t.Fatalf("ApplyPackageRelease expected_sha256 = %q, want empty; actor must not synthesize a value", seen)
	}
}

// TestDispatch_WithBlockOverridesInputs verifies the step-level `with:` block
// takes precedence over workflow-level inputs when both are present. The yaml
// pattern is `expected_sha256: $.expected_sha256` (template substitution), and
// resume paths should always honour the step's explicit value.
func TestDispatch_WithBlockOverridesInputs(t *testing.T) {
	const inputsHash = "0000000000000000000000000000000000000000000000000000000000000000"
	const withHash = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	var seen string
	cfg := ControllerDeployConfig{
		ApplyPackageRelease: func(ctx context.Context, nodeID, agentEndpoint, pkgName, pkgKind, version, publisher, repoAddr string, buildNumber int64, force bool, buildID, expectedSha256 string) error {
			seen = expectedSha256
			return nil
		},
	}
	handler := deployApplyPackageRelease(cfg)

	req := ActionRequest{
		Inputs: map[string]any{
			"expected_sha256": inputsHash,
		},
		With: map[string]any{
			"node_id":         "node-a",
			"package_name":    "cluster-controller",
			"version":         "1.2.119",
			"expected_sha256": withHash, // explicit step value wins
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if seen != withHash {
		t.Fatalf("expected_sha256 = %q, want %q (step with-block must override inputs)", seen, withHash)
	}
}
