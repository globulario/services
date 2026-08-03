package store

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

const (
	tProject = "globular-services"
	tDomain  = "cluster_operator"
	tRef     = "evidence.objectstore.live_write_quorum"
)

func ev(id string, observedAt int64, mut ...func(*api.Evidence)) *api.Evidence {
	e := &api.Evidence{
		ID:         id,
		Project:    tProject,
		Domain:     api.DomainRef(tDomain),
		ObservedAt: observedAt,
		ClusterID:  "cluster-a",
		Satisfies:  []api.RequiredEvidenceRef{api.RequiredEvidenceRef(tRef)},
		Result:     "pass",
	}
	for _, f := range mut {
		f(e)
	}
	return e
}

func query(mut ...func(*EvidenceSatisfactionQuery)) EvidenceSatisfactionQuery {
	q := EvidenceSatisfactionQuery{
		Project:             tProject,
		Domain:              tDomain,
		RequiredEvidenceRef: tRef,
		ClusterID:           "cluster-a",
	}
	for _, f := range mut {
		f(&q)
	}
	return q
}

func seed(t *testing.T, evs ...*api.Evidence) *MemoryStore {
	t.Helper()
	m := NewMemoryStore()
	for _, e := range evs {
		if err := m.PutEvidence(context.Background(), e); err != nil {
			t.Fatalf("PutEvidence(%s): %v", e.ID, err)
		}
	}
	return m
}

// The core contract: evidence declaring the reference is found; evidence that
// does not is not.
func TestListEvidenceSatisfying_MatchesOnSatisfiesRef(t *testing.T) {
	m := seed(t,
		ev("match", 100),
		ev("other", 100, func(e *api.Evidence) {
			e.Satisfies = []api.RequiredEvidenceRef{"evidence.something.else"}
		}),
		ev("none", 100, func(e *api.Evidence) { e.Satisfies = nil }),
	)

	got, err := m.ListEvidenceSatisfying(context.Background(), query())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "match" {
		t.Fatalf("want only [match], got %v", ids(got))
	}
}

// Cluster scope is an exact match, never a wildcard. Empty ClusterID selects the
// cluster-less partition — it must NOT act as "any cluster", or one cluster's
// evidence could authorize another's action.
func TestListEvidenceSatisfying_ClusterScopeIsExact(t *testing.T) {
	m := seed(t,
		ev("a", 100),
		ev("b", 100, func(e *api.Evidence) { e.ClusterID = "cluster-b" }),
		ev("none", 100, func(e *api.Evidence) { e.ClusterID = "" }),
	)

	got, _ := m.ListEvidenceSatisfying(context.Background(), query())
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("cluster-a query → [a], got %v", ids(got))
	}

	got, _ = m.ListEvidenceSatisfying(context.Background(), query(func(q *EvidenceSatisfactionQuery) {
		q.ClusterID = ""
	}))
	if len(got) != 1 || got[0].ID != "none" {
		t.Fatalf("empty-cluster query selects the cluster-less partition, got %v", ids(got))
	}
}

// Freshness is caller policy: rows older than the bound are excluded.
func TestListEvidenceSatisfying_AgeBound(t *testing.T) {
	m := seed(t, ev("old", 50), ev("new", 500))

	got, _ := m.ListEvidenceSatisfying(context.Background(), query(func(q *EvidenceSatisfactionQuery) {
		q.NotOlderThan = 100
	}))
	if len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("age bound 100 → [new], got %v", ids(got))
	}

	got, _ = m.ListEvidenceSatisfying(context.Background(), query())
	if len(got) != 2 {
		t.Fatalf("no age bound → both rows, got %v", ids(got))
	}
}

// Newest-first, matching CLUSTERING ORDER BY (observed_at DESC): a caller taking
// the first row gets the freshest evidence.
func TestListEvidenceSatisfying_NewestFirst(t *testing.T) {
	m := seed(t, ev("mid", 200), ev("oldest", 100), ev("newest", 300))

	got, _ := m.ListEvidenceSatisfying(context.Background(), query())
	if len(got) != 3 {
		t.Fatalf("want 3, got %v", ids(got))
	}
	if got[0].ID != "newest" || got[2].ID != "oldest" {
		t.Errorf("want newest→oldest, got %v", ids(got))
	}
}

