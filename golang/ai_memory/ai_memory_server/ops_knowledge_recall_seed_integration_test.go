package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	"github.com/gocql/gocql"
)

// Integration test for the ops-knowledge recall self-seed convergence
// (loadOpsKnowledgeRecallSeed) against a REAL ScyllaDB.
//
// It enforces the seed-idempotency invariant (ai-memory decision 45f7fc31):
// re-running the embedded self-seed must leave EXACTLY ONE canonical row per
// semantic key (project, id), and it must self-heal duplicates an older seeder
// left behind. The live store held 162 ids × 3 rows (created_at/escaped-content
// drift) precisely because the previous LIMIT-1 replace logic converged one row
// per run instead of all of them.
//
// Skipped unless BEHAVIORAL_SCYLLA_HOSTS is set (no Scylla container in the
// default dev/CI environment here); the deterministic content-normalization half
// of the invariant is covered by TestRecallSeedContentNormalized in the
// cluster_operator package, which runs everywhere.
func TestOpsKnowledgeRecallSeedConvergesDuplicates(t *testing.T) {
	hostsEnv := os.Getenv("BEHAVIORAL_SCYLLA_HOSTS")
	if hostsEnv == "" {
		t.Skip("BEHAVIORAL_SCYLLA_HOSTS not set — skipping ScyllaDB integration test (no container in this environment)")
	}
	hosts := strings.Split(hostsEnv, ",")
	ctx := context.Background()

	srv := &server{ScyllaHosts: hosts, ScyllaPort: 9042}
	if err := srv.applySchema(ctx); err != nil {
		t.Fatalf("applySchema: %v", err)
	}

	cluster := gocql.NewCluster(hosts...)
	cluster.Port = 9042
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.One
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer session.Close()
	srv.session = session

	entries, err := cluster_operator.RecallSeedEntries()
	if err != nil || len(entries) == 0 {
		t.Fatalf("RecallSeedEntries: %v (n=%d)", err, len(entries))
	}
	sample := entries[0].ID

	countRows := func(id string) int {
		iter := session.Query(
			`SELECT created_at FROM memories WHERE project = ? AND id = ? ALLOW FILTERING`,
			recallSeedProject, id).WithContext(ctx).Iter()
		var created int64
		n := 0
		for iter.Scan(&created) {
			n++
		}
		_ = iter.Close()
		return n
	}

	// Inject an artificial pre-existing duplicate for one id at a distinct
	// created_at with stale, escaped-drift content and a mismatching seed hash —
	// reproducing the exact live-store condition an older seeder created.
	if err := session.Query(
		`INSERT INTO memories (id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sample, recallSeedProject, "REFERENCE", []string{"seed"}, "stale", `stale\ncontent`,
		int64(1), int64(1), "ops-knowledge-seeder", "", "",
		map[string]string{"source": "seed", "immutable": "true", "seed_sha256": "stale-hash"},
		[]string(nil), 0,
	).WithContext(ctx).Exec(); err != nil {
		t.Fatalf("seed stale duplicate: %v", err)
	}

	// First run: converge every id to exactly one canonical row, healing the
	// injected duplicate.
	srv.loadOpsKnowledgeRecallSeed()
	if got := countRows(sample); got != 1 {
		t.Fatalf("after first seed: id %s has %d rows, want 1 (pre-existing duplicate not converged)", sample, got)
	}
	for _, e := range entries {
		if got := countRows(e.ID); got != 1 {
			t.Fatalf("after first seed: id %s has %d rows, want 1", e.ID, got)
		}
	}

	// Second run: idempotent — still exactly one row per id, nothing re-forked.
	srv.loadOpsKnowledgeRecallSeed()
	for _, e := range entries {
		if got := countRows(e.ID); got != 1 {
			t.Fatalf("after second seed (idempotency): id %s has %d rows, want 1", e.ID, got)
		}
	}
}
