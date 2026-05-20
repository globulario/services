package main

// runtime_proof_test.go — Phase 2 of the Diagnostic Honesty Refactor.
//
// Pins the contract of collectServiceRuntimeProof. The handler itself does no
// I/O; everything goes through runtimeProofDeps so tests assert behaviour
// without booting systemd / touching /proc.
//
// Findings the consumer raises from these proofs (the test names mirror them
// so a regression points at the failure-mode the brief enumerates):
//   service.running_binary_hash_mismatch
//   service.running_version_mismatch
//   service.old_pid_after_upgrade
//   service.runtime_identity_unproven

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// stubDeps returns deps wired against a small in-memory file table + fake
// systemctl-show output. Each test owns its own table so cases don't leak.
type fakeFile struct {
	contents []byte
}

func newStubDeps(t *testing.T) (runtimeProofDeps, *stubState) {
	t.Helper()
	s := &stubState{
		files:     make(map[string]fakeFile),
		exeLinks:  make(map[int]string),
		showFn:    func(_ context.Context, _ string, _ ...string) (map[string]string, error) { return map[string]string{}, nil },
		nowVal:    time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
	}
	deps := runtimeProofDeps{
		ShowProperties: func(ctx context.Context, unit string, props ...string) (map[string]string, error) {
			return s.showFn(ctx, unit, props...)
		},
		HashFile: func(path string) (string, error) {
			f, ok := s.files[path]
			if !ok {
				return "", fmt.Errorf("no such file: %s", path)
			}
			sum := sha256.Sum256(f.contents)
			return hex.EncodeToString(sum[:]), nil
		},
		ReadProcExe: func(pid int) (string, error) {
			link, ok := s.exeLinks[pid]
			if !ok {
				return "", fmt.Errorf("no exe link for pid %d", pid)
			}
			return link, nil
		},
		Now: func() time.Time { return s.nowVal },
	}
	return deps, s
}

type stubState struct {
	files    map[string]fakeFile
	exeLinks map[int]string
	showFn   func(ctx context.Context, unit string, props ...string) (map[string]string, error)
	nowVal   time.Time
}

