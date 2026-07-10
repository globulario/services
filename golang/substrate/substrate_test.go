package substrate

import (
	"context"
	"encoding/base64"
	"os"
	"sort"
	"strings"
	"testing"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

// ── fake KV ──────────────────────────────────────────────────────────────────

type fakeKV struct {
	data map[string]*mvccpb.KeyValue
	rev  int64
}

func newFakeKV() *fakeKV {
	return &fakeKV{data: map[string]*mvccpb.KeyValue{}, rev: 1}
}

func (f *fakeKV) put(key string, val []byte, lease bool) {
	f.rev++
	kv := &mvccpb.KeyValue{
		Key:            []byte(key),
		Value:          append([]byte(nil), val...),
		CreateRevision: f.rev,
		ModRevision:    f.rev,
		Version:        1,
	}
	if lease {
		kv.Lease = 42
	}
	f.data[key] = kv
}

func (f *fakeKV) Range(_ context.Context, start, end string, _, limit int64) ([]*mvccpb.KeyValue, int64, bool, error) {
	var keys []string
	for k := range f.data {
		if k >= start && k < end {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	more := false
	if int64(len(keys)) > limit {
		keys = keys[:limit]
		more = true
	}
	var out []*mvccpb.KeyValue
	for _, k := range keys {
		out = append(out, f.data[k])
	}
	return out, f.rev, more, nil
}

func (f *fakeKV) Get(_ context.Context, key string) (*mvccpb.KeyValue, int64, error) {
	return f.data[key], f.rev, nil
}

func (f *fakeKV) Put(_ context.Context, key string, val []byte) error {
	f.put(key, val, false)
	return nil
}

// ── classification ───────────────────────────────────────────────────────────

func TestClassify(t *testing.T) {
	cases := []struct {
		key    string
		policy RestorePolicy
		known  bool
	}{
		// identity / trust / counters / audit
		{"/globular/system/cluster/id", RestoreAuthoritative, true},
		{"/globular/system/config", RestoreAuthoritative, true},
		{"/globular/pki/ca.crt", RestoreAuthoritative, true},
		{"/globular/secrets/anthropic-credentials", RestoreAuthoritative, true},
		{"/globular/audit/desired_writes/x", RestoreAuthoritative, true},
		{"/globular/clustercontroller/epoch", RestoreAuthoritative, true},
		{"/globular/nodes/bootstrap_marker", RestoreAuthoritative, true},
		// desired state
		{"/globular/resources/DesiredService/dns", RestoreAsUnverified, true},
		{"/globular/platform/active_release", RestoreAsUnverified, true},
		{"/globular/services/echo.EchoService/config", RestoreAsUnverified, true},
		{"/globular/workflows/node.join", RestoreAsUnverified, true},
		{"/globular/domains/v1/globular.io", RestoreAsUnverified, true},
		{"/globular/clustercontroller/state", RestoreAsUnverified, true},
		// observations
		{"/globular/nodes/abc123/packages/SERVICE/echo", RebuildFromObservation, true},
		{"/globular/cluster/dns/hosts", RebuildFromObservation, true},
		{"/globular/system/etcd_endpoints", RebuildFromObservation, true},
		// ephemera
		{"/globular/clustercontroller/leader", Discard, true},
		{"/globular/clustercontroller/leader/addr", Discard, true},
		{"/globular/workflow/runs/run-1", Discard, true},
		{"/globular/controller/node_removals/requests/n1", Discard, true},
		{"/globular/approvals/delete/release/x", Discard, true},
		{"/globular/recovery/v1/restore", Discard, true},
		{"/globular/tokens/sa", Discard, true},
		{"/globular/system/posture", Discard, true},
		// structural overrides
		{"/globular/locks/start/echo", Discard, true},
		{"/globular/pki/locks/ca", Discard, true},
		{"/globular/migrations/scylla/ai_memory/lock", Discard, true},
		{"/globular/services/echo.EchoService/instances/node-1", Discard, true},
		{"/globular/services/echo.EchoService/runtime", Discard, true},
		// specific beats general (longest prefix wins)
		{"/globular/resources/bootstrap_marker", RestoreAuthoritative, true},
		{"/globular/scylla/schema_guard/enforce_request", Discard, true},
		{"/globular/scylla/schema_guard/bootstrap_marker", RestoreAuthoritative, true},
		// unknown → unverified, flagged
		{"/globular/some_future_subsystem/key", RestoreAsUnverified, false},
	}
	for _, c := range cases {
		got := Classify(c.key)
		if got.Policy != c.policy || got.Known != c.known {
			t.Errorf("Classify(%s) = (%s, known=%v), want (%s, known=%v)",
				c.key, got.Policy, got.Known, c.policy, c.known)
		}
	}
}

func TestClassify_MigrationStateRestoresButLocksDiscard(t *testing.T) {
	if got := Classify("/globular/migrations/scylla/ai_memory/state"); got.Policy != RestoreAuthoritative {
		t.Errorf("migration state must restore authoritatively, got %s", got.Policy)
	}
	if got := Classify("/globular/migrations/scylla/ai_memory/lock"); got.Policy != Discard {
		t.Errorf("migration lock must be discarded, got %s", got.Policy)
	}
}

// ── dump / restore round trip ────────────────────────────────────────────────

func seedRepresentativeCluster(f *fakeKV) {
	f.put("/globular/system/cluster/id", []byte("uid-alpha"), false)
	f.put("/globular/system/config", []byte(`{"Name":"globular"}`), false)
	f.put("/globular/secrets/anthropic-credentials", []byte("s3cret"), false)
	f.put("/globular/resources/DesiredService/dns", []byte(`{"v":"1.2.3"}`), false)
	f.put("/globular/platform/active_release", []byte(`{"platform_release":"1.2.271"}`), false)
	f.put("/globular/nodes/n1/packages/SERVICE/echo", []byte(`{"installed":true}`), false) // rebuild
	f.put("/globular/nodes/n1/status", []byte(`{"hb":"now"}`), false)                      // rebuild (nodes/)
	f.put("/globular/workflow/runs/run-1", []byte(`{"state":"RUNNING"}`), false)           // discard
	f.put("/globular/clustercontroller/leader", []byte("n1"), true)                        // discard + lease
	f.put("/globular/locks/start/echo", []byte("held"), true)                              // discard + lease
	f.put("/globular/approvals/delete/release/echo", []byte("approved"), false)            // discard
	f.put("/globular/recovery/v1/restore", []byte(`{"status":"stale"}`), false)            // never restored
	f.put("/globular/some_future_subsystem/key", []byte("x"), false)                       // unknown
}

func TestDumpRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := newFakeKV()
	seedRepresentativeCluster(src)

	dump, err := TakeDump(ctx, src, true)
	if err != nil {
		t.Fatalf("TakeDump: %v", err)
	}
	if dump.Manifest.ClusterUID != "uid-alpha" {
		t.Errorf("manifest cluster UID: got %q, want uid-alpha", dump.Manifest.ClusterUID)
	}
	if dump.Manifest.KeyCount != len(src.data) {
		t.Errorf("dump must capture the FULL keyspace: got %d keys, want %d", dump.Manifest.KeyCount, len(src.data))
	}
	if !dump.Manifest.SerializableRead {
		t.Error("serializable flag must be recorded in the manifest")
	}

	// Round-trip through disk with integrity checks.
	dir := t.TempDir()
	path, err := dump.WriteFile(dir)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	loaded, err := ReadDumpFile(path)
	if err != nil {
		t.Fatalf("ReadDumpFile: %v", err)
	}

	dst := newFakeKV()
	res, err := RestoreDump(ctx, dst, loaded, RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreDump: %v", err)
	}

	// Restored: identity + secret (authoritative), desired service + release (unverified),
	// unknown key (unverified, flagged).
	for _, key := range []string{
		"/globular/system/cluster/id",
		"/globular/system/config",
		"/globular/secrets/anthropic-credentials",
		"/globular/resources/DesiredService/dns",
		"/globular/platform/active_release",
		"/globular/some_future_subsystem/key",
	} {
		if dst.data[key] == nil {
			t.Errorf("key %s must be restored", key)
		}
	}
	// Never restored: observations, transient runs, leased keys, stale approvals,
	// prior restore markers.
	for _, key := range []string{
		"/globular/nodes/n1/packages/SERVICE/echo",
		"/globular/nodes/n1/status",
		"/globular/workflow/runs/run-1",
		"/globular/clustercontroller/leader",
		"/globular/locks/start/echo",
		"/globular/approvals/delete/release/echo",
	} {
		if dst.data[key] != nil {
			t.Errorf("key %s must NOT be restored", key)
		}
	}
	if string(dst.data["/globular/secrets/anthropic-credentials"].Value) != "s3cret" {
		t.Error("restored value mismatch")
	}

	// The marker must exist, be UNVERIFIED, and not be the stale one.
	marker, err := ReadMarker(ctx, dst)
	if err != nil || marker == nil {
		t.Fatalf("ReadMarker: %v, marker=%v", err, marker)
	}
	if marker.Status != StatusRestoredUnverified {
		t.Errorf("marker status: got %s, want %s", marker.Status, StatusRestoredUnverified)
	}
	if marker.DumpClusterUID != "uid-alpha" {
		t.Errorf("marker dump cluster uid: got %s", marker.DumpClusterUID)
	}

	// Unknown prefix must be reported.
	if len(res.UnknownPrefixes) != 1 || !strings.Contains(res.UnknownPrefixes[0], "some_future_subsystem") {
		t.Errorf("unknown prefixes: got %v, want the future subsystem flagged", res.UnknownPrefixes)
	}
}

func TestRestore_LiveKeysWinWithoutForce(t *testing.T) {
	ctx := context.Background()
	src := newFakeKV()
	seedRepresentativeCluster(src)
	dump, err := TakeDump(ctx, src, false)
	if err != nil {
		t.Fatalf("TakeDump: %v", err)
	}

	dst := newFakeKV()
	dst.put("/globular/system/cluster/id", []byte("uid-alpha"), false) // same cluster
	dst.put("/globular/resources/DesiredService/dns", []byte(`{"v":"9.9.9-newer-live"}`), false)

	res, err := RestoreDump(ctx, dst, dump, RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreDump: %v", err)
	}
	if got := string(dst.data["/globular/resources/DesiredService/dns"].Value); got != `{"v":"9.9.9-newer-live"}` {
		t.Errorf("live key was overwritten without --force: %s", got)
	}
	if res.SkippedExisting < 2 { // cluster id + desired service
		t.Errorf("SkippedExisting = %d, want >= 2", res.SkippedExisting)
	}

	// With Force, the dump value wins.
	if _, err := RestoreDump(ctx, dst, dump, RestoreOptions{Force: true}); err != nil {
		t.Fatalf("RestoreDump force: %v", err)
	}
	if got := string(dst.data["/globular/resources/DesiredService/dns"].Value); got != `{"v":"1.2.3"}` {
		t.Errorf("force restore must overwrite: %s", got)
	}
}

func TestRestore_RefusesForeignCluster(t *testing.T) {
	ctx := context.Background()
	src := newFakeKV()
	seedRepresentativeCluster(src)
	dump, _ := TakeDump(ctx, src, false)

	dst := newFakeKV()
	dst.put("/globular/system/cluster/id", []byte("uid-OTHER"), false)

	if _, err := RestoreDump(ctx, dst, dump, RestoreOptions{}); err == nil {
		t.Fatal("restoring a dump from another cluster must be refused without --force")
	}
	if _, err := RestoreDump(ctx, dst, dump, RestoreOptions{Force: true}); err != nil {
		t.Fatalf("force must override the cluster guard: %v", err)
	}
}

func TestRestore_DryRunWritesNothing(t *testing.T) {
	ctx := context.Background()
	src := newFakeKV()
	seedRepresentativeCluster(src)
	dump, _ := TakeDump(ctx, src, false)

	dst := newFakeKV()
	before := len(dst.data)
	res, err := RestoreDump(ctx, dst, dump, RestoreOptions{DryRun: true})
	if err != nil {
		t.Fatalf("RestoreDump dry-run: %v", err)
	}
	if len(dst.data) != before {
		t.Errorf("dry-run wrote %d keys", len(dst.data)-before)
	}
	if res.Restored[RestoreAuthoritative] == 0 || res.Restored[RestoreAsUnverified] == 0 {
		t.Errorf("dry-run must still report what WOULD be restored: %v", res.Restored)
	}
}

func TestReadDumpFile_RejectsTamper(t *testing.T) {
	ctx := context.Background()
	src := newFakeKV()
	seedRepresentativeCluster(src)
	dump, _ := TakeDump(ctx, src, false)

	dir := t.TempDir()
	path, err := dump.WriteFile(dir)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, _ := readAll(path)
	// Entry values are JSON-marshaled []byte, i.e. base64 — tamper that form.
	secretB64 := base64.StdEncoding.EncodeToString([]byte("s3cret"))
	hackedB64 := base64.StdEncoding.EncodeToString([]byte("hacked"))
	tampered := strings.Replace(string(data), secretB64, hackedB64, 1)
	if tampered == string(data) {
		t.Fatal("test setup: tamper target not found")
	}
	if err := writeAll(path, []byte(tampered)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDumpFile(path); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tampered dump must fail the checksum: %v", err)
	}
}

func TestMarkVerified(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV()

	if _, err := MarkVerified(ctx, kv, ""); err == nil {
		t.Fatal("MarkVerified without a marker must refuse — nothing to attest")
	}

	if err := WriteMarker(ctx, kv, RestoreMarker{Status: StatusRestoredUnverified, Mode: "from-dump"}); err != nil {
		t.Fatal(err)
	}
	m, err := MarkVerified(ctx, kv, "operator checked convergence")
	if err != nil {
		t.Fatalf("MarkVerified: %v", err)
	}
	if m.Status != StatusRestoredVerified || m.VerifiedAt == "" {
		t.Errorf("marker after verify: %+v", m)
	}
}

func TestSelectLatestDump_OrdersByDesiredEpochNotTime(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Older desired state, written second (newer file).
	old := newFakeKV()
	old.put("/globular/system/cluster/id", []byte("uid-alpha"), false)
	old.put("/globular/resources/DesiredService/dns", []byte("v1"), false)
	oldDump, _ := TakeDump(ctx, old, false)

	// Newer desired state: more desired-surface writes → higher epoch.
	fresh := newFakeKV()
	fresh.put("/globular/system/cluster/id", []byte("uid-alpha"), false)
	fresh.put("/globular/resources/DesiredService/dns", []byte("v1"), false)
	fresh.put("/globular/resources/DesiredService/dns", []byte("v2"), false)
	fresh.put("/globular/resources/DesiredService/echo", []byte("v1"), false)
	freshDump, _ := TakeDump(ctx, fresh, false)

	if freshDump.Manifest.DesiredEpoch <= oldDump.Manifest.DesiredEpoch {
		t.Fatalf("test setup: fresh epoch %d must exceed old epoch %d",
			freshDump.Manifest.DesiredEpoch, oldDump.Manifest.DesiredEpoch)
	}

	// Distinct filenames regardless of write order.
	freshDump.Manifest.CreatedAt = "2026-07-10T01:00:00Z"
	oldDump.Manifest.CreatedAt = "2026-07-10T02:00:00Z" // old data, later timestamp
	if _, err := freshDump.WriteFile(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := oldDump.WriteFile(dir); err != nil {
		t.Fatal(err)
	}

	path, best, err := SelectLatestDump(dir, "uid-alpha")
	if err != nil {
		t.Fatalf("SelectLatestDump: %v", err)
	}
	if best.Manifest.DesiredEpoch != freshDump.Manifest.DesiredEpoch {
		t.Errorf("selected %s with epoch %d — timestamp beat desired epoch", path, best.Manifest.DesiredEpoch)
	}

	// A foreign cluster's dump is never selected.
	if _, _, err := SelectLatestDump(dir, "uid-OTHER"); err == nil {
		t.Error("selection must reject dumps from a different cluster UID")
	}
}

func TestDesiredEpoch_IgnoresEphemeralChurn(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV()
	kv.put("/globular/system/cluster/id", []byte("uid-alpha"), false)
	kv.put("/globular/resources/DesiredService/dns", []byte("v1"), false)
	d1, _ := TakeDump(ctx, kv, false)

	// Heartbeat / runtime churn only — desired epoch must NOT advance.
	kv.put("/globular/nodes/n1/status", []byte("hb1"), false)
	kv.put("/globular/system/posture", []byte("tick"), false)
	kv.put("/globular/workflow/runs/r1", []byte("running"), false)
	d2, _ := TakeDump(ctx, kv, false)

	if d2.Manifest.DesiredEpoch != d1.Manifest.DesiredEpoch {
		t.Errorf("ephemeral churn advanced desired_epoch: %d → %d",
			d1.Manifest.DesiredEpoch, d2.Manifest.DesiredEpoch)
	}

	// A desired-state write MUST advance it.
	kv.put("/globular/resources/DesiredService/dns", []byte("v2"), false)
	d3, _ := TakeDump(ctx, kv, false)
	if d3.Manifest.DesiredEpoch <= d2.Manifest.DesiredEpoch {
		t.Errorf("desired-state write did not advance desired_epoch: %d → %d",
			d2.Manifest.DesiredEpoch, d3.Manifest.DesiredEpoch)
	}
}

func TestAppendForceNewCluster(t *testing.T) {
	out, err := appendForceNewCluster([]byte("name: n1\ndata-dir: /var/lib/globular/etcd\n"))
	if err != nil {
		t.Fatalf("appendForceNewCluster: %v", err)
	}
	if !strings.HasSuffix(string(out), "force-new-cluster: true\n") {
		t.Errorf("flag not appended: %q", string(out))
	}
	if _, err := appendForceNewCluster(out); err == nil {
		t.Error("double-forcing must be refused — a prior recovery did not clean up")
	}
}

// ── Dump pagination ──────────────────────────────────────────────────────────

func TestTakeDump_Paginates(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV()
	for i := 0; i < rangePageLimit*2+7; i++ {
		kv.put("/globular/audit/rec-"+padded(i), []byte("x"), false)
	}
	d, err := TakeDump(ctx, kv, false)
	if err != nil {
		t.Fatalf("TakeDump: %v", err)
	}
	if d.Manifest.KeyCount != rangePageLimit*2+7 {
		t.Errorf("pagination lost keys: got %d, want %d", d.Manifest.KeyCount, rangePageLimit*2+7)
	}
	seen := map[string]bool{}
	for _, e := range d.Entries {
		if seen[e.Key] {
			t.Fatalf("duplicate key across pages: %s", e.Key)
		}
		seen[e.Key] = true
	}
}

// ── small helpers ────────────────────────────────────────────────────────────

func padded(i int) string {
	return strings.Repeat("0", 6-len(itoa(i))) + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func readAll(path string) ([]byte, error)  { return os.ReadFile(path) }
func writeAll(path string, b []byte) error { return os.WriteFile(path, b, 0o600) }
