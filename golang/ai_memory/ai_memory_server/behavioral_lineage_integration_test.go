package main

// Scylla lineage + parity for sat.doctor.finding_observed.
//
// Closes 4B. Proves the production chain against a REAL store:
//
//	cluster_doctorpb.Finding
//	→ observation.FromDoctorFinding   (real producer, no substitute fixture)
//	→ BindSatisfies                    (applied inside the producer)
//	→ ScyllaStore.PutEvidence
//	→ ListEvidenceSatisfying
//	→ QualifyEvidence
//	→ SATISFIED
//
// WHY SCYLLA AND NOT ONLY MEMORYSTORE
//
// MemoryStore mirrors the CQL selection semantics by implementation intent.
// Divergence is plausible through partition-key encoding, empty-cluster
// representation, collection serialization, timestamp precision, or
// tie-breaking — a governor that passes every unit test and finds an empty
// cabinet in production. Declaring governance-ready on an in-memory proof is the
// same circular comfort that produced the earlier fictional catalog.
//
// Skipped unless BEHAVIORAL_SCYLLA_HOSTS is set, matching the convention in
// behavioral_scylla_integration_test.go. A skip is NOT proof.
//
//	BEHAVIORAL_SCYLLA_HOSTS=10.10.0.11 go test ./ai_memory/ai_memory_server \
//	  -run TestScyllaLineage -count=1

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/gocql/gocql"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	linReq    = "evidence.doctor.finding_observed"
	linEntity = "node-4"
	linInvar  = "cluster.desired_state.absent"
)

var (
	linEvalAt  = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	linObserve = linEvalAt.Add(-2 * time.Minute) // inside the rule's 15m MaxAge
)

// linScope returns project/domain/cluster unique to this run so the test never
// depends on, or collides with, rows already in the live keyspace.
func linScope() (project, domain, cluster string) {
	u := strings.ReplaceAll(gocql.TimeUUID().String(), "-", "")[:12]
	return "lineage-" + u, "cluster_operator", "cluster-" + u
}

// realEvidence invokes the ACTUAL producer. Nothing here hand-builds api.Evidence.
func realEvidence(t *testing.T, project, domain, cluster, findingID string, observedAt time.Time) api.Evidence {
	t.Helper()
	b := observation.FromDoctorFinding(project, api.DomainRef(domain), cluster,
		&cluster_doctorpb.ReportHeader{Source: "cluster-doctor", SnapshotId: "snap-1",
			ObservedAt: timestamppb.New(observedAt)},
		&cluster_doctorpb.Finding{
			FindingId: findingID, InvariantId: linInvar, EntityRef: linEntity,
			Category: "drift", Summary: "no desired state for any registered node",
			Evidence: []*cluster_doctorpb.Evidence{{
				SourceService: "cluster_controller", SourceRpc: "GetClusterHealthV1",
				Timestamp: timestamppb.New(observedAt),
			}},
		})
	if len(b.Evidence) != 1 {
		t.Fatalf("producer emitted %d evidence rows, want 1", len(b.Evidence))
	}
	return b.Evidence[0]
}

func linQuery(project, domain, cluster string) cluster_operator.SatisfactionQuery {
	return cluster_operator.SatisfactionQuery{
		Project: project, Domain: api.DomainRef(domain), ClusterID: cluster,
		RequirementID: linReq, EntityRef: linEntity, ConditionRef: linInvar,
		EvaluatedAt: linEvalAt,
	}
}

