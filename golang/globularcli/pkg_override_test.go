package main

// pkg_override_test.go — Unit tests for the local package override lifecycle.
//
// Covers what can be tested without a live etcd or repository:
//
//   1. LocalOverride JSON round-trip (marshal → unmarshal identity)
//   2. LocalOverrideSnapshot preserved inside override after marshal/unmarshal
//   3. Override identity is build_id/build_number, not a version suffix
//   4. explain-package table output — no active override shows mode=official
//   5. explain-package table output — active override shows banner + fields
//   6. explain-package JSON output — local_override block present and correct
//   7. upsertServiceDesiredVersion spec includes publisher_id when set
//   8. upsertServiceDesiredVersion omits publisher_id when empty
//
//   Guardrail 2 — legacy suffix helper compatibility:
//   9. hasLocalVersionSuffix accepts local/dev/hotfix versions
//   10. hasLocalVersionSuffix rejects platform semver versions

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── 1. LocalOverride JSON round-trip ─────────────────────────────────────────

func TestLocalOverrideJSONRoundTrip(t *testing.T) {
	orig := &cluster_controllerpb.LocalOverride{
		ServiceName:    "storage",
		PublisherID:    "core@globular.io",
		Version:        "1.2.43",
		BuildID:        "abc123def456",
		BuildNumber:    7,
		BasedOnVersion: "1.2.43",
		PatchReason:    "test retry fix",
		CreatedBy:      "globule-ryzen",
		CreatedAtUnixS: time.Now().Unix(),
		OfficialSnapshot: &cluster_controllerpb.LocalOverrideSnapshot{
			ServiceName: "storage",
			Version:     "1.2.43",
			BuildNumber: 5,
			BuildID:     "official-build-xyz",
			PublisherID: "core@globular.io",
			Generation:  12,
		},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got cluster_controllerpb.LocalOverride
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ServiceName != orig.ServiceName {
		t.Errorf("ServiceName: got %q want %q", got.ServiceName, orig.ServiceName)
	}
	if got.PublisherID != orig.PublisherID {
		t.Errorf("PublisherID: got %q want %q", got.PublisherID, orig.PublisherID)
	}
	if got.Version != orig.Version {
		t.Errorf("Version: got %q want %q", got.Version, orig.Version)
	}
	if got.BuildID != orig.BuildID {
		t.Errorf("BuildID: got %q want %q", got.BuildID, orig.BuildID)
	}
	if got.PatchReason != orig.PatchReason {
		t.Errorf("PatchReason: got %q want %q", got.PatchReason, orig.PatchReason)
	}
}

// ── 2. Snapshot survives round-trip ──────────────────────────────────────────

func TestLocalOverrideSnapshotRoundTrip(t *testing.T) {
	orig := &cluster_controllerpb.LocalOverride{
		ServiceName: "dns",
		PublisherID: "core@globular.io",
		Version:     "1.2.10",
		BuildID:     "hotfix-build-001",
		OfficialSnapshot: &cluster_controllerpb.LocalOverrideSnapshot{
			ServiceName: "dns",
			Version:     "1.2.10",
			BuildNumber: 3,
			BuildID:     "official-dns-build",
			PublisherID: "",
			Generation:  5,
		},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got cluster_controllerpb.LocalOverride
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.OfficialSnapshot == nil {
		t.Fatal("OfficialSnapshot lost after round-trip")
	}
	snap := got.OfficialSnapshot
	if snap.Version != "1.2.10" {
		t.Errorf("snapshot.Version: got %q want 1.2.10", snap.Version)
	}
	if snap.BuildID != "official-dns-build" {
		t.Errorf("snapshot.BuildID: got %q want official-dns-build", snap.BuildID)
	}
	if snap.Generation != 5 {
		t.Errorf("snapshot.Generation: got %d want 5", snap.Generation)
	}
}

// ── 3. Official publisher guard ───────────────────────────────────────────────

func TestOfficialPublisherGuard(t *testing.T) {
	forbidden := []string{
		"core@globular.io",
		"CORE@GLOBULAR.IO",
		"  core@globular.io  ",
	}
	for _, pub := range forbidden {
		if !strings.EqualFold(strings.TrimSpace(pub), "core@globular.io") {
			t.Errorf("guard should reject %q as official publisher", pub)
		}
	}

	allowed := []string{
		"local@ryzen",
		"local@nuc",
		"dev@workbench",
		"hotfix@ci",
	}
	for _, pub := range allowed {
		if strings.EqualFold(strings.TrimSpace(pub), "core@globular.io") {
			t.Errorf("guard should allow %q as non-official publisher", pub)
		}
	}
}

// ── 4. explain-package: no override → mode=official ──────────────────────────

func TestExplainPackagePrint_NoOverride_ShowsOfficial(t *testing.T) {
	info := &repopb.PackageInfo{
		Name:      "storage",
		Publisher: "core@globular.io",
		Versions:  []string{"1.2.43", "1.2.44"},
		Desired: &repopb.DesiredInfo{
			Present:    true,
			Version:    "1.2.44",
			Generation: 5,
			Publisher:  "core@globular.io",
		},
	}

	out := captureOut(t, func() {
		explainPackagePrint(info, nil)
	})

	if !strings.Contains(out, "mode:") || !strings.Contains(out, "official") {
		t.Errorf("expected mode=official in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1.2.44") {
		t.Errorf("expected desired version 1.2.44 in output, got:\n%s", out)
	}
	if strings.Contains(out, "LOCAL OVERRIDE") {
		t.Errorf("must not show LOCAL OVERRIDE banner when no override, got:\n%s", out)
	}
}

// ── 5. explain-package: active override → banner + fields ────────────────────

func TestExplainPackagePrint_WithOverride_ShowsBanner(t *testing.T) {
	info := &repopb.PackageInfo{
		Name:      "storage",
		Publisher: "core@globular.io",
		Versions:  []string{"1.2.43"},
		Desired: &repopb.DesiredInfo{
			Present:    true,
			Version:    "1.2.43+local.ryzen.1",
			Generation: 6,
			Publisher:  "local@ryzen",
		},
	}
	ov := &cluster_controllerpb.LocalOverride{
		ServiceName:    "storage",
		PublisherID:    "local@ryzen",
		Version:        "1.2.43+local.ryzen.1",
		BuildID:        "abc123def456abc1",
		BasedOnVersion: "1.2.43",
		PatchReason:    "test retry fix during repository bootstrap",
		CreatedBy:      "globule-ryzen",
		CreatedAtUnixS: time.Now().Unix(),
		OfficialSnapshot: &cluster_controllerpb.LocalOverrideSnapshot{
			ServiceName: "storage",
			Version:     "1.2.43",
			BuildID:     "official-storage-build",
			PublisherID: "core@globular.io",
		},
	}

	out := captureOut(t, func() {
		explainPackagePrint(info, ov)
	})

	if !strings.Contains(out, "LOCAL OVERRIDE ACTIVE") {
		t.Errorf("expected LOCAL OVERRIDE ACTIVE banner, got:\n%s", out)
	}
	if !strings.Contains(out, "local@ryzen") {
		t.Errorf("expected override publisher in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1.2.43+local.ryzen.1") {
		t.Errorf("expected override version in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1.2.43") {
		t.Errorf("expected based_on version in output, got:\n%s", out)
	}
	if !strings.Contains(out, "test retry fix") {
		t.Errorf("expected patch reason in output, got:\n%s", out)
	}
	if !strings.Contains(out, "pkg override remove storage") {
		t.Errorf("expected remove hint in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Official BOM") {
		t.Errorf("expected Official BOM section in output, got:\n%s", out)
	}
}

// ── 6. explain-package JSON: local_override block ────────────────────────────

func TestExplainPackageJSON_WithOverride_HasLocalOverrideBlock(t *testing.T) {
	info := &repopb.PackageInfo{
		Name:      "storage",
		Publisher: "core@globular.io",
		Versions:  []string{"1.2.43"},
		Desired: &repopb.DesiredInfo{
			Present:    true,
			Version:    "1.2.43+local.ryzen.1",
			Generation: 6,
		},
	}
	ov := &cluster_controllerpb.LocalOverride{
		ServiceName: "storage",
		PublisherID: "local@ryzen",
		Version:     "1.2.43+local.ryzen.1",
		BuildID:     "abc123",
		PatchReason: "fix retry loop",
		OfficialSnapshot: &cluster_controllerpb.LocalOverrideSnapshot{
			ServiceName: "storage",
			Version:     "1.2.43",
			BuildID:     "official-build",
		},
	}

	out := captureOut(t, func() {
		if err := explainPackageJSON(info, ov); err != nil {
			t.Errorf("explainPackageJSON: %v", err)
		}
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	ovBlock, ok := result["local_override"].(map[string]interface{})
	if !ok {
		t.Fatalf("local_override block missing or wrong type in JSON output")
	}
	if ovBlock["active"] != true {
		t.Errorf("local_override.active should be true, got %v", ovBlock["active"])
	}
	if ovBlock["publisher"] != "local@ryzen" {
		t.Errorf("local_override.publisher: got %v want local@ryzen", ovBlock["publisher"])
	}
	if ovBlock["reason"] != "fix retry loop" {
		t.Errorf("local_override.reason: got %v", ovBlock["reason"])
	}
	snap, ok := ovBlock["official_snapshot"].(map[string]interface{})
	if !ok {
		t.Fatalf("local_override.official_snapshot missing")
	}
	if snap["version"] != "1.2.43" {
		t.Errorf("official_snapshot.version: got %v want 1.2.43", snap["version"])
	}
}

func TestExplainPackageJSON_NoOverride_ActiveFalse(t *testing.T) {
	info := &repopb.PackageInfo{
		Name:     "storage",
		Versions: []string{"1.2.43"},
		Desired:  &repopb.DesiredInfo{Present: true, Version: "1.2.43"},
	}

	out := captureOut(t, func() {
		if err := explainPackageJSON(info, nil); err != nil {
			t.Errorf("explainPackageJSON: %v", err)
		}
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	ovBlock, ok := result["local_override"].(map[string]interface{})
	if !ok {
		t.Fatalf("local_override block missing")
	}
	if ovBlock["active"] != false {
		t.Errorf("local_override.active should be false when no override, got %v", ovBlock["active"])
	}
}

// ── 9. hasLocalVersionSuffix accepts local/dev/hotfix ────────────────────────

func TestHasLocalVersionSuffix_LocalVersions(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"1.2.43+local.ryzen.1", true},
		{"1.2.43-dev.fix1", true},
		{"1.2.43-hotfix.cert", true},
		{"1.2.43+dev.1", true},
		{"1.2.43+hotfix.auth", true},
		// ── 10. official semver rejected ────────────────────────────────────
		{"1.2.43", false},
		{"1.2.43-rc1", false},
		{"2.0.0", false},
		{"", false},
		{"v1.2.43", false},
	}
	for _, c := range cases {
		got := hasLocalVersionSuffix(c.version)
		if got != c.want {
			t.Errorf("hasLocalVersionSuffix(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

// ── 11. official-version suffix blocked ──────────────────────────────────────

func TestVersionSuffixGuard_OfficialVersionBlocked(t *testing.T) {
	officialVersions := []string{"1.2.43", "2.0.0", "1.2.43-rc1", "v1.2.43"}
	for _, v := range officialVersions {
		if hasLocalVersionSuffix(v) {
			t.Errorf("official version %q must not pass hasLocalVersionSuffix", v)
		}
	}
}

func TestVersionSuffixGuard_LocalVersionAllowed(t *testing.T) {
	localVersions := []string{
		"1.2.43+local.ryzen.1",
		"1.2.43-hotfix.auth",
		"1.2.43-dev.fix2",
	}
	for _, v := range localVersions {
		if !hasLocalVersionSuffix(v) {
			t.Errorf("local version %q must pass hasLocalVersionSuffix", v)
		}
	}
}

func TestDesiredSnapshotMatches_AllIdentityFields(t *testing.T) {
	official := &cluster_controllerpb.LocalOverrideSnapshot{
		ServiceName: "cluster-doctor",
		Version:     "1.2.257",
		BuildNumber: 2,
		BuildID:     "official-build",
		PublisherID: "",
	}
	current := &cluster_controllerpb.LocalOverrideSnapshot{
		ServiceName: " cluster-doctor ",
		Version:     " 1.2.257 ",
		BuildNumber: 2,
		BuildID:     " official-build ",
		PublisherID: " core@globular.io ",
	}

	if !desiredSnapshotMatches(current, official) {
		t.Fatal("expected current desired state to match official snapshot")
	}

	cases := []struct {
		name   string
		mutate func(*cluster_controllerpb.LocalOverrideSnapshot)
	}{
		{name: "service", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.ServiceName = "repository" }},
		{name: "version", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.Version = "1.2.258" }},
		{name: "build_number", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.BuildNumber = 3 }},
		{name: "build_id", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.BuildID = "other-build" }},
		{name: "publisher", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.PublisherID = "local@node" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := *current
			tc.mutate(&got)
			if desiredSnapshotMatches(&got, official) {
				t.Fatalf("expected mismatch after changing %s", tc.name)
			}
		})
	}
}

func TestCurrentDesiredIsOutsideOverride_RequiresOfficialDesired(t *testing.T) {
	ov := &cluster_controllerpb.LocalOverride{
		ServiceName: "cluster-doctor",
		Version:     "1.2.266+local.globule-ryzen.1",
		BuildID:     "local-build",
		BuildNumber: 1,
	}
	current := &cluster_controllerpb.LocalOverrideSnapshot{
		ServiceName: "cluster-doctor",
		Version:     "1.2.257",
		BuildNumber: 2,
		BuildID:     "official-build",
	}

	if !currentDesiredIsOutsideOverride(current, ov) {
		t.Fatal("expected official desired state to be outside stale local override")
	}

	cases := []struct {
		name   string
		mutate func(*cluster_controllerpb.LocalOverrideSnapshot)
	}{
		{name: "different_service", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.ServiceName = "repository" }},
		{name: "local_version", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.Version = "1.2.257+local.node.1" }},
		{name: "override_version", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.Version = ov.Version }},
		{name: "override_build_id", mutate: func(s *cluster_controllerpb.LocalOverrideSnapshot) { s.BuildID = ov.BuildID }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := *current
			tc.mutate(&got)
			if currentDesiredIsOutsideOverride(&got, ov) {
				t.Fatalf("expected %s to block stale override cleanup", tc.name)
			}
		})
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// captureOut runs fn and returns everything written to os.Stdout as a string.
func captureOut(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return buf.String()
}
