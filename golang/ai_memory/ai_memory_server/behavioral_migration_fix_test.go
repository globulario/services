package main

import (
	"strings"
	"testing"
)

// Bug #1 regression: the migration mutex key must be a leaf sibling of the state
// key, never a prefix of it. etcd's concurrency.Mutex treats every key under its
// prefix as a lock contender, so a state key nested under the mutex prefix
// (the historical "/globular/migrations/scylla/behavioral_memory" mutex with the
// state at ".../behavioral_memory/state") is mistaken for an un-releasable lock
// holder and every Mutex.Lock waits for it forever, timing out every migration.
func TestBehavioralMigrationMutexKeyNotPrefixOfStateKey(t *testing.T) {
	if behavioralMigrationMutexKey == behavioralMigrationStateKey {
		t.Fatalf("mutex and state keys must differ")
	}
	if strings.HasPrefix(behavioralMigrationStateKey, behavioralMigrationMutexKey) {
		t.Fatalf("mutex key %q is a prefix of state key %q — etcd Mutex would treat the state key as an un-releasable lock holder",
			behavioralMigrationMutexKey, behavioralMigrationStateKey)
	}
	if strings.HasPrefix(behavioralMigrationMutexKey, behavioralMigrationStateKey) {
		t.Fatalf("state key %q is a prefix of mutex key %q — both must be leaf siblings",
			behavioralMigrationStateKey, behavioralMigrationMutexKey)
	}
}

// Bug #2 regression: every PR-9 governed-observation column added to the
// signals/evidence CREATE TABLE must have a matching backfill ALTER in the
// ordered migration statements, positioned after the CREATE — otherwise tables
// created before the column existed (CREATE IF NOT EXISTS is a no-op on them)
// never get it and behavioral writes fail with "Unknown identifier <col>".
func TestBehavioralSchemaPR9ColumnsHaveBackfillAlter(t *testing.T) {
	cols := []string{"cluster_id", "condition_ref", "severity", "authority_level"}
	tables := []string{"signals", "evidence"}

	indexOf := func(substr string) int {
		for i, s := range behavioralSchemaStatements {
			if strings.Contains(s, substr) {
				return i
			}
		}
		return -1
	}

	for _, table := range tables {
		createIdx := indexOf("CREATE TABLE IF NOT EXISTS behavioral_memory." + table + " (")
		if createIdx < 0 {
			t.Fatalf("no CREATE TABLE for behavioral_memory.%s in behavioralSchemaStatements", table)
		}
		for _, col := range cols {
			alter := "ALTER TABLE behavioral_memory." + table + " ADD " + col
			alterIdx := indexOf(alter)
			if alterIdx < 0 {
				t.Errorf("missing backfill ALTER %q — pre-existing %s tables will lack %q", alter, table, col)
				continue
			}
			if alterIdx < createIdx {
				t.Errorf("ALTER for %s.%s appears before its CREATE (idx %d < %d) — table may not exist yet", table, col, alterIdx, createIdx)
			}
		}
	}
}
