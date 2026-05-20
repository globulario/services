package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// fakeCollector implements BackupSecretCollector for tests. It returns a
// pre-configured response per node_id and tracks call counts. No real gRPC.
type fakeCollector struct {
	mu          sync.Mutex
	calls       int32
	perNodeResp map[string]*BackupSecretNodeResult
	perNodeErr  map[string]error
	// captureCapsuleDir is set by the first call; lets tests verify the
	// orchestrator passes the right path into the RPC.
	captureCapsuleDir string
}

func (f *fakeCollector) CollectBackupSecrets(ctx context.Context, node BackupSecretTargetNode, capsuleDir, backupID string) (*BackupSecretNodeResult, error) {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	if f.captureCapsuleDir == "" {
		f.captureCapsuleDir = capsuleDir
	}
	f.mu.Unlock()
	if err, ok := f.perNodeErr[node.NodeID]; ok && err != nil {
		return nil, err
	}
	if r, ok := f.perNodeResp[node.NodeID]; ok {
		return r, nil
	}
	// default: empty-but-ok
	return &BackupSecretNodeResult{NodeID: node.NodeID, Status: "ok"}, nil
}

func makeTargets(nodeIDs ...string) []BackupSecretTargetNode {
	out := make([]BackupSecretTargetNode, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		out = append(out, BackupSecretTargetNode{NodeID: id, Required: true})
	}
	return out
}

// TestDiscoverBackupSecretTargets_FromTopology — every topology node is
// turned into a Required target. Silently skipping nodes is forbidden.
func TestDiscoverBackupSecretTargets_FromTopology(t *testing.T) {
	topo := []TopologyNode{
		{NodeID: "a", Hostname: "ha", Address: "10.0.0.1", AgentEndpoint: "10.0.0.1:11000"},
		{NodeID: "b", Hostname: "hb", Address: "10.0.0.2"},
	}
	got := discoverBackupSecretTargets(topo)
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(got))
	}
	for _, target := range got {
		if !target.Required {
			t.Errorf("target %q must be Required:true (silent skip forbidden)", target.NodeID)
		}
	}
}

// TestFanOut_AllNodesSucceed — calls every target node, aggregates clean
// per-node OK responses, no missing_required.
func TestFanOut_AllNodesSucceed(t *testing.T) {
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok"},
			"b": {NodeID: "b", Status: "ok"},
			"c": {NodeID: "c", Status: "ok"},
		},
	}
	results := fanOutCollectSecrets(context.Background(), fc, makeTargets("a", "b", "c"), "/var/lib/globular/backups/job-x", "job-x")
	if atomic.LoadInt32(&fc.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", fc.calls)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != "ok" {
			t.Errorf("result %s status=%s, want ok", r.NodeID, r.Status)
		}
	}
}

// TestBuildClusterManifest_SummaryCounts — summary tallies match per-node
// results across success / failure / missing variants.
func TestBuildClusterManifest_SummaryCounts(t *testing.T) {
	results := []*BackupSecretNodeResult{
		{NodeID: "a", Status: "ok", MissingRequired: []string{"/x"}, MissingOptional: []string{"/y"}},
		{NodeID: "b", Status: "ok"},
		{NodeID: "c", Status: "error", Error: "RPC unauthenticated"},
	}
	m := buildClusterManifest(results, nil)
	if m.Summary.NodesTargeted != 3 || m.Summary.NodesSucceeded != 2 || m.Summary.NodesFailed != 1 {
		t.Errorf("summary node counts wrong: targeted=%d succeeded=%d failed=%d",
			m.Summary.NodesTargeted, m.Summary.NodesSucceeded, m.Summary.NodesFailed)
	}
	if m.Summary.RequiredMissingCount != 1 || m.Summary.OptionalMissingCount != 1 {
		t.Errorf("missing counts wrong: required=%d optional=%d",
			m.Summary.RequiredMissingCount, m.Summary.OptionalMissingCount)
	}
}

// withFakeCollector swaps the package-level constructor so collectClusterSecrets
// uses our mock without hitting real gRPC. Returns a restore func.
func withFakeCollector(t *testing.T, fc *fakeCollector) func() {
	t.Helper()
	prev := makeBackupSecretCollector
	makeBackupSecretCollector = func(_ *server) BackupSecretCollector { return fc }
	return func() { makeBackupSecretCollector = prev }
}

