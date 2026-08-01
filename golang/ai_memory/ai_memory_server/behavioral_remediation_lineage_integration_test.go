package main

// Scylla round-trip + MemoryStore parity for sat.remediation.fresh_convergence_verified.
//
//	remediation.Outcome
//	→ observation.FromRemediationOutcome  (real adapter, no substitute evidence)
//	→ BindSatisfies
//	→ ScyllaStore.PutEvidence
//	→ ListEvidenceSatisfying
//	→ QualifyEvidence
//	→ SATISFIED
//
// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT
//
// It proves the EVIDENCE artifact survives real persistence and that both stores
// return the same verdict. It does NOT prove the chain that builds the Outcome —
// that is proven in workflow/engine/actors_doctor_evidence_test.go, which drives
// the real resolve → execute → verify handlers and the unexported
// buildRemediationOutcome.
//
// The two proofs are in different packages for a structural reason worth naming:
// the behavioral schema DDL lives in package main, so no other package can apply
// it, while buildRemediationOutcome is unexported, so no other package can call
// it. Neither proof can currently be moved to join the other. The Outcome below
// is therefore constructed here — and every field of the evidence it produces is
// asserted against the same expectations the engine test asserts, so an adapter
// that drifts fails on both sides rather than quietly agreeing with one test.
//
// WHY SCYLLA AND NOT ONLY MEMORYSTORE
//
// This rule is the first to require an action binding, and the satisfaction
// index projects a FIXED column set. A binding carried anywhere outside that
// projection is present in MemoryStore (which keeps whole structs) and absent
// after a Scylla round trip — enforcement that passes every unit test and never
// fires in production. That divergence is exactly what this file exists to
// catch, and it is why ActionRef is a first-class field rather than a metadata
// key.
//
// Skipped unless BEHAVIORAL_SCYLLA_HOSTS is set. A skip is NOT proof.
//
//	BEHAVIORAL_SCYLLA_HOSTS=10.10.0.11 go test ./ai_memory/ai_memory_server \
//	  -run TestScyllaRemediation -count=1

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/remediation"
	"github.com/gocql/gocql"
)

const (
	remReq    = "evidence.remediation.fresh_convergence_verification"
	remInvar  = "cluster.desired_state.absent"
	remEntity = "svc/repository" // deliberately NOT the node id
	remNode   = "node-4"
	remFind   = "finding-1"
	remRun    = "run-abc"
)

var (
	remDispatchAt = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	remVerifyAt   = remDispatchAt.Add(90 * time.Second)
	remEvalAt     = remVerifyAt.Add(1 * time.Minute) // inside the rule's 15m window
)

// remOutcome is the lineage-complete, successful outcome the engine chain
// produces. Field values mirror workflow/engine/actors_doctor_lineage_test.go so
// the two proofs describe one artifact, not two.
func remOutcome(cluster, run string, verifiedAt time.Time) remediation.Outcome {
	return remediation.Outcome{
		FindingID: remFind, WorkflowRunID: run,
		ClusterID: cluster, InvariantID: remInvar, EntityRef: remEntity, NodeID: remNode,
		Dispatched: true, Verified: true, FindingResolved: true,
		DispatchedAt: remDispatchAt, VerifiedAt: verifiedAt,
	}
}

