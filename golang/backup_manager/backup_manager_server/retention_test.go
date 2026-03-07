package main

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

func makeArt(id string, createdMs int64, quality backup_managerpb.QualityState, totalBytes uint64) *backup_managerpb.BackupArtifact {
	return &backup_managerpb.BackupArtifact{
		BackupId:      id,
		CreatedUnixMs: createdMs,
		QualityState:  quality,
		TotalBytes:    totalBytes,
	}
}

func TestRetention_PromotedNeverDeleted(t *testing.T) {
	srv := &server{RetentionKeepLastN: 2}
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		makeArt("a", now, backup_managerpb.QualityState_QUALITY_PROMOTED, 100),
		makeArt("b", now-1000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("c", now-2000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("d", now-3000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
	}

	toDelete, toKeep := srv.evaluateRetention(arts)

	// Promoted "a" + 2 non-promoted kept ("b", "c"), "d" deleted
	keptIDs := map[string]bool{}
	for _, a := range toKeep {
		keptIDs[a.BackupId] = true
	}
	if !keptIDs["a"] {
		t.Error("promoted backup 'a' should always be kept")
	}
	if !keptIDs["b"] || !keptIDs["c"] {
		t.Error("first 2 non-promoted should be kept")
	}
	if len(toDelete) != 1 || toDelete[0].BackupId != "d" {
		t.Errorf("expected 'd' to be deleted, got %v", toDelete)
	}
}

func TestRetention_PromotedDoesNotInflateKeepN(t *testing.T) {
	srv := &server{RetentionKeepLastN: 2}
	now := time.Now().UnixMilli()

	// Promoted backup is between two non-promoted. It should not count toward keep_last_n.
	arts := []*backup_managerpb.BackupArtifact{
		makeArt("a", now, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("promoted", now-500, backup_managerpb.QualityState_QUALITY_PROMOTED, 100),
		makeArt("b", now-1000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("c", now-2000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
	}

	toDelete, toKeep := srv.evaluateRetention(arts)

	keptIDs := map[string]bool{}
	for _, a := range toKeep {
		keptIDs[a.BackupId] = true
	}
	// "promoted" always kept, "a" and "b" kept (first 2 non-promoted), "c" deleted
	if !keptIDs["promoted"] {
		t.Error("promoted backup should always be kept")
	}
	if !keptIDs["a"] || !keptIDs["b"] {
		t.Error("first 2 non-promoted should be kept")
	}
	if len(toDelete) != 1 || toDelete[0].BackupId != "c" {
		t.Errorf("expected only 'c' deleted, got %v", ids(toDelete))
	}
}

func TestRetention_KeepDays(t *testing.T) {
	srv := &server{RetentionKeepDays: 7}
	now := time.Now().UnixMilli()
	oldMs := now - int64(10*24*time.Hour/time.Millisecond) // 10 days ago

	arts := []*backup_managerpb.BackupArtifact{
		makeArt("recent", now, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("old", oldMs, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
	}

	toDelete, toKeep := srv.evaluateRetention(arts)
	if len(toKeep) != 1 || toKeep[0].BackupId != "recent" {
		t.Errorf("expected 'recent' kept, got %v", ids(toKeep))
	}
	if len(toDelete) != 1 || toDelete[0].BackupId != "old" {
		t.Errorf("expected 'old' deleted, got %v", ids(toDelete))
	}
}

func TestRetention_MaxTotalBytes(t *testing.T) {
	srv := &server{RetentionMaxTotalBytes: 250}
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		makeArt("a", now, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("b", now-1000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("c", now-2000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
	}

	toDelete, toKeep := srv.evaluateRetention(arts)
	if len(toKeep) != 2 {
		t.Errorf("expected 2 kept (a+b=200 < 250), got %v", ids(toKeep))
	}
	if len(toDelete) != 1 || toDelete[0].BackupId != "c" {
		t.Errorf("expected 'c' deleted (a+b+c=300 > 250), got %v", ids(toDelete))
	}
}

func TestRetention_MaxTotalBytesWithPromoted(t *testing.T) {
	srv := &server{RetentionMaxTotalBytes: 250}
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		makeArt("promoted", now, backup_managerpb.QualityState_QUALITY_PROMOTED, 200),
		makeArt("a", now-1000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("b", now-2000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
	}

	toDelete, toKeep := srv.evaluateRetention(arts)
	// promoted=200 always kept. a=100 → total=300>250 → a deleted. b=100 → not even reached.
	keptIDs := map[string]bool{}
	for _, a := range toKeep {
		keptIDs[a.BackupId] = true
	}
	if !keptIDs["promoted"] {
		t.Error("promoted should be kept")
	}
	// promoted bytes (200) counted + a (100) = 300 > 250, so a deleted
	if len(toDelete) < 1 {
		t.Error("expected at least 1 deletion")
	}
}

func TestRetention_MinRestoreTestedToKeep(t *testing.T) {
	srv := &server{RetentionKeepLastN: 1, MinRestoreTestedToKeep: 1}
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		makeArt("newest", now, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("tested", now-1000, backup_managerpb.QualityState_QUALITY_RESTORE_TESTED, 100),
		makeArt("old", now-2000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
	}

	toDelete, toKeep := srv.evaluateRetention(arts)
	keptIDs := map[string]bool{}
	for _, a := range toKeep {
		keptIDs[a.BackupId] = true
	}
	if !keptIDs["newest"] {
		t.Error("newest should be kept by keep_last_n")
	}
	if !keptIDs["tested"] {
		t.Error("restore-tested should be kept by MinRestoreTestedToKeep")
	}
	if len(toDelete) != 1 || toDelete[0].BackupId != "old" {
		t.Errorf("expected 'old' deleted, got %v", ids(toDelete))
	}
}

func TestRetention_NoPolicyKeepsAll(t *testing.T) {
	srv := &server{}
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		makeArt("a", now, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("b", now-1000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
	}

	toDelete, toKeep := srv.evaluateRetention(arts)
	if len(toDelete) != 0 {
		t.Errorf("no policy should keep all, got %d deleted", len(toDelete))
	}
	if len(toKeep) != 2 {
		t.Errorf("no policy should keep all, got %d kept", len(toKeep))
	}
}

func TestRetention_DryRunOutput(t *testing.T) {
	srv := &server{RetentionKeepLastN: 1}
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		makeArt("keep", now, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
		makeArt("delete", now-1000, backup_managerpb.QualityState_QUALITY_UNVERIFIED, 100),
	}

	toDelete, toKeep := srv.evaluateRetention(arts)
	if len(toDelete) != 1 || toDelete[0].BackupId != "delete" {
		t.Errorf("expected 'delete' in toDelete, got %v", ids(toDelete))
	}
	if len(toKeep) != 1 || toKeep[0].BackupId != "keep" {
		t.Errorf("expected 'keep' in toKeep, got %v", ids(toKeep))
	}
}

func ids(arts []*backup_managerpb.BackupArtifact) []string {
	var out []string
	for _, a := range arts {
		out = append(out, a.BackupId)
	}
	return out
}
