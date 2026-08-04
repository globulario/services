package main

import (
	"strings"
	"testing"
	"time"

	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Recurrence identity for infra restarts.
//
// These tests exercise the identity contract over the drift-history authority's
// PERSISTED state: workflow.drift_unresolved. RecordDriftObservation preserves
// first_observed_at while the row lives and mints it only when the row is
// absent; ClearDriftObservation deletes the row. So one unresolved episode has
// exactly one first_observed_at, and a recurrence after resolution has a
// different one.
//
// Scope note, stated plainly: these are deterministic-identity proofs over
// persisted rows, not live restart drills. They prove the correlation identity
// is a pure function of persisted drift state with no process-local or
// wall-clock input — which is what makes it survive a controller restart, a
// Workflow Service restart, and a lost dispatch response. They do not
// substitute for exercising those events against a running cluster.

const testCluster = "globular.internal"

// driftRow builds one persisted unresolved-drift row.
func driftRow(driftType, entityRef string, firstObserved time.Time, cycles int32) *workflowpb.DriftUnresolved {
	return &workflowpb.DriftUnresolved{
		ClusterId:         testCluster,
		DriftType:         driftType,
		EntityRef:         entityRef,
		ConsecutiveCycles: cycles,
		FirstObservedAt:   timestamppb.New(firstObserved),
		LastObservedAt:    timestamppb.New(firstObserved.Add(time.Duration(cycles) * time.Minute)),
	}
}

// correlationFor resolves the episode identity from persisted rows and builds
// the correlation id exactly as the dispatch path does.
func correlationFor(t *testing.T, rows []*workflowpb.DriftUnresolved, entityRef, node, component string) string {
	t.Helper()
	ep, err := episodeIDFromDriftRows(rows, "infra_unhealthy", entityRef)
	if err != nil {
		t.Fatalf("episode identity unavailable for %s: %v", entityRef, err)
	}
	return restartInfraCorrelationID(testCluster, ep, node, component)
}

// ── 1. repeated ticks in one episode ──────────────────────────────────

// While one episode stays unresolved the row is re-observed, not re-minted:
// consecutive_cycles and last_observed_at advance but first_observed_at does
// not. Every tick must therefore reconcile to the same child run.
func TestEpisode_RepeatedTicksReconcileToOneRun(t *testing.T) {
	const entity = "node-a/etcd"
	firstSeen := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

	var ids []string
	for cycle := int32(1); cycle <= 5; cycle++ {
		// The authority's row as it looks on tick N of the SAME episode.
		rows := []*workflowpb.DriftUnresolved{driftRow("infra_unhealthy", entity, firstSeen, cycle)}
		ids = append(ids, correlationFor(t, rows, entity, "node-a", "etcd"))
	}
	for i, got := range ids {
		if got != ids[0] {
			t.Fatalf("tick %d allocated a different run: %s != %s", i+1, got, ids[0])
		}
	}
	if len(ids) != 5 {
		t.Fatalf("expected 5 ticks, got %d", len(ids))
	}
}

// ── 2. resolution then recurrence — PRIMARY ACCEPTANCE ────────────────

// Episode A is remedied and its row is cleared. The same node/component later
// fails again: the authority mints a NEW first_observed_at, so episode B must
// receive a different child run. Episode A's record is untouched.
func TestEpisode_RecurrenceAfterResolutionGetsANewRun(t *testing.T) {
	const entity = "node-a/etcd"

	// Episode A: observed, remediated.
	epAFirstSeen := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	rowsA := []*workflowpb.DriftUnresolved{driftRow("infra_unhealthy", entity, epAFirstSeen, 2)}
	runA := correlationFor(t, rowsA, entity, "node-a", "etcd")

	// Drift clears: ClearDriftObservation DELETEs the row, so the authority
	// now holds nothing for this entity.
	rowsCleared := []*workflowpb.DriftUnresolved{}
	if _, err := episodeIDFromDriftRows(rowsCleared, "infra_unhealthy", entity); err == nil {
		t.Fatal("a cleared entity must not resolve to an episode identity")
	}

	// Episode B: the component fails again three hours later. A fresh row is
	// inserted, so first_observed_at is new.
	epBFirstSeen := epAFirstSeen.Add(3 * time.Hour)
	rowsB := []*workflowpb.DriftUnresolved{driftRow("infra_unhealthy", entity, epBFirstSeen, 1)}
	runB := correlationFor(t, rowsB, entity, "node-a", "etcd")

	if runA == runB {
		t.Fatalf("episode B aliased onto episode A's run (%s) — a later independent "+
			"outage was recorded as a replay of the earlier one, so the second "+
			"restart could not execute and the two episodes collapsed into one "+
			"durable record", runA)
	}

	// Episode A's identity must not have been mutated by B's arrival.
	if again := correlationFor(t, rowsA, entity, "node-a", "etcd"); again != runA {
		t.Errorf("episode A's identity changed after B began: %s != %s", again, runA)
	}
}

// ── 3./4./5. reconstruction and indeterminate dispatch ────────────────

// The identity is a pure function of the persisted row. Rebuilding the value
// from the same row — which is what a restarted controller, a restarted
// Workflow Service, or a retry after a lost response all do — must reproduce
// the same run. Nothing process-local participates.
func TestEpisode_IdentitySurvivesReconstructionAndRetry(t *testing.T) {
	const entity = "node-a/minio"
	firstSeen := time.Date(2026, 8, 2, 11, 30, 0, 0, time.UTC)
	row := func() []*workflowpb.DriftUnresolved {
		return []*workflowpb.DriftUnresolved{driftRow("infra_unhealthy", entity, firstSeen, 1)}
	}
	before := correlationFor(t, row(), entity, "node-a", "minio")

	for _, event := range []string{
		"controller reconstructed between ticks",
		"workflow service reconstructed after persisting the child",
		"dispatch response lost, remediation retried",
	} {
		// Each event rebuilds the value from freshly-read persisted state.
		if got := correlationFor(t, row(), entity, "node-a", "minio"); got != before {
			t.Errorf("%s: identity changed (%s != %s) — a new run would be allocated",
				event, got, before)
		}
	}

	// The row's mutable columns advance while the episode continues; they must
	// not perturb identity, or a restart mid-episode would fork the run.
	advanced := []*workflowpb.DriftUnresolved{driftRow("infra_unhealthy", entity, firstSeen, 9)}
	if got := correlationFor(t, advanced, entity, "node-a", "minio"); got != before {
		t.Errorf("advancing consecutive_cycles/last_observed_at changed identity: %s != %s", got, before)
	}
}

// ── 6. separate nodes ─────────────────────────────────────────────────

// The same component failing on two nodes at the same moment is two episodes on
// two hosts, even if both rows carry an identical first_observed_at.
func TestEpisode_SeparateNodesGetSeparateRuns(t *testing.T) {
	sameInstant := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	rows := []*workflowpb.DriftUnresolved{
		driftRow("infra_unhealthy", "node-a/scylladb", sameInstant, 1),
		driftRow("infra_unhealthy", "node-b/scylladb", sameInstant, 1),
	}
	a := correlationFor(t, rows, "node-a/scylladb", "node-a", "scylladb")
	b := correlationFor(t, rows, "node-b/scylladb", "node-b", "scylladb")
	if a == b {
		t.Fatalf("two nodes share one run identity: %s", a)
	}
}

// ── 7. separate components ────────────────────────────────────────────

// etcd and MinIO unhealthy on ONE node are separate episodes with separate
// rows, and must not collapse onto a single restart run.
func TestEpisode_SeparateComponentsGetSeparateRuns(t *testing.T) {
	sameInstant := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	rows := []*workflowpb.DriftUnresolved{
		driftRow("infra_unhealthy", "node-a/etcd", sameInstant, 1),
		driftRow("infra_unhealthy", "node-a/minio", sameInstant, 1),
	}
	etcd := correlationFor(t, rows, "node-a/etcd", "node-a", "etcd")
	minio := correlationFor(t, rows, "node-a/minio", "node-a", "minio")
	if etcd == minio {
		t.Fatalf("etcd and minio on one node share a run identity: %s", etcd)
	}
}

// ── 8. missing episode authority — FAIL CLOSED ────────────────────────

// When the episode cannot be proven, no identity may be invented. Each case
// must yield an error, never a usable id.
func TestEpisode_MissingAuthorityFailsClosed(t *testing.T) {
	const entity = "node-a/etcd"
	good := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		rows []*workflowpb.DriftUnresolved
	}{
		{"no rows at all", nil},
		{"empty row set", []*workflowpb.DriftUnresolved{}},
		{"row for a different entity", []*workflowpb.DriftUnresolved{
			driftRow("infra_unhealthy", "node-b/etcd", good, 1)}},
		{"row for a different drift type", []*workflowpb.DriftUnresolved{
			driftRow("missing_package", entity, good, 1)}},
		{"corrupt: nil first_observed_at", []*workflowpb.DriftUnresolved{
			{ClusterId: testCluster, DriftType: "infra_unhealthy", EntityRef: entity}}},
		{"corrupt: zero first_observed_at", []*workflowpb.DriftUnresolved{
			driftRow("infra_unhealthy", entity, time.Time{}, 1)}},
		{"nil row in the set", []*workflowpb.DriftUnresolved{nil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := episodeIDFromDriftRows(tc.rows, "infra_unhealthy", entity)
			if err == nil {
				t.Fatalf("expected fail-closed, got episode id %q", id)
			}
			if id != "" {
				t.Errorf("failed lookup must not yield an id, got %q", id)
			}
			var missing *errNoDriftEpisode
			if !asNoDriftEpisode(err, &missing) {
				t.Errorf("error %v is not classified as a missing-episode error", err)
			}
			if !strings.Contains(err.Error(), entity) {
				t.Errorf("error %q does not surface the entity whose episode is missing", err)
			}
		})
	}
}