func (s *stubState) writeFile(path string, contents []byte) string {
	s.files[path] = fakeFile{contents: contents}
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

// ─────────────────────────────────────────────────────────────────────────
// Happy path — running exe == installed binary; full proof, no critical
// findings beyond runtime_identity_unproven (the version probe is still TBD).
// ─────────────────────────────────────────────────────────────────────────

func TestCollectServiceRuntimeProof_HappyPath(t *testing.T) {
	dir := withTempBinDir(t) // overrides globularBinDir for installedBinaryPath
	deps, s := newStubDeps(t)

	pkg := &node_agentpb.InstalledPackage{
		NodeId: "node-1", Name: "dns", Kind: "SERVICE",
		Version: "1.2.3", BuildId: "build-abc",
	}
	installedPath := installedBinaryPath("dns", "SERVICE") // <dir>/dns_server
	matchHash := s.writeFile(installedPath, []byte("dns-binary-v123"))

	// Running exe is the same path & same bytes — happy.
	const pid = 4242
	s.exeLinks[pid] = installedPath
	unitPath := "/etc/systemd/system/globular-dns.service"
	unitHash := s.writeFile(unitPath, []byte("[Unit]\nDescription=Globular DNS\n[Service]\nType=simple\nExecStart=/usr/lib/globular/bin/dns_server\n"))

	s.showFn = func(_ context.Context, unit string, _ ...string) (map[string]string, error) {
		if unit != "globular-dns.service" {
			t.Errorf("unexpected unit: %s", unit)
		}
		return map[string]string{
			"ActiveState":             "active",
			"SubState":                "running",
			"Type":                    "simple",
			"ExecStart":               "{ path=/usr/lib/globular/bin/dns_server ; ... }",
			"FragmentPath":            unitPath,
			"MainPID":                 fmt.Sprintf("%d", pid),
			"ExecMainStartTimestamp":  "Mon 2026-05-20 11:00:00 UTC",
		}, nil
	}
	_ = dir

	p := collectServiceRuntimeProof(context.Background(), "node-1", pkg, deps)

	if p.GetServiceName() != "dns" {
		t.Errorf("service_name=%s want=dns", p.GetServiceName())
	}
	if p.GetNodeId() != "node-1" || p.GetExpectedBuildId() != "build-abc" || p.GetExpectedVersion() != "1.2.3" {
		t.Errorf("claimed identity not propagated: %+v", p)
	}
	if p.GetInstalledSha256() != matchHash {
		t.Errorf("installed sha256: got=%s want=%s", p.GetInstalledSha256(), matchHash)
	}
	if p.GetRunningPid() != int32(pid) {
		t.Errorf("pid=%d want=%d", p.GetRunningPid(), pid)
	}
	if p.GetRunningExeSha256() != matchHash {
		t.Errorf("running exe sha256: got=%s want=%s (same path)", p.GetRunningExeSha256(), matchHash)
	}
	if p.GetSystemdActiveState() != "active" || p.GetSystemdSubState() != "running" {
		t.Errorf("systemd state: %s/%s", p.GetSystemdActiveState(), p.GetSystemdSubState())
	}
	if p.GetSystemdUnitSha256() != unitHash {
		t.Errorf("unit file sha256: got=%s want=%s", p.GetSystemdUnitSha256(), unitHash)
	}
	if p.GetEffectiveType() != "simple" {
		t.Errorf("effective Type=%s want=simple", p.GetEffectiveType())
	}
	// Runtime version probe is still unimplemented — the proof MUST carry an
	// errors entry naming the finding consumers should raise. If this test
	// fails, either (a) version probing landed and the marker can be dropped,
	// or (b) the marker text changed and consumers need to track the new
	// string. Either way, this is the moment to update both.
	if !containsErrorContaining(p.GetErrors(), "service.runtime_identity_unproven") {
		t.Errorf("expected runtime_identity_unproven marker in errors, got: %v", p.GetErrors())
	}
	if p.GetProcessStartTime() == nil {
		t.Error("ProcessStartTime should be populated from ExecMainStartTimestamp")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// service.running_binary_hash_mismatch — new binary on disk, OLD pid running.
// This is the headline failure mode the brief describes: apply replaced the
// disk binary but systemd never restarted, so /proc/<pid>/exe still resolves
// to the OLD bytes. The proof must surface BOTH hashes so the consumer can
// raise the finding.
// ─────────────────────────────────────────────────────────────────────────

func TestCollectServiceRuntimeProof_RunningExeDiffersFromInstalled(t *testing.T) {
	_ = withTempBinDir(t)
	deps, s := newStubDeps(t)

	pkg := &node_agentpb.InstalledPackage{
		NodeId: "node-1", Name: "workflow", Kind: "SERVICE",
		Version: "2.0.0", BuildId: "build-new",
	}
	newPath := installedBinaryPath("workflow", "SERVICE")
	newHash := s.writeFile(newPath, []byte("workflow-binary-v2-new"))

	// Old binary held by an orphaned PID at a different path (the squatter).
	oldExe := "/proc/12345/old-exe-link-target"
	oldHash := s.writeFile(oldExe, []byte("workflow-binary-v1-old"))
	const pid = 12345
	s.exeLinks[pid] = oldExe

	s.showFn = func(_ context.Context, _ string, _ ...string) (map[string]string, error) {
		return map[string]string{
			"ActiveState": "active", "SubState": "running",
			"Type": "simple", "FragmentPath": "/etc/systemd/system/globular-workflow.service",
			"MainPID": "12345",
		}, nil
	}
	// Provide the unit file so we don't trip an unrelated error.
	s.writeFile("/etc/systemd/system/globular-workflow.service", []byte("stub"))

	p := collectServiceRuntimeProof(context.Background(), "node-1", pkg, deps)

	if p.GetInstalledSha256() != newHash {
		t.Errorf("installed sha256 should be NEW: got=%s want=%s", p.GetInstalledSha256(), newHash)
	}
	if p.GetRunningExeSha256() != oldHash {
		t.Errorf("running exe sha256 should be OLD (orphan): got=%s want=%s",
			p.GetRunningExeSha256(), oldHash)
	}
	if p.GetInstalledSha256() == p.GetRunningExeSha256() {
		t.Fatal("test wired wrong — installed and running hashes should differ; this is the whole point of the test")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Systemctl show error — proof returns DEGRADED (errors[] populated) rather
// than nil. The consumer raises service.runtime_identity_unproven.
// ─────────────────────────────────────────────────────────────────────────

func TestCollectServiceRuntimeProof_SystemctlShowError_DegradedNotFatal(t *testing.T) {
	_ = withTempBinDir(t)
	deps, s := newStubDeps(t)

	pkg := &node_agentpb.InstalledPackage{Name: "rbac", Kind: "SERVICE"}
	s.writeFile(installedBinaryPath("rbac", "SERVICE"), []byte("rbac-payload"))

	s.showFn = func(_ context.Context, _ string, _ ...string) (map[string]string, error) {
		return nil, errors.New("D-Bus connection refused")
	}

	p := collectServiceRuntimeProof(context.Background(), "node-1", pkg, deps)
	if p == nil {
		t.Fatal("proof must never be nil — partial is the contract")
	}
	if !containsErrorContaining(p.GetErrors(), "systemctl show") {
		t.Errorf("expected systemctl error captured, got: %v", p.GetErrors())
	}
	// Disk hash should still be present — that probe ran before systemctl.
	if p.GetInstalledSha256() == "" {
		t.Error("installed_sha256 should be populated even when systemctl fails")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Inactive unit — MainPID="0" means no live process. The proof should not
// fabricate a running_exe; absence is the truth.
// ─────────────────────────────────────────────────────────────────────────

func TestCollectServiceRuntimeProof_NoMainPID_NoRunningExe(t *testing.T) {
	_ = withTempBinDir(t)
	deps, s := newStubDeps(t)

	pkg := &node_agentpb.InstalledPackage{Name: "blog", Kind: "SERVICE"}
	s.writeFile(installedBinaryPath("blog", "SERVICE"), []byte("blog"))
	s.writeFile("/etc/systemd/system/globular-blog.service", []byte("unit"))

	s.showFn = func(_ context.Context, _ string, _ ...string) (map[string]string, error) {
		return map[string]string{
			"ActiveState":  "inactive",
			"SubState":     "dead",
			"Type":         "simple",
			"FragmentPath": "/etc/systemd/system/globular-blog.service",
			"MainPID":      "0",
		}, nil
	}

	p := collectServiceRuntimeProof(context.Background(), "node-1", pkg, deps)
	if p.GetRunningPid() != 0 {
		t.Errorf("running_pid should be 0 for inactive unit, got %d", p.GetRunningPid())
	}
	if p.GetRunningExePath() != "" || p.GetRunningExeSha256() != "" {
		t.Errorf("running_exe should be empty for inactive unit: %+v", p)
	}
	if p.GetSystemdActiveState() != "inactive" {
		t.Errorf("ActiveState=%s want=inactive", p.GetSystemdActiveState())
	}
}

// ─────────────────────────────────────────────────────────────────────────
// COMMAND kind — no systemd unit; the proof returns the disk hash + a
// marker error explaining why systemd was skipped. This guards against
// accidentally trying to systemctl show globular-etcdctl.service (which
// would error and pollute the proof).
// ─────────────────────────────────────────────────────────────────────────

func TestCollectServiceRuntimeProof_CommandKindSkipsSystemd(t *testing.T) {
	_ = withTempBinDir(t)
	deps, s := newStubDeps(t)

	called := false
	s.showFn = func(_ context.Context, _ string, _ ...string) (map[string]string, error) {
		called = true
		return map[string]string{}, nil
	}
	pkg := &node_agentpb.InstalledPackage{Name: "etcdctl", Kind: "COMMAND"}
	hash := s.writeFile(installedBinaryPath("etcdctl", "COMMAND"), []byte("etcdctl-bin"))

	p := collectServiceRuntimeProof(context.Background(), "node-1", pkg, deps)
	if called {
		t.Error("COMMAND kind must not call systemctl show")
	}
	if p.GetInstalledSha256() != hash {
		t.Errorf("disk hash still required for COMMAND: got=%s want=%s", p.GetInstalledSha256(), hash)
	}
	if !containsErrorContaining(p.GetErrors(), "kind=COMMAND") {
		t.Errorf("expected COMMAND skip marker: %v", p.GetErrors())
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Timestamp parser — pins the layouts the parser accepts. systemd emits
// ExecMainStartTimestamp in locale-dependent forms; missing a layout would
// silently drop process_start_time and mask the old_pid_after_upgrade
// finding.
// ─────────────────────────────────────────────────────────────────────────

func TestParseSystemdTimestamp_AcceptsCanonicalLayouts(t *testing.T) {
	cases := []string{
		"Mon 2026-05-20 11:00:00 UTC",
		"Mon 2026-05-20 11:00:00 -0400",
	}
	for _, s := range cases {
		if _, err := parseSystemdTimestamp(s); err != nil {
			t.Errorf("parseSystemdTimestamp(%q) returned err=%v; consumers depend on this format", s, err)
		}
	}
}

func TestParseSystemdTimestamp_RejectsBogus(t *testing.T) {
	if _, err := parseSystemdTimestamp("n/a"); err == nil {
		t.Error("expected error for 'n/a'")
	}
	if _, err := parseSystemdTimestamp(""); err == nil {
		t.Error("expected error for empty string")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────

func containsErrorContaining(errors []string, sub string) bool {
	for _, e := range errors {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}
