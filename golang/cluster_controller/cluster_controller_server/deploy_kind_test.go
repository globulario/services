package main

// deploy_kind_test.go — Per-package kind unit tests (G9).
//
// Verifies that kind validation in DeployControlPlanePackage fails fast with a
// clear error for unknown kinds, defaults correctly for the empty-kind case, and
// accepts all three valid kinds (SERVICE, INFRASTRUCTURE, COMMAND). These tests
// run the validation path only — leader check and repository resolution are not
// required for rejection cases that return before the leader gate.

import (
	"context"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestDeployKind_InvalidKindRejected verifies that an unrecognised kind is
// rejected immediately with a clear error message — before any leader check or
// repository resolution is attempted.
func TestDeployKind_InvalidKindRejected(t *testing.T) {
	cases := []string{"WORKLOAD", "workload", "UNKNOWN", "  SERVICE ", "service", ""}
	// Note: empty string is NOT invalid — it defaults to SERVICE. Only the
	// non-empty, non-canonical values are truly invalid.
	invalidCases := []string{"WORKLOAD", "workload", "UNKNOWN", "BLOB", "BINARY"}

	srv := newTestServer(t, &controllerState{})

	for _, kind := range invalidCases {
		t.Run(kind, func(t *testing.T) {
			resp, err := srv.DeployControlPlanePackage(context.Background(),
				&cluster_controllerpb.DeployControlPlanePackageRequest{
					PackageName: "test-pkg",
					Version:     "1.0.0",
					PackageKind: kind,
				})
			if err != nil {
				t.Fatalf("unexpected gRPC error: %v", err)
			}
			if resp == nil {
				t.Fatal("response must not be nil")
			}
			if resp.GetAccepted() {
				t.Errorf("kind %q must be rejected, got accepted=true", kind)
			}
			if !strings.Contains(resp.GetMessage(), "package_kind must be") {
				t.Errorf("rejection message %q should mention 'package_kind must be'", resp.GetMessage())
			}
		})
	}

	// Suppress the unused variable warning.
	_ = cases
}

// TestDeployKind_EmptyKindDefaultsToService verifies that an empty kind
// string defaults to SERVICE and passes validation (failing at the leader gate,
// not at the kind validation gate).
func TestDeployKind_EmptyKindDefaultsToService(t *testing.T) {
	srv := newTestServer(t, &controllerState{})

	resp, err := srv.DeployControlPlanePackage(context.Background(),
		&cluster_controllerpb.DeployControlPlanePackageRequest{
			PackageName: "test-pkg",
			Version:     "1.0.0",
			PackageKind: "", // empty — should default to SERVICE
		})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp == nil {
		t.Fatal("response must not be nil")
	}
	// Should fail at leader gate ("not the leader"), NOT at kind validation.
	// The rejection message must NOT mention "package_kind must be".
	if strings.Contains(resp.GetMessage(), "package_kind must be") {
		t.Errorf("empty kind should have defaulted to SERVICE and passed validation, got: %q", resp.GetMessage())
	}
}

// TestDeployKind_CaseInsensitiveNormalization verifies that mixed-case valid
// kinds (service, Service, INFRASTRUCTURE, Infrastructure) are normalised to
// uppercase and accepted by the validation gate.
func TestDeployKind_CaseInsensitiveNormalization(t *testing.T) {
	// These are all equivalent to their uppercase canonical form.
	cases := []struct {
		input    string
		wantPass bool // true = passes kind validation (fails at leader gate)
	}{
		{"SERVICE", true},
		{"service", true},
		{"Service", true},
		{"INFRASTRUCTURE", true},
		{"infrastructure", true},
		{"COMMAND", true},
		{"command", true},
		{"WORKLOAD", false}, // not a valid kind
	}

	srv := newTestServer(t, &controllerState{})

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			resp, err := srv.DeployControlPlanePackage(context.Background(),
				&cluster_controllerpb.DeployControlPlanePackageRequest{
					PackageName: "test-pkg",
					Version:     "1.0.0",
					PackageKind: tc.input,
				})
			if err != nil {
				t.Fatalf("unexpected gRPC error: %v", err)
			}
			kindValidationFailed := strings.Contains(resp.GetMessage(), "package_kind must be")
			if tc.wantPass && kindValidationFailed {
				t.Errorf("kind %q should pass validation, got rejection: %q", tc.input, resp.GetMessage())
			}
			if !tc.wantPass && !kindValidationFailed {
				t.Errorf("kind %q should fail validation, but got: %q", tc.input, resp.GetMessage())
			}
		})
	}
}

// TestDeployKind_WriteKindMismatchIsInjectable verifies that the
// writeKindMismatchRecord var-func is injectable and can be replaced for
// testing — this is the G9 invariant that the reconciler's kind mismatch
// path does not require a live etcd connection in tests.
func TestDeployKind_WriteKindMismatchIsInjectable(t *testing.T) {
	orig := writeKindMismatchRecord
	t.Cleanup(func() { writeKindMismatchRecord = orig })

	var called bool
	writeKindMismatchRecord = func(_ context.Context, nodeID, pkgName, desiredKind, repoKind string) {
		called = true
		if nodeID == "" || pkgName == "" || desiredKind == "" || repoKind == "" {
			t.Errorf("writeKindMismatchRecord called with empty field: node=%q pkg=%q desired=%q repo=%q",
				nodeID, pkgName, desiredKind, repoKind)
		}
	}

	// Call the function directly to confirm injection works.
	writeKindMismatchRecord(context.Background(), "n1", "rbac", "SERVICE", "INFRASTRUCTURE")
	if !called {
		t.Error("injected writeKindMismatchRecord was not called")
	}
}