// remEvidence maps an outcome through the REAL adapter and asserts the artifact
// matches what the engine-side proof asserts. If these drift apart, the shared
// description of the artifact is wrong and both tests must be revisited.
func remEvidence(t *testing.T, project, domain string, o remediation.Outcome) api.Evidence {
	t.Helper()
	if !o.IsSuccess() || !o.LineageComplete() {
		t.Fatalf("fixture must be eligible: success=%t defects=%v", o.IsSuccess(), o.LineageDefects())
	}
	b, qualifies := observation.FromRemediationOutcome(project, api.DomainRef(domain), o)
	observation.BindRemediationEvidence(&b)
	if !qualifies || len(b.Evidence) != 1 {
		t.Fatalf("adapter must emit exactly one qualifying row: qualifies=%t rows=%d", qualifies, len(b.Evidence))
	}
	ev := b.Evidence[0]

	for _, c := range []struct{ name, got, want string }{
		{"kind", ev.Kind, observation.KindRemediationVerification},
		{"source", ev.SourceKind, observation.SourceKindDoctorRemediationWorkflow},
		{"result", ev.Result, observation.ResultFindingResolved},
		{"authority", string(ev.AuthorityLevel), string(api.ObservationAuthorityDerived)},
		{"cluster", ev.ClusterID, o.ClusterID},
		{"condition", ev.ConditionRef, remInvar},
		{"entity", ev.EntityRef, remEntity},
		{"action_ref", ev.ActionRef, o.WorkflowRunID},
	} {
		if c.got != c.want {
			t.Fatalf("adapter %s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if ev.ObservedAt != o.VerifiedAt.Unix() {
		t.Fatalf("observed_at = %d, want the verification time %d", ev.ObservedAt, o.VerifiedAt.Unix())
	}
	if len(ev.Satisfies) != 1 || string(ev.Satisfies[0]) != remReq {
		t.Fatalf("producer must bind before persistence, got %v", ev.Satisfies)
	}
	return ev
}

func remScope() (project, domain, cluster string) {
	u := strings.ReplaceAll(gocql.TimeUUID().String(), "-", "")[:12]
	return "remlineage-" + u, "cluster_operator", "cluster-" + u
}

func remQuery(project, domain, cluster string, mut ...func(*cluster_operator.SatisfactionQuery)) cluster_operator.SatisfactionQuery {
	q := cluster_operator.SatisfactionQuery{
		Project: project, Domain: api.DomainRef(domain), ClusterID: cluster,
		RequirementID: remReq, EntityRef: remEntity, ConditionRef: remInvar,
		WorkflowRunID: remRun, ActionDispatchedAt: remDispatchAt,
		EvaluatedAt: remEvalAt,
	}
	for _, f := range mut {
		f(&q)
	}
	return q
}

// remFixture reuses the canonical session/cleanup convention; cleanup removes
// exactly the rows this scope wrote.
func remFixture(t *testing.T) (*store.ScyllaStore, *gocql.Session, func(project, domain, cluster string, ids ...string)) {
	t.Helper()
	st, session, cleanupDoctor := scyllaFixture(t)
	cleanup := func(project, domain, cluster string, ids ...string) {
		cleanupDoctor(project, domain, cluster, ids...)
		_ = session.Query(`DELETE FROM behavioral_memory.evidence_by_satisfaction
WHERE project=? AND domain=? AND required_evidence_ref=? AND cluster_id=?`,
			project, domain, remReq, cluster).Exec()
	}
	return st, session, cleanup
}

// ── the round trip ──────────────────────────────────────────────────────────

func TestScyllaRemediation_RealAdapterQualifiesAfterRoundTrip(t *testing.T) {
	st, session, cleanup := remFixture(t)
	defer session.Close()
	project, domain, cluster := remScope()
	ctx := context.Background()

	ev := remEvidence(t, project, domain, remOutcome(cluster, remRun, remVerifyAt))
	defer cleanup(project, domain, cluster, ev.ID)

	if err := st.PutEvidence(ctx, &ev); err != nil {
		t.Fatalf("PutEvidence: %v", err)
	}

	res, err := cluster_operator.QualifyEvidence(ctx, st, remQuery(project, domain, cluster))
	if err != nil {
		t.Fatalf("QualifyEvidence: %v", err)
	}
	if !res.Satisfied() {
		t.Fatalf("SCYLLA LINEAGE FAILED — adapter output did not qualify after round trip; rejected=%+v", res.Rejected)
	}
	got := res.Qualified[0]
	if got.ID != ev.ID {
		t.Fatalf("qualified the wrong row: %q vs %q", got.ID, ev.ID)
	}
	if len(got.Satisfies) == 0 {
		t.Error("Satisfies did not survive the round trip")
	}
	// The load-bearing one. Read back from the index projection, not from the
	// caller's in-memory copy.
	if got.ActionRef != remRun {
		t.Errorf("ActionRef did not survive the round trip: got %q want %q — "+
			"an action binding that vanishes in persistence is unenforceable in production",
			got.ActionRef, remRun)
	}
	if got.Result != observation.ResultFindingResolved {
		t.Errorf("result did not survive: %q", got.Result)
	}
	if got.ObservedAt != remVerifyAt.Unix() {
		t.Errorf("observed_at did not survive: %d want %d", got.ObservedAt, remVerifyAt.Unix())
	}
}

// ── parity: identical cases through both stores ─────────────────────────────

func TestScyllaRemediation_ParityWithMemoryStore(t *testing.T) {
	st, session, cleanup := remFixture(t)
	defer session.Close()
	ctx := context.Background()

	mustQualify := func(t *testing.T, s store.Store, q cluster_operator.SatisfactionQuery, why string) {
		t.Helper()
		res, err := cluster_operator.QualifyEvidence(ctx, s, q)
		if err != nil {
			t.Fatalf("%s: err=%v", why, err)
		}
		if !res.Satisfied() {
			t.Fatalf("%s must qualify; rejected=%+v", why, res.Rejected)
		}
	}
	mustNotQualify := func(t *testing.T, s store.Store, q cluster_operator.SatisfactionQuery, why string) {
		t.Helper()
		res, err := cluster_operator.QualifyEvidence(ctx, s, q)
		if err != nil {
			t.Fatalf("%s: err=%v", why, err)
		}
		if res.Satisfied() {
			t.Fatalf("%s must NOT qualify", why)
		}
	}

	checks := []struct {
		name string
		run  func(t *testing.T, s store.Store, p, d, c string)
	}{
		{"qualification result", func(t *testing.T, s store.Store, p, d, c string) {
			mustQualify(t, s, remQuery(p, d, c), "the real artifact")
		}},
		{"exact cluster isolation", func(t *testing.T, s store.Store, p, d, c string) {
			mustNotQualify(t, s, remQuery(p, d, c+"-other"), "another cluster's query")
			mustNotQualify(t, s, remQuery(p, d, ""), "an empty cluster (must select the cluster-less partition, not act as a wildcard)")
		}},
		{"workflow-run binding", func(t *testing.T, s store.Store, p, d, c string) {
			mustNotQualify(t, s, remQuery(p, d, c, func(q *cluster_operator.SatisfactionQuery) {
				q.WorkflowRunID = "run-elsewhere"
			}), "another run's requirement")
			mustNotQualify(t, s, remQuery(p, d, c, func(q *cluster_operator.SatisfactionQuery) {
				q.WorkflowRunID = ""
			}), "a query carrying no action identity")
		}},
		{"accepted result", func(t *testing.T, s store.Store, p, d, c string) {
			// Proven by construction on the positive side; the negative side is
			// that a non-accepted result never reaches the accepted set — see
			// the engine-side mutation matrix, which mutates result directly.
			res, err := cluster_operator.QualifyEvidence(ctx, s, remQuery(p, d, c))
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if res.Qualified[0].Result != observation.ResultFindingResolved {
				t.Fatalf("qualified on result %q, want %q", res.Qualified[0].Result, observation.ResultFindingResolved)
			}
		}},
		{"newer-than-dispatch", func(t *testing.T, s store.Store, p, d, c string) {
			mustNotQualify(t, s, remQuery(p, d, c, func(q *cluster_operator.SatisfactionQuery) {
				// Dispatch claimed AFTER the observation: the evidence cannot
				// show this action worked.
				q.ActionDispatchedAt = remVerifyAt.Add(1 * time.Minute)
			}), "evidence predating the action")
			mustNotQualify(t, s, remQuery(p, d, c, func(q *cluster_operator.SatisfactionQuery) {
				q.ActionDispatchedAt = time.Time{}
			}), "a query with no dispatch time")
		}},
		{"bounded freshness", func(t *testing.T, s store.Store, p, d, c string) {
			mustNotQualify(t, s, remQuery(p, d, c, func(q *cluster_operator.SatisfactionQuery) {
				q.EvaluatedAt = remVerifyAt.Add(1 * time.Hour)
			}), "a verification an hour stale")
		}},
		{"malformed query errors", func(t *testing.T, s store.Store, p, d, c string) {
			q := remQuery(p, d, c)
			q.RequirementID = ""
			if _, err := cluster_operator.QualifyEvidence(ctx, s, q); err == nil {
				t.Fatal("malformed query must error, not return empty")
			}
		}},
	}

	for _, ch := range checks {
		t.Run(ch.name, func(t *testing.T) {
			project, domain, cluster := remScope()
			ev := remEvidence(t, project, domain, remOutcome(cluster, remRun, remVerifyAt))
			defer cleanup(project, domain, cluster, ev.ID)

			mem := store.NewMemoryStore()
			memCopy := ev
			if err := mem.PutEvidence(ctx, &memCopy); err != nil {
				t.Fatalf("memory PutEvidence: %v", err)
			}
			scyCopy := ev
			if err := st.PutEvidence(ctx, &scyCopy); err != nil {
				t.Fatalf("scylla PutEvidence: %v", err)
			}
			t.Run("memory", func(t *testing.T) { ch.run(t, mem, project, domain, cluster) })
			t.Run("scylla", func(t *testing.T) { ch.run(t, st, project, domain, cluster) })
		})
	}
}

// Ordering parity: newest first, stable evidence ID as the tie-break. Three runs
// against the same subject, two sharing a verification timestamp.
func TestScyllaRemediation_OrderingParity(t *testing.T) {
	st, session, cleanup := remFixture(t)
	defer session.Close()
	ctx := context.Background()
	project, domain, cluster := remScope()

	rows := []api.Evidence{
		remEvidence(t, project, domain, remOutcome(cluster, "run-older", remDispatchAt.Add(30*time.Second))),
		remEvidence(t, project, domain, remOutcome(cluster, "run-tie-a", remVerifyAt)),
		remEvidence(t, project, domain, remOutcome(cluster, "run-tie-b", remVerifyAt)),
	}
	var ids []string
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	defer cleanup(project, domain, cluster, ids...)

	mem := store.NewMemoryStore()
	for i := range rows {
		m, s := rows[i], rows[i]
		if err := mem.PutEvidence(ctx, &m); err != nil {
			t.Fatalf("memory put: %v", err)
		}
		if err := st.PutEvidence(ctx, &s); err != nil {
			t.Fatalf("scylla put: %v", err)
		}
	}

	// Ordering is a property of the LOOKUP, so query without the run binding
	// that would narrow the set to one — then read the candidate order the
	// qualifier saw, via its rejections plus its qualified rows.
	order := func(s store.Store) []string {
		res, err := s.ListEvidenceSatisfying(ctx, store.EvidenceSatisfactionQuery{
			Project: project, Domain: domain, RequiredEvidenceRef: remReq, ClusterID: cluster,
		})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		var out []string
		for _, e := range res {
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
	if !strings.HasSuffix(memOrder[2], fmt.Sprintf("@%d", remDispatchAt.Add(30*time.Second).Unix())) {
		t.Errorf("oldest row must sort last, got %v", memOrder)
	}

	// And only the run actually being governed qualifies, out of three rows that
	// are otherwise identical in cluster, subject, result and recency.
	res, err := cluster_operator.QualifyEvidence(ctx, st,
		remQuery(project, domain, cluster, func(q *cluster_operator.SatisfactionQuery) { q.WorkflowRunID = "run-tie-a" }))
	if err != nil {
		t.Fatalf("qualify: %v", err)
	}
	if len(res.Qualified) != 1 {
		t.Fatalf("exactly one run's verification may qualify, got %d", len(res.Qualified))
	}
	if res.Qualified[0].ActionRef != "run-tie-a" {
		t.Errorf("qualified the wrong run: %q", res.Qualified[0].ActionRef)
	}
}

// A non-qualifying outcome must be recordable and invisible to the governor —
// not silently dropped, and not quietly countable.
func TestScyllaRemediation_DiagnosticRowIsRecordableButNotFindable(t *testing.T) {
	st, session, cleanup := remFixture(t)
	defer session.Close()
	project, domain, cluster := remScope()
	ctx := context.Background()

	o := remOutcome(cluster, remRun, remVerifyAt)
	o.ClusterID = "" // succeeded, but unattributable
	b, qualifies := observation.FromRemediationOutcome(project, api.DomainRef(domain), o)
	observation.BindRemediationEvidence(&b)
	if qualifies || len(b.Evidence) != 1 {
		t.Fatalf("expected one non-qualifying row, got qualifies=%t rows=%d", qualifies, len(b.Evidence))
	}
	ev := b.Evidence[0]
	defer cleanup(project, domain, cluster, ev.ID)

	if err := st.PutEvidence(ctx, &ev); err != nil {
		t.Fatalf("a non-qualifying outcome must still be recordable: %v", err)
	}
	res, err := cluster_operator.QualifyEvidence(ctx, st, remQuery(project, domain, cluster))
	if err != nil {
		t.Fatalf("qualify: %v", err)
	}
	if res.Satisfied() {
		t.Fatal("a diagnostic row must never satisfy the verification requirement")
	}
}
