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

func TestArtifact_ValidationReportRoundTrip(t *testing.T) {
	store := setupTestStore(t)
	now := time.Now().UnixMilli()

	art := &backup_managerpb.BackupArtifact{
		BackupId:      "val-test",
		CreatedUnixMs: now,
		ValidationReport: &backup_managerpb.ValidationReport{
			Valid:             true,
			ValidatedAtUnixMs: now,
			Issues: []*backup_managerpb.ValidationIssue{
				{Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO, Code: "OK", Message: "all good"},
			},
			ReplicationChecks: []*backup_managerpb.ReplicationValidation{
				{DestinationName: "minio-1", Ok: true},
			},
		},
	}

	if err := store.SaveArtifact(art); err != nil {
		t.Fatalf("SaveArtifact: %v", err)
	}

	loaded, err := store.GetArtifact("val-test")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if loaded.ValidationReport == nil {
		t.Fatal("expected validation report to be persisted")
	}
	if !loaded.ValidationReport.Valid {
		t.Error("expected valid=true")
	}
	if loaded.ValidationReport.ValidatedAtUnixMs != now {
		t.Errorf("expected validated_at=%d, got %d", now, loaded.ValidationReport.ValidatedAtUnixMs)
	}
	if len(loaded.ValidationReport.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(loaded.ValidationReport.Issues))
	}
	if len(loaded.ValidationReport.ReplicationChecks) != 1 {
		t.Errorf("expected 1 replication check, got %d", len(loaded.ValidationReport.ReplicationChecks))
	}
}

func TestArtifact_RestoreTestReportRoundTrip(t *testing.T) {
	store := setupTestStore(t)
	now := time.Now().UnixMilli()

	art := &backup_managerpb.BackupArtifact{
		BackupId:      "rt-test",
		CreatedUnixMs: now,
		RestoreTestReport: &backup_managerpb.RestoreTestReport{
			BackupId:       "rt-test",
			Level:          backup_managerpb.RestoreTestLevel_RESTORE_TEST_LIGHT,
			Passed:         true,
			StartedUnixMs:  now - 5000,
			FinishedUnixMs: now,
			Checks: []*backup_managerpb.RestoreTestCheck{
				{Provider: "etcd", Ok: true, Summary: "snapshot valid"},
				{Provider: "restic", Ok: true, Summary: "snapshot found"},
			},
		},
	}

	if err := store.SaveArtifact(art); err != nil {
		t.Fatalf("SaveArtifact: %v", err)
	}

	loaded, err := store.GetArtifact("rt-test")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if loaded.RestoreTestReport == nil {
		t.Fatal("expected restore test report to be persisted")
	}
	if !loaded.RestoreTestReport.Passed {
		t.Error("expected passed=true")
	}
	if len(loaded.RestoreTestReport.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(loaded.RestoreTestReport.Checks))
	}
	if loaded.RestoreTestReport.Checks[0].Provider != "etcd" {
		t.Errorf("expected first check provider=etcd, got %s", loaded.RestoreTestReport.Checks[0].Provider)
	}
}

func TestArtifact_NodeCoverageRoundTrip(t *testing.T) {
	store := setupTestStore(t)
	now := time.Now().UnixMilli()

	art := &backup_managerpb.BackupArtifact{
		BackupId:      "cov-test",
		CreatedUnixMs: now,
		NodeCoverage: []*backup_managerpb.NodeCoverageReport{
			{
				Provider:  "restic",
				Succeeded: 2,
				Failed:    1,
				Total:     3,
				Entries: []*backup_managerpb.NodeCoverageReportEntry{
					{NodeId: "node-1", Hostname: "host1", Ok: true},
					{NodeId: "node-2", Hostname: "host2", Ok: true},
					{NodeId: "node-3", Hostname: "host3", Ok: false, ErrorMessage: "timeout"},
				},
			},
		},
		CompletedUnixMs: now,
	}

	if err := store.SaveArtifact(art); err != nil {
		t.Fatalf("SaveArtifact: %v", err)
	}

	loaded, err := store.GetArtifact("cov-test")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if len(loaded.NodeCoverage) != 1 {
		t.Fatalf("expected 1 coverage report, got %d", len(loaded.NodeCoverage))
	}
	cov := loaded.NodeCoverage[0]
	if cov.Provider != "restic" {
		t.Errorf("expected provider=restic, got %s", cov.Provider)
	}
	if cov.Succeeded != 2 || cov.Failed != 1 || cov.Total != 3 {
		t.Errorf("expected 2/1/3, got %d/%d/%d", cov.Succeeded, cov.Failed, cov.Total)
	}
	if len(cov.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(cov.Entries))
	}
	if loaded.CompletedUnixMs != now {
		t.Errorf("expected completed_unix_ms=%d, got %d", now, loaded.CompletedUnixMs)
	}
}

func TestArtifact_EvidenceAbsentSafe(t *testing.T) {
	store := setupTestStore(t)
	now := time.Now().UnixMilli()

	// Artifact with no evidence fields — should load cleanly
	art := &backup_managerpb.BackupArtifact{
		BackupId:      "no-evidence",
		CreatedUnixMs: now,
	}
	if err := store.SaveArtifact(art); err != nil {
		t.Fatalf("SaveArtifact: %v", err)
	}

	loaded, err := store.GetArtifact("no-evidence")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if loaded.ValidationReport != nil {
		t.Error("expected nil validation report")
	}
	if loaded.RestoreTestReport != nil {
		t.Error("expected nil restore test report")
	}
	if len(loaded.NodeCoverage) != 0 {
		t.Errorf("expected 0 coverage, got %d", len(loaded.NodeCoverage))
	}
}

func init() {
	// Suppress slog output during tests
	os.Setenv("SLOG_LEVEL", "ERROR")
}
