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
	// Per-table PR-9 column sets: signals carried source_kind/source_ref/entity_ref
	// from v1, but the evidence table ADDED them in PR-9 — so evidence needs all
	// seven backfilled, signals only the four newer ones.
	colsByTable := map[string][]string{
		"signals":  {"cluster_id", "condition_ref", "severity", "authority_level"},
		"evidence": {"cluster_id", "condition_ref", "severity", "authority_level", "source_kind", "source_ref", "entity_ref"},
	}

	indexOf := func(substr string) int {
		for i, s := range behavioralSchemaStatements {
			if strings.Contains(s, substr) {
				return i
			}
		}
		return -1
	}

	for table, cols := range colsByTable {
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

// v10 regression: ListPromotionCandidates / ListReconciliationReports failed at
// runtime with ScyllaDB's ALLOW FILTERING error because they enumerated the entity
// table by the (project,domain) PREFIX of a composite ((project,domain,id))
// partition key. The fix lists from dedicated single-partition by-scope indexes
// keyed ((project,domain), id). This guards that (a) those index tables are part of
// the schema and (b) the schema version was bumped so already-"complete" clusters
// re-run the migration and create them.
func TestBehavioralListByScopeIndexesPresent(t *testing.T) {
	has := func(substr string) bool {
		for _, s := range behavioralSchemaStatements {
			if strings.Contains(s, substr) {
				return true
			}
		}
		return false
	}
	for _, tbl := range []string{
		"CREATE TABLE IF NOT EXISTS behavioral_memory.promotion_candidates_by_scope (",
		"CREATE TABLE IF NOT EXISTS behavioral_memory.reconciliation_reports_by_scope (",
	} {
		if !has(tbl) {
			t.Errorf("missing list-by-scope index in behavioralSchemaStatements: %q — List RPCs will hit ALLOW FILTERING", tbl)
		}
	}
	// Both index tables must partition by ((project, domain)) so a (project,domain)
	// list is a single-partition read, not a partition-key-prefix scan.
	for _, key := range []string{
		"behavioral_memory.promotion_candidates_by_scope",
		"behavioral_memory.reconciliation_reports_by_scope",
	} {
		found := false
		for _, s := range behavioralSchemaStatements {
			if strings.Contains(s, key) && strings.Contains(s, "PRIMARY KEY ((project, domain), id)") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s must declare PRIMARY KEY ((project, domain), id) for single-partition listing", key)
		}
	}
	// Adding tables without bumping the version leaves already-"complete" clusters
	// on the old schema (the migration fast-path-skips them), so the indexes never
	// get created and the live List RPCs stay broken.
	if behavioralSchemaVersion < 10 {
		t.Errorf("behavioralSchemaVersion=%d but the by-scope indexes require >=10 to force re-migration", behavioralSchemaVersion)
	}
}