func asNoDriftEpisode(err error, target **errNoDriftEpisode) bool {
	e, ok := err.(*errNoDriftEpisode)
	if ok {
		*target = e
	}
	return ok
}

// The dispatch path must convert a missing episode into a no-op that names the
// reason, never into a restart. This is the source-level guard on the branch.
func TestEpisode_DispatchFailsClosedWithClassifiedReason(t *testing.T) {
	src := reconcileActionsSource(t)
	i := strings.Index(src, `case "infra_unhealthy":`)
	if i < 0 {
		t.Fatal("no infra_unhealthy branch in reconcileChooseWorkflow")
	}
	block := src[i:]
	if j := strings.Index(block, `case "unmanaged_package":`); j > 0 {
		block = block[:j]
	}
	if !strings.Contains(block, "driftEpisodeID") {
		t.Error("dispatch does not consult the drift-history authority for an episode identity")
	}
	if !strings.Contains(block, "episode_identity_unavailable") {
		t.Error("dispatch does not emit a classified reason when the episode cannot be proven")
	}
	// The fail-closed branch must precede the restart dispatch.
	epErr := strings.Index(block, "episode_identity_unavailable")
	dispatch := strings.Index(block, `"workflow_name": "node.restart_infra_unit"`)
	if epErr < 0 || dispatch < 0 || epErr > dispatch {
		t.Error("the missing-episode guard does not precede the restart dispatch")
	}
}