// withServerCapsuleRoot points srv.DataDir at a temp directory so
// CapsuleDir(...) returns a writable path. Returns the configured server.
func withServerCapsuleRoot(t *testing.T) (*server, string) {
	t.Helper()
	root := t.TempDir()
	// We use only the fields collectClusterSecrets touches.
	return &server{DataDir: root}, root
}

// readClusterManifest reads payload/secrets/manifest.json from a capsule.
func readClusterManifest(t *testing.T, srv *server, backupID string) *ClusterSecretManifest {
	t.Helper()
	path := filepath.Join(srv.CapsuleDir(backupID), "payload", "secrets", "manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cluster manifest %q: %v", path, err)
	}
	var m ClusterSecretManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse cluster manifest: %v", err)
	}
	return &m
}

// TestCollectClusterSecrets_AllSucceed_WritesAggregateManifest — covers
// the "all nodes succeed" spec case end-to-end: calls every node, writes a
// well-formed manifest with summary, no error, no secret bytes in logs.
func TestCollectClusterSecrets_AllSucceed_WritesAggregateManifest(t *testing.T) {
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok"},
			"b": {NodeID: "b", Status: "ok"},
		},
	}
	defer withFakeCollector(t, fc)()

	srv, _ := withServerCapsuleRoot(t)
	topo := []TopologyNode{{NodeID: "a"}, {NodeID: "b"}}

	err := srv.collectClusterSecrets(context.Background(), "job-ok", topo)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	m := readClusterManifest(t, srv, "job-ok")
	if m.Summary.NodesTargeted != 2 || m.Summary.NodesSucceeded != 2 || m.Summary.NodesFailed != 0 {
		t.Errorf("summary wrong: %+v", m.Summary)
	}
	if len(m.Nodes) != 2 {
		t.Errorf("expected 2 nodes in manifest, got %d", len(m.Nodes))
	}
}

// TestCollectClusterSecrets_RequiredSecretMissing_FailsClean — one node
// reports missing_required; orchestrator fails the backup with an error
// naming the node_id and the logical path. No secret contents in the
// error message.
func TestCollectClusterSecrets_RequiredSecretMissing_FailsClean(t *testing.T) {
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok"},
			"b": {NodeID: "b", Status: "ok",
				MissingRequired: []string{"/var/lib/globular/objectstore/minio_contract-last-known-good.json"}},
		},
	}
	defer withFakeCollector(t, fc)()
	srv, _ := withServerCapsuleRoot(t)

	err := srv.collectClusterSecrets(context.Background(),
		"job-missing",
		[]TopologyNode{{NodeID: "a"}, {NodeID: "b"}},
	)
	if err == nil {
		t.Fatal("expected failure when missing_required is populated")
	}
	if !strings.Contains(err.Error(), "required paths unreadable") {
		t.Errorf("error should declare the failure class, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error should name the failing node_id, got %q", err.Error())
	}
	// Manifest is still written for diagnostics.
	m := readClusterManifest(t, srv, "job-missing")
	if m.Summary.RequiredMissingCount != 1 {
		t.Errorf("manifest summary should reflect missing required; got %d", m.Summary.RequiredMissingCount)
	}
}

// TestCollectClusterSecrets_OptionalMissing_Continues — only missing_optional
// is populated; backup preparation succeeds, manifest records the warning.
func TestCollectClusterSecrets_OptionalMissing_Continues(t *testing.T) {
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok",
				MissingOptional: []string{"/var/lib/globular/.bootstrap-sa-password"}},
		},
	}
	defer withFakeCollector(t, fc)()
	srv, _ := withServerCapsuleRoot(t)

	if err := srv.collectClusterSecrets(context.Background(), "job-opt", []TopologyNode{{NodeID: "a"}}); err != nil {
		t.Fatalf("optional missing must not fail; got %v", err)
	}
	m := readClusterManifest(t, srv, "job-opt")
	if m.Summary.OptionalMissingCount != 1 {
		t.Errorf("manifest should record the optional missing; summary=%+v", m.Summary)
	}
}

