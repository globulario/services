package main

import (
	"strings"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

// ── assertSHA256Hex tests ─────────────────────────────────────────────────────

func TestAssertSHA256Hex_lowercase(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	if err := assertSHA256Hex(checksum, "pub", "svc", "1.0.0"); err != nil {
		t.Fatalf("expected lowercase hex to pass: %v", err)
	}
}

func TestAssertSHA256Hex_uppercase(t *testing.T) {
	// Must accept uppercase (repository may emit DEADBEEF-style digests).
	checksum := strings.Repeat("A", 64)
	if err := assertSHA256Hex(checksum, "pub", "svc", "1.0.0"); err != nil {
		t.Fatalf("expected uppercase hex to pass after normalization: %v", err)
	}
}

func TestAssertSHA256Hex_mixed(t *testing.T) {
	// Mixed case: first 32 lowercase, last 32 uppercase.
	checksum := strings.Repeat("a", 32) + strings.Repeat("F", 32)
	if err := assertSHA256Hex(checksum, "pub", "svc", "1.0.0"); err != nil {
		t.Fatalf("expected mixed-case hex to pass: %v", err)
	}
}

func TestAssertSHA256Hex_tooShort(t *testing.T) {
	if err := assertSHA256Hex("deadbeef", "pub", "svc", "1.0.0"); err == nil {
		t.Fatal("expected error for short checksum, got nil")
	}
}

func TestAssertSHA256Hex_invalidChars(t *testing.T) {
	checksum := strings.Repeat("z", 64) // 'z' is not hex
	if err := assertSHA256Hex(checksum, "pub", "svc", "1.0.0"); err == nil {
		t.Fatal("expected error for non-hex chars, got nil")
	}
}

func TestAssertSHA256Hex_withWhitespace(t *testing.T) {
	checksum := "  " + strings.Repeat("a", 64) + "  "
	if err := assertSHA256Hex(checksum, "pub", "svc", "1.0.0"); err != nil {
		t.Fatalf("expected trimmed hex to pass: %v", err)
	}
}

// ── CompileReleasePlan rollback guard tests ───────────────────────────────────

func validRelease(installedVersion string) *clustercontrollerpb.ServiceRelease {
	return &clustercontrollerpb.ServiceRelease{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "my-svc", Generation: 1},
		Spec: &clustercontrollerpb.ServiceReleaseSpec{
			PublisherID: "globulario",
			ServiceName: "my-svc",
			Platform:    "linux_amd64",
		},
		Status: &clustercontrollerpb.ServiceReleaseStatus{
			Phase:                  clustercontrollerpb.ReleasePhaseResolved,
			ResolvedVersion:        "1.2.0",
			ResolvedArtifactDigest: strings.Repeat("a", 64),
		},
	}
}

func TestCompileReleasePlan_noRollbackWhenInstalledEmpty(t *testing.T) {
	rel := validRelease("")
	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.GetSpec().GetRollback()) != 0 {
		t.Fatalf("expected no rollback steps when installedVersion is empty, got %d",
			len(plan.GetSpec().GetRollback()))
	}
}

func TestCompileReleasePlan_noRollbackWhenInstalledEqualsTarget(t *testing.T) {
	// Node already at 1.2.0, target is also 1.2.0 → rollback to self must be suppressed.
	rel := validRelease("1.2.0")
	plan, err := CompileReleasePlan("node-1", rel, "1.2.0", "cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.GetSpec().GetRollback()) != 0 {
		t.Fatalf("expected no rollback steps when installedVersion == resolvedVersion, got %d",
			len(plan.GetSpec().GetRollback()))
	}
}

func TestCompileReleasePlan_rollbackArmedWhenInstalledDiffersFromTarget(t *testing.T) {
	// Node at 1.1.0, upgrading to 1.2.0 → rollback steps must be present.
	rel := validRelease("1.1.0")
	plan, err := CompileReleasePlan("node-1", rel, "1.1.0", "cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.GetSpec().GetRollback()) == 0 {
		t.Fatal("expected rollback steps when installedVersion != resolvedVersion, got none")
	}
}

func TestCompileReleasePlan_stagingPathIncludesPublisher(t *testing.T) {
	rel := validRelease("")
	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fetchStep := plan.GetSpec().GetSteps()[0]
	artPath := fetchStep.GetArgs().GetFields()["artifact_path"].GetStringValue()
	if !strings.Contains(artPath, "globulario") {
		t.Fatalf("staging path should include publisher_id 'globulario', got %q", artPath)
	}
	if !strings.Contains(artPath, "my-svc") {
		t.Fatalf("staging path should include service name 'my-svc', got %q", artPath)
	}
	if !strings.Contains(artPath, "1.2.0") {
		t.Fatalf("staging path should include version '1.2.0', got %q", artPath)
	}
}

func TestCompileReleasePlan_publisherIDInFetchArgs(t *testing.T) {
	rel := validRelease("")
	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fetchStep := plan.GetSpec().GetSteps()[0]
	pubID := fetchStep.GetArgs().GetFields()["publisher_id"].GetStringValue()
	if pubID != "globulario" {
		t.Fatalf("expected publisher_id=globulario in fetch args, got %q", pubID)
	}
}

func TestCompileReleasePlan_expectedSHA256InFetchAndVerifyArgs(t *testing.T) {
	rel := validRelease("")
	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	steps := plan.GetSpec().GetSteps()
	fetchSHA := steps[0].GetArgs().GetFields()["expected_sha256"].GetStringValue()
	verifySHA := steps[1].GetArgs().GetFields()["expected_sha256"].GetStringValue()
	expected := strings.Repeat("a", 64)
	if fetchSHA != expected {
		t.Fatalf("expected_sha256 in fetch step: want %q got %q", expected, fetchSHA)
	}
	if verifySHA != expected {
		t.Fatalf("expected_sha256 in verify step: want %q got %q", expected, verifySHA)
	}
}
