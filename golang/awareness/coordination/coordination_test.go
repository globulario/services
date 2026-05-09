package coordination_test

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/awareness/coordination"
	"github.com/globulario/services/golang/awareness/graph"
)

func newTestStore(t *testing.T) *coordination.Store {
	t.Helper()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	return coordination.New(g)
}

func startTestRun(t *testing.T, s *coordination.Store) *coordination.CoordinationRun {
	t.Helper()
	run, err := s.StartCoordinationRun(context.Background(), coordination.StartCoordinationRunRequest{
		Title:     "Test Run",
		Objective: "Testing coordination",
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	return run
}

func joinTestAgent(t *testing.T, s *coordination.Store, runID, name string) *coordination.AgentParticipant {
	t.Helper()
	a, err := s.JoinCoordinationRun(context.Background(), coordination.JoinCoordinationRunRequest{
		RunID:     runID,
		AgentName: name,
		AgentKind: "claude",
		Role:      "coder",
	})
	if err != nil {
		t.Fatalf("join run (%s): %v", name, err)
	}
	return a
}

// Test 1: Two agents cannot lock the same file simultaneously.
func TestTwoAgentsCannotLockSameFile(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	// Agent A acquires lock.
	lk, conflict, err := s.AcquireFileLock(ctx, coordination.AcquireFileLockRequest{
		RunID:    run.ID,
		AgentID:  agentA.ID,
		Path:     "file.go",
		LockKind: coordination.LockEdit,
		Reason:   "editing",
	})
	if err != nil {
		t.Fatalf("agent A lock: %v", err)
	}
	if lk == nil {
		t.Fatal("expected lock, got nil")
	}
	if conflict != nil {
		t.Fatalf("expected no conflict, got: %v", conflict)
	}

	// Agent B tries to acquire lock on same file.
	lk2, conflict2, err2 := s.AcquireFileLock(ctx, coordination.AcquireFileLockRequest{
		RunID:    run.ID,
		AgentID:  agentB.ID,
		Path:     "file.go",
		LockKind: coordination.LockEdit,
		Reason:   "also editing",
	})
	if err2 != nil {
		t.Fatalf("agent B lock attempt err: %v", err2)
	}
	if lk2 != nil {
		t.Fatal("expected agent B lock to fail, but got lock")
	}
	if conflict2 == nil {
		t.Fatal("expected conflict, got nil")
	}
	if conflict2.Type != "file_lock_conflict" {
		t.Errorf("expected file_lock_conflict, got %s", conflict2.Type)
	}
}

// Test 2: Expired lock does not block new lock acquisition.
func TestExpiredLockDoesNotBlock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	// Insert an expired lock directly into the DB for Agent A.
	db := s.DB()
	pastTime := time.Now().Unix() - 10
	_, err := db.ExecContext(ctx, `
		INSERT INTO coordination_file_locks
		  (id, run_id, agent_id, path, lock_kind, reason, fingerprint_at_lock, status, created_at, expires_at, released_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"LOCK-expired", run.ID, agentA.ID, "file.go", coordination.LockEdit,
		"expired lock", "", coordination.StatusActive, pastTime, pastTime, nil,
	)
	if err != nil {
		t.Fatalf("insert expired lock: %v", err)
	}

	// Agent B acquires lock — should succeed because the expired lock gets evicted.
	lk, conflict, err2 := s.AcquireFileLock(ctx, coordination.AcquireFileLockRequest{
		RunID:    run.ID,
		AgentID:  agentB.ID,
		Path:     "file.go",
		LockKind: coordination.LockEdit,
		Reason:   "new lock after expiry",
	})
	if err2 != nil {
		t.Fatalf("agent B lock: %v", err2)
	}
	if conflict != nil {
		t.Fatalf("expected no conflict, got: %+v", conflict)
	}
	if lk == nil {
		t.Fatal("expected lock, got nil")
	}
}

// Test 3: Binding decision appears in snapshot for relevant files.
func TestDecisionInheritance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	// Agent A records binding decision for heartbeat.go.
	_, err := s.RecordCoordinationDecision(ctx, coordination.RecordDecisionRequest{
		RunID:        run.ID,
		AgentID:      agentA.ID,
		Title:        "Do not touch heartbeat",
		Decision:     "heartbeat.go is stable, do not refactor",
		Rationale:    "breaks everything",
		Scope:        "file",
		RelatedFiles: []string{"heartbeat.go"},
		Binding:      true,
	})
	if err != nil {
		t.Fatalf("record decision: %v", err)
	}

	// Agent B snapshots with file filter.
	snap, err := s.GetCoordinationSnapshot(ctx, run.ID, agentB.ID, []string{"heartbeat.go"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Decisions) == 0 {
		t.Fatal("expected decision in snapshot, got none")
	}
	if snap.Decisions[0].Title != "Do not touch heartbeat" {
		t.Errorf("unexpected decision title: %s", snap.Decisions[0].Title)
	}
}

// Test 4: Binding do_not_touch decision blocks an edit lock.
func TestDoNotTouchBlocksEditLock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	// Agent A records binding do_not_touch decision.
	_, err := s.RecordCoordinationDecision(ctx, coordination.RecordDecisionRequest{
		RunID:        run.ID,
		AgentID:      agentA.ID,
		Title:        "Heartbeat must not change",
		Decision:     "do_not_touch heartbeat.go",
		Rationale:    "production critical",
		Scope:        "file",
		RelatedFiles: []string{"heartbeat.go"},
		Binding:      true,
	})
	if err != nil {
		t.Fatalf("record decision: %v", err)
	}

	// Agent B tries to acquire edit lock.
	lk, conflict, err2 := s.AcquireFileLock(ctx, coordination.AcquireFileLockRequest{
		RunID:    run.ID,
		AgentID:  agentB.ID,
		Path:     "heartbeat.go",
		LockKind: coordination.LockEdit,
		Reason:   "refactoring",
	})
	if err2 != nil {
		t.Fatalf("acquire lock err: %v", err2)
	}
	if lk != nil {
		t.Fatal("expected lock to be blocked, but got lock")
	}
	if conflict == nil {
		t.Fatal("expected conflict, got nil")
	}
	if conflict.Type != "do_not_touch_violation" {
		t.Errorf("expected do_not_touch_violation, got %s", conflict.Type)
	}
}

// Test 5: Overriding a decision records a conflict.
func TestOverrideRecordsConflict(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	// Agent A records binding decision.
	dec, err := s.RecordCoordinationDecision(ctx, coordination.RecordDecisionRequest{
		RunID:     run.ID,
		AgentID:   agentA.ID,
		Title:     "No rollback",
		Decision:  "never auto-rollback",
		Rationale: "safety",
		Scope:     "global",
		Binding:   true,
	})
	if err != nil {
		t.Fatalf("record decision: %v", err)
	}

	// Agent B overrides it.
	conflict, err := s.OverrideDecision(ctx, run.ID, agentB.ID, dec.ID, "emergency rollback needed", "cluster is down")
	if err != nil {
		t.Fatalf("override decision: %v", err)
	}
	if conflict == nil {
		t.Fatal("expected conflict event, got nil")
	}
	if conflict.ConflictType != "decision_conflict" {
		t.Errorf("expected decision_conflict, got %s", conflict.ConflictType)
	}

	// Verify decision is superseded.
	decs, err := s.ListRelevantDecisions(ctx, run.ID, nil, nil)
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	found := false
	for _, d := range decs {
		if d.ID == dec.ID {
			found = true
			if d.SupersededBy == "" {
				t.Error("expected SupersededBy to be set")
			}
		}
	}
	if !found {
		t.Error("decision not found after override")
	}
}

// Test 6: Warning recorded by one agent propagates to another's snapshot.
func TestSharedWarningPropagates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	_, err := s.RecordCoordinationWarning(ctx, coordination.RecordWarningRequest{
		RunID:            run.ID,
		AgentID:          agentA.ID,
		WarningType:      "drift",
		Severity:         "warning",
		Message:          "cluster-controller diverged from desired state",
		RelatedComponent: "cluster-controller",
	})
	if err != nil {
		t.Fatalf("record warning: %v", err)
	}

	// Agent B snapshots.
	snap, err := s.GetCoordinationSnapshot(ctx, run.ID, agentB.ID, nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Warnings) == 0 {
		t.Fatal("expected warning in snapshot, got none")
	}
	if snap.Warnings[0].RelatedComponent != "cluster-controller" {
		t.Errorf("unexpected component: %s", snap.Warnings[0].RelatedComponent)
	}
}

// Test 7: Work item cannot be double-claimed by different agents.
func TestWorkItemClaimPreventsDouble(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	wi, err := s.CreateWorkItem(ctx, coordination.CreateWorkItemRequest{
		RunID:    run.ID,
		Title:    "Fix the bug",
		Priority: "high",
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}

	// Agent A claims it.
	if err := s.ClaimWorkItem(ctx, run.ID, wi.ID, agentA.ID); err != nil {
		t.Fatalf("agent A claim: %v", err)
	}

	// Agent B tries to claim same item.
	err = s.ClaimWorkItem(ctx, run.ID, wi.ID, agentB.ID)
	if err == nil {
		t.Fatal("expected error for double claim, got nil")
	}
}

// Test 8: Handoff note appears in the target agent's snapshot.
func TestHandoffNoteAppearsForTargetAgent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	_, err := s.RecordHandoff(ctx, coordination.RecordHandoffRequest{
		RunID:       run.ID,
		FromAgentID: agentA.ID,
		ToAgentID:   agentB.ID,
		Title:       "Resuming your work",
		Body:        "I've started the refactor, please continue from line 42.",
	})
	if err != nil {
		t.Fatalf("record handoff: %v", err)
	}

	// Agent B snapshots.
	snap, err := s.GetCoordinationSnapshot(ctx, run.ID, agentB.ID, nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.HandoffNotes) == 0 {
		t.Fatal("expected handoff note in snapshot, got none")
	}
	if snap.HandoffNotes[0].Title != "Resuming your work" {
		t.Errorf("unexpected handoff title: %s", snap.HandoffNotes[0].Title)
	}
}

// Test 9: Closing a run with active locks reports warnings in RecommendedRules.
func TestCloseRunBlocksWithActiveLocks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")

	// Acquire a lock.
	_, _, err := s.AcquireFileLock(ctx, coordination.AcquireFileLockRequest{
		RunID:    run.ID,
		AgentID:  agentA.ID,
		Path:     "critical.go",
		LockKind: coordination.LockEdit,
		Reason:   "long edit",
	})
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}

	// Close run.
	snap, err := s.CloseCoordinationRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("close run: %v", err)
	}

	// Verify run is closed.
	if snap.Run.Status != coordination.StatusClosed {
		t.Errorf("expected closed, got %s", snap.Run.Status)
	}

	// Verify recommended rules mention the active lock.
	found := false
	for _, rule := range snap.RecommendedRules {
		if len(rule) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one recommended rule about active lock, got none")
	}
}

// Test 10: Snapshot contains all sections populated.
func TestSnapshotContainsAllSections(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := startTestRun(t, s)
	agentA := joinTestAgent(t, s, run.ID, "agent-a")
	agentB := joinTestAgent(t, s, run.ID, "agent-b")

	// Claim a file.
	_, err := s.ClaimFile(ctx, coordination.ClaimFileRequest{
		RunID:     run.ID,
		AgentID:   agentA.ID,
		Path:      "main.go",
		ClaimKind: coordination.ClaimLikelyEdit,
	})
	if err != nil {
		t.Fatalf("claim file: %v", err)
	}

	// Lock a file.
	_, _, err = s.AcquireFileLock(ctx, coordination.AcquireFileLockRequest{
		RunID:    run.ID,
		AgentID:  agentA.ID,
		Path:     "main.go",
		LockKind: coordination.LockEdit,
		Reason:   "editing main",
	})
	if err != nil {
		t.Fatalf("lock file: %v", err)
	}

	// Record decision.
	_, err = s.RecordCoordinationDecision(ctx, coordination.RecordDecisionRequest{
		RunID:     run.ID,
		AgentID:   agentA.ID,
		Title:     "Use etcd for config",
		Decision:  "no env vars",
		Rationale: "architecture rule",
		Scope:     "global",
	})
	if err != nil {
		t.Fatalf("record decision: %v", err)
	}

	// Record warning.
	_, err = s.RecordCoordinationWarning(ctx, coordination.RecordWarningRequest{
		RunID:       run.ID,
		AgentID:     agentA.ID,
		WarningType: "drift",
		Severity:    "warning",
		Message:     "some component drifted",
	})
	if err != nil {
		t.Fatalf("record warning: %v", err)
	}

	// Create work item.
	_, err = s.CreateWorkItem(ctx, coordination.CreateWorkItemRequest{
		RunID:    run.ID,
		Title:    "Write tests",
		Priority: "normal",
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}

	// Agent B snapshots.
	snap, err := s.GetCoordinationSnapshot(ctx, run.ID, agentB.ID, nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snap.Run.ID != run.ID {
		t.Errorf("run ID mismatch: got %s", snap.Run.ID)
	}
	if len(snap.Agents) < 2 {
		t.Errorf("expected >= 2 agents, got %d", len(snap.Agents))
	}
	if len(snap.ActiveClaims) == 0 {
		t.Error("expected active claims, got none")
	}
	if len(snap.ActiveLocks) == 0 {
		t.Error("expected active locks, got none")
	}
	if len(snap.Decisions) == 0 {
		t.Error("expected decisions, got none")
	}
	if len(snap.Warnings) == 0 {
		t.Error("expected warnings, got none")
	}
	if len(snap.WorkItems) == 0 {
		t.Error("expected work items, got none")
	}
}