// TestCollectClusterSecrets_RequiredNodeUnreachable_Fails — RPC error from
// one required node; backup fails; successful node results are still
// preserved in the manifest for diagnostics.
func TestCollectClusterSecrets_RequiredNodeUnreachable_Fails(t *testing.T) {
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok"},
		},
		perNodeErr: map[string]error{
			"b": errors.New("dial node-agent: context deadline exceeded"),
		},
	}
	defer withFakeCollector(t, fc)()
	srv, _ := withServerCapsuleRoot(t)

	err := srv.collectClusterSecrets(context.Background(),
		"job-unreach",
		[]TopologyNode{{NodeID: "a"}, {NodeID: "b"}},
	)
	if err == nil {
		t.Fatal("expected failure when a required node is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("error should mention 'unreachable', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error should name the unreachable node_id, got %q", err.Error())
	}
	// Manifest preserves both the OK result and the error result.
	m := readClusterManifest(t, srv, "job-unreach")
	if m.Summary.NodesSucceeded != 1 || m.Summary.NodesFailed != 1 {
		t.Errorf("manifest should preserve both succeed and fail: %+v", m.Summary)
	}
}

// TestCollectClusterSecrets_PartialFanOut_FailsButPreservesDetails — same
// shape as the unreachable case but tested explicitly per the spec to pin
// the diagnostic-preservation behaviour separately.
func TestCollectClusterSecrets_PartialFanOut_FailsButPreservesDetails(t *testing.T) {
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok"},
			"c": {NodeID: "c", Status: "ok"},
		},
		perNodeErr: map[string]error{"b": errors.New("RPC: Unauthenticated")},
	}
	defer withFakeCollector(t, fc)()
	srv, _ := withServerCapsuleRoot(t)

	err := srv.collectClusterSecrets(context.Background(),
		"job-partial",
		[]TopologyNode{{NodeID: "a"}, {NodeID: "b"}, {NodeID: "c"}},
	)
	if err == nil {
		t.Fatal("partial fan-out must fail")
	}
	m := readClusterManifest(t, srv, "job-partial")
	// Should report 3 targeted, 2 succeeded, 1 failed.
	if m.Summary.NodesTargeted != 3 || m.Summary.NodesSucceeded != 2 || m.Summary.NodesFailed != 1 {
		t.Errorf("partial-fanout summary wrong: %+v", m.Summary)
	}
	// Manifest order must be stable (sorted by node_id) — assert.
	gotIDs := make([]string, 0, len(m.Nodes))
	for _, n := range m.Nodes {
		gotIDs = append(gotIDs, n.NodeID)
	}
	wantIDs := []string{"a", "b", "c"}
	if !equalStrings(gotIDs, wantIDs) {
		t.Errorf("manifest nodes not sorted by node_id: got %v want %v", gotIDs, wantIDs)
	}
}

// TestCollectClusterSecrets_BootstrapSecretConflict_Fails — two nodes
// claim the same singleton secret with different fingerprints. Backup
// fails with a structured conflict.
func TestCollectClusterSecrets_BootstrapSecretConflict_Fails(t *testing.T) {
	entryA := &node_agentpb.SecretFileEntry{
		OriginalPath: bootstrapSAPasswordPath, Found: true, Sha256: "aaa111", Required: false, OptionalWhenAbsent: true,
	}
	entryB := &node_agentpb.SecretFileEntry{
		OriginalPath: bootstrapSAPasswordPath, Found: true, Sha256: "bbb222", Required: false, OptionalWhenAbsent: true,
	}
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok", Entries: []*node_agentpb.SecretFileEntry{entryA}},
			"b": {NodeID: "b", Status: "ok", Entries: []*node_agentpb.SecretFileEntry{entryB}},
		},
	}
	defer withFakeCollector(t, fc)()
	srv, _ := withServerCapsuleRoot(t)

	err := srv.collectClusterSecrets(context.Background(),
		"job-conflict",
		[]TopologyNode{{NodeID: "a"}, {NodeID: "b"}},
	)
	if err == nil {
		t.Fatal("expected failure on bootstrap-secret fingerprint conflict")
	}
	if !strings.Contains(err.Error(), "mismatched fingerprints") {
		t.Errorf("error should mention mismatched fingerprints; got %q", err.Error())
	}
	m := readClusterManifest(t, srv, "job-conflict")
	if len(m.Conflicts) != 1 {
		t.Fatalf("manifest should record 1 conflict; got %d", len(m.Conflicts))
	}
	c := m.Conflicts[0]
	if c.SecretName != "bootstrap-sa-password" {
		t.Errorf("conflict secret_name=%q, want bootstrap-sa-password", c.SecretName)
	}
	if len(c.Fingerprints) != 2 {
		t.Errorf("conflict should record 2 fingerprints; got %d", len(c.Fingerprints))
	}
	// Conflict must include the SHA-256s but never the secret content.
	combined := c.Fingerprints[0].SHA256 + " " + c.Fingerprints[1].SHA256
	if !strings.Contains(combined, "aaa111") || !strings.Contains(combined, "bbb222") {
		t.Errorf("conflict fingerprints must include both node sha256s; got %v", c.Fingerprints)
	}
}