// ── 9. historical integrity ───────────────────────────────────────────

// Two completed episodes for one entity must remain independently addressable.
// Identity is what keeps their runs, receipts and terminal states separate: if
// the ids collided, the second episode would overwrite the first's record.
func TestEpisode_CompletedEpisodesRemainIndependentlyQueryable(t *testing.T) {
	const entity = "node-a/scylladb"
	starts := []time.Time{
		time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 3, 2, 0, 0, 0, time.UTC),
	}
	seen := map[string]time.Time{}
	for _, start := range starts {
		rows := []*workflowpb.DriftUnresolved{driftRow("infra_unhealthy", entity, start, 1)}
		id := correlationFor(t, rows, entity, "node-a", "scylladb")
		if prev, dup := seen[id]; dup {
			t.Fatalf("episode starting %s reuses the run of the episode starting %s (%s) — "+
				"the earlier episode's receipts and terminal state would be overwritten",
				start, prev, id)
		}
		seen[id] = start
	}
	if len(seen) != len(starts) {
		t.Fatalf("expected %d distinct runs, got %d", len(starts), len(seen))
	}

	// Each episode's identity must still be reconstructible from its own row
	// after later episodes exist — history is addressable, not just distinct.
	for id, start := range seen {
		rows := []*workflowpb.DriftUnresolved{driftRow("infra_unhealthy", entity, start, 1)}
		if got := correlationFor(t, rows, entity, "node-a", "scylladb"); got != id {
			t.Errorf("episode starting %s no longer resolves to its own run: %s != %s", start, got, id)
		}
	}
}

// The episode identity must render the authority's persisted value, not a value
// this process invented. Re-reading the same row at any later moment yields the
// same id.
func TestEpisodeID_RendersThePersistedValueOnly(t *testing.T) {
	firstSeen := time.Date(2026, 8, 2, 10, 15, 30, 123456789, time.UTC)
	rows := []*workflowpb.DriftUnresolved{driftRow("infra_unhealthy", "node-a/etcd", firstSeen, 1)}

	id, err := episodeIDFromDriftRows(rows, "infra_unhealthy", "node-a/etcd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := firstSeen.UTC().Format(time.RFC3339Nano); id != want {
		t.Errorf("episode id %q is not the persisted first_observed_at %q", id, want)
	}
	time.Sleep(2 * time.Millisecond)
	again, err := episodeIDFromDriftRows(rows, "infra_unhealthy", "node-a/etcd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if again != id {
		t.Errorf("episode id moved with the clock: %s != %s", again, id)
	}
}