// scyllaFixture opens the canonical session, applies the schema, and returns a
// store plus a cleanup that removes exactly the rows this test inserted.
func scyllaFixture(t *testing.T) (*store.ScyllaStore, *gocql.Session, func(project, domain, cluster string, ids ...string)) {
	t.Helper()
	hostsEnv := os.Getenv("BEHAVIORAL_SCYLLA_HOSTS")
	if hostsEnv == "" {
		t.Skip("BEHAVIORAL_SCYLLA_HOSTS not set — skipping (a skip is not proof)")
	}
	hosts := strings.Split(hostsEnv, ",")
	if err := (&server{ScyllaHosts: hosts, ScyllaPort: 9042}).applyBehavioralSchema(context.Background()); err != nil {
		t.Fatalf("applyBehavioralSchema: %v", err)
	}
	session, err := newSchemaSession(hosts, 9042, len(hosts))
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	cleanup := func(project, domain, cluster string, ids ...string) {
		for _, id := range ids {
			_ = session.Query(`DELETE FROM behavioral_memory.evidence WHERE project=? AND domain=? AND id=?`,
				project, domain, id).Exec()
		}
		// The satisfaction index is keyed by (project,domain,ref,cluster); one
		// partition delete removes every row this scope wrote.
		_ = session.Query(`DELETE FROM behavioral_memory.evidence_by_satisfaction
WHERE project=? AND domain=? AND required_evidence_ref=? AND cluster_id=?`,
			project, domain, linReq, cluster).Exec()
	}
	return store.NewScyllaStore(session), session, cleanup
}

// ── the lineage ─────────────────────────────────────────────────────────────

func TestScyllaLineage_RealProducerQualifiesAfterRoundTrip(t *testing.T) {
	st, session, cleanup := scyllaFixture(t)
	defer session.Close()
	project, domain, cluster := linScope()
	ctx := context.Background()

	ev := realEvidence(t, project, domain, cluster, "finding-1", linObserve)

	// The producer must already have bound the requirement; persistence writes
	// the satisfaction index from Satisfies, so an unbound row would be stored
	// yet invisible to lookup.
	if len(ev.Satisfies) != 1 || string(ev.Satisfies[0]) != linReq {
		t.Fatalf("producer output lacks the binding: %v", ev.Satisfies)
	}
	defer cleanup(project, domain, cluster, ev.ID)

	if err := st.PutEvidence(ctx, &ev); err != nil {
		t.Fatalf("PutEvidence: %v", err)
	}

	res, err := cluster_operator.QualifyEvidence(ctx, st, linQuery(project, domain, cluster))
	if err != nil {
		t.Fatalf("QualifyEvidence: %v", err)
	}
	if !res.Satisfied() {
		t.Fatalf("SCYLLA LINEAGE FAILED — real producer output did not qualify after round trip; rejected=%+v", res.Rejected)
	}
	if res.Qualified[0].ID != ev.ID {
		t.Fatalf("qualified the wrong row: %q vs %q", res.Qualified[0].ID, ev.ID)
	}
	// Satisfies must survive persistence, not merely be reconstructed by the
	// caller: the index round trip is what the governor depends on.
	if len(res.Qualified[0].Satisfies) == 0 {
		t.Error("Satisfies did not survive the round trip")
	}
}

// ── parity: identical cases through both stores ─────────────────────────────

func TestScyllaLineage_ParityWithMemoryStore(t *testing.T) {
	st, session, cleanup := scyllaFixture(t)
	defer session.Close()
	ctx := context.Background()

	type check struct {
		name string
		run  func(t *testing.T, s store.Store, project, domain, cluster string)
	}
	checks := []check{
		{"exact cluster match", func(t *testing.T, s store.Store, p, d, c string) {
			res, err := cluster_operator.QualifyEvidence(ctx, s, linQuery(p, d, c))
			if err != nil || !res.Satisfied() {
				t.Fatalf("exact cluster must qualify: err=%v rejected=%+v", err, res.Rejected)
			}
		}},
		{"wrong cluster excluded", func(t *testing.T, s store.Store, p, d, c string) {
			res, err := cluster_operator.QualifyEvidence(ctx, s, linQuery(p, d, c+"-other"))
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if res.Satisfied() {
				t.Fatal("another cluster's query must not qualify this evidence")
			}
		}},
		{"empty cluster selects only cluster-less", func(t *testing.T, s store.Store, p, d, c string) {
			res, err := cluster_operator.QualifyEvidence(ctx, s, linQuery(p, d, ""))
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if res.Satisfied() {
				t.Fatal("empty cluster must select the cluster-less partition, never act as a wildcard")
			}
		}},
		{"malformed query errors", func(t *testing.T, s store.Store, p, d, c string) {
			q := linQuery(p, d, c)
			q.RequirementID = ""
			if _, err := cluster_operator.QualifyEvidence(ctx, s, q); err == nil {
				t.Fatal("malformed query must error, not return empty")
			}
		}},
	}

	for _, ch := range checks {
		t.Run(ch.name, func(t *testing.T) {
			project, domain, cluster := linScope()
			ev := realEvidence(t, project, domain, cluster, "finding-1", linObserve)
			defer cleanup(project, domain, cluster, ev.ID)

			mem := store.NewMemoryStore()
			if err := mem.PutEvidence(ctx, &ev); err != nil {
				t.Fatalf("memory PutEvidence: %v", err)
			}
			evCopy := ev
			if err := st.PutEvidence(ctx, &evCopy); err != nil {
				t.Fatalf("scylla PutEvidence: %v", err)
			}
			t.Run("memory", func(t *testing.T) { ch.run(t, mem, project, domain, cluster) })
			t.Run("scylla", func(t *testing.T) { ch.run(t, st, project, domain, cluster) })
		})
	}
}

