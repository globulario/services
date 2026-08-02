package substrate

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// Substrate restore marker ordering.
//
// The restore marker is a durable receipt, and its value depends entirely on
// WHEN it is written. A marker written before the data lands claims a restore
// that may not have happened; a marker written before validation claims a
// restore that may not be usable. RestoreDump and FromSurvivor already write it
// last — these tests pin that ordering so it cannot silently regress, since
// nothing else in the suite covers the failure paths.
//
// Behavior is exercised through the real RestoreDump seam with a KV whose Put
// fails on a chosen key. No production code is changed by this file.

// failingKV fails Put on any key containing failOn, and can count writes so a
// test can assert what had already landed when the failure hit.
type failingKV struct {
	*fakeKV
	failOn  string
	written []string
}

func (f *failingKV) Put(ctx context.Context, key string, val []byte) error {
	if f.failOn != "" && strings.Contains(key, f.failOn) {
		return errors.New("simulated etcd write failure")
	}
	f.written = append(f.written, key)
	return f.fakeKV.Put(ctx, key, val)
}

func newFailingKV(failOn string) *failingKV {
	return &failingKV{fakeKV: newFakeKV(), failOn: failOn}
}

func representativeDump(t *testing.T) *Dump {
	t.Helper()
	src := newFakeKV()
	seedRepresentativeCluster(src)
	d, err := TakeDump(context.Background(), src, true)
	if err != nil {
		t.Fatalf("TakeDump: %v", err)
	}
	return d
}

// 1. A per-key write failure mid-restore leaves NO success marker. The restore
// did not complete, so nothing may claim it did.
func TestRestoreMarker_PerKeyWriteFailureLeavesNoMarker(t *testing.T) {
	ctx := context.Background()
	dst := newFailingKV("DesiredService")
	if _, err := RestoreDump(ctx, dst, representativeDump(t), RestoreOptions{}); err == nil {
		t.Fatal("expected the restore to fail")
	}
	m, err := ReadMarker(ctx, dst)
	if err != nil {
		t.Fatalf("ReadMarker: %v", err)
	}
	if m != nil {
		t.Fatalf("failed restore left a success marker: %+v", m)
	}
}

// 2. A failure on the FIRST key — before anything has been mutated — also
// leaves no marker, and leaves the destination untouched.
func TestRestoreMarker_FailureBeforeMutationLeavesNoMarkerAndNoData(t *testing.T) {
	ctx := context.Background()
	dst := newFailingKV("/globular/")
	if _, err := RestoreDump(ctx, dst, representativeDump(t), RestoreOptions{}); err == nil {
		t.Fatal("expected the restore to fail")
	}
	if len(dst.written) != 0 {
		t.Errorf("nothing should have been written, got %d keys", len(dst.written))
	}
	if m, _ := ReadMarker(ctx, dst); m != nil {
		t.Fatalf("marker written despite zero mutations: %+v", m)
	}
}

// 3. A marker-write failure is SURFACED, not swallowed. The data landed but the
// receipt did not, and the caller must learn that rather than assume success.
func TestRestoreMarker_MarkerWriteFailureIsSurfaced(t *testing.T) {
	ctx := context.Background()
	dst := newFailingKV(RestoreMarkerKey)
	_, err := RestoreDump(ctx, dst, representativeDump(t), RestoreOptions{})
	if err == nil {
		t.Fatal("marker-write failure must surface as an error, not silent success")
	}
	if !strings.Contains(err.Error(), "marker") {
		t.Errorf("error should identify the marker write: %v", err)
	}
	if m, _ := ReadMarker(ctx, dst); m != nil {
		t.Fatalf("marker readable after its write failed: %+v", m)
	}
}

// 4. A dry run writes no marker — and no data.
func TestRestoreMarker_DryRunWritesNoMarker(t *testing.T) {
	ctx := context.Background()
	dst := newFailingKV("")
	if _, err := RestoreDump(ctx, dst, representativeDump(t), RestoreOptions{DryRun: true}); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(dst.written) != 0 {
		t.Errorf("dry run wrote %d keys", len(dst.written))
	}
	if m, _ := ReadMarker(ctx, dst); m != nil {
		t.Fatalf("dry run wrote a marker: %+v", m)
	}
}

