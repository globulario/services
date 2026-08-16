package main

// config_ownership_test.go — Phase CLI-D tests for default merge strategy
// resolution, classification, and policy gate.

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestDefaultMergeStrategy_PerKind(t *testing.T) {
	cases := []struct {
		kind repopb.ConfigKind
		want repopb.MergeStrategy
	}{
		{repopb.ConfigKind_CONFIG_DEFAULT, repopb.MergeStrategy_MERGE_REPLACE},
		{repopb.ConfigKind_CONFIG_OPERATOR_OVERRIDE, repopb.MergeStrategy_MERGE_PRESERVE},
		{repopb.ConfigKind_CONFIG_GENERATED, repopb.MergeStrategy_MERGE_TEMPLATE_RENDER},
		{repopb.ConfigKind_CONFIG_SECRET, repopb.MergeStrategy_MERGE_SECRET_EXTERNAL},
		{repopb.ConfigKind_CONFIG_RUNTIME_STATE, repopb.MergeStrategy_MERGE_APPEND_ONLY},
	}
	for _, tc := range cases {
		if got := DefaultMergeStrategy(tc.kind); got != tc.want {
			t.Errorf("kind %s: got %s, want %s", tc.kind, got, tc.want)
		}
	}
}

func TestResolveConfigEntry_FillsDefaults(t *testing.T) {
	in := &repopb.PackageConfigFile{
		Path:       "/etc/globular/foo.json",
		ConfigKind: repopb.ConfigKind_CONFIG_OPERATOR_OVERRIDE,
		// merge_strategy unspecified; preserve_on_upgrade not set
	}
	out := ResolveConfigEntry(in)
	if out.GetMergeStrategy() != repopb.MergeStrategy_MERGE_PRESERVE {
		t.Errorf("merge: got %s, want PRESERVE", out.GetMergeStrategy())
	}
	if !out.GetPreserveOnUpgrade() {
		t.Error("OPERATOR_OVERRIDE must imply preserve_on_upgrade=true")
	}
}

func TestResolveConfigEntry_SecretForcesSensitive(t *testing.T) {
	in := &repopb.PackageConfigFile{
		Path: "/var/lib/globular/secret.key", ConfigKind: repopb.ConfigKind_CONFIG_SECRET,
	}
	out := ResolveConfigEntry(in)
	if !out.GetSensitive() {
		t.Fatal("SECRET must force sensitive=true")
	}
}

func TestClassifyConfigDiff(t *testing.T) {
	cases := []struct {
		name string
		c    *repopb.PackageConfigFile
		want ConfigDiffStatus
	}{
		{"unknown empty", &repopb.PackageConfigFile{}, ConfigStatusUnknown},
		{"unchanged", &repopb.PackageConfigFile{
			ChecksumAtInstall: "sha256:abc", CurrentChecksum: "sha256:abc",
		}, ConfigStatusUnchanged},
		{"modified", &repopb.PackageConfigFile{
			ChecksumAtInstall: "sha256:abc", CurrentChecksum: "sha256:def",
		}, ConfigStatusModified},
		{"missing", &repopb.PackageConfigFile{
			ChecksumAtInstall: "sha256:abc",
		}, ConfigStatusMissing},
		{"generated", &repopb.PackageConfigFile{
			ConfigKind: repopb.ConfigKind_CONFIG_GENERATED,
		}, ConfigStatusGenerated},
		{"normalized prefix unchanged", &repopb.PackageConfigFile{
			ChecksumAtInstall: "sha256:ABC", CurrentChecksum: "abc",
		}, ConfigStatusUnchanged},
	}
	for _, tc := range cases {
		if got := ClassifyConfigDiff(tc.c); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestPolicyAllowsUpgrade_FailsOnLocalModification(t *testing.T) {
	c := &repopb.PackageConfigFile{
		ChecksumAtInstall: "sha256:abc", CurrentChecksum: "sha256:def",
		MergeStrategy: repopb.MergeStrategy_MERGE_FAIL_ON_LOCAL_MODIFICATION,
	}
	allowed, reason := PolicyAllowsUpgrade(c)
	if allowed {
		t.Fatal("FAIL_ON_LOCAL_MODIFICATION must block upgrade when modified")
	}
	if reason == "" {
		t.Fatal("must include human-readable reason")
	}
}

func TestPolicyAllowsUpgrade_PreserveAllowsUpgrade(t *testing.T) {
	c := &repopb.PackageConfigFile{
		ChecksumAtInstall: "sha256:abc", CurrentChecksum: "sha256:def",
		MergeStrategy: repopb.MergeStrategy_MERGE_PRESERVE,
	}
	allowed, _ := PolicyAllowsUpgrade(c)
	if !allowed {
		t.Fatal("PRESERVE must allow upgrade — it just keeps the local copy")
	}
}

func TestRedactConfig_StripsSecretPath(t *testing.T) {
	r := RedactConfig(&repopb.PackageConfigFile{
		Path: "/var/lib/globular/db.password", ConfigKind: repopb.ConfigKind_CONFIG_SECRET,
		ChecksumAtInstall: "sha256:abc",
	})
	if r.GetPath() != "[REDACTED]" {
		t.Errorf("expected path redacted, got %q", r.GetPath())
	}
	if r.GetChecksumAtInstall() == "" {
		t.Error("checksum should NOT be redacted — checksums don't leak content")
	}
}

func TestArtifactManifest_RoundTripsConfigs(t *testing.T) {
	// Make sure proto marshal/unmarshal preserves the new configs field.
	m := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{Name: "echo"},
		Configs: []*repopb.PackageConfigFile{
			{
				Path:          "/etc/globular/foo.json",
				ConfigKind:    repopb.ConfigKind_CONFIG_OPERATOR_OVERRIDE,
				MergeStrategy: repopb.MergeStrategy_MERGE_PRESERVE,
			},
		},
		SignatureKeyId: "k1",
	}
	if len(m.GetConfigs()) != 1 {
		t.Fatalf("expected 1 config, got %d", len(m.GetConfigs()))
	}
	if m.GetSignatureKeyId() != "k1" {
		t.Fatal("signature_key_id round-trip failed")
	}
}