func TestListEvidenceSatisfying_NarrowsByConditionAndEntity(t *testing.T) {
	m := seed(t,
		ev("c1e1", 100, func(e *api.Evidence) { e.ConditionRef = "c1"; e.EntityRef = "e1" }),
		ev("c1e2", 100, func(e *api.Evidence) { e.ConditionRef = "c1"; e.EntityRef = "e2" }),
		ev("c2e1", 100, func(e *api.Evidence) { e.ConditionRef = "c2"; e.EntityRef = "e1" }),
	)

	got, _ := m.ListEvidenceSatisfying(context.Background(), query(func(q *EvidenceSatisfactionQuery) {
		q.ConditionRef = "c1"
	}))
	if len(got) != 2 {
		t.Errorf("condition c1 → 2 rows, got %v", ids(got))
	}

	got, _ = m.ListEvidenceSatisfying(context.Background(), query(func(q *EvidenceSatisfactionQuery) {
		q.ConditionRef = "c1"
		q.EntityRef = "e2"
	}))
	if len(got) != 1 || got[0].ID != "c1e2" {
		t.Errorf("condition c1 + entity e2 → [c1e2], got %v", ids(got))
	}
}

// An unbounded governance lookup is a denial of service on the decision path.
func TestListEvidenceSatisfying_LimitIsApplied(t *testing.T) {
	var evs []*api.Evidence
	for i := 0; i < 10; i++ {
		evs = append(evs, ev(string(rune('a'+i)), int64(100+i)))
	}
	m := seed(t, evs...)

	got, _ := m.ListEvidenceSatisfying(context.Background(), query(func(q *EvidenceSatisfactionQuery) {
		q.Limit = 3
	}))
	if len(got) != 3 {
		t.Fatalf("limit 3 → 3 rows, got %d", len(got))
	}
	// The limit must keep the FRESHEST rows, not an arbitrary three.
	if got[0].ObservedAt != 109 {
		t.Errorf("limit must retain newest first, got observed_at=%d", got[0].ObservedAt)
	}
}

// A malformed query is an error, not an empty result. Returning "no evidence"
// for a query that could never match would read as "no evidence exists" and
// silently under-govern the action.
func TestListEvidenceSatisfying_RejectsIncompleteQuery(t *testing.T) {
	m := NewMemoryStore()
	for _, q := range []EvidenceSatisfactionQuery{
		{Domain: tDomain, RequiredEvidenceRef: tRef},
		{Project: tProject, RequiredEvidenceRef: tRef},
		{Project: tProject, Domain: tDomain},
	} {
		if _, err := m.ListEvidenceSatisfying(context.Background(), q); err == nil {
			t.Errorf("incomplete query %+v must error, not return empty", q)
		}
	}
}

// Multi-ref evidence is findable under each reference it declares.
func TestListEvidenceSatisfying_MultipleRefs(t *testing.T) {
	m := seed(t, ev("multi", 100, func(e *api.Evidence) {
		e.Satisfies = []api.RequiredEvidenceRef{
			api.RequiredEvidenceRef(tRef),
			"evidence.objectstore.pool_membership",
		}
	}))

	for _, ref := range []string{tRef, "evidence.objectstore.pool_membership"} {
		got, _ := m.ListEvidenceSatisfying(context.Background(), query(func(q *EvidenceSatisfactionQuery) {
			q.RequiredEvidenceRef = ref
		}))
		if len(got) != 1 {
			t.Errorf("ref %q → 1 row, got %v", ref, ids(got))
		}
	}
}

// The Unconfigured store must refuse rather than report an empty result — a
// missing backend is not evidence of missing evidence.
func TestListEvidenceSatisfying_UnconfiguredRefuses(t *testing.T) {
	if _, err := (Unconfigured{}).ListEvidenceSatisfying(context.Background(), query()); err == nil {
		t.Error("unconfigured store must return ErrUnconfigured, not an empty result")
	}
}

func ids(evs []api.Evidence) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.ID
	}
	return out
}
