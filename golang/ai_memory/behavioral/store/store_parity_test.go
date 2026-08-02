package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/gocql/gocql"
)

// THE FAST LOOP.
//
// Two of the defects found on 2026-08-01 existed because MemoryStore — the
// double every unit test ran against — was MORE CAPABLE than ScyllaStore, the
// thing production uses:
//
//   - ListOutcomesByTheme: MemoryStore hydrated index → full record; ScyllaStore
//     scanned the index alone and returned outcomes with no EvidenceIDs. That
//     made GeneratePromotionCandidate structurally impossible on a real cluster.
//     Full coverage, green suite, dead in production.
//
// Finding that took a release build, a five-node bring-up, a Day-0, four joins
// and a fault injection — roughly 45 minutes per question. This file answers the
// same class of question in seconds.
//
// The rule it enforces: a contract is asserted ONCE and every Store
// implementation must satisfy it. A behaviour that only the in-memory store has
// is not a contract, it is a fiction the tests agree to believe.
//
// ScyllaStore is exercised when BEHAVIORAL_SCYLLA_ADDR points at a Scylla that
// already carries the behavioral schema — e.g. a running quickstart sim:
//
//	BEHAVIORAL_SCYLLA_ADDR=10.10.0.11:9042 go test ./ai_memory/behavioral/store/ -run Parity
//
// Absent that, the Scylla leg SKIPS rather than fails: a developer without a
// cluster still runs the MemoryStore leg, and CI can opt in. A skip is reported
// loudly, because a silently-skipped parity test is how the divergence returns.

type namedStore struct {
	name  string
	store Store
}

// storesUnderTest returns every implementation the contract applies to.
func storesUnderTest(t *testing.T) []namedStore {
	t.Helper()
	out := []namedStore{{name: "MemoryStore", store: NewMemoryStore()}}

	addr := os.Getenv("BEHAVIORAL_SCYLLA_ADDR")
	if addr == "" {
		t.Log("SKIPPING ScyllaStore leg: BEHAVIORAL_SCYLLA_ADDR unset. " +
			"The production store is NOT under test — set it to a Scylla carrying the " +
			"behavioral schema (a running quickstart sim works: 10.10.0.11:9042).")
		return out
	}

	cluster := gocql.NewCluster(addr)
	cluster.Keyspace = "behavioral_memory"
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	sess, err := cluster.CreateSession()
	if err != nil {
		t.Fatalf("BEHAVIORAL_SCYLLA_ADDR=%s was set but unusable: %v\n"+
			"Failing rather than skipping: an explicitly requested parity run that quietly "+
			"degrades to in-memory only is worse than no run at all.", addr, err)
	}
	t.Cleanup(sess.Close)
	return append(out, namedStore{name: "ScyllaStore", store: NewScyllaStore(sess)})
}

// uniqueScope keeps parallel runs and repeated runs against a persistent Scylla
// from colliding — the Scylla leg writes to a real, shared keyspace.
func uniqueScope(prefix string) (project, domain string) {
	n := time.Now().UnixNano()
	return fmt.Sprintf("%s-%d", prefix, n), "cluster_operator"
}

