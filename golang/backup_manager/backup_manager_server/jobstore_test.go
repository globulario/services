package main

import (
	"os"
	"testing"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

func setupTestStore(t *testing.T) *jobStore {
	t.Helper()
	dir := t.TempDir()
	store, err := newJobStore(dir)
	if err != nil {
		t.Fatalf("newJobStore: %v", err)
	}
	return store
}

func TestListArtifacts_FilterByMode(t *testing.T) {
	store := setupTestStore(t)
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		{BackupId: "cluster1", CreatedUnixMs: now, Mode: backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER},
		{BackupId: "service1", CreatedUnixMs: now - 1000, Mode: backup_managerpb.BackupMode_BACKUP_MODE_SERVICE},
		{BackupId: "cluster2", CreatedUnixMs: now - 2000, Mode: backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER},
	}
	for _, a := range arts {
		if err := store.SaveArtifact(a); err != nil {
			t.Fatalf("SaveArtifact: %v", err)
		}
	}

	// Filter cluster only
	result, total, err := store.ListArtifacts("", backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER, 0, 0, 0)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total=2 cluster backups, got %d", total)
	}
	for _, r := range result {
		if r.Mode != backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER {
			t.Errorf("expected cluster mode, got %v", r.Mode)
		}
	}

	// Filter service only
	result, total, err = store.ListArtifacts("", backup_managerpb.BackupMode_BACKUP_MODE_SERVICE, 0, 0, 0)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 service backup, got %d", total)
	}

	// No filter
	result, total, err = store.ListArtifacts("", 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total=3 backups, got %d", total)
	}
	_ = result
}

func TestListArtifacts_FilterByQualityState(t *testing.T) {
	store := setupTestStore(t)
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		{BackupId: "unverified", CreatedUnixMs: now, QualityState: backup_managerpb.QualityState_QUALITY_UNVERIFIED},
		{BackupId: "validated", CreatedUnixMs: now - 1000, QualityState: backup_managerpb.QualityState_QUALITY_VALIDATED},
		{BackupId: "promoted", CreatedUnixMs: now - 2000, QualityState: backup_managerpb.QualityState_QUALITY_PROMOTED},
	}
	for _, a := range arts {
		if err := store.SaveArtifact(a); err != nil {
			t.Fatalf("SaveArtifact: %v", err)
		}
	}

	result, total, err := store.ListArtifacts("", 0, backup_managerpb.QualityState_QUALITY_PROMOTED, 0, 0)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if total != 1 || result[0].BackupId != "promoted" {
		t.Errorf("expected only 'promoted', got %v", result)
	}
}

func TestListArtifacts_CombinedFilters(t *testing.T) {
	store := setupTestStore(t)
	now := time.Now().UnixMilli()

	arts := []*backup_managerpb.BackupArtifact{
		{BackupId: "a", CreatedUnixMs: now, Mode: backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER, QualityState: backup_managerpb.QualityState_QUALITY_VALIDATED, PlanName: "daily"},
		{BackupId: "b", CreatedUnixMs: now - 1000, Mode: backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER, QualityState: backup_managerpb.QualityState_QUALITY_UNVERIFIED, PlanName: "daily"},
		{BackupId: "c", CreatedUnixMs: now - 2000, Mode: backup_managerpb.BackupMode_BACKUP_MODE_SERVICE, QualityState: backup_managerpb.QualityState_QUALITY_VALIDATED, PlanName: "manual"},
	}
	for _, a := range arts {
		if err := store.SaveArtifact(a); err != nil {
			t.Fatalf("SaveArtifact: %v", err)
		}
	}

	// Filter: cluster + validated
	result, total, err := store.ListArtifacts("", backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER, backup_managerpb.QualityState_QUALITY_VALIDATED, 0, 0)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if total != 1 || result[0].BackupId != "a" {
		t.Errorf("expected only 'a', got total=%d", total)
	}

	// Filter: plan_name=daily + cluster
	result, total, err = store.ListArtifacts("daily", backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER, 0, 0, 0)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 daily cluster backups, got %d", total)
	}
}

func TestListArtifacts_Pagination(t *testing.T) {
	store := setupTestStore(t)
	now := time.Now().UnixMilli()

	for i := 0; i < 5; i++ {
		a := &backup_managerpb.BackupArtifact{
			BackupId:      "art-" + string(rune('a'+i)),
			CreatedUnixMs: now - int64(i*1000),
		}
		if err := store.SaveArtifact(a); err != nil {
			t.Fatalf("SaveArtifact: %v", err)
		}
	}

	result, total, err := store.ListArtifacts("", 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results with limit=2, got %d", len(result))
	}

	result, _, err = store.ListArtifacts("", 0, 0, 2, 2)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results with limit=2 offset=2, got %d", len(result))
	}
}

func init() {
	// Suppress slog output during tests
	os.Setenv("SLOG_LEVEL", "ERROR")
}