// 5. On success the marker is written LAST — after every data write.
func TestRestoreMarker_SuccessWritesMarkerAfterAllDataWrites(t *testing.T) {
	ctx := context.Background()
	dst := newFailingKV("")
	res, err := RestoreDump(ctx, dst, representativeDump(t), RestoreOptions{})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(dst.written) == 0 {
		t.Fatal("restore wrote nothing")
	}
	last := dst.written[len(dst.written)-1]
	if last != RestoreMarkerKey {
		t.Errorf("last write was %q, want the marker %q — the marker must be written "+
			"after all restored data", last, RestoreMarkerKey)
	}
	for _, k := range dst.written[:len(dst.written)-1] {
		if k == RestoreMarkerKey {
			t.Error("marker written more than once, or before the data")
		}
	}
	if res == nil {
		t.Fatal("nil result on success")
	}
}

// 6. The marker binds to the restored generation: the dump's payload digest,
// cluster UID and desired epoch. A receipt that does not name what it restored
// cannot be checked against anything later.
func TestRestoreMarker_BindsToRestoredGeneration(t *testing.T) {
	ctx := context.Background()
	d := representativeDump(t)
	dst := newFakeKV()
	if _, err := RestoreDump(ctx, dst, d, RestoreOptions{}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	m, err := ReadMarker(ctx, dst)
	if err != nil || m == nil {
		t.Fatalf("ReadMarker: %v (marker=%v)", err, m)
	}
	if m.DumpSHA256 == "" || m.DumpSHA256 != d.Manifest.PayloadSHA256 {
		t.Errorf("DumpSHA256=%q want=%q", m.DumpSHA256, d.Manifest.PayloadSHA256)
	}
	if m.DumpClusterUID != d.Manifest.ClusterUID {
		t.Errorf("DumpClusterUID=%q want=%q", m.DumpClusterUID, d.Manifest.ClusterUID)
	}
	if m.RestoredAt == "" {
		t.Error("marker records no restore time")
	}
}

// 7. A fresh restore is RESTORED_UNVERIFIED — never verified. Restoring desired
// state is not the same as having reconciled it against observed reality.
func TestRestoreMarker_FreshRestoreIsNotVerified(t *testing.T) {
	ctx := context.Background()
	dst := newFakeKV()
	if _, err := RestoreDump(ctx, dst, representativeDump(t), RestoreOptions{}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	m, _ := ReadMarker(ctx, dst)
	if m == nil {
		t.Fatal("no marker after successful restore")
	}
	if m.Status != StatusRestoredUnverified {
		t.Fatalf("fresh restore reported %q; a restore cannot self-certify as %q",
			m.Status, StatusRestoredVerified)
	}
	if m.VerifiedAt != "" {
		t.Errorf("fresh restore carries a verification timestamp: %q", m.VerifiedAt)
	}
}

// 8. MarkVerified is a distinct LATER transition, and it refuses to invent one:
// with no marker present there is nothing to attest to.
func TestRestoreMarker_MarkVerifiedIsADistinctLaterTransition(t *testing.T) {
	ctx := context.Background()

	// No restore has happened — attestation must be refused.
	if _, err := MarkVerified(ctx, newFakeKV(), "operator attests"); err == nil {
		t.Error("MarkVerified must refuse when no restore marker exists")
	}

	dst := newFakeKV()
	if _, err := RestoreDump(ctx, dst, representativeDump(t), RestoreOptions{}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	before, _ := ReadMarker(ctx, dst)
	m, err := MarkVerified(ctx, dst, "operator attests")
	if err != nil {
		t.Fatalf("MarkVerified: %v", err)
	}
	if m.Status != StatusRestoredVerified || m.VerifiedAt == "" {
		t.Errorf("MarkVerified did not record the attestation: %+v", m)
	}
	// The transition must preserve what was restored, not re-stamp it.
	if m.DumpSHA256 != before.DumpSHA256 {
		t.Errorf("verification changed the restored generation: %q -> %q",
			before.DumpSHA256, m.DumpSHA256)
	}
}

// 9. A prior restore marker carried INSIDE a dump is never resurrected. Without
// this, restoring an old dump would republish that dump's stale receipt as if
// it described the restore just performed.
func TestRestoreMarker_StaleMarkerInDumpIsNotResurrected(t *testing.T) {
	ctx := context.Background()

	// Build a source that already carries a restore marker, then dump it.
	src := newFakeKV()
	seedRepresentativeCluster(src)
	if err := WriteMarker(ctx, src, RestoreMarker{
		Status: StatusRestoredVerified, Mode: "from-dump", DumpSHA256: "staledigest",
	}); err != nil {
		t.Fatalf("seed stale marker: %v", err)
	}
	d, err := TakeDump(ctx, src, true)
	if err != nil {
		t.Fatalf("TakeDump: %v", err)
	}

	dst := newFakeKV()
	if _, err := RestoreDump(ctx, dst, d, RestoreOptions{}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	m, _ := ReadMarker(ctx, dst)
	if m == nil {
		t.Fatal("no marker after restore")
	}
	if m.DumpSHA256 == "staledigest" || m.Status == StatusRestoredVerified {
		t.Fatalf("the dump's own stale marker was resurrected as this restore's receipt: %+v", m)
	}
	if m.DumpSHA256 != d.Manifest.PayloadSHA256 {
		t.Errorf("marker does not describe the restore just performed: %q want %q",
			m.DumpSHA256, d.Manifest.PayloadSHA256)
	}
}

// 10. Existing live state survives a failed replacement: without Force, live
// keys always win, so a partial restore cannot destroy newer evidence.
func TestRestoreMarker_ExistingStateSurvivesFailedReplacement(t *testing.T) {
	ctx := context.Background()
	d := representativeDump(t)

	dst := newFailingKV("DesiredService")
	// Pre-existing live value that the dump also carries.
	live := []byte(`{"live":"newer"}`)
	for _, e := range d.Entries {
		if strings.Contains(e.Key, "/globular/") && !e.Lease {
			dst.fakeKV.put(e.Key, live, false)
			break
		}
	}
	before := len(dst.fakeKV.data)

	_, _ = RestoreDump(ctx, dst, d, RestoreOptions{})

	if len(dst.fakeKV.data) < before {
		t.Errorf("failed restore destroyed pre-existing state: %d -> %d keys",
			before, len(dst.fakeKV.data))
	}
	if m, _ := ReadMarker(ctx, dst); m != nil {
		t.Fatalf("failed replacement left a success marker: %+v", m)
	}
}

// Structural ratchet — support only. The behavioral tests above are the real
// contract; this pins the ordering in the source so a refactor that moves the
// marker write above the data loop is caught at review time rather than by a
// failing restore in production.
func TestRestoreMarker_OrderingRatchet(t *testing.T) {
	raw, err := os.ReadFile("restore.go")
	if err != nil {
		t.Fatalf("read restore.go: %v", err)
	}
	var b strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	src := b.String()

	fn := src[strings.Index(src, "func RestoreDump("):]
	if end := strings.Index(fn, "\nfunc "); end > 0 {
		fn = fn[:end]
	}
	putIdx := strings.Index(fn, "kv.Put(ctx, e.Key, e.Value)")
	markerIdx := strings.Index(fn, "WriteMarker(ctx, kv, marker)")
	if putIdx < 0 || markerIdx < 0 {
		t.Fatalf("could not locate the data write (%d) and marker write (%d) in RestoreDump",
			putIdx, markerIdx)
	}
	if markerIdx < putIdx {
		t.Error("the restore marker is written BEFORE the per-key data writes")
	}
	if !strings.Contains(fn[:markerIdx], "if !opts.DryRun {") {
		t.Error("the marker write is not gated on DryRun")
	}
}