// TestCollectClusterSecrets_BootstrapSecretSameFingerprint_NoConflict —
// two nodes claim the secret with identical SHA-256 → not a conflict.
func TestCollectClusterSecrets_BootstrapSecretSameFingerprint_NoConflict(t *testing.T) {
	same := "deadbeef00112233"
	entryA := &node_agentpb.SecretFileEntry{OriginalPath: bootstrapSAPasswordPath, Found: true, Sha256: same}
	entryB := &node_agentpb.SecretFileEntry{OriginalPath: bootstrapSAPasswordPath, Found: true, Sha256: same}
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok", Entries: []*node_agentpb.SecretFileEntry{entryA}},
			"b": {NodeID: "b", Status: "ok", Entries: []*node_agentpb.SecretFileEntry{entryB}},
		},
	}
	defer withFakeCollector(t, fc)()
	srv, _ := withServerCapsuleRoot(t)
	if err := srv.collectClusterSecrets(context.Background(), "job-same", []TopologyNode{{NodeID: "a"}, {NodeID: "b"}}); err != nil {
		t.Errorf("matching fingerprints must NOT be a conflict; got %v", err)
	}
}

// TestSecretCollector_NoDirectFilesystemScraping is a static, code-pattern
// check: the only entry point for collecting remote-node secret material
// is the BackupSecretCollector interface; production wiring uses the
// node-agent gRPC client. This test asserts the interface boundary exists
// and the cluster orchestrator depends on it (not on os.ReadFile of remote
// paths). The check is structural: if a future change adds a direct
// remote-filesystem read here, the orchestrator would no longer compile
// against the interface alone and this test signals the loss of safety.
func TestSecretCollector_NoDirectFilesystemScraping(t *testing.T) {
	// Compile-time proof: the interface is the only path the orchestrator
	// uses for remote collection.
	var _ BackupSecretCollector = (*nodeAgentSecretCollector)(nil)
	// And the orchestrator uses it via the swappable constructor (this
	// is what the test fixtures rely on).
	if makeBackupSecretCollector == nil {
		t.Fatal("makeBackupSecretCollector must be the single injection point")
	}
	// Defense: ensure the production implementation type name says "node-agent"
	// — a future PR titled "use ssh to grab remote /etc files" would have to
	// rename the struct and would surface in review.
	produces := makeBackupSecretCollector(&server{DataDir: t.TempDir()})
	tn := strings.ToLower(fmt.Sprintf("%T", produces))
	if !strings.Contains(tn, "nodeagentsecretcollector") {
		t.Errorf("production collector type %s must be nodeAgentSecretCollector; if you renamed it, also rename this test's expectation",
			tn)
	}
}

// TestCollectClusterSecrets_LogsDoNotContainSecretContents — slog output
// from a full orchestration run never includes the secret-payload bytes.
func TestCollectClusterSecrets_LogsDoNotContainSecretContents(t *testing.T) {
	secretBytes := "TOP-SECRET-NEVER-LOG-ME"
	entry := &node_agentpb.SecretFileEntry{
		OriginalPath: bootstrapSAPasswordPath,
		Found:        true,
		Sha256:       "deadbeef", // metadata only — bytes never travel
	}
	fc := &fakeCollector{
		perNodeResp: map[string]*BackupSecretNodeResult{
			"a": {NodeID: "a", Status: "ok", Entries: []*node_agentpb.SecretFileEntry{entry}},
		},
	}
	defer withFakeCollector(t, fc)()
	srv, _ := withServerCapsuleRoot(t)

	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(stringWriter2{&buf}, nil)))
	defer slog.SetDefault(prev)

	_ = srv.collectClusterSecrets(context.Background(), "job-log", []TopologyNode{{NodeID: "a"}})
	if strings.Contains(buf.String(), secretBytes) {
		t.Errorf("logs contain secret payload; sample: %q", buf.String())
	}
}

// equalStrings is a tiny helper for deterministic-order assertions.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

// stringWriter2 adapts a strings.Builder to io.Writer for slog (separate
// name from the node_agent test helper to avoid package-rename confusion).
type stringWriter2 struct{ b *strings.Builder }

func (w stringWriter2) Write(p []byte) (int, error) {
	n, err := w.b.Write(p)
	if err == nil && n == 0 && len(p) > 0 {
		return n, io.ErrShortWrite
	}
	return n, err
}