// TestParity_ListOutcomesByThemeReturnsHydratedRecords is the contract the
// ScyllaStore bug violated.
//
// A theme lookup must return outcomes carrying the fields a caller acts on.
// GeneratePromotionCandidate derives a candidate's supporting evidence from
// these records; an outcome without EvidenceIDs silently makes every candidate
// unqualifiable, with no error anywhere.
func TestParity_ListOutcomesByThemeReturnsHydratedRecords(t *testing.T) {
	ctx := context.Background()
	const theme = "remediation.node.systemd.units_running"

	for _, s := range storesUnderTest(t) {
		t.Run(s.name, func(t *testing.T) {
			project, domain := uniqueScope("parity-outcomes")

			want := api.Outcome{
				ID: "outcome-1", Project: project, Domain: api.DomainRef(domain),
				Theme: theme, Status: "success",
				ActionCheckID: "check-1",
				EvidenceIDs:   []string{"ev-1"},
				Note:          "restarted drifted unit after observed finding",
				CreatedAt:     time.Now().Unix(),
			}
			if err := s.store.RecordOutcome(ctx, &want); err != nil {
				t.Fatalf("record outcome: %v", err)
			}

			got, err := s.store.ListOutcomesByTheme(ctx, project, domain, theme)
			if err != nil {
				t.Fatalf("list outcomes by theme: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d outcomes, want 1", len(got))
			}

			if len(got[0].EvidenceIDs) == 0 {
				t.Errorf("EvidenceIDs empty after a theme lookup.\n" +
					"This is the 2026-08-01 defect: the index carries no evidence column, so a\n" +
					"store that scans it alone returns hollow records and every promotion\n" +
					"candidate is refused with \"explicit supporting evidence is required\".")
			}
			if got[0].ActionCheckID == "" {
				t.Error("ActionCheckID empty — the decision the outcome followed is unrecoverable, " +
					"so the outcome cannot be attributed to the principle that allowed it")
			}
			if got[0].Status != want.Status {
				t.Errorf("Status = %q, want %q", got[0].Status, want.Status)
			}
		})
	}
}

// TestParity_UnknownThemeIsEmptyNotError verifies the common case agrees across
// stores. Most themes never repeat; asking about one is a normal question.
func TestParity_UnknownThemeIsEmptyNotError(t *testing.T) {
	ctx := context.Background()
	for _, s := range storesUnderTest(t) {
		t.Run(s.name, func(t *testing.T) {
			project, domain := uniqueScope("parity-unknown")
			got, err := s.store.ListOutcomesByTheme(ctx, project, domain, "remediation.never.happened")
			if err != nil {
				t.Fatalf("unknown theme must not error: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("got %d outcomes for an unrecorded theme, want 0", len(got))
			}
		})
	}
}

// TestParity_EvidenceSatisfactionNarrowsBySubject is the contract behind the
// governed-remediation refusal chased on 2026-08-01.
//
// A satisfaction lookup scoped to an entity must not return evidence about a
// DIFFERENT entity. When it does, the caller evaluates unrelated rows, and the
// operator is told their action was refused because of evidence describing
// something else entirely.
func TestParity_EvidenceSatisfactionNarrowsBySubject(t *testing.T) {
	ctx := context.Background()
	const (
		req     = "evidence.doctor.finding_observed"
		cluster = "globular.internal"
		mine    = "node-1/globular-torrent.service"
		theirs  = "node-1/sha256sum"
	)

	for _, s := range storesUnderTest(t) {
		t.Run(s.name, func(t *testing.T) {
			project, domain := uniqueScope("parity-satisfaction")
			now := time.Now().Unix()

			for i, ent := range []string{theirs, mine} {
				ev := api.Evidence{
					ID: fmt.Sprintf("ev-%d", i), Project: project, Domain: api.DomainRef(domain),
					Kind: "cluster_doctor_evidence", SourceKind: "cluster_doctor_evidence",
					Lane: api.EvidenceLane("RUNTIME_REQUIRED"), Result: "claim",
					EntityRef: ent, ClusterID: cluster,
					ConditionRef:   "node.systemd.units_running",
					AuthorityLevel: api.ObservationAuthorityDerived,
					ObservedAt:     now,
					Satisfies:      []api.RequiredEvidenceRef{api.RequiredEvidenceRef(req)},
				}
				if err := s.store.PutEvidence(ctx, &ev); err != nil {
					t.Fatalf("record evidence %s: %v", ent, err)
				}
			}

			got, err := s.store.ListEvidenceSatisfying(ctx, EvidenceSatisfactionQuery{
				Project: project, Domain: domain, RequiredEvidenceRef: req,
				ClusterID: cluster, EntityRef: mine,
			})
			if err != nil {
				t.Fatalf("list evidence satisfying: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d rows, want exactly 1 (only the requested entity)", len(got))
			}
			if got[0].EntityRef != mine {
				t.Errorf("EntityRef = %q, want %q — a subject-scoped lookup returned evidence\n"+
					"about a different entity, which is how a refusal ends up explained by an\n"+
					"unrelated row", got[0].EntityRef, mine)
			}
		})
	}
}