// Ordering parity: newest first, stable evidence ID as the timestamp tie-break.
func TestScyllaLineage_OrderingParity(t *testing.T) {
	st, session, cleanup := scyllaFixture(t)
	defer session.Close()
	ctx := context.Background()
	project, domain, cluster := linScope()

	// Three rows: two sharing a timestamp so the ID tie-break is exercised.
	mk := func(findingID string, at time.Time) api.Evidence {
		return realEvidence(t, project, domain, cluster, findingID, at)
	}
	rows := []api.Evidence{
		mk("f-older", linEvalAt.Add(-5*time.Minute)),
		mk("f-tie-a", linEvalAt.Add(-1*time.Minute)),
		mk("f-tie-b", linEvalAt.Add(-1*time.Minute)),
	}
	var ids []string
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	defer cleanup(project, domain, cluster, ids...)

	mem := store.NewMemoryStore()
	for i := range rows {
		if err := mem.PutEvidence(ctx, &rows[i]); err != nil {
			t.Fatalf("memory put: %v", err)
		}
		c := rows[i]
		if err := st.PutEvidence(ctx, &c); err != nil {
			t.Fatalf("scylla put: %v", err)
		}
	}

	order := func(s store.Store) []string {
		res, err := cluster_operator.QualifyEvidence(ctx, s, linQuery(project, domain, cluster))
		if err != nil {
			t.Fatalf("qualify: %v", err)
		}
		var out []string
		for _, e := range res.Qualified {
			out = append(out, fmt.Sprintf("%s@%d", e.ID, e.ObservedAt))
		}
		return out
	}
	memOrder, scyOrder := order(mem), order(st)

	if len(memOrder) != 3 || len(scyOrder) != 3 {
		t.Fatalf("both stores must return 3 rows: memory=%v scylla=%v", memOrder, scyOrder)
	}
	for i := range memOrder {
		if memOrder[i] != scyOrder[i] {
			t.Fatalf("ordering diverged at %d:\n  memory=%v\n  scylla=%v", i, memOrder, scyOrder)
		}
	}
	// Newest first.
	if !strings.HasSuffix(memOrder[2], fmt.Sprintf("@%d", linEvalAt.Add(-5*time.Minute).Unix())) {
		t.Errorf("oldest row must sort last, got %v", memOrder)
	}
}

// The unconfigured store refuses rather than reporting an empty result — a
// missing backend is not evidence of missing evidence.
func TestScyllaLineage_UnconfiguredStoreRefuses(t *testing.T) {
	project, domain, cluster := linScope()
	if _, err := cluster_operator.QualifyEvidence(context.Background(),
		store.Unconfigured{}, linQuery(project, domain, cluster)); err == nil {
		t.Fatal("unconfigured store must error, not return an empty successful result")
	}
}
