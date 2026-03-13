package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestCompileReleasePlan_DefaultPlatform(t *testing.T) {
	rel := &cluster_controllerpb.ServiceRelease{
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "echo",
			// Platform intentionally empty.
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedVersion:        "1.0.0",
			ResolvedArtifactDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Check that artifact.fetch step includes platform.
	for _, step := range plan.GetSpec().GetSteps() {
		if step.GetAction() == "artifact.fetch" {
			plat := step.GetArgs().GetFields()["platform"].GetStringValue()
			if plat == "" {
				t.Error("artifact.fetch step missing platform arg")
			}
			if plat != "linux_amd64" {
				t.Errorf("expected default platform linux_amd64, got %s", plat)
			}
			return
		}
	}
	t.Error("artifact.fetch step not found in plan")
}

func TestCompileReleasePlan_ExplicitPlatform(t *testing.T) {
	rel := &cluster_controllerpb.ServiceRelease{
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "echo",
			Platform:    "linux_arm64",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedVersion:        "1.0.0",
			ResolvedArtifactDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}

	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	for _, step := range plan.GetSpec().GetSteps() {
		if step.GetAction() == "artifact.fetch" {
			plat := step.GetArgs().GetFields()["platform"].GetStringValue()
			if plat != "linux_arm64" {
				t.Errorf("expected linux_arm64, got %s", plat)
			}
			return
		}
	}
	t.Error("artifact.fetch step not found in plan")
}

func TestAssertSHA256Hex_Valid(t *testing.T) {
	valid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := assertSHA256Hex(valid, "pub", "svc", "1.0.0"); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestAssertSHA256Hex_TooShort(t *testing.T) {
	short := "aabb"
	if err := assertSHA256Hex(short, "pub", "svc", "1.0.0"); err == nil {
		t.Error("expected error for short checksum")
	}
}

func TestAssertSHA256Hex_InvalidChars(t *testing.T) {
	invalid := "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg"
	if err := assertSHA256Hex(invalid, "pub", "svc", "1.0.0"); err == nil {
		t.Error("expected error for invalid hex chars")
	}
}
