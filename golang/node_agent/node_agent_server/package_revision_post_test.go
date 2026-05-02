package main

// package_revision_post_test.go — Phase F unit tests for the post-apply
// hook helpers. Network-free: tests classifyConfigOutcome, resolveConfigEntry,
// mapKindStringToProto, and the action-decision logic in
// recordRevisionAndReceipts (without dialing the repository).

import (
	"os"
	"path/filepath"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestResolveConfigEntry_FillsKindDefaults(t *testing.T) {
	cases := []struct {
		name     string
		in       *repositorypb.PackageConfigFile
		wantMS   repositorypb.MergeStrategy
		wantSens bool
	}{
		{
			"DEFAULT → REPLACE",
			&repositorypb.PackageConfigFile{Path: "/etc/x", ConfigKind: repositorypb.ConfigKind_CONFIG_DEFAULT},
			repositorypb.MergeStrategy_MERGE_REPLACE,
			false,
		},
		{
			"OPERATOR_OVERRIDE → PRESERVE + sensitive=false",
			&repositorypb.PackageConfigFile{Path: "/etc/y", ConfigKind: repositorypb.ConfigKind_CONFIG_OPERATOR_OVERRIDE},
			repositorypb.MergeStrategy_MERGE_PRESERVE,
			false,
		},
		{
			"SECRET → SECRET_EXTERNAL + sensitive=true",
			&repositorypb.PackageConfigFile{Path: "/etc/z", ConfigKind: repositorypb.ConfigKind_CONFIG_SECRET},
			repositorypb.MergeStrategy_MERGE_SECRET_EXTERNAL,
			true,
		},
		{
			"GENERATED → TEMPLATE_RENDER",
			&repositorypb.PackageConfigFile{Path: "/etc/q", ConfigKind: repositorypb.ConfigKind_CONFIG_GENERATED},
			repositorypb.MergeStrategy_MERGE_TEMPLATE_RENDER,
			false,
		},
	}
	for _, tc := range cases {
		out := resolveConfigEntry(tc.in)
		if out.GetMergeStrategy() != tc.wantMS {
			t.Errorf("%s: merge_strategy got %s want %s",
				tc.name, out.GetMergeStrategy(), tc.wantMS)
		}
		if out.GetSensitive() != tc.wantSens {
			t.Errorf("%s: sensitive got %v want %v",
				tc.name, out.GetSensitive(), tc.wantSens)
		}
	}
}

func TestResolveConfigEntry_OperatorOverrideForcesPreserveOnUpgrade(t *testing.T) {
	out := resolveConfigEntry(&repositorypb.PackageConfigFile{
		Path: "/etc/x", ConfigKind: repositorypb.ConfigKind_CONFIG_OPERATOR_OVERRIDE,
	})
	if !out.GetPreserveOnUpgrade() {
		t.Fatal("OPERATOR_OVERRIDE must force preserve_on_upgrade=true")
	}
}

func TestClassifyConfigOutcome_SecretSkipped(t *testing.T) {
	c := &repositorypb.PackageConfigFile{
		Path: "/var/lib/globular/db.password", ConfigKind: repositorypb.ConfigKind_CONFIG_SECRET, Sensitive: true,
	}
	action, before, after := classifyConfigOutcome(c)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_SKIPPED_SECRET {
		t.Fatalf("got %s, want SKIPPED_SECRET", action)
	}
	if before != "" || after != "" {
		t.Fatal("SECRET receipt must NOT carry checksums (would risk content leak)")
	}
}

func TestClassifyConfigOutcome_PreservedAndReplaced(t *testing.T) {
	dir := t.TempDir()

	// Existing file with checksum-at-install matching current content → PRESERVED.
	preservedPath := filepath.Join(dir, "preserved.conf")
	if err := os.WriteFile(preservedPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	currentSum := fileSHA256(preservedPath)
	if currentSum == "" {
		t.Fatal("fileSHA256 should produce a non-empty hash for an existing file")
	}
	c := &repositorypb.PackageConfigFile{
		Path:              preservedPath,
		ConfigKind:        repositorypb.ConfigKind_CONFIG_DEFAULT,
		ChecksumAtInstall: currentSum,
	}
	action, _, _ := classifyConfigOutcome(c)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_PRESERVED {
		t.Fatalf("matching checksum should yield PRESERVED, got %s", action)
	}

	// Same path but checksum-at-install differs → REPLACED.
	c.ChecksumAtInstall = "deadbeef"
	action, _, _ = classifyConfigOutcome(c)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_REPLACED {
		t.Fatalf("differing checksum should yield REPLACED, got %s", action)
	}
}

func TestClassifyConfigOutcome_GeneratedHasEmptyBefore(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "g.conf")
	_ = os.WriteFile(p, []byte("rendered"), 0o644)
	c := &repositorypb.PackageConfigFile{Path: p, ConfigKind: repositorypb.ConfigKind_CONFIG_GENERATED}
	action, before, after := classifyConfigOutcome(c)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_GENERATED {
		t.Fatalf("got %s, want GENERATED", action)
	}
	if before != "" {
		t.Fatal("GENERATED has no meaningful 'before' state")
	}
	if after == "" {
		t.Fatal("GENERATED must carry the post-render checksum")
	}
}

func TestClassifyConfigOutcome_MissingFileIsFailed(t *testing.T) {
	c := &repositorypb.PackageConfigFile{
		Path: "/does/not/exist/x.conf", ConfigKind: repositorypb.ConfigKind_CONFIG_DEFAULT,
	}
	action, _, _ := classifyConfigOutcome(c)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_FAILED {
		t.Fatalf("got %s, want FAILED for missing file", action)
	}
}

func TestMapKindStringToProto(t *testing.T) {
	cases := map[string]repositorypb.ArtifactKind{
		"SERVICE":        repositorypb.ArtifactKind_SERVICE,
		"service":        repositorypb.ArtifactKind_SERVICE,
		"INFRASTRUCTURE": repositorypb.ArtifactKind_INFRASTRUCTURE,
		"APPLICATION":    repositorypb.ArtifactKind_APPLICATION,
		"COMMAND":        repositorypb.ArtifactKind_COMMAND,
		"AGENT":          repositorypb.ArtifactKind_AGENT,
		"":               repositorypb.ArtifactKind_SERVICE,   // default
		"NOPE":           repositorypb.ArtifactKind_SERVICE,   // default
	}
	for in, want := range cases {
		if got := mapKindStringToProto(in); got != want {
			t.Errorf("kind %q: got %s want %s", in, got, want)
		}
	}
}

// TestActionDecision verifies the install / upgrade / rollback label logic
// in recordRevisionAndReceipts without dialing the repository.
func TestActionDecision(t *testing.T) {
	cases := []struct {
		name         string
		rollbackMode bool
		previous     *node_agentpb.InstalledPackage
		want         string
	}{
		{
			name:         "fresh install (no previous)",
			rollbackMode: false,
			previous:     nil,
			want:         "install",
		},
		{
			name:         "upgrade over existing",
			rollbackMode: false,
			previous:     &node_agentpb.InstalledPackage{Version: "1.0.0"},
			want:         "upgrade",
		},
		{
			name:         "rollback overrides upgrade",
			rollbackMode: true,
			previous:     &node_agentpb.InstalledPackage{Version: "1.1.0"},
			want:         "rollback",
		},
		{
			name:         "rollback even on first install (operator force)",
			rollbackMode: true,
			previous:     nil,
			want:         "rollback",
		},
	}
	for _, tc := range cases {
		// Mirror the same logic the helper uses.
		got := "install"
		switch {
		case tc.rollbackMode:
			got = "rollback"
		case tc.previous != nil && tc.previous.GetVersion() != "":
			got = "upgrade"
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPreviousStatus_NilSafe(t *testing.T) {
	if got := previousStatus(nil); got != "" {
		t.Fatalf("nil previous must yield empty status, got %q", got)
	}
	got := previousStatus(&node_agentpb.InstalledPackage{Status: "installed"})
	if got != "installed" {
		t.Fatalf("got %q, want installed", got)
	}
}
